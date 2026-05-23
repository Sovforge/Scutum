package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"scutum/cmd/internal/utils"
)

type otelStore interface {
	PersistTrace(e utils.TraceEntry)
	PersistLog(e utils.LogEntry)
	PersistMetric(e utils.MetricPoint)
	ListTraces(ctx context.Context, limit int) ([]utils.TraceEntry, error)
	ListMetrics(ctx context.Context, limit int, name, service string) ([]utils.MetricPoint, error)
}

// OTelHandler receives OTLP HTTP/JSON payloads (traces, logs, metrics) from
// instrumented services and stores them alongside Scutum's own telemetry.
type OTelHandler struct {
	store     otelStore
	nodeStore nodeProxyStore
}

func NewOTelHandler(store otelStore, nodeStore nodeProxyStore) *OTelHandler {
	return &OTelHandler{store: store, nodeStore: nodeStore}
}

// HandleOTLPTraces handles POST /otlp/v1/traces
func (h *OTelHandler) HandleOTLPTraces(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	spans, err := utils.ParseOTLPTraces(body)
	if err != nil {
		http.Error(w, "invalid OTLP traces payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	for _, s := range spans {
		utils.AppendSpan(s)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"accepted": len(spans)})
}

// HandleOTLPLogs handles POST /otlp/v1/logs
func (h *OTelHandler) HandleOTLPLogs(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	entries, err := utils.ParseOTLPLogs(body)
	if err != nil {
		http.Error(w, "invalid OTLP logs payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	for _, e := range entries {
		utils.AppendExternalLog(e)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"accepted": len(entries)})
}

// HandleOTLPMetrics handles POST /otlp/v1/metrics
func (h *OTelHandler) HandleOTLPMetrics(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	points, err := utils.ParseOTLPMetrics(body)
	if err != nil {
		http.Error(w, "invalid OTLP metrics payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	for _, p := range points {
		utils.AppendMetric(p)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"accepted": len(points)})
}

// HandleListMetrics handles GET /observability/metrics
func (h *OTelHandler) HandleListMetrics(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	limit := queryInt(r, "limit", 500)
	name := r.URL.Query().Get("name")
	service := r.URL.Query().Get("service")

	points, err := h.store.ListMetrics(r.Context(), limit, name, service)
	if err != nil {
		points = []utils.MetricPoint{}
	}
	if points == nil {
		points = []utils.MetricPoint{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}
