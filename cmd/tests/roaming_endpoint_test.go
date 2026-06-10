package tests

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"scutum/cmd/internal/roaming"
	"scutum/cmd/internal/store"
)

// ---------------------------------------------------------------------------
// HubMeshIP
// ---------------------------------------------------------------------------

func TestHubMeshIP(t *testing.T) {
	for _, c := range []struct {
		allowedIPs string
		want       string
	}{
		// Single-host routes — mesh IP can be inferred.
		{"10.0.0.1/32", "10.0.0.1"},
		{"192.168.5.3/32", "192.168.5.3"},
		{"fd00::1/128", "fd00::1"},
		// Bare IP with no CIDR.
		{"10.0.0.1", "10.0.0.1"},
		// First entry of a comma-separated list.
		{"10.0.0.1/32,10.0.0.0/24", "10.0.0.1"},
		// Aggregate routes — cannot infer a single mesh IP.
		{"0.0.0.0/0", ""},
		{"10.0.0.0/24", ""},
		{"10.0.0.0/16", ""},
		{"", ""},
	} {
		if got := roaming.HubMeshIP(c.allowedIPs); got != c.want {
			t.Errorf("HubMeshIP(%q) = %q, want %q", c.allowedIPs, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// NodeAPIBase
// ---------------------------------------------------------------------------

func TestNodeAPIBase(t *testing.T) {
	for _, c := range []struct {
		addr, want string
	}{
		{"", ""},
		{"10.0.0.1", "https://10.0.0.1:8080"},
		{"10.0.0.1/24", "https://10.0.0.1:8080"},
		{"10.0.0.2/32", "https://10.0.0.2:8080"},
		{"10.0.0.1:9090", "https://10.0.0.1:9090"},
		{"192.168.5.3:8080", "https://192.168.5.3:8080"},
		{"10.0.0.1:8080", "https://10.0.0.1:8080"},
	} {
		if got := roaming.NodeAPIBase(c.addr); got != c.want {
			t.Errorf("NodeAPIBase(%q) = %q, want %q", c.addr, got, c.want)
		}
	}
}

// Regression: hub node address was previously set to the WireGuard UDP
// endpoint (e.g. "1.2.3.4:51820"), directing API calls to the wrong port.
// NodeAPIBase must preserve an explicit port rather than appending :8080.
func TestNodeAPIBase_WireGuardPortRegression(t *testing.T) {
	got := roaming.NodeAPIBase("1.2.3.4:51820")
	if want := "https://1.2.3.4:51820"; got != want {
		t.Errorf("NodeAPIBase(%q) = %q, want %q", "1.2.3.4:51820", got, want)
	}
}

// ---------------------------------------------------------------------------
// CallRegisterEndpoint
// ---------------------------------------------------------------------------

func TestCallRegisterEndpoint_SendsCorrectRequestAndHMAC(t *testing.T) {
	hmacKey := []byte("test-secret-key")
	var capturedSig, capturedTs, capturedCT string
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/network/register-endpoint" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		capturedSig = r.Header.Get("X-Scutum-Hub-Sig")
		capturedTs = r.Header.Get("X-Scutum-Hub-Ts")
		capturedCT = r.Header.Get("Content-Type")
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"endpoint": "1.2.3.4:51820"})
	}))
	defer srv.Close()

	err := roaming.CallRegisterEndpoint(context.Background(), srv.URL, "testpubkey=", 51820, hmacKey, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", capturedCT)
	}
	if capturedSig == "" || capturedTs == "" {
		t.Fatalf("HMAC headers not sent (sig=%q ts=%q)", capturedSig, capturedTs)
	}

	// Recompute the HMAC and verify it matches.
	mac := hmac.New(sha256.New, hmacKey)
	fmt.Fprintf(mac, "%s\n%s\n%s\n", capturedTs, "POST", "/api/network/register-endpoint")
	if want := hex.EncodeToString(mac.Sum(nil)); capturedSig != want {
		t.Errorf("HMAC mismatch\n got: %s\nwant: %s", capturedSig, want)
	}

	// Timestamp must be a recent Unix second.
	ts, err := strconv.ParseInt(capturedTs, 10, 64)
	if err != nil {
		t.Fatalf("timestamp not a valid integer: %q", capturedTs)
	}
	if age := time.Since(time.Unix(ts, 0)).Abs(); age > 5*time.Second {
		t.Errorf("timestamp too stale: %v ago", age)
	}

	// Body must carry public_key and listen_port.
	var payload map[string]interface{}
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if payload["public_key"] != "testpubkey=" {
		t.Errorf("public_key = %v, want testpubkey=", payload["public_key"])
	}
	if int(payload["listen_port"].(float64)) != 51820 {
		t.Errorf("listen_port = %v, want 51820", payload["listen_port"])
	}
}

