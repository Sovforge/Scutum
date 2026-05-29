package tests

import (
	"net/http/httptest"
	"os"
	"testing"

	scutumacme "scutum/cmd/internal/acme"
	"scutum/cmd/internal/handlers"
)

func TestACMEConfig_EnabledWhenBothSet(t *testing.T) {
	cfg := scutumacme.Config{Domain: "example.com", Email: "a@b.com"}
	if !cfg.Enabled() {
		t.Fatal("expected Enabled() true when both Domain and Email are set")
	}
}

func TestACMEConfig_DisabledWhenDomainMissing(t *testing.T) {
	cfg := scutumacme.Config{Email: "a@b.com"}
	if cfg.Enabled() {
		t.Fatal("expected Enabled() false when Domain is empty")
	}
}

func TestACMEConfig_DisabledWhenEmailMissing(t *testing.T) {
	cfg := scutumacme.Config{Domain: "example.com"}
	if cfg.Enabled() {
		t.Fatal("expected Enabled() false when Email is empty")
	}
}

func TestACMEConfig_FromEnv(t *testing.T) {
	t.Setenv("ACME_DOMAIN", "test.example.com")
	t.Setenv("ACME_EMAIL", "ops@example.com")
	t.Setenv("ACME_STAGING", "true")

	cfg := scutumacme.FromEnv("/tmp")
	if !cfg.Enabled() {
		t.Fatal("expected Enabled() true from env vars")
	}
	if cfg.Domain != "test.example.com" {
		t.Errorf("Domain = %q, want test.example.com", cfg.Domain)
	}
	if cfg.Email != "ops@example.com" {
		t.Errorf("Email = %q, want ops@example.com", cfg.Email)
	}
	if !cfg.Staging {
		t.Error("expected Staging true")
	}
}

func TestSystemHandler_TLSModeNone(t *testing.T) {
	os.Unsetenv("ACME_DOMAIN")
	os.Unsetenv("ACME_EMAIL")
	os.Unsetenv("CERT_FILE")

	h := handlers.NewSystemHandler()
	req := httptest.NewRequest("GET", "/system/tls-mode", nil)
	w := httptest.NewRecorder()
	h.HandleTLSMode(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !contains(body, `"none"`) {
		t.Errorf("expected mode=none in response, got: %s", body)
	}
}

func TestSystemHandler_TLSModeACME(t *testing.T) {
	t.Setenv("ACME_DOMAIN", "acme.example.com")
	t.Setenv("ACME_EMAIL", "admin@example.com")

	h := handlers.NewSystemHandler()
	req := httptest.NewRequest("GET", "/system/tls-mode", nil)
	w := httptest.NewRecorder()
	h.HandleTLSMode(w, req)

	body := w.Body.String()
	if !contains(body, `"acme"`) {
		t.Errorf("expected mode=acme in response, got: %s", body)
	}
	if !contains(body, "acme.example.com") {
		t.Errorf("expected domain in response, got: %s", body)
	}
}
