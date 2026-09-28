package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// NetworkPolicy is one rule in the mesh's network policy set. Src/Dst
// describe which side of the traffic the rule matches: "any" (unrestricted),
// "node" (a single node, by ID), or "group" (a node group, by ID) — resolved
// to WireGuard mesh IPs at apply time, not stored here.
type NetworkPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"` // lower runs first
	Action      string `json:"action"`   // "allow" | "deny"
	Protocol    string `json:"protocol"` // "tcp" | "udp" | "icmp" | "any"
	Port        string `json:"port"`     // "" (any), "22", or "1000-2000"
	SrcType     string `json:"src_type"` // "any" | "node" | "group"
	SrcID       string `json:"src_id"`
	DstType     string `json:"dst_type"`
	DstID       string `json:"dst_id"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedBy   string `json:"updated_by"`
	UpdatedAt   string `json:"updated_at"`
}

func (s *Store) CreateNetworkPolicy(ctx context.Context, p NetworkPolicy) (NetworkPolicy, error) {
	q := fmt.Sprintf(`
		INSERT INTO network_policies
			(id, name, description, enabled, priority, action, protocol, port, src_type, src_id, dst_type, dst_id, created_by, updated_by)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11), s.ph(12), s.ph(13), s.ph(14),
	)
	_, err := s.db.ExecContext(ctx, q,
		p.ID, p.Name, p.Description, boolToInt(p.Enabled), p.Priority, p.Action, p.Protocol, p.Port,
		p.SrcType, p.SrcID, p.DstType, p.DstID, p.CreatedBy, p.CreatedBy,
	)
	if err != nil {
		return NetworkPolicy{}, err
	}
	return s.GetNetworkPolicy(ctx, p.ID)
}

func (s *Store) GetNetworkPolicy(ctx context.Context, id string) (NetworkPolicy, error) {
	q := fmt.Sprintf(`
		SELECT id, name, description, enabled, priority, action, protocol, port, src_type, src_id, dst_type, dst_id, created_by, created_at, updated_by, updated_at
		FROM network_policies WHERE id = %s`, s.ph(1))
	var p NetworkPolicy
	var enabled int
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&p.ID, &p.Name, &p.Description, &enabled, &p.Priority, &p.Action, &p.Protocol, &p.Port,
		&p.SrcType, &p.SrcID, &p.DstType, &p.DstID, &p.CreatedBy, &p.CreatedAt, &p.UpdatedBy, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return NetworkPolicy{}, fmt.Errorf("network policy %q not found", id)
	}
	if err != nil {
		return NetworkPolicy{}, err
	}
	p.Enabled = enabled == 1
	return p, nil
}

// ListNetworkPolicies returns every policy ordered by priority ascending —
// the same order the iptables chain is rebuilt in, so lower numbers win.
func (s *Store) ListNetworkPolicies(ctx context.Context) ([]NetworkPolicy, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, enabled, priority, action, protocol, port, src_type, src_id, dst_type, dst_id, created_by, created_at, updated_by, updated_at
		FROM network_policies ORDER BY priority ASC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NetworkPolicy
	for rows.Next() {
		var p NetworkPolicy
		var enabled int
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &enabled, &p.Priority, &p.Action, &p.Protocol, &p.Port,
			&p.SrcType, &p.SrcID, &p.DstType, &p.DstID, &p.CreatedBy, &p.CreatedAt, &p.UpdatedBy, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateNetworkPolicy replaces every mutable field of an existing policy.
func (s *Store) UpdateNetworkPolicy(ctx context.Context, p NetworkPolicy) (NetworkPolicy, error) {
	q := fmt.Sprintf(`
		UPDATE network_policies SET
			name = %s, description = %s, enabled = %s, priority = %s, action = %s, protocol = %s, port = %s,
			src_type = %s, src_id = %s, dst_type = %s, dst_id = %s, updated_by = %s, updated_at = %s
		WHERE id = %s`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11), s.ph(12), s.ph(13), s.ph(14),
	)
	res, err := s.db.ExecContext(ctx, q,
		p.Name, p.Description, boolToInt(p.Enabled), p.Priority, p.Action, p.Protocol, p.Port,
		p.SrcType, p.SrcID, p.DstType, p.DstID, p.UpdatedBy, time.Now().UTC(), p.ID,
	)
	if err != nil {
		return NetworkPolicy{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NetworkPolicy{}, fmt.Errorf("network policy %q not found", p.ID)
	}
	return s.GetNetworkPolicy(ctx, p.ID)
}

func (s *Store) DeleteNetworkPolicy(ctx context.Context, id string) error {
	q := fmt.Sprintf(`DELETE FROM network_policies WHERE id = %s`, s.ph(1))
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("network policy %q not found", id)
	}
	return nil
}

// GetNetworkDefaultDeny reports whether unmatched wg0-to-wg0 traffic should
// be dropped rather than left to fall through (the existing, pre-policy
// behavior). Off by default — turning it on is an explicit opt-in.
func (s *Store) GetNetworkDefaultDeny(ctx context.Context) (bool, error) {
	q := `SELECT default_deny FROM network_policy_settings WHERE id = 1`
	var v int
	err := s.db.QueryRowContext(ctx, q).Scan(&v)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v == 1, nil
}

func (s *Store) SetNetworkDefaultDeny(ctx context.Context, defaultDeny bool, updatedBy string) error {
	var q string
	switch s.driver.(type) {
	case *MySQLDriver:
		q = fmt.Sprintf(`
			INSERT INTO network_policy_settings (id, default_deny, updated_by, updated_at) VALUES (1, %s, %s, %s)
			ON DUPLICATE KEY UPDATE default_deny = VALUES(default_deny), updated_by = VALUES(updated_by), updated_at = VALUES(updated_at)`,
			s.ph(1), s.ph(2), s.ph(3))
	default:
		q = fmt.Sprintf(`
			INSERT INTO network_policy_settings (id, default_deny, updated_by, updated_at) VALUES (1, %s, %s, %s)
			ON CONFLICT (id) DO UPDATE SET default_deny = excluded.default_deny, updated_by = excluded.updated_by, updated_at = excluded.updated_at`,
			s.ph(1), s.ph(2), s.ph(3))
	}
	_, err := s.db.ExecContext(ctx, q, boolToInt(defaultDeny), updatedBy, time.Now().UTC())
	return err
}
