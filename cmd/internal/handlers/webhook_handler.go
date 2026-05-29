package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/webhooks"
)

type webhookStore interface {
	CreateWebhook(ctx context.Context, id, name, url, secret string, events []string) error
	ListWebhooks(ctx context.Context) ([]store.WebhookConfig, error)
	GetWebhook(ctx context.Context, id string) (store.WebhookConfig, error)
	UpdateWebhook(ctx context.Context, id, name, url, secret string, events []string, enabled bool) error
	DeleteWebhook(ctx context.Context, id string) error
}

type WebhookHandler struct {
	store webhookStore
}

func NewWebhookHandler(s webhookStore) *WebhookHandler {
	return &WebhookHandler{store: s}
}

type webhookRequest struct {
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Secret  string   `json:"secret"`
	Events  []string `json:"events"`
	Enabled *bool    `json:"enabled"`
}

func (h *WebhookHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	hooks, err := h.store.ListWebhooks(r.Context())
	if err != nil {
		http.Error(w, "failed to list webhooks", http.StatusInternalServerError)
		return
	}
	if hooks == nil {
		hooks = []store.WebhookConfig{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hooks)
}

func (h *WebhookHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.URL == "" {
		http.Error(w, "name and url are required", http.StatusBadRequest)
		return
	}
	if len(req.Events) == 0 {
		http.Error(w, "events must not be empty", http.StatusBadRequest)
		return
	}
	id := uuid.New().String()
	if err := h.store.CreateWebhook(r.Context(), id, req.Name, req.URL, req.Secret, req.Events); err != nil {
		http.Error(w, "failed to create webhook", http.StatusInternalServerError)
		return
	}
	wh, _ := h.store.GetWebhook(r.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(wh)
}

func (h *WebhookHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	wh, err := h.store.GetWebhook(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "webhook not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wh)
}

func (h *WebhookHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	existing, err := h.store.GetWebhook(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "webhook not found", http.StatusNotFound)
		return
	}
	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		req.Name = existing.Name
	}
	if req.URL == "" {
		req.URL = existing.URL
	}
	if len(req.Events) == 0 {
		req.Events = existing.Events
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := h.store.UpdateWebhook(r.Context(), existing.ID, req.Name, req.URL, req.Secret, req.Events, enabled); err != nil {
		http.Error(w, "failed to update webhook", http.StatusInternalServerError)
		return
	}
	wh, _ := h.store.GetWebhook(r.Context(), existing.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wh)
}

func (h *WebhookHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if _, err := h.store.GetWebhook(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, "webhook not found", http.StatusNotFound)
		return
	}
	if err := h.store.DeleteWebhook(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, "failed to delete webhook", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WebhookHandler) HandleTest(w http.ResponseWriter, r *http.Request) {
	wh, err := h.store.GetWebhook(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "webhook not found", http.StatusNotFound)
		return
	}
	event := webhooks.Event{
		Type:      "webhook.test",
		Timestamp: time.Now().UTC(),
		Payload:   map[string]any{"message": "Test delivery from Scutum"},
	}
	body, _ := json.Marshal(event)
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "invalid webhook URL", http.StatusBadRequest)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scutum-Event", event.Type)
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "delivery failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, "endpoint returned non-2xx", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
