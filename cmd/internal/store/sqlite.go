package store

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite"
)

type SQLiteDriver struct{}

func (d SQLiteDriver) Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite doesn't support concurrent writes
	return db, nil
}

func (d SQLiteDriver) Placeholder(_ int) string { return "?" }

func (d SQLiteDriver) Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, sqliteSchema); err != nil {
		return err
	}
	// Idempotent column additions for databases created before these columns existed.
	for _, q := range []string{
		`ALTER TABLE users ADD COLUMN totp_secret TEXT`,
		`ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0`,
	} {
		db.ExecContext(ctx, q) // intentionally ignore "duplicate column" errors
	}
	return nil
}

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS nodes (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			type        TEXT NOT NULL CHECK(type IN ('hub','peer','edge')),
			address     TEXT NOT NULL,
			public_key  TEXT NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS wg_peers (
			node_id         TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
			endpoint        TEXT NOT NULL,
			allowed_ips     TEXT NOT NULL,
			last_handshake  DATETIME
		);

		CREATE TABLE IF NOT EXISTS git_config (
			id             INTEGER PRIMARY KEY CHECK(id = 1),
			repo_url       TEXT NOT NULL,
			branch         TEXT NOT NULL DEFAULT 'main',
			last_synced_at DATETIME
		);

		CREATE TABLE IF NOT EXISTS s3_config (
			id       INTEGER PRIMARY KEY CHECK(id = 1),
			endpoint TEXT NOT NULL,
			bucket   TEXT NOT NULL,
			region   TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS plugins (
			id        TEXT PRIMARY KEY,
			name      TEXT NOT NULL UNIQUE,
			path      TEXT NOT NULL,
			enabled   INTEGER NOT NULL DEFAULT 1,
			loaded_at DATETIME
		);

		CREATE TABLE IF NOT EXISTS plugin_kv (
			plugin_id  TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
			key        TEXT NOT NULL,
			value      BLOB NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (plugin_id, key)
		);

		CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			totp_secret   TEXT,
			totp_enabled  INTEGER NOT NULL DEFAULT 0,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS api_keys (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			key_hash   TEXT NOT NULL UNIQUE,
			name       TEXT NOT NULL,
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS roles (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL UNIQUE,
			description TEXT
		);

		CREATE TABLE IF NOT EXISTS permissions (
			id       TEXT PRIMARY KEY,
			name     TEXT NOT NULL UNIQUE,
			resource TEXT NOT NULL,
			action   TEXT NOT NULL,
			UNIQUE(resource, action)
		);

		CREATE TABLE IF NOT EXISTS role_permissions (
			role_id       TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
			permission_id TEXT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
			PRIMARY KEY (role_id, permission_id)
		);

		CREATE TABLE IF NOT EXISTS user_roles (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
			PRIMARY KEY (user_id, role_id)
		);

		CREATE TABLE IF NOT EXISTS secrets (
			key             TEXT PRIMARY KEY,
			dek_encrypted   BLOB NOT NULL,
			value_encrypted BLOB NOT NULL,
			provider        TEXT NOT NULL,
			updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS setup_state (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS storage_backends (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL UNIQUE,
		provider   TEXT NOT NULL DEFAULT 'minio',
		endpoint   TEXT NOT NULL,
		region     TEXT NOT NULL DEFAULT 'us-east-1',
		access_key TEXT NOT NULL DEFAULT '',
		path_style INTEGER NOT NULL DEFAULT 1,
		use_ssl    INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS recovery_codes (
		id         TEXT PRIMARY KEY,
		user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		code_hash  TEXT NOT NULL UNIQUE,
		used       INTEGER NOT NULL DEFAULT 0,
		used_at    DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id        TEXT PRIMARY KEY,
		time      DATETIME NOT NULL,
		action    TEXT NOT NULL,
		method    TEXT NOT NULL,
		path      TEXT NOT NULL,
		trace_id  TEXT NOT NULL DEFAULT '',
		client_ip TEXT NOT NULL DEFAULT '',
		extra     TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS system_logs (
		id         TEXT PRIMARY KEY,
		time       DATETIME NOT NULL,
		level      TEXT NOT NULL,
		message    TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS traces (
		id          TEXT PRIMARY KEY,
		time        DATETIME NOT NULL,
		name        TEXT NOT NULL,
		duration_ms INTEGER NOT NULL,
		status      TEXT NOT NULL,
		error       TEXT NOT NULL DEFAULT '',
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS hub_leases (
		holder_id  TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		-- single-row table enforced by constant primary key
		id         INTEGER PRIMARY KEY CHECK(id = 1)
	);

	PRAGMA journal_mode=WAL;
		PRAGMA foreign_keys=ON;
		PRAGMA secure_delete=ON;
		PRAGMA auto_vacuum=FULL;
`
