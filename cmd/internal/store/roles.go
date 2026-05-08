package store

import (
	"context"
	"fmt"
)

type RoleRecord struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Perms       []string `json:"perms"`
}

func (s *Store) ListRoles(ctx context.Context) ([]RoleRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, COALESCE(description,'') FROM roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RoleRecord
	for rows.Next() {
		var r RoleRecord
		if err := rows.Scan(&r.ID, &r.Name, &r.Description); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		perms, err := s.ListRolePerms(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Perms = perms
	}
	return out, nil
}

func (s *Store) CreateRole(ctx context.Context, id, name, description string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO roles (id, name, description) VALUES (?, ?, ?)`,
		id, name, description)
	return err
}

func (s *Store) UpdateRole(ctx context.Context, id, name, description string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE roles SET name = ?, description = ? WHERE id = ?`, name, description, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("role not found")
	}
	return nil
}

func (s *Store) DeleteRole(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM roles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("role not found")
	}
	return nil
}

func (s *Store) ListRolePerms(ctx context.Context, roleID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.name FROM permissions p
		JOIN role_permissions rp ON rp.permission_id = p.id
		WHERE rp.role_id = ?`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (s *Store) SetRolePerms(ctx context.Context, roleID string, permNames []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM role_permissions WHERE role_id = ?`, roleID); err != nil {
		return err
	}

	for _, name := range permNames {
		var permID string
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM permissions WHERE name = ?`, name).Scan(&permID)
		if err != nil {
			// Insert unknown permission dynamically
			permID = "perm_dyn_" + name
			res, action := splitPerm(name)
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO permissions (id, name, resource, action) VALUES (?, ?, ?, ?)`,
				permID, name, res, action); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO role_permissions (role_id, permission_id) VALUES (?, ?)`,
			roleID, permID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func splitPerm(name string) (resource, action string) {
	for i, c := range name {
		if c == ':' {
			return name[:i], name[i+1:]
		}
	}
	return name, ""
}
