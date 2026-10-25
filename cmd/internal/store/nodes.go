package store

import (
	"context"
	"fmt"
)

func (s *Store) CreateNode(ctx context.Context, n NodeRecord) error {
	status := n.Status
	if status == "" {
		status = "approved"
	}
	q := fmt.Sprintf(
		`INSERT INTO nodes (id, name, type, address, public_key, status) VALUES (%s, %s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6),
	)
	_, err := s.db.ExecContext(ctx, q, n.ID, n.Name, n.Type, n.Address, n.PublicKey, status)
	return err
}

func (s *Store) GetNode(ctx context.Context, id string) (NodeRecord, error) {
	q := fmt.Sprintf(
		`SELECT id, name, type, address, public_key, status FROM nodes WHERE id = %s`, s.ph(1),
	)
	var n NodeRecord
	err := s.db.QueryRowContext(ctx, q, id).Scan(&n.ID, &n.Name, &n.Type, &n.Address, &n.PublicKey, &n.Status)
	if err != nil {
		return NodeRecord{}, fmt.Errorf("node not found")
	}
	return n, nil
}

// UpdateNodeStatus approves or rejects a pending node enrollment.
func (s *Store) UpdateNodeStatus(ctx context.Context, id, status string) error {
	q := fmt.Sprintf(`UPDATE nodes SET status = %s WHERE id = %s`, s.ph(1), s.ph(2))
	res, err := s.db.ExecContext(ctx, q, status, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("node not found")
	}
	return nil
}

func (s *Store) DeleteNode(ctx context.Context, id string) error {
	q := fmt.Sprintf(`DELETE FROM nodes WHERE id = %s`, s.ph(1))
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("node not found")
	}
	return nil
}

func (s *Store) GetNodeByPublicKey(ctx context.Context, publicKey string) (NodeRecord, error) {
	q := fmt.Sprintf(
		`SELECT id, name, type, address, public_key, status FROM nodes WHERE public_key = %s`, s.ph(1),
	)
	var n NodeRecord
	err := s.db.QueryRowContext(ctx, q, publicKey).Scan(&n.ID, &n.Name, &n.Type, &n.Address, &n.PublicKey, &n.Status)
	if err != nil {
		return NodeRecord{}, fmt.Errorf("node not found for public key")
	}
	return n, nil
}
