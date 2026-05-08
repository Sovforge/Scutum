package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/kms"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/sync"
	"scutum/cmd/internal/utils"
)

func newSyncTestStore(t *testing.T) *store.Store {
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

func newSyncHandler(t *testing.T) (*handlers.SyncHandler, *sync.Pusher) {
	t.Helper()
	utils.InitLogger(0, false)
	s := newSyncTestStore(t)
	hmacKey, err := sync.NewHMACKey()
	if err != nil {
		t.Fatalf("NewHMACKey: %v", err)
	}
	pusher := sync.NewPusher(sync.PushConfig{HMACKey: hmacKey})
	t.Cleanup(pusher.Stop)
	return handlers.NewSyncHandler(s, pusher, nil), pusher
}

// --- HandleRegisterEdge ---

func TestHandleRegisterEdgeInvalidJSON(t *testing.T) {
	h, _ := newSyncHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/sync/register-edge", bytes.NewReader([]byte("{")))
	w := httptest.NewRecorder()
	h.HandleRegisterEdge(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRegisterEdgeMissingNodeID(t *testing.T) {
	h, _ := newSyncHandler(t)
	body, _ := json.Marshal(map[string]string{"url": "http://edge1:8080/sync"})
	req := httptest.NewRequest(http.MethodPost, "/sync/register-edge", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRegisterEdge(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRegisterEdgeMissingURL(t *testing.T) {
	h, _ := newSyncHandler(t)
	body, _ := json.Marshal(map[string]string{"node_id": "edge-1"})
	req := httptest.NewRequest(http.MethodPost, "/sync/register-edge", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRegisterEdge(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRegisterEdgeSuccess(t *testing.T) {
	h, pusher := newSyncHandler(t)
	body, _ := json.Marshal(map[string]string{
		"node_id": "edge-42",
		"url":     "http://edge42:8080/sync",
		"token":   "secret-token",
	})
	req := httptest.NewRequest(http.MethodPost, "/sync/register-edge", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRegisterEdge(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "registered" {
		t.Errorf("expected status=registered, got %q", resp["status"])
	}
	if pusher.EdgeCount() != 1 {
		t.Errorf("expected 1 registered edge, got %d", pusher.EdgeCount())
	}
}

func TestHandleRegisterEdgeNoToken(t *testing.T) {
	h, pusher := newSyncHandler(t)
	body, _ := json.Marshal(map[string]string{
		"node_id": "edge-no-token",
		"url":     "http://edge:8080/sync",
	})
	req := httptest.NewRequest(http.MethodPost, "/sync/register-edge", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRegisterEdge(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if pusher.EdgeCount() != 1 {
		t.Errorf("expected 1 registered edge, got %d", pusher.EdgeCount())
	}
}

func TestHandleRegisterEdgeMultipleEdges(t *testing.T) {
	h, pusher := newSyncHandler(t)

	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(map[string]string{
			"node_id": "edge-" + string(rune('a'+i)),
			"url":     "http://edge:8080/sync",
		})
		req := httptest.NewRequest(http.MethodPost, "/sync/register-edge", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.HandleRegisterEdge(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("edge %d: expected 200, got %d", i, w.Code)
		}
	}
	if pusher.EdgeCount() != 3 {
		t.Errorf("expected 3 edges, got %d", pusher.EdgeCount())
	}
}

// --- HandlePush ---

func TestHandlePushNoEdges(t *testing.T) {
	h, _ := newSyncHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/sync/push", nil)
	w := httptest.NewRecorder()
	h.HandlePush(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]int
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["total"] != 0 {
		t.Errorf("expected total=0, got %d", resp["total"])
	}
}

func TestHandlePushResponseShape(t *testing.T) {
	h, _ := newSyncHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/sync/push", nil)
	w := httptest.NewRecorder()
	h.HandlePush(w, req)

	var resp map[string]int
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// All three keys must be present.
	for _, key := range []string{"success", "failed", "total"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("response missing key %q", key)
		}
	}
	if resp["success"]+resp["failed"] != resp["total"] {
		t.Errorf("success+failed != total: %d+%d != %d", resp["success"], resp["failed"], resp["total"])
	}
}

func TestHandlePushWithEdge(t *testing.T) {
	// Register a mock edge server that returns 200.
	edgeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer edgeSrv.Close()

	h, pusher := newSyncHandler(t)

	// Register the edge.
	body, _ := json.Marshal(map[string]string{
		"node_id": "live-edge",
		"url":     edgeSrv.URL + "/sync",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/sync/register-edge", bytes.NewReader(body))
	regW := httptest.NewRecorder()
	h.HandleRegisterEdge(regW, regReq)
	if regW.Code != http.StatusOK {
		t.Fatalf("register: expected 200, got %d", regW.Code)
	}

	if pusher.EdgeCount() != 1 {
		t.Fatalf("expected 1 edge registered")
	}

	pushReq := httptest.NewRequest(http.MethodPost, "/sync/push", nil)
	pushW := httptest.NewRecorder()
	h.HandlePush(pushW, pushReq)

	if pushW.Code != http.StatusOK {
		t.Fatalf("push: expected 200, got %d: %s", pushW.Code, pushW.Body.String())
	}

	var resp map[string]int
	json.NewDecoder(pushW.Body).Decode(&resp)
	if resp["total"] != 1 {
		t.Errorf("expected total=1, got %d", resp["total"])
	}
}
