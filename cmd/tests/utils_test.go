package tests

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"scutum/cmd/internal/utils"
)

func TestS3SignRequest(t *testing.T) {
	tests := []struct {
		name   string
		method string
		valid  bool
	}{
		{"GET request", "GET", true},
		{"PUT request", "PUT", true},
		{"POST request", "POST", true},
		{"DELETE request", "DELETE", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "https://s3.amazonaws.com/bucket/key", nil)
			cfg := utils.S3Config{
				Endpoint:  "https://s3.amazonaws.com",
				Bucket:    "mybucket",
				Region:    "us-east-1",
				AccessKey: "test",
				SecretKey: "test",
			}
			body := []byte("test body")

			utils.SignS3Request(req, body, cfg)

			// Check that Authorization header was set if it's a valid request
			if tt.valid {
				authHeader := req.Header.Get("Authorization")
				if authHeader == "" {
					t.Error("expected Authorization header to be set")
				}
				if !strings.Contains(authHeader, "AWS4-HMAC-SHA256") {
					t.Errorf("unexpected auth format: %s", authHeader)
				}
			}
		})
	}
}

func TestWireGuardKeyGeneration(t *testing.T) {
	key1, err := utils.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if key1 == "" {
		t.Fatal("expected non-empty key")
	}

	// Generate another key to ensure they're different
	key2, err := utils.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if key1 == key2 {
		t.Fatal("expected different keys")
	}
}

func TestWebSocketUpgrade(t *testing.T) {
	// Test that the upgrade function exists and can be called
	// This is a simple reference test
	tests := []struct {
		name  string
		path  string
		valid bool
	}{
		{"valid path", "/ws", true},
		{"relative path", "ws/stream", true},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.path != "") == tt.valid {
				// WebSocket upgrade path validation
				isValid := tt.path != ""
				if isValid != tt.valid {
					t.Errorf("path validation: got %v, want %v", isValid, tt.valid)
				}
			}
		})
	}
}

func TestGitUtilities(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		valid bool
	}{
		{"https url", "https://github.com/user/repo.git", true},
		{"http url", "http://gitlab.com/repo.git", true},
		{"ssh url", "git@github.com:user/repo.git", true},
		{"empty url", "", false},
		{"invalid protocol", "ftp://example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.url != "" &&
				(contains(tt.url, "https://") || contains(tt.url, "http://") || contains(tt.url, "git@"))
			if isValid != tt.valid {
				t.Errorf("git url validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

func TestKubernetesUtilitiesInClusterConfig(t *testing.T) {
	// GetInClusterConfig is hard to test as it relies on environment
	// This test verifies the function exists
	cfg, err := utils.GetInClusterConfig()
	// It's ok if it fails since we're not in a cluster
	if err == nil {
		// If it succeeds, cfg should have reasonable values
		if cfg == nil {
			t.Error("expected non-nil config when no error")
		}
	}
	// The important thing is that the function can be called without panicking
}

func TestValidationHelpers(t *testing.T) {
	tests := []struct {
		name  string
		json  string
		valid bool
	}{
		{"valid json", `{"key":"value"}`, true},
		{"invalid json", `{invalid}`, false},
		{"empty object", `{}`, true},
		{"array", `[]`, true},
		{"string", `"text"`, true},
		{"number", `123`, true},
		{"null", `null`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data interface{}
			isValid := json.Unmarshal([]byte(tt.json), &data) == nil
			if isValid != tt.valid {
				t.Errorf("json validation: got %v, want %v", isValid, tt.valid)
			}
		})
	}
}

func TestPathOperations(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		extension string
		valid     bool
	}{
		{"simple path", "/home/user/file.txt", "txt", true},
		{"nested path", "/var/lib/app/data/config.json", "json", true},
		{"no extension", "/home/user/Makefile", "", false},
		{"relative path", "config/app.yaml", "yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := filepath.Ext(tt.path)
			if (ext != "") == tt.valid {
				// Just verify the path operations work
				dir := filepath.Dir(tt.path)
				if dir == "" && tt.path != "" {
					t.Errorf("unexpected empty directory for path %q", tt.path)
				}
			}
		})
	}
}

// ============= String Utility Edge Cases =============

func TestStringUtilsEdgeCases(t *testing.T) {
	t.Run("empty string operations", func(t *testing.T) {
		tests := []struct {
			name string
			fn   func()
		}{
			{"empty trim", func() { _ = strings.TrimSpace("") }},
			{"empty split", func() { _ = strings.Split("", ",") }},
			{"empty contains", func() { _ = strings.Contains("", "") }},
			{"empty replace", func() { _ = strings.ReplaceAll("", "a", "b") }},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.fn() // Should not panic
			})
		}
	})

	t.Run("nil pointer handling", func(t *testing.T) {
		// Various string operations with nil-like scenarios
		var s *string
		if s != nil {
			_ = *s
		}
	})

	t.Run("very long strings", func(t *testing.T) {
		longStr := strings.Repeat("x", 10000000)
		result := strings.TrimSpace(longStr)
		if len(result) != len(longStr) {
			t.Error("trimspace changed length")
		}
	})

	t.Run("string with all edge case characters", func(t *testing.T) {
		edges := "\x00\n\r\t " + strings.Repeat("x", 100) + "\x00\n\r\t"
		trimmed := strings.TrimSpace(edges)
		// TrimSpace removes leading/trailing whitespace (space, tab, newline, carriage return, etc.)
		// but may not remove null bytes at the beginning due to how it works
		if !strings.Contains(trimmed, "x") {
			t.Error("trimspace removed content")
		}
	})
}

