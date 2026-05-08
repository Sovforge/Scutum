package tests

import (
	"context"
	"testing"
	"time"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"
)

func TestStoreObservability(t *testing.T) {
	st, err := store.New(context.Background(), ":memory:", &mockKMS{})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	t.Run("AuditLogs", func(t *testing.T) {
		e := utils.AuditEntry{
			Time:     time.Now(),
			Action:   "LOGIN",
			Method:   "POST",
			Path:     "/login",
			TraceID:  "trace-1",
			ClientIP: "127.0.0.1",
		}
		st.PersistAudit(e)
		time.Sleep(100 * time.Millisecond) // wait for goroutine

		logs, err := st.ListAuditLogs(ctx, 10)
		if err != nil {
			t.Fatalf("ListAuditLogs failed: %v", err)
		}
		if len(logs) == 0 {
			t.Fatal("expected audit logs")
		}
		if logs[0].Action != "LOGIN" {
			t.Errorf("log mismatch: %+v", logs[0])
		}
	})

	t.Run("SystemLogs", func(t *testing.T) {
		e := utils.LogEntry{
			Time:    time.Now(),
			Level:   "INFO",
			Message: "server started",
		}
		st.PersistLog(e)
		time.Sleep(100 * time.Millisecond)

		logs, err := st.ListSystemLogs(ctx, 10)
		if err != nil {
			t.Fatalf("ListSystemLogs failed: %v", err)
		}
		if len(logs) == 0 {
			t.Fatal("expected system logs")
		}
		if logs[0].Level != "INFO" || logs[0].Message != "server started" {
			t.Errorf("log mismatch: %+v", logs[0])
		}
	})

	t.Run("Traces", func(t *testing.T) {
		e := utils.TraceEntry{
			Time:       time.Now(),
			Name:       "span-1",
			DurationMs: 10,
			Status:     "OK",
		}
		st.PersistTrace(e)
		time.Sleep(100 * time.Millisecond)

		traces, err := st.ListTraces(ctx, 10)
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(traces) == 0 {
			t.Fatal("expected traces")
		}
		if traces[0].Name != "span-1" {
			t.Errorf("trace mismatch: %+v", traces[0])
		}
	})
}


