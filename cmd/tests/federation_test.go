package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"scutum/cmd/internal/handlers"
)

func TestFederationStore_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.CreateFederationPeer(ctx, "p1", "hub-b",
		"https://hub-b.example.com", "203.0.113.10:51820",
		"pubkey123", "10.200.0.0/24", "10.200.0.0/24")
	if err != nil {
		t.Fatalf("CreateFederationPeer: %v", err)
	}

	peers, err := s.ListFederationPeers(ctx)
	if err != nil {
		t.Fatalf("ListFederationPeers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].Name != "hub-b" {
		t.Errorf("Name = %q, want hub-b", peers[0].Name)
	}
	if peers[0].Status != "pending" {
		t.Errorf("Status = %q, want pending", peers[0].Status)
	}
}

func TestFederationStore_UpdateStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateFederationPeer(ctx, "p1", "hub-b", "", "10.0.0.1:51820", "pubkey", "10.1.0.0/24", "")
	if err := s.UpdateFederationPeerStatus(ctx, "p1", "connected"); err != nil {
		t.Fatalf("UpdateFederationPeerStatus: %v", err)
	}

	p, err := s.GetFederationPeer(ctx, "p1")
	if err != nil {
		t.Fatalf("GetFederationPeer: %v", err)
	}
	if p.Status != "connected" {
		t.Errorf("Status = %q, want connected", p.Status)
	}
}

func TestFederationStore_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateFederationPeer(ctx, "p1", "hub-b", "", "10.0.0.1:51820", "pubkey", "10.1.0.0/24", "")
	if err := s.DeleteFederationPeer(ctx, "p1"); err != nil {
		t.Fatalf("DeleteFederationPeer: %v", err)
	}

	peers, _ := s.ListFederationPeers(ctx)
	if len(peers) != 0 {
		t.Errorf("expected 0 peers after delete, got %d", len(peers))
	}
}

func TestFederationHandler_ListEmpty(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewFederationHandler(s)

	req := httptest.NewRequest("GET", "/federation/peers", nil)
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

func TestFederationHandler_CreateValidation(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewFederationHandler(s)

	body, _ := json.Marshal(map[string]any{
		"name":       "hub-b",
		"wg_endpoint": "10.0.0.1:51820",
		// wg_public_key missing — should fail
		"mesh_cidr": "10.1.0.0/24",
	})
	req := httptest.NewRequest("POST", "/federation/peers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400 for missing wg_public_key, got %d", w.Code)
	}
}
