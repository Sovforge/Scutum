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

  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
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
