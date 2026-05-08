package tests

import (
	"context"
	"path/filepath"
	"testing"

	"scutum/cmd/internal/kms"
	"scutum/cmd/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	provider, err := kms.NewLocalKeyProvider(filepath.Join(dir, "master.key"))
	if err != nil {
		t.Fatalf("NewLocalKeyProvider: %v", err)
	}
	s, err := store.New(context.Background(), filepath.Join(dir, "db.sqlite"), provider)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- IsSetupComplete / MarkSetupComplete ---

func TestIsSetupCompleteInitiallyFalse(t *testing.T) {
	s := newTestStore(t)
	complete, err := s.IsSetupComplete(context.Background())
	if err != nil {
		t.Fatalf("IsSetupComplete: %v", err)
	}
	if complete {
		t.Error("expected setup to be incomplete initially")
	}
}

func TestMarkSetupCompleteAndCheck(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.MarkSetupComplete(ctx); err != nil {
		t.Fatalf("MarkSetupComplete: %v", err)
	}
	complete, err := s.IsSetupComplete(ctx)
	if err != nil {
		t.Fatalf("IsSetupComplete after mark: %v", err)
	}
	if !complete {
		t.Error("expected setup to be complete after MarkSetupComplete")
	}
}

func TestMarkSetupCompleteIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := s.MarkSetupComplete(ctx); err != nil {
			t.Fatalf("MarkSetupComplete call %d: %v", i+1, err)
		}
	}
	complete, err := s.IsSetupComplete(ctx)
	if err != nil {
		t.Fatalf("IsSetupComplete: %v", err)
	}
	if !complete {
		t.Error("expected complete after repeated MarkSetupComplete")
	}
}

// --- SetKMSProvider / GetKMSProvider ---

func TestSetAndGetKMSProvider(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.SetKMSProvider(ctx, "vault"); err != nil {
		t.Fatalf("SetKMSProvider: %v", err)
	}
	got, err := s.GetKMSProvider(ctx)
	if err != nil {
		t.Fatalf("GetKMSProvider: %v", err)
	}
	if got != "vault" {
		t.Errorf("expected vault, got %q", got)
	}
}

func TestGetKMSProviderNotSet(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetKMSProvider(context.Background())
	if err == nil {
		t.Error("expected error when KMS provider not set")
	}
}

func TestSetKMSProviderOverwrite(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, p := range []string{"local", "aws", "gcp"} {
		if err := s.SetKMSProvider(ctx, p); err != nil {
			t.Fatalf("SetKMSProvider(%q): %v", p, err)
		}
	}
	got, err := s.GetKMSProvider(ctx)
	if err != nil {
		t.Fatalf("GetKMSProvider: %v", err)
	}
	if got != "gcp" {
		t.Errorf("expected gcp (last write wins), got %q", got)
	}
}

// --- SetInstallType / GetInstallType ---

func TestSetAndGetInstallType(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, it := range []store.InstallType{store.InstallHub, store.InstallRemote, store.InstallCombined} {
		if err := s.SetInstallType(ctx, it); err != nil {
			t.Fatalf("SetInstallType(%q): %v", it, err)
		}
		got, err := s.GetInstallType(ctx)
		if err != nil {
			t.Fatalf("GetInstallType: %v", err)
		}
		if got != it {
			t.Errorf("expected %q, got %q", it, got)
		}
	}
}

func TestGetInstallTypeNotSet(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetInstallType(context.Background())
	if err == nil {
		t.Error("expected error when install type not set")
	}
}

// --- Seed ---

func TestSeedIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := s.Seed(ctx); err != nil {
			t.Fatalf("Seed call %d: %v", i+1, err)
		}
	}
}

func TestSeedCreatesAdminRole(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// Create a user, assign admin role, then verify they have a docker:read permission.
	if err := s.CreateUser(ctx, "u1", "admin_user", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.AssignRole(ctx, "u1", "role_admin"); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}
	ok, err := s.UserHasPermission("u1", "docker", "read")
	if err != nil {
		t.Fatalf("UserHasPermission: %v", err)
	}
	if !ok {
		t.Error("admin user should have docker:read permission after Seed")
	}
}

func TestSeedViewerRoleRestrictedAccess(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	if err := s.CreateUser(ctx, "u2", "viewer_user", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.AssignRole(ctx, "u2", "role_viewer"); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}

	// Viewer should NOT have delete access.
	ok, err := s.UserHasPermission("u2", "docker", "delete")
	if err != nil {
		t.Fatalf("UserHasPermission: %v", err)
	}
	if ok {
		t.Error("viewer should not have docker:delete permission")
	}

	// Viewer SHOULD have read access.
	ok, err = s.UserHasPermission("u2", "docker", "read")
	if err != nil {
		t.Fatalf("UserHasPermission read: %v", err)
	}
	if !ok {
		t.Error("viewer should have docker:read permission")
	}
}

func TestSeedUserWithNoRoleHasNoPermissions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.Seed(ctx); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if err := s.CreateUser(ctx, "u3", "norole_user", "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	ok, err := s.UserHasPermission("u3", "docker", "read")
	if err != nil {
		t.Fatalf("UserHasPermission: %v", err)
	}
	if ok {
		t.Error("user with no role should have no permissions")
	}
}

// --- RotateKeys ---

func TestRotateKeysNoSecrets(t *testing.T) {
	dir := t.TempDir()
	provider, err := kms.NewLocalKeyProvider(filepath.Join(dir, "master.key"))
	if err != nil {
		t.Fatalf("NewLocalKeyProvider: %v", err)
	}
	s, err := store.New(context.Background(), filepath.Join(dir, "db.sqlite"), provider)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer s.Close()

	// Rotating with no secrets should be a no-op.
	if err := s.RotateKeys(context.Background(), provider); err != nil {
		t.Fatalf("RotateKeys on empty store: %v", err)
	}
}

func TestRotateKeysPreservesSecretValues(t *testing.T) {
	dir := t.TempDir()
	oldProvider, err := kms.NewLocalKeyProvider(filepath.Join(dir, "old.key"))
	if err != nil {
		t.Fatalf("old provider: %v", err)
	}
	s, err := store.New(context.Background(), filepath.Join(dir, "db.sqlite"), oldProvider)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	want := []byte("super-secret-value")
	if err := s.SetSecret(ctx, "my-secret", want); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}

	// Swap to a new KMS provider and rotate.
	newProvider, err := kms.NewLocalKeyProvider(filepath.Join(dir, "new.key"))
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	s.SwapKMS(newProvider)

	if err := s.RotateKeys(ctx, oldProvider); err != nil {
		t.Fatalf("RotateKeys: %v", err)
	}

	// Secret should still be retrievable under the new provider.
	got, err := s.GetSecret(ctx, "my-secret")
	if err != nil {
		t.Fatalf("GetSecret after rotate: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("secret value changed: got %q, want %q", got, want)
	}
}
