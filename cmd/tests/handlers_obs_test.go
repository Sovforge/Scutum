package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/utils"
)

type mockObsStore struct {
	logs   []utils.LogEntry
	audit  []utils.AuditEntry
	traces []utils.TraceEntry
	err    error
}

func (m *mockObsStore) ListAuditLogs(ctx context.Context, limit int) ([]utils.AuditEntry, error) {
	return m.audit, m.err
}

func (m *mockObsStore) ListSystemLogs(ctx context.Context, limit int) ([]utils.LogEntry, error) {
	return m.logs, m.err
}

func (m *mockObsStore) ListTraces(ctx context.Context, limit int) ([]utils.TraceEntry, error) {
	return m.traces, m.err
}

func TestObservabilityHandler(t *testing.T) {
	store := &mockObsStore{
		logs:   []utils.LogEntry{{Message: "system log"}},
		audit:  []utils.AuditEntry{{Action: "audit log"}},
		traces: []utils.TraceEntry{{Name: "trace-1"}},
	}
	h := handlers.NewObservabilityHandler(store)

	t.Run("HandleLogs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs?limit=10", nil)
		w := httptest.NewRecorder()
		h.HandleLogs(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleAuditLogs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/audit/logs", nil)
		w := httptest.NewRecorder()
		h.HandleAuditLogs(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleTraces", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/traces", nil)
		w := httptest.NewRecorder()
		h.HandleTraces(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleExportAuditLogs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/audit/logs/export?format=csv", nil)
		w := httptest.NewRecorder()
		h.HandleExportAuditLogs(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}
