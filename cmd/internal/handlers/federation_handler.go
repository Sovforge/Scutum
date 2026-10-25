package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"
)

type federationStore interface {
	CreateFederationPeer(ctx context.Context, id, name, hubURL, wgEndpoint, wgPublicKey, meshCIDR, allowedIPs string) error
	ListFederationPeers(ctx context.Context) ([]store.FederationPeer, error)
	GetFederationPeer(ctx context.Context, id string) (store.FederationPeer, error)
	UpdateFederationPeerStatus(ctx context.Context, id, status string) error
	DeleteFederationPeer(ctx context.Context, id string) error
}

type FederationHandler struct {
	store  federationStore
	runner utils.CommandRunner
}

func NewFederationHandler(s federationStore) *FederationHandler {
	return &FederationHandler{store: s, runner: utils.DefaultCommandRunner}
}

type federationPeerRequest struct {
	Name        string `json:"name"`
	HubURL      string `json:"hub_url"`
	WGEndpoint  string `json:"wg_endpoint"`
	WGPublicKey string `json:"wg_public_key"`
	MeshCIDR    string `json:"mesh_cidr"`
	AllowedIPs  string `json:"allowed_ips"`
}

func (h *FederationHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	peers, err := h.store.ListFederationPeers(r.Context())
	if err != nil {
		http.Error(w, "failed to list federation peers", http.StatusInternalServerError)
		return
	}
	if peers == nil {
		peers = []store.FederationPeer{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peers)
}

func (h *FederationHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req federationPeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.WGEndpoint == "" || req.WGPublicKey == "" || req.MeshCIDR == "" {
		http.Error(w, "name, wg_endpoint, wg_public_key, and mesh_cidr are required", http.StatusBadRequest)
		return
	}
	allowedIPs := req.AllowedIPs
	if allowedIPs == "" {
		allowedIPs = req.MeshCIDR
	}
	id := uuid.New().String()
	if err := h.store.CreateFederationPeer(r.Context(), id, req.Name, req.HubURL,
		req.WGEndpoint, req.WGPublicKey, req.MeshCIDR, allowedIPs); err != nil {
		http.Error(w, "failed to create federation peer", http.StatusInternalServerError)
		return
	}

	// Add WireGuard peer for the federated hub
	h.runner.Output("wg", "set", "wg0",
		"peer", req.WGPublicKey,
		"endpoint", req.WGEndpoint,
		"allowed-ips", allowedIPs,
		"persistent-keepalive", "25")

	peer, _ := h.store.GetFederationPeer(r.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(peer)
}

func (h *FederationHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	peer, err := h.store.GetFederationPeer(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "peer not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peer)
}

func (h *FederationHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	peer, err := h.store.GetFederationPeer(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "peer not found", http.StatusNotFound)
		return
	}
	// Remove WireGuard peer
	h.runner.Output("wg", "set", "wg0", "peer", peer.WGPublicKey, "remove")

	if err := h.store.DeleteFederationPeer(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, "failed to delete peer", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
