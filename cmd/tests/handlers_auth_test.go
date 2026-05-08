package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scutum/cmd/internal/auth"
	"scutum/cmd/internal/handlers"
)

type fakeUserAuthStore struct {
	createUserErr   error
	createAPIKeyErr error
	createdUserID   string
	createdUsername string
	createdHash     string
	userID          string
	username        string
	passwordHash    string
	userErr         error
}

func (f *fakeUserAuthStore) CreateUser(ctx context.Context, id, username, passwordHash string) error {
	if f.createUserErr != nil {
		return f.createUserErr
	}
	f.createdUserID = id
	f.createdUsername = username
	f.createdHash = passwordHash
	return nil
}

func (f *fakeUserAuthStore) UserByUsername(ctx context.Context, username string) (string, string, error) {
	if f.userErr != nil {
		return "", "", f.userErr
	}
	if username != f.username {
		return "", "", errors.New("not found")
	}
	return f.userID, f.passwordHash, nil
}

func (f *fakeUserAuthStore) CreateAPIKey(ctx context.Context, id, userID, name, keyHash string, expiresAt *time.Time) error {
	if f.createAPIKeyErr != nil {
		return f.createAPIKeyErr
	}
	return nil
}

func (f *fakeUserAuthStore) GetUserTOTP(ctx context.Context, userID string) (string, bool, error) {
	return "", false, nil
}

func (f *fakeUserAuthStore) SetUserTOTPSecret(ctx context.Context, userID, secret string) error {
	return nil
}

func (f *fakeUserAuthStore) SetUserTOTPEnabled(ctx context.Context, userID string, enabled bool) error {
	return nil
}

func (f *fakeUserAuthStore) CreateRecoveryCodes(ctx context.Context, userID string, codeHashes []string) error {
	return nil
}

func (f *fakeUserAuthStore) UseRecoveryCode(ctx context.Context, userID, codeHash string) error {
	return nil
}

func (f *fakeUserAuthStore) CountRemainingRecoveryCodes(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (f *fakeUserAuthStore) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	return nil
}

func TestAuthHandlerRegisterValidation(t *testing.T) {
	h := handlers.NewAuthHandler(&fakeUserAuthStore{}, []byte("secret-key-1234567890"))

	cases := []struct {
		name string
		body string
		code int
	}{
		{"invalid-json", `{`, http.StatusBadRequest},
		{"missing-fields", `{"username":"alice"}`, http.StatusBadRequest},
		{"short-password", `{"username":"alice","password":"short"}`, http.StatusBadRequest},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			h.HandleRegister(w, req)
			if w.Code != tt.code {
				t.Fatalf("expected %d, got %d", tt.code, w.Code)
			}
		})
	}
}

func TestAuthHandlerRegisterSuccess(t *testing.T) {
	store := &fakeUserAuthStore{}
	h := handlers.NewAuthHandler(store, []byte("secret-key-1234567890"))

	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"username":"alice","password":"verystrongpassword"}`))
	w := httptest.NewRecorder()
	h.HandleRegister(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["username"] != "alice" {
		t.Fatalf("unexpected username: %q", result["username"])
	}
}

func TestAuthHandlerLoginAndCreateAPIKey(t *testing.T) {
	secret := []byte("secret-key-1234567890")
	store := &fakeUserAuthStore{userID: "uid", username: "alice"}
	h := handlers.NewAuthHandler(store, secret)

	if _, err := auth.HashPassword("password123"); err != nil {
		t.Fatal(err)
	}

	// Use a valid hash for successful login.
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	store.passwordHash = hash

	t.Run("invalid-json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{`))
		w := httptest.NewRecorder()
		h.HandleLogin(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid-credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"alice","password":"wrongpass"}`))
		w := httptest.NewRecorder()
		h.HandleLogin(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"alice","password":"password123"}`))
		w := httptest.NewRecorder()
		h.HandleLogin(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var result map[string]string
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result["token"] == "" {
			t.Fatalf("expected token in response")
		}
	})
}

