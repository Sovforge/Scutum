package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type UserRecord struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type APIKeyRecord struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Store) ListUsers(ctx context.Context) ([]UserRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, username, password_hash, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserRecord
	for rows.Next() {
		var u UserRecord
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) GetUser(ctx context.Context, id string) (UserRecord, error) {
	var u UserRecord
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return UserRecord{}, fmt.Errorf("user not found")
	}
	return u, err
}

func (s *Store) UpdateUserUsername(ctx context.Context, id, username string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = ? WHERE id = ?`, username, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (s *Store) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (s *Store) GetUserRoleNames(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.name FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *Store) SetUserRoles(ctx context.Context, userID string, roleIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, rid := range roleIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)`, userID, rid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListAPIKeys(ctx context.Context, userID string) ([]APIKeyRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, expires_at, created_at FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []APIKeyRecord
	for rows.Next() {
		var k APIKeyRecord
		var exp sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &exp, &k.CreatedAt); err != nil {
			return nil, err
		}
		if exp.Valid {
			k.ExpiresAt = &exp.Time
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// TOTP / MFA
// ---------------------------------------------------------------------------

func (s *Store) GetUserTOTP(ctx context.Context, userID string) (secret string, enabled bool, err error) {
	q := fmt.Sprintf(
		`SELECT COALESCE(totp_secret,''), totp_enabled FROM users WHERE id = %s`,
		s.ph(1),
	)
	var enabledInt int
	err = s.db.QueryRowContext(ctx, q, userID).Scan(&secret, &enabledInt)
	enabled = enabledInt != 0
	return
}

func (s *Store) SetUserTOTPSecret(ctx context.Context, userID, secret string) error {
	q := fmt.Sprintf(
		`UPDATE users SET totp_secret = %s, totp_enabled = 0 WHERE id = %s`,
		s.ph(1), s.ph(2),
	)
	_, err := s.db.ExecContext(ctx, q, secret, userID)
	return err
}

func (s *Store) SetUserTOTPEnabled(ctx context.Context, userID string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	q := fmt.Sprintf(
		`UPDATE users SET totp_enabled = %s WHERE id = %s`,
		s.ph(1), s.ph(2),
	)
	_, err := s.db.ExecContext(ctx, q, val, userID)
	return err
}

func (s *Store) DeleteAPIKey(ctx context.Context, id, userID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM api_keys WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("key not found")
	}
	return nil
}
