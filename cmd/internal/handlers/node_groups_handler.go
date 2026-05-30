package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"scutum/cmd/internal/store"
)

type nodeGroupStore interface {
	SetNodeLabels(ctx context.Context, nodeID string, labels map[string]string) error
	GetNodeLabels(ctx context.Context, nodeID string) (map[string]string, error)
	CreateNodeGroup(ctx context.Context, id, name, description string) error
	ListNodeGroups(ctx context.Context) ([]store.NodeGroup, error)
	GetNodeGroup(ctx context.Context, id string) (store.NodeGroup, error)
	UpdateNodeGroup(ctx context.Context, id, name, description string) error
	DeleteNodeGroup(ctx context.Context, id string) error
	AddNodeToGroup(ctx context.Context, groupID, nodeID string) error
	RemoveNodeFromGroup(ctx context.Context, groupID, nodeID string) error
	ListNodesInGroup(ctx context.Context, groupID string) ([]store.NodeRecord, error)
}

type NodeGroupsHandler struct {
	store nodeGroupStore
}

func NewNodeGroupsHandler(s nodeGroupStore) *NodeGroupsHandler {
	return &NodeGroupsHandler{store: s}
}

// ── Labels ───────────────────────────────────────────────────────────────────

func (h *NodeGroupsHandler) HandleGetLabels(w http.ResponseWriter, r *http.Request) {
	labels, err := h.store.GetNodeLabels(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "failed to get labels", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(labels)
}

func (h *NodeGroupsHandler) HandleSetLabels(w http.ResponseWriter, r *http.Request) {
	var labels map[string]string
	if err := json.NewDecoder(r.Body).Decode(&labels); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := h.store.SetNodeLabels(r.Context(), r.PathValue("id"), labels); err != nil {
		http.Error(w, "failed to set labels", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(labels)
}

// ── Groups ───────────────────────────────────────────────────────────────────

func (h *NodeGroupsHandler) HandleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListNodeGroups(r.Context())
	if err != nil {
		http.Error(w, "failed to list groups", http.StatusInternalServerError)
		return
	}
	if groups == nil {
		groups = []store.NodeGroup{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

func (h *NodeGroupsHandler) HandleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	id := uuid.New().String()
	if err := h.store.CreateNodeGroup(r.Context(), id, req.Name, req.Description); err != nil {
		http.Error(w, "failed to create group", http.StatusInternalServerError)
		return
	}
	g, _ := h.store.GetNodeGroup(r.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(g)
}

func (h *NodeGroupsHandler) HandleGetGroup(w http.ResponseWriter, r *http.Request) {
	g, err := h.store.GetNodeGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}

func (h *NodeGroupsHandler) HandleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	existing, err := h.store.GetNodeGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		req.Name = existing.Name
	}
	if err := h.store.UpdateNodeGroup(r.Context(), existing.ID, req.Name, req.Description); err != nil {
		http.Error(w, "failed to update group", http.StatusInternalServerError)
		return
	}
	g, _ := h.store.GetNodeGroup(r.Context(), existing.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g)
}

func (h *NodeGroupsHandler) HandleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if _, err := h.store.GetNodeGroup(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}
	if err := h.store.DeleteNodeGroup(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, "failed to delete group", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NodeGroupsHandler) HandleListGroupNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.store.ListNodesInGroup(r.Context(), r.PathValue("id"))
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

func (h *NodeGroupsHandler) HandleAddMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NodeID == "" {
		http.Error(w, "node_id is required", http.StatusBadRequest)
		return
	}
	if err := h.store.AddNodeToGroup(r.Context(), r.PathValue("id"), req.NodeID); err != nil {
		http.Error(w, "failed to add member", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NodeGroupsHandler) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	if err := h.store.RemoveNodeFromGroup(r.Context(), r.PathValue("id"), r.PathValue("nodeId")); err != nil {
		http.Error(w, "failed to remove member", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
