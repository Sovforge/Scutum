export type PermLevel = 'none' | 'read' | 'write' | 'admin'

// The 9 resources actually enforced by the Go backend's route table
// (auth.Require(store, resource, action) in cmd/api/main.go).
// compliance/federation/scim are also real resources there, but are
// intentionally left out of this pass — see Settings.tsx's tab list for
// why those sections aren't built yet. The backend also accepts a
// `delete` action for docker/kubernetes specifically, but the old app's
// own permission matrix never exposed it either — matched here, not
// "fixed", since the point is parity with the real UI.
export const RESOURCES = ['nodes', 'docker', 'kubernetes', 'git', 'storage', 'wireguard', 'plugins', 'sync', 'admin'] as const
export type Resource = (typeof RESOURCES)[number]

export type UserRecord = {
  id: string
  username: string
  roles: string[]
  createdAt: string
}

export type RoleRecord = {
  id: string
  name: string
  description: string
  // Canonical stored form is "resource:level" per role_handler.go's
  // expandWildcards (a trailing ":*" gets rewritten to ":admin" before
  // storage) — perms are already in that normalized form here.
  perms: string[]
}

export function permLevel(role: RoleRecord, resource: Resource): PermLevel {
  const entry = role.perms.find((p) => p.startsWith(`${resource}:`))
  if (!entry) return 'none'
  return entry.split(':')[1] as PermLevel
}

export function setPermLevel(perms: string[], resource: Resource, level: PermLevel): string[] {
  const rest = perms.filter((p) => !p.startsWith(`${resource}:`))
  return level === 'none' ? rest : [...rest, `${resource}:${level}`]
}

// admin's created_at matches the "Member since" date on the Account page;
// ops-oncall's matches the USER_CREATED audit entry; j.doe's ties to the
// PASSWORD_RESET actor already in the Audit Log mock.
export const INITIAL_USERS: UserRecord[] = [
  { id: 'usr_admin01', username: 'admin', roles: ['role_admin'], createdAt: '2026-04-02 00:00:00' },
  { id: 'usr_ops03', username: 'ops-oncall', roles: ['role_ops'], createdAt: '2026-06-28 14:22:10' },
  { id: 'usr_jdoe02', username: 'j.doe', roles: ['role_viewer'], createdAt: '2026-05-10 09:30:00' },
]

export const INITIAL_ROLES: RoleRecord[] = [
  {
    id: 'role_admin',
    name: 'admin',
    description: 'Full administrative access to every resource.',
    perms: RESOURCES.map((r) => `${r}:admin`),
  },
  {
    id: 'role_ops',
    name: 'ops-oncall',
    description: 'On-call operations — restart containers and inspect mesh/cluster state.',
    perms: ['nodes:read', 'docker:write', 'kubernetes:read', 'wireguard:read'],
  },
  {
    id: 'role_viewer',
    name: 'viewer',
    description: 'Read-only access for auditors and observers.',
    perms: ['nodes:read', 'docker:read', 'kubernetes:read'],
  },
]
