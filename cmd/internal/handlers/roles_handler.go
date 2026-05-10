package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"scutum/cmd/internal/store"

	"github.com/google/uuid"
)

type roleStore interface {
	ListRoles(ctx context.Context) ([]store.RoleRecord, error)
	CreateRole(ctx context.Context, id, name, description string) error
	UpdateRole(ctx context.Context, id, name, description string) error
	DeleteRole(ctx context.Context, id string) error
	SetRolePerms(ctx context.Context, roleID string, permNames []string) error
}

type RoleHandler struct {
	store roleStore
}

func NewRoleHandler(s roleStore) *RoleHandler {
	return &RoleHandler{store: s}
}

func (h *RoleHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	roles, err := h.store.ListRoles(r.Context())
	if err != nil {
		http.Error(w, "failed to list roles", http.StatusInternalServerError)
		return
	}
	if roles == nil {
		roles = []store.RoleRecord{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roles)
}

type roleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Perms       []string `json:"perms"` // e.g. ["nodes:read", "containers:*"]
}

func (h *RoleHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req roleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	if err := h.store.CreateRole(r.Context(), id, req.Name, req.Description); err != nil {
		http.Error(w, "role name already exists", http.StatusConflict)
		return
	}
	if len(req.Perms) > 0 {
		_ = h.store.SetRolePerms(r.Context(), id, expandWildcards(req.Perms))
	}

	audit("ROLE_CREATED", r, "role_id", id, "role_name", req.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "name": req.Name})
}

func (h *RoleHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req roleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name != "" {
		if err := h.store.UpdateRole(r.Context(), id, req.Name, req.Description); err != nil {
			http.Error(w, "role not found", http.StatusNotFound)
			return
		}
	}
	if req.Perms != nil {
		_ = h.store.SetRolePerms(r.Context(), id, expandWildcards(req.Perms))
	}

	audit("ROLE_UPDATED", r, "role_id", id)

	w.WriteHeader(http.StatusNoContent)
}

func (h *RoleHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteRole(r.Context(), id); err != nil {
		http.Error(w, "role not found", http.StatusNotFound)
		return
	}
	audit("ROLE_DELETED", r, "role_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// expandWildcards converts frontend perm strings like "nodes:*"
// into the canonical "nodes:admin" name used in the permissions table.
func expandWildcards(perms []string) []string {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		for i, c := range p {
			if c == ':' && p[i+1:] == "*" {
				p = p[:i] + ":admin"
			}
		}
		out = append(out, p)
	}
	return out
}
