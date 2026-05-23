package store

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLDriver struct{}

func (d MySQLDriver) Open(dsn string) (*sql.DB, error) {
	return sql.Open("mysql", dsn)
}

func (d MySQLDriver) Placeholder(n int) string {
	// MySQL uses '?' for prepared statements, regardless of index
	return "?"
}

func (d MySQLDriver) Migrate(ctx context.Context, db *sql.DB) error {
	// MySQL doesn't support multi-statement execution in a single Exec call
	// unless 'multiStatements=true' is in the DSN.
	if _, err := db.ExecContext(ctx, mysqlSchema); err != nil {
		return err
	}
	for _, q := range []string{
		`ALTER TABLE users ADD COLUMN totp_secret VARCHAR(64)`,
		`ALTER TABLE users ADD COLUMN totp_enabled TINYINT(1) NOT NULL DEFAULT 0`,
		`ALTER TABLE audit_logs ADD COLUMN actor VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE audit_logs ADD COLUMN actor_id VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE audit_logs ADD COLUMN outcome VARCHAR(20) NOT NULL DEFAULT 'success'`,
		// OTEL span fields
		`ALTER TABLE traces ADD COLUMN trace_id VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE traces ADD COLUMN span_id VARCHAR(32) NOT NULL DEFAULT ''`,
		`ALTER TABLE traces ADD COLUMN parent_span_id VARCHAR(32) NOT NULL DEFAULT ''`,
		`ALTER TABLE traces ADD COLUMN service VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE traces ADD COLUMN kind VARCHAR(20) NOT NULL DEFAULT 'internal'`,
		`ALTER TABLE traces ADD COLUMN source VARCHAR(20) NOT NULL DEFAULT 'internal'`,
		`ALTER TABLE traces ADD COLUMN attributes JSON`,
		// OTEL log fields
		`ALTER TABLE system_logs ADD COLUMN service VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE system_logs ADD COLUMN source VARCHAR(20) NOT NULL DEFAULT 'internal'`,
		`ALTER TABLE system_logs ADD COLUMN trace_id VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE system_logs ADD COLUMN span_id VARCHAR(32) NOT NULL DEFAULT ''`,
		`ALTER TABLE system_logs ADD COLUMN attributes JSON`,
		// otel_metrics table (MySQL runs one statement at a time)
		`CREATE TABLE IF NOT EXISTS otel_metrics (
			id         VARCHAR(255) PRIMARY KEY,
			time       TIMESTAMP NOT NULL,
			name       VARCHAR(255) NOT NULL,
			service    VARCHAR(255) NOT NULL DEFAULT '',
			source     VARCHAR(50) NOT NULL DEFAULT '',
			type       VARCHAR(20) NOT NULL DEFAULT 'gauge',
			value      DOUBLE NOT NULL DEFAULT 0,
			labels     JSON,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	} {
		db.ExecContext(ctx, q) // intentionally ignore "duplicate column" errors
	}
	return nil
}

const mysqlSchema = `
CREATE TABLE IF NOT EXISTS nodes (
    id          VARCHAR(255) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    type        ENUM('hub', 'remote', 'combined') NOT NULL,
    address     VARCHAR(255) NOT NULL,
    public_key  TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS wg_peers (
    node_id         VARCHAR(255) PRIMARY KEY,
    endpoint        VARCHAR(255) NOT NULL,
    allowed_ips     TEXT NOT NULL,
    last_handshake  TIMESTAMP NULL,
    CONSTRAINT fk_wg_nodes FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS git_config (
    id             INT PRIMARY KEY CHECK (id = 1),
    repo_url       TEXT NOT NULL,
    branch         VARCHAR(255) NOT NULL DEFAULT 'main',
    last_synced_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS s3_config (
    id       INT PRIMARY KEY CHECK (id = 1),
    endpoint VARCHAR(255) NOT NULL,
    bucket   VARCHAR(255) NOT NULL,
    region   VARCHAR(100) NOT NULL
);

CREATE TABLE IF NOT EXISTS plugins (
    id        VARCHAR(255) PRIMARY KEY,
    name      VARCHAR(255) NOT NULL UNIQUE,
    path      TEXT NOT NULL,
    enabled   TINYINT(1) NOT NULL DEFAULT 1,
    loaded_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS plugin_kv (
    plugin_id  VARCHAR(255) NOT NULL,
    ` + "`key`" + `        VARCHAR(255) NOT NULL,
    value      LONGBLOB NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (plugin_id, ` + "`key`" + `),
    CONSTRAINT fk_plugin_kv FOREIGN KEY (plugin_id) REFERENCES plugins(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS users (
    id            VARCHAR(255) PRIMARY KEY,
    username      VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    totp_secret   VARCHAR(64),
    totp_enabled  TINYINT(1) NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
    id         VARCHAR(255) PRIMARY KEY,
    user_id    VARCHAR(255) NOT NULL,
    key_hash   VARCHAR(255) NOT NULL UNIQUE,
    name       VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_api_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS roles (
    id          VARCHAR(255) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE IF NOT EXISTS permissions (
    id       VARCHAR(255) PRIMARY KEY,
    name     VARCHAR(255) NOT NULL UNIQUE,
    resource VARCHAR(255) NOT NULL,
    action   VARCHAR(255) NOT NULL,
    UNIQUE(resource, action)
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       VARCHAR(255) NOT NULL,
    permission_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    CONSTRAINT fk_rp_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_rp_perm FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id VARCHAR(255) NOT NULL,
    role_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_ur_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_ur_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS secrets (
    ` + "`key`" + `             VARCHAR(255) PRIMARY KEY,
    dek_encrypted   LONGBLOB NOT NULL,
    value_encrypted LONGBLOB NOT NULL,
    provider        VARCHAR(100) NOT NULL,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS setup_state (
    ` + "`key`" + `  VARCHAR(255) PRIMARY KEY,
    value VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS storage_backends (
    id         VARCHAR(255) PRIMARY KEY,
    name       VARCHAR(255) NOT NULL UNIQUE,
    provider   VARCHAR(100) NOT NULL DEFAULT 'minio',
    endpoint   TEXT NOT NULL,
    region     VARCHAR(100) NOT NULL DEFAULT 'us-east-1',
    access_key TEXT NOT NULL DEFAULT '',
    path_style TINYINT(1) NOT NULL DEFAULT 1,
    use_ssl    TINYINT(1) NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recovery_codes (
    id         VARCHAR(255) PRIMARY KEY,
    user_id    VARCHAR(255) NOT NULL,
    code_hash  VARCHAR(255) NOT NULL UNIQUE,
    used       TINYINT(1) NOT NULL DEFAULT 0,
    used_at    TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_rc_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id         VARCHAR(255) PRIMARY KEY,
    time       TIMESTAMP NOT NULL,
    action     VARCHAR(255) NOT NULL,
    actor      VARCHAR(255) NOT NULL DEFAULT '',
    actor_id   VARCHAR(255) NOT NULL DEFAULT '',
    outcome    VARCHAR(20) NOT NULL DEFAULT 'success',
    method     VARCHAR(10) NOT NULL,
    path       TEXT NOT NULL,
    trace_id   VARCHAR(255) NOT NULL DEFAULT '',
    client_ip  VARCHAR(255) NOT NULL DEFAULT '',
    extra      TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS system_logs (
    id         VARCHAR(255) PRIMARY KEY,
    time       TIMESTAMP NOT NULL,
    level      VARCHAR(20) NOT NULL,
    message    TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS traces (
    id          VARCHAR(255) PRIMARY KEY,
    time        TIMESTAMP NOT NULL,
    name        VARCHAR(255) NOT NULL,
    duration_ms BIGINT NOT NULL,
    status      VARCHAR(20) NOT NULL,
    error       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS hub_leases (
    id         INT PRIMARY KEY CHECK(id = 1),
    holder_id  VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
`