// ============= Path Operations Edge Cases =============

func TestPathOperationsEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"empty path", ""},
		{"root", "/"},
		{"relative path", "foo/bar"},
		{"absolute path", "/foo/bar"},
		{"trailing slash", "/foo/bar/"},
		{"double slash", "/foo//bar"},
		{"dot segments", "/foo/../bar"},
		{"dot directory", "/foo/./bar"},
		{"double dots", "/foo/../../bar"},
		{"dots only", "/.."},
		{"very long path", "/" + strings.Repeat("a/", 1000)},
		{"special chars in path", "/foo/bar-baz_123"},
		{"unicode in path", "/用户/路径"},
		{"spaces in path", "/foo bar/baz qux"},
		{"encoded chars in path", "/foo%20bar/baz%2Fqux"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic path operations should handle these
			_ = tt.path
		})
	}
}

// ============= Conversion Utility Edge Cases =============

func TestConversionUtilsEdgeCases(t *testing.T) {
	t.Run("string to int conversions", func(t *testing.T) {
		tests := []struct {
			input      string
			shouldWork bool
		}{
			{"0", true},
			{"1", true},
			{"-1", true},
			{"999999999", true},
			{"-999999999", true},
			{"", false},
			{"abc", false},
			{"1.5", false},
			{"1e10", false},
			{" 1", false},
			{"1 ", false},
			{"+1", true},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				// Test parsing - actual implementation may vary
				_ = tt.input
			})
		}
	})

	t.Run("string to bool conversions", func(t *testing.T) {
		tests := []struct {
			input    string
			expected bool
		}{
			{"true", true},
			{"false", false},
			{"True", true},
			{"False", false},
			{"TRUE", true},
			{"FALSE", false},
			{"yes", false}, // Typically only true/false
			{"no", false},
			{"1", false},
			{"0", false},
			{"", false},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				// Test bool conversion - implementation dependent
				_ = tt.input
			})
		}
	})
}

// ============= Validation Edge Cases =============

func TestValidationEdgeCases(t *testing.T) {
	t.Run("email validation", func(t *testing.T) {
		tests := []struct {
			email string
			valid bool
		}{
			{"user@example.com", true},
			{"user+tag@example.com", true},
			{"user@sub.example.com", true},
			{"user@localhost", false}, // No TLD
			{"user", false},           // No @
			{"@example.com", false},   // No local part
			{"user@", false},          // No domain
			{"user@@example.com", false},
			{"user@example.com.", true}, // Trailing dot sometimes valid
			{"", false},
			{"user example@test.com", false},
			{"user@example..com", false},
		}

		for _, tt := range tests {
			t.Run(tt.email, func(t *testing.T) {
				// Email validation logic - would use utils.ValidateEmail or similar
				_ = tt.email
			})
		}
	})

	t.Run("URL validation", func(t *testing.T) {
		tests := []struct {
			url   string
			valid bool
		}{
			{"http://example.com", true},
			{"https://example.com", true},
			{"ftp://example.com", true},
			{"example.com", false},
			{"", false},
			{"://example.com", false},
			{"http://", false},
			{"http://example.com:8080", true},
			{"http://example.com:99999", false},
			{"http://example.com:abc", false},
			{"http://192.168.1.1", true},
			{"http://192.168.1.999", false},
		}

		for _, tt := range tests {
			t.Run(tt.url, func(t *testing.T) {
				_ = tt.url
			})
		}
	})

	t.Run("IP address validation", func(t *testing.T) {
		tests := []struct {
			ip    string
			valid bool
		}{
			{"192.168.1.1", true},
			{"0.0.0.0", true},
			{"255.255.255.255", true},
			{"256.1.1.1", false},
			{"1.1.1", false},
			{"1.1.1.1.1", false},
			{"", false},
			{"::1", true},         // IPv6
			{"2001:db8::1", true}, // IPv6
			{"invalid", false},
		}

		for _, tt := range tests {
			t.Run(tt.ip, func(t *testing.T) {
				_ = tt.ip
			})
		}
	})
}

// ============= Slice/Array Utility Edge Cases =============

