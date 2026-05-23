package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"

	"scutum/cmd/internal/utils"
)

type obsStore interface {
	ListAuditLogs(ctx context.Context, limit int) ([]utils.AuditEntry, error)
	ListSystemLogs(ctx context.Context, limit int) ([]utils.LogEntry, error)
	ListTraces(ctx context.Context, limit int) ([]utils.TraceEntry, error)
	ListMetrics(ctx context.Context, limit int, name, service string) ([]utils.MetricPoint, error)
}

type ObservabilityHandler struct {
	store     obsStore
	nodeStore nodeProxyStore
}

func NewObservabilityHandler(store obsStore, nodeStore nodeProxyStore) *ObservabilityHandler {
	return &ObservabilityHandler{store: store, nodeStore: nodeStore}
}

func (h *ObservabilityHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	limit := queryInt(r, "limit", 500)
	entries, err := h.store.ListSystemLogs(r.Context(), limit)
	if err != nil {
		// Fall back to ring buffer if DB query fails.
		entries = utils.GetLogEntries()
	}
	if entries == nil {
		entries = []utils.LogEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *ObservabilityHandler) HandleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	limit := queryInt(r, "limit", 1000)
	entries, err := h.store.ListAuditLogs(r.Context(), limit)
	if err != nil {
		entries = utils.GetAuditEntries()
	}
	if entries == nil {
		entries = []utils.AuditEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *ObservabilityHandler) HandleTraces(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	limit := queryInt(r, "limit", 500)
	entries, err := h.store.ListTraces(r.Context(), limit)
	if err != nil {
		entries = utils.GetTraceEntries()
	}
	if entries == nil {
		entries = []utils.TraceEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// HandleExportAuditLogs streams audit logs as a CSV download.
// GET /audit/logs/export?limit=5000&format=csv  (default) or format=json
func (h *ObservabilityHandler) HandleExportAuditLogs(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	limit := queryInt(r, "limit", 5000)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}

	entries, err := h.store.ListAuditLogs(r.Context(), limit)
	if err != nil {
		entries = utils.GetAuditEntries()
	}
	if entries == nil {
		entries = []utils.AuditEntry{}
	}

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=\"audit_logs.json\"")
		json.NewEncoder(w).Encode(entries)

	default: // csv
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=\"audit_logs.csv\"")
		cw := csv.NewWriter(w)
		cw.Write([]string{"time", "action", "method", "path", "trace_id", "client_ip", "extra"})
		for _, e := range entries {
			extra, _ := json.Marshal(e.Extra)
			cw.Write([]string{
				e.Time.Format("2006-01-02T15:04:05Z"),
				e.Action,
				e.Method,
				e.Path,
				e.TraceID,
				e.ClientIP,
				string(extra),
			})
		}
		cw.Flush()
	}
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

