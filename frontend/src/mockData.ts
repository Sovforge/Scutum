import type { MeshNode } from './components/TopologyMap/TopologyMap'
import { positionFromMeshIp } from './lib/nodePosition'

// Shared mock data shaped like the real endpoints (see README.md § API Quick
// Reference: GET /nodes, GET /docker/containers, GET /network/mesh-summary,
// GET /kubernetes/summary, GET /audit/logs) — not wired to a live backend
// yet. Shared across Dashboard and TV mode so both show a consistent picture.

// Each node's globe position is hashed from its own WireGuard mesh IP
// (`address`, below) — deterministic and entirely local, not real
// geolocation and not a third-party lookup. See lib/nodePosition.ts: the
// mesh address is private (10.100.0.x) and carries no real-world location,
// so this only ever produces a consistent placement, never an actual one.
// Publishing where self-hosted hardware physically sits would be an
// infosec liability for this product, not a feature.
const RAW_NODES: (Omit<MeshNode, 'lat' | 'lng'> & { address: string })[] = [
  { id: 'hub-fra1', name: 'hub-fra1', role: 'hub', address: '10.100.0.1/24', status: 'healthy' },
  { id: 'edge-london', name: 'edge-london', role: 'remote', address: '10.100.0.4/24', status: 'healthy' },
  { id: 'edge-nyc', name: 'edge-nyc', role: 'remote', address: '10.100.0.7/24', status: 'degraded' },
  { id: 'build-runner', name: 'build-runner', role: 'combined', address: '10.100.0.9/24', status: 'healthy' },
  { id: 'edge-singapore', name: 'edge-singapore', role: 'remote', address: '10.100.0.14/24', status: 'healthy' },
  { id: 'edge-sydney', name: 'edge-sydney', role: 'remote', address: '10.100.0.18/24', status: 'healthy' },
]

export const NODES: (MeshNode & { address: string })[] = RAW_NODES.map((node) => ({
  ...node,
  ...positionFromMeshIp(node.address),
}))

export const CONTAINERS = [
  { name: 'scutum-agent', image: 'ghcr.io/sovforge/scutum:latest', status: 'running', cpuPct: 4, memPct: 12, mem: '128M' },
  { name: 'postgres', image: 'postgres:16-alpine', status: 'running', cpuPct: 18, memPct: 45, mem: '512M' },
  { name: 'grafana', image: 'grafana/grafana:11', status: 'running', cpuPct: 62, memPct: 71, mem: '890M' },
  { name: 'minio', image: 'minio/minio:latest', status: 'running', cpuPct: 8, memPct: 30, mem: '340M' },
  { name: 'backup-job', image: 'restic/restic:latest', status: 'exited', cpuPct: 0, memPct: 0, mem: '—' },
] as const

export const KUBERNETES = { pods: 23, deployments: 3 }

// Event names match the real webhook events documented in README.md §
// Webhook Notifications (node.enrolled, node.offline, node.online,
// healer.service_restart, audit.critical, user.created, auth.sso_login).
export const LOG_ENTRIES = [
  { time: '14:32:07', event: 'node.online', actor: 'edge-nyc', message: 'Node recovered after 42s offline', severity: 'info' },
  { time: '14:28:51', event: 'healer.service_restart', actor: 'healer', message: 'Restarted grafana after health check failure', severity: 'warning' },
  { time: '14:15:03', event: 'auth.sso_login', actor: 'j.doe@example.com', message: 'Authenticated via Microsoft Entra ID', severity: 'info' },
  { time: '13:58:44', event: 'node.enrolled', actor: 'admin', message: 'Approved build-runner into the mesh', severity: 'info' },
  { time: '13:40:12', event: 'node.offline', actor: 'edge-nyc', message: 'Missed 3 consecutive handshakes', severity: 'warning' },
  { time: '13:22:30', event: 'user.created', actor: 'admin', message: 'Created account for ops-oncall', severity: 'info' },
  { time: '12:55:18', event: 'audit.critical', actor: 'unknown', message: 'Repeated failed login attempts from 203.0.113.44', severity: 'danger' },
  { time: '12:40:02', event: 'docker.deploy-compose', actor: 'admin', message: 'Deployed stack "observability" to hub-fra1', severity: 'info' },
] as const