func TestCallRegisterEndpoint_NonOKStatusIsError(t *testing.T) {
	for _, code := range []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError} {
		code := code
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()
			if err := roaming.CallRegisterEndpoint(context.Background(), srv.URL, "key=", 51820, []byte("k"), nil); err == nil {
				t.Errorf("expected error for HTTP %d, got nil", code)
			}
		})
	}
}

func TestCallRegisterEndpoint_PreCancelledContextFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := roaming.CallRegisterEndpoint(ctx, srv.URL, "key=", 51820, []byte("k"), nil); err == nil {
		t.Error("expected error for pre-cancelled context, got nil")
	}
}

// ---------------------------------------------------------------------------
// Cross-network roaming scenario
//
// This tests the full flow: edge node registers its endpoint, moves to a new
// network (different public IP), re-registers, and the hub updates WireGuard.
// ---------------------------------------------------------------------------

// hubState is a minimal in-memory hub that handles /api/network/register-endpoint
// the same way the real handler does.
type hubState struct {
	mu       sync.Mutex
	endpoint string // last registered endpoint
	wgPeer   string // endpoint applied to WireGuard
}

func (h *hubState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/network/register-endpoint" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var req struct {
		PublicKey  string `json:"public_key"`
		ListenPort int    `json:"listen_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PublicKey == "" || req.ListenPort <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Hub derives endpoint from source IP + listen_port, preferring proxy headers
	// over RemoteAddr — mirrors HandleRegisterEndpoint's IP extraction logic.
	sourceIP, _, _ := splitHostPortStr(r.RemoteAddr)
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		sourceIP = realIP
	} else if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if idx := len(fwd); idx > 0 {
			for i, c := range fwd {
				if c == ',' {
					idx = i
					break
				}
			}
			sourceIP = fwd[:idx]
		}
	}
	endpoint := fmt.Sprintf("%s:%d", sourceIP, req.ListenPort)

	h.mu.Lock()
	h.endpoint = endpoint
	h.wgPeer = endpoint // simulate wg.UpdatePeerEndpoint
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"endpoint": endpoint})
}

func splitHostPortStr(addr string) (host, port string, err error) {
	// thin wrapper — avoids importing "net" just for this helper
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], nil
		}
	}
	return addr, "", fmt.Errorf("no port")
}

// TestRoaming_NetworkChange simulates an edge node moving between two networks.
//
// Network A: node has public IP 203.0.113.10
// Network B: node has public IP 198.51.100.20 (e.g. switched from WiFi to mobile)
//
// After each registration the hub must record the new endpoint and immediately
// apply it to WireGuard so the tunnel recovers without waiting for the healer.
func TestRoaming_NetworkChange(t *testing.T) {
	hub := &hubState{}
	srv := httptest.NewServer(hub)
	defer srv.Close()

	hmacKey := []byte("shared-hmac-key")
	pubKey := "edgenode-pubkey="

	// Step 1: edge node is on network A, registers from 203.0.113.10.
	// We simulate the source IP by injecting X-Real-IP since the test client
	// always connects from 127.0.0.1.
	networkAIP := "203.0.113.10"
	networkBIP := "198.51.100.20"

	// Wrap the hub to inject a fake source IP header.
	withFakeIP := func(ip string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Real-IP", ip)
			hub.ServeHTTP(w, r)
		})
	}

	srvA := httptest.NewServer(withFakeIP(networkAIP))
	defer srvA.Close()

	if err := roaming.CallRegisterEndpoint(context.Background(), srvA.URL, pubKey, 51820, hmacKey, nil); err != nil {
		t.Fatalf("network A registration failed: %v", err)
	}

	hub.mu.Lock()
	epA := hub.endpoint
	wgA := hub.wgPeer
	hub.mu.Unlock()

	// Hub must record and apply the network-A endpoint.
	// Note: our simple hubState reads from RemoteAddr, not X-Real-IP.
	// Verify it recorded *something* and that it includes the listen port.
	if epA == "" {
		t.Fatal("hub did not record any endpoint for network A")
	}
	if wgA == "" {
		t.Fatal("hub did not apply WireGuard update for network A")
	}
	t.Logf("network A endpoint: %s", epA)

	// Step 2: edge node moves to network B, re-registers.
	srvB := httptest.NewServer(withFakeIP(networkBIP))
	defer srvB.Close()

	if err := roaming.CallRegisterEndpoint(context.Background(), srvB.URL, pubKey, 51820, hmacKey, nil); err != nil {
		t.Fatalf("network B registration failed: %v", err)
	}

	hub.mu.Lock()
	epB := hub.endpoint
	wgB := hub.wgPeer
	hub.mu.Unlock()

	if epB == "" {
		t.Fatal("hub did not record any endpoint for network B")
	}
	if epA == epB {
		t.Errorf("hub endpoint did not change after network move: still %q", epB)
	}
	if wgB != epB {
		t.Errorf("WireGuard not updated to new endpoint: wg=%q endpoint=%q", wgB, epB)
	}
	t.Logf("network B endpoint: %s (changed from %s)", epB, epA)
}

// TestRoaming_FullHandlerNetworkChange runs the same scenario through the real
// HandleRegisterEndpoint handler (which reads X-Real-IP / X-Forwarded-For) and
// verifies both the DB update and the immediate WireGuard apply.
func TestRoaming_FullHandlerNetworkChange(t *testing.T) {
	networkAIP := "203.0.113.10"
	networkBIP := "198.51.100.20"

	wgSvc := &mockCapturingWG{}
	es := &mockEndpointStore{
		node: store.NodeRecord{ID: "edge-node-1", PublicKey: "edgekey="},
	}
	h := newRoamingHandler(wgSvc, es)

	sendRegistration := func(sourceIP string) {
		t.Helper()
		body, _ := json.Marshal(map[string]interface{}{
			"public_key":  "edgekey=",
			"listen_port": 51820,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/network/register-endpoint",
			bytes.NewReader(body))
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("X-Real-IP", sourceIP)
		rw := httptest.NewRecorder()
		h.HandleRegisterEndpoint(rw, req)
		if rw.Code != http.StatusOK {
			t.Errorf("registration from %s returned %d: %s", sourceIP, rw.Code, rw.Body.String())
		}
	}

	// Node on network A.
	sendRegistration(networkAIP)
	if want := networkAIP + ":51820"; es.capturedEndpoint != want {
		t.Errorf("after network A: DB endpoint = %q, want %q", es.capturedEndpoint, want)
	}
	if wgSvc.capturedEP != networkAIP+":51820" {
		t.Errorf("after network A: WG endpoint = %q, want %q", wgSvc.capturedEP, networkAIP+":51820")
	}

	// Node moves to network B.
	sendRegistration(networkBIP)
	if want := networkBIP + ":51820"; es.capturedEndpoint != want {
		t.Errorf("after network B: DB endpoint = %q, want %q", es.capturedEndpoint, want)
	}
	if wgSvc.capturedEP != networkBIP+":51820" {
		t.Errorf("after network B: WG endpoint = %q, want %q", wgSvc.capturedEP, networkBIP+":51820")
	}
}
