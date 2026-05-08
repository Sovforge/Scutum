package tests

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auth "scutum/cmd/internal/auth"
)

type fakeAuthStore struct {
	apiKeys map[string]auth.Claims
	perms   map[string]bool
	err     error
}

func (f fakeAuthStore) UserByAPIKey(keyHash string) (string, string, error) {
	claims, ok := f.apiKeys[keyHash]
	if !ok {
		return "", "", f.err
	}
	return claims.UserID, claims.Username, nil
}

func (f fakeAuthStore) UserHasPermission(userID, resource, action string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.perms[userID+"|"+resource+"|"+action], nil
}

func TestMiddlewareAllowsPublicRoutes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	called := false

	handler := auth.Middleware(nil, []byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected public route handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestMiddlewareAcceptsValidJWT(t *testing.T) {
	secret := []byte("test-secret")
	token, err := auth.IssueJWT("user-id", "alice", secret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	called := false

	handler := auth.Middleware(fakeAuthStore{}, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			t.Fatal("expected claims in context")
		}
		if claims.UserID != "user-id" || claims.Username != "alice" {
			t.Fatalf("unexpected claims: %#v", claims)
		}
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected protected handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestMiddlewareAcceptsValidAPIKey(t *testing.T) {
	apiKey := "test-api-key"
	store := fakeAuthStore{
		apiKeys: map[string]auth.Claims{
			auth.HashAPIKey(apiKey): {UserID: "api-user", Username: "api-user"},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/resource", nil)
	req.Header.Set("X-API-Key", apiKey)
	rr := httptest.NewRecorder()
	called := false

	handler := auth.Middleware(store, []byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			t.Fatal("expected claims in context")
		}
		if claims.UserID != "api-user" || claims.Username != "api-user" {
			t.Fatalf("unexpected claims: %#v", claims)
		}
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected API key handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestMiddlewareRejectsMissingCredentials(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	rr := httptest.NewRecorder()

	handler := auth.Middleware(fakeAuthStore{}, []byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestRequireAllowsPermittedUser(t *testing.T) {
	store := fakeAuthStore{
		perms: map[string]bool{"user-id|repos|read": true},
	}

	req := httptest.NewRequest(http.MethodGet, "/repos", nil)
	req = req.WithContext(auth.WithClaims(context.Background(), auth.Claims{UserID: "user-id", Username: "alice"}))
	rr := httptest.NewRecorder()

	handler := auth.Require(store, "repos", "read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestRequireDeniesUnauthorizedUser(t *testing.T) {
	store := fakeAuthStore{
		perms: map[string]bool{"user-id|repos|read": false},
	}

	req := httptest.NewRequest(http.MethodGet, "/repos", nil)
	req = req.WithContext(auth.WithClaims(context.Background(), auth.Claims{UserID: "user-id", Username: "alice"}))
	rr := httptest.NewRecorder()

	handler := auth.Require(store, "repos", "read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestRequireRejectsMissingClaims(t *testing.T) {
	store := fakeAuthStore{
		perms: map[string]bool{"user-id|repos|read": true},
	}

	req := httptest.NewRequest(http.MethodGet, "/repos", nil)
	rr := httptest.NewRecorder()

	handler := auth.Require(store, "repos", "read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestMiddlewareRejectsExpiredJWT(t *testing.T) {
	secret := []byte("test-secret")
	// Issue a token that expired 1 minute ago.
	token, err := auth.IssueJWT("user-id", "alice", secret, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler := auth.Middleware(fakeAuthStore{}, secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with expired token")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", rr.Code)
	}
}

func TestMiddlewareRejectsMalformedJWT(t *testing.T) {
	cases := []string{
		"notavalidtoken",
		"Bearer",
		"only.two",
		"Bearer invalid.jwt.token",
	}
	for _, token := range cases {
		t.Run(token, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", token)
			rr := httptest.NewRecorder()

			handler := auth.Middleware(fakeAuthStore{}, []byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("handler should not be called")
			}))

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("token %q: expected 401, got %d", token, rr.Code)
			}
		})
	}
}

func TestMiddlewareRejectsUnknownAPIKey(t *testing.T) {
	store := fakeAuthStore{
		apiKeys: map[string]auth.Claims{},
		err:     errors.New("not found"),
	}

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("X-API-Key", "unknown-key")
	rr := httptest.NewRecorder()

	handler := auth.Middleware(store, []byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown API key, got %d", rr.Code)
	}
}

func TestRequirePermissionStoreError(t *testing.T) {
	store := fakeAuthStore{
		err: errors.New("db error"),
	}

	req := httptest.NewRequest(http.MethodGet, "/repos", nil)
	req = req.WithContext(auth.WithClaims(context.Background(), auth.Claims{UserID: "user-id", Username: "alice"}))
	rr := httptest.NewRecorder()

	handler := auth.Require(store, "repos", "read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called on store error")
	}))

	handler.ServeHTTP(rr, req)

	// Store error should result in denial (403 or 500, not 200).
	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200, got %d", rr.Code)
	}
}
