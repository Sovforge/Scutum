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

// UserRecord/RoleRecord live in lib/api.ts now (real API types) — re-exported
// here so existing imports don't need to change.
export type { UserRecord, RoleRecord } from '../../lib/api'
import type { RoleRecord } from '../../lib/api'

export function permLevel(role: RoleRecord, resource: Resource): PermLevel {
  const entry = role.perms.find((p) => p.startsWith(`${resource}:`))
  if (!entry) return 'none'
  return entry.split(':')[1] as PermLevel
}

export function setPermLevel(perms: string[], resource: Resource, level: PermLevel): string[] {
  const rest = perms.filter((p) => !p.startsWith(`${resource}:`))
  return level === 'none' ? rest : [...rest, `${resource}:${level}`]
}
