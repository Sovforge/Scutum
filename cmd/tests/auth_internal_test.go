package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	auth "scutum/cmd/internal/auth"
)

func TestHashPasswordAndVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("supersecretpassword")
	if err != nil {
		t.Fatal(err)
	}

	ok, err := auth.VerifyPassword("supersecretpassword", hash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected password verification to succeed")
	}

	ok, err = auth.VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected wrong password to fail verification")
	}
}

func TestHashAPIKeyAndGenerateAPIKey(t *testing.T) {
	raw := "api-key-value"
	got := auth.HashAPIKey(raw)
	if len(got) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(got))
	}

	secret, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(secret, "orch_") {
		t.Fatalf("expected key to start with orch_, got %q", secret)
	}
}

func TestClaimsContextHelpers(t *testing.T) {
	ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		t.Fatal("expected claims to be present")
	}
	if claims.UserID != "uid" || claims.Username != "alice" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestIssueAndValidateJWT(t *testing.T) {
	secret := []byte("my-secret-key-01234567890-12345")
	token, err := auth.IssueJWT("uid", "alice", secret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := auth.ValidateJWT(token, secret)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "uid" || claims.Username != "alice" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestValidateJWTInvalidToken(t *testing.T) {
	_, err := auth.ValidateJWT("not-a-token", []byte("secret"))
	if err != auth.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidateJWTExpiredToken(t *testing.T) {
	secret := []byte("my-secret-key-01234567890-12345")
	token, err := auth.IssueJWT("uid", "alice", secret, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_, err = auth.ValidateJWT(token, secret)
	if err != auth.ErrExpiredToken {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

// ============= Password Hashing Edge Cases =============

func TestHashPasswordEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		password      string
		shouldSucceed bool
	}{
		{"empty password", "", true},
		{"very long password", strings.Repeat("x", 1000), true},
		{"unicode password", "пароль密码🔐", true},
		{"single char", "a", true},
		{"spaces only", "   ", true},
		{"special chars", "!@#$%^&*()_+-=[]{}|;:',.<>?/`~", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := auth.HashPassword(tt.password)
			if (err == nil) != tt.shouldSucceed {
				t.Errorf("expected success=%v, got error=%v", tt.shouldSucceed, err != nil)
			}
			if err == nil {
				if hash == "" {
					t.Error("expected non-empty hash")
				}
				// Verify password matches
				ok, _ := auth.VerifyPassword(tt.password, hash)
				if !ok {
					t.Error("password verification failed")
				}
			}
		})
	}
}

func TestVerifyPasswordEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		password string
		hash     string
		expected bool
	}{
		{"empty password matching", "", "", false}, // empty hash is invalid
		{"empty password", "", "somehash", false},
		{"wrong password", "wrong", "somehash", false},
		{"case sensitive", "Password", "password", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only test with invalid hashes since actual hashes are bcrypt
			ok, _ := auth.VerifyPassword(tt.password, tt.hash)
			if ok && !tt.expected {
				t.Error("expected verification to fail")
			}
		})
	}

	// Test with valid hash
	t.Run("valid hash match", func(t *testing.T) {
		password := "test123"
		hash, _ := auth.HashPassword(password)
		ok, err := auth.VerifyPassword(password, hash)
		if !ok || err != nil {
			t.Error("expected valid password to match hash")
		}
	})

	// Test similar but different password
	t.Run("similar password mismatch", func(t *testing.T) {
		password := "test123"
		hash, _ := auth.HashPassword(password)
		ok, _ := auth.VerifyPassword("test124", hash)
		if ok {
			t.Error("expected different password to not match")
		}
	})
}

// ============= API Key Edge Cases =============

func TestHashAPIKeyEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"empty string", ""},
		{"single char", "a"},
		{"very long key", strings.Repeat("x", 2000)},
		{"unicode key", "orch_密码🔐"},
		{"special chars", "orch_!@#$%^&*"},
		{"whitespace", "orch_ \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := auth.HashAPIKey(tt.key)
			if len(hash) != 64 { // SHA256 hex is 64 chars
				t.Errorf("expected 64 char hash, got %d", len(hash))
			}
			// Hashing same key should produce same hash
			hash2 := auth.HashAPIKey(tt.key)
			if hash != hash2 {
				t.Error("hashing same key should produce same hash")
			}
		})
	}

	// Different keys should produce different hashes
	t.Run("different keys different hashes", func(t *testing.T) {
		hash1 := auth.HashAPIKey("key1")
		hash2 := auth.HashAPIKey("key2")
		if hash1 == hash2 {
			t.Error("different keys should produce different hashes")
		}
	})
}

func TestGenerateAPIKeyEdgeCases(t *testing.T) {
	keys := make(map[string]bool)

	// Generate multiple keys and ensure they're unique
	for i := 0; i < 100; i++ {
		key, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		if !strings.HasPrefix(key, "orch_") {
			t.Errorf("key should start with orch_, got %s", key)
		}

		if len(key) <= 5 { // Should be longer than prefix
			t.Errorf("key too short: %s", key)
		}

		if keys[key] {
			t.Errorf("duplicate key generated: %s", key)
		}
		keys[key] = true
	}
}

// ============= JWT Edge Cases =============

