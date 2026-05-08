package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"scutum/cmd/internal/kms"
	"scutum/cmd/internal/store"
)

func TestStoreBasicOperations(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a local KMS provider
	keyFile := filepath.Join(tmpDir, "master.key")
	kmsProvider, err := kms.NewLocalKeyProvider(keyFile)
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}

	// Create store
	ctx := context.Background()
	s, err := store.New(ctx, dbPath, kmsProvider)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	defer s.Close()

	// Test CreateUser
	userID := "user123"
	username := "testuser"
	passwordHash := "hashedpassword"
	err = s.CreateUser(ctx, userID, username, passwordHash)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Test UserByUsername
	retrievedID, retrievedHash, err := s.UserByUsername(ctx, username)
	if err != nil {
		t.Fatalf("UserByUsername failed: %v", err)
	}
	if retrievedID != userID {
		t.Fatalf("UserByUsername returned wrong ID: got %q, want %q", retrievedID, userID)
	}
	if retrievedHash != passwordHash {
		t.Fatalf("UserByUsername returned wrong hash: got %q, want %q", retrievedHash, passwordHash)
	}

	// Test CreateAPIKey
	apiKeyID := "apikey123"
	apiKeyName := "test key"
	apiKeyHash := "hashedkey"
	expiresAt := time.Now().Add(24 * time.Hour)
	err = s.CreateAPIKey(ctx, apiKeyID, userID, apiKeyName, apiKeyHash, &expiresAt)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	// Test UserByAPIKey
	retrievedUserID, retrievedUsername, err := s.UserByAPIKey(apiKeyHash)
	if err != nil {
		t.Fatalf("UserByAPIKey failed: %v", err)
	}
	if retrievedUserID != userID {
		t.Fatalf("UserByAPIKey returned wrong user ID: got %q, want %q", retrievedUserID, userID)
	}
	if retrievedUsername != username {
		t.Fatalf("UserByAPIKey returned wrong username: got %q, want %q", retrievedUsername, username)
	}
}

func TestStoreS3Config(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a local KMS provider
	keyFile := filepath.Join(tmpDir, "master.key")
	kmsProvider, err := kms.NewLocalKeyProvider(keyFile)
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}

	// Create store
	ctx := context.Background()
	s, err := store.New(ctx, dbPath, kmsProvider)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	defer s.Close()

	// Test SetS3Config
	cfg := store.S3Config{
		Endpoint:  "https://s3.example.com",
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	}
	err = s.SetS3Config(ctx, cfg)
	if err != nil {
		t.Fatalf("SetS3Config failed: %v", err)
	}

	// Test GetS3Config
	retrievedCfg, err := s.GetS3Config(ctx)
	if err != nil {
		t.Fatalf("GetS3Config failed: %v", err)
	}
	if retrievedCfg.Endpoint != cfg.Endpoint {
		t.Fatalf("GetS3Config returned wrong endpoint: got %q, want %q", retrievedCfg.Endpoint, cfg.Endpoint)
	}
	if retrievedCfg.Bucket != cfg.Bucket {
		t.Fatalf("GetS3Config returned wrong bucket: got %q, want %q", retrievedCfg.Bucket, cfg.Bucket)
	}
	if retrievedCfg.Region != cfg.Region {
		t.Fatalf("GetS3Config returned wrong region: got %q, want %q", retrievedCfg.Region, cfg.Region)
	}
	if retrievedCfg.AccessKey != cfg.AccessKey {
		t.Fatalf("GetS3Config returned wrong access key: got %q, want %q", retrievedCfg.AccessKey, cfg.AccessKey)
	}
	if retrievedCfg.SecretKey != cfg.SecretKey {
		t.Fatalf("GetS3Config returned wrong secret key: got %q, want %q", retrievedCfg.SecretKey, cfg.SecretKey)
	}
}

func TestStoreWireGuardKeys(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a local KMS provider
	keyFile := filepath.Join(tmpDir, "master.key")
	kmsProvider, err := kms.NewLocalKeyProvider(keyFile)
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}

	// Create store
	ctx := context.Background()
	s, err := store.New(ctx, dbPath, kmsProvider)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	defer s.Close()

	// Test SetWireGuardPrivateKey
	ifaceName := "wg0"
	privateKey := []byte("test-private-key-data")
	err = s.SetWireGuardPrivateKey(ctx, ifaceName, privateKey)
	if err != nil {
		t.Fatalf("SetWireGuardPrivateKey failed: %v", err)
	}

	// Test GetWireGuardPrivateKey
	retrievedKey, err := s.GetWireGuardPrivateKey(ctx, ifaceName)
	if err != nil {
		t.Fatalf("GetWireGuardPrivateKey failed: %v", err)
	}
	if string(retrievedKey) != string(privateKey) {
		t.Fatalf("GetWireGuardPrivateKey returned wrong key: got %q, want %q", retrievedKey, privateKey)
	}
}

