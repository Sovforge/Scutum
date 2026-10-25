package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/sync"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// mockEndpointStore satisfies handlers.wgEndpointStore for testing
// HandleRegisterEndpoint.
type mockEndpointStore struct {
	node      store.NodeRecord
	nodeErr   error
	updateErr error

	capturedNodeID   string
	capturedEndpoint string
}

func (m *mockEndpointStore) GetNodeByPublicKey(_ context.Context, _ string) (store.NodeRecord, error) {
	return m.node, m.nodeErr
}

func (m *mockEndpointStore) UpdateWGPeerEndpoint(_ context.Context, nodeID, endpoint string) error {
	m.capturedNodeID = nodeID
	m.capturedEndpoint = endpoint
	return m.updateErr
}

// mockCapturingWG is a WG service that records the last UpdatePeerEndpoint call.
type mockCapturingWG struct {
	updateErr       error
	capturedKey     string
	capturedEP      string
}

func (m *mockCapturingWG) AddPeer(_, _, _, _ string, _ int) error { return nil }
func (m *mockCapturingWG) GetStatus(_ string) (string, error)     { return "", nil }
func (m *mockCapturingWG) GetDump(_ string) (string, error)       { return "", nil }
func (m *mockCapturingWG) UpdatePeerEndpoint(_, pub, ep string) error {
	m.capturedKey = pub
	m.capturedEP = ep
	return m.updateErr
}

func (m *mockCapturingWG) GetPeerEndpoint(_, _ string) (string, error) {
	return "", nil
}

// mockRoamingWGChecker records the full WGPeer passed to ReAddPeer so tests
// can assert the endpoint used.
type mockRoamingWGChecker struct {
	age    time.Duration
	ageErr error
	reAdds []sync.WGPeer
}

func (m *mockRoamingWGChecker) PeerHandshakeAge(_ context.Context, _, _ string) (time.Duration, error) {
	return m.age, m.ageErr
}

func (m *mockRoamingWGChecker) ReAddPeer(_ context.Context, peer sync.WGPeer) error {
	m.reAdds = append(m.reAdds, peer)
	return nil
}

// ---------------------------------------------------------------------------
// HandleRegisterEndpoint tests
// ---------------------------------------------------------------------------

func newRoamingHandler(wgSvc *mockCapturingWG, es *mockEndpointStore) *handlers.WireGuardHandler {
	healer := sync.NewHealer(sync.HealerConfig{}, &sync.DefaultWGChecker{})
	h := handlers.NewWireGuardHandler("wg0", wgSvc, healer, nil, nil)
	h.SetEndpointStore(es)
	return h
}

func TestHandleRegisterEndpoint_Valid(t *testing.T) {
	wgSvc := &mockCapturingWG{}
	es := &mockEndpointStore{
		node: store.NodeRecord{ID: "node-edge-1", PublicKey: "pubkey123="},
	}
	h := newRoamingHandler(wgSvc, es)

	body, _ := json.Marshal(map[string]interface{}{
		"public_key":  "pubkey123=",
		"listen_port": 51820,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.10:54321"
	rw := httptest.NewRecorder()
	h.HandleRegisterEndpoint(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rw.Code, rw.Body.String())
	}

	wantEndpoint := "203.0.113.10:51820"
	if es.capturedEndpoint != wantEndpoint {
		t.Errorf("DB endpoint = %q, want %q", es.capturedEndpoint, wantEndpoint)
	}
	if es.capturedNodeID != "node-edge-1" {
		t.Errorf("DB node ID = %q, want node-edge-1", es.capturedNodeID)
	}
	if wgSvc.capturedEP != wantEndpoint {
		t.Errorf("WG endpoint = %q, want %q", wgSvc.capturedEP, wantEndpoint)
	}
	if wgSvc.capturedKey != "pubkey123=" {
		t.Errorf("WG pubkey = %q, want pubkey123=", wgSvc.capturedKey)
	}

	var resp map[string]string
	json.NewDecoder(rw.Body).Decode(&resp)
	if resp["endpoint"] != wantEndpoint {
		t.Errorf("response body endpoint = %q, want %q", resp["endpoint"], wantEndpoint)
	}
}

func TestHandleRegisterEndpoint_InvalidJSON(t *testing.T) {
	h := newRoamingHandler(&mockCapturingWG{}, &mockEndpointStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint",
		bytes.NewReader([]byte(`{not json`)))
	req.RemoteAddr = "10.0.0.1:1234"
	rw := httptest.NewRecorder()
	h.HandleRegisterEndpoint(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rw.Code)
	}
}

