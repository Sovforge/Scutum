package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// publicKeyB64 is injected at build time via -ldflags.
// Self-built binaries get an empty string → zeroed key → Verify always fails → no enterprise features.
var publicKeyB64 string

var publicKey [ed25519.PublicKeySize]byte

func init() {
	if publicKeyB64 == "" {
		return
	}
	b, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil || len(b) != ed25519.PublicKeySize {
		return
	}
	copy(publicKey[:], b)
}

var (
	ErrInvalid = errors.New("invalid license")
	ErrExpired = errors.New("license expired")
)

type Claims struct {
	Licensee  string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtPayload struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

// Verify parses and validates an EdDSA-signed license JWT against the build-injected public key.
func Verify(token string) (Claims, error) {
	if publicKey == [ed25519.PublicKeySize]byte{} {
		return Claims{}, ErrInvalid // no key injected at build time
	}

	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalid
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return Claims{}, ErrInvalid
	}

	msg := []byte(parts[0] + "." + parts[1])
	if !ed25519.Verify(publicKey[:], msg, sigBytes) {
		return Claims{}, ErrInvalid
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalid
	}

	var payload jwtPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return Claims{}, ErrInvalid
	}

	if time.Now().Unix() > payload.Exp {
		return Claims{}, ErrExpired
	}

	return Claims{
		Licensee:  payload.Sub,
		IssuedAt:  time.Unix(payload.Iat, 0).UTC(),
		ExpiresAt: time.Unix(payload.Exp, 0).UTC(),
	}, nil
}

// Require returns a middleware that gates an HTTP handler behind a valid enterprise license.
// Unauthenticated or expired licenses receive a 402 with a JSON error body.
func Require(w *Watcher, feature string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if !isActive(w) {
				rw.Header().Set("Content-Type", "application/json")
				rw.WriteHeader(http.StatusPaymentRequired)
				json.NewEncoder(rw).Encode(map[string]string{ //nolint:errcheck
					"error":   "enterprise license required",
					"feature": feature,
				})
				return
			}
			next.ServeHTTP(rw, r)
		})
	}
}

// IsLicensed reports whether a valid, non-expired license is currently active.
func IsLicensed(w *Watcher) bool {
	return isActive(w)
}

func isActive(w *Watcher) bool {
	if w == nil {
		return false
	}
	c := w.Active()
	return c != nil && time.Now().Before(c.ExpiresAt)
}

func jsonBase64(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal jwt part: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
