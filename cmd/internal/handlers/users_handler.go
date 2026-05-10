package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"scutum/cmd/internal/auth"
	"scutum/cmd/internal/store"

	"github.com/google/uuid"
)

type userAdminStore interface {
	ListUsers(ctx context.Context) ([]store.UserRecord, error)
	GetUser(ctx context.Context, id string) (store.UserRecord, error)
	CreateUser(ctx context.Context, id, username, passwordHash string) error
	UpdateUserUsername(ctx context.Context, id, username string) error
	UpdateUserPassword(ctx context.Context, id, passwordHash string) error
	DeleteUser(ctx context.Context, id string) error
	GetUserRoleNames(ctx context.Context, userID string) ([]string, error)
	SetUserRoles(ctx context.Context, userID string, roleIDs []string) error
	ListRoles(ctx context.Context) ([]store.RoleRecord, error)
	ListAPIKeys(ctx context.Context, userID string) ([]store.APIKeyRecord, error)
	DeleteAPIKey(ctx context.Context, id, userID string) error
}

type UserHandler struct {
	store userAdminStore
}

func NewUserHandler(s userAdminStore) *UserHandler {
	return &UserHandler{store: s}
}

type userResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *UserHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		http.Error(w, "failed to list users", http.StatusInternalServerError)
		return
	}

	out := make([]userResponse, 0, len(users))
	for _, u := range users {
		roles, _ := h.store.GetUserRoleNames(r.Context(), u.ID)
		out = append(out, userResponse{
			ID: u.ID, Username: u.Username, Roles: roles, CreatedAt: u.CreatedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *UserHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	u, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	roles, _ := h.store.GetUserRoleNames(r.Context(), u.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userResponse{
		ID: u.ID, Username: u.Username, Roles: roles, CreatedAt: u.CreatedAt,
	})
}

type createUserRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Roles    []string `json:"roles"` // role names
}

func (h *UserHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	id := uuid.New().String()
	if err := h.store.CreateUser(r.Context(), id, req.Username, hash); err != nil {
		http.Error(w, "username already exists", http.StatusConflict)
		return
	}

	if len(req.Roles) > 0 {
		roleIDs := h.resolveRoleIDs(r.Context(), req.Roles)
		_ = h.store.SetUserRoles(r.Context(), id, roleIDs)
	}

	audit("USER_CREATED", r, "target_user_id", id, "target_username", req.Username)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "username": req.Username})
}

type updateUserRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"` // optional
	Roles    []string `json:"roles"`    // optional
}

func (h *UserHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Username != "" {
		if err := h.store.UpdateUserUsername(r.Context(), id, req.Username); err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
	}

	if req.Password != "" {
		if len(req.Password) < 8 {
			http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
			return
		}
		_ = h.store.UpdateUserPassword(r.Context(), id, hash)
	}

	if req.Roles != nil {
		roleIDs := h.resolveRoleIDs(r.Context(), req.Roles)
		_ = h.store.SetUserRoles(r.Context(), id, roleIDs)
	}

	audit("USER_UPDATED", r, "target_user_id", id)

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteUser(r.Context(), id); err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	audit("USER_DELETED", r, "target_user_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// HandleMe returns the currently authenticated user's profile.
func (h *UserHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	u, err := h.store.GetUser(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	roles, _ := h.store.GetUserRoleNames(r.Context(), u.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userResponse{
		ID: u.ID, Username: u.Username, Roles: roles, CreatedAt: u.CreatedAt,
	})
}

// HandleListTokens returns API keys for the authenticated user.
func (h *UserHandler) HandleListTokens(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	keys, err := h.store.ListAPIKeys(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "failed to list tokens", http.StatusInternalServerError)
		return
	}
	if keys == nil {
		keys = []store.APIKeyRecord{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

// HandleDeleteToken revokes an API key owned by the authenticated user.
func (h *UserHandler) HandleDeleteToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	keyID := r.PathValue("id")
	if err := h.store.DeleteAPIKey(r.Context(), keyID, claims.UserID); err != nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}
	audit("API_KEY_REVOKED", r, "key_id", keyID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) resolveRoleIDs(ctx context.Context, names []string) []string {
	roles, err := h.store.ListRoles(ctx)
	if err != nil {
		return nil
	}
	nameToID := make(map[string]string, len(roles))
	for _, r := range roles {
		nameToID[r.Name] = r.ID
	}
	var ids []string
	for _, n := range names {
		if id, ok := nameToID[n]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}
