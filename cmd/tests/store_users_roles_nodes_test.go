package tests

import (
	"context"
	"strings"
	"testing"

	"scutum/cmd/internal/store"
)

// ── user store tests ──────────────────────────────────────────────────────────

func TestStoreListUsers(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	initial := len(users)

	if err := s.CreateUser(ctx, "u1", "alice", "hash1"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.CreateUser(ctx, "u2", "bob", "hash2"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	users, err = s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers after inserts failed: %v", err)
	}
	if len(users) != initial+2 {
		t.Errorf("expected %d users, got %d", initial+2, len(users))
	}
}

func TestStoreGetUser(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateUser(ctx, "u1", "alice", "hash1"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	u, err := s.GetUser(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("expected alice, got %q", u.Username)
	}
}

func TestStoreGetUserNotFound(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	_, err := s.GetUser(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for missing user, got nil")
	}
}

func TestStoreUpdateUserUsername(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateUser(ctx, "u1", "alice", "hash1"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := s.UpdateUserUsername(ctx, "u1", "alice-updated"); err != nil {
		t.Fatalf("UpdateUserUsername: %v", err)
	}

	u, _ := s.GetUser(ctx, "u1")
	if u.Username != "alice-updated" {
		t.Errorf("expected alice-updated, got %q", u.Username)
	}
}

func TestStoreUpdateUserUsernameNotFound(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	err := s.UpdateUserUsername(ctx, "missing", "x")
	if err == nil {
		t.Error("expected error for missing user")
	}
}

func TestStoreUpdateUserPassword(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateUser(ctx, "u1", "alice", "hash1"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := s.UpdateUserPassword(ctx, "u1", "new-hash"); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}

	u, _ := s.GetUser(ctx, "u1")
	if u.PasswordHash != "new-hash" {
		t.Errorf("expected new-hash, got %q", u.PasswordHash)
	}
}

func TestStoreDeleteUser(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateUser(ctx, "u1", "alice", "hash1"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := s.DeleteUser(ctx, "u1"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	_, err := s.GetUser(ctx, "u1")
	if err == nil {
		t.Error("expected user to be gone")
	}
}

func TestStoreDeleteUserNotFound(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	err := s.DeleteUser(ctx, "missing")
	if err == nil {
		t.Error("expected error for missing user")
	}
}

func TestStoreGetUserRoleNames(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	if err := s.CreateUser(ctx, "u1", "alice", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Before assigning any roles
	names, err := s.GetUserRoleNames(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUserRoleNames: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 roles, got %d", len(names))
	}

	// Assign the seeded admin role
	if err := s.AssignRole(ctx, "u1", "role_admin"); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}

	names, err = s.GetUserRoleNames(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUserRoleNames after assign: %v", err)
	}
	if len(names) == 0 {
		t.Error("expected at least 1 role name after assignment")
	}
}

func TestStoreSetUserRoles(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if err := s.CreateUser(ctx, "u1", "alice", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Assign roles by ID
	if err := s.SetUserRoles(ctx, "u1", []string{"role_admin"}); err != nil {
		t.Fatalf("SetUserRoles: %v", err)
	}

	names, _ := s.GetUserRoleNames(ctx, "u1")
	if len(names) == 0 {
		t.Error("expected role to be set")
	}

	// Clear roles
	if err := s.SetUserRoles(ctx, "u1", []string{}); err != nil {
		t.Fatalf("SetUserRoles (clear): %v", err)
	}

	names, _ = s.GetUserRoleNames(ctx, "u1")
	if len(names) != 0 {
		t.Errorf("expected 0 roles after clear, got %d", len(names))
	}
}

func TestStoreListAPIKeys(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateUser(ctx, "u1", "alice", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	keys, err := s.ListAPIKeys(ctx, "u1")
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}

	if err := s.CreateAPIKey(ctx, "k1", "u1", "ci-key", "hash-k1", nil); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	keys, err = s.ListAPIKeys(ctx, "u1")
	if err != nil {
		t.Fatalf("ListAPIKeys after insert: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}
	if keys[0].Name != "ci-key" {
		t.Errorf("expected ci-key, got %q", keys[0].Name)
	}
}

func TestStoreDeleteAPIKey(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateUser(ctx, "u1", "alice", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.CreateAPIKey(ctx, "k1", "u1", "ci-key", "hash-k1", nil); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	if err := s.DeleteAPIKey(ctx, "k1", "u1"); err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}

	keys, _ := s.ListAPIKeys(ctx, "u1")
	if len(keys) != 0 {
		t.Error("expected key to be deleted")
	}
}

func TestStoreDeleteAPIKeyNotFound(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateUser(ctx, "u1", "alice", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	err := s.DeleteAPIKey(ctx, "missing", "u1")
	if err == nil {
		t.Error("expected error for missing key")
	}
}

// ── role store tests ──────────────────────────────────────────────────────────

func TestStoreListRoles(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateRole(ctx, "r1", "developer", "can deploy"); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	roles, err := s.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}

	var found bool
	for _, r := range roles {
		if r.ID == "r1" && r.Name == "developer" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find created role in ListRoles")
	}
}

func TestStoreCreateRole(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateRole(ctx, "r1", "viewer", "read only"); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	roles, _ := s.ListRoles(ctx)
	var found bool
	for _, r := range roles {
		if r.ID == "r1" {
			found = true
			if r.Description != "read only" {
				t.Errorf("expected description 'read only', got %q", r.Description)
			}
		}
	}
	if !found {
		t.Error("created role not found")
	}
}

func TestStoreUpdateRole(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateRole(ctx, "r1", "viewer", "read only"); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	if err := s.UpdateRole(ctx, "r1", "reader", "updated desc"); err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}

	roles, _ := s.ListRoles(ctx)
	for _, r := range roles {
		if r.ID == "r1" {
			if r.Name != "reader" {
				t.Errorf("expected reader, got %q", r.Name)
			}
			if r.Description != "updated desc" {
				t.Errorf("expected 'updated desc', got %q", r.Description)
			}
		}
	}
}

func TestStoreUpdateRoleNotFound(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	err := s.UpdateRole(ctx, "missing", "x", "y")
	if err == nil {
		t.Error("expected error for missing role")
	}
}

func TestStoreDeleteRole(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateRole(ctx, "r1", "viewer", ""); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	if err := s.DeleteRole(ctx, "r1"); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}

	roles, _ := s.ListRoles(ctx)
	for _, r := range roles {
		if r.ID == "r1" {
			t.Error("role should have been deleted")
		}
	}
}

func TestStoreDeleteRoleNotFound(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	err := s.DeleteRole(ctx, "missing")
	if err == nil {
		t.Error("expected error for missing role")
	}
}

func TestStoreSetRolePerms(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateRole(ctx, "r1", "developer", ""); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	perms := []string{"nodes:read", "docker:write"}
	if err := s.SetRolePerms(ctx, "r1", perms); err != nil {
		t.Fatalf("SetRolePerms: %v", err)
	}

	got, err := s.ListRolePerms(ctx, "r1")
	if err != nil {
		t.Fatalf("ListRolePerms: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 perms, got %d: %v", len(got), got)
	}
}

func TestStoreSetRolePermsReplace(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateRole(ctx, "r1", "developer", ""); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	if err := s.SetRolePerms(ctx, "r1", []string{"nodes:read", "docker:write"}); err != nil {
		t.Fatalf("SetRolePerms initial: %v", err)
	}

	// Replace with a different set
	if err := s.SetRolePerms(ctx, "r1", []string{"git:write"}); err != nil {
		t.Fatalf("SetRolePerms replace: %v", err)
	}

	got, _ := s.ListRolePerms(ctx, "r1")
	if len(got) != 1 {
		t.Errorf("expected 1 perm after replace, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "git") {
		t.Errorf("expected git:write, got %v", got)
	}
}

func TestStoreListRolePerms(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateRole(ctx, "r1", "developer", ""); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	// Empty before any perms set
	perms, err := s.ListRolePerms(ctx, "r1")
	if err != nil {
		t.Fatalf("ListRolePerms: %v", err)
	}
	if len(perms) != 0 {
		t.Errorf("expected 0 perms, got %d", len(perms))
	}
}

// ── node store tests ──────────────────────────────────────────────────────────

func TestStoreCreateNode(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	n := store.NodeRecord{
		ID:        "n1",
		Name:      "core-01",
		Type:      "hub",
		Address:   "10.0.0.1:51820",
		PublicKey: "abc=",
	}
	if err := s.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	got, err := s.GetNode(ctx, "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Name != "core-01" || got.Type != "hub" {
		t.Errorf("node data mismatch: %+v", got)
	}
}

func TestStoreGetNodeNotFound(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	_, err := s.GetNode(ctx, "missing")
	if err == nil {
		t.Error("expected error for missing node")
	}
}

func TestStoreDeleteNode(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	n := store.NodeRecord{ID: "n1", Name: "core-01", Type: "hub", Address: "10.0.0.1:51820", PublicKey: "abc="}
	if err := s.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if err := s.DeleteNode(ctx, "n1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	_, err := s.GetNode(ctx, "n1")
	if err == nil {
		t.Error("expected node to be gone after delete")
	}
}

func TestStoreDeleteNodeNotFound(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	err := s.DeleteNode(ctx, "missing")
	if err == nil {
		t.Error("expected error for missing node")
	}
}

func TestStoreListNodes(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	initial, err := s.ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}

	nodes := []store.NodeRecord{
		{ID: "n1", Name: "core-01", Type: "hub", Address: "10.0.0.1:51820", PublicKey: "abc="},
		{ID: "n2", Name: "edge-01", Type: "combined", Address: "10.0.0.2:51820", PublicKey: "def="},
	}
	for _, n := range nodes {
		if err := s.CreateNode(ctx, n); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
	}

	list, err := s.ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes after inserts: %v", err)
	}
	if len(list) != len(initial)+2 {
		t.Errorf("expected %d nodes, got %d", len(initial)+2, len(list))
	}
}

func TestStoreTOTP(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	if err := s.CreateUser(ctx, "u1", "alice", "hash1"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Initial state
	secret, enabled, err := s.GetUserTOTP(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUserTOTP: %v", err)
	}
	if secret != "" || enabled {
		t.Errorf("expected empty/disabled, got %q/%v", secret, enabled)
	}

	// Set secret
	if err := s.SetUserTOTPSecret(ctx, "u1", "JBSWY3DPEHPK3PXP"); err != nil {
		t.Fatalf("SetUserTOTPSecret: %v", err)
	}

	secret, enabled, _ = s.GetUserTOTP(ctx, "u1")
	if secret != "JBSWY3DPEHPK3PXP" || enabled {
		t.Errorf("expected secret, but still disabled. got %q/%v", secret, enabled)
	}

	// Enable
	if err := s.SetUserTOTPEnabled(ctx, "u1", true); err != nil {
		t.Fatalf("SetUserTOTPEnabled: %v", err)
	}

	secret, enabled, _ = s.GetUserTOTP(ctx, "u1")
	if !enabled {
		t.Error("expected TOTP to be enabled")
	}
}

