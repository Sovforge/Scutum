package store

import (
	"context"
	"fmt"
)

type StorageBackend struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key"`
	PathStyle bool   `json:"path_style"`
	UseSSL    bool   `json:"use_ssl"`
	CreatedAt string `json:"created_at,omitempty"`
}

func (s *Store) CreateStorageBackend(ctx context.Context, b StorageBackend, secretKey string) error {
	pathStyle := 0
	if b.PathStyle {
		pathStyle = 1
	}
	useSSL := 0
	if b.UseSSL {
		useSSL = 1
	}
	q := fmt.Sprintf(
		`INSERT INTO storage_backends (id, name, provider, endpoint, region, access_key, path_style, use_ssl)
		 VALUES (%s, %s, %s, %s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8),
	)
	if _, err := s.db.ExecContext(ctx, q, b.ID, b.Name, b.Provider, b.Endpoint, b.Region, b.AccessKey, pathStyle, useSSL); err != nil {
		return err
	}
	if secretKey != "" {
		return s.SetSecret(ctx, "storage:"+b.ID+":secret_key", []byte(secretKey))
	}
	return nil
}

func (s *Store) ListStorageBackends(ctx context.Context) ([]StorageBackend, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, provider, endpoint, region, access_key, path_style, use_ssl, created_at
		 FROM storage_backends ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backends []StorageBackend
	for rows.Next() {
		var b StorageBackend
		var pathStyle, useSSL int
		if err := rows.Scan(&b.ID, &b.Name, &b.Provider, &b.Endpoint, &b.Region, &b.AccessKey, &pathStyle, &useSSL, &b.CreatedAt); err != nil {
			return nil, err
		}
		b.PathStyle = pathStyle == 1
		b.UseSSL = useSSL == 1
		backends = append(backends, b)
	}
	return backends, rows.Err()
}

func (s *Store) GetStorageBackendWithSecret(ctx context.Context, id string) (StorageBackend, string, error) {
	q := fmt.Sprintf(
		`SELECT id, name, provider, endpoint, region, access_key, path_style, use_ssl
		 FROM storage_backends WHERE id = %s`, s.ph(1),
	)
	var b StorageBackend
	var pathStyle, useSSL int
	if err := s.db.QueryRowContext(ctx, q, id).Scan(
		&b.ID, &b.Name, &b.Provider, &b.Endpoint, &b.Region, &b.AccessKey, &pathStyle, &useSSL,
	); err != nil {
		return StorageBackend{}, "", fmt.Errorf("storage backend not found")
	}
	b.PathStyle = pathStyle == 1
	b.UseSSL = useSSL == 1

	secretBytes, err := s.GetSecret(ctx, "storage:"+id+":secret_key")
	if err != nil {
		return b, "", nil
	}
	return b, string(secretBytes), nil
}

func (s *Store) DeleteStorageBackend(ctx context.Context, id string) error {
	q := fmt.Sprintf(`DELETE FROM storage_backends WHERE id = %s`, s.ph(1))
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("storage backend not found")
	}
	s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM secrets WHERE key = %s`, s.ph(1)), "storage:"+id+":secret_key") //nolint
	return nil
}
