package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"scutum/cmd/internal/store"
)

type scimStore interface {
	SCIMListUsers(ctx context.Context) ([]store.SCIMUser, error)
	SCIMGetUser(ctx context.Context, id string) (store.SCIMUser, error)
	SCIMCreateUser(ctx context.Context, id, username, email, passwordHash string) error
	SCIMUpdateUser(ctx context.Context, id, username, email string, active bool) error
	SCIMDeleteUser(ctx context.Context, id string) error
	CreateSCIMToken(ctx context.Context, id, tokenHash, description string) error
	ValidateSCIMToken(ctx context.Context, rawToken string) (bool, error)
	ListSCIMTokens(ctx context.Context) ([]store.SCIMTokenInfo, error)
	DeleteSCIMToken(ctx context.Context, id string) error
}

type SCIMHandler struct {
	store scimStore
}

func NewSCIMHandler(s scimStore) *SCIMHandler {
	return &SCIMHandler{store: s}
}

var scimUserSchemas = []string{"urn:ietf:params:scim:schemas:core:2.0:User"}
var scimListSchema = []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"}
var scimErrorSchema = []string{"urn:ietf:params:scim:api:messages:2.0:Error"}

func (h *SCIMHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			h.scimError(w, http.StatusUnauthorized, "missing token")
			return
		}
		ok, err := h.store.ValidateSCIMToken(r.Context(), token)
		if err != nil || !ok {
			h.scimError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *SCIMHandler) HandleServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/scim+json")
	json.NewEncoder(w).Encode(map[string]any{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"patch":   map[string]any{"supported": true},
		"bulk":    map[string]any{"supported": false},
		"filter":  map[string]any{"supported": true, "maxResults": 200},
		"changePassword": map[string]any{"supported": false},
		"sort":    map[string]any{"supported": false},
		"etag":    map[string]any{"supported": false},
		"authenticationSchemes": []map[string]any{{
			"type": "oauthbearertoken", "name": "OAuth Bearer Token", "primary": true,
		}},
	})
}

func (h *SCIMHandler) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.SCIMListUsers(r.Context())
	if err != nil {
		h.scimError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	// Simple filter: userName eq "value"
	if f := r.URL.Query().Get("filter"); f != "" {
		lower := strings.ToLower(f)
		if strings.HasPrefix(lower, "username eq ") {
			val := strings.Trim(f[len("username eq "):], `"'`)
			var filtered []store.SCIMUser
			for _, u := range users {
				if strings.EqualFold(u.Username, val) {
					filtered = append(filtered, u)
				}
			}
			users = filtered
		}
	}

	startIndex, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
	if startIndex < 1 {
		startIndex = 1
	}
	count, _ := strconv.Atoi(r.URL.Query().Get("count"))
	if count < 1 || count > 200 {
		count = 200
	}

	total := len(users)
	start := startIndex - 1
	if start >= total {
		users = nil
	} else {
		end := start + count
		if end > total {
			end = total
		}
		users = users[start:end]
	}

	resources := make([]any, len(users))
	for i, u := range users {
		resources[i] = h.toSCIMUser(u)
	}

	w.Header().Set("Content-Type", "application/scim+json")
	json.NewEncoder(w).Encode(map[string]any{
		"schemas":      scimListSchema,
		"totalResults": total,
		"startIndex":   startIndex,
		"itemsPerPage": len(resources),
		"Resources":    resources,
	})
}

func (h *SCIMHandler) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	u, err := h.store.SCIMGetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		h.scimError(w, http.StatusNotFound, "user not found")
		return
	}
	w.Header().Set("Content-Type", "application/scim+json")
	json.NewEncoder(w).Encode(h.toSCIMUser(u))
}

func (h *SCIMHandler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.scimError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	username, _ := body["userName"].(string)
	if username == "" {
		h.scimError(w, http.StatusBadRequest, "userName is required")
		return
	}
	email := extractSCIMEmail(body)
	id := uuid.New().String()
	// Random password — SSO-only users will authenticate via SSO
	rawPw := make([]byte, 18)
	rand.Read(rawPw)
	hash := sha256.Sum256(rawPw)
	pwHash := base64.StdEncoding.EncodeToString(hash[:])

	if err := h.store.SCIMCreateUser(r.Context(), id, username, email, pwHash); err != nil {
		h.scimError(w, http.StatusConflict, "user already exists or invalid data")
		return
	}
	u, _ := h.store.SCIMGetUser(r.Context(), id)
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(h.toSCIMUser(u))
}

