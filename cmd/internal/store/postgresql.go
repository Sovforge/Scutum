package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type PostgresDriver struct{}

func (d PostgresDriver) Open(dsn string) (*sql.DB, error) {
	return sql.Open("postgres", dsn)
}

func (d PostgresDriver) Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

func (d PostgresDriver) Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, postgresSchema); err != nil {
		return err
	}
	for _, q := range []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret TEXT`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS actor TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS actor_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS outcome TEXT NOT NULL DEFAULT 'success'`,
		// OTEL span fields
		`ALTER TABLE traces ADD COLUMN IF NOT EXISTS trace_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE traces ADD COLUMN IF NOT EXISTS span_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE traces ADD COLUMN IF NOT EXISTS parent_span_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE traces ADD COLUMN IF NOT EXISTS service TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE traces ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'internal'`,
		`ALTER TABLE traces ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'internal'`,
		`ALTER TABLE traces ADD COLUMN IF NOT EXISTS attributes JSONB NOT NULL DEFAULT '{}'`,
		// OTEL log fields
		`ALTER TABLE system_logs ADD COLUMN IF NOT EXISTS service TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE system_logs ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'internal'`,
		`ALTER TABLE system_logs ADD COLUMN IF NOT EXISTS trace_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE system_logs ADD COLUMN IF NOT EXISTS span_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE system_logs ADD COLUMN IF NOT EXISTS attributes JSONB NOT NULL DEFAULT '{}'`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS disabled INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT`,
		// otel_metrics table
		`CREATE TABLE IF NOT EXISTS otel_metrics (
			id         TEXT PRIMARY KEY,
			time       TIMESTAMPTZ NOT NULL,
			name       TEXT NOT NULL,
			service    TEXT NOT NULL DEFAULT '',
			source     TEXT NOT NULL DEFAULT '',
			type       TEXT NOT NULL DEFAULT 'gauge',
			value      DOUBLE PRECISION NOT NULL DEFAULT 0,
			labels     JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	} {
		db.ExecContext(ctx, q)
	}
	return nil
}

const postgresSchema = `
CREATE TABLE IF NOT EXISTS nodes (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	type        TEXT NOT NULL CHECK(type IN ('hub','remote','combined')),
	address     TEXT NOT NULL,
	public_key  TEXT NOT NULL,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wg_peers (
	node_id         TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
	endpoint        TEXT NOT NULL,
	allowed_ips     TEXT NOT NULL,
	last_handshake  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS git_config (
	id             INTEGER PRIMARY KEY CHECK(id = 1),
	repo_url       TEXT NOT NULL,
	branch         TEXT NOT NULL DEFAULT 'main',
	last_synced_at TIMESTAMPTZ
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
	loaded_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS plugin_kv (
	plugin_id  TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
	key        TEXT NOT NULL,
	value      BYTEA NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (plugin_id, key)
);

CREATE TABLE IF NOT EXISTS users (
	id            TEXT PRIMARY KEY,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	totp_secret   TEXT,
	totp_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_keys (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	key_hash   TEXT NOT NULL UNIQUE,
	name       TEXT NOT NULL,
	expires_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
	dek_encrypted   BYTEA NOT NULL,
	value_encrypted BYTEA NOT NULL,
	provider        TEXT NOT NULL,
	updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
	path_style BOOLEAN NOT NULL DEFAULT TRUE,
	use_ssl    BOOLEAN NOT NULL DEFAULT TRUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recovery_codes (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	code_hash  TEXT NOT NULL UNIQUE,
	used       BOOLEAN NOT NULL DEFAULT FALSE,
	used_at    TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
	id         TEXT PRIMARY KEY,
	time       TIMESTAMPTZ NOT NULL,
	action     TEXT NOT NULL,
	actor      TEXT NOT NULL DEFAULT '',
	actor_id   TEXT NOT NULL DEFAULT '',
	outcome    TEXT NOT NULL DEFAULT 'success',
	method     TEXT NOT NULL,
	path       TEXT NOT NULL,
	trace_id   TEXT NOT NULL DEFAULT '',
	client_ip  TEXT NOT NULL DEFAULT '',
	extra      TEXT NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS system_logs (
	id         TEXT PRIMARY KEY,
	time       TIMESTAMPTZ NOT NULL,
	level      TEXT NOT NULL,
	message    TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS traces (
	id          TEXT PRIMARY KEY,
	time        TIMESTAMPTZ NOT NULL,
	name        TEXT NOT NULL,
	duration_ms BIGINT NOT NULL,
	status      TEXT NOT NULL,
	error       TEXT NOT NULL DEFAULT '',
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS hub_leases (
	id         INTEGER PRIMARY KEY CHECK(id = 1),
	holder_id  TEXT NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS federation_peers (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL,
	hub_url       TEXT NOT NULL,
	wg_endpoint   TEXT NOT NULL,
	wg_public_key TEXT NOT NULL,
	mesh_cidr     TEXT NOT NULL,
	allowed_ips   TEXT NOT NULL DEFAULT '',
	status        TEXT NOT NULL DEFAULT 'pending',
	last_seen     TIMESTAMPTZ,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS node_labels (
	node_id   TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
	label_key TEXT NOT NULL,
	value     TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (node_id, label_key)
);

CREATE TABLE IF NOT EXISTS node_groups (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL UNIQUE,
CREATE TABLE IF NOT EXISTS webhook_configs (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	url        TEXT NOT NULL,
	secret     TEXT NOT NULL DEFAULT '',
	events     TEXT NOT NULL DEFAULT '[]',
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scim_tokens (
	id          TEXT PRIMARY KEY,
	token_hash  TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS node_group_members (
	group_id TEXT NOT NULL REFERENCES node_groups(id) ON DELETE CASCADE,
	node_id  TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
	PRIMARY KEY (group_id, node_id)
CREATE TABLE IF NOT EXISTS audit_forwarders (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	url        TEXT NOT NULL,
	format     TEXT NOT NULL DEFAULT 'json',
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
CREATE TABLE IF NOT EXISTS sso_identities (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id),
	provider   TEXT NOT NULL,
	subject    TEXT NOT NULL,
	email      TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE(provider, subject)
);
`
