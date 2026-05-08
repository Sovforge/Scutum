package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"scutum/cmd/internal/handlers"
)

func TestVersionHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	handlers.VersionHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !contains(ct, "application/json") {
		t.Errorf("expected JSON content-type, got %s", ct)
	}
}

func TestVersionHandlerResponseShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	handlers.VersionHandler(w, req)

	var out map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"version", "build", "commit"} {
		if _, ok := out[key]; !ok {
			t.Errorf("missing key %q in response", key)
		}
	}
}

func TestVersionHandlerVersionFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	handlers.VersionHandler(w, req)

	var out map[string]string
	json.Unmarshal(w.Body.Bytes(), &out)

	v := out["version"]
	if v == "" {
		t.Error("version must not be empty")
	}
}
