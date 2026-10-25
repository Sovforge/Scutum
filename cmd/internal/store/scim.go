package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

type SCIMUser struct {
	ID           string
	Username     string
	Email        string
	Active       bool
	CreatedAt    time.Time
}

type SCIMTokenInfo struct {
	ID          string
	Description string
	CreatedAt   time.Time
}

func (s *Store) SCIMListUsers(ctx context.Context) ([]SCIMUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, username, COALESCE(email,''), COALESCE(disabled,0), created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SCIMUser
	for rows.Next() {
		var u SCIMUser
		var disabled int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &disabled, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Active = disabled == 0
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) SCIMGetUser(ctx context.Context, id string) (SCIMUser, error) {
	var u SCIMUser
	var disabled int
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id,username,COALESCE(email,''),COALESCE(disabled,0),created_at FROM users WHERE id=%s`, s.ph(1)), id).
		Scan(&u.ID, &u.Username, &u.Email, &disabled, &u.CreatedAt)
	if err != nil {
		return SCIMUser{}, err
	}
	u.Active = disabled == 0
	return u, nil
}

func (s *Store) SCIMCreateUser(ctx context.Context, id, username, email, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO users (id,username,password_hash,email) VALUES (%s,%s,%s,%s)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4)),
		id, username, passwordHash, email)
	return err
}

func (s *Store) SCIMUpdateUser(ctx context.Context, id, username, email string, active bool) error {
	disabled := 0
	if !active {
		disabled = 1
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE users SET username=%s,email=%s,disabled=%s WHERE id=%s`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4)),
		username, email, disabled, id)
	return err
}

func (s *Store) SCIMDeleteUser(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM users WHERE id=%s`, s.ph(1)), id)
	return err
}

func (s *Store) CreateSCIMToken(ctx context.Context, id, tokenHash, description string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO scim_tokens (id,token_hash,description) VALUES (%s,%s,%s)`,
			s.ph(1), s.ph(2), s.ph(3)),
		id, tokenHash, description)
	return err
}

func (s *Store) ValidateSCIMToken(ctx context.Context, rawToken string) (bool, error) {
	h := sha256.Sum256([]byte(rawToken))
	hash := fmt.Sprintf("%x", h)
	var count int
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM scim_tokens WHERE token_hash=%s`, s.ph(1)), hash).Scan(&count)
	return count > 0, err
}

func (s *Store) ListSCIMTokens(ctx context.Context) ([]SCIMTokenInfo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,description,created_at FROM scim_tokens ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SCIMTokenInfo
	for rows.Next() {
		var t SCIMTokenInfo
		if err := rows.Scan(&t.ID, &t.Description, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSCIMToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM scim_tokens WHERE id=%s`, s.ph(1)), id)
	return err
}
