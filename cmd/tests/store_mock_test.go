package tests

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"scutum/cmd/internal/kms"
	"scutum/cmd/internal/store"
)

type mockDriver struct {
	placeholder string
	migrateFn   func(ctx context.Context, db *sql.DB) error
}

func (d *mockDriver) Open(dsn string) (*sql.DB, error) {
	return nil, nil
}

func (d *mockDriver) Migrate(ctx context.Context, db *sql.DB) error {
	if d.migrateFn != nil {
		return d.migrateFn(ctx, db)
	}
	return nil
}

func (d *mockDriver) Placeholder(n int) string {
	return d.placeholder
}

func TestStoreWithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS users").
		WillReturnResult(sqlmock.NewResult(0, 0))

	drv := &mockDriver{
		placeholder: "?",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			_, err := db.Exec("CREATE TABLE IF NOT EXISTS users (id VARCHAR(255) PRIMARY KEY)")
			return err
		},
	}

	s, err := store.NewWithDB(context.Background(), db, nil, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}
	if s == nil {
		t.Fatal("expected store, got nil")
	}
	s.Close()
}

func TestMockStoreUserOperations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO users").
		WithArgs("user-123", "testuser", "hash123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	drv := &mockDriver{
		placeholder: "?",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			return nil
		},
	}

	s, err := store.NewWithDB(context.Background(), db, nil, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}

	err = s.CreateUser(context.Background(), "user-123", "testuser", "hash123")
	if err != nil {
		t.Errorf("CreateUser failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
	s.Close()
}

func TestMockStoreS3Config(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO s3_config").
		WithArgs("s3.amazonaws.com", "mybucket", "us-east-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO secrets").
		WillReturnResult(sqlmock.NewResult(1, 1))

	drv := &mockDriver{
		placeholder: "?",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			return nil
		},
	}

	s, err := store.NewWithDB(context.Background(), db, nil, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}

	err = s.SetS3Config(context.Background(), store.S3Config{
		Endpoint:  "s3.amazonaws.com",
		Bucket:    "mybucket",
		Region:    "us-east-1",
		AccessKey: "access",
		SecretKey: "secret",
	})
	if err != nil {
		t.Logf("SetS3Config: %v", err)
	}

	s.Close()
}

func TestMockStoreListPlugins(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"name", "path"}).
		AddRow("plugin1", "/path/to/plugin1.wasm").
		AddRow("plugin2", "/path/to/plugin2.wasm")

	mock.ExpectQuery("SELECT name, path FROM plugins").
		WillReturnRows(rows)

	drv := &mockDriver{
		placeholder: "?",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			return nil
		},
	}

	s, err := store.NewWithDB(context.Background(), db, nil, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}

	plugins, err := s.ListEnabledPlugins(context.Background())
	if err != nil {
		t.Errorf("ListEnabledPlugins failed: %v", err)
	}
	if len(plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(plugins))
	}

	s.Close()
}

func TestMockStoreAssignRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO user_roles").
		WithArgs("user-123", "role_admin").
		WillReturnResult(sqlmock.NewResult(1, 1))

	drv := &mockDriver{
		placeholder: "?",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			return nil
		},
	}

	s, err := store.NewWithDB(context.Background(), db, nil, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}

	err = s.AssignRole(context.Background(), "user-123", "role_admin")
	if err != nil {
		t.Errorf("AssignRole failed: %v", err)
	}

	s.Close()
}

func TestMockStoreAPIKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO api_keys").
		WithArgs("key-123", "user-123", "test-key", "keyhash123", nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	drv := &mockDriver{
		placeholder: "?",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			return nil
		},
	}

	s, err := store.NewWithDB(context.Background(), db, nil, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}

	err = s.CreateAPIKey(context.Background(), "key-123", "user-123", "test-key", "keyhash123", nil)
	if err != nil {
		t.Errorf("CreateAPIKey failed: %v", err)
	}

	s.Close()
}

func TestMySQLDriver(t *testing.T) {
	drv := store.MySQLDriver{}

	placeholder := drv.Placeholder(1)
	if placeholder != "?" {
		t.Errorf("Placeholder = %q, want ?", placeholder)
	}
}

func TestPostgresDriver(t *testing.T) {
	drv := store.PostgresDriver{}

	placeholder := drv.Placeholder(1)
	if placeholder != "$1" {
		t.Errorf("Placeholder = %q, want $1", placeholder)
	}
}

func TestMockStoreMigrate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS users").
		WillReturnResult(sqlmock.NewResult(0, 0))

	drv := &mockDriver{
		placeholder: "?",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			_, err := db.Exec("CREATE TABLE IF NOT EXISTS users (id VARCHAR(255) PRIMARY KEY)")
			return err
		},
	}

	s, err := store.NewWithDB(context.Background(), db, nil, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}
	s.Close()
}

func TestMockStoreKMSProvider(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO setup_state").WillReturnResult(sqlmock.NewResult(1, 1))

	drv := &mockDriver{
		placeholder: "$1",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			return nil
		},
	}

	s, err := store.NewWithDB(context.Background(), db, nil, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}

	err = s.SetKMSProvider(context.Background(), "local")
	if err != nil {
		t.Logf("SetKMSProvider: %v", err)
	}

	s.Close()
}

func TestMockStoreInstallType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO setup_state").WillReturnResult(sqlmock.NewResult(1, 1))

	drv := &mockDriver{
		placeholder: "$1",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			return nil
		},
	}

	s, err := store.NewWithDB(context.Background(), db, nil, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}

	err = s.SetInstallType(context.Background(), store.InstallHub)
	if err != nil {
		t.Logf("SetInstallType: %v", err)
	}

	s.Close()
}

func TestNewDriver(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		wantErr bool
	}{
		{
			name:    "sqlite",
			driver:  "sqlite",
			wantErr: false,
		},
		{
			name:    "postgres",
			driver:  "postgres",
			wantErr: false,
		},
		{
			name:    "mysql",
			driver:  "mysql",
			wantErr: false,
		},
		{
			name:    "unknown",
			driver:  "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := store.NewDriver(tt.driver)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if d == nil {
				t.Error("expected driver, got nil")
			}
		})
	}
}

func TestStoreWithKMS(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	drv := &mockDriver{
		placeholder: "?",
		migrateFn: func(ctx context.Context, db *sql.DB) error {
			return nil
		},
	}

	localKMS, err := kms.NewLocalKeyProvider("/tmp/test.key")
	if err != nil {
		t.Skipf("LocalKeyProvider: %v", err)
	}

	s, err := store.NewWithDB(context.Background(), db, localKMS, drv)
	if err != nil {
		t.Fatalf("NewWithDB failed: %v", err)
	}

	swapKMS, err := kms.NewLocalKeyProvider("/tmp/test2.key")
	if err != nil {
		t.Skipf("LocalKeyProvider: %v", err)
	}

	s.SwapKMS(swapKMS)

	s.Close()
}
