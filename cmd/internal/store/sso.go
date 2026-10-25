package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) UserByID(ctx context.Context, id string) (username string, err error) {
	q := fmt.Sprintf(`SELECT username FROM users WHERE id = %s`, s.ph(1))
	err = s.db.QueryRowContext(ctx, q, id).Scan(&username)
	return
}

func (s *Store) UserByEmail(ctx context.Context, email string) (id, username string, err error) {
	q := fmt.Sprintf(`SELECT id, username FROM users WHERE email = %s`, s.ph(1))
	err = s.db.QueryRowContext(ctx, q, email).Scan(&id, &username)
	return
}

func (s *Store) CreateUserWithEmail(ctx context.Context, id, username, email string) error {
	q := fmt.Sprintf(
		`INSERT INTO users (id, username, password_hash, email) VALUES (%s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4),
	)
	_, err := s.db.ExecContext(ctx, q, id, username, "", email)
	return err
}

func (s *Store) UpsertSSOIdentity(ctx context.Context, id, userID, provider, subject, email string) error {
	switch s.driver.(type) {
	case *MySQLDriver:
		q := fmt.Sprintf(`
			INSERT INTO sso_identities (id, user_id, provider, subject, email)
			VALUES (%s, %s, %s, %s, %s)
			ON DUPLICATE KEY UPDATE user_id=VALUES(user_id), email=VALUES(email)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5),
		)
		_, err := s.db.ExecContext(ctx, q, id, userID, provider, subject, email)
		return err
	default:
		q := fmt.Sprintf(`
			INSERT INTO sso_identities (id, user_id, provider, subject, email)
			VALUES (%s, %s, %s, %s, %s)
			ON CONFLICT (provider, subject) DO UPDATE SET user_id=EXCLUDED.user_id, email=EXCLUDED.email`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5),
		)
		_, err := s.db.ExecContext(ctx, q, id, userID, provider, subject, email)
		return err
	}
}

func (s *Store) UserBySSOIdentity(ctx context.Context, provider, subject string) (userID string, err error) {
	q := fmt.Sprintf(
		`SELECT user_id FROM sso_identities WHERE provider = %s AND subject = %s`,
		s.ph(1), s.ph(2),
	)
	err = s.db.QueryRowContext(ctx, q, provider, subject).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", sql.ErrNoRows
	}
	return
}
