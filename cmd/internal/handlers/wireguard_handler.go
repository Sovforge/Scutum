package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/sync"
	"scutum/cmd/internal/wireguard"
)

type WireGuardHandler struct {
	IfaceName string
	wg        wireguard.Service
	healer    *sync.Healer
}

func NewWireGuardHandler(ifaceName string, wg wireguard.Service, healer *sync.Healer) *WireGuardHandler {
	return &WireGuardHandler{
		IfaceName: ifaceName,
		wg:        wg,
		healer:    healer,
	}
}

type PeerRequest struct {
	PublicKey           string `json:"public_key"`
	Endpoint            string `json:"endpoint"`
	AllowedIPs          string `json:"allowed_ips"` // comma-separated; caller's responsibility
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
