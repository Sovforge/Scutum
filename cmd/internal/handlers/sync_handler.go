package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"

	"scutum/cmd/internal/store"
	"scutum/cmd/internal/sync"
)

type SyncHandler struct {
	db        *store.Store
	pusher    *sync.Pusher
	tlsConfig *tls.Config
}

func NewSyncHandler(db *store.Store, pusher *sync.Pusher, tlsConfig *tls.Config) *SyncHandler {
	return &SyncHandler{db: db, pusher: pusher, tlsConfig: tlsConfig}
}

type RegisterEdgeRequest struct {
	NodeID  string `json:"node_id"`
	URL     string `json:"url"`
	Token   string `json:"token"`
}

func (h *SyncHandler) HandleRegisterEdge(w http.ResponseWriter, r *http.Request) {
	var req RegisterEdgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.NodeID == "" || req.URL == "" {
		http.Error(w, "node_id and url are required", http.StatusBadRequest)
		return
	}

	sink := sync.NewHTTPEdgeSink(req.NodeID, req.URL, req.Token, h.tlsConfig)
	h.pusher.Register(sink)

	if req.Token != "" {
		h.db.SetSecret(context.Background(), "edge_token_"+req.NodeID, []byte(req.Token))
	}

	base := NewBaseHandler(nil)
	base.Audit("EDGE_REGISTERED", r, "node_id", req.NodeID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}

func (h *SyncHandler) HandlePush(w http.ResponseWriter, r *http.Request) {
	base := NewBaseHandler(nil)
	base.Audit("SYNC_PUSH", r)

	ctx := r.Context()

	peers, err := h.db.ListWGPeers(ctx)
	if err != nil {
		base.Audit("SYNC_PUSH_FAILED", r, "error", err.Error())
		http.Error(w, "failed to list peers", http.StatusInternalServerError)
		return
	}

	plugins, err := h.db.ListEnabledPlugins(ctx)
	if err != nil {
		http.Error(w, "failed to list plugins", http.StatusInternalServerError)
		return
	}

	peerEntries := make([]sync.PeerEntry, len(peers))
	for i, p := range peers {
		peerEntries[i] = sync.PeerEntry{
			PublicKey:  p.NodeID,
			Endpoint:   p.Endpoint,
			AllowedIPs: p.AllowedIPs,
		}
	}

	pluginNames := make([]string, len(plugins))
	for i, p := range plugins {
		pluginNames[i] = p.Name
	}

	payload := sync.SyncPayload{
		Version:  sync.NewVersion(),
		Peers:   peerEntries,
		Plugins: pluginNames,
	}

	results := h.pusher.Push(ctx, payload)

	successCount := 0
	failedCount := 0
	for _, result := range results {
		if result.Err != nil {
			failedCount++
		} else {
			successCount++
		}
	}

	base.Audit("SYNC_PUSH_COMPLETED", r,
		"success", successCount,
		"failed", failedCount)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{
		"success": successCount,
		"failed":  failedCount,
		"total":   len(results),
	})
}