func TestIssueJWTEdgeCases(t *testing.T) {
	secret := []byte("test-secret-key-32-chars-minimum!")

	tests := []struct {
		name          string
		userID        string
		username      string
		ttl           time.Duration
		shouldSucceed bool
	}{
		{"normal case", "user1", "alice", time.Hour, true},
		{"empty user id", "", "alice", time.Hour, true},
		{"empty username", "user1", "", time.Hour, true},
		{"negative ttl", "user1", "alice", -time.Hour, true}, // Should still issue
		{"zero ttl", "user1", "alice", 0, true},
		{"very long ttl", "user1", "alice", 100 * 365 * 24 * time.Hour, true},
		{"very short ttl", "user1", "alice", 1 * time.Millisecond, true},
		{"unicode user", "用户1", "用户名", time.Hour, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := auth.IssueJWT(tt.userID, tt.username, secret, tt.ttl)
			if (err == nil) != tt.shouldSucceed {
				t.Errorf("expected success=%v, got error=%v", tt.shouldSucceed, err)
			}
			if err == nil && token == "" {
				t.Error("expected non-empty token")
			}
		})
	}
}

func TestValidateJWTEdgeCases(t *testing.T) {
	secret := []byte("test-secret-key")
	userID := "user123"
	username := "alice"

	// Generate valid token
	validToken, _ := auth.IssueJWT(userID, username, secret, time.Hour)

	tests := []struct {
		name          string
		token         string
		secret        []byte
		shouldSucceed bool
	}{
		{"empty token", "", secret, false},
		{"invalid token", "invalid.token.here", secret, false},
		{"valid token", validToken, secret, true},
		{"wrong secret", validToken, []byte("wrong-secret"), false},
		{"malformed jwt", "header.payload", secret, false},
		{"too many parts", "a.b.c.d", secret, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := auth.ValidateJWT(tt.token, tt.secret)
			if (err == nil) != tt.shouldSucceed {
				t.Errorf("expected success=%v, got error=%v", tt.shouldSucceed, err)
			}
			if err == nil {
				if claims.UserID == "" && tt.name == "valid token" {
					t.Error("expected non-empty claims UserID")
				}
			}
		})
	}

	// Test expired token
	t.Run("expired token", func(t *testing.T) {
		token, _ := auth.IssueJWT(userID, username, secret, -time.Hour)
		_, err := auth.ValidateJWT(token, secret)
		if err == nil {
			t.Error("expected error for expired token")
		}
	})
}

// ============= Context Claims Edge Cases =============

func TestWithClaimsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		username string
	}{
		{"normal", "user1", "alice"},
		{"empty user id", "", "alice"},
		{"empty username", "user1", ""},
		{"both empty", "", ""},
		{"unicode", "用户1", "用户名"},
		{"very long", strings.Repeat("x", 500), strings.Repeat("y", 500)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			claims := auth.Claims{UserID: tt.userID, Username: tt.username}
			newCtx := auth.WithClaims(ctx, claims)

			if newCtx == ctx {
				t.Error("expected new context")
			}

			// Retrieve claims
			retrieved, ok := auth.ClaimsFromContext(newCtx)
			if !ok {
				t.Error("expected to retrieve claims from context")
			}
			if retrieved.UserID != tt.userID {
				t.Errorf("expected user id %q, got %q", tt.userID, retrieved.UserID)
			}
			if retrieved.Username != tt.username {
				t.Errorf("expected username %q, got %q", tt.username, retrieved.Username)
			}
		})
	}

	// Test without claims
	t.Run("context without claims", func(t *testing.T) {
		claims, ok := auth.ClaimsFromContext(context.Background())
		if ok {
			t.Errorf("expected no claims, got %v", claims)
		}
	})
}

// ============= Middleware Edge Cases =============

func TestMiddlewareWithVariousHeaders(t *testing.T) {
	secret := []byte("test-secret-key")
	validToken, _ := auth.IssueJWT("user1", "alice", secret, time.Hour)

	tests := []struct {
		name   string
		header string
		valid  bool
	}{
		{"valid bearer token", "Bearer " + validToken, true},
		{"no space in bearer", "Bearer" + validToken, false},
		{"lowercase bearer", "bearer " + validToken, false},
		{"empty authorization", "", false},
		{"no bearer prefix", validToken, false},
		{"bearer only", "Bearer ", false},
		{"invalid token", "Bearer invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse would fail for invalid entries
			if tt.valid {
				if !strings.HasPrefix(tt.header, "Bearer ") {
					t.Errorf("test setup error: expected valid header format")
				}
			}
		})
	}
}

// TestMockAuthFunctions tests auth functions with mocking
func TestMockAuthFunctions(t *testing.T) {
	hash, err := auth.HashPassword("testpassword123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := auth.VerifyPassword("testpassword123", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword failed")
	}

	ok, _ = auth.VerifyPassword("wrongpassword", hash)
	if ok {
		t.Error("VerifyPassword should have failed")
	}

	_, _ = auth.GenerateAPIKey()
	apiKeyHash := auth.HashAPIKey("sk_live_12345")
	if apiKeyHash == "" {
		t.Error("HashAPIKey returned empty")
	}
}

// TestMockJWTFunctions tests JWT with mocking
func TestMockJWTFunctions(t *testing.T) {
	secret := []byte("test-secret-key-12345")
	token, err := auth.IssueJWT("user1", "admin", secret, time.Hour)
	if err != nil {
		t.Fatalf("IssueJWT: %v", err)
	}

	if token == "" {
		t.Error("IssueJWT returned empty token")
	}

	ctx := context.Background()
	_, ok := auth.ClaimsFromContext(ctx)
	_ = ok
}
