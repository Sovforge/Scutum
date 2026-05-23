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

func (s *Store) PersistMetric(e utils.MetricPoint) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.insertMetric(ctx, e)
	}()
}

// ── Insert methods ──────────────────────────────────────────────────────────

func (s *Store) insertAuditLog(ctx context.Context, e utils.AuditEntry) error {
	extra, _ := json.Marshal(e.Extra)
	outcome := e.Outcome
	if outcome == "" {
		outcome = utils.OutcomeSuccess
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO audit_logs
			(id, time, action, actor, actor_id, outcome, method, path, trace_id, client_ip, extra)
			VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6),
			s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11)),
		uuid.New().String(), e.Time, e.Action, e.Actor, e.ActorID, outcome,
		e.Method, e.Path, e.TraceID, e.ClientIP, string(extra),
	)
	return err
}

func (s *Store) insertSystemLog(ctx context.Context, e utils.LogEntry) error {
	attrs, _ := json.Marshal(e.Attributes)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO system_logs
			(id, time, level, message, service, source, trace_id, span_id, attributes)
			VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5),
			s.ph(6), s.ph(7), s.ph(8), s.ph(9)),
		uuid.New().String(), e.Time, e.Level, e.Message,
		e.Service, e.Source, e.TraceID, e.SpanID, string(attrs),
	)
	return err
}

func (s *Store) insertTrace(ctx context.Context, e utils.TraceEntry) error {
	attrs, _ := json.Marshal(e.Attributes)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO traces
			(id, time, name, duration_ms, status, error,
			 trace_id, span_id, parent_span_id, service, kind, source, attributes)
			VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6),
			s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11), s.ph(12), s.ph(13)),
		uuid.New().String(), e.Time, e.Name, e.DurationMs, e.Status, e.Error,
		e.TraceID, e.SpanID, e.ParentSpanID, e.Service, e.Kind, e.Source, string(attrs),
	)
	return err
}

func (s *Store) insertMetric(ctx context.Context, e utils.MetricPoint) error {
	labels, _ := json.Marshal(e.Labels)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO otel_metrics
			(id, time, name, service, source, type, value, labels)
			VALUES (%s, %s, %s, %s, %s, %s, %s, %s)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8)),
		uuid.New().String(), e.Time, e.Name, e.Service, e.Source, e.Type, e.Value, string(labels),
	)
	return err
}

// ── Query methods used by the observability handler ─────────────────────────

func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]utils.AuditEntry, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT time, action, actor, actor_id, outcome, method, path, trace_id, client_ip, extra
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
		if err := rows.Scan(&e.Time, &e.Action, &e.Actor, &e.ActorID, &e.Outcome,
			&e.Method, &e.Path, &e.TraceID, &e.ClientIP, &extraJSON); err != nil {
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
		fmt.Sprintf(`SELECT time, level, message, service, source, trace_id, span_id, attributes
		             FROM system_logs ORDER BY time DESC LIMIT %s`, s.ph(1)),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []utils.LogEntry
	for rows.Next() {
		var e utils.LogEntry
		var attrsJSON string
		if err := rows.Scan(&e.Time, &e.Level, &e.Message, &e.Service, &e.Source,
			&e.TraceID, &e.SpanID, &attrsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(attrsJSON), &e.Attributes)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ListTraces(ctx context.Context, limit int) ([]utils.TraceEntry, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT time, name, duration_ms, status, error,
		             trace_id, span_id, parent_span_id, service, kind, source, attributes
		             FROM traces ORDER BY time DESC LIMIT %s`, s.ph(1)),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []utils.TraceEntry
	for rows.Next() {
		var e utils.TraceEntry
		var attrsJSON string
		if err := rows.Scan(&e.Time, &e.Name, &e.DurationMs, &e.Status, &e.Error,
			&e.TraceID, &e.SpanID, &e.ParentSpanID, &e.Service, &e.Kind, &e.Source,
			&attrsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(attrsJSON), &e.Attributes)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ListMetrics(ctx context.Context, limit int, name, service string) ([]utils.MetricPoint, error) {
	if limit <= 0 {
		limit = 500
	}
	q := fmt.Sprintf(`SELECT time, name, service, source, type, value, labels
	                  FROM otel_metrics WHERE 1=1`)
	args := []interface{}{}
	n := 1
	if name != "" {
		q += fmt.Sprintf(" AND name = %s", s.ph(n))
		args = append(args, name)
		n++
	}
	if service != "" {
		q += fmt.Sprintf(" AND service = %s", s.ph(n))
		args = append(args, service)
		n++
	}
	q += fmt.Sprintf(" ORDER BY time DESC LIMIT %s", s.ph(n))
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []utils.MetricPoint
	for rows.Next() {
		var e utils.MetricPoint
		var labelsJSON string
		if err := rows.Scan(&e.Time, &e.Name, &e.Service, &e.Source, &e.Type, &e.Value, &labelsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(labelsJSON), &e.Labels)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) DeleteOldLogs(ctx context.Context, olderThan time.Time) error {
	tables := []string{"audit_logs", "system_logs", "traces", "otel_metrics"}
	for _, table := range tables {
		q := fmt.Sprintf("DELETE FROM %s WHERE time < %s", table, s.ph(1))
		if _, err := s.db.ExecContext(ctx, q, olderThan); err != nil {
			return fmt.Errorf("delete old %s: %w", table, err)
		}
	}
	return nil
}
