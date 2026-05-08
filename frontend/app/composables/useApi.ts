// API base prefix — all calls go through Nuxt's /api/** → backend proxy
const BASE = '/api'

export interface KmsConfig {
  provider: 'local' | 'vault' | 'aws' | 'gcp' | 'azure'
  local?:   { key_file?: string }
  vault?:   { addr: string; key_name: string; token_file?: string }
  aws?:     { region: string; key_id: string; access_key?: string; secret_key?: string }
  gcp?:     { project_id: string; location_id?: string; key_ring_id: string; key_id: string; token_file?: string }
  azure?:   { vault_url: string; key_name?: string; tenant_id: string; client_id: string; token_file?: string }
}

export interface SetupRequest {
  install_type: 'hub' | 'remote' | 'combined'
  kms:       KmsConfig
  wireguard: {
    listen_port?:     number
    address:          string
    mtu?:             number
    hub_endpoint?:    string
    hub_public_key?:  string
    hub_allowed_ips?: string
  }
  admin: { username: string; password: string }
  recovery?: { n_shares: number; threshold: number }
}

export interface SetupResponse {
  // Normal 201 response
  message?:         string
  admin_id?:        string
  kms_provider?:    string
  install_type?:    string
  wireguard?:       { public_key: string; address?: string; listen_port?: number; warning?: string }
  recovery_shares?: string[]
  // 202 restart response (wireguard-go was just installed)
  status?:          'restarting'
}

export interface AppVersion {
  version: string
  build:   string
  commit:  string
}

export interface UserRecord {
  id:         string
  username:   string
  roles:      string[]
  created_at: string
}

export interface APIKeyRecord {
  id:         string
  name:       string
  expires_at: string | null
  created_at: string
}

export interface RoleRecord {
  id:          string
  name:        string
  description: string
  perms:       string[]
}

export interface NodeRecord {
  id:         string
  name:       string
  type:       string
  address:    string
  public_key: string
}

export interface DockerContainer {
  Id:      string
  Names:   string[]
  Image:   string
  Status:  string
  State:   string
  Ports:   Array<{ PrivatePort?: number; PublicPort?: number; Type?: string }>
}

export interface PluginRecord {
  name:      string
  filename:  string
  path:      string
  loadedAt?: string
}

export interface StorageBackend {
  id:         string
  name:       string
  provider:   string
  endpoint:   string
  region:     string
  access_key: string
  path_style: boolean
  use_ssl:    boolean
  created_at?: string
}

export interface BucketInfo {
  name:       string
  created_at?: string
}

export interface LogEntry {
  time:    string
  level:   'debug' | 'info' | 'warn' | 'error'
  message: string
}

export interface AuditEntry {
  time:      string
  action:    string
  method:    string
  path:      string
  trace_id?: string
  client_ip?: string
  extra?:    Record<string, string>
}

export interface TraceEntry {
  time:        string
  name:        string
  duration_ms: number
  status:      'ok' | 'error'
  error?:      string
}

export interface GitSyncRequest {
  repo_url:   string
  username?:  string
  token?:     string
  target_dir: string
}

export function useApi() {
  const { getToken } = useAuth()

  function h(): Record<string, string> {
    const t = getToken()
    return t ? { Authorization: `Bearer ${t}` } : {}
  }

  // ── Auth / setup (public) ────────────────────────────────────────────────
  async function login(username: string, password: string, totpCode?: string): Promise<{ token?: string; totp_required?: boolean }> {
    const body: Record<string, string> = { username, password }
    if (totpCode) body.totp_code = totpCode
    return $fetch<{ token?: string; totp_required?: boolean }>(`${BASE}/auth/login`, {
      method: 'POST', body,
    })
  }

  async function setupStatus(): Promise<{ complete: boolean }> {
    return $fetch<{ complete: boolean }>(`${BASE}/setup/status`)
  }

  async function doSetup(payload: SetupRequest): Promise<SetupResponse> {
    return $fetch<SetupResponse>(`${BASE}/setup`, { method: 'POST', body: payload })
  }

  // ── Version ──────────────────────────────────────────────────────────────
  async function getVersion(): Promise<AppVersion> {
    try {
      return await $fetch<AppVersion>(`${BASE}/version`, { headers: h() })
    } catch {
      return { version: 'v0.9.1', build: '2026.04', commit: 'a3f9c12' }
    }
  }

  // ── Users ────────────────────────────────────────────────────────────────
  async function listUsers(): Promise<UserRecord[]> {
    return $fetch<UserRecord[]>(`${BASE}/users`, { headers: h() })
  }

  async function createUser(payload: { username: string; password: string; roles?: string[] }): Promise<UserRecord> {
    return $fetch<UserRecord>(`${BASE}/users`, { method: 'POST', body: payload, headers: h() })
  }

  async function updateUser(id: string, payload: { username?: string; password?: string; roles?: string[] }): Promise<void> {
    await $fetch(`${BASE}/users/${id}`, { method: 'PUT', body: payload, headers: h() })
  }

  async function deleteUser(id: string): Promise<void> {
    await $fetch(`${BASE}/users/${id}`, { method: 'DELETE', headers: h() })
  }

  // ── Roles ────────────────────────────────────────────────────────────────
  async function listRoles(): Promise<RoleRecord[]> {
    return $fetch<RoleRecord[]>(`${BASE}/roles`, { headers: h() })
  }

  async function createRole(payload: { name: string; description?: string; perms?: string[] }): Promise<RoleRecord> {
    return $fetch<RoleRecord>(`${BASE}/roles`, { method: 'POST', body: payload, headers: h() })
  }

  async function updateRole(id: string, payload: { name: string; description?: string; perms?: string[] }): Promise<void> {
    await $fetch(`${BASE}/roles/${id}`, { method: 'PUT', body: payload, headers: h() })
  }

  async function deleteRole(id: string): Promise<void> {
    await $fetch(`${BASE}/roles/${id}`, { method: 'DELETE', headers: h() })
  }

  // ── Nodes ────────────────────────────────────────────────────────────────
  async function listNodes(): Promise<NodeRecord[]> {
    return $fetch<NodeRecord[]>(`${BASE}/nodes`, { headers: h() })
  }

  async function getNode(id: string): Promise<NodeRecord> {
    return $fetch<NodeRecord>(`${BASE}/nodes/${id}`, { headers: h() })
  }

  async function createNode(payload: { name: string; type: string; address: string; public_key: string }): Promise<NodeRecord> {
    return $fetch<NodeRecord>(`${BASE}/nodes`, { method: 'POST', body: payload, headers: h() })
  }

  async function deleteNode(id: string): Promise<void> {
    await $fetch(`${BASE}/nodes/${id}`, { method: 'DELETE', headers: h() })
  }

  // ── Auth / profile ────────────────────────────────────────────────────────
  async function getMe(): Promise<UserRecord> {
    return $fetch<UserRecord>(`${BASE}/auth/me`, { headers: h() })
  }

  async function listTokens(): Promise<APIKeyRecord[]> {
    return $fetch<APIKeyRecord[]>(`${BASE}/auth/tokens`, { headers: h() })
  }

  async function deleteToken(id: string): Promise<void> {
    await $fetch(`${BASE}/auth/tokens/${id}`, { method: 'DELETE', headers: h() })
  }

  async function createToken(name: string, expiresAt?: string): Promise<{ id: string; key: string }> {
    return $fetch<{ id: string; key: string }>(`${BASE}/auth/keys`, {
      method: 'POST',
      body: expiresAt ? { name, expires_at: expiresAt } : { name },
      headers: h(),
    })
  }

  // ── Docker ───────────────────────────────────────────────────────────────
  async function listContainers(): Promise<DockerContainer[]> {
    return $fetch<DockerContainer[]>(`${BASE}/docker/containers`, { headers: h() })
  }

  async function getContainerInspect(id: string): Promise<any> {
    return $fetch<any>(`${BASE}/docker/containers/${id}`, { headers: h() })
  }

  async function getContainerLogsJSON(id: string, tail = 100): Promise<Array<{ ts: string; stream: string; msg: string }>> {
    return $fetch<Array<{ ts: string; stream: string; msg: string }>>(`${BASE}/docker/containers/${id}/logs-json?tail=${tail}`, { headers: h() })
  }

  async function getContainerStats(id: string): Promise<{ cpu_percent: number; mem_usage: number; mem_limit: number; net_rx: number; net_tx: number; blk_read: number; blk_write: number }> {
    return $fetch(`${BASE}/docker/containers/${id}/stats-snapshot`, { headers: h() })
  }

  async function deployCompose(yaml: string, nodeId?: string): Promise<{ output?: string; error?: string }> {
    const hdrs: Record<string, string> = { ...h(), 'Content-Type': 'text/yaml' }
    if (nodeId) hdrs['X-Target-Node'] = nodeId
    return $fetch<{ output?: string; error?: string }>(`${BASE}/docker/deploy-compose`, {
      method: 'POST', body: yaml, headers: hdrs,
    })
  }

  async function applyK8s(yaml: string, nodeId?: string): Promise<{ output?: string; error?: string }> {
    const hdrs: Record<string, string> = { ...h(), 'Content-Type': 'text/yaml' }
    if (nodeId) hdrs['X-Target-Node'] = nodeId
    return $fetch<{ output?: string; error?: string }>(`${BASE}/kubernetes/apply`, {
      method: 'POST', body: yaml, headers: hdrs,
    })
  }

  async function getK8sPod(ns: string, name: string): Promise<any> {
    return $fetch<any>(`${BASE}/kubernetes/${ns}/pods/${name}`, { headers: h() })
  }

  async function getK8sPodLogsJSON(ns: string, name: string, container?: string, tail = 100): Promise<Array<{ ts: string; msg: string }>> {
    const q = new URLSearchParams({ tail: String(tail) })
    if (container) q.set('container', container)
    return $fetch<Array<{ ts: string; msg: string }>>(`${BASE}/kubernetes/${ns}/pods/${name}/logs-json?${q}`, { headers: h() })
  }

  async function getK8sSummary(): Promise<{ pods: number; running: number; pending: number; failed: number; succeeded: number; namespaces: number; nodes: number; deployments: number; healthy_deploys: number; unhealthy_deploys: number }> {
    return $fetch(`${BASE}/kubernetes/summary`, { headers: h() })
  }

  async function listAllK8sPods(): Promise<any> {
    return $fetch<any>(`${BASE}/kubernetes/pods`, { headers: h() })
  }

  async function deleteK8sPod(ns: string, name: string): Promise<void> {
    await $fetch(`${BASE}/kubernetes/${ns}/pods/${name}`, { method: 'DELETE', headers: h() })
  }

  async function startContainer(id: string): Promise<void> {
    await $fetch(`${BASE}/docker/containers/${id}/start`, { method: 'POST', headers: h() })
  }

  async function stopContainer(id: string): Promise<void> {
    await $fetch(`${BASE}/docker/containers/${id}/stop`, { method: 'POST', headers: h() })
  }

  async function restartContainer(id: string): Promise<void> {
    await $fetch(`${BASE}/docker/containers/${id}/restart`, { method: 'POST', headers: h() })
  }

  async function removeContainer(id: string): Promise<void> {
    await $fetch(`${BASE}/docker/containers/${id}`, { method: 'DELETE', headers: h() })
  }

  // ── Plugins ──────────────────────────────────────────────────────────────
  async function listPlugins(): Promise<PluginRecord[]> {
    return $fetch<PluginRecord[]>(`${BASE}/plugins`, { headers: h() })
  }

  async function unloadPlugin(name: string): Promise<void> {
    await $fetch(`${BASE}/plugins/${name}`, { method: 'DELETE', headers: h() })
  }

  async function uploadPlugin(formData: FormData): Promise<PluginRecord> {
    return $fetch<PluginRecord>(`${BASE}/plugins/upload`, { method: 'POST', body: formData, headers: h() })
  }

  // ── MFA / TOTP ────────────────────────────────────────────────────────────
  async function getMfaStatus(): Promise<{ enabled: boolean }> {
    return $fetch<{ enabled: boolean }>(`${BASE}/auth/mfa/status`, { headers: h() })
  }

  async function setupMfa(): Promise<{ secret: string; uri: string; qr_code: string }> {
    return $fetch<{ secret: string; uri: string; qr_code: string }>(`${BASE}/auth/mfa/setup`, { method: 'POST', headers: h() })
  }

  async function enableMfa(code: string): Promise<{ enabled: boolean }> {
    return $fetch<{ enabled: boolean }>(`${BASE}/auth/mfa/enable`, { method: 'POST', body: { code }, headers: h() })
  }

  async function disableMfa(code: string): Promise<{ enabled: boolean }> {
    return $fetch<{ enabled: boolean }>(`${BASE}/auth/mfa/disable`, { method: 'POST', body: { code }, headers: h() })
  }

  // ── Storage backends ──────────────────────────────────────────────────────
  async function listStorageBackends(): Promise<StorageBackend[]> {
    return $fetch<StorageBackend[]>(`${BASE}/storage/backends`, { headers: h() })
  }

  async function createStorageBackend(payload: {
    name: string; provider: string; endpoint: string; region: string
    access_key: string; secret_key: string; path_style: boolean; use_ssl: boolean
  }): Promise<StorageBackend> {
    return $fetch<StorageBackend>(`${BASE}/storage/backends`, { method: 'POST', body: payload, headers: h() })
  }

  async function deleteStorageBackend(id: string): Promise<void> {
    await $fetch(`${BASE}/storage/backends/${id}`, { method: 'DELETE', headers: h() })
  }

  async function testStorageBackend(id: string): Promise<{ ok: boolean; buckets?: number; error?: string }> {
    return $fetch(`${BASE}/storage/backends/${id}/test`, { method: 'POST', headers: h() })
  }

  async function listStorageBuckets(id: string): Promise<BucketInfo[]> {
    return $fetch<BucketInfo[]>(`${BASE}/storage/backends/${id}/buckets`, { headers: h() })
  }

  // ── Network / mesh ────────────────────────────────────────────────────────
  async function getMeshSummary(): Promise<{ total: number; healthy: number; degraded: number }> {
    return $fetch<{ total: number; healthy: number; degraded: number }>(`${BASE}/network/mesh-summary`, { headers: h() })
  }

  // ── Observability ─────────────────────────────────────────────────────────
  async function listLogs(): Promise<LogEntry[]> {
    return $fetch<LogEntry[]>(`${BASE}/observability/logs`, { headers: h() })
  }

  async function listAuditLogs(): Promise<AuditEntry[]> {
    return $fetch<AuditEntry[]>(`${BASE}/audit/logs`, { headers: h() })
  }

  async function listTraces(): Promise<TraceEntry[]> {
    return $fetch<TraceEntry[]>(`${BASE}/observability/traces`, { headers: h() })
  }

  async function gitSync(payload: GitSyncRequest): Promise<{ message: string }> {
    return $fetch<{ message: string }>(`${BASE}/git/sync`, { method: 'POST', body: payload, headers: h() })
  }

  // ── Recovery codes ─────────────────────────────────────────────────────────
  async function getRecoveryCodeStatus(): Promise<{ remaining: number }> {
    return $fetch<{ remaining: number }>(`${BASE}/auth/recovery-codes`, { headers: h() })
  }

  async function regenerateRecoveryCodes(): Promise<{ recovery_codes: string[] }> {
    return $fetch<{ recovery_codes: string[] }>(`${BASE}/auth/recovery-codes/regenerate`, { method: 'POST', headers: h() })
  }

  async function forgotPassword(payload: {
    username: string
    new_password: string
    recovery_code?: string
    totp_code?: string
  }): Promise<{ message: string }> {
    return $fetch<{ message: string }>(`${BASE}/auth/forgot-password`, { method: 'POST', body: payload })
  }

  // ── Audit export ───────────────────────────────────────────────────────────
  function auditExportUrl(format: 'csv' | 'json' = 'csv', limit = 5000): string {
    const token = getToken()
    return `${BASE}/audit/logs/export?format=${format}&limit=${limit}&token=${encodeURIComponent(token)}`
  }

  // ── Database export ────────────────────────────────────────────────────────
  async function exportDatabase(): Promise<Blob> {
    const res = await fetch(`${BASE}/admin/export`, { headers: h() })
    if (!res.ok) throw new Error(`export failed: ${res.status}`)
    return res.blob()
  }

  return {
    login, setupStatus, doSetup,
    getVersion,
    listUsers, createUser, updateUser, deleteUser,
    listRoles, createRole, updateRole, deleteRole,
    listNodes, getNode, createNode, deleteNode,
    getMe, listTokens, deleteToken, createToken,
    getMfaStatus, setupMfa, enableMfa, disableMfa,
    listContainers, getContainerInspect, getContainerLogsJSON, getContainerStats, deployCompose,
    startContainer, stopContainer, restartContainer, removeContainer,
    applyK8s, getK8sPod, getK8sPodLogsJSON, getK8sSummary, listAllK8sPods, deleteK8sPod,
    listPlugins, unloadPlugin, uploadPlugin,
    listStorageBackends, createStorageBackend, deleteStorageBackend, testStorageBackend, listStorageBuckets,
    getMeshSummary,
    listLogs, listAuditLogs, listTraces,
    gitSync,
    getRecoveryCodeStatus, regenerateRecoveryCodes, forgotPassword, auditExportUrl,
    exportDatabase,
  }
}