func (h *SCIMHandler) HandleReplaceUser(w http.ResponseWriter, r *http.Request) {
	if _, err := h.store.SCIMGetUser(r.Context(), r.PathValue("id")); err != nil {
		h.scimError(w, http.StatusNotFound, "user not found")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.scimError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	username, _ := body["userName"].(string)
	email := extractSCIMEmail(body)
	active := true
	if v, ok := body["active"].(bool); ok {
		active = v
	}
	if err := h.store.SCIMUpdateUser(r.Context(), r.PathValue("id"), username, email, active); err != nil {
		h.scimError(w, http.StatusInternalServerError, "update failed")
		return
	}
	u, _ := h.store.SCIMGetUser(r.Context(), r.PathValue("id"))
	w.Header().Set("Content-Type", "application/scim+json")
	json.NewEncoder(w).Encode(h.toSCIMUser(u))
}

func (h *SCIMHandler) HandlePatchUser(w http.ResponseWriter, r *http.Request) {
	u, err := h.store.SCIMGetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		h.scimError(w, http.StatusNotFound, "user not found")
		return
	}
	var body struct {
		Operations []struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		} `json:"Operations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.scimError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	active := u.Active
	username := u.Username
	email := u.Email
	for _, op := range body.Operations {
		if strings.ToLower(op.Op) != "replace" {
			continue
		}
		switch strings.ToLower(op.Path) {
		case "active":
			if v, ok := op.Value.(bool); ok {
				active = v
			}
		case "username":
			if v, ok := op.Value.(string); ok {
				username = v
			}
		}
	}
	if err := h.store.SCIMUpdateUser(r.Context(), u.ID, username, email, active); err != nil {
		h.scimError(w, http.StatusInternalServerError, "update failed")
		return
	}
	updated, _ := h.store.SCIMGetUser(r.Context(), u.ID)
	w.Header().Set("Content-Type", "application/scim+json")
	json.NewEncoder(w).Encode(h.toSCIMUser(updated))
}

func (h *SCIMHandler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if _, err := h.store.SCIMGetUser(r.Context(), r.PathValue("id")); err != nil {
		h.scimError(w, http.StatusNotFound, "user not found")
		return
	}
	if err := h.store.SCIMDeleteUser(r.Context(), r.PathValue("id")); err != nil {
		h.scimError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── SCIM Token Management (admin JWT auth) ──────────────────────────────────

func (h *SCIMHandler) HandleListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.store.ListSCIMTokens(r.Context())
	if err != nil {
		http.Error(w, "failed to list tokens", http.StatusInternalServerError)
		return
	}
	if tokens == nil {
		tokens = []store.SCIMTokenInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

func (h *SCIMHandler) HandleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	raw := make([]byte, 32)
	rand.Read(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	id := uuid.New().String()

	if err := h.store.CreateSCIMToken(r.Context(), id, hash, req.Description); err != nil {
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "token": token})
}

func (h *SCIMHandler) HandleDeleteToken(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteSCIMToken(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, "token not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (h *SCIMHandler) toSCIMUser(u store.SCIMUser) map[string]any {
	return map[string]any{
		"schemas":  scimUserSchemas,
		"id":       u.ID,
		"userName": u.Username,
		"emails": []map[string]any{
			{"value": u.Email, "primary": true},
		},
		"active": u.Active,
		"meta": map[string]any{
			"resourceType": "User",
			"created":      u.CreatedAt.Format(time.RFC3339),
			"lastModified": u.CreatedAt.Format(time.RFC3339),
			"location":     "/scim/v2/Users/" + u.ID,
		},
	}
}

func (h *SCIMHandler) scimError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"schemas": scimErrorSchema,
		"status":  strconv.Itoa(status),
		"detail":  detail,
	})
}

func extractSCIMEmail(body map[string]any) string {
	emails, _ := body["emails"].([]any)
	for _, e := range emails {
		if m, ok := e.(map[string]any); ok {
			if v, _ := m["value"].(string); v != "" {
				return v
			}
		}
	}
	return ""
}
