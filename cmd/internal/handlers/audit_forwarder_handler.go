package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"
)

type auditForwarderStore interface {
	CreateAuditForwarder(ctx context.Context, id, name, url, format string) error
	ListAuditForwarders(ctx context.Context) ([]store.AuditForwarder, error)
	GetAuditForwarder(ctx context.Context, id string) (store.AuditForwarder, error)
	UpdateAuditForwarder(ctx context.Context, id, name, url, format string, enabled bool) error
	DeleteAuditForwarder(ctx context.Context, id string) error
	ListEnabledAuditForwarders(ctx context.Context) ([]store.AuditForwarder, error)
	ListAuditLogs(ctx context.Context, limit int) ([]utils.AuditEntry, error)
}

type AuditForwarderHandler struct {
	store auditForwarderStore
}

func NewAuditForwarderHandler(s auditForwarderStore) *AuditForwarderHandler {
	return &AuditForwarderHandler{store: s}
}

func (h *AuditForwarderHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	fwds, err := h.store.ListAuditForwarders(r.Context())
	if err != nil {
		http.Error(w, "failed to list forwarders", http.StatusInternalServerError)
		return
	}
	if fwds == nil {
		fwds = []store.AuditForwarder{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fwds)
}

func (h *AuditForwarderHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		URL    string `json:"url"`
		Format string `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.URL == "" {
		http.Error(w, "name and url are required", http.StatusBadRequest)
		return
	}
	if req.Format == "" {
		req.Format = "json"
	}
	if req.Format != "json" && req.Format != "cef" {
		http.Error(w, "format must be json or cef", http.StatusBadRequest)
		return
	}
	id := uuid.New().String()
	if err := h.store.CreateAuditForwarder(r.Context(), id, req.Name, req.URL, req.Format); err != nil {
		http.Error(w, "failed to create forwarder", http.StatusInternalServerError)
		return
	}
	fwd, _ := h.store.GetAuditForwarder(r.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fwd)
}

func (h *AuditForwarderHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	fwd, err := h.store.GetAuditForwarder(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "forwarder not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fwd)
}

func (h *AuditForwarderHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	existing, err := h.store.GetAuditForwarder(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "forwarder not found", http.StatusNotFound)
		return
	}
	var req struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		Format  string `json:"format"`
		Enabled *bool  `json:"enabled"`
	}
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
	if req.Format == "" {
		req.Format = existing.Format
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := h.store.UpdateAuditForwarder(r.Context(), existing.ID, req.Name, req.URL, req.Format, enabled); err != nil {
		http.Error(w, "failed to update forwarder", http.StatusInternalServerError)
		return
	}
	fwd, _ := h.store.GetAuditForwarder(r.Context(), existing.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fwd)
}

func (h *AuditForwarderHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if _, err := h.store.GetAuditForwarder(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, "forwarder not found", http.StatusNotFound)
		return
	}
	if err := h.store.DeleteAuditForwarder(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, "failed to delete forwarder", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RunForwarder polls audit logs every 30 seconds and ships them to all enabled forwarders.
// Intended to run as a long-lived goroutine; stops when ctx is cancelled.
func RunForwarder(ctx context.Context, s auditForwarderStore) {
	client := &http.Client{Timeout: 10 * time.Second}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			forwardAuditLogs(ctx, s, client)
		}
	}
}

func forwardAuditLogs(ctx context.Context, s auditForwarderStore, client *http.Client) {
	fwds, err := s.ListEnabledAuditForwarders(ctx)
	if err != nil || len(fwds) == 0 {
		return
	}
	entries, err := s.ListAuditLogs(ctx, 500)
	if err != nil || len(entries) == 0 {
		return
	}
	for _, fwd := range fwds {
		var body []byte
		if fwd.Format == "cef" {
			body = encodeCEF(entries)
		} else {
			body, _ = json.Marshal(map[string]any{
				"forwarder": fwd.Name,
				"timestamp": time.Now().UTC(),
				"events":    entries,
			})
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fwd.URL, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}

func encodeCEF(entries []utils.AuditEntry) []byte {
	var buf bytes.Buffer
	for _, e := range entries {
		outcome := e.Outcome
		if outcome == "" {
			outcome = "success"
		}
		line := fmt.Sprintf("CEF:0|Scutum|Scutum|1.0|%s|%s|5|src=%s suser=%s outcome=%s requestMethod=%s request=%s\n",
			e.Action, e.Action, e.ClientIP, e.Actor, outcome, e.Method, e.Path)
		buf.WriteString(line)
	}
	return buf.Bytes()
}
