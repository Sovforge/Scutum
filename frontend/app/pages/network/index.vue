<template>
  <div class="network">

    <!-- Graph + sidebar layout -->
    <div class="network__body">

      <!-- Graph -->
      <UiCard class="network__graph-card">
        <template #header-right>
          <div class="graph-toolbar">
            <div class="legend">
              <span class="legend__item"><span class="legend__dot legend__dot--healthy" />Healthy</span>
              <span class="legend__item"><span class="legend__dot legend__dot--degraded" />Degraded</span>
              <span class="legend__item"><span class="legend__dot legend__dot--offline" />Offline</span>
              <span class="legend__sep" />
              <span class="legend__item"><span class="legend__line legend__line--good" />Good</span>
              <span class="legend__item"><span class="legend__line legend__line--degraded" />Degraded</span>
              <span class="legend__item"><span class="legend__line legend__line--dead" />Dead</span>
            </div>
          </div>
        </template>
        <div class="graph-wrap">
          <ClientOnly>
            <NetworkMeshGraph :nodes="graphNodes" :edges="graphEdges" />
          </ClientOnly>
        </div>
      </UiCard>

      <!-- Peer list sidebar -->
      <div class="network__peers">
        <UiCard title="Peers">
          <ul class="peer-list">
            <li
              v-for="peer in peers"
              :key="peer.id"
              class="peer-list__item"
              :class="{ 'peer-list__item--active': selected === peer.id }"
              @click="selected = peer.id"
            >
              <UiStatusDot :status="peer.status" />
              <div class="peer-list__info">
                <span class="peer-list__name">{{ peer.name }}</span>
                <span class="peer-list__meta">{{ peer.meshIp }}</span>
              </div>
              <span class="peer-list__rtt" :class="`peer-list__rtt--${peer.status}`">
                {{ peer.rtt }}
              </span>
            </li>
          </ul>
        </UiCard>

        <!-- Selected peer detail -->
        <UiCard v-if="selectedPeer" :title="selectedPeer.name">
          <dl class="info-list">
            <div class="info-list__row">
              <dt>Status</dt>
              <dd><UiBadge :variant="statusVariant(selectedPeer.status)">{{ selectedPeer.status }}</UiBadge></dd>
            </div>
            <div class="info-list__row">
              <dt>Role</dt>
              <dd><UiBadge variant="info">{{ selectedPeer.role }}</UiBadge></dd>
            </div>
            <div class="info-list__row">
              <dt>Endpoint</dt>
              <dd class="mono">{{ selectedPeer.endpoint }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Mesh IP</dt>
              <dd class="mono">{{ selectedPeer.meshIp }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Latency</dt>
              <dd class="mono">{{ selectedPeer.rtt }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Last handshake</dt>
              <dd>{{ selectedPeer.lastHandshake }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Rx</dt>
              <dd class="mono">{{ selectedPeer.rx }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Tx</dt>
              <dd class="mono">{{ selectedPeer.tx }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Public key</dt>
              <dd class="mono key">{{ selectedPeer.pubkey }}</dd>
            </div>
          </dl>
        </UiCard>
      </div>

    </div>

    <!-- Peer connections table -->
    <UiCard title="Connections">
      <table class="data-table">
        <thead>
          <tr>
            <th>From</th>
            <th>To</th>
            <th>Quality</th>
            <th>Latency</th>
            <th>Rx</th>
            <th>Tx</th>
            <th>Last handshake</th>
            <th>Allowed IPs</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="conn in connections" :key="conn.from + conn.to">
            <td class="bold">{{ conn.from }}</td>
            <td class="bold">{{ conn.to }}</td>
            <td>
              <UiBadge :variant="conn.quality === 'good' ? 'success' : conn.quality === 'degraded' ? 'warning' : 'danger'">
                {{ conn.quality }}
              </UiBadge>
            </td>
            <td class="mono">{{ conn.latency }}</td>
            <td class="mono">{{ conn.rx }}</td>
            <td class="mono">{{ conn.tx }}</td>
            <td class="muted">{{ conn.lastHandshake }}</td>
            <td class="mono">{{ conn.allowedIps }}</td>
          </tr>
        </tbody>
      </table>
    </UiCard>

  </div>
</template>

<script setup lang="ts">
import type { MeshNode, MeshEdge } from '~/components/network/MeshGraph.vue'
import type { NodeStatus } from '~/components/ui/StatusDot.vue'

definePageMeta({ layout: 'default' })

interface Connection {
  from: string; to: string; quality: 'good' | 'degraded' | 'dead'
  latency: string; rx: string; tx: string; lastHandshake: string; allowedIps: string
}

const api = useApi()

const rawNodes = ref<NodeRecord[]>([])
const selected = ref<string>('')

onMounted(async () => {
  try {
    rawNodes.value = await api.listNodes()
    selected.value = rawNodes.value[0]?.id ?? ''
  } catch {}
})

const peers = computed(() => rawNodes.value.map(n => ({
  id:            n.id,
  name:          n.name,
  role:          n.type,
  status:        'healthy' as NodeStatus,
  meshIp:        n.address,
  endpoint:      n.address,
  rtt:           '—',
  lastHandshake: '—',
  rx:            '—',
  tx:            '—',
  pubkey:        n.public_key,
})))

const selectedPeer = computed(() => peers.value.find(p => p.id === selected.value))

const graphNodes = computed<MeshNode[]>(() =>
  peers.value.map(p => ({ id: p.id, label: p.name, role: p.role, status: p.status }))
)

const graphEdges = computed<MeshEdge[]>(() => [])

const connections = computed<Connection[]>(() => [])

function statusVariant(s: NodeStatus): 'success' | 'warning' | 'danger' {
  return s === 'healthy' ? 'success' : s === 'degraded' ? 'warning' : 'danger'
}
</script>

<style scoped>
.network { display: flex; flex-direction: column; gap: 1rem; }

/* Body split */
.network__body {
  display: grid;
  grid-template-columns: 1fr 280px;
  gap: 1rem;
  align-items: start;
}
.network__graph-card { height: 100%; }
.graph-wrap { height: 420px; margin: -1.25rem; }

/* Graph toolbar */
.graph-toolbar { display: flex; align-items: center; gap: 1rem; }
.legend { display: flex; align-items: center; gap: 0.875rem; flex-wrap: wrap; }
.legend__item { display: flex; align-items: center; gap: 0.35rem; font-size: 0.72rem; color: var(--text-muted); }
.legend__dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.legend__dot--healthy  { background: var(--success); box-shadow: 0 0 5px var(--success-glow); }
.legend__dot--degraded { background: var(--warning); }
.legend__dot--offline  { background: var(--danger); }
.legend__line { width: 18px; height: 2px; border-radius: 1px; flex-shrink: 0; }
.legend__line--good     { background: var(--success); }
.legend__line--degraded { background: var(--warning); border-top: 2px dashed var(--warning); height: 0; }
.legend__line--dead     { background: var(--danger-glow); border-top: 2px dotted var(--danger); height: 0; }
.legend__sep { width: 1px; height: 14px; background: var(--border); }

/* Peer list */
.network__peers { display: flex; flex-direction: column; gap: 1rem; }
.peer-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
.peer-list__item {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.6rem 0.5rem;
  border-radius: 0.375rem;
  cursor: pointer;
  transition: background 0.1s;
  border-left: 2px solid transparent;
}
.peer-list__item:hover { background: var(--hover-bg); }
.peer-list__item--active {
  background: rgba(124, 58, 237, 0.07);
  border-left-color: var(--accent);
}
.peer-list__info { display: flex; flex-direction: column; gap: 0.1rem; flex: 1; min-width: 0; }
.peer-list__name { font-size: 0.8rem; font-weight: 500; color: var(--text-primary); }
.peer-list__meta { font-size: 0.7rem; color: var(--text-dim); font-family: monospace; }
.peer-list__rtt  { font-size: 0.72rem; font-family: monospace; flex-shrink: 0; }
.peer-list__rtt--healthy  { color: var(--success-light); }
.peer-list__rtt--degraded { color: var(--warning); }
.peer-list__rtt--offline  { color: var(--text-dim); }

/* Info list */
.info-list { margin: 0; display: flex; flex-direction: column; }
.info-list__row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  padding: 0.4rem 0;
  border-bottom: 1px solid var(--border-subtle);
  gap: 0.75rem;
}
.info-list__row:last-child { border-bottom: none; }
dt { font-size: 0.72rem; color: var(--text-dim); white-space: nowrap; flex-shrink: 0; }
dd { margin: 0; font-size: 0.75rem; color: var(--text-secondary); text-align: right; }

/* Connections table */
.data-table { width: 100%; border-collapse: collapse; font-size: 0.8rem; }
.data-table th {
  text-align: left;
  color: var(--text-dim);
  font-weight: 500;
  padding: 0 0.75rem 0.75rem;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
.data-table td { padding: 0.65rem 0.75rem; color: var(--text-secondary); border-bottom: 1px solid var(--border-faint); }
.data-table tbody tr:last-child td { border-bottom: none; }

.mono  { font-family: monospace; font-size: 0.75rem; color: var(--text-tertiary); }
.bold  { color: var(--text-primary); font-weight: 500; }
.muted { color: var(--text-dim); }
.key   { font-size: 0.68rem; word-break: break-all; text-align: right; max-width: 140px; }
</style>
