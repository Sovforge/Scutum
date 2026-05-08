package tests

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/utils"
)

func newBaseHandler(t *testing.T) *handlers.BaseHandler {
	t.Helper()
	return handlers.NewBaseHandler(nil) // falls back to DefaultLogger
}

// TestBaseHandlerWriteError writes a JSON error response with correct status.
func TestBaseHandlerWriteError(t *testing.T) {
	utils.InitLogger(0, false) // ensure DefaultLogger is set

	h := newBaseHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	h.WriteError(w, req, http.StatusBadRequest, errors.New("something went wrong"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "something went wrong") {
		t.Errorf("body missing error text: %s", body)
	}
}

func TestBaseHandlerWriteError500(t *testing.T) {
	utils.InitLogger(0, false)
	h := newBaseHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	h.WriteError(w, req, http.StatusInternalServerError, errors.New("internal oops"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestBaseHandlerWriteJSON writes a JSON success response.
func TestBaseHandlerWriteJSON(t *testing.T) {
	utils.InitLogger(0, false)
	h := newBaseHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	h.WriteJSON(w, req, http.StatusCreated, map[string]string{"key": "value"})

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "value") {
		t.Errorf("body missing data: %s", body)
	}
}

// TestBaseHandlerWrapPanicRecovery verifies panicking handlers return 500.
func TestBaseHandlerWrapPanicRecovery(t *testing.T) {
	utils.InitLogger(0, false)
	h := newBaseHandler(t)

	panicky := h.Wrap(func(w http.ResponseWriter, r *http.Request) {
		panic("intentional panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()
	panicky(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", w.Code)
	}
}

// TestBaseHandlerWrapNormalRequest verifies non-panicking handlers pass through.
func TestBaseHandlerWrapNormalRequest(t *testing.T) {
	utils.InitLogger(0, false)
	h := newBaseHandler(t)

	called := false
	wrapped := h.Wrap(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	wrapped(w, req)

	if !called {
		t.Error("inner handler was not called")
	}
}

// TestBaseHandlerAudit verifies Audit does not panic with DefaultLogger.
func TestBaseHandlerAudit(t *testing.T) {
	utils.InitLogger(0, false)
	h := newBaseHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/action", nil)
	// Must not panic.
	h.Audit("TEST_ACTION", req, "key", "value")
}

// TestBaseHandlerLogRequest verifies LogRequest does not panic.
func TestBaseHandlerLogRequest(t *testing.T) {
	utils.InitLogger(0, false)
	h := newBaseHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	h.LogRequest(req, http.StatusOK, 0)
}

// TestNewBaseHandlerNilFallback verifies nil logger falls back to DefaultLogger.
func TestNewBaseHandlerNilFallback(t *testing.T) {
	utils.InitLogger(0, false)
	h := handlers.NewBaseHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil BaseHandler")
	}
	// Should not panic when calling methods.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.WriteJSON(w, req, http.StatusOK, nil)
}
