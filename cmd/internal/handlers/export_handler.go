package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"
)

type exportStore interface {
	ListUsers(ctx context.Context) ([]store.UserRecord, error)
	ListRoles(ctx context.Context) ([]store.RoleRecord, error)
	ListNodes(ctx context.Context) ([]store.NodeRecord, error)
	ListEnabledPlugins(ctx context.Context) ([]store.PluginRecord, error)
	ListStorageBackends(ctx context.Context) ([]store.StorageBackend, error)
	ListWGPeers(ctx context.Context) ([]store.WGPeerRecord, error)
	ListAuditLogs(ctx context.Context, limit int) ([]utils.AuditEntry, error)
}

type ExportHandler struct {
	store exportStore
}

func NewExportHandler(s exportStore) *ExportHandler {
	return &ExportHandler{store: s}
}

type exportUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at,omitempty"`
}

type dbExport struct {
	ExportedAt      time.Time              `json:"exported_at"`
	Users           []exportUser           `json:"users"`
	Roles           []store.RoleRecord     `json:"roles"`
	Nodes           []store.NodeRecord     `json:"nodes"`
	Plugins         []store.PluginRecord   `json:"plugins"`
	StorageBackends []store.StorageBackend `json:"storage_backends"`
	WGPeers         []store.WGPeerRecord   `json:"wg_peers"`
	AuditLogs       []utils.AuditEntry     `json:"audit_logs"`
}

func (h *ExportHandler) HandleExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rawUsers, err := h.store.ListUsers(ctx)
	if err != nil {
		http.Error(w, "list users: "+err.Error(), http.StatusInternalServerError)
		return
	}
	users := make([]exportUser, len(rawUsers))
	for i, u := range rawUsers {
		users[i] = exportUser{ID: u.ID, Username: u.Username}
	}

	roles, err := h.store.ListRoles(ctx)
	if err != nil {
		http.Error(w, "list roles: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if roles == nil {
		roles = []store.RoleRecord{}
	}

	nodes, err := h.store.ListNodes(ctx)
	if err != nil {
		http.Error(w, "list nodes: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if nodes == nil {
		nodes = []store.NodeRecord{}
	}

	plugins, err := h.store.ListEnabledPlugins(ctx)
	if err != nil {
		http.Error(w, "list plugins: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if plugins == nil {
		plugins = []store.PluginRecord{}
	}

	backends, err := h.store.ListStorageBackends(ctx)
	if err != nil {
		http.Error(w, "list storage: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if backends == nil {
		backends = []store.StorageBackend{}
	}

	peers, err := h.store.ListWGPeers(ctx)
	if err != nil {
		http.Error(w, "list wg peers: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if peers == nil {
		peers = []store.WGPeerRecord{}
	}

	auditLogs, err := h.store.ListAuditLogs(ctx, 1000)
	if err != nil {
		auditLogs = utils.GetAuditEntries()
	}
	if auditLogs == nil {
		auditLogs = []utils.AuditEntry{}
	}

	payload := dbExport{
		ExportedAt:      time.Now().UTC(),
		Users:           users,
		Roles:           roles,
		Nodes:           nodes,
		Plugins:         plugins,
		StorageBackends: backends,
		WGPeers:         peers,
		AuditLogs:       auditLogs,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="scutum-export.json"`)
	json.NewEncoder(w).Encode(payload)
}
