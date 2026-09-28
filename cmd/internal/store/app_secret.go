package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AppSecret is a named, user-managed secret. The plaintext value is never
// held on this struct — it lives only in the KMS-sealed "secrets" table,
// under the key "vault:"+Name, reusing the same envelope encryption every
// other internal secret (wg keys, storage credentials, ...) already goes
// through. This table just tracks the metadata needed to list/manage them.
type AppSecret struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedBy   string `json:"updated_by"`
	UpdatedAt   string `json:"updated_at"`
}

func appSecretValueKey(name string) string { return "vault:" + name }

// CreateAppSecret inserts the metadata row and seals the value. Fails if a
// secret with this name already exists.
func (s *Store) CreateAppSecret(ctx context.Context, id, name, description string, value []byte, createdBy string) (AppSecret, error) {
	q := fmt.Sprintf(
		`INSERT INTO app_secrets (id, name, description, created_by, updated_by) VALUES (%s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5),
	)
	if _, err := s.db.ExecContext(ctx, q, id, name, description, createdBy, createdBy); err != nil {
		return AppSecret{}, err
	}
	if err := s.SetSecret(ctx, appSecretValueKey(name), value); err != nil {
		s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM app_secrets WHERE id = %s`, s.ph(1)), id) //nolint
		return AppSecret{}, fmt.Errorf("seal secret value: %w", err)
	}
	return s.GetAppSecretMeta(ctx, name)
}

// UpdateAppSecret updates the description and/or value of an existing secret.
// Pass nil for value to leave it unchanged.
func (s *Store) UpdateAppSecret(ctx context.Context, name string, description *string, value []byte, updatedBy string) (AppSecret, error) {
	if _, err := s.GetAppSecretMeta(ctx, name); err != nil {
		return AppSecret{}, err
	}

	if description != nil {
		q := fmt.Sprintf(`UPDATE app_secrets SET description = %s, updated_by = %s, updated_at = %s WHERE name = %s`, s.ph(1), s.ph(2), s.ph(3), s.ph(4))
		if _, err := s.db.ExecContext(ctx, q, *description, updatedBy, time.Now().UTC(), name); err != nil {
			return AppSecret{}, err
		}
	} else {
		q := fmt.Sprintf(`UPDATE app_secrets SET updated_by = %s, updated_at = %s WHERE name = %s`, s.ph(1), s.ph(2), s.ph(3))
		if _, err := s.db.ExecContext(ctx, q, updatedBy, time.Now().UTC(), name); err != nil {
			return AppSecret{}, err
		}
	}

	if value != nil {
		if err := s.SetSecret(ctx, appSecretValueKey(name), value); err != nil {
			return AppSecret{}, fmt.Errorf("seal secret value: %w", err)
		}
	}
	return s.GetAppSecretMeta(ctx, name)
}

func (s *Store) GetAppSecretMeta(ctx context.Context, name string) (AppSecret, error) {
	q := fmt.Sprintf(
		`SELECT id, name, description, created_by, created_at, updated_by, updated_at FROM app_secrets WHERE name = %s`,
		s.ph(1),
	)
	var a AppSecret
	err := s.db.QueryRowContext(ctx, q, name).Scan(&a.ID, &a.Name, &a.Description, &a.CreatedBy, &a.CreatedAt, &a.UpdatedBy, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return AppSecret{}, fmt.Errorf("secret %q not found", name)
	}
	if err != nil {
		return AppSecret{}, err
	}
	return a, nil
}

func (s *Store) ListAppSecrets(ctx context.Context) ([]AppSecret, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, created_by, created_at, updated_by, updated_at FROM app_secrets ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AppSecret
	for rows.Next() {
		var a AppSecret
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.CreatedBy, &a.CreatedAt, &a.UpdatedBy, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAppSecretValue returns the decrypted plaintext value. Callers must audit
// every use of this themselves — it is the one path that leaves KMS sealing.
func (s *Store) GetAppSecretValue(ctx context.Context, name string) ([]byte, error) {
	if _, err := s.GetAppSecretMeta(ctx, name); err != nil {
		return nil, err
	}
	return s.GetSecret(ctx, appSecretValueKey(name))
}

func (s *Store) DeleteAppSecret(ctx context.Context, name string) error {
	q := fmt.Sprintf(`DELETE FROM app_secrets WHERE name = %s`, s.ph(1))
	res, err := s.db.ExecContext(ctx, q, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("secret %q not found", name)
	}
	s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM secrets WHERE key = %s`, s.ph(1)), appSecretValueKey(name)) //nolint
	return nil
}
