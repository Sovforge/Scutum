package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AcquireHubLease attempts to claim the single-row hub leader lease.
// Returns true if this holder successfully owns the lease, false if another
// instance holds a valid (non-expired) lease.
func (s *Store) AcquireHubLease(ctx context.Context, holderID string, ttl time.Duration) (bool, error) {
	expiresAt := time.Now().Add(ttl)

	// Try to insert a brand-new lease row.
	var insertQ string
	switch s.driver.(type) {
	case *MySQLDriver:
		insertQ = fmt.Sprintf(
			"INSERT IGNORE INTO hub_leases (id, holder_id, expires_at, updated_at) VALUES (1, %s, %s, %s)",
			s.ph(1), s.ph(2), s.ph(3),
		)
	default:
		insertQ = fmt.Sprintf(
			"INSERT INTO hub_leases (id, holder_id, expires_at, updated_at) VALUES (1, %s, %s, %s) ON CONFLICT(id) DO NOTHING",
			s.ph(1), s.ph(2), s.ph(3),
		)
	}

	res, err := s.db.ExecContext(ctx, insertQ, holderID, expiresAt, time.Now())
	if err != nil {
		return false, fmt.Errorf("insert hub lease: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows > 0 {
		// We inserted the row — we own the lease.
		return true, nil
	}

	// Row already exists; steal it if the current lease has expired.
	updateQ := fmt.Sprintf(
		"UPDATE hub_leases SET holder_id = %s, expires_at = %s, updated_at = %s WHERE id = 1 AND expires_at < %s",
		s.ph(1), s.ph(2), s.ph(3), s.ph(4),
	)
	res, err = s.db.ExecContext(ctx, updateQ, holderID, expiresAt, time.Now(), time.Now())
	if err != nil {
		return false, fmt.Errorf("steal hub lease: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows > 0 {
		return true, nil
	}

	// Check if we already own it (e.g. restart before lease expired).
	var currentHolder string
	err = s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT holder_id FROM hub_leases WHERE id = 1 AND holder_id = %s AND expires_at >= %s",
			s.ph(1), s.ph(2)),
		holderID, time.Now(),
	).Scan(&currentHolder)
	if err == nil && currentHolder == holderID {
		return true, nil
	}

	return false, nil
}

// RenewHubLease extends the expiry of an existing lease held by holderID.
func (s *Store) RenewHubLease(ctx context.Context, holderID string, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)
	q := fmt.Sprintf(
		"UPDATE hub_leases SET expires_at = %s, updated_at = %s WHERE id = 1 AND holder_id = %s",
		s.ph(1), s.ph(2), s.ph(3),
	)
	res, err := s.db.ExecContext(ctx, q, expiresAt, time.Now(), holderID)
	if err != nil {
		return fmt.Errorf("renew hub lease: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("lease lost: not renewed (holder_id mismatch or row missing)")
	}
	return nil
}

// ReleaseHubLease releases the lease held by holderID by setting an expired timestamp.
func (s *Store) ReleaseHubLease(ctx context.Context, holderID string) error {
	q := fmt.Sprintf(
		"UPDATE hub_leases SET expires_at = %s WHERE id = 1 AND holder_id = %s",
		s.ph(1), s.ph(2),
	)
	_, err := s.db.ExecContext(ctx, q, time.Now().Add(-time.Second), holderID)
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}
