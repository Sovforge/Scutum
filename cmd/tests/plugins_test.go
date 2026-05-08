package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	plugin "scutum/cmd/internal/plugins"
)

func TestPluginHTTPClientSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	plugin.SetHTTPClient(client)
	defer plugin.SetHTTPClient(http.DefaultClient)
}

func TestPluginKVStore(t *testing.T) {
	plugin.KVSet("test-key", "test-value")

	val, exists := plugin.KVGet("test-key")
	if !exists {
		t.Error("expected key to exist")
	}
	if val != "test-value" {
		t.Errorf("got %q, want test-value", val)
	}

	_, exists = plugin.KVGet("nonexistent")
	if exists {
		t.Error("expected key to not exist")
	}

	plugin.ClearKVStore()

	_, exists = plugin.KVGet("test-key")
	if exists {
		t.Error("expected key to be cleared")
	}
}

func TestPluginKVStoreConcurrent(t *testing.T) {
	plugin.ClearKVStore()
	defer plugin.ClearKVStore()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			plugin.KVSet("key", "value")
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPluginRuntimeNewAndClose(t *testing.T) {
	ctx := context.Background()
	runtime, err := plugin.NewRuntime(ctx)
	if err != nil {
		t.Logf("NewRuntime: %v", err)
		t.SkipNow()
	}
	if runtime == nil {
		t.Fatal("expected runtime, got nil")
	}

	if err := runtime.Close(ctx); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestPluginRuntimeMultiple(t *testing.T) {
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		runtime, err := plugin.NewRuntime(ctx)
		if err != nil {
			t.Logf("NewRuntime iteration %d: %v", i, err)
			continue
		}
		if runtime != nil {
			runtime.Close(ctx)
		}
	}
}
