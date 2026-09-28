package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"scutum/cmd/internal/auth"
	"scutum/cmd/internal/store"

	"github.com/google/uuid"
)

// secretNameRe restricts names to a safe charset: it's used both as a
// net/http path segment (via {name...}) and inside the "secret://name"
// deploy-time reference syntax, so no whitespace and no characters that
// would be ambiguous in either context.
var secretNameRe = regexp.MustCompile(`^[A-Za-z0-9_.\-/]+$`)

// ValidateSecretName reports whether name is safe to use as a vault secret
// name: non-empty, safe charset, no path traversal, no leading/trailing/
// double slashes.
func ValidateSecretName(name string) bool {
	if name == "" || len(name) > 512 {
		return false
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(name, "//") {
		return false
	}
	return secretNameRe.MatchString(name)
}

type SecretsVaultHandler struct {
	store *store.Store
}

func NewSecretsVaultHandler(st *store.Store) *SecretsVaultHandler {
	return &SecretsVaultHandler{store: st}
}

func actorFromRequest(r *http.Request) string {
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		return claims.Username
	}
	return ""
}

// HandleList returns metadata only — GET /secrets. Values never appear here.
func (h *SecretsVaultHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	secrets, err := h.store.ListAppSecrets(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if secrets == nil {
		secrets = []store.AppSecret{}
	}
	writeJSON(w, http.StatusOK, secrets)
}

type createSecretReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Value       string `json:"value"`
}

// HandleCreate — POST /secrets. The response never echoes the value back.
func (h *SecretsVaultHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req createSecretReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !ValidateSecretName(req.Name) {
		http.Error(w, "name must be non-empty and may only contain letters, digits, '_', '-', '.', and '/' as a path separator", http.StatusBadRequest)
		return
	}
	if req.Value == "" {
		http.Error(w, "value is required", http.StatusBadRequest)
		return
	}

	secret, err := h.store.CreateAppSecret(r.Context(), uuid.New().String(), req.Name, req.Description, []byte(req.Value), actorFromRequest(r))
	if err != nil {
		code := http.StatusInternalServerError
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
			code = http.StatusConflict
			err = fmt.Errorf("secret %q already exists", req.Name)
		}
		http.Error(w, err.Error(), code)
		return
	}

	audit("SECRET_CREATED", r, "name", req.Name)
	writeJSON(w, http.StatusCreated, secret)
}

type updateSecretReq struct {
	Description *string `json:"description"`
	Value       *string `json:"value"`
}

// HandleUpdate — PUT /secrets/{name...}. Response never echoes the value back.
func (h *SecretsVaultHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req updateSecretReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Description == nil && req.Value == nil {
		http.Error(w, "nothing to update", http.StatusBadRequest)
		return
	}
	if req.Value != nil && *req.Value == "" {
		http.Error(w, "value cannot be set to empty", http.StatusBadRequest)
		return
	}

	var valueBytes []byte
	if req.Value != nil {
		valueBytes = []byte(*req.Value)
	}
	secret, err := h.store.UpdateAppSecret(r.Context(), name, req.Description, valueBytes, actorFromRequest(r))
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}

	audit("SECRET_UPDATED", r, "name", name)
	writeJSON(w, http.StatusOK, secret)
}

// HandleDelete — DELETE /secrets/{name...}.
func (h *SecretsVaultHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.store.DeleteAppSecret(r.Context(), name); err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}
	audit("SECRET_DELETED", r, "name", name)
	w.WriteHeader(http.StatusNoContent)
}
