package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scutum/cmd/internal/auth"
	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/store"
)

var errNotFound = errors.New("not found")

// ── fake user admin store ─────────────────────────────────────────────────────

type fakeUserAdminStore struct {
	users     []store.UserRecord
	roles     []store.RoleRecord
	apiKeys   []store.APIKeyRecord
	userRoles map[string][]string // userID -> role names
	createErr error
	deleteErr error
}

func newFakeUserStore() *fakeUserAdminStore {
	return &fakeUserAdminStore{userRoles: make(map[string][]string)}
}

func (f *fakeUserAdminStore) ListUsers(_ context.Context) ([]store.UserRecord, error) {
	return f.users, nil
}

func (f *fakeUserAdminStore) GetUser(_ context.Context, id string) (store.UserRecord, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return store.UserRecord{}, errNotFound
}

func (f *fakeUserAdminStore) CreateUser(_ context.Context, id, username, passwordHash string) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.users = append(f.users, store.UserRecord{ID: id, Username: username, PasswordHash: passwordHash})
	return nil
}

func (f *fakeUserAdminStore) UpdateUserUsername(_ context.Context, id, username string) error {
	for i, u := range f.users {
		if u.ID == id {
			f.users[i].Username = username
			return nil
		}
	}
	return errNotFound
}

func (f *fakeUserAdminStore) UpdateUserPassword(_ context.Context, id, hash string) error {
	for i, u := range f.users {
		if u.ID == id {
			f.users[i].PasswordHash = hash
			return nil
		}
	}
	return errNotFound
}

func (f *fakeUserAdminStore) DeleteUser(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for i, u := range f.users {
		if u.ID == id {
			f.users = append(f.users[:i], f.users[i+1:]...)
			return nil
		}
	}
	return errNotFound
}

func (f *fakeUserAdminStore) GetUserRoleNames(_ context.Context, userID string) ([]string, error) {
	return f.userRoles[userID], nil
}

func (f *fakeUserAdminStore) SetUserRoles(_ context.Context, userID string, roleIDs []string) error {
	f.userRoles[userID] = roleIDs
	return nil
}

func (f *fakeUserAdminStore) ListRoles(_ context.Context) ([]store.RoleRecord, error) {
	return f.roles, nil
}

func (f *fakeUserAdminStore) ListAPIKeys(_ context.Context, userID string) ([]store.APIKeyRecord, error) {
	return f.apiKeys, nil
}

func (f *fakeUserAdminStore) DeleteAPIKey(_ context.Context, id, userID string) error {
	for i, k := range f.apiKeys {
		if k.ID == id {
			f.apiKeys = append(f.apiKeys[:i], f.apiKeys[i+1:]...)
			return nil
		}
	}
	return errNotFound
}

// ── user handler tests ────────────────────────────────────────────────────────

func TestUserHandlerList(t *testing.T) {
	s := newFakeUserStore()
	s.users = []store.UserRecord{
		{ID: "u1", Username: "alice", CreatedAt: time.Now()},
		{ID: "u2", Username: "bob", CreatedAt: time.Now()},
	}
	h := handlers.NewUserHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 users, got %d", len(out))
	}
}

func TestUserHandlerCreate(t *testing.T) {
	s := newFakeUserStore()
	h := handlers.NewUserHandler(s)

	body, _ := json.Marshal(map[string]interface{}{
		"username": "charlie",
		"password": "password123",
		"roles":    []string{},
	})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if len(s.users) != 1 {
		t.Errorf("expected 1 user in store, got %d", len(s.users))
	}
}

