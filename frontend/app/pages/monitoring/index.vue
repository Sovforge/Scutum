<template>
  <div class="monitoring">

    <div class="monitoring-header">
      <div class="monitoring-header__title">
        <Icon name="lucide:cpu" size="16" class="monitoring-header__icon" />
        Resource Monitoring
      </div>
      <div class="monitoring-header__actions">
        <span class="last-refresh">Updated {{ lastRefreshLabel }}</span>
        <button class="refresh-btn" :class="{ 'refresh-btn--spinning': refreshing }" @click="refresh">
          <Icon name="lucide:refresh-cw" size="13" />
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading-state">Loading stats…</div>
    <div v-else-if="!nodeStats.length" class="empty-state">
      <Icon name="lucide:cpu" size="24" class="empty-icon" />
      <p>No monitoring data yet. Stats are collected every 60 seconds.</p>
    </div>

    <div v-else class="node-grid">
      <div v-for="entry in nodeStats" :key="entry.nodeId" class="node-card">

        <!-- Card header -->
        <div class="node-card__header">
          <div class="node-card__title">
            <UiStatusDot :status="entry.error ? 'offline' : 'healthy'" />
            <span class="node-card__name">{{ entry.nodeName }}</span>
            <UiBadge variant="info">{{ entry.nodeType }}</UiBadge>
          </div>
          <span v-if="entry.stats" class="node-card__ts">{{ relativeTime(entry.stats.recorded_at) }}</span>
        </div>

        <!-- Error state -->
        <div v-if="entry.error" class="node-card__error">
          <Icon name="lucide:wifi-off" size="13" /> Unreachable
        </div>

        <!-- Gauges -->
        <div v-else-if="entry.stats" class="gauge-row">

          <div class="gauge">
            <div class="gauge__label">CPU</div>
            <div class="gauge__bar-wrap">
              <div class="gauge__bar" :class="severityClass(entry.stats.cpu_percent)" :style="{ width: entry.stats.cpu_percent + '%' }" />
            </div>
            <div class="gauge__value" :class="severityClass(entry.stats.cpu_percent)">
              {{ entry.stats.cpu_percent.toFixed(1) }}%
            </div>
          </div>

          <div class="gauge">
            <div class="gauge__label">Memory</div>
            <div class="gauge__bar-wrap">
              <div class="gauge__bar" :class="severityClass(entry.stats.mem_percent)" :style="{ width: entry.stats.mem_percent + '%' }" />
            </div>
            <div class="gauge__value" :class="severityClass(entry.stats.mem_percent)">
              {{ fmtBytes(entry.stats.mem_used) }} / {{ fmtBytes(entry.stats.mem_total) }}
            </div>
          </div>

          <div class="gauge">
            <div class="gauge__label">Disk</div>
            <div class="gauge__bar-wrap">
              <div class="gauge__bar" :class="severityClass(entry.stats.disk_percent)" :style="{ width: entry.stats.disk_percent + '%' }" />
            </div>
            <div class="gauge__value" :class="severityClass(entry.stats.disk_percent)">
              {{ fmtBytes(entry.stats.disk_used) }} / {{ fmtBytes(entry.stats.disk_total) }}
            </div>
          </div>

          <!-- Load average -->
          <div class="load-row">
            <span class="load-label">Load avg</span>
            <span class="load-vals">
              {{ entry.stats.load_1.toFixed(2) }} &nbsp;
              {{ entry.stats.load_5.toFixed(2) }} &nbsp;
              {{ entry.stats.load_15.toFixed(2) }}
            </span>
            <span class="load-periods">(1m&nbsp;5m&nbsp;15m)</span>
          </div>

          <!-- Sparkline (CPU history) -->
          <div v-if="entry.history.length > 1" class="sparkline-wrap">
            <span class="sparkline-label">CPU (last {{ entry.history.length }}m)</span>
            <svg class="sparkline" viewBox="0 0 200 40" preserveAspectRatio="none">
              <polyline
                :points="sparklinePoints(entry.history.map(h => h.cpu_percent))"
                fill="none"
                stroke="var(--accent)"
                stroke-width="1.5"
                stroke-linejoin="round"
                stroke-linecap="round"
              />
            </svg>
          </div>

        </div>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import type { SystemStats, SystemStatRecord } from '~/composables/useApi'

definePageMeta({ layout: 'default' })

const api = useApi()

interface NodeEntry {
  nodeId:   string
  nodeName: string
  nodeType: string
  stats:    SystemStats | null
  history:  SystemStatRecord[]
  error:    boolean
}

const nodeStats    = ref<NodeEntry[]>([])
const loading      = ref(true)
const refreshing   = ref(false)
const lastRefresh  = ref<Date | null>(null)

const lastRefreshLabel = computed(() => {
  if (!lastRefresh.value) return '—'
  const secs = Math.floor((Date.now() - lastRefresh.value.getTime()) / 1000)
  if (secs < 5)  return 'just now'
  if (secs < 60) return `${secs}s ago`
  return `${Math.floor(secs / 60)}m ago`
})

