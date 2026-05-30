package tests

import (
	"net/http/httptest"
	"strings"
	"testing"

	"scutum/cmd/internal/handlers"
)

func TestComplianceReport_JSONFormat(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewComplianceHandler(s, "1.1.0-test")

	req := httptest.NewRequest("GET", "/compliance/report", nil)
	w := httptest.NewRecorder()
	h.HandleReport(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON content-type, got %q", ct)
	}
	body := w.Body.String()
	for _, field := range []string{"meta", "users", "mesh", "audit", "security", "key_management", "incidents"} {
		if !strings.Contains(body, `"`+field+`"`) {
			t.Errorf("JSON report missing field %q", field)
		}
	}
	if !strings.Contains(body, "CRA") {
		t.Error("expected CRA standard reference in report")
	}
}

func TestComplianceReport_CSVFormat(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewComplianceHandler(s, "1.1.0-test")

	req := httptest.NewRequest("GET", "/compliance/report?format=csv", nil)
	w := httptest.NewRecorder()
	h.HandleReport(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Errorf("expected text/csv content-type, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "time,action,actor") {
		t.Errorf("expected CSV header row, got: %s", body[:min(80, len(body))])
	}
}

func TestComplianceReport_TextFormat(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewComplianceHandler(s, "1.1.0-test")

	req := httptest.NewRequest("GET", "/compliance/report?format=text", nil)
	w := httptest.NewRecorder()
	h.HandleReport(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content-type, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "CRA COMPLIANCE REPORT") {
		t.Error("expected 'CRA COMPLIANCE REPORT' header in text output")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
