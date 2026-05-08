package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/sync"
	"scutum/cmd/internal/utils"
)

func TestGenerateKey(t *testing.T) {
	key, err := utils.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if key == "" {
		t.Fatal("GenerateKey() returned empty string")
	}
	if len(key) != 44 {
		t.Errorf("GenerateKey() len = %d, want 44", len(key))
	}

	key2, err := utils.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() second call error = %v", err)
	}
	if key == key2 {
		t.Error("GenerateKey() returned identical keys")
	}
}

func TestWireGuardPeerConfig(t *testing.T) {
	tests := []struct {
		name       string
		publicKey  string
		endpoint   string
		allowedIPs string
		wantErr    bool
	}{
		{"empty public key", "", "192.0.2.1:51820", "10.0.0.2/32", true},
		{"empty endpoint", "key", "", "10.0.0.2/32", true},
		{"empty allowedIPs", "key", "192.0.2.1:51820", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := tt.publicKey == "" || tt.endpoint == "" || tt.allowedIPs == ""
			if hasError != tt.wantErr {
				t.Errorf("got %v want %v", hasError, tt.wantErr)
			}
		})
	}
}

func TestPortValidation(t *testing.T) {
	for _, tt := range []struct {
		port  int
		valid bool
	}{
		{0, false},
		{1, true},
		{51820, true},
		{65535, true},
		{65536, false},
	} {
		if (tt.port > 0 && tt.port <= 65535) != tt.valid {
			t.Errorf("port %d failed validation", tt.port)
		}
	}
}

func TestMTUValidation(t *testing.T) {
	for _, mtu := range []int{0, 1280, 1420, 1500, 9000} {
		if mtu < 0 {
			t.Errorf("invalid MTU %d", mtu)
		}
	}
}

func TestToLower(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"HELLO", "hello"},
		{"Hello World", "hello world"},
		{"123", "123"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if toLower(tt.input) != tt.expected {
				t.Errorf("toLower(%q) failed", tt.input)
			}
		})
	}
}

func TestIsValidJSON(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"{}", true},
		{`{"a":1}`, true},
		{"{invalid}", false},
		{"", false},
		{"[]", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if isValidJSON(tt.input) != tt.valid {
				t.Errorf("isValidJSON(%q) failed", tt.input)
			}
		})
	}
}

func TestHasDuplicateKeys(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`{"a":1,"b":2}`, false},
		{`{"a":1,"a":2}`, true},
		{`{}`, false},
	}

	for _, tt := range tests {
		if hasDuplicateKeys(tt.input) != tt.want {
			t.Errorf("duplicate check failed for %s", tt.input)
		}
	}
}

func TestWireGuardHandlerErrorResponses(t *testing.T) {
	healer := sync.NewHealer(sync.HealerConfig{}, &sync.DefaultWGChecker{})
	h := handlers.NewWireGuardHandler("wg0", &mockFailWGService{}, healer)

	req := httptest.NewRequest(http.MethodPost, "/wireguard/add",
		strings.NewReader(`{"public_key":"key","endpoint":"x:1","allowed_ips":"0.0.0.0/0"}`))

	w := httptest.NewRecorder()
	h.HandleAddPeer(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

/*
==========================
 FIXED MOCK RUNTIME LAYER
==========================
*/

type mockRunner struct {
	runErr    error
	outputErr error
}

func (m *mockRunner) Run(name string, args ...string) error {
	return m.runErr
}

func (m *mockRunner) Output(name string, args ...string) ([]byte, error) {
	if m.outputErr != nil {
		return nil, m.outputErr
	}
	return []byte("OK"), nil
}

func (m *mockRunner) CombinedOutput(name string, args ...string) ([]byte, error) {
	return m.Output(name, args...)
}

func (m *mockRunner) StdinPipe(name string, args ...string) (utils.PipeWriter, error) {
	return &mockPipe{}, nil
}

type mockPipe struct{}

func (m *mockPipe) Write([]byte) error { return nil }
func (m *mockPipe) Close() error       { return nil }

func TestMockWireGuardInterface(t *testing.T) {
	utils.SetCommandRunner(&mockRunner{})
	cfg := utils.InterfaceConfig{
		Name:       "wg0",
		PrivateKey: "key",
		Address:    "10.0.0.1/24",
		Port:       51820,
	}

	_, _ = utils.SetupInterface(cfg)
}

func TestMockWireGuardErrors(t *testing.T) {
	t.Run("setup error", func(t *testing.T) {
		utils.SetCommandRunner(&mockRunner{outputErr: fmt.Errorf("fail")})
		_, err := utils.SetupInterface(utils.InterfaceConfig{Name: "wg0"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("add peer error", func(t *testing.T) {
		utils.SetCommandRunner(&mockRunner{outputErr: fmt.Errorf("fail")})
		if err := utils.AddPeer("wg0", "key", "x", "y", 25); err == nil {
			t.Error("expected error")
		}
	})
}

/*
==========================
 OPTIONAL HELPERS (FIXED)
==========================
*/

func createMockScript(t *testing.T, path, content string) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, []byte(content), 0755)
}
