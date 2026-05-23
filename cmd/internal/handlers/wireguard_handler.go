package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/store"
	"scutum/cmd/internal/sync"
	"scutum/cmd/internal/wireguard"
)

type wgNodeStore interface {
	ListNodes(ctx context.Context) ([]store.NodeRecord, error)
}

type wgKeyStore interface {
	GetSecret(ctx context.Context, key string) ([]byte, error)
}

type wgPeerStore interface {
	UpsertWGPeer(ctx context.Context, p store.WGPeerRecord) error
}

type wgEndpointStore interface {
	GetNodeByPublicKey(ctx context.Context, publicKey string) (store.NodeRecord, error)
	UpdateWGPeerEndpoint(ctx context.Context, nodeID, endpoint string) error
}

type WireGuardHandler struct {
	IfaceName     string
	SecretsDir    string
	wg            wireguard.Service
	healer        *sync.Healer
	nodeStore     wgNodeStore
	keyStore      wgKeyStore
	peerStore     wgPeerStore
	endpointStore wgEndpointStore
}

func NewWireGuardHandler(ifaceName string, wg wireguard.Service, healer *sync.Healer, ns wgNodeStore, ks wgKeyStore) *WireGuardHandler {
	return &WireGuardHandler{
		IfaceName: ifaceName,
		wg:        wg,
		healer:    healer,
		nodeStore: ns,
		keyStore:  ks,
	}
}

// SetPeerStore attaches a persistent store so that peers added via HandleAddPeer
// survive container restarts.
func (h *WireGuardHandler) SetPeerStore(ps wgPeerStore) {
	h.peerStore = ps
}

type PeerRequest struct {
	PublicKey           string `json:"public_key"`
	Endpoint            string `json:"endpoint"`
	AllowedIPs          string `json:"allowed_ips"` // comma-separated; caller's responsibility
	NodeID              string `json:"node_id,omitempty"` // optional: links peer to a node record for persistence
	PersistentKeepalive int    `json:"persistent_keepalive,omitempty"` // seconds; 0 uses default (25)
}

func (h *WireGuardHandler) HandleAddPeer(w http.ResponseWriter, r *http.Request) {
	var req PeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.PublicKey == "" || req.Endpoint == "" || req.AllowedIPs == "" {
		http.Error(w, "public_key, endpoint, and allowed_ips are required", http.StatusBadRequest)
		return
	}

	keepalive := req.PersistentKeepalive
	if keepalive <= 0 {
		keepalive = 25
	}
	if err := h.wg.AddPeer(h.IfaceName, req.PublicKey, req.Endpoint, req.AllowedIPs, keepalive); err != nil {
		http.Error(w, fmt.Sprintf("wg error: %v", err), http.StatusInternalServerError)
		return
	}

	// Persist so peers survive container restarts.
	if h.peerStore != nil && req.NodeID != "" {
		_ = h.peerStore.UpsertWGPeer(r.Context(), store.WGPeerRecord{
			NodeID:     req.NodeID,
			Endpoint:   req.Endpoint,
			AllowedIPs: req.AllowedIPs,
		})
	}

	// Register with healer for health monitoring
	if h.healer != nil {
		h.healer.AddPeer(sync.WGPeer{
			IfaceName:  h.IfaceName,
			PublicKey:  req.PublicKey,
			Endpoint:   req.Endpoint,
			AllowedIPs: req.AllowedIPs,
		})
	}

	audit("PEER_ADDED", r, "public_key", req.PublicKey, "endpoint", req.Endpoint, "allowed_ips", req.AllowedIPs)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("peer added"))
}

