package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/webhooks"
)

func TestWebhookStore_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.CreateWebhook(ctx, "id1", "Slack", "https://example.com", "secret", []string{"node.enrolled"}); err != nil {
		t.Fatalf("CreateWebhook: %v", err)
	}

	hooks, err := s.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(hooks))
	}
	if hooks[0].Name != "Slack" {
		t.Errorf("Name = %q, want Slack", hooks[0].Name)
	}
	if len(hooks[0].Events) != 1 || hooks[0].Events[0] != "node.enrolled" {
		t.Errorf("unexpected events: %v", hooks[0].Events)
	}
}

func TestWebhookStore_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateWebhook(ctx, "id1", "Hook", "https://example.com", "", []string{"*"})
	if err := s.DeleteWebhook(ctx, "id1"); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}
	hooks, _ := s.ListWebhooks(ctx)
	if len(hooks) != 0 {
		t.Errorf("expected 0 webhooks after delete, got %d", len(hooks))
	}
}

func TestWebhookHandler_ListEmpty(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	req := httptest.NewRequest("GET", "/webhooks", nil)
	w := httptest.NewRecorder()
	h.HandleList(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result []any
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

func TestWebhookHandler_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	body, _ := json.Marshal(map[string]any{
		"name":   "Test Hook",
		"url":    "https://example.com/hook",
		"events": []string{"node.enrolled", "node.offline"},
	})
	req := httptest.NewRequest("POST", "/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest("GET", "/webhooks", nil)
	w2 := httptest.NewRecorder()
	h.HandleList(w2, req2)

	var list []map[string]any
	json.Unmarshal(w2.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(list))
	}
	if list[0]["name"] != "Test Hook" {
		t.Errorf("unexpected name: %v", list[0]["name"])
	}
}

func TestWebhookHandler_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.CreateWebhook(ctx, "wh1", "Hook", "https://example.com", "", []string{"*"})

	h := handlers.NewWebhookHandler(s)
	req := httptest.NewRequest("DELETE", "/webhooks/wh1", nil)
	req.SetPathValue("id", "wh1")
	w := httptest.NewRecorder()
	h.HandleDelete(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestWebhookDispatcher_SendNonBlocking(t *testing.T) {
	s := newTestStore(t)
	d := webhooks.NewDispatcher(s)
	// Fill the channel buffer then send one more — must not block
	done := make(chan struct{})
	go func() {
		for range 300 {
			d.Send(webhooks.Event{Type: webhooks.EventNodeEnrolled, Payload: map[string]any{}})
		}
		close(done)
	}()
	select {
	case <-done:
	default:
		// completed synchronously — also fine
	}
	<-done
}