async function refresh() {
  refreshing.value = true
  try {
    const nodes = await api.listNodes()

    const results = await Promise.all(nodes.map(async n => {
      try {
        const [stats, history] = await Promise.all([
          api.getSystemStats(n.id),
          api.getSystemStatsHistory(n.id, 60),
        ])
        return { nodeId: n.id, nodeName: n.name, nodeType: n.type, stats, history: history ?? [], error: false }
      } catch {
        return { nodeId: n.id, nodeName: n.name, nodeType: n.type, stats: null, history: [], error: true }
      }
    }))

    nodeStats.value = results
    lastRefresh.value = new Date()
  } finally {
    refreshing.value = false
    loading.value = false
  }
}

onMounted(() => {
  refresh()
  const timer = setInterval(refresh, 30_000)
  onUnmounted(() => clearInterval(timer))
})

// ── Helpers ────────────────────────────────────────────────────────────────

function severityClass(pct: number): string {
  if (pct >= 90) return 'critical'
  if (pct >= 70) return 'warning'
  return 'ok'
}

function fmtBytes(bytes: number): string {
  if (bytes >= 1_073_741_824) return (bytes / 1_073_741_824).toFixed(1) + ' GB'
  if (bytes >= 1_048_576)     return (bytes / 1_048_576).toFixed(1) + ' MB'
  return (bytes / 1024).toFixed(0) + ' KB'
}

function relativeTime(iso: string): string {
  const secs = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (secs < 90)  return `${secs}s ago`
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`
  return `${Math.floor(secs / 3600)}h ago`
}

function sparklinePoints(values: number[]): string {
  if (!values.length) return ''
  const w = 200, h = 40, pad = 2
  const max = Math.max(...values, 1)
  return values
    .slice()
    .reverse()
    .map((v, i) => {
      const x = pad + (i / (values.length - 1)) * (w - pad * 2)
      const y = h - pad - (v / max) * (h - pad * 2)
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}
</script>

<style scoped>
.monitoring {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.monitoring-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.monitoring-header__title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}
.monitoring-header__icon { color: var(--accent); }
.monitoring-header__actions { display: flex; align-items: center; gap: 0.75rem; }
.last-refresh { font-size: 0.75rem; color: var(--text-dim); }

.refresh-btn {
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  color: var(--text-muted);
  padding: 0.3rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  transition: color 0.15s, border-color 0.15s;
}
.refresh-btn:hover { color: var(--text-primary); border-color: var(--border-hover); }
.refresh-btn--spinning svg { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.loading-state, .empty-state {
  text-align: center;
  padding: 3rem 1rem;
  color: var(--text-muted);
  font-size: 0.85rem;
}
.empty-icon { display: block; margin: 0 auto 0.75rem; opacity: 0.25; }

/* Grid */
.node-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 1rem;
}

/* Node card */
.node-card {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 1rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.node-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.node-card__title { display: flex; align-items: center; gap: 0.5rem; }
.node-card__name { font-size: 0.875rem; font-weight: 600; color: var(--text-primary); }
.node-card__ts { font-size: 0.7rem; color: var(--text-dim); }
.node-card__error {
  display: flex; align-items: center; gap: 0.4rem;
  font-size: 0.78rem; color: var(--danger-light);
  padding: 0.5rem 0;
}

/* Gauges */
.gauge-row {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.gauge { display: flex; align-items: center; gap: 0.5rem; }
.gauge__label {
  width: 3.5rem;
  font-size: 0.72rem;
  color: var(--text-dim);
  flex-shrink: 0;
}
.gauge__bar-wrap {
  flex: 1;
  height: 6px;
  background: var(--bg-elevated);
  border-radius: 3px;
  overflow: hidden;
}
.gauge__bar {
  height: 100%;
  border-radius: 3px;
  transition: width 0.4s ease;
}
.gauge__bar.ok       { background: var(--success-light, #4ade80); }
.gauge__bar.warning  { background: #f59e0b; }
.gauge__bar.critical { background: var(--danger-light, #f87171); }

.gauge__value {
  width: 8rem;
  text-align: right;
  font-size: 0.72rem;
  font-family: monospace;
  flex-shrink: 0;
}
.gauge__value.ok       { color: var(--success-light, #4ade80); }
.gauge__value.warning  { color: #f59e0b; }
.gauge__value.critical { color: var(--danger-light, #f87171); }

/* Load average */
.load-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.72rem;
}
.load-label  { width: 3.5rem; color: var(--text-dim); flex-shrink: 0; }
.load-vals   { font-family: monospace; color: var(--text-secondary); }
.load-periods { color: var(--text-subtle); }

/* Sparkline */
.sparkline-wrap {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin-top: 0.25rem;
}
.sparkline-label { font-size: 0.68rem; color: var(--text-dim); }
.sparkline {
  width: 100%;
  height: 40px;
  display: block;
}
</style>
