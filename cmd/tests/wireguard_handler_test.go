package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/sync"
)

// TestWireGuardHandlerAddPeer
func TestWireGuardHandlerAddPeer(t *testing.T) {
	tests := []struct {
		name           string
		ifaceName      string
		requestBody    interface{}
		expectedStatus int
		mockErr        error
	}{
		{
			name:           "invalid JSON",
			ifaceName:      "wg0",
			requestBody:    "invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "missing public_key",
			ifaceName: "wg0",
			requestBody: map[string]interface{}{
				"endpoint":    "192.168.1.100:51820",
				"allowed_ips": "10.0.0.2/32",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "valid peer request",
			ifaceName: "wg0",
			requestBody: map[string]interface{}{
				"public_key":  "pubkey123",
				"endpoint":    "192.168.1.100:51820",
				"allowed_ips": "10.0.0.2/32",
			},
			expectedStatus: http.StatusOK,
			mockErr:        nil,
		},
		{
			name:      "wg failure",
			ifaceName: "wg0",
			requestBody: map[string]interface{}{
				"public_key":  "pubkey123",
				"endpoint":    "192.168.1.100:51820",
				"allowed_ips": "10.0.0.2/32",
			},
			expectedStatus: http.StatusInternalServerError,
			mockErr:        errors.New("wg failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)

			wg := &mockWG{addPeerErr: tt.mockErr}
			healer := sync.NewHealer(sync.HealerConfig{}, &sync.DefaultWGChecker{})
			handler := handlers.NewWireGuardHandler(tt.ifaceName, wg, healer)

			req := httptest.NewRequest(http.MethodPost, "/wireguard/peers/add", bytes.NewReader(body))
			rw := httptest.NewRecorder()

			handler.HandleAddPeer(rw, req)

			if rw.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, rw.Code, rw.Body.String())
			}
		})
	}
}

// TestWireGuardHandlerGetStatus
func TestWireGuardHandlerGetStatus(t *testing.T) {
	tests := []struct {
		name           string
		ifaceName      string
		expectedStatus int
		mockErr        error
	}{
		{
			name:           "success",
			ifaceName:      "wg0",
			expectedStatus: http.StatusOK,
			mockErr:        nil,
		},
		{
			name:           "wg failure",
			ifaceName:      "wg0",
			expectedStatus: http.StatusInternalServerError,
			mockErr:        errors.New("fail"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wg := &mockWG{
				status:    "mock status",
				statusErr: tt.mockErr,
			}

			healer := sync.NewHealer(sync.HealerConfig{}, &sync.DefaultWGChecker{})
			handler := handlers.NewWireGuardHandler(tt.ifaceName, wg, healer)

			req := httptest.NewRequest(http.MethodGet, "/wireguard/status", nil)
			rw := httptest.NewRecorder()

			handler.HandleGetStatus(rw, req)

			if rw.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rw.Code)
			}

			if rw.Code == http.StatusOK && rw.Header().Get("Content-Type") != "text/plain" {
				t.Errorf("Expected text/plain Content-Type, got %s", rw.Header().Get("Content-Type"))
			}
		})
	}
}

// TestWireGuardHandlerPeerRequestValidation
func TestWireGuardHandlerPeerRequestValidation(t *testing.T) {
	tests := []struct {
		name              string
		publicKey         string
		endpoint          string
		allowedIPs        string
		expectedToBeValid bool
	}{
		{"valid", "key", "1.1.1.1:51820", "10.0.0.1/32", true},
		{"missing key", "", "1.1.1.1:51820", "10.0.0.1/32", false},
		{"missing endpoint", "key", "", "10.0.0.1/32", false},
		{"missing ips", "key", "1.1.1.1:51820", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"public_key":  tt.publicKey,
				"endpoint":    tt.endpoint,
				"allowed_ips": tt.allowedIPs,
			})

			healer := sync.NewHealer(sync.HealerConfig{}, &sync.DefaultWGChecker{})
			handler := handlers.NewWireGuardHandler("wg0", &mockWG{}, healer)

			req := httptest.NewRequest(http.MethodPost, "/wireguard/peers/add", bytes.NewReader(body))
			rw := httptest.NewRecorder()

			handler.HandleAddPeer(rw, req)

			if tt.expectedToBeValid && rw.Code == http.StatusBadRequest {
				t.Errorf("Valid request rejected")
			}

			if !tt.expectedToBeValid && rw.Code != http.StatusBadRequest {
				t.Errorf("Invalid request not rejected, got %d", rw.Code)
			}
		})
	}
}

// TestWireGuardHandlerInterfaceNames
func TestWireGuardHandlerInterfaceNames(t *testing.T) {
	healer := sync.NewHealer(sync.HealerConfig{}, &sync.DefaultWGChecker{})
	handler := handlers.NewWireGuardHandler("wg-test", &mockWG{}, healer)

	if handler.IfaceName != "wg-test" {
		t.Errorf("Expected interface name wg-test, got %s", handler.IfaceName)
	}
}

// TestWireGuardHandlerIPValidation
func TestWireGuardHandlerIPValidation(t *testing.T) {
	healer := sync.NewHealer(sync.HealerConfig{}, &sync.DefaultWGChecker{})
	handler := handlers.NewWireGuardHandler("wg0", &mockWG{}, healer)

	body, _ := json.Marshal(map[string]interface{}{
		"public_key":  "key",
		"endpoint":    "1.1.1.1:51820",
		"allowed_ips": "not-an-ip",
	})

	req := httptest.NewRequest(http.MethodPost, "/wireguard/peers/add", bytes.NewReader(body))
	rw := httptest.NewRecorder()

	handler.HandleAddPeer(rw, req)

	if rw.Code == http.StatusBadRequest && rw.Body.String() == "invalid JSON" {
		t.Errorf("Unexpected JSON failure")
	}
}

func TestHandleMeshSummary(t *testing.T) {
	now := time.Now().Unix()
	// Mock dump output: interface line + 2 peers (one healthy, one stale)
	dump := fmt.Sprintf("wg0\tprivate\tpublic\t51820\n"+
		"pub1\tpsk1\tend1\t10.0.0.1/32\t%d\t100\t200\t0\n"+ // healthy
		"pub2\tpsk2\tend2\t10.0.0.2/32\t%d\t100\t200\t0\n", // stale
		now-10, // 10s ago
		now-500, // 500s ago (>180s)
	)

	wg := &mockWG{dump: dump}
	healer := sync.NewHealer(sync.HealerConfig{}, &sync.DefaultWGChecker{})
	h := handlers.NewWireGuardHandler("wg0", wg, healer)

	req := httptest.NewRequest(http.MethodGet, "/wireguard/mesh-summary", nil)
	w := httptest.NewRecorder()
	h.HandleMeshSummary(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res map[string]int
	json.NewDecoder(w.Body).Decode(&res)
	// local node + 2 peers (one healthy, one stale)
	if res["total"] != 3 {
		t.Errorf("expected 3 total, got %d", res["total"])
	}
	if res["healthy"] != 2 {
		t.Errorf("expected 2 healthy, got %d", res["healthy"])
	}
	if res["degraded"] != 1 {
		t.Errorf("expected 1 degraded, got %d", res["degraded"])
	}
}

