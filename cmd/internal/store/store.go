package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"scutum/cmd/internal/kms"
)

// DriverType identifies the database backend to use.
type DriverType string

const (
	DriverSQLite   DriverType = "sqlite"
	DriverPostgres DriverType = "postgres"
	DriverMySQL    DriverType = "mysql"
)

// Config holds all information needed to open and configure the database.
// No environment variables are read; the caller populates this struct directly.
type Config struct {
	// Driver selects the database backend.
	Driver DriverType

	// DSN is the data source name / connection string for the chosen driver.
	//   SQLite:   "/var/lib/scutum/data.db"  or  "file::memory:?cache=shared"
	//   Postgres: "host=localhost port=5432 user=orc password=secret dbname=orc sslmode=disable"
	//   MySQL:    "orc:secret@tcp(localhost:3306)/orc?parseTime=true"
	DSN string
}

// newDriver returns the Driver implementation for the given DriverType.
func newDriver(dt DriverType) (Driver, error) {
	switch dt {
	case DriverSQLite:
		return &SQLiteDriver{}, nil
	case DriverPostgres:
		return &PostgresDriver{}, nil
	case DriverMySQL:
		return &MySQLDriver{}, nil
	default:
		return nil, fmt.Errorf("unknown driver %q: choose sqlite, postgres, or mysql", dt)
	}
}

// Store is the single access point for all persistent data.
// It is safe for concurrent use.
type Store struct {
	db     *sql.DB
	kms    kms.Provider
	driver Driver
	mu     sync.RWMutex
}