func TestHandleRegisterEndpoint_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{"missing public_key", map[string]interface{}{"listen_port": 51820}},
		{"missing listen_port", map[string]interface{}{"public_key": "key="}},
		{"zero listen_port", map[string]interface{}{"public_key": "key=", "listen_port": 0}},
		{"negative listen_port", map[string]interface{}{"public_key": "key=", "listen_port": -1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newRoamingHandler(&mockCapturingWG{}, &mockEndpointStore{})
			body, _ := json.Marshal(c.body)
			req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint", bytes.NewReader(body))
			req.RemoteAddr = "10.0.0.1:1234"
			rw := httptest.NewRecorder()
			h.HandleRegisterEndpoint(rw, req)
			if rw.Code != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", c.name, rw.Code)
			}
		})
	}
}

func TestHandleRegisterEndpoint_NodeNotFound(t *testing.T) {
	es := &mockEndpointStore{nodeErr: errors.New("no such node")}
	h := newRoamingHandler(&mockCapturingWG{}, es)

	body, _ := json.Marshal(map[string]interface{}{"public_key": "unknown=", "listen_port": 51820})
	req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	rw := httptest.NewRecorder()
	h.HandleRegisterEndpoint(rw, req)
	if rw.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown key, got %d", rw.Code)
	}
}

func TestHandleRegisterEndpoint_DBUpdateFails(t *testing.T) {
	es := &mockEndpointStore{
		node:      store.NodeRecord{ID: "node-1"},
		updateErr: errors.New("db error"),
	}
	h := newRoamingHandler(&mockCapturingWG{}, es)

	body, _ := json.Marshal(map[string]interface{}{"public_key": "pubkey=", "listen_port": 51820})
	req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	rw := httptest.NewRecorder()
	h.HandleRegisterEndpoint(rw, req)
	if rw.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when DB update fails, got %d", rw.Code)
	}
}

// WG update failure must be non-fatal: the DB is already updated and the
// healer will apply the new endpoint on its next cycle.
func TestHandleRegisterEndpoint_WGUpdateFails_StillReturns200(t *testing.T) {
	es := &mockEndpointStore{node: store.NodeRecord{ID: "node-1"}}
	wgSvc := &mockCapturingWG{updateErr: errors.New("wg cli error")}
	h := newRoamingHandler(wgSvc, es)

	body, _ := json.Marshal(map[string]interface{}{"public_key": "pubkey=", "listen_port": 51820})
	req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	rw := httptest.NewRecorder()
	h.HandleRegisterEndpoint(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200 even when WG update fails, got %d", rw.Code)
	}
	if es.capturedEndpoint != "10.0.0.1:51820" {
		t.Errorf("DB should still be updated when WG fails, got %q", es.capturedEndpoint)
	}
}

// X-Real-IP overrides RemoteAddr for the endpoint source IP.
func TestHandleRegisterEndpoint_XRealIP(t *testing.T) {
	es := &mockEndpointStore{node: store.NodeRecord{ID: "node-1"}}
	h := newRoamingHandler(&mockCapturingWG{}, es)

	body, _ := json.Marshal(map[string]interface{}{"public_key": "pubkey=", "listen_port": 51820})
	req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234" // load-balancer address
	req.Header.Set("X-Real-IP", "203.0.113.55")
	rw := httptest.NewRecorder()
	h.HandleRegisterEndpoint(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rw.Code)
	}
	if es.capturedEndpoint != "203.0.113.55:51820" {
		t.Errorf("expected endpoint from X-Real-IP, got %q", es.capturedEndpoint)
	}
}

// X-Forwarded-For leftmost IP overrides RemoteAddr for the endpoint source IP.
func TestHandleRegisterEndpoint_XForwardedFor(t *testing.T) {
	es := &mockEndpointStore{node: store.NodeRecord{ID: "node-1"}}
	h := newRoamingHandler(&mockCapturingWG{}, es)

	body, _ := json.Marshal(map[string]interface{}{"public_key": "pubkey=", "listen_port": 51820})
	req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.77, 10.0.0.2, 10.0.0.1")
	rw := httptest.NewRecorder()
	h.HandleRegisterEndpoint(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rw.Code)
	}
	if es.capturedEndpoint != "203.0.113.77:51820" {
		t.Errorf("expected leftmost X-Forwarded-For IP, got %q", es.capturedEndpoint)
	}
}