func TestUserHandlerCreateShortPassword(t *testing.T) {
	h := handlers.NewUserHandler(newFakeUserStore())
	body, _ := json.Marshal(map[string]string{"username": "x", "password": "short"})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUserHandlerCreateDuplicate(t *testing.T) {
	s := newFakeUserStore()
	s.createErr = errors.New("UNIQUE constraint failed")
	h := handlers.NewUserHandler(s)

	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestUserHandlerUpdate(t *testing.T) {
	s := newFakeUserStore()
	s.users = []store.UserRecord{{ID: "u1", Username: "alice"}}
	h := handlers.NewUserHandler(s)

	body, _ := json.Marshal(map[string]string{"username": "alice-updated"})
	req := httptest.NewRequest(http.MethodPut, "/users/u1", bytes.NewReader(body))
	req.SetPathValue("id", "u1")
	w := httptest.NewRecorder()
	h.HandleUpdate(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if s.users[0].Username != "alice-updated" {
		t.Errorf("expected username updated, got %s", s.users[0].Username)
	}
}

func TestUserHandlerDelete(t *testing.T) {
	s := newFakeUserStore()
	s.users = []store.UserRecord{{ID: "u1", Username: "alice"}}
	h := handlers.NewUserHandler(s)

	req := httptest.NewRequest(http.MethodDelete, "/users/u1", nil)
	req.SetPathValue("id", "u1")
	w := httptest.NewRecorder()
	h.HandleDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if len(s.users) != 0 {
		t.Errorf("expected user to be removed")
	}
}

func TestUserHandlerDeleteNotFound(t *testing.T) {
	h := handlers.NewUserHandler(newFakeUserStore())
	req := httptest.NewRequest(http.MethodDelete, "/users/missing", nil)
	req.SetPathValue("id", "missing")
	w := httptest.NewRecorder()
	h.HandleDelete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUserHandlerMe(t *testing.T) {
	s := newFakeUserStore()
	s.users = []store.UserRecord{{ID: "u1", Username: "alice", CreatedAt: time.Now()}}
	h := handlers.NewUserHandler(s)

	token, _ := auth.IssueJWT("u1", "alice", []byte("test-secret"), time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Inject claims into context
	claims, _ := auth.ValidateJWT(token, []byte("test-secret"))
	ctx := auth.WithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.HandleMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out["username"] != "alice" {
		t.Errorf("expected username alice, got %v", out["username"])
	}
}

func TestUserHandlerListTokens(t *testing.T) {
	s := newFakeUserStore()
	s.users = []store.UserRecord{{ID: "u1", Username: "alice"}}
	exp := time.Now().Add(24 * time.Hour)
	s.apiKeys = []store.APIKeyRecord{
		{ID: "k1", Name: "ci-key", ExpiresAt: &exp, CreatedAt: time.Now()},
	}
	h := handlers.NewUserHandler(s)

	token, _ := auth.IssueJWT("u1", "alice", []byte("test-secret"), time.Hour)
	claims, _ := auth.ValidateJWT(token, []byte("test-secret"))
	req := httptest.NewRequest(http.MethodGet, "/auth/tokens", nil)
	req = req.WithContext(auth.WithClaims(req.Context(), claims))
	w := httptest.NewRecorder()
	h.HandleListTokens(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 key, got %d", len(out))
	}
}

// ── role handler tests ────────────────────────────────────────────────────────

type fakeRoleStore struct {
	roles     []store.RoleRecord
	createErr error
}

func (f *fakeRoleStore) ListRoles(_ context.Context) ([]store.RoleRecord, error) {
	return f.roles, nil
}

func (f *fakeRoleStore) CreateRole(_ context.Context, id, name, description string) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.roles = append(f.roles, store.RoleRecord{ID: id, Name: name, Description: description})
	return nil
}

func (f *fakeRoleStore) UpdateRole(_ context.Context, id, name, description string) error {
	for i, r := range f.roles {
		if r.ID == id {
			f.roles[i].Name = name
			f.roles[i].Description = description
			return nil
		}
	}
	return errNotFound
}

func (f *fakeRoleStore) DeleteRole(_ context.Context, id string) error {
	for i, r := range f.roles {
		if r.ID == id {
			f.roles = append(f.roles[:i], f.roles[i+1:]...)
			return nil
		}
	}
	return errNotFound
}

func (f *fakeRoleStore) SetRolePerms(_ context.Context, roleID string, permNames []string) error {
	return nil
}

func TestRoleHandlerList(t *testing.T) {
	s := &fakeRoleStore{roles: []store.RoleRecord{
		{ID: "r1", Name: "admin", Description: "Full access", Perms: []string{"nodes:admin"}},
	}}
	h := handlers.NewRoleHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/roles", nil)
	w := httptest.NewRecorder()
	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var out []store.RoleRecord
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 role, got %d", len(out))
	}
}

func TestRoleHandlerCreate(t *testing.T) {
	s := &fakeRoleStore{}
	h := handlers.NewRoleHandler(s)

	body, _ := json.Marshal(map[string]interface{}{
		"name":        "developer",
		"description": "Deploy and read",
		"perms":       []string{"containers:write", "nodes:read"},
	})
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if len(s.roles) != 1 {
		t.Errorf("expected 1 role in store, got %d", len(s.roles))
	}
}

func TestRoleHandlerCreateMissingName(t *testing.T) {
	h := handlers.NewRoleHandler(&fakeRoleStore{})
	body, _ := json.Marshal(map[string]string{"description": "no name"})
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRoleHandlerUpdate(t *testing.T) {
	s := &fakeRoleStore{roles: []store.RoleRecord{{ID: "r1", Name: "viewer"}}}
	h := handlers.NewRoleHandler(s)

	body, _ := json.Marshal(map[string]string{"name": "reader", "description": "updated"})
	req := httptest.NewRequest(http.MethodPut, "/roles/r1", bytes.NewReader(body))
	req.SetPathValue("id", "r1")
	w := httptest.NewRecorder()
	h.HandleUpdate(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if s.roles[0].Name != "reader" {
		t.Errorf("expected name updated, got %s", s.roles[0].Name)
	}
}

func TestRoleHandlerDelete(t *testing.T) {
	s := &fakeRoleStore{roles: []store.RoleRecord{{ID: "r1", Name: "viewer"}}}
	h := handlers.NewRoleHandler(s)

	req := httptest.NewRequest(http.MethodDelete, "/roles/r1", nil)
	req.SetPathValue("id", "r1")
	w := httptest.NewRecorder()
	h.HandleDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if len(s.roles) != 0 {
		t.Errorf("expected role to be removed")
	}
}

func TestRoleHandlerDeleteNotFound(t *testing.T) {
	h := handlers.NewRoleHandler(&fakeRoleStore{})
	req := httptest.NewRequest(http.MethodDelete, "/roles/missing", nil)
	req.SetPathValue("id", "missing")
	w := httptest.NewRecorder()
	h.HandleDelete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