func TestAuthHandlerCreateAPIKey(t *testing.T) {
	store := &fakeUserAuthStore{}
	h := handlers.NewAuthHandler(store, []byte("secret-key-1234567890"))

	t.Run("unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/keys", strings.NewReader(`{"name":"keyname"}`))
		w := httptest.NewRecorder()
		h.HandleCreateAPIKey(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("invalid-expires-at", func(t *testing.T) {
		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		req := httptest.NewRequest(http.MethodPost, "/auth/keys", strings.NewReader(`{"name":"keyname","expires_at":"not-a-time"}`))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleCreateAPIKey(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		req := httptest.NewRequest(http.MethodPost, "/auth/keys", strings.NewReader(`{"name":"keyname"}`))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleCreateAPIKey(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", w.Code)
		}
		var result map[string]string
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result["key"] == "" {
			t.Fatal("expected key in response")
		}
	})
}

func TestAuthHandlerErrors(t *testing.T) {
	secret := []byte("secret-key-1234567890")

	t.Run("register-user-exists", func(t *testing.T) {
		store := &fakeUserAuthStore{createUserErr: errors.New("username already exists")}
		h := handlers.NewAuthHandler(store, secret)

		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"username":"alice","password":"verystrongpassword"}`))
		w := httptest.NewRecorder()
		h.HandleRegister(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", w.Code)
		}
	})

	t.Run("login-user-not-found", func(t *testing.T) {
		store := &fakeUserAuthStore{userErr: errors.New("not found")}
		h := handlers.NewAuthHandler(store, secret)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"nobody","password":"password123"}`))
		w := httptest.NewRecorder()
		h.HandleLogin(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("create-apikey-store-error", func(t *testing.T) {
		store := &fakeUserAuthStore{createAPIKeyErr: errors.New("database error")}
		h := handlers.NewAuthHandler(store, secret)

		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		req := httptest.NewRequest(http.MethodPost, "/auth/keys", strings.NewReader(`{"name":"keyname"}`))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleCreateAPIKey(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", w.Code)
		}
	})

	t.Run("login-empty-username", func(t *testing.T) {
		store := &fakeUserAuthStore{}
		h := handlers.NewAuthHandler(store, secret)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"","password":"password123"}`))
		w := httptest.NewRecorder()
		h.HandleLogin(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("login-empty-password", func(t *testing.T) {
		store := &fakeUserAuthStore{}
		h := handlers.NewAuthHandler(store, secret)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"alice","password":""}`))
		w := httptest.NewRecorder()
		h.HandleLogin(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("create-apikey-empty-name", func(t *testing.T) {
		store := &fakeUserAuthStore{}
		h := handlers.NewAuthHandler(store, secret)

		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		req := httptest.NewRequest(http.MethodPost, "/auth/keys", strings.NewReader(`{"name":""}`))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleCreateAPIKey(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestAuthHandlerMFA(t *testing.T) {
	secret := []byte("secret-key-1234567890")
	store := &fakeUserAuthStore{userID: "uid", username: "alice"}
	h := handlers.NewAuthHandler(store, secret)

	t.Run("HandleMFAStatus", func(t *testing.T) {
		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		req := httptest.NewRequest(http.MethodGet, "/auth/mfa/status", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleMFAStatus(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleMFASetup", func(t *testing.T) {
		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		req := httptest.NewRequest(http.MethodPost, "/auth/mfa/setup", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleMFASetup(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var result map[string]string
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result["secret"] == "" || result["qr_code"] == "" {
			t.Fatal("expected secret and qr_code in response")
		}
	})

	t.Run("HandleMFAEnable-InvalidCode", func(t *testing.T) {
		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		body := `{"code":"000000"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/mfa/enable", strings.NewReader(body))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleMFAEnable(w, req)
		if w.Code != http.StatusUnprocessableEntity && w.Code != http.StatusBadRequest {
			// It might be 400 if no pending secret exists in the fake store
			t.Fatalf("expected error status, got %d", w.Code)
		}
	})

	t.Run("HandleMFADisable-NotEnabled", func(t *testing.T) {
		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		body := `{"code":"000000"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/mfa/disable", strings.NewReader(body))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleMFADisable(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}


func TestAuthHandlerRecoveryCodes(t *testing.T) {
	secret := []byte("secret-key-1234567890")
	store := &fakeUserAuthStore{userID: "uid", username: "alice"}
	h := handlers.NewAuthHandler(store, secret)

	t.Run("HandleRecoveryCodeStatus", func(t *testing.T) {
		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		req := httptest.NewRequest(http.MethodGet, "/auth/recovery-codes", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleRecoveryCodeStatus(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleRegenerateRecoveryCodes", func(t *testing.T) {
		ctx := auth.WithClaims(context.Background(), auth.Claims{UserID: "uid", Username: "alice"})
		req := httptest.NewRequest(http.MethodPost, "/auth/recovery-codes/regenerate", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.HandleRegenerateRecoveryCodes(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var result map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if codes, ok := result["recovery_codes"].([]interface{}); !ok || len(codes) == 0 {
			t.Fatal("expected recovery_codes in response")
		}
	})
}

func TestAuthHandlerForgotPassword(t *testing.T) {
	secret := []byte("secret-key-1234567890")
	store := &fakeUserAuthStore{userID: "uid", username: "alice"}
	h := handlers.NewAuthHandler(store, secret)

	t.Run("success-with-recovery-code", func(t *testing.T) {
		body := `{"username":"alice","new_password":"new-very-strong-password","recovery_code":"REC-1234-5678"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.HandleForgotPassword(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("invalid-json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{`))
		w := httptest.NewRecorder()
		h.HandleForgotPassword(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("short-password", func(t *testing.T) {
		body := `{"username":"alice","new_password":"short","recovery_code":"REC"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.HandleForgotPassword(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}


