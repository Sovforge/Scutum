package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/store"
)

func TestStorageHandler(t *testing.T) {
	// Initialize a real SQLite store in memory for testing
	st, err := store.New(context.Background(), ":memory:", &mockKMS{})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	h := handlers.NewStorageHandler(st)

	t.Run("HandleCreateBackend", func(t *testing.T) {
		body := `{"name":"test-s3","provider":"minio","endpoint":"localhost:9000","access_key":"user","secret_key":"pass"}`
		req := httptest.NewRequest(http.MethodPost, "/storage/backends", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.HandleCreateBackend(w, req)
		if w.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", w.Code)
		}
	})

	t.Run("HandleListBackends", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/storage/backends", nil)
		w := httptest.NewRecorder()
		h.HandleListBackends(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		var backends []store.StorageBackend
		json.NewDecoder(w.Body).Decode(&backends)
		if len(backends) == 0 {
			t.Error("expected at least one backend")
		}
	})

	t.Run("HandleDeleteBackend", func(t *testing.T) {
		// First get the ID
		backends, _ := st.ListStorageBackends(context.Background())
		id := backends[0].ID

		// Test GetStorageBackendWithSecret indirectly or directly
		b, secret, err := st.GetStorageBackendWithSecret(context.Background(), id)
		if err != nil {
			t.Fatalf("GetStorageBackendWithSecret failed: %v", err)
		}
		if b.Name != "test-s3" || secret != "pass" {
			t.Errorf("data mismatch: %s/%s", b.Name, secret)
		}

		req := httptest.NewRequest(http.MethodDelete, "/storage/backends/"+id, nil)
		req.SetPathValue("id", id)
		w := httptest.NewRecorder()
		h.HandleDeleteBackend(w, req)
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", w.Code)
		}
	})
}

