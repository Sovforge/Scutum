package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	plugin "scutum/cmd/internal/plugins"
)

func TestRuntimeOperations(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()

	t.Run("new runtime", func(t *testing.T) {
		rt, err := plugin.NewRuntime(ctx)
		if err != nil {
			t.Fatalf("NewRuntime failed: %v", err)
		}
		defer rt.Close(ctx)

		if rt == nil {
			t.Error("NewRuntime returned nil")
		}
	})

	t.Run("load invalid wasm file", func(t *testing.T) {
		rt, err := plugin.NewRuntime(ctx)
		if err != nil {
			t.Fatalf("NewRuntime failed: %v", err)
		}
		defer rt.Close(ctx)
		rr := plugin.NewRouteRegistrar(mux, rt)

		tmpDir := t.TempDir()
		invalidPath := filepath.Join(tmpDir, "invalid.wasm")
		_ = os.WriteFile(invalidPath, []byte("not wasm"), 0644)

		// FIX: Pass rr to Load
		err = rt.Load(ctx, "invalid-plugin", invalidPath, rr)
		if err == nil {
			t.Error("Expected error when loading invalid WASM file")
		}
	})

	t.Run("unload non-existent plugin", func(t *testing.T) {
		rt, err := plugin.NewRuntime(ctx)
		if err != nil {
			t.Fatalf("NewRuntime failed: %v", err)
		}
		defer rt.Close(ctx)

		err = rt.Unload(ctx, "non-existent")
		if err == nil {
			t.Error("Expected error when unloading non-existent plugin")
		}
	})
}

func TestRouteRegistrar(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()

	rt, err := plugin.NewRuntime(ctx)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	defer rt.Close(ctx)

	rr := plugin.NewRouteRegistrar(mux, rt)

	t.Run("add route for non-existent plugin", func(t *testing.T) {
		rr.AddRoute("non-existent", "GET", "/test", "handler")

		req := httptest.NewRequest("GET", "/plugin/non-existent/test", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected 503 for non-existent plugin, got %d", w.Code)
		}
	})

	t.Run("load minimal valid wasm", func(t *testing.T) {
		wasmPath := createMinimalWASM(t)

		// FIX: Pass rr to Load
		err := rt.Load(ctx, "min-plugin", wasmPath, rr)
		if err != nil {
			t.Errorf("Failed to load minimal valid WASM: %v", err)
		}

		info := rt.List()
		found := false
		for _, i := range info {
			if i.Name == "min-plugin" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Plugin not found in list after loading")
		}

		// Test Unload
		err = rt.Unload(ctx, "min-plugin")
		if err != nil {
			t.Errorf("Unload failed: %v", err)
		}
		if len(rt.List()) != 0 {
			t.Error("expected empty list after Unload")
		}
	})
}


// createMinimalWASM creates the smallest possible valid WASM v1 binary.
// This allows rt.Load and rt.CompileModule to succeed.
func createMinimalWASM(t *testing.T) string {
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "minimal.wasm")

	// Standard WASM Header: \0asm followed by version 1
	wasmBytes := []byte{
		0x00, 0x61, 0x73, 0x6d, // Magic: 0x6d736100
		0x01, 0x00, 0x00, 0x00, // Version: 1
	}

	err := os.WriteFile(wasmPath, wasmBytes, 0644)
	if err != nil {
		t.Fatalf("Failed to create minimal WASM file: %v", err)
	}

	return wasmPath
}
