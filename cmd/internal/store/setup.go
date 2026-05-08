package store

import (
	"context"
	"fmt"
)

type InstallType string

const (
	InstallHub      InstallType = "hub"
	InstallRemote   InstallType = "remote"
	InstallCombined InstallType = "combined"
)

func (s *Store) IsSetupComplete(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM setup_state WHERE key = 'complete' AND value = 'true'`,
	).Scan(&count)
	return count > 0, err
}

func (s *Store) MarkSetupComplete(ctx context.Context) error {
	var q string
	switch s.driver.(type) {
	case *MySQLDriver:
		q = "INSERT INTO setup_state (`key`, value) VALUES ('complete', 'true') ON DUPLICATE KEY UPDATE value = 'true'"
	default:
		q = "INSERT INTO setup_state (key, value) VALUES ('complete', 'true') ON CONFLICT(key) DO UPDATE SET value = 'true'"
	}
	_, err := s.db.ExecContext(ctx, q)
	return err
}

func (s *Store) SetKMSProvider(ctx context.Context, provider string) error {
	var q string
	switch s.driver.(type) {
	case *MySQLDriver:
		q = fmt.Sprintf("INSERT INTO setup_state (`key`, value) VALUES ('kms_provider', %s) ON DUPLICATE KEY UPDATE value = VALUES(value)", s.ph(1))
	default:
		q = fmt.Sprintf("INSERT INTO setup_state (key, value) VALUES ('kms_provider', %s) ON CONFLICT(key) DO UPDATE SET value = excluded.value", s.ph(1))
	}
	_, err := s.db.ExecContext(ctx, q, provider)
	return err
}

func (s *Store) GetKMSProvider(ctx context.Context) (string, error) {
	var provider string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM setup_state WHERE key = 'kms_provider'`,
	).Scan(&provider)
	if err != nil {
		return "", fmt.Errorf("kms provider not set: %w", err)
	}
	return provider, nil
}

func (s *Store) SetInstallType(ctx context.Context, t InstallType) error {
	var q string
	switch s.driver.(type) {
	case *MySQLDriver:
		q = fmt.Sprintf("INSERT INTO setup_state (`key`, value) VALUES ('install_type', %s) ON DUPLICATE KEY UPDATE value = VALUES(value)", s.ph(1))
	default:
		q = fmt.Sprintf("INSERT INTO setup_state (key, value) VALUES ('install_type', %s) ON CONFLICT(key) DO UPDATE SET value = excluded.value", s.ph(1))
	}
	_, err := s.db.ExecContext(ctx, q, string(t))
	return err
}

func (s *Store) GetInstallType(ctx context.Context) (InstallType, error) {
	var t string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM setup_state WHERE key = 'install_type'`,
	).Scan(&t)
	if err != nil {
		return "", fmt.Errorf("install type not set: %w", err)
	}
	return InstallType(t), nil
}

func NewDriver(name string) (Driver, error) {
	switch name {
	case "sqlite":
		return SQLiteDriver{}, nil
	case "postgres":
		return PostgresDriver{}, nil
	case "mysql":
		return MySQLDriver{}, nil
	default:
		return nil, fmt.Errorf("unknown database driver: %q", name)
	}
}
