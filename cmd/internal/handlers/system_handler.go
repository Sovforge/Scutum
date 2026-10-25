package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"scutum/cmd/internal/store"
	"scutum/cmd/internal/sysinfo"
)

type statsStore interface {
	SaveNodeStats(ctx context.Context, r store.NodeStatRecord) error
	LatestNodeStats(ctx context.Context) (store.NodeStatRecord, error)
	ListNodeStatsHistory(ctx context.Context, limit int) ([]store.NodeStatRecord, error)
}

type SystemHandler struct {
	stats statsStore
	nodes nodeProxyStore
}

func NewSystemHandler(stats statsStore, nodes nodeProxyStore) *SystemHandler {
	return &SystemHandler{stats: stats, nodes: nodes}
}

type tlsModeResponse struct {
	Mode     string `json:"mode"`              // "acme" | "manual" | "none"
	Domain   string `json:"domain,omitempty"`  // ACME only
	Email    string `json:"email,omitempty"`   // ACME only
	Staging  bool   `json:"staging,omitempty"` // ACME only
	CertFile string `json:"cert_file,omitempty"` // manual only
}

func (h *SystemHandler) HandleTLSMode(w http.ResponseWriter, r *http.Request) {
	resp := tlsModeResponse{Mode: "none"}

	if domain := os.Getenv("ACME_DOMAIN"); domain != "" && os.Getenv("ACME_EMAIL") != "" {
		resp.Mode = "acme"
		resp.Domain = domain
		resp.Email = os.Getenv("ACME_EMAIL")
		resp.Staging = os.Getenv("ACME_STAGING") == "true"
	} else if certFile := os.Getenv("CERT_FILE"); certFile != "" {
		if _, err := os.Stat(certFile); err == nil {
			resp.Mode = "manual"
			resp.CertFile = certFile
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleSystemStats returns a live stats snapshot for this node.
// If X-Target-Node is set, the request is proxied to the remote node.
func (h *SystemHandler) HandleSystemStats(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodes) {
		return
	}
	s, err := sysinfo.Collect()
	if err != nil {
		http.Error(w, `{"error":"failed to collect stats"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

// HandleSystemStatsHistory returns stored stats history (newest first, up to 1440 points = 24h at 1/min).
// If X-Target-Node is set, the request is proxied to the remote node.
func (h *SystemHandler) HandleSystemStatsHistory(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodes) {
		return
	}
	limit := queryInt(r, "limit", 1440)
	history, err := h.stats.ListNodeStatsHistory(r.Context(), limit)
	if err != nil || history == nil {
		history = []store.NodeStatRecord{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}
