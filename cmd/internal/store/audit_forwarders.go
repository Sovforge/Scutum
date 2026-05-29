package store

import (
	"context"
	"fmt"
	"time"
)

type AuditForwarder struct {
	ID        string
	Name      string
	URL       string
	Format    string // "json" or "cef"
	Enabled   bool
	CreatedAt time.Time
}

func (s *Store) CreateAuditForwarder(ctx context.Context, id, name, url, format string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO audit_forwarders (id,name,url,format,enabled) VALUES (%s,%s,%s,%s,1)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4)),
		id, name, url, format)
	return err
}

func (s *Store) ListAuditForwarders(ctx context.Context) ([]AuditForwarder, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,name,url,format,enabled,created_at FROM audit_forwarders ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditForwarder
	for rows.Next() {
		var f AuditForwarder
		var enabled int
		if err := rows.Scan(&f.ID, &f.Name, &f.URL, &f.Format, &enabled, &f.CreatedAt); err != nil {
			return nil, err
		}
		f.Enabled = enabled != 0
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) GetAuditForwarder(ctx context.Context, id string) (AuditForwarder, error) {
	var f AuditForwarder
	var enabled int
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id,name,url,format,enabled,created_at FROM audit_forwarders WHERE id=%s`, s.ph(1)), id).
		Scan(&f.ID, &f.Name, &f.URL, &f.Format, &enabled, &f.CreatedAt)
	if err != nil {
		return AuditForwarder{}, err
	}
	f.Enabled = enabled != 0
	return f, nil
}

func (s *Store) UpdateAuditForwarder(ctx context.Context, id, name, url, format string, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE audit_forwarders SET name=%s,url=%s,format=%s,enabled=%s WHERE id=%s`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5)),
		name, url, format, en, id)
	return err
}

func (s *Store) DeleteAuditForwarder(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM audit_forwarders WHERE id=%s`, s.ph(1)), id)
	return err
}

func (s *Store) ListEnabledAuditForwarders(ctx context.Context) ([]AuditForwarder, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,name,url,format,enabled,created_at FROM audit_forwarders WHERE enabled=1 ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditForwarder
	for rows.Next() {
		var f AuditForwarder
		var enabled int
		if err := rows.Scan(&f.ID, &f.Name, &f.URL, &f.Format, &enabled, &f.CreatedAt); err != nil {
			return nil, err
		}
		f.Enabled = enabled != 0
		out = append(out, f)
	}
	return out, rows.Err()
}
