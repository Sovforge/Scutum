package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RecoveryCodeRecord struct {
	ID        string
	UserID    string
	Used      bool
	UsedAt    *time.Time
	CreatedAt time.Time
}

// CreateRecoveryCodes stores hashed recovery codes for a user, replacing any existing ones.
func (s *Store) CreateRecoveryCodes(ctx context.Context, userID string, codeHashes []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM recovery_codes WHERE user_id = %s`, s.ph(1)), userID,
	); err != nil {
		return err
	}

	for _, hash := range codeHashes {
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO recovery_codes (id, user_id, code_hash) VALUES (%s, %s, %s)`,
				s.ph(1), s.ph(2), s.ph(3)),
			uuid.New().String(), userID, hash,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UseRecoveryCode marks a code as used if it exists, is unused, and belongs to the user.
// Returns an error if the code is not found or already used.
func (s *Store) UseRecoveryCode(ctx context.Context, userID, codeHash string) error {
	var id string
	var used int
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id, used FROM recovery_codes WHERE user_id = %s AND code_hash = %s`,
			s.ph(1), s.ph(2)),
		userID, codeHash,
	).Scan(&id, &used)
	if err == sql.ErrNoRows {
		return fmt.Errorf("recovery code not found")
	}
	if err != nil {
		return err
	}
	if used != 0 {
		return fmt.Errorf("recovery code already used")
	}

	_, err = s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE recovery_codes SET used = 1, used_at = %s WHERE id = %s`,
			s.ph(1), s.ph(2)),
		time.Now(), id,
	)
	return err
}

// CountRemainingRecoveryCodes returns the number of unused recovery codes for a user.
func (s *Store) CountRemainingRecoveryCodes(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM recovery_codes WHERE user_id = %s AND used = 0`, s.ph(1)),
		userID,
	).Scan(&count)
	return count, err
}
