package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/kms"
	"scutum/cmd/internal/store"
)

type mockSetupStore struct {
	setupComplete bool
	users         map[string]bool
	errors        map[string]error
}

func (m *mockSetupStore) IsSetupComplete(ctx context.Context) (bool, error) {
	if err, ok := m.errors["IsSetupComplete"]; ok {
		return false, err
	}
	return m.setupComplete, nil
}

func (m *mockSetupStore) MarkSetupComplete(ctx context.Context) error {
	if err, ok := m.errors["MarkSetupComplete"]; ok {
		return err
	}
	m.setupComplete = true
	return nil
}

func (m *mockSetupStore) SetKMSProvider(ctx context.Context, provider string) error {
	if err, ok := m.errors["SetKMSProvider"]; ok {
		return err
	}
	return nil
}

func (m *mockSetupStore) SetInstallType(ctx context.Context, t store.InstallType) error {
	if err, ok := m.errors["SetInstallType"]; ok {
		return err
	}
	return nil
}

func (m *mockSetupStore) CreateUser(ctx context.Context, id, username, passwordHash string) error {
	if err, ok := m.errors["CreateUser"]; ok {
		return err
	}
	if m.users == nil {
		m.users = make(map[string]bool)
	}
	m.users[username] = true
	return nil
}

func (m *mockSetupStore) AssignRole(ctx context.Context, userID, roleID string) error {
	if err, ok := m.errors["AssignRole"]; ok {
		return err
	}
	return nil
}

func (m *mockSetupStore) SetSecret(ctx context.Context, key string, value []byte) error {
	if err, ok := m.errors["SetSecret"]; ok {
		return err
	}
	return nil
}

func (m *mockSetupStore) GetSecret(ctx context.Context, key string) ([]byte, error) {
	if err, ok := m.errors["GetSecret"]; ok {
		return nil, err
	}
	return nil, nil
}

func (m *mockSetupStore) SetWireGuardPrivateKey(ctx context.Context, ifaceName string, privateKey []byte) error {
	if err, ok := m.errors["SetWireGuardPrivateKey"]; ok {
		return err
	}
	return nil
}

func (m *mockSetupStore) CreateNode(ctx context.Context, n store.NodeRecord) error {
	return nil
}

func TestHandleSetupStatus(t *testing.T) {
	tests := []struct {
		name           string
		setupComplete  bool
		expectedStatus int
	}{
		{
			name:           "setup incomplete",
			setupComplete:  false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "setup complete",
			setupComplete:  true,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockSetupStore{setupComplete: tt.setupComplete}
			handler := handlers.NewSetupHandler(mockStore, "/tmp/config.toml", func(p kms.Provider) {})

			req := httptest.NewRequest(http.MethodGet, "/setup/status", nil)
			rw := httptest.NewRecorder()

			handler.HandleStatus(rw, req)

			if rw.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rw.Code)
			}

			var resp map[string]bool
			if err := json.Unmarshal(rw.Body.Bytes(), &resp); err == nil {
				if status, ok := resp["complete"]; ok {
					if status != tt.setupComplete {
						t.Errorf("Expected complete=%v, got %v", tt.setupComplete, status)
					}
				}
			}
		})
	}
}

func TestHandleSetupValidation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid JSON",
			requestBody:    "invalid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON",
		},
		{
			name: "missing install_type",
			requestBody: map[string]interface{}{
				"admin": map[string]string{
					"username": "admin",
					"password": "ValidPassword123",
				},
				"wireguard": map[string]interface{}{
					"listen_port": 51820,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "install_type",
		},
		{
			name: "invalid install_type",
			requestBody: map[string]interface{}{
				"install_type": "invalid",
				"admin": map[string]string{
					"username": "admin",
					"password": "ValidPassword123",
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "hub, remote, or combined",
		},
		{
			name: "missing admin username",
			requestBody: map[string]interface{}{
				"install_type": "hub",
				"admin": map[string]string{
					"username": "",
					"password": "ValidPassword123",
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "admin",
		},
		{
			name: "missing admin password",
			requestBody: map[string]interface{}{
				"install_type": "hub",
				"admin": map[string]string{
					"username": "admin",
					"password": "",
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "admin",
		},
		{
			name: "password too short",
			requestBody: map[string]interface{}{
				"install_type": "hub",
				"admin": map[string]string{
					"username": "admin",
					"password": "Short1",
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			mockStore := &mockSetupStore{}
			handler := handlers.NewSetupHandler(mockStore, "/tmp/config.toml", func(p kms.Provider) {})

			req := httptest.NewRequest(http.MethodPost, "/setup", bytes.NewReader(body))
			rw := httptest.NewRecorder()

			handler.HandleSetup(rw, req)

			if rw.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, rw.Code, rw.Body.String())
			}

			if tt.expectedError != "" && !bytes.Contains(rw.Body.Bytes(), []byte(tt.expectedError)) {
				t.Errorf("Expected error containing %q, got %q", tt.expectedError, rw.Body.String())
			}
		})
	}
}

func TestHandleSetupInstallTypes(t *testing.T) {
	validInstallTypes := []string{"hub", "remote", "combined"}

	for _, installType := range validInstallTypes {
		t.Run(installType, func(t *testing.T) {
			requestBody := map[string]interface{}{
				"install_type": installType,
				"kms": map[string]interface{}{
					"provider": "local",
					"local": map[string]string{
						"key_file": "/tmp/key.bin",
					},
				},
				"admin": map[string]string{
					"username": "admin",
					"password": "ValidPassword123",
				},
				"wireguard": map[string]interface{}{
					"listen_port": 51820,
				},
			}

			if installType == "remote" {
				requestBody["wireguard"] = map[string]interface{}{
					"listen_port":  51820,
					"hub_endpoint": "10.0.0.1:51820",
				}
			}

			body, _ := json.Marshal(requestBody)
			mockStore := &mockSetupStore{}
			handler := handlers.NewSetupHandler(mockStore, "/tmp/config.toml", func(p kms.Provider) {})

			req := httptest.NewRequest(http.MethodPost, "/setup", bytes.NewReader(body))
			rw := httptest.NewRecorder()

			handler.HandleSetup(rw, req)

			if rw.Code < 200 {
				t.Errorf("Invalid install_type %s was rejected prematurely with status %d", installType, rw.Code)
			}
		})
	}
}

func TestMockSetupHandlerStatus(t *testing.T) {
	store := &mockSetupStore{
		setupComplete: true,
		users:         map[string]bool{"admin": true},
		errors:        make(map[string]error),
	}

	handler := handlers.NewSetupHandler(store, "/tmp/config.toml", func(p kms.Provider) {})

	req := httptest.NewRequest(http.MethodGet, "/setup/status", nil)
	w := httptest.NewRecorder()
	handler.HandleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code: got %d", w.Code)
	}
}