// New opens the database described by cfg, runs migrations, and returns a
// ready-to-use Store. kmsProvider may be nil if secrets are not needed.
// The dsnOrConfig parameter can be either a string (interpreted as SQLite DSN)
// or a Config struct for full control.
func New(ctx context.Context, dsnOrConfig interface{}, kmsProvider kms.Provider) (*Store, error) {
	var cfg Config
	switch v := dsnOrConfig.(type) {
	case string:
		cfg = Config{Driver: DriverSQLite, DSN: v}
	case Config:
		cfg = v
	default:
		return nil, fmt.Errorf("invalid store config type: %T", dsnOrConfig)
	}
	drv, err := newDriver(cfg.Driver)
	if err != nil {
		return nil, err
	}

	db, err := drv.Open(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	s := &Store{db: db, kms: kmsProvider, driver: drv}

	if err := drv.Migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// NewWithDB creates a Store with an existing database connection.
// This is useful for testing with mock databases (e.g., sqlmock).
func NewWithDB(ctx context.Context, db *sql.DB, kmsProvider kms.Provider, driver Driver) (*Store, error) {
	s := &Store{db: db, kms: kmsProvider, driver: driver}
	if err := driver.Migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close releases the underlying database connection pool.
func (s *Store) Close() error { return s.db.Close() }

// SwapKMS atomically replaces the KMS provider (e.g. after a key rotation).
func (s *Store) SwapKMS(provider kms.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kms = provider
}

// CurrentKMS returns the active KMS provider.
func (s *Store) CurrentKMS() kms.Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.kms
}

// ph is a convenience shorthand for s.driver.Placeholder(n).
func (s *Store) ph(n int) string { return s.driver.Placeholder(n) }

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (s *Store) CreateUser(ctx context.Context, id, username, passwordHash string) error {
	q := fmt.Sprintf(
		`INSERT INTO users (id, username, password_hash) VALUES (%s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3),
	)
	_, err := s.db.ExecContext(ctx, q, id, username, passwordHash)
	return err
}

func (s *Store) UserByUsername(ctx context.Context, username string) (id, passwordHash string, err error) {
	q := fmt.Sprintf(`SELECT id, password_hash FROM users WHERE username = %s`, s.ph(1))
	err = s.db.QueryRowContext(ctx, q, username).Scan(&id, &passwordHash)
	return
}

func (s *Store) UserByAPIKey(keyHash string) (userID, username string, err error) {
	q := fmt.Sprintf(`
		SELECT u.id, u.username FROM api_keys k
		JOIN users u ON u.id = k.user_id
		WHERE k.key_hash = %s
		AND (k.expires_at IS NULL OR k.expires_at > CURRENT_TIMESTAMP)
	`, s.ph(1))
	err = s.db.QueryRowContext(context.Background(), q, keyHash).Scan(&userID, &username)
	return
}

func (s *Store) UserHasPermission(userID, resource, action string) (bool, error) {
	q := fmt.Sprintf(`
		SELECT COUNT(*) FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_id = %s
		AND p.resource = %s
		AND p.action = %s
	`, s.ph(1), s.ph(2), s.ph(3))
	var count int
	err := s.db.QueryRowContext(context.Background(), q, userID, resource, action).Scan(&count)
	return count > 0, err
}

// ---------------------------------------------------------------------------
// API keys
// ---------------------------------------------------------------------------

func (s *Store) CreateAPIKey(ctx context.Context, id, userID, name, keyHash string, expiresAt *time.Time) error {
	q := fmt.Sprintf(
		`INSERT INTO api_keys (id, user_id, name, key_hash, expires_at) VALUES (%s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5),
	)
	_, err := s.db.ExecContext(ctx, q, id, userID, name, keyHash, expiresAt)
	return err
}

// ---------------------------------------------------------------------------
// Roles
// ---------------------------------------------------------------------------

func (s *Store) AssignRole(ctx context.Context, userID, roleID string) error {
	// ON CONFLICT / INSERT IGNORE syntax differs by backend.
	var q string
	switch s.driver.(type) {
	case *MySQLDriver:
		q = fmt.Sprintf(
			`INSERT IGNORE INTO user_roles (user_id, role_id) VALUES (%s, %s)`,
			s.ph(1), s.ph(2),
		)
	default: // SQLite and PostgreSQL both support ON CONFLICT DO NOTHING
		q = fmt.Sprintf(
			`INSERT INTO user_roles (user_id, role_id) VALUES (%s, %s) ON CONFLICT DO NOTHING`,
			s.ph(1), s.ph(2),
		)
	}
	_, err := s.db.ExecContext(ctx, q, userID, roleID)
	return err
}

// ---------------------------------------------------------------------------
// Plugins
// ---------------------------------------------------------------------------

// PluginRecord is a minimal view of the plugins table.
type PluginRecord struct {
	Name string
	Path string
}

func (s *Store) ListEnabledPlugins(ctx context.Context) ([]PluginRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, path FROM plugins WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []PluginRecord
	for rows.Next() {
		var p PluginRecord
		if err := rows.Scan(&p.Name, &p.Path); err != nil {
			return nil, err
		}
		plugins = append(plugins, p)
	}
	return plugins, rows.Err()
}

// ---------------------------------------------------------------------------
// S3 config
// ---------------------------------------------------------------------------

// S3Config holds S3-compatible storage credentials.
// AccessKey and SecretKey are stored in the KMS-backed secrets table,
// not in plaintext in s3_config.
type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

func (s *Store) SetS3Config(ctx context.Context, cfg S3Config) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("s3 config tx: %w", err)
	}
	defer tx.Rollback()

	var q string
	switch s.driver.(type) {
	case *MySQLDriver:
		q = fmt.Sprintf(`
			INSERT INTO s3_config (id, endpoint, bucket, region)
			VALUES (1, %s, %s, %s)
			ON DUPLICATE KEY UPDATE
				endpoint = VALUES(endpoint),
				bucket   = VALUES(bucket),
				region   = VALUES(region)
		`, s.ph(1), s.ph(2), s.ph(3))
	default: // SQLite and PostgreSQL
		q = fmt.Sprintf(`
			INSERT INTO s3_config (id, endpoint, bucket, region)
			VALUES (1, %s, %s, %s)
			ON CONFLICT(id) DO UPDATE SET
				endpoint = excluded.endpoint,
				bucket   = excluded.bucket,
				region   = excluded.region
		`, s.ph(1), s.ph(2), s.ph(3))
	}

	if _, err = tx.ExecContext(ctx, q, cfg.Endpoint, cfg.Bucket, cfg.Region); err != nil {
		return fmt.Errorf("upsert s3_config: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	if err := s.SetSecret(ctx, "s3_access_key", []byte(cfg.AccessKey)); err != nil {
		return fmt.Errorf("store s3 access key: %w", err)
	}
	return s.SetSecret(ctx, "s3_secret_key", []byte(cfg.SecretKey))
}

func (s *Store) GetS3Config(ctx context.Context) (S3Config, error) {
	var cfg S3Config
	err := s.db.QueryRowContext(ctx,
		`SELECT endpoint, bucket, region FROM s3_config WHERE id = 1`,
	).Scan(&cfg.Endpoint, &cfg.Bucket, &cfg.Region)
	if err == sql.ErrNoRows {
		return S3Config{}, fmt.Errorf("s3 config not set")
	}
	if err != nil {
		return S3Config{}, fmt.Errorf("query s3_config: %w", err)
	}

	accessKey, err := s.GetSecret(ctx, "s3_access_key")
	if err != nil {
		return S3Config{}, fmt.Errorf("get s3 access key: %w", err)
	}
	secretKey, err := s.GetSecret(ctx, "s3_secret_key")
	if err != nil {
		return S3Config{}, fmt.Errorf("get s3 secret key: %w", err)
	}

	cfg.AccessKey = string(accessKey)
	cfg.SecretKey = string(secretKey)
	return cfg, nil
}

// ---------------------------------------------------------------------------
// WireGuard keys (stored via KMS secrets)
// ---------------------------------------------------------------------------

func (s *Store) SetWireGuardPrivateKey(ctx context.Context, ifaceName string, privateKey []byte) error {
	return s.SetSecret(ctx, "wg_private_key_"+ifaceName, privateKey)
}

func (s *Store) GetWireGuardPrivateKey(ctx context.Context, ifaceName string) ([]byte, error) {
	return s.GetSecret(ctx, "wg_private_key_"+ifaceName)
}

// ---------------------------------------------------------------------------
// WireGuard peers (for sync)
// ---------------------------------------------------------------------------

// WGPeerRecord represents a WireGuard peer from the database.
type WGPeerRecord struct {
	NodeID     string
	Endpoint   string
	AllowedIPs string
}

func (s *Store) UpsertWGPeer(ctx context.Context, p WGPeerRecord) error {
	switch s.driver.(type) {
	case *MySQLDriver:
		_, err := s.db.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO wg_peers (node_id, endpoint, allowed_ips)
				VALUES (%s, %s, %s)
				ON DUPLICATE KEY UPDATE endpoint=VALUES(endpoint), allowed_ips=VALUES(allowed_ips)`,
				s.ph(1), s.ph(2), s.ph(3)),
			p.NodeID, p.Endpoint, p.AllowedIPs)
		return err
	default: // SQLite and PostgreSQL
		_, err := s.db.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO wg_peers (node_id, endpoint, allowed_ips)
				VALUES (%s, %s, %s)
				ON CONFLICT (node_id) DO UPDATE SET endpoint=EXCLUDED.endpoint, allowed_ips=EXCLUDED.allowed_ips`,
				s.ph(1), s.ph(2), s.ph(3)),
			p.NodeID, p.Endpoint, p.AllowedIPs)
		return err
	}
}

