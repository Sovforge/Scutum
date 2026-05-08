package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/store"
)

// ── fake store ────────────────────────────────────────────────────────────────

type fakeNodeStore struct {
	nodes     []store.NodeRecord
	createErr error
	deleteErr error
}

func (f *fakeNodeStore) ListNodes(_ context.Context) ([]store.NodeRecord, error) {
	return f.nodes, nil
}

func (f *fakeNodeStore) GetNode(_ context.Context, id string) (store.NodeRecord, error) {
	for _, n := range f.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return store.NodeRecord{}, errNotFound
}

func (f *fakeNodeStore) CreateNode(_ context.Context, n store.NodeRecord) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.nodes = append(f.nodes, n)
	return nil
}

func (f *fakeNodeStore) DeleteNode(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for i, n := range f.nodes {
		if n.ID == id {
			f.nodes = append(f.nodes[:i], f.nodes[i+1:]...)
			return nil
		}
	}
	return errNotFound
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestNodeHandlerList(t *testing.T) {
	s := &fakeNodeStore{nodes: []store.NodeRecord{
		{ID: "n1", Name: "core-01", Type: "hub", Address: "10.0.0.1:51820", PublicKey: "abc="},
		{ID: "n2", Name: "edge-01", Type: "edge", Address: "10.0.0.2:51820", PublicKey: "def="},
	}}
	h := handlers.NewNodeHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/nodes", nil)
	w := httptest.NewRecorder()
	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var out []store.NodeRecord
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(out))
	}
}

func TestNodeHandlerListEmpty(t *testing.T) {
	h := handlers.NewNodeHandler(&fakeNodeStore{})
	req := httptest.NewRequest(http.MethodGet, "/nodes", nil)
	w := httptest.NewRecorder()
	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// must return a JSON array, not null
	body := w.Body.String()
	if !contains(body, "[") {
		t.Errorf("expected JSON array, got: %s", body)
	}
}

func TestNodeHandlerGet(t *testing.T) {
	s := &fakeNodeStore{nodes: []store.NodeRecord{
		{ID: "n1", Name: "core-01", Type: "hub", Address: "10.0.0.1:51820", PublicKey: "abc="},
	}}
	h := handlers.NewNodeHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/nodes/n1", nil)
	req.SetPathValue("id", "n1")
	w := httptest.NewRecorder()
	h.HandleGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestNodeHandlerGetNotFound(t *testing.T) {
	h := handlers.NewNodeHandler(&fakeNodeStore{})
	req := httptest.NewRequest(http.MethodGet, "/nodes/missing", nil)
	req.SetPathValue("id", "missing")
	w := httptest.NewRecorder()
	h.HandleGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestNodeHandlerCreate(t *testing.T) {
	s := &fakeNodeStore{}
	h := handlers.NewNodeHandler(s)

	body, _ := json.Marshal(map[string]string{
		"name": "worker-01", "type": "hub",
		"address": "10.0.0.3:51820", "public_key": "xyz=",
	})
	req := httptest.NewRequest(http.MethodPost, "/nodes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if len(s.nodes) != 1 {
		t.Errorf("expected 1 node in store, got %d", len(s.nodes))
	}
}

func TestNodeHandlerCreateInvalidType(t *testing.T) {
	h := handlers.NewNodeHandler(&fakeNodeStore{})
	body, _ := json.Marshal(map[string]string{
		"name": "x", "type": "invalid",
		"address": "10.0.0.1:51820", "public_key": "abc=",
	})
	req := httptest.NewRequest(http.MethodPost, "/nodes", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestNodeHandlerCreateMissingFields(t *testing.T) {
	h := handlers.NewNodeHandler(&fakeNodeStore{})
	body, _ := json.Marshal(map[string]string{"name": "x"})
	req := httptest.NewRequest(http.MethodPost, "/nodes", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestNodeHandlerDelete(t *testing.T) {
	s := &fakeNodeStore{nodes: []store.NodeRecord{
		{ID: "n1", Name: "core-01", Type: "hub", Address: "10.0.0.1:51820", PublicKey: "abc="},
	}}
	h := handlers.NewNodeHandler(s)

	req := httptest.NewRequest(http.MethodDelete, "/nodes/n1", nil)
	req.SetPathValue("id", "n1")
	w := httptest.NewRecorder()
	h.HandleDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if len(s.nodes) != 0 {
		t.Errorf("expected node to be removed from store")
	}
}

func TestNodeHandlerDeleteNotFound(t *testing.T) {
	h := handlers.NewNodeHandler(&fakeNodeStore{})
	req := httptest.NewRequest(http.MethodDelete, "/nodes/missing", nil)
	req.SetPathValue("id", "missing")
	w := httptest.NewRecorder()
	h.HandleDelete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