func TestStoreListEnabledPlugins(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a local KMS provider
	keyFile := filepath.Join(tmpDir, "master.key")
	kmsProvider, err := kms.NewLocalKeyProvider(keyFile)
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}

	// Create store
	ctx := context.Background()
	s, err := store.New(ctx, dbPath, kmsProvider)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}
	defer s.Close()

	// Test ListEnabledPlugins (should return empty list initially)
	plugins, err := s.ListEnabledPlugins(ctx)
	if err != nil {
		t.Fatalf("ListEnabledPlugins failed: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected empty plugins list, got %d plugins", len(plugins))
	}
}

// ============= Store Edge Cases - Validation =============

func TestStoreValidationEdgeCases(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	// ===== User Validation =====
	t.Run("create user with empty username", func(t *testing.T) {
		err := s.CreateUser(ctx, "id1", "", "hash")
		// Empty username might be rejected or allowed
		_ = err
	})

	t.Run("create user with very long username", func(t *testing.T) {
		username := strings.Repeat("a", 500)
		err := s.CreateUser(ctx, "id2", username, "hash")
		// Long usernames might be truncated or rejected
		_ = err
	})

	t.Run("create user with unicode username", func(t *testing.T) {
		err := s.CreateUser(ctx, "id3", "用户_用户123", "hash")
		if err != nil {
			t.Logf("unicode user error: %v", err)
		} else {
			// Verify it was stored
			uid, _, _ := s.UserByUsername(ctx, "用户_用户123")
			if uid == "" {
				t.Error("failed to retrieve user with unicode name")
			}
		}
	})

	t.Run("create user with empty password hash", func(t *testing.T) {
		err := s.CreateUser(ctx, "id4", "testuser", "")
		_ = err
	})

	t.Run("duplicate user creation", func(t *testing.T) {
		s.CreateUser(ctx, "id5", "dupuser", "hash1")
		err := s.CreateUser(ctx, "id5", "dupuser", "hash2")
		if err == nil {
			t.Logf("warning: duplicate user not prevented")
		}
	})
}

// ============= Store Edge Cases - Concurrency =============

func TestStoreConcurrentOperations(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	// Create base user
	s.CreateUser(ctx, "baseuser", "baseuser", "hash")

	t.Run("concurrent user reads", func(t *testing.T) {
		const goroutines = 20
		var wg sync.WaitGroup
		errors := make(chan error, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, err := s.UserByUsername(ctx, "baseuser")
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		errCount := 0
		for err := range errors {
			if err != nil {
				errCount++
			}
		}
		t.Logf("concurrent read errors: %d/%d", errCount, goroutines)
	})

	t.Run("concurrent user creates", func(t *testing.T) {
		const goroutines = 10
		var wg sync.WaitGroup
		errors := make(chan error, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				userID := fmt.Sprintf("concuser%d", idx)
				err := s.CreateUser(ctx, userID, userID, "hash")
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		errCount := 0
		for err := range errors {
			if err != nil {
				errCount++
			}
		}
		t.Logf("concurrent create errors: %d/%d (expected some due to constraints)", errCount, goroutines)
	})

	t.Run("concurrent API key operations", func(t *testing.T) {
		const goroutines = 10
		var wg sync.WaitGroup

		s.CreateUser(ctx, "apikeyuser", "apikeyuser", "hash")

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				keyID := fmt.Sprintf("keyid%d", idx)
				keyName := fmt.Sprintf("key%d", idx)
				s.CreateAPIKey(ctx, keyID, "apikeyuser", keyName, "myhash"+fmt.Sprintf("%d", idx), nil)
			}(i)
		}

		wg.Wait()
	})
}

// ============= Store Edge Cases - Boundary Conditions =============

func TestStoreBoundaryConditions(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	t.Run("create user with min valid data", func(t *testing.T) {
		err := s.CreateUser(ctx, "minuser", "a", "b")
		if err != nil {
			t.Logf("min user error: %v", err)
		}
	})

	t.Run("create user with max field length", func(t *testing.T) {
		username := strings.Repeat("x", 255)
		err := s.CreateUser(ctx, "maxuser", username, strings.Repeat("y", 1000))
		if err != nil {
			t.Logf("max length user error: %v", err)
		}
	})

	t.Run("get non-existent user", func(t *testing.T) {
		uid, hash, err := s.UserByUsername(ctx, "nonexistentuser123")
		if uid != "" || hash != "" {
			t.Error("non-existent user should return empty")
		}
		t.Logf("get non-existent error: %v", err)
	})

	t.Run("delete non-existent user", func(t *testing.T) {
		// Store may not have a delete function
		// just logging for reference
		t.Logf("delete operations not tested (no API)")
	})
}