func TestHandleRegisterEndpoint_NoEndpointStore(t *testing.T) {
	healer := sync.NewHealer(sync.HealerConfig{}, &sync.DefaultWGChecker{})
	h := handlers.NewWireGuardHandler("wg0", &mockCapturingWG{}, healer, nil, nil)
	// SetEndpointStore intentionally not called

	body, _ := json.Marshal(map[string]interface{}{"public_key": "pubkey=", "listen_port": 51820})
	req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	rw := httptest.NewRecorder()
	h.HandleRegisterEndpoint(rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when endpoint store not configured, got %d", rw.Code)
	}
}

// ---------------------------------------------------------------------------
// Healer FreshEndpoint (roaming recovery) tests
// ---------------------------------------------------------------------------

// TestHealerFreshEndpoint_NewEndpointTriggersReAdd verifies that a stale peer
// whose endpoint has changed (e.g. laptop moved to a new network) is re-added
// with the fresh endpoint from the database.
func TestHealerFreshEndpoint_NewEndpointTriggersReAdd(t *testing.T) {
	checker := &mockRoamingWGChecker{age: 5 * time.Minute}
	cfg := sync.HealerConfig{
		Interval:        20 * time.Millisecond,
		HandshakeMaxAge: 1 * time.Minute,
	}
	h := sync.NewHealer(cfg, checker)
	h.AddPeer(sync.WGPeer{
		IfaceName:  "wg0",
		PublicKey:  "edgekey=",
		Endpoint:   "1.2.3.4:51820",
		AllowedIPs: "10.0.0.2/32",
		FreshEndpoint: func(_ context.Context) (string, error) {
			return "5.6.7.8:51820", nil // node changed NAT IP
		},
	})

	h.Start(context.Background())
	time.Sleep(60 * time.Millisecond)
	h.Stop()

	if len(checker.reAdds) == 0 {
		t.Fatal("expected healer to re-add peer when endpoint changed, got 0 re-adds")
	}
	if checker.reAdds[0].Endpoint != "5.6.7.8:51820" {
		t.Errorf("re-add used endpoint %q, want 5.6.7.8:51820", checker.reAdds[0].Endpoint)
	}
}

// TestHealerFreshEndpoint_UnchangedEndpointNotReAddedRepeatedly verifies that
// after the first re-add, subsequent healer rounds with the same endpoint do
// not keep stacking re-adds (would cause handshake-loop thrash).
func TestHealerFreshEndpoint_UnchangedEndpointNotReAddedRepeatedly(t *testing.T) {
	checker := &mockRoamingWGChecker{age: 5 * time.Minute}
	cfg := sync.HealerConfig{
		Interval:        20 * time.Millisecond,
		HandshakeMaxAge: 1 * time.Minute,
	}
	h := sync.NewHealer(cfg, checker)
	h.AddPeer(sync.WGPeer{
		IfaceName:  "wg0",
		PublicKey:  "edgekey=",
		Endpoint:   "1.2.3.4:51820",
		AllowedIPs: "10.0.0.2/32",
		FreshEndpoint: func(_ context.Context) (string, error) {
			return "1.2.3.4:51820", nil // endpoint unchanged
		},
	})

	h.Start(context.Background())
	time.Sleep(80 * time.Millisecond) // enough for 3+ healer rounds
	h.Stop()

	// First round re-adds (lastUsed was ""), subsequent rounds see same endpoint.
	if len(checker.reAdds) > 1 {
		t.Errorf("expected at most 1 re-add for unchanged endpoint across %d rounds, got %d",
			4, len(checker.reAdds))
	}
}

// TestHealerFreshEndpoint_FetchErrorSuppressesReAdd verifies that if the DB
// lookup fails (e.g. store temporarily unavailable), the healer does NOT
// re-add the peer with a stale endpoint.
func TestHealerFreshEndpoint_FetchErrorSuppressesReAdd(t *testing.T) {
	checker := &mockRoamingWGChecker{age: 5 * time.Minute}
	cfg := sync.HealerConfig{
		Interval:        20 * time.Millisecond,
		HandshakeMaxAge: 1 * time.Minute,
	}
	h := sync.NewHealer(cfg, checker)
	h.AddPeer(sync.WGPeer{
		IfaceName:  "wg0",
		PublicKey:  "edgekey=",
		Endpoint:   "1.2.3.4:51820",
		AllowedIPs: "10.0.0.2/32",
		FreshEndpoint: func(_ context.Context) (string, error) {
			return "", errors.New("db unavailable")
		},
	})

	h.Start(context.Background())
	time.Sleep(60 * time.Millisecond)
	h.Stop()

	if len(checker.reAdds) != 0 {
		t.Errorf("expected 0 re-adds when FreshEndpoint errors, got %d", len(checker.reAdds))
	}
}