func (h *WireGuardHandler) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	out, err := h.wg.GetStatus(h.IfaceName)
	if err != nil {
		http.Error(w, fmt.Sprintf("wg error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(out))
}

// HandleMeshSummary parses `wg show <iface> dump` and returns peer health counts.
// A peer is healthy when its last handshake was within the past 3 minutes.
func (h *WireGuardHandler) HandleMeshSummary(w http.ResponseWriter, r *http.Request) {
	dump, err := h.wg.GetDump(h.IfaceName)
	if err != nil {
		// WireGuard not running — report just the local node
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"total": 1, "healthy": 1, "degraded": 0})
		return
	}

	// Start at 1 to include the local node itself.
	total, healthy := 1, 1
	now := time.Now().Unix()
	for i, line := range strings.Split(strings.TrimSpace(dump), "\n") {
		if i == 0 || line == "" {
			continue // first line is the interface row
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		total++
		ts, err := strconv.ParseInt(fields[4], 10, 64)
		if err == nil && ts > 0 && now-ts < 180 {
			healthy++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"total":    total,
		"healthy":  healthy,
		"degraded": total - healthy,
	})
}

type PeerStatus struct {
	PublicKey     string `json:"public_key"`
	NodeID        string `json:"node_id,omitempty"`
	NodeName      string `json:"node_name,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	AllowedIPs    string `json:"allowed_ips,omitempty"`
	LastHandshake int64  `json:"last_handshake"`
	RxBytes       int64  `json:"rx_bytes"`
	TxBytes       int64  `json:"tx_bytes"`
	Quality       string `json:"quality"` // "good" | "degraded" | "dead"
}

// HandlePeerStatus parses `wg show <iface> dump` and returns a structured per-peer
// status list, annotated with enrolled node names where the public key matches.
func (h *WireGuardHandler) HandlePeerStatus(w http.ResponseWriter, r *http.Request) {
	dump, err := h.wg.GetDump(h.IfaceName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]PeerStatus{})
		return
	}

	// Build a public-key → NodeRecord lookup from enrolled nodes.
	keyToNode := map[string]store.NodeRecord{}
	if h.nodeStore != nil {
		if nodes, err := h.nodeStore.ListNodes(r.Context()); err == nil {
			for _, n := range nodes {
				keyToNode[n.PublicKey] = n
			}
		}
	}

	now := time.Now().Unix()
	var peers []PeerStatus
	for i, line := range strings.Split(strings.TrimSpace(dump), "\n") {
		if i == 0 || line == "" {
			continue // first line is the interface row
		}
		// fields: pubkey  preshared  endpoint  allowed_ips  last_handshake  rx  tx  keepalive
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		pubKey := fields[0]
		endpoint := fields[2]
		if endpoint == "(none)" {
			endpoint = ""
		}
		allowedIPs := fields[3]

		lastHS, _ := strconv.ParseInt(fields[4], 10, 64)
		rx, _      := strconv.ParseInt(fields[5], 10, 64)
		tx, _      := strconv.ParseInt(fields[6], 10, 64)

		quality := "dead"
		if lastHS > 0 {
			age := now - lastHS
			if age < 180 {
				quality = "good"
			} else if age < 900 {
				quality = "degraded"
			}
		}

		p := PeerStatus{
			PublicKey:     pubKey,
			Endpoint:      endpoint,
			AllowedIPs:    allowedIPs,
			LastHandshake: lastHS,
			RxBytes:       rx,
			TxBytes:       tx,
			Quality:       quality,
		}
		if n, ok := keyToNode[pubKey]; ok {
			p.NodeID   = n.ID
			p.NodeName = n.Name
		}
		peers = append(peers, p)
	}

	if peers == nil {
		peers = []PeerStatus{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peers)
}

// SetEndpointStore attaches the store used by HandleRegisterEndpoint.
func (h *WireGuardHandler) SetEndpointStore(es wgEndpointStore) {
	h.endpointStore = es
}

// RegisterEndpointRequest is the body accepted by HandleRegisterEndpoint.
type RegisterEndpointRequest struct {
	PublicKey  string `json:"public_key"`
	ListenPort int    `json:"listen_port"`
}

// HandleRegisterEndpoint lets an edge node report its current WireGuard listen
// address. The hub derives the endpoint as "observed-source-IP:listen_port",
// updates the wg_peers table, and immediately applies it to the WireGuard peer
// config so reconnection happens without waiting for the next healer cycle.
func (h *WireGuardHandler) HandleRegisterEndpoint(w http.ResponseWriter, r *http.Request) {
	var req RegisterEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.PublicKey == "" || req.ListenPort <= 0 {
		http.Error(w, "public_key and listen_port are required", http.StatusBadRequest)
		return
	}
	if h.endpointStore == nil {
		http.Error(w, "endpoint store not configured", http.StatusInternalServerError)
		return
	}

	// Derive source IP — prefer explicit proxy headers over RemoteAddr.
	sourceIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "cannot determine source IP", http.StatusBadRequest)
		return
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		sourceIP = strings.TrimSpace(realIP)
	} else if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		sourceIP = strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
	}

	endpoint := fmt.Sprintf("%s:%d", sourceIP, req.ListenPort)

	node, err := h.endpointStore.GetNodeByPublicKey(r.Context(), req.PublicKey)
	if err != nil {
		http.Error(w, "node not found for public key", http.StatusNotFound)
		return
	}

	if err := h.endpointStore.UpdateWGPeerEndpoint(r.Context(), node.ID, endpoint); err != nil {
		http.Error(w, fmt.Sprintf("update endpoint: %v", err), http.StatusInternalServerError)
		return
	}

	// Apply immediately to the live WireGuard config so the tunnel resumes
	// without waiting for the healer.
	if err := h.wg.UpdatePeerEndpoint(h.IfaceName, req.PublicKey, endpoint); err != nil {
		if handlerLogger != nil {
			handlerLogger.Warn("register-endpoint: wg update failed (DB updated, healer will retry)",
				"node_id", node.ID, "endpoint", endpoint, "err", err)
		}
	}

	audit("ENDPOINT_REGISTERED", r, "node_id", node.ID, "public_key", req.PublicKey, "endpoint", endpoint)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"endpoint": endpoint})
}

func (h *WireGuardHandler) HandleGetHubKey(w http.ResponseWriter, r *http.Request) {
	var key []byte
	var err error

	if h.keyStore != nil {
		key, err = h.keyStore.GetSecret(r.Context(), "sync_hmac_key")
	} else {
		err = fmt.Errorf("no key store")
	}

	if err != nil && h.SecretsDir != "" {
		key, err = os.ReadFile(filepath.Join(h.SecretsDir, "sync_hmac.key"))
	}

	if err != nil || len(key) == 0 {
		http.Error(w, "hub key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"hmac_key": fmt.Sprintf("%x", key)})
}
