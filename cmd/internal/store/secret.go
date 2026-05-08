package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"scutum/cmd/internal/kms"
)

func (s *Store) SetSecret(ctx context.Context, key string, value []byte) error {
	s.mu.RLock()
	provider := s.kms
	s.mu.RUnlock()

	d, err := kms.Seal(ctx, provider, value)
	if err != nil {
		return fmt.Errorf("seal secret %s: %w", key, err)
	}

	var q string
	switch s.driver.(type) {
	case *MySQLDriver:
		q = fmt.Sprintf(`
			INSERT INTO secrets (`+"`key`"+`, dek_encrypted, value_encrypted, provider, updated_at)
			VALUES (%s, %s, %s, %s, %s)
			ON DUPLICATE KEY UPDATE
				dek_encrypted   = VALUES(dek_encrypted),
				value_encrypted = VALUES(value_encrypted),
				provider        = VALUES(provider),
				updated_at      = VALUES(updated_at)
		`, s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5))
	default:
		q = fmt.Sprintf(`
			INSERT INTO secrets (key, dek_encrypted, value_encrypted, provider, updated_at)
			VALUES (%s, %s, %s, %s, %s)
			ON CONFLICT(key) DO UPDATE SET
				dek_encrypted   = excluded.dek_encrypted,
				value_encrypted = excluded.value_encrypted,
				provider        = excluded.provider,
				updated_at      = excluded.updated_at
		`, s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5))
	}

	_, err = s.db.ExecContext(ctx, q, key, d.EncryptedKey, d.Ciphertext, s.kms.Name(), time.Now())
	return err
}

func (s *Store) GetSecret(ctx context.Context, key string) ([]byte, error) {
	var d kms.DEK
	var providerName string
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT dek_encrypted, value_encrypted, provider FROM secrets WHERE key = %s`, s.ph(1)), key,
	).Scan(&d.EncryptedKey, &d.Ciphertext, &providerName)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("secret %q not found", key)
	}
	if err != nil {
		return nil, fmt.Errorf("query secret %s: %w", key, err)
	}
	_ = providerName // Provider name stored for future use, but we use current KMS provider
	return kms.Open(ctx, s.kms, d)
}

func (s *Store) RotateKeys(ctx context.Context, oldProvider kms.Provider) error {
	rows, err := s.db.QueryContext(ctx, `SELECT key, dek_encrypted FROM secrets`)
	if err != nil {
		return fmt.Errorf("rotate query: %w", err)
	}
	defer rows.Close()

	type entry struct {
		key          string
		dekEncrypted []byte
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.key, &e.dekEncrypted); err != nil {
			return err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	updateQ := fmt.Sprintf(
		`UPDATE secrets SET dek_encrypted = %s, provider = %s, updated_at = %s WHERE key = %s`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4),
	)
	for _, e := range entries {
		newDEK, err := kms.ReWrap(ctx, oldProvider, s.kms, e.dekEncrypted)
		if err != nil {
			return fmt.Errorf("rewrap secret %s: %w", e.key, err)
		}
		_, err = s.db.ExecContext(ctx, updateQ, newDEK, s.kms.Name(), time.Now(), e.key)
		if err != nil {
			return fmt.Errorf("update secret %s: %w", e.key, err)
		}
	}
	return nil
}

func (s *Store) ReEncryptAllDEKs(ctx context.Context, oldMasterKey, newMasterKey []byte) error {
	oldProvider := &localKeyAdapter{key: oldMasterKey}
	newProvider := &localKeyAdapter{key: newMasterKey}

	rows, err := s.db.QueryContext(ctx, `SELECT key, dek_encrypted FROM secrets`)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	type entry struct {
		key          string
		dekEncrypted []byte
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.key, &e.dekEncrypted); err != nil {
			return err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	updateQ := fmt.Sprintf(
		`UPDATE secrets SET dek_encrypted = %s, provider = %s, updated_at = %s WHERE key = %s`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4),
	)
	for _, e := range entries {
		newDEK, err := kms.ReWrap(ctx, oldProvider, newProvider, e.dekEncrypted)
		if err != nil {
			return fmt.Errorf("re-encrypt DEK for %s: %w", e.key, err)
		}
		_, err = s.db.ExecContext(ctx, updateQ, newDEK, s.kms.Name(), time.Now(), e.key)
		if err != nil {
			return fmt.Errorf("update secret %s: %w", e.key, err)
		}
	}
	return nil
}

type localKeyAdapter struct {
	key []byte
}

func (a *localKeyAdapter) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (a *localKeyAdapter) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

func (a *localKeyAdapter) Name() string { return "adapter" }
func (a *localKeyAdapter) Wipe() {}
func (a *localKeyAdapter) LoadMasterKey(key []byte) error { return nil }
func (a *localKeyAdapter) MasterKeyDigest() ([]byte, error) { return a.key[:16], nil }
