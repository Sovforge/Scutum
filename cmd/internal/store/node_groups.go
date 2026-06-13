package store

import (
	"context"
	"fmt"
	"time"
)

type NodeLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type NodeGroup struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Members     []string    `json:"members,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

func (s *Store) SetNodeLabels(ctx context.Context, nodeID string, labels map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM node_labels WHERE node_id=%s`, s.ph(1)), nodeID); err != nil {
		return err
	}
	for k, v := range labels {
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO node_labels (node_id,label_key,value) VALUES (%s,%s,%s)`,
				s.ph(1), s.ph(2), s.ph(3)), nodeID, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetNodeLabels(ctx context.Context, nodeID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT label_key,value FROM node_labels WHERE node_id=%s`, s.ph(1)), nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (s *Store) CreateNodeGroup(ctx context.Context, id, name, description string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO node_groups (id,name,description) VALUES (%s,%s,%s)`,
			s.ph(1), s.ph(2), s.ph(3)),
		id, name, description)
	return err
}

func (s *Store) ListNodeGroups(ctx context.Context) ([]NodeGroup, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,name,description,created_at FROM node_groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeGroup
	for rows.Next() {
		var g NodeGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Members, _ = s.listGroupMemberIDs(ctx, out[i].ID)
	}
	return out, nil
}

func (s *Store) GetNodeGroup(ctx context.Context, id string) (NodeGroup, error) {
	var g NodeGroup
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id,name,description,created_at FROM node_groups WHERE id=%s`, s.ph(1)), id).
		Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt)
	if err != nil {
		return NodeGroup{}, err
	}
	g.Members, _ = s.listGroupMemberIDs(ctx, g.ID)
	return g, nil
}

func (s *Store) UpdateNodeGroup(ctx context.Context, id, name, description string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE node_groups SET name=%s,description=%s WHERE id=%s`,
			s.ph(1), s.ph(2), s.ph(3)),
		name, description, id)
	return err
}

func (s *Store) DeleteNodeGroup(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM node_groups WHERE id=%s`, s.ph(1)), id)
	return err
}

func (s *Store) AddNodeToGroup(ctx context.Context, groupID, nodeID string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO node_group_members (group_id,node_id) VALUES (%s,%s)
		             ON CONFLICT DO NOTHING`, s.ph(1), s.ph(2)),
		groupID, nodeID)
	return err
}

func (s *Store) RemoveNodeFromGroup(ctx context.Context, groupID, nodeID string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM node_group_members WHERE group_id=%s AND node_id=%s`,
			s.ph(1), s.ph(2)),
		groupID, nodeID)
	return err
}

func (s *Store) ListNodesInGroup(ctx context.Context, groupID string) ([]NodeRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT n.id,n.name,n.type,n.address,n.public_key
		             FROM nodes n JOIN node_group_members m ON m.node_id=n.id
		             WHERE m.group_id=%s ORDER BY n.name`, s.ph(1)), groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeRecord
	for rows.Next() {
		var n NodeRecord
		if err := rows.Scan(&n.ID, &n.Name, &n.Type, &n.Address, &n.PublicKey); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) listGroupMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT node_id FROM node_group_members WHERE group_id=%s`, s.ph(1)), groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