func (s *Store) GetWGPeer(ctx context.Context, nodeID string) (WGPeerRecord, error) {
	q := fmt.Sprintf(`SELECT node_id, endpoint, allowed_ips FROM wg_peers WHERE node_id = %s`, s.ph(1))
	var p WGPeerRecord
	err := s.db.QueryRowContext(ctx, q, nodeID).Scan(&p.NodeID, &p.Endpoint, &p.AllowedIPs)
	if err != nil {
		return WGPeerRecord{}, fmt.Errorf("peer not found")
	}
	return p, nil
}

func (s *Store) UpdateWGPeerEndpoint(ctx context.Context, nodeID, endpoint string) error {
	q := fmt.Sprintf(`UPDATE wg_peers SET endpoint = %s WHERE node_id = %s`, s.ph(1), s.ph(2))
	_, err := s.db.ExecContext(ctx, q, endpoint, nodeID)
	return err
}

func (s *Store) ListWGPeers(ctx context.Context) ([]WGPeerRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT node_id, endpoint, allowed_ips FROM wg_peers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []WGPeerRecord
	for rows.Next() {
		var p WGPeerRecord
		if err := rows.Scan(&p.NodeID, &p.Endpoint, &p.AllowedIPs); err != nil {
			return nil, err
		}
		peers = append(peers, p)
	}
	return peers, rows.Err()
}

// NodeRecord represents a node from the database.
type NodeRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
}

func (s *Store) ListNodes(ctx context.Context) ([]NodeRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, type, address, public_key FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []NodeRecord
	for rows.Next() {
		var n NodeRecord
		if err := rows.Scan(&n.ID, &n.Name, &n.Type, &n.Address, &n.PublicKey); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
