package tests

import (
	"context"
	"log/slog"
	"testing"
	"scutum/cmd/internal/utils"
)

type mockObsSink struct {
	logs   []utils.LogEntry
	audit  []utils.AuditEntry
	traces []utils.TraceEntry
}

func (m *mockObsSink) PersistLog(e utils.LogEntry)   { m.logs = append(m.logs, e) }
func (m *mockObsSink) PersistAudit(e utils.AuditEntry) { m.audit = append(m.audit, e) }
func (m *mockObsSink) PersistTrace(e utils.TraceEntry) { m.traces = append(m.traces, e) }

func TestLoggerUtils(t *testing.T) {
	sink := &mockObsSink{}
	utils.SetObsSink(sink)
	logger := utils.InitLogger(slog.LevelDebug, true)

	t.Run("Logging", func(t *testing.T) {
		logger.Info("test info message", "key", "val")
		if len(sink.logs) == 0 {
			t.Error("expected log to be appended to sink")
		}
	})

	t.Run("Audit", func(t *testing.T) {
		logger.Audit("LOGIN", "method", "POST", "username", "alice")
		if len(sink.audit) == 0 {
			t.Error("expected audit log to be appended")
		}
	})

	t.Run("Trace", func(t *testing.T) {
		tr := logger.Trace(context.Background(), "my-span")
		tr.End(nil)
		if len(sink.traces) == 0 {
			t.Error("expected trace to be appended")
		}
	})

	t.Run("LogWriter", func(t *testing.T) {
		w := &utils.LogWriter{}
		w.Write([]byte("hello"))
		if w.String() != "hello" {
			t.Errorf("expected hello, got %s", w.String())
		}
		w.Reset()
		if w.String() != "" {
			t.Error("expected empty after reset")
		}
	})

	t.Run("StdLogger", func(t *testing.T) {
		l := utils.StdLogger(logger)
		if l == nil {
			t.Fatal("expected non-nil std logger")
		}
		l.Println("log from std")
	})
}

