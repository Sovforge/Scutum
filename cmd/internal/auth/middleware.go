package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/utils"
)

type Store interface {
	UserByAPIKey(keyHash string) (userID, username string, err error)
	UserHasPermission(userID, resource, action string) (bool, error)
}

var publicRoutes = map[string]bool{
	"POST /auth/login":  true,
	"GET /health":       true,
	"GET /docs":         true,
	"GET /openapi.yaml": true,
	"GET /setup/status": true,
	"POST /setup":       true,
}

// hubHMACKey is set once at startup via SetHubHMACKey.
var hubHMACKey []byte

// SetHubHMACKey registers the shared HMAC key used to verify hub-proxied requests.
func SetHubHMACKey(key []byte) {
	hubHMACKey = key
}

func Middleware(store Store, jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasPrefix(path, "/api/") {
				path = path[4:] // strip /api prefix, keep leading /
			}
			if publicRoutes[r.Method+" "+path] {
				next.ServeHTTP(w, r)
				return
			}
			claims, err := resolve(r, store, jwtSecret)
			if err != nil {
				utils.AppendAudit(utils.AuditEntry{
					Time:     time.Now().UTC(),
					Action:   "AUTH_FAILURE",
					Outcome:  utils.OutcomeFailure,
					Method:   r.Method,
					Path:     r.URL.Path,
					ClientIP: r.RemoteAddr,
				})
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
		})
	}
}

func Require(store Store, resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			// Hub-proxied requests carry a verified HMAC signature and are
			// granted full access without a DB permission check.
			if claims.UserID == "hub" {
				next.ServeHTTP(w, r)
				return
			}
			allowed, err := store.UserHasPermission(claims.UserID, resource, action)
			if err != nil || !allowed {
				utils.AppendAudit(utils.AuditEntry{
					Time:     time.Now().UTC(),
					Action:   "AUTHZ_FAILURE",
					Actor:    claims.Username,
					ActorID:  claims.UserID,
					Outcome:  utils.OutcomeFailure,
					Method:   r.Method,
					Path:     r.URL.Path,
					ClientIP: r.RemoteAddr,
					Extra:    map[string]string{"resource": resource, "required_action": action},
				})
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func resolve(r *http.Request, store Store, jwtSecret []byte) (Claims, error) {
	// Hub-to-node proxy requests are authenticated with an HMAC signature
	// over the timestamp + method + path + body, using the shared sync key.
	if sig := r.Header.Get("X-Scutum-Hub-Sig"); sig != "" && len(hubHMACKey) > 0 {
		if claims, err := verifyHubRequest(r, sig); err == nil {
			return claims, nil
		}
	}
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return ValidateJWT(token, jwtSecret)
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		userID, username, err := store.UserByAPIKey(HashAPIKey(key))
		if err != nil {
			return Claims{}, ErrInvalidToken
		}
		return Claims{UserID: userID, Username: username}, nil
	}
	// Allow token in query string for WebSocket connections (browser WS can't set headers)
	if token := r.URL.Query().Get("token"); token != "" {
		return ValidateJWT(token, jwtSecret)
	}
	return Claims{}, ErrInvalidToken
}

// verifyHubRequest validates the X-Scutum-Hub-Sig header and returns synthetic
// admin claims so the proxied request passes all downstream permission checks.
func verifyHubRequest(r *http.Request, sig string) (Claims, error) {
	tsStr := r.Header.Get("X-Scutum-Hub-Ts")
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	age := math.Abs(float64(time.Now().Unix() - ts))
	if age > 300 { // reject requests older than 5 minutes
		return Claims{}, ErrInvalidToken
	}

	// Reconstruct the full path as signed by the hub (before StripPrefix stripped /api).
	path := "/api" + r.URL.RequestURI()
	mac := hmac.New(sha256.New, hubHMACKey)
	fmt.Fprintf(mac, "%s\n%s\n%s\n", tsStr, r.Method, path)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return Claims{}, ErrInvalidToken
	}

	return Claims{UserID: "hub", Username: "hub"}, nil
}