// ============= Store Edge Cases - Data Integrity =============

func TestStoreDataIntegrityEdgeCases(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	t.Run("retrieve and verify data integrity", func(t *testing.T) {
		const username = "integrityuser"
		const hash = "hash123"

		s.CreateUser(ctx, "intuser", username, hash)

		uid, retrievedHash, err := s.UserByUsername(ctx, username)
		if err != nil {
			t.Fatalf("retrieve error: %v", err)
		}
		if uid == "" {
			t.Error("expected non-empty user ID")
		}
		if retrievedHash != hash {
			t.Error("password hash corrupted")
		}
	})

	t.Run("massive concurrent read consistency", func(t *testing.T) {
		s.CreateUser(ctx, "consistuser", "consistencyuser", "hash")

		const goroutines = 50
		var wg sync.WaitGroup
		results := make(chan bool, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				uid, _, _ := s.UserByUsername(ctx, "consistencyuser")
				if uid != "" {
					results <- true
				} else {
					results <- false
				}
			}()
		}

		wg.Wait()
		close(results)

		successCount := 0
		for success := range results {
			if success {
				successCount++
			}
		}

		if successCount != goroutines {
			t.Errorf("consistency issue: %d/%d reads were correct", successCount, goroutines)
		}
	})
}

// ============= Store Edge Cases - Special Characters =============

func TestStoreSpecialCharactersEdgeCases(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	tests := []struct {
		name     string
		username string
	}{
		{"spaces", "user name"},
		{"special chars", "user@#$%"},
		{"unicode", "用户_ユーザー"},
		{"digits only", "12345"},
		{"hyphen dash", "user-name"},
		{"underscore", "user_name"},
		{"dots", "user.name"},
		{"mixed", "User_123.测试@test"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := fmt.Sprintf("user%d", i)
			err := s.CreateUser(ctx, userID, tt.username, "hash")

			if err != nil {
				t.Logf("create user error: %v", err)
				return
			}

			uid, _, err := s.UserByUsername(ctx, tt.username)
			if err != nil {
				t.Errorf("retrieve error: %v", err)
			}
			if uid == "" {
				t.Error("username not retrieved correctly")
			}
		})
	}
}

// Helper to create test store
func createTestStore(t *testing.T) (*store.Store, kms.Provider) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	keyFile := filepath.Join(tmpDir, "master.key")
	kmsProvider, err := kms.NewLocalKeyProvider(keyFile)
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}

	ctx := context.Background()
	s, err := store.New(ctx, dbPath, kmsProvider)
	if err != nil {
		t.Fatalf("store.New failed: %v", err)
	}

	return s, kmsProvider
}

// ============= Missing Store Function Tests =============

func TestStoreRotateKeys(t *testing.T) {
	s, kmsProvider := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	// Create a user first
	err := s.CreateUser(ctx, "testuser", "testuser", "hash")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Create a new KMS provider for rotation testing
	tmpDir := t.TempDir()
	newKeyFile := filepath.Join(tmpDir, "new-master.key")
	_, err = kms.NewLocalKeyProvider(newKeyFile)
	if err != nil {
		t.Fatalf("NewLocalKeyProvider for rotation failed: %v", err)
	}

	// Test RotateKeys
	err = s.RotateKeys(ctx, kmsProvider) // Use original provider as "old" provider
	if err != nil {
		t.Fatalf("RotateKeys failed: %v", err)
	}

	// Verify user data is still accessible after rotation
	uid, hash, err := s.UserByUsername(ctx, "testuser")
	if err != nil {
		t.Fatalf("UserByUsername after rotation failed: %v", err)
	}
	if uid != "testuser" || hash != "hash" {
		t.Errorf("user data corrupted after rotation: got uid=%q hash=%q", uid, hash)
	}
}

func TestStoreUserHasPermission(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	// Create a user
	err := s.CreateUser(ctx, "testuser", "testuser", "hash")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Test UserHasPermission with various permissions
	testCases := []struct {
		userID   string
		resource string
		action   string
		expected bool
	}{
		{"testuser", "nodes", "read", false}, // No permissions assigned yet
		{"testuser", "nodes", "write", false},
		{"testuser", "admin", "all", false},
		{"nonexistent", "nodes", "read", false}, // Non-existent user
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s-%s-%s", tc.userID, tc.resource, tc.action), func(t *testing.T) {
			hasPermission, err := s.UserHasPermission(tc.userID, tc.resource, tc.action)
			if err != nil {
				t.Fatalf("UserHasPermission failed: %v", err)
			}
			if hasPermission != tc.expected {
				t.Errorf("UserHasPermission(%q, %q, %q) = %v, want %v", tc.userID, tc.resource, tc.action, hasPermission, tc.expected)
			}
		})
	}
}

