package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"scutum/cmd/internal/store"

	"github.com/google/uuid"
)

type nodeStore interface {
	ListNodes(ctx context.Context) ([]store.NodeRecord, error)
	GetNode(ctx context.Context, id string) (store.NodeRecord, error)
	CreateNode(ctx context.Context, n store.NodeRecord) error
	DeleteNode(ctx context.Context, id string) error
}

type NodeHandler struct {
	store nodeStore
}

func NewNodeHandler(s nodeStore) *NodeHandler {
	return &NodeHandler{store: s}
}

type createNodeRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`       // hub | peer | edge
	Address   string `json:"address"`    // host:port
	PublicKey string `json:"public_key"` // WireGuard public key
}

func (h *NodeHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.store.ListNodes(r.Context())
	if err != nil {
		http.Error(w, "failed to list nodes", http.StatusInternalServerError)
		return
	}
	if nodes == nil {
		nodes = []store.NodeRecord{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

func (h *NodeHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	node, err := h.store.GetNode(r.Context(), id)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

func (h *NodeHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req createNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Type == "" || req.Address == "" || req.PublicKey == "" {
		http.Error(w, "name, type, address, and public_key are required", http.StatusBadRequest)
		return
	}
	switch req.Type {
	case "hub", "remote", "combined":
	default:
		http.Error(w, "type must be hub, remote, or combined", http.StatusBadRequest)
		return
	}

	node := store.NodeRecord{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Type:      req.Type,
		Address:   req.Address,
		PublicKey: req.PublicKey,
	}
	if err := h.store.CreateNode(r.Context(), node); err != nil {
		http.Error(w, "failed to create node", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(node)
}

func (h *NodeHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteNode(r.Context(), id); err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
