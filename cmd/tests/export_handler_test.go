package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"
)

// ── fake export store ─────────────────────────────────────────────────────────

type fakeExportStore struct {
	users    []store.UserRecord
	roles    []store.RoleRecord
	nodes    []store.NodeRecord
	plugins  []store.PluginRecord
	backends []store.StorageBackend
	peers    []store.WGPeerRecord
	audits   []utils.AuditEntry
	failOn   string // set to "users", "roles", etc. to simulate errors
}

func (f *fakeExportStore) ListUsers(_ context.Context) ([]store.UserRecord, error) {
	if f.failOn == "users" {
		return nil, fmt.Errorf("db error")
	}
	return f.users, nil
}

func (f *fakeExportStore) ListRoles(_ context.Context) ([]store.RoleRecord, error) {
	if f.failOn == "roles" {
		return nil, fmt.Errorf("db error")
	}
	return f.roles, nil
}

func (f *fakeExportStore) ListNodes(_ context.Context) ([]store.NodeRecord, error) {
	if f.failOn == "nodes" {
		return nil, fmt.Errorf("db error")
	}
	return f.nodes, nil
}

func (f *fakeExportStore) ListEnabledPlugins(_ context.Context) ([]store.PluginRecord, error) {
	if f.failOn == "plugins" {
		return nil, fmt.Errorf("db error")
	}
	return f.plugins, nil
}

func (f *fakeExportStore) ListStorageBackends(_ context.Context) ([]store.StorageBackend, error) {
	if f.failOn == "storage" {
		return nil, fmt.Errorf("db error")
	}
	return f.backends, nil
}

func (f *fakeExportStore) ListWGPeers(_ context.Context) ([]store.WGPeerRecord, error) {
	if f.failOn == "peers" {
		return nil, fmt.Errorf("db error")
	}
	return f.peers, nil
}

func (f *fakeExportStore) ListAuditLogs(_ context.Context, _ int) ([]utils.AuditEntry, error) {
	if f.failOn == "audit" {
		return nil, fmt.Errorf("db error")
	}
	return f.audits, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newExportStore() *fakeExportStore {
	return &fakeExportStore{
		users: []store.UserRecord{
			{ID: "u1", Username: "admin", PasswordHash: "secret-hash"},
			{ID: "u2", Username: "viewer", PasswordHash: "another-hash"},
		},
		roles: []store.RoleRecord{
			{ID: "r1", Name: "admin"},
			{ID: "r2", Name: "viewer"},
		},
		nodes: []store.NodeRecord{
			{ID: "n1", Name: "edge-1", Type: "edge", Address: "10.0.0.2"},
		},
		plugins: []store.PluginRecord{
			{Name: "my-plugin", Path: "/plugins/my-plugin.wasm"},
		},
		backends: []store.StorageBackend{
			{ID: "b1", Name: "minio", Provider: "s3", Endpoint: "http://minio:9000", AccessKey: "minioadmin"},
		},
		peers: []store.WGPeerRecord{
			{NodeID: "n1", Endpoint: "1.2.3.4:51820", AllowedIPs: "10.0.0.2/32"},
		},
		audits: []utils.AuditEntry{
			{Action: "login", Method: "POST", Path: "/auth/login"},
		},
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestExportHandlerSuccess(t *testing.T) {
	h := handlers.NewExportHandler(newExportStore())
	req := httptest.NewRequest(http.MethodGet, "/admin/export", nil)
	w := httptest.NewRecorder()

	h.HandleExport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if !contains(ct, "application/json") {
		t.Errorf("expected JSON Content-Type, got %q", ct)
	}

	cd := w.Header().Get("Content-Disposition")
	if !contains(cd, "attachment") || !contains(cd, ".json") {
		t.Errorf("expected attachment Content-Disposition, got %q", cd)
	}

	var payload map[string]any
	if err := json.NewDecoder(w.Body).Decode(&payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	for _, key := range []string{"exported_at", "users", "roles", "nodes", "plugins", "storage_backends", "wg_peers", "audit_logs"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("missing key %q in export payload", key)
		}
	}
}

func TestExportHandlerPasswordsNotExposed(t *testing.T) {
	h := handlers.NewExportHandler(newExportStore())
	req := httptest.NewRequest(http.MethodGet, "/admin/export", nil)
	w := httptest.NewRecorder()

	h.HandleExport(w, req)

	body := w.Body.String()
	if contains(body, "secret-hash") || contains(body, "another-hash") {
		t.Error("export must not include password hashes")
	}
}

func TestExportHandlerEmptyStore(t *testing.T) {
	h := handlers.NewExportHandler(&fakeExportStore{})
	req := httptest.NewRequest(http.MethodGet, "/admin/export", nil)
	w := httptest.NewRecorder()

	h.HandleExport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(w.Body).Decode(&payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// All list fields should be present and be arrays (empty or null).
	for _, key := range []string{"users", "roles", "nodes", "plugins", "storage_backends", "wg_peers", "audit_logs"} {
		v, ok := payload[key]
		if !ok {
			t.Errorf("missing key %q", key)
			continue
		}
		if v != nil {
			arr, ok2 := v.([]any)
			if !ok2 {
				t.Errorf("key %q should be an array, got %T", key, v)
			} else if len(arr) != 0 {
				t.Errorf("key %q should be empty, got %d elements", key, len(arr))
			}
		}
	}
}

func TestExportHandlerUserStoreError(t *testing.T) {
	fs := newExportStore()
	fs.failOn = "users"
	h := handlers.NewExportHandler(fs)

	req := httptest.NewRequest(http.MethodGet, "/admin/export", nil)
	w := httptest.NewRecorder()
	h.HandleExport(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on user list error, got %d", w.Code)
	}
}

func TestExportHandlerNodeStoreError(t *testing.T) {
	fs := newExportStore()
	fs.failOn = "nodes"
	h := handlers.NewExportHandler(fs)

	req := httptest.NewRequest(http.MethodGet, "/admin/export", nil)
	w := httptest.NewRecorder()
	h.HandleExport(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on node list error, got %d", w.Code)
	}
}

func TestExportHandlerRoleStoreError(t *testing.T) {
	fs := newExportStore()
	fs.failOn = "roles"
	h := handlers.NewExportHandler(fs)

	req := httptest.NewRequest(http.MethodGet, "/admin/export", nil)
	w := httptest.NewRecorder()
	h.HandleExport(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on role list error, got %d", w.Code)
	}
}

func TestExportHandlerExportedAtIsRecent(t *testing.T) {
	h := handlers.NewExportHandler(newExportStore())
	req := httptest.NewRequest(http.MethodGet, "/admin/export", nil)
	w := httptest.NewRecorder()

	before := time.Now().UTC()
	h.HandleExport(w, req)
	after := time.Now().UTC()

	var payload struct {
		ExportedAt time.Time `json:"exported_at"`
	}
	if err := json.NewDecoder(w.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if payload.ExportedAt.Before(before) || payload.ExportedAt.After(after) {
		t.Errorf("exported_at %v is not between %v and %v", payload.ExportedAt, before, after)
	}
}

func TestExportHandlerAuditFallback(t *testing.T) {
	fs := newExportStore()
	fs.failOn = "audit"
	h := handlers.NewExportHandler(fs)

	req := httptest.NewRequest(http.MethodGet, "/admin/export", nil)
	w := httptest.NewRecorder()
	h.HandleExport(w, req)

	// Audit errors fall back to the in-memory ring buffer — should still be 200.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on audit fallback, got %d", w.Code)
	}
}