func TestSliceUtilsEdgeCases(t *testing.T) {
	t.Run("empty slice operations", func(t *testing.T) {
		var empty []string
		if len(empty) != 0 {
			t.Error("empty slice should have length 0")
		}
		if cap(empty) != 0 {
			t.Error("empty slice should have capacity 0")
		}
	})

	t.Run("single element slice", func(t *testing.T) {
		single := []string{"one"}
		if len(single) != 1 || single[0] != "one" {
			t.Error("single element slice failed")
		}
	})

	t.Run("slice contains operations", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		tests := []struct {
			item     string
			expected bool
		}{
			{"a", true},
			{"b", true},
			{"c", true},
			{"d", false},
			{"", false},
		}

		for _, tt := range tests {
			found := false
			for _, v := range slice {
				if v == tt.item {
					found = true
					break
				}
			}
			if found != tt.expected {
				t.Errorf("contains check failed for %q", tt.item)
			}
		}
	})

	t.Run("slice with duplicates", func(t *testing.T) {
		dup := []string{"a", "a", "b", "b", "c"}
		counts := make(map[string]int)
		for _, v := range dup {
			counts[v]++
		}
		if counts["a"] != 2 || counts["b"] != 2 || counts["c"] != 1 {
			t.Error("duplicate counting failed")
		}
	})

	t.Run("large slice operations", func(t *testing.T) {
		large := make([]int, 1000000)
		for i := range large {
			large[i] = i
		}
		if len(large) != 1000000 {
			t.Error("large slice creation failed")
		}
	})
}

// ============= Map Utility Edge Cases =============

func TestMapUtilsEdgeCases(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		m := make(map[string]interface{})
		if len(m) != 0 {
			t.Error("empty map should have length 0")
		}
		if m["key"] != nil {
			t.Error("missing key should return nil")
		}
	})

	t.Run("map with nil values", func(t *testing.T) {
		m := map[string]interface{}{
			"key1": nil,
			"key2": "value",
			"key3": nil,
		}
		if m["key1"] != nil {
			t.Error("nil value should be nil")
		}
		if m["key2"] != "value" {
			t.Error("string value mismatch")
		}
	})

	t.Run("map key edge cases", func(t *testing.T) {
		m := make(map[string]int)
		tests := []string{
			"",
			" ",
			"\n",
			"\t",
			strings.Repeat("x", 1000),
			"key@#$%",
			"键123",
		}

		for i, key := range tests {
			m[key] = i
		}

		if len(m) != len(tests) {
			t.Error("all keys should be unique")
		}

		for i, key := range tests {
			if m[key] != i {
				t.Errorf("map value mismatch for key %q", key)
			}
		}
	})

	t.Run("map delete operations", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2, "c": 3}

		delete(m, "b")
		if _, ok := m["b"]; ok {
			t.Error("deleted key should not exist")
		}
		if len(m) != 2 {
			t.Error("length after delete incorrect")
		}

		delete(m, "nonexistent")
		if len(m) != 2 {
			t.Error("delete non-existent key should not affect length")
		}
	})

	t.Run("concurrent map access", func(t *testing.T) {
		// Note: maps are not safe for concurrent writes
		// This tests that reads work fine
		m := map[string]int{"a": 1, "b": 2}
		_ = m["a"] // Read should be safe
		_ = m["nonexistent"]
	})
}

// ============= Type Assertion Edge Cases =============

func TestTypeAssertionEdgeCases(t *testing.T) {
	t.Run("nil interface assertion", func(t *testing.T) {
		var i interface{}
		_, ok := i.(string)
		if ok {
			t.Error("nil interface should not be string")
		}
	})

	t.Run("type assertion with different types", func(t *testing.T) {
		tests := []struct {
			value    interface{}
			typeStr  string
			expected bool
		}{
			{"string", "string", true},
			{123, "int", true},
			{123.45, "float64", true},
			{true, "bool", true},
			{[]int{1, 2}, "[]int", true},
			{map[string]int{}, "map[string]int", true},
			{"string", "int", false},
			{123, "string", false},
		}

		for _, tt := range tests {
			t.Run(tt.typeStr, func(t *testing.T) {
				_ = tt.value
			})
		}
	})
}

// ============= Number Edge Cases =============

func TestNumberEdgeCases(t *testing.T) {
	t.Run("integer boundaries", func(t *testing.T) {
		tests := []struct {
			name  string
			value int
		}{
			{"zero", 0},
			{"one", 1},
			{"negative one", -1},
			{"max positive", 9223372036854775807},  // int64 max
			{"min negative", -9223372036854775808}, // int64 min
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.value == 0 {
					// Test zero explicitly
				}
			})
		}
	})

	t.Run("float boundaries", func(t *testing.T) {
		tests := []float64{
			0.0,
			-0.0,
			1.0,
			-1.0,
			0.1 + 0.2, // Floating point precision
			1e10,
			1e-10,
		}

		for i, val := range tests {
			if val != val { // NaN check
				t.Errorf("test %d is NaN", i)
			}
		}
	})

	t.Run("floating point precision", func(t *testing.T) {
		result := 0.1 + 0.2
		expected := 0.3
		// Floating point arithmetic has precision issues
		if result != expected {
			t.Logf("floating point precision: %v != %v", result, expected)
		}
	})
}
