package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"scutum/cmd/internal/handlers"
)

func TestAuditForwarderStore_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.CreateAuditForwarder(ctx, "fwd1", "Elastic", "https://elastic.example.com/scutum", "json"); err != nil {
		t.Fatalf("CreateAuditForwarder: %v", err)
	}

	fwds, err := s.ListAuditForwarders(ctx)
	if err != nil {
		t.Fatalf("ListAuditForwarders: %v", err)
	}
	if len(fwds) != 1 {
		t.Fatalf("expected 1 forwarder, got %d", len(fwds))
	}
	if fwds[0].Name != "Elastic" {
		t.Errorf("Name = %q, want Elastic", fwds[0].Name)
	}
	if !fwds[0].Enabled {
		t.Error("expected Enabled true by default")
	}
}

func TestAuditForwarderHandler_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewAuditForwarderHandler(s)

	body, _ := json.Marshal(map[string]any{
		"name":   "Splunk",
		"url":    "https://splunk.example.com/hec",
		"format": "json",
	})
	req := httptest.NewRequest("POST", "/audit/forwarders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest("GET", "/audit/forwarders", nil)
	w2 := httptest.NewRecorder()
	h.HandleList(w2, req2)

	var list []map[string]any
	json.Unmarshal(w2.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("expected 1 forwarder, got %d", len(list))
	}
}

func TestAuditForwarderHandler_ToggleEnabled(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.CreateAuditForwarder(ctx, "fwd1", "Test", "https://example.com", "json")

	h := handlers.NewAuditForwarderHandler(s)
	enabled := false
	body, _ := json.Marshal(map[string]any{"enabled": enabled})
	req := httptest.NewRequest("PUT", "/audit/forwarders/fwd1", bytes.NewReader(body))
	req.SetPathValue("id", "fwd1")
	w := httptest.NewRecorder()
	h.HandleUpdate(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", result["enabled"])
	}
}
