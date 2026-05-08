package store

import (
	"context"
	"fmt"
)

func (s *Store) CreateNode(ctx context.Context, n NodeRecord) error {
	q := fmt.Sprintf(
		`INSERT INTO nodes (id, name, type, address, public_key) VALUES (%s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5),
	)
	_, err := s.db.ExecContext(ctx, q, n.ID, n.Name, n.Type, n.Address, n.PublicKey)
	return err
}

func (s *Store) GetNode(ctx context.Context, id string) (NodeRecord, error) {
	q := fmt.Sprintf(
		`SELECT id, name, type, address, public_key FROM nodes WHERE id = %s`, s.ph(1),
	)
	var n NodeRecord
	err := s.db.QueryRowContext(ctx, q, id).Scan(&n.ID, &n.Name, &n.Type, &n.Address, &n.PublicKey)
	if err != nil {
		return NodeRecord{}, fmt.Errorf("node not found")
	}
	return n, nil
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
