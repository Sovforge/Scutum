package store

import (
	"context"
	"fmt"
)

var defaultRoles = []struct {
	id          string
	name        string
	description string
}{
	{"role_admin", "admin", "Full access to all resources"},
	{"role_operator", "operator", "Read and write access, no delete or admin"},
	{"role_viewer", "viewer", "Read-only access to all resources"},
}

var defaultPermissions = []struct {
	id       string
	name     string
	resource string
	action   string
}{
	// Nodes
	{"perm_nodes_read", "nodes:read", "nodes", "read"},
	{"perm_nodes_write", "nodes:write", "nodes", "write"},
	{"perm_nodes_admin", "nodes:admin", "nodes", "admin"},
	// Docker
	{"perm_docker_read", "docker:read", "docker", "read"},
	{"perm_docker_write", "docker:write", "docker", "write"},
	{"perm_docker_admin", "docker:admin", "docker", "admin"},
	// Kubernetes
	{"perm_k8s_read", "kubernetes:read", "kubernetes", "read"},
	{"perm_k8s_write", "kubernetes:write", "kubernetes", "write"},
	{"perm_k8s_admin", "kubernetes:admin", "kubernetes", "admin"},
	// Git
	{"perm_git_read", "git:read", "git", "read"},
	{"perm_git_write", "git:write", "git", "write"},
	{"perm_git_admin", "git:admin", "git", "admin"},
	// Storage (S3-compatible)
	{"perm_storage_read", "storage:read", "storage", "read"},
	{"perm_storage_write", "storage:write", "storage", "write"},
	{"perm_storage_admin", "storage:admin", "storage", "admin"},
	// WireGuard
	{"perm_wg_read", "wireguard:read", "wireguard", "read"},
	{"perm_wg_write", "wireguard:write", "wireguard", "write"},
	{"perm_wg_admin", "wireguard:admin", "wireguard", "admin"},
	// Plugins
	{"perm_plugins_read", "plugins:read", "plugins", "read"},
	{"perm_plugins_write", "plugins:write", "plugins", "write"},
	{"perm_plugins_admin", "plugins:admin", "plugins", "admin"},
	// Sync
	{"perm_sync_read", "sync:read", "sync", "read"},
	{"perm_sync_write", "sync:write", "sync", "write"},
	{"perm_sync_admin", "sync:admin", "sync", "admin"},
	// Admin (platform-level management: users, roles, audit)
	{"perm_admin_read", "admin:read", "admin", "read"},
	{"perm_admin_write", "admin:write", "admin", "write"},
	{"perm_admin_admin", "admin:admin", "admin", "admin"},
}

// rolePermissions maps role ID to the permission IDs it receives.
var rolePermissions = map[string][]string{
	"role_admin": {
		"perm_nodes_read", "perm_nodes_write", "perm_nodes_admin",
		"perm_docker_read", "perm_docker_write", "perm_docker_admin",
		"perm_k8s_read", "perm_k8s_write", "perm_k8s_admin",
		"perm_git_read", "perm_git_write", "perm_git_admin",
		"perm_storage_read", "perm_storage_write", "perm_storage_admin",
		"perm_wg_read", "perm_wg_write", "perm_wg_admin",
		"perm_plugins_read", "perm_plugins_write", "perm_plugins_admin",
		"perm_sync_read", "perm_sync_write", "perm_sync_admin",
		"perm_admin_read", "perm_admin_write", "perm_admin_admin",
	},
	"role_operator": {
		"perm_nodes_read", "perm_nodes_write",
		"perm_docker_read", "perm_docker_write",
		"perm_k8s_read", "perm_k8s_write",
		"perm_git_read", "perm_git_write",
		"perm_storage_read", "perm_storage_write",
		"perm_wg_read", "perm_wg_write",
		"perm_plugins_read",
		"perm_sync_read", "perm_sync_write",
	},
	"role_viewer": {
		"perm_nodes_read",
		"perm_docker_read",
		"perm_k8s_read",
		"perm_git_read",
		"perm_storage_read",
		"perm_wg_read",
		"perm_plugins_read",
		"perm_sync_read",
		"perm_admin_read",
	},
}

// Seed inserts default roles and permissions if they don't already exist.
// Safe to call on every startup.
func (s *Store) Seed(ctx context.Context) error {
	var qRole, qPerm, qRolePerm string
	switch s.driver.(type) {
	case *MySQLDriver:
		qRole = fmt.Sprintf(`INSERT IGNORE INTO roles (id, name, description) VALUES (%s, %s, %s)`, s.ph(1), s.ph(2), s.ph(3))
		qPerm = fmt.Sprintf(`INSERT IGNORE INTO permissions (id, name, resource, action) VALUES (%s, %s, %s, %s)`, s.ph(1), s.ph(2), s.ph(3), s.ph(4))
		qRolePerm = fmt.Sprintf(`INSERT IGNORE INTO role_permissions (role_id, permission_id) VALUES (%s, %s)`, s.ph(1), s.ph(2))
	case *SQLiteDriver:
		qRole = fmt.Sprintf(`INSERT OR IGNORE INTO roles (id, name, description) VALUES (%s, %s, %s)`, s.ph(1), s.ph(2), s.ph(3))
		qPerm = fmt.Sprintf(`INSERT OR IGNORE INTO permissions (id, name, resource, action) VALUES (%s, %s, %s, %s)`, s.ph(1), s.ph(2), s.ph(3), s.ph(4))
		qRolePerm = fmt.Sprintf(`INSERT OR IGNORE INTO role_permissions (role_id, permission_id) VALUES (%s, %s)`, s.ph(1), s.ph(2))
	default: // PostgreSQL
		qRole = fmt.Sprintf(`INSERT INTO roles (id, name, description) VALUES (%s, %s, %s) ON CONFLICT DO NOTHING`, s.ph(1), s.ph(2), s.ph(3))
		qPerm = fmt.Sprintf(`INSERT INTO permissions (id, name, resource, action) VALUES (%s, %s, %s, %s) ON CONFLICT DO NOTHING`, s.ph(1), s.ph(2), s.ph(3), s.ph(4))
		qRolePerm = fmt.Sprintf(`INSERT INTO role_permissions (role_id, permission_id) VALUES (%s, %s) ON CONFLICT DO NOTHING`, s.ph(1), s.ph(2))
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("seed begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, r := range defaultRoles {
		if _, err := tx.ExecContext(ctx, qRole, r.id, r.name, r.description); err != nil {
			return fmt.Errorf("seed role %s: %w", r.name, err)
		}
	}

	for _, p := range defaultPermissions {
		if _, err := tx.ExecContext(ctx, qPerm, p.id, p.name, p.resource, p.action); err != nil {
			return fmt.Errorf("seed permission %s: %w", p.name, err)
		}
	}

	for roleID, permIDs := range rolePermissions {
		for _, permID := range permIDs {
			if _, err := tx.ExecContext(ctx, qRolePerm, roleID, permID); err != nil {
				return fmt.Errorf("seed role_permission %s->%s: %w", roleID, permID, err)
			}
		}
	}

	return tx.Commit()
}