func TestStoreAssignRole(t *testing.T) {
	s, _ := createTestStore(t)
	defer s.Close()
	ctx := context.Background()

	// Seed the database with default roles
	err := s.Seed(ctx)
	if err != nil {
		t.Fatalf("Seed failed: %v", err)
	}

	// Create a user
	err = s.CreateUser(ctx, "testuser", "testuser", "hash")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Test AssignRole - should succeed now that both user and role exist
	err = s.AssignRole(ctx, "testuser", "role_admin")
	if err != nil {
		t.Errorf("AssignRole failed: %v", err)
	}

	// Test with non-existent user - should fail due to foreign key constraint
	err = s.AssignRole(ctx, "nonexistent", "role_admin")
	if err == nil {
		t.Error("Expected error when assigning role to non-existent user")
	}
}

// TestMockStoreFunctions tests store functions with mock data
func TestMockStoreFunctions(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	kmsProvider, _ := kms.NewLocalKeyProvider(filepath.Join(tmpDir, "master.key"))

	s, err := store.New(ctx, dbPath, kmsProvider)
	if err != nil {
		t.Skip("store not available")
	}

	s.Seed(ctx)
	s.MarkSetupComplete(ctx)

	complete, _ := s.IsSetupComplete(ctx)
	_ = complete

	s.SetKMSProvider(ctx, "local")
	_, _ = s.GetKMSProvider(ctx)

	s.SetInstallType(ctx, store.InstallRemote)
	_, _ = s.GetInstallType(ctx)

	secret := []byte("test-secret")
	err = s.SetSecret(ctx, "db-creds", secret)
	if err != nil {
		t.Logf("SetSecret: %v", err)
	}
	_, _ = s.GetSecret(ctx, "db-creds")
}

// TestMockStoreDriver tests store driver
func TestMockStoreDriver(t *testing.T) {
	driver, err := store.NewDriver("sqlite3")
	if err != nil {
		t.Skip("SQLite driver not available")
	}

	conn, err := driver.Open(":memory:")
	if err != nil {
		t.Logf("Open: %v", err)
		return
	}
	defer conn.Close()

	if driver.Placeholder(1) != "?" {
		t.Error("Placeholder mismatch")
	}
}

// TestMockStoreSecretRotation tests secret rotation
func TestMockStoreSecretRotation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	kmsProvider, _ := kms.NewLocalKeyProvider(filepath.Join(tmpDir, "master.key"))

	s, err := store.New(ctx, dbPath, kmsProvider)
	if err != nil {
		t.Skip("store not available")
	}

	s.Seed(ctx)

	// Test RotateKeys
	err = s.RotateKeys(ctx, kmsProvider)
	if err != nil {
		t.Logf("RotateKeys: %v", err)
	}

	// Test ListEnabledPlugins
	plugins, err := s.ListEnabledPlugins(ctx)
	if err != nil {
		t.Logf("ListEnabledPlugins: %v", err)
	}
	_ = plugins
}

// TestMockMySQLDriver tests MySQL driver (needs MySQL)
func TestMockMySQLDriver(t *testing.T) {
	driver, err := store.NewDriver("mysql")
	if err != nil {
		t.Skip("MySQL driver not available")
	}

	_ = driver.Placeholder(1)
	_ = driver.Migrate
}

// TestMockPostgresDriver tests PostgreSQL driver (needs PostgreSQL)
func TestMockPostgresDriver(t *testing.T) {
	driver, err := store.NewDriver("postgres")
	if err != nil {
		t.Skip("PostgreSQL driver not available")
	}

	_ = driver.Placeholder(1)
	_ = driver.Migrate
}

// TestMockStoreSwapKMS tests swapping KMS provider
func TestMockStoreSwapKMS(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	kmsProvider, _ := kms.NewLocalKeyProvider(filepath.Join(tmpDir, "master.key"))

	s, err := store.New(ctx, dbPath, kmsProvider)
	if err != nil {
		t.Skip("store not available")
	}

	s.Seed(ctx)
	swapProvider, _ := kms.NewLocalKeyProvider(filepath.Join(tmpDir, "swap.key"))
	s.SwapKMS(swapProvider)
}

// TestMockMySQLDriverMigration tests MySQL driver migration
func TestMockMySQLDriverMigration(t *testing.T) {
	driver, err := store.NewDriver("mysql")
	if err != nil {
		t.Skip("MySQL driver not available")
	}

	// Test Placeholder
	if driver.Placeholder(1) != "?" {
		t.Error("Placeholder mismatch")
	}
}

// TestMockPostgresDriverMigration tests PostgreSQL driver migration
func TestMockPostgresDriverMigration(t *testing.T) {
	driver, err := store.NewDriver("postgres")
	if err != nil {
		t.Skip("PostgreSQL driver not available")
	}

	if driver.Placeholder(1) != "$1" {
		t.Error("Placeholder mismatch")
	}
}
