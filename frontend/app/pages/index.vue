<template>
  <div class="dashboard">

    <!-- Stat strip -->
    <div class="stat-grid">
      <div v-for="stat in stats" :key="stat.label" class="stat-card">
        <div class="stat-card__header">
          <span class="stat-card__label">{{ stat.label }}</span>
          <span class="stat-card__icon-wrap">
            <Icon :name="stat.icon" size="18" />
          </span>
        </div>
        <span class="stat-card__value">{{ stat.value }}</span>
        <div class="stat-card__sub">{{ stat.sub }}</div>
      </div>
    </div>

    <!-- Mesh Topology (full width) -->
    <UiCard title="Mesh Topology" class="topo-card">
      <template #header-right>
        <NuxtLink to="/network" class="see-all">Full view →</NuxtLink>
      </template>
      <div v-if="rawNodes.length === 0" class="empty-hint">No nodes registered.</div>
      <div v-else class="graph-wrap">
        <ClientOnly>
          <NetworkMeshGraph :nodes="graphNodes" :edges="[]" />
        </ClientOnly>
      </div>
    </UiCard>

    <!-- Bottom two-column grid -->
    <div class="main-grid">

      <!-- Left: containers -->
      <UiCard title="Containers">
        <template #header-right>
          <NuxtLink to="/containers" class="see-all">See all →</NuxtLink>
        </template>
        <div v-if="rawContainers.length === 0" class="empty-hint">No containers found.</div>
        <table v-else class="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Image</th>
              <th>Status</th>
              <th>CPU</th>
              <th>Memory</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in containers" :key="c.id">
              <td class="data-table__name">{{ c.name }}</td>
              <td class="data-table__mono">{{ c.image }}</td>
              <td>
                <UiBadge :variant="c.status === 'running' ? 'success' : 'warning'">{{ c.status }}</UiBadge>
              </td>
              <td>
                <div v-if="c.hasStats" class="mini-bar">
                  <div class="mini-bar__track">
                    <div class="mini-bar__fill"
                      :class="c.cpuPct > 80 ? 'mini-bar__fill--danger' : c.cpuPct > 50 ? 'mini-bar__fill--warn' : 'mini-bar__fill--ok'"
                      :style="{ width: c.cpuPct + '%' }" />
                  </div>
                  <span class="mini-bar__val">{{ c.cpu }}</span>
                </div>
                <span v-else class="data-table__mono">{{ c.cpu }}</span>
              </td>
              <td>
                <div v-if="c.hasStats" class="mini-bar">
                  <div class="mini-bar__track">
                    <div class="mini-bar__fill"
                      :class="c.memPct > 80 ? 'mini-bar__fill--danger' : c.memPct > 60 ? 'mini-bar__fill--warn' : 'mini-bar__fill--ok'"
                      :style="{ width: c.memPct + '%' }" />
                  </div>
                  <span class="mini-bar__val">{{ c.memory }}</span>
                </div>
                <span v-else class="data-table__mono">{{ c.memory }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </UiCard>

      <!-- Right: nodes -->
      <UiCard title="Nodes">
        <template #header-right>
          <NuxtLink to="/nodes" class="see-all">See all →</NuxtLink>
        </template>
        <div v-if="rawNodes.length === 0" class="empty-hint">No nodes registered.</div>
        <table v-else class="data-table">
          <thead>
            <tr><th>Name</th><th>Role</th><th>Address</th></tr>
          </thead>
          <tbody>
            <tr v-for="node in nodes" :key="node.id">
              <td class="data-table__name">{{ node.name }}</td>
              <td><UiBadge variant="info">{{ node.role }}</UiBadge></td>
              <td class="data-table__mono">{{ node.endpoint }}</td>
            </tr>
          </tbody>
        </table>
      </UiCard>

    </div>

  </div>
</template>

<script setup lang="ts">
import type { MeshNode } from '~/components/network/MeshGraph.vue'

definePageMeta({ layout: 'default' })

const api = useApi()

const rawNodes      = ref<NodeRecord[]>([])
const rawContainers = ref<DockerContainer[]>([])
const meshSummary   = ref({ total: 0, healthy: 0, degraded: 0 })

interface ContainerStat {
  cpu_percent: number; mem_usage: number; mem_limit: number
  net_rx: number; net_tx: number; blk_read: number; blk_write: number
}
const containerStats = ref<Record<string, ContainerStat>>({})

function fmtBytes(n: number): string {
  if (n < 1048576)    return `${(n / 1024).toFixed(0)}K`
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)}M`
  return `${(n / 1073741824).toFixed(2)}G`
}

async function refreshStats(ctrs: DockerContainer[]) {
  const running = ctrs.filter(c => c.State === 'running')
  if (running.length === 0) return
  const results = await Promise.allSettled(
    running.map(c => api.getContainerStats(c.Id).then(s => [c.Id, s] as [string, ContainerStat]))
  )
  const map: Record<string, ContainerStat> = {}
  for (const r of results) {
    if (r.status === 'fulfilled') map[r.value[0]] = r.value[1]
  }
  containerStats.value = map
}

onMounted(async () => {
  try {
    [rawNodes.value, rawContainers.value] = await Promise.all([
      api.listNodes(),
      api.listContainers(),
    ])
    refreshStats(rawContainers.value)
  } catch {}
  try { meshSummary.value = await api.getMeshSummary() } catch {}
})

const running = computed(() => rawContainers.value.filter(c => c.State === 'running').length)

const stats = computed(() => [
  {
    label:   'Nodes',
    value:   rawNodes.value.length,
    icon:    'lucide:server',
    sub:     'enrolled in mesh',
    variant: 'info' as const,
  },
  {
    label:   'Containers',
    value:   rawContainers.value.length,
    icon:    'lucide:box',
    sub:     `${running.value} running`,
    variant: 'success' as const,
  },
  {
    label:   'Running',
    value:   running.value,
    icon:    'lucide:circle-check',
    sub:     `of ${rawContainers.value.length} total`,
    variant: running.value > 0 ? 'success' as const : 'neutral' as const,
  },
  {
    label:   'Mesh Health',
    value:   meshSummary.value.total === 0 ? '—' : `${meshSummary.value.healthy}/${meshSummary.value.total}`,
    icon:    'lucide:network',
    sub:     meshSummary.value.total === 0 ? 'no peers' : `${meshSummary.value.degraded} degraded`,
    variant: (meshSummary.value.total === 0 || meshSummary.value.degraded > 0) ? 'warning' as const : 'success' as const,
  },
])

const nodes = computed(() => rawNodes.value.map(n => ({
  id:       n.id,
  name:     n.name,
  role:     n.type,
  endpoint: n.address,
})))

const graphNodes = computed<MeshNode[]>(() =>
  rawNodes.value.map(n => ({
    id:     n.id,
    label:  n.name,
    role:   n.type,
    status: (meshSummary.value.healthy === meshSummary.value.total ? 'healthy' : 'degraded') as MeshNode['status'],
  }))
)

const containers = computed(() => rawContainers.value.map(c => {
  const s = containerStats.value[c.Id]
  const isRunning = c.State === 'running'
  return {
    id:       c.Id,
    name:     (c.Names?.[0] ?? c.Id.slice(0, 12)).replace(/^\//, ''),
    image:    c.Image,
    status:   c.State,
    cpu:      s ? `${s.cpu_percent.toFixed(1)}%` : (isRunning ? '…' : '—'),
    cpuPct:   s ? Math.min(s.cpu_percent, 100) : 0,
    memory:   s ? fmtBytes(s.mem_usage) : (isRunning ? '…' : '—'),
    memPct:   s && s.mem_limit > 0 ? Math.min((s.mem_usage / s.mem_limit) * 100, 100) : 0,
    hasStats: !!s,
  }
}))
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

/* ── Stat strip ─────────────────────────────────────────────────────────── */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
}
.stat-card {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.625rem;
  padding: 1.25rem 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.stat-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.5rem;
}
.stat-card__label {
  font-size: 0.72rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 500;
}
.stat-card__icon-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 0.5rem;
  background: var(--bg-overlay);
  color: var(--accent-light);
  flex-shrink: 0;
}
.stat-card__value {
  font-size: 2rem;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1;
}
.stat-card__sub {
  font-size: 0.75rem;
  color: var(--text-dim);
  margin-top: 0.25rem;
}

/* ── Topology card ──────────────────────────────────────────────────────── */
.graph-wrap {
  height: 340px;
  margin: -1.25rem;
}

/* ── Bottom grid ────────────────────────────────────────────────────────── */
.main-grid {
  display: grid;
  grid-template-columns: 1fr 340px;
  gap: 1.25rem;
  align-items: start;
}

/* ── Data table ─────────────────────────────────────────────────────────── */
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
}
.data-table th {
  text-align: left;
  color: var(--text-dim);
  font-weight: 500;
  font-size: 0.68rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 0 0.75rem 0.625rem;
  border-bottom: 1px solid var(--border);
}
.data-table td {
  padding: 0.65rem 0.75rem;
  color: var(--text-secondary);
  border-bottom: 1px solid var(--border-faint);
}
.data-table tbody tr:last-child td { border-bottom: none; }
.data-table tbody tr:hover td { background: var(--hover-subtle); }
.data-table__name { color: var(--text-primary); font-weight: 500; }
.data-table__mono { font-family: monospace; font-size: 0.75rem; color: var(--text-tertiary); }

/* ── Mini resource bars ─────────────────────────────────────────────────── */
.mini-bar { display: flex; align-items: center; gap: 0.5rem; }
.mini-bar__track { flex: 1; height: 4px; background: var(--border); border-radius: 9999px; overflow: hidden; min-width: 40px; }
.mini-bar__fill { height: 100%; border-radius: 9999px; transition: width 0.4s; min-width: 0; }
.mini-bar__fill--ok     { background: var(--success); }
.mini-bar__fill--warn   { background: var(--warning); }
.mini-bar__fill--danger { background: var(--danger); }
.mini-bar__val { font-family: monospace; font-size: 0.72rem; color: var(--text-secondary); white-space: nowrap; min-width: 3rem; text-align: right; }

/* ── Mesh topology ──────────────────────────────────────────────────────── */
.topology-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}
.topology-svg {
  width: 100%;
  max-width: 280px;
  height: 180px;
  overflow: visible;
}
.topo-edge {
  stroke: var(--border);
  stroke-width: 1.5;
  stroke-dasharray: 4 3;
}
.topo-hub {
  fill: var(--accent);
  filter: drop-shadow(0 0 4px color-mix(in srgb, var(--accent) 40%, transparent));
}
.topo-peer--ok   { fill: var(--success); }
.topo-peer--warn { fill: var(--warning); }
.topo-label {
  font-size: 9px;
  fill: var(--text-dim);
  text-anchor: middle;
  font-family: monospace;
}
.topo-label--hub { fill: var(--accent-light); }
.topo-legend {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.7rem;
  color: var(--text-dim);
}
.topo-legend__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.topo-legend__dot--hub  { background: var(--accent); }
.topo-legend__dot--peer { background: var(--success); }
.topo-legend__dot--warn { background: var(--warning); }

.empty-hint { color: var(--text-dim); font-size: 0.82rem; padding: 0.25rem 0; text-align: center; }
.see-all { font-size: 0.75rem; color: var(--accent); text-decoration: none; }
.see-all:hover { color: var(--accent-light); }
</style>
