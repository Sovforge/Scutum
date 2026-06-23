package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"scutum/cmd/internal/store"

	"github.com/google/uuid"
)

type alertStore interface {
	CreateAlertRule(ctx context.Context, r store.AlertRule) error
	ListAlertRules(ctx context.Context) ([]store.AlertRule, error)
	UpdateAlertRule(ctx context.Context, r store.AlertRule) error
	DeleteAlertRule(ctx context.Context, id string) error
	ListAlertEvents(ctx context.Context, limit int) ([]store.AlertEvent, error)
	AcknowledgeAlertEvent(ctx context.Context, eventID, ackedAt string) error
}

type AlertsHandler struct{ store alertStore }

func NewAlertsHandler(s alertStore) *AlertsHandler { return &AlertsHandler{store: s} }

// POST /api/alerts/rules
func (h *AlertsHandler) HandleCreateRule(w http.ResponseWriter, r *http.Request) {
	var body store.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if !validCondition(body.Condition) {
		http.Error(w, "invalid condition", http.StatusBadRequest)
		return
	}
	if !validSeverity(body.Severity) {
		body.Severity = "warning"
	}
	body.ID = uuid.New().String()
	body.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := h.store.CreateAlertRule(r.Context(), body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(body)
}

// GET /api/alerts/rules
func (h *AlertsHandler) HandleListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.store.ListAlertRules(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []store.AlertRule{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

// PUT /api/alerts/rules/{id}
func (h *AlertsHandler) HandleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body store.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	body.ID = id
	if !validCondition(body.Condition) {
		http.Error(w, "invalid condition", http.StatusBadRequest)
		return
	}
	if !validSeverity(body.Severity) {
		body.Severity = "warning"
	}
	if err := h.store.UpdateAlertRule(r.Context(), body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}

// DELETE /api/alerts/rules/{id}
func (h *AlertsHandler) HandleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteAlertRule(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/alerts/rules/{id}/silence  body: {"until":"2026-01-02T15:04:05Z"}
func (h *AlertsHandler) HandleSilenceRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Until string `json:"until"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	// Validate the timestamp.
	if _, err := time.Parse(time.RFC3339, body.Until); err != nil {
		http.Error(w, "until must be RFC3339", http.StatusBadRequest)
		return
	}

	rules, err := h.store.ListAlertRules(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var rule *store.AlertRule
	for i := range rules {
		if rules[i].ID == id {
			rule = &rules[i]
			break
		}
	}
	if rule == nil {
		http.Error(w, "rule not found", http.StatusNotFound)
		return
	}
	rule.SilencedUntil = body.Until
	if err := h.store.UpdateAlertRule(r.Context(), *rule); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// GET /api/alerts/events?limit=100
func (h *AlertsHandler) HandleListEvents(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	events, err := h.store.ListAlertEvents(r.Context(), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if events == nil {
		events = []store.AlertEvent{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// POST /api/alerts/events/{id}/acknowledge
func (h *AlertsHandler) HandleAcknowledgeEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ackedAt := time.Now().UTC().Format(time.RFC3339)
	if err := h.store.AcknowledgeAlertEvent(r.Context(), id, ackedAt); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validCondition(c string) bool {
	switch c {
	case "cpu_percent", "mem_percent", "disk_percent", "node_offline", "handshake_age":
		return true
	}
	return false
}

func validSeverity(s string) bool {
	switch s {
	case "info", "warning", "critical":
		return true
	}
	return false
}