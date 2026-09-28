import { useNodeSelection } from './nodeSelection'

// API base prefix — proxied to the Go backend by Vite in dev (see vite.config.ts)
// and served from the same origin in production.
const BASE = '/api'

const TOKEN_KEY = 'scutum_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

function authHeaders(): Record<string, string> {
  const token = getToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

// A node target of `null` means local (no header); a string pins a specific
// node; omitting the argument (see nodeHeaders callers below) means "follow
// whatever's picked in the topbar switcher" via useNodeSelection.
function nodeHeaders(target: string | null): Record<string, string> {
  return target ? { 'X-Target-Node': target } : {}
}

function globalNodeHeaders(): Record<string, string> {
  return nodeHeaders(useNodeSelection.getState().selectedNodeId)
}

async function request<T>(path: string, init?: RequestInit, nodeTarget?: Record<string, string>): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders(),
      ...nodeTarget,
      ...init?.headers,
    },
  })

  if (!res.ok) {
    // Backend error bodies are inconsistent: most handlers use http.Error with a
    // plain-text message, a few write a JSON string through http.Error, and a
    // handful encode real JSON with an "error" field. Handle all three.
    let message = res.statusText
    const text = await res.text()
    if (text) {
      try {
        const body = JSON.parse(text)
        message = typeof body?.error === 'string' ? body.error : text
      } catch {
        message = text.trim()
      }
    }
    throw new ApiError(res.status, message)
  }

  // Several handlers (Scale, Restart, Create Secret, …) succeed with a
  // 200/201 and an empty body (Content-Length: 0), not just a 204 — and
  // res.json() throws a SyntaxError on empty text regardless of status.
  // Read as text first so a genuinely empty success response resolves to
  // undefined instead of rejecting the promise.
  const text = await res.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

function get<T>(path: string, nodeTarget?: Record<string, string>): Promise<T> {
  return request<T>(path, undefined, nodeTarget)
}

function post<T>(path: string, body?: unknown, nodeTarget?: Record<string, string>): Promise<T> {
  return request<T>(
    path,
    { method: 'POST', body: body !== undefined ? JSON.stringify(body) : undefined },
    nodeTarget,
  )
}

function put<T>(path: string, body?: unknown, nodeTarget?: Record<string, string>): Promise<T> {
  return request<T>(
    path,
    { method: 'PUT', body: body !== undefined ? JSON.stringify(body) : undefined },
    nodeTarget,
  )
}

function del<T>(path: string, nodeTarget?: Record<string, string>): Promise<T> {
  return request<T>(path, { method: 'DELETE' }, nodeTarget)
}

// ── Setup ────────────────────────────────────────────────────────────────
export interface KmsConfig {
  provider: 'local' | 'vault' | 'aws' | 'gcp' | 'azure'
  local?: { key_file?: string }
  vault?: { addr: string; key_name: string; token_file?: string }
  aws?: { region: string; key_id: string; access_key?: string; secret_key?: string }
  gcp?: { project_id: string; location_id?: string; key_ring_id: string; key_id: string; token_file?: string }
  azure?: { vault_url: string; key_name?: string; tenant_id: string; client_id: string; token_file?: string }
}

export interface SetupRequest {
  install_type: 'hub' | 'remote' | 'combined'
  kms: KmsConfig
  wireguard: {
    listen_port?: number
    address: string
    mtu?: number
    hub_endpoint?: string
    hub_public_key?: string
    hub_allowed_ips?: string
    hub_hmac_key?: string
    hub_api_address?: string
  }
  admin: { username: string; password: string }
  recovery?: { n_shares: number; threshold: number }
}

export interface SetupResponse {
  // Normal 201 response
  message?: string
  admin_id?: string
  kms_provider?: string
  install_type?: string
  wireguard?: { public_key: string; address?: string; listen_port?: number; warning?: string }
  recovery_shares?: string[]
  // 202 restart response (wireguard-go was just installed)
  status?: 'restarting'
}

export function getSetupStatus(): Promise<{ complete: boolean }> {
  return get('/setup/status')
}

export function runSetup(payload: SetupRequest): Promise<SetupResponse> {
  return post('/setup', payload)
}

// ── Auth ─────────────────────────────────────────────────────────────────
export interface LoginResponse {
  token?: string
  totp_required?: boolean
  // Set when Settings → Auth's "require MFA for every user" is on and this
  // account hasn't enabled it yet. The credentials were still correct, so a
  // token is issued — the frontend routes straight to Account's MFA setup
  // instead of silently letting them into the rest of the app.
  mfa_setup_required?: boolean
}

export function login(username: string, password: string, totpCode?: string): Promise<LoginResponse> {
  const body: Record<string, string> = { username, password }
  if (totpCode) body.totp_code = totpCode
  return post('/auth/login', body)
}

export interface UserProfile {
  id: string
  username: string
  roles: string[]
  created_at: string
}

export function getMe(): Promise<UserProfile> {
  return get('/auth/me')
}

export interface ForgotPasswordRequest {
  username: string
  new_password: string
  recovery_code?: string
  totp_code?: string
}

export function forgotPassword(payload: ForgotPasswordRequest): Promise<{ message: string }> {
  return post('/auth/forgot-password', payload)
}

// ── SSO ──────────────────────────────────────────────────────────────────
export interface SSOProvider {
  id: string
  name: string
  icon: string
}

export function getSSOProviders(): Promise<SSOProvider[]> {
  return get('/auth/sso/providers')
}

// ── Nodes ────────────────────────────────────────────────────────────────
export interface NodeRecord {
  id: string
  name: string
  type: string
  address: string
  public_key: string
  // "approved" or "pending" — only meaningful when Settings → Nodes has
  // "require approval" turned on. Nodes created while it's off go straight
  // to "approved", same as before this field existed.
  status: string
}

export function listNodes(): Promise<NodeRecord[]> {
  return get('/nodes')
}

export interface CreateNodeRequest {
  name: string
  type: string
  address: string
  public_key: string
}

export function createNode(payload: CreateNodeRequest): Promise<NodeRecord> {
  return post('/nodes', payload)
}

export function deleteNode(id: string): Promise<void> {
  return del(`/nodes/${id}`)
}

export function approveNode(id: string): Promise<void> {
  return post(`/nodes/${id}/approve`)
}

export function rejectNode(id: string): Promise<void> {
  return post(`/nodes/${id}/reject`)
}

// ── Node groups ──────────────────────────────────────────────────────────
export interface NodeGroup {
  id: string
  name: string
  description: string
  members?: string[]
  created_at: string
}

export function listNodeGroups(): Promise<NodeGroup[]> {
  return get('/groups')
}

export function createNodeGroup(payload: { name: string; description?: string }): Promise<NodeGroup> {
  return post('/groups', payload)
}

export function deleteNodeGroup(id: string): Promise<void> {
  return del(`/groups/${id}`)
}

export function getGroupNodes(groupId: string): Promise<NodeRecord[]> {
  return get(`/groups/${groupId}/nodes`)
}

export function addNodeToGroup(groupId: string, nodeId: string): Promise<void> {
  return post(`/groups/${groupId}/members`, { node_id: nodeId })
}

export function removeNodeFromGroup(groupId: string, nodeId: string): Promise<void> {
  return del(`/groups/${groupId}/members/${nodeId}`)
}

// ── Docker ───────────────────────────────────────────────────────────────
export interface DockerContainer {
  Id: string
  Names: string[]
  Image: string
  Status: string
  State: string
  Ports: Array<{ PrivatePort?: number; PublicPort?: number; Type?: string }>
}

export interface ContainerStats {
  cpu_percent: number
  mem_usage: number
  mem_limit: number
  net_rx: number
  net_tx: number
  blk_read: number
  blk_write: number
}

// `nodeId` explicit and omitted are different here: omitted means "local",
// it does NOT follow the topbar's node switcher (matching the real app —
// the containers list fetches every node's containers explicitly, one call
// per node, rather than only ever showing whichever node is selected).
export function listContainers(nodeId?: string): Promise<DockerContainer[]> {
  return get('/docker/containers', nodeId ? nodeHeaders(nodeId) : undefined)
}

// ── System stats (Resource Monitoring page) ───────────────────────────────
export interface SystemStats {
  cpu_percent: number
  mem_used: number
  mem_total: number
  mem_percent: number
  disk_used: number
  disk_total: number
  disk_percent: number
  load_1: number
  load_5: number
  load_15: number
  recorded_at: string
}

export interface NodeStatRecord {
  id: string
  cpu_percent: number
  mem_used: number
  mem_total: number
  disk_used: number
  disk_total: number
  load_1: number
  load_5: number
  load_15: number
  recorded_at: string
}

// nodeId explicit (not the topbar switcher) — the Monitoring page fetches
// every node's stats in parallel, same pattern as listContainers.
export function getSystemStats(nodeId?: string | null): Promise<SystemStats> {
  return get('/system/stats', nodeId ? nodeHeaders(nodeId) : undefined)
}

// History rows don't carry mem_percent/disk_percent — only cpu_percent is
// used for the sparkline, matching the real app's history endpoint shape.
export function getSystemStatsHistory(nodeId?: string | null, limit = 60): Promise<NodeStatRecord[]> {
  return get(`/system/stats/history?limit=${limit}`, nodeId ? nodeHeaders(nodeId) : undefined)
}

export function getContainerInspect(id: string): Promise<Record<string, any>> {
  return get(`/docker/containers/${id}`, globalNodeHeaders())
}

export interface ContainerLogLine {
  ts: string
  stream: string
  msg: string
}

export function getContainerLogsJSON(id: string, tail = 100): Promise<ContainerLogLine[]> {
  return get(`/docker/containers/${id}/logs-json?tail=${tail}`, globalNodeHeaders())
}

// Unlike listContainers, omitting `nodeId` here follows the global switcher
// (matches the real app's getContainerStats — used from a container detail
// page that's already "inside" whatever node context it was opened from).
// Pass `null` explicitly to force local regardless of the switcher.
export function getContainerStats(id: string, nodeId?: string | null): Promise<ContainerStats> {
  // /stats is Docker's raw infinite-streaming endpoint (chunked, never closes);
  // /stats-snapshot asks the backend for `stream=false`, one object, connection closes.
  return get(`/docker/containers/${id}/stats-snapshot`, nodeId === undefined ? globalNodeHeaders() : nodeHeaders(nodeId))
}

export function startContainer(id: string): Promise<void> {
  return post(`/docker/containers/${id}/start`, undefined, globalNodeHeaders())
}

export function stopContainer(id: string): Promise<void> {
  return post(`/docker/containers/${id}/stop`, undefined, globalNodeHeaders())
}

export function restartContainer(id: string): Promise<void> {
  return post(`/docker/containers/${id}/restart`, undefined, globalNodeHeaders())
}

export function removeContainer(id: string): Promise<void> {
  return del(`/docker/containers/${id}`, globalNodeHeaders())
}

export function deployCompose(yaml: string, nodeId?: string): Promise<{ output?: string; error?: string }> {
  return request(
    '/docker/deploy-compose',
    { method: 'POST', body: yaml, headers: { 'Content-Type': 'text/yaml' } },
    nodeId ? nodeHeaders(nodeId) : globalNodeHeaders(),
  )
}

// ── Network ──────────────────────────────────────────────────────────────
export interface MeshSummary {
  total: number
  healthy: number
  degraded: number
}

export function getMeshSummary(): Promise<MeshSummary> {
  return get('/network/mesh-summary')
}

export interface AddPeerRequest {
  public_key: string
  endpoint: string
  allowed_ips: string
  node_id?: string
  persistent_keepalive?: number
}

export function addPeer(payload: AddPeerRequest): Promise<void> {
  return post('/network/peer', payload)
}

export function getHubKey(): Promise<{ hmac_key: string }> {
  return get('/network/hub-key')
}

export interface PeerStatus {
  public_key: string
  node_id?: string
  node_name?: string
  endpoint?: string
  allowed_ips?: string
  last_handshake: number
  rx_bytes: number
  tx_bytes: number
  quality: 'good' | 'degraded' | 'dead'
}

export function getMeshPeers(): Promise<PeerStatus[]> {
  return get('/network/peers')
}

// ── Federation ───────────────────────────────────────────────────────────
export interface FederationPeer {
  id: string
  name: string
  hub_url: string
  wg_endpoint: string
  wg_public_key: string
  mesh_cidr: string
  allowed_ips: string
  status: 'pending' | 'connected' | 'error'
  last_seen?: string
  created_at: string
}

export interface CreateFederationPeerRequest {
  name: string
  hub_url?: string
  wg_endpoint: string
  wg_public_key: string
  mesh_cidr: string
  allowed_ips?: string
}

export function listFederationPeers(): Promise<FederationPeer[]> {
  return get('/federation/peers')
}

export function createFederationPeer(payload: CreateFederationPeerRequest): Promise<FederationPeer> {
  return post('/federation/peers', payload)
}

export function deleteFederationPeer(id: string): Promise<void> {
  return del(`/federation/peers/${id}`)
}

// ── Kubernetes ───────────────────────────────────────────────────────────
export interface K8sSummary {
  pods: number
  running: number
  pending: number
  failed: number
  succeeded: number
  namespaces: number
  nodes: number
  deployments: number
  healthy_deploys: number
  unhealthy_deploys: number
}

// Same 3-way targeting as getContainerStats: omit nodeId to follow the
// topbar switcher, pass null to force local, pass a node id to force remote.
export function getK8sSummary(nodeId?: string | null): Promise<K8sSummary> {
  return get('/kubernetes/summary', nodeId === undefined ? globalNodeHeaders() : nodeHeaders(nodeId))
}

// Raw Kubernetes pod list response — passed straight through from the
// cluster's API server, hence `any` items (matches the real app's typing).
export function listAllK8sPods(nodeId?: string | null): Promise<{ items?: any[] }> {
  return get('/kubernetes/pods', nodeId === undefined ? globalNodeHeaders() : nodeHeaders(nodeId))
}

export function getK8sPod(ns: string, name: string): Promise<any> {
  return get(`/kubernetes/${ns}/pods/${name}`, globalNodeHeaders())
}

export interface K8sLogLine {
  ts: string
  msg: string
}

export function getK8sPodLogsJSON(ns: string, name: string, container?: string, tail = 100): Promise<K8sLogLine[]> {
  const q = new URLSearchParams({ tail: String(tail) })
  if (container) q.set('container', container)
  return get(`/kubernetes/${ns}/pods/${name}/logs-json?${q}`, globalNodeHeaders())
}

export function deleteK8sPod(ns: string, name: string): Promise<void> {
  return del(`/kubernetes/${ns}/pods/${name}`, globalNodeHeaders())
}

export function applyK8s(yaml: string, nodeId?: string): Promise<{ output?: string; error?: string }> {
  return request(
    '/kubernetes/apply',
    { method: 'POST', body: yaml, headers: { 'Content-Type': 'text/yaml' } },
    nodeId ? nodeHeaders(nodeId) : globalNodeHeaders(),
  )
}

// GET /kubernetes/deployments — namespace/name/replica counts across every
// namespace, just enough to list, scale, and restart from the Deployments
// tab. Same 3-way node targeting as getK8sSummary/listAllK8sPods.
export interface K8sDeployment {
  namespace: string
  name: string
  replicas: number
  ready_replicas: number
}

export function listDeployments(nodeId?: string | null): Promise<K8sDeployment[]> {
  return get('/kubernetes/deployments', nodeId === undefined ? globalNodeHeaders() : nodeHeaders(nodeId))
}

export function scaleDeployment(ns: string, name: string, replicas: number, nodeId?: string | null): Promise<void> {
  return post(
    `/kubernetes/${ns}/deployments/${name}/scale?replicas=${replicas}`,
    undefined,
    nodeId === undefined ? globalNodeHeaders() : nodeHeaders(nodeId),
  )
}

export function restartDeployment(ns: string, name: string, nodeId?: string | null): Promise<void> {
  return post(
    `/kubernetes/${ns}/deployments/${name}/restart`,
    undefined,
    nodeId === undefined ? globalNodeHeaders() : nodeHeaders(nodeId),
  )
}

// Real Kubernetes Secret objects (Settings → Secrets), not app-level
// credentials. Values are never returned by the API — list/get expose only
// key NAMES — so nothing here can leak secret material to the browser.
export interface K8sSecretInfo {
  namespace: string
  name: string
  type: string
  created_at: string
  keys: string[]
}

export function listSecrets(): Promise<K8sSecretInfo[]> {
  return get('/kubernetes/secrets', globalNodeHeaders())
}

export function getSecret(ns: string, name: string): Promise<K8sSecretInfo> {
  return get(`/kubernetes/${ns}/secrets/${name}`, globalNodeHeaders())
}

export function createSecret(ns: string, name: string, type: string, data: Record<string, string>): Promise<void> {
  return post(`/kubernetes/${ns}/secrets`, { name, type, data }, globalNodeHeaders())
}

export function deleteSecret(ns: string, name: string): Promise<void> {
  return del(`/kubernetes/${ns}/secrets/${name}`, globalNodeHeaders())
}

// Only meaningful for Opaque secrets — the backend rejects TLS/Docker-
// registry secrets with a 400 since randomizing structured data would just
// corrupt it.
export function rotateSecret(ns: string, name: string): Promise<void> {
  return post(`/kubernetes/${ns}/secrets/${name}/rotate`, undefined, globalNodeHeaders())
}

// ── Secrets vault (KMS-backed named secrets, distinct from the Kubernetes
// Secret objects above) ─────────────────────────────────────────────────
// Values are write-only from the browser's perspective: create/update send
// a plaintext value, but no response — including list — ever returns one
// back. Reference a vault secret from a container/pod env var or a Compose/
// Kubernetes YAML manifest with "secret://<name>"; it's resolved to the
// real value on the hub at deploy time, never sent to the target node as
// a reference.
export interface VaultSecret {
  id: string
  name: string
  description: string
  created_by: string
  created_at: string
  updated_by: string
  updated_at: string
}

function vaultSecretPath(name: string): string {
  return `/secrets/${name.split('/').map(encodeURIComponent).join('/')}`
}

export function listVaultSecrets(): Promise<VaultSecret[]> {
  return get('/secrets')
}

export function createVaultSecret(name: string, description: string, value: string): Promise<VaultSecret> {
  return post('/secrets', { name, description, value })
}

export function updateVaultSecret(name: string, fields: { description?: string; value?: string }): Promise<VaultSecret> {
  return put(vaultSecretPath(name), fields)
}

export function deleteVaultSecret(name: string): Promise<void> {
  return del(vaultSecretPath(name))
}

// ── Observability ────────────────────────────────────────────────────────
export interface LogEntry {
  time: string
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
}

export function listLogs(): Promise<LogEntry[]> {
  return get('/observability/logs')
}

export interface TraceEntry {
  trace_id?: string
  span_id?: string
  parent_span_id?: string
  name: string
  service?: string
  kind?: string
  time: string
  duration_ms: number
  status: 'ok' | 'error' | 'unset'
  error?: string
  source?: string
  attributes?: Record<string, string>
}

export function listTraces(): Promise<TraceEntry[]> {
  return get('/observability/traces')
}

export function getContainerTraces(id: string, tail = 200): Promise<TraceEntry[]> {
  return get(`/docker/containers/${id}/traces?tail=${tail}`)
}

export interface MetricPoint {
  time: string
  name: string
  service?: string
  source?: string
  type: string
  value: number
  labels?: Record<string, string>
}

export function listMetrics(params?: { name?: string; service?: string; limit?: number }): Promise<MetricPoint[]> {
  const q = new URLSearchParams()
  if (params?.name) q.set('name', params.name)
  if (params?.service) q.set('service', params.service)
  if (params?.limit) q.set('limit', String(params.limit))
  return get(`/observability/metrics?${q}`)
}

export function scrapeContainerMetrics(id: string, port = '9090', path = '/metrics'): Promise<MetricPoint[]> {
  const q = new URLSearchParams({ port, path })
  return get(`/docker/containers/${id}/metrics-scrape?${q}`)
}

// Raw streaming reads (Docker container logs, Kubernetes events) bypass the
// request()/get() JSON helpers — these responses are unbounded text/NDJSON
// streams, not a single JSON body — so callers read the fetch body directly.
export function streamUrl(path: string): string {
  return `${BASE}${path}`
}

// WebSocket endpoints (terminal exec) take the auth token as a query param
// instead of an Authorization header, since the browser WebSocket API can't
// set custom headers on the upgrade request.
export function wsUrl(path: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${window.location.host}${BASE}${path}`
}

export function streamHeaders(nodeId?: string | null): Record<string, string> {
  return {
    ...authHeaders(),
    ...(nodeId === undefined ? globalNodeHeaders() : nodeHeaders(nodeId)),
  }
}

// ── Storage ──────────────────────────────────────────────────────────────
export interface StorageBackend {
  id: string
  name: string
  provider: string
  endpoint: string
  region: string
  access_key: string
  path_style: boolean
  use_ssl: boolean
  created_at?: string
}

export interface CreateStorageBackendRequest {
  name: string
  provider: string
  endpoint: string
  region: string
  access_key: string
  secret_key: string
  path_style: boolean
  use_ssl: boolean
}

export function listStorageBackends(): Promise<StorageBackend[]> {
  return get('/storage/backends')
}

export function createStorageBackend(payload: CreateStorageBackendRequest): Promise<StorageBackend> {
  return post('/storage/backends', payload)
}

export function deleteStorageBackend(id: string): Promise<void> {
  return del(`/storage/backends/${id}`)
}

// Always resolves — the backend reports connection failures as {ok: false,
// error} in a 200 response rather than throwing, so a caller-visible
// failure doesn't need a try/catch.
export function testStorageBackend(id: string): Promise<{ ok: boolean; buckets?: number; error?: string }> {
  return post(`/storage/backends/${id}/test`)
}

export interface BucketInfo {
  name: string
  created_at?: string
}

export function listStorageBuckets(id: string): Promise<BucketInfo[]> {
  return get(`/storage/backends/${id}/buckets`)
}

// ── GitOps ───────────────────────────────────────────────────────────────
// The only real endpoint here — clones the repo into target_dir if it
// doesn't exist yet, else pulls. Only https:// URLs are accepted. There's
// no repo registry or sync history server-side; both are client-side state
// (see GitOps.tsx).
export interface GitSyncRequest {
  repo_url: string
  username?: string
  token?: string
  target_dir: string
}

// The handler writes a plain-text body ("Sync successful") on success, not
// JSON — unlike nearly everything else in this API — so this bypasses
// post()/request()'s res.json() and reads text directly.
export async function gitSync(payload: GitSyncRequest): Promise<string> {
  const res = await fetch(`${BASE}/git/sync`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(payload),
  })
  const text = await res.text()
  if (!res.ok) throw new ApiError(res.status, text.trim() || res.statusText)
  return text
}

// ── Plugins ──────────────────────────────────────────────────────────────
// The backend tracks only { name, path } per loaded WASM plugin — no
// version, description, or load timestamp (see PluginInfo in
// cmd/internal/plugins/registry.go). Every plugin gets the same fixed set
// of sandboxed host functions; there's no per-plugin capability model.
export interface PluginInfo {
  name: string
  path: string
}

export function listPlugins(): Promise<PluginInfo[]> {
  return get('/plugins')
}

export function unloadPlugin(name: string): Promise<void> {
  return del(`/plugins/${encodeURIComponent(name)}`)
}

// Multipart upload — deliberately bypasses post()'s forced
// Content-Type: application/json so the browser can set the multipart
// boundary itself.
export async function uploadPlugin(name: string, file: File): Promise<{ name: string; path: string }> {
  const body = new FormData()
  body.append('name', name)
  body.append('file', file)
  const res = await fetch(`${BASE}/plugins/upload`, {
    method: 'POST',
    headers: authHeaders(),
    body,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new ApiError(res.status, text.trim() || res.statusText)
  }
  return res.json()
}

// ── Audit log ────────────────────────────────────────────────────────────
export interface AuditEntry {
  time: string
  action: string
  actor?: string
  actor_id?: string
  outcome?: 'success' | 'failure'
  method: string
  path: string
  trace_id?: string
  client_ip?: string
  extra?: Record<string, string>
}

export function listAuditLogs(): Promise<AuditEntry[]> {
  return get('/audit/logs')
}

// A plain download link, not a fetch() call — the token travels as a query
// param for the same reason WebSocket URLs do (no way to set a header on
// an <a download> navigation).
export function auditExportUrl(format: 'csv' | 'json' = 'csv', limit = 5000): string {
  const token = getToken() ?? ''
  return `${BASE}/audit/logs/export?format=${format}&limit=${limit}&token=${encodeURIComponent(token)}`
}

// ── Compliance report ────────────────────────────────────────────────────
const COMPLIANCE_EXT: Record<'json' | 'csv' | 'text', string> = { json: 'json', csv: 'csv', text: 'txt' }

export async function downloadComplianceReport(format: 'json' | 'csv' | 'text'): Promise<void> {
  const res = await fetch(`${BASE}/compliance/report?format=${format}`, { headers: authHeaders() })
  if (!res.ok) {
    const text = await res.text()
    throw new ApiError(res.status, text.trim() || res.statusText)
  }
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `cra-compliance-report.${COMPLIANCE_EXT[format]}`
  a.click()
  URL.revokeObjectURL(url)
}

// ── Users & Roles ────────────────────────────────────────────────────────
// `roles` on a user is a list of role *names*, not IDs — the backend
// resolves names to role IDs server-side (see resolveRoleIDs in
// users_handler.go) and returns names back via GetUserRoleNames.
export interface UserRecord {
  id: string
  username: string
  roles: string[]
  created_at: string
}

export function listUsers(): Promise<UserRecord[]> {
  return get('/users')
}

export function createUser(payload: { username: string; password: string; roles?: string[] }): Promise<{ id: string; username: string }> {
  return post('/users', payload)
}

export function updateUser(id: string, payload: { username?: string; password?: string; roles?: string[] }): Promise<void> {
  return put(`/users/${id}`, payload)
}

export function deleteUser(id: string): Promise<void> {
  return del(`/users/${id}`)
}

// perms are stored canonically as "resource:level" (a trailing ":*" is
// rewritten to ":admin" server-side — sending ":admin" directly works too).
export interface RoleRecord {
  id: string
  name: string
  description: string
  perms: string[]
}

export function listRoles(): Promise<RoleRecord[]> {
  return get('/roles')
}

export function createRole(payload: { name: string; description?: string; perms?: string[] }): Promise<{ id: string; name: string }> {
  return post('/roles', payload)
}

export function updateRole(id: string, payload: { name: string; description?: string; perms?: string[] }): Promise<void> {
  return put(`/roles/${id}`, payload)
}

export function deleteRole(id: string): Promise<void> {
  return del(`/roles/${id}`)
}

// ── Webhooks ─────────────────────────────────────────────────────────────
// Unlike most secret fields elsewhere in this API, `secret` is NOT
// write-only — GET/list responses include it verbatim when set (it's
// omitted only when empty). PUT has no "leave unchanged" semantics either:
// whatever `secret` you send replaces the stored one, including "" — so a
// caller editing a webhook must carry the existing secret forward itself.
export interface WebhookConfig {
  id: string
  name: string
  url: string
  secret?: string
  events: string[]
  enabled: boolean
  created_at: string
}

export interface WebhookRequest {
  name: string
  url: string
  secret: string
  events: string[]
  enabled?: boolean
}

export function listWebhooks(): Promise<WebhookConfig[]> {
  return get('/webhooks')
}

export function createWebhook(payload: WebhookRequest): Promise<WebhookConfig> {
  return post('/webhooks', payload)
}

export function updateWebhook(id: string, payload: WebhookRequest): Promise<WebhookConfig> {
  return put(`/webhooks/${id}`, payload)
}

export function deleteWebhook(id: string): Promise<void> {
  return del(`/webhooks/${id}`)
}

export function testWebhook(id: string): Promise<void> {
  return post(`/webhooks/${id}/test`)
}

// ── SCIM ─────────────────────────────────────────────────────────────────
// store.SCIMTokenInfo has no json tags, so GET /scim/tokens serializes Go's
// exported field names verbatim (PascalCase) — the one place in this API
// that isn't snake_case. Confirmed against the live response, not assumed.
export interface ScimToken {
  ID: string
  Description: string
  CreatedAt: string
}

export function listSCIMTokens(): Promise<ScimToken[]> {
  return get('/scim/tokens')
}

export function createSCIMToken(description: string): Promise<{ id: string; token: string }> {
  return post('/scim/tokens', { description })
}

export function deleteSCIMToken(id: string): Promise<void> {
  return del(`/scim/tokens/${id}`)
}

// ── Backup & Restore ─────────────────────────────────────────────────────
// Manual-trigger only — no schedule/cron config in the real backend.
// `driver` is chosen server-side from config, not at creation time.
export interface BackupRecord {
  id: string
  filename: string
  driver: string
  size_bytes: number
  created_at: string
}

export function listBackups(): Promise<BackupRecord[]> {
  return get('/admin/backups')
}

export function createBackup(): Promise<BackupRecord> {
  return post('/admin/backups')
}

export function deleteBackup(id: string): Promise<void> {
  return del(`/admin/backups/${encodeURIComponent(id)}`)
}

export function restoreBackup(id: string): Promise<{ status: string; requires_restart: boolean; message: string }> {
  return post(`/admin/backups/${encodeURIComponent(id)}/restore`)
}

// The old app's downloadBackupUrl doesn't append a token, so a plain <a
// download> navigation 401s (no Authorization header on that request) —
// fixed here the same way auditExportUrl already does it correctly.
export function downloadBackupUrl(id: string): string {
  const token = getToken() ?? ''
  return `${BASE}/admin/backups/${encodeURIComponent(id)}/download?token=${encodeURIComponent(token)}`
}

// ── Alerts ───────────────────────────────────────────────────────────────
export interface AlertRule {
  id: string
  name: string
  condition: 'cpu_percent' | 'mem_percent' | 'disk_percent' | 'node_offline' | 'handshake_age'
  threshold: number
  severity: 'info' | 'warning' | 'critical'
  enabled: boolean
  silenced_until?: string
  created_at: string
}

export interface AlertEvent {
  id: string
  rule_id: string
  rule_name: string
  severity: string
  message: string
  fired_at: string
  resolved_at?: string
  acknowledged_at?: string
}

export function listAlertRules(): Promise<AlertRule[]> {
  return get('/alerts/rules')
}

export function createAlertRule(rule: Omit<AlertRule, 'id' | 'created_at'>): Promise<AlertRule> {
  return post('/alerts/rules', rule)
}

export function updateAlertRule(id: string, rule: Partial<AlertRule>): Promise<AlertRule> {
  return put(`/alerts/rules/${id}`, rule)
}

export function deleteAlertRule(id: string): Promise<void> {
  return del(`/alerts/rules/${id}`)
}

export function silenceAlertRule(id: string, until: string): Promise<AlertRule> {
  return put(`/alerts/rules/${id}/silence`, { until })
}

export function listAlertEvents(limit = 100): Promise<AlertEvent[]> {
  return get(`/alerts/events?limit=${limit}`)
}

export function acknowledgeAlertEvent(id: string): Promise<void> {
  return post(`/alerts/events/${id}/acknowledge`)
}

// ── Audit forwarders ─────────────────────────────────────────────────────
export interface AuditForwarder {
  id: string
  name: string
  url: string
  format: 'json' | 'cef'
  enabled: boolean
  created_at: string
}

export function listAuditForwarders(): Promise<AuditForwarder[]> {
  return get('/audit/forwarders')
}

export function createAuditForwarder(payload: { name: string; url: string; format: 'json' | 'cef' }): Promise<AuditForwarder> {
  return post('/audit/forwarders', payload)
}

export function updateAuditForwarder(id: string, payload: Partial<Pick<AuditForwarder, 'name' | 'url' | 'format' | 'enabled'>>): Promise<AuditForwarder> {
  return put(`/audit/forwarders/${id}`, payload)
}

export function deleteAuditForwarder(id: string): Promise<void> {
  return del(`/audit/forwarders/${id}`)
}

// ── TLS mode ─────────────────────────────────────────────────────────────
// Read-only — TLS is controlled by env vars (ACME_DOMAIN / CERT_FILE), not
// this UI. No auth required, matching the real endpoint.
export function getTLSMode(): Promise<{ mode: string; domain?: string; email?: string; staging?: boolean; cert_file?: string }> {
  return get('/system/tls-mode')
}

// ── System settings (Settings → General/Mesh/Nodes/Auth) ─────────────────
// GET/PUT /settings, admin-only. All fields are genuinely persisted and
// consumed server-side — see cmd/internal/handlers/settings_handler.go —
// not just stored for display: LogLevel changes the running logger's level
// immediately, MeshMTU is applied live to the wg0 interface, MeshKeepalive
// becomes the default persistent-keepalive for newly added peers,
// NodeRequireApproval gates new node enrollment behind Nodes page
// approve/reject, and AuthSessionTimeoutMin/AuthRequireMFA take effect on
// the next login.
export interface SystemSettings {
  ClusterName: string
  Region: string
  LogLevel: 'debug' | 'info' | 'warn' | 'error'
  MeshMTU: number
  MeshKeepaliveSeconds: number
  NodeDefaultRole: 'hub' | 'remote'
  NodeRequireApproval: boolean
  AuthRequireMFA: boolean
  AuthSessionTimeoutMin: number
}

export function getSystemSettings(): Promise<SystemSettings> {
  return get('/settings')
}

export function updateSystemSettings(settings: SystemSettings): Promise<SystemSettings> {
  return put('/settings', settings)
}

// ── Database export ──────────────────────────────────────────────────────
export async function exportDatabase(): Promise<Blob> {
  const res = await fetch(`${BASE}/admin/export`, { headers: authHeaders() })
  if (!res.ok) {
    const text = await res.text()
    throw new ApiError(res.status, text.trim() || res.statusText)
  }
  return res.blob()
}

// ── Emergency Recovery Key (ERK) shares ─────────────────────────────────
// Shamir's Secret Sharing over the cluster's master encryption key —
// distinct from the personal MFA recovery codes on the Account page. Only
// applies to the "local" KMS provider; cloud KMS backends manage the key
// externally.
export async function erkGenerateShares(nShares: number, threshold: number): Promise<string[]> {
  const res = await post<{ shares: string[] }>('/recovery/generate-shares', { n_shares: nShares, threshold })
  return res.shares
}

export async function erkReissueShares(shares: string[], nShares: number, threshold: number): Promise<string[]> {
  const res = await post<{ new_shares: string[] }>('/recovery/reissue-shares', { shares, n_shares: nShares, threshold })
  return res.new_shares
}

// ── MFA (TOTP) ───────────────────────────────────────────────────────────
export function getMfaStatus(): Promise<{ enabled: boolean }> {
  return get('/auth/mfa/status')
}

export function setupMfa(): Promise<{ secret: string; uri: string; qr_code: string }> {
  return post('/auth/mfa/setup')
}

export function enableMfa(code: string): Promise<{ enabled: boolean }> {
  return post('/auth/mfa/enable', { code })
}

export function disableMfa(code: string): Promise<{ enabled: boolean }> {
  return post('/auth/mfa/disable', { code })
}

// ── Personal MFA recovery codes ─────────────────────────────────────────
// Distinct from the cluster-wide ERK shares above — these are per-user
// codes to reset a password if MFA/login access is lost.
export function getRecoveryCodeStatus(): Promise<{ remaining: number }> {
  return get('/auth/recovery-codes')
}

export function regenerateRecoveryCodes(): Promise<{ recovery_codes: string[] }> {
  return post('/auth/recovery-codes/regenerate')
}

// ── Personal API tokens ──────────────────────────────────────────────────
export interface APIKeyRecord {
  id: string
  name: string
  expires_at: string | null
  created_at: string
}

export function listTokens(): Promise<APIKeyRecord[]> {
  return get('/auth/tokens')
}

export function createToken(name: string, expiresAt?: string): Promise<{ id: string; key: string }> {
  return post('/auth/keys', expiresAt ? { name, expires_at: expiresAt } : { name })
}

export function deleteToken(id: string): Promise<void> {
  return del(`/auth/tokens/${id}`)
}
