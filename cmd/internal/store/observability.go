package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"scutum/cmd/internal/utils"
)

// ── Implement utils.ObsSink so the store can be registered as the DB sink ──

func (s *Store) PersistAudit(e utils.AuditEntry) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.insertAuditLog(ctx, e)
	}()
}

func (s *Store) PersistLog(e utils.LogEntry) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.insertSystemLog(ctx, e)
	}()
}

func (s *Store) PersistTrace(e utils.TraceEntry) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.insertTrace(ctx, e)
	}()
}

// ── Insert methods ──────────────────────────────────────────────────────────

func (s *Store) insertAuditLog(ctx context.Context, e utils.AuditEntry) error {
	extra, _ := json.Marshal(e.Extra)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO audit_logs (id, time, action, method, path, trace_id, client_ip, extra)
		             VALUES (%s, %s, %s, %s, %s, %s, %s, %s)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8)),
		uuid.New().String(), e.Time, e.Action, e.Method, e.Path,
		e.TraceID, e.ClientIP, string(extra),
	)
	return err
}

func (s *Store) insertSystemLog(ctx context.Context, e utils.LogEntry) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO system_logs (id, time, level, message) VALUES (%s, %s, %s, %s)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4)),
		uuid.New().String(), e.Time, e.Level, e.Message,
	)
	return err
}

func (s *Store) insertTrace(ctx context.Context, e utils.TraceEntry) error {
	errStr := ""
	if e.Error != "" {
		errStr = e.Error
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO traces (id, time, name, duration_ms, status, error)
		             VALUES (%s, %s, %s, %s, %s, %s)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6)),
		uuid.New().String(), e.Time, e.Name, e.DurationMs, e.Status, errStr,
	)
	return err
}

// ── Query methods used by the observability handler ─────────────────────────

func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]utils.AuditEntry, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT time, action, method, path, trace_id, client_ip, extra
		             FROM audit_logs ORDER BY time DESC LIMIT %s`, s.ph(1)),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []utils.AuditEntry
	for rows.Next() {
		var e utils.AuditEntry
		var extraJSON string
		if err := rows.Scan(&e.Time, &e.Action, &e.Method, &e.Path,
			&e.TraceID, &e.ClientIP, &extraJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(extraJSON), &e.Extra)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ListSystemLogs(ctx context.Context, limit int) ([]utils.LogEntry, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT time, level, message FROM system_logs ORDER BY time DESC LIMIT %s`, s.ph(1)),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []utils.LogEntry
	for rows.Next() {
		var e utils.LogEntry
		if err := rows.Scan(&e.Time, &e.Level, &e.Message); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ListTraces(ctx context.Context, limit int) ([]utils.TraceEntry, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT time, name, duration_ms, status, error FROM traces ORDER BY time DESC LIMIT %s`, s.ph(1)),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []utils.TraceEntry
	for rows.Next() {
		var e utils.TraceEntry
		if err := rows.Scan(&e.Time, &e.Name, &e.DurationMs, &e.Status, &e.Error); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) DeleteOldLogs(ctx context.Context, olderThan time.Time) error {
	tables := []string{"audit_logs", "system_logs", "traces"}
	for _, table := range tables {
		q := fmt.Sprintf("DELETE FROM %s WHERE time < %s", table, s.ph(1))
		if _, err := s.db.ExecContext(ctx, q, olderThan); err != nil {
			return fmt.Errorf("delete old %s: %w", table, err)
		}
	}
	return nil
}
