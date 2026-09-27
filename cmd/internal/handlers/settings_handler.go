package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"
)

type settingsStore interface {
	GetSystemSettings(ctx context.Context) (store.SystemSettings, error)
	UpdateSystemSettings(ctx context.Context, cfg store.SystemSettings) error
}

type SettingsHandler struct {
	store settingsStore
}

func NewSettingsHandler(s settingsStore) *SettingsHandler {
	return &SettingsHandler{store: s}
}

func (h *SettingsHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.GetSystemSettings(r.Context())
	if err != nil {
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func parseLogLevel(level string) (slog.Level, bool) {
	switch level {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

func (h *SettingsHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	var req store.SystemSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.LogLevel == "" {
		req.LogLevel = "info"
	}
	if _, ok := parseLogLevel(req.LogLevel); !ok {
		http.Error(w, "log_level must be one of: debug, info, warn, error", http.StatusBadRequest)
		return
	}
	if req.NodeDefaultRole != "hub" && req.NodeDefaultRole != "remote" {
		http.Error(w, "node_default_role must be hub or remote", http.StatusBadRequest)
		return
	}
	if req.MeshMTU < 576 || req.MeshMTU > 9000 {
		http.Error(w, "mesh_mtu must be between 576 and 9000", http.StatusBadRequest)
		return
	}
	if req.MeshKeepaliveSeconds < 0 || req.MeshKeepaliveSeconds > 3600 {
		http.Error(w, "mesh_keepalive_s must be between 0 and 3600", http.StatusBadRequest)
		return
	}
	if req.AuthSessionTimeoutMin < 5 || req.AuthSessionTimeoutMin > 43200 {
		http.Error(w, "auth_session_timeout_min must be between 5 and 43200 (30 days)", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateSystemSettings(r.Context(), req); err != nil {
		http.Error(w, "failed to save settings", http.StatusInternalServerError)
		return
	}

	// Real side effects, applied immediately rather than only on next
	// restart. Both are best-effort: log level always has something sane to
	// fall back to, and MTU only matters once wg0 is actually up (setup
	// hasn't necessarily run yet, or WireGuard may be unavailable on this
	// host entirely — neither is an error worth failing the save over).
	if level, ok := parseLogLevel(req.LogLevel); ok {
		if logger := GetLogger(); logger != nil {
			logger.SetLevel(level)
		}
	}
	if err := utils.SetInterfaceMTU("wg0", req.MeshMTU); err != nil {
		base := NewBaseHandler(nil)
		base.Audit("SETTINGS_MTU_APPLY_FAILED", r, "error", err.Error())
	}

	base := NewBaseHandler(nil)
	base.Audit("SETTINGS_UPDATED", r)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}
