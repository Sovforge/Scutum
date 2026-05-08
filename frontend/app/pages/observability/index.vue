<template>
  <div class="obs-page">

    <!-- Tab bar -->
    <div class="tab-bar">
      <button
        v-for="t in tabs"
        :key="t.id"
        class="tab-btn"
        :class="{ 'tab-btn--active': activeTab === t.id }"
        @click="activeTab = t.id"
      >
        <Icon :name="t.icon" size="14" />
        {{ t.label }}
      </button>

      <!-- right-side controls that shift per tab -->
      <div class="tab-bar__right">
        <!-- Logs controls -->
        <template v-if="activeTab === 'logs'">
          <select v-model="logSourceMode" class="select-input">
            <option value="app">Application</option>
            <option value="k8s">Kubernetes Events</option>
            <option value="docker">Docker Container</option>
          </select>
          <select v-if="logSourceMode === 'docker'" v-model="selectedContainerId" class="select-input">
            <option value="">Select container…</option>
            <option v-for="c in containers" :key="c.Id" :value="c.Id">
              {{ c.Names[0]?.replace(/^\//, '') ?? c.Id.slice(0, 12) }}
            </option>
          </select>
          <div class="search-wrap">
            <Icon name="lucide:search" size="13" class="search-icon" />
            <input v-model="logQuery" class="search-input" placeholder="Filter logs…" />
          </div>
          <select v-model="logLevel" class="select-input">
            <option value="">All levels</option>
            <option value="error">Error</option>
            <option value="warn">Warn</option>
            <option value="info">Info</option>
            <option value="debug">Debug</option>
          </select>
          <label class="follow-label">
            <input v-model="logFollow" type="checkbox" class="follow-cb" /> Follow
          </label>
          <button class="icon-btn" title="Refresh" @click="refreshLogs">
            <Icon name="lucide:refresh-cw" size="13" :class="{ spin: logLoading }" />
          </button>
          <button class="icon-btn" title="Clear" @click="logs = []">
            <Icon name="lucide:trash-2" size="13" />
          </button>
        </template>

        <!-- Traces controls (placeholder — no data yet) -->
        <template v-if="activeTab === 'traces'" />
      </div>
    </div>

    <!-- ══════════════ LOGS ══════════════ -->
    <div v-if="activeTab === 'logs'" class="tab-content tab-content--logs">
      <div class="log-panel" ref="logPanel">
        <div
          v-for="(line, i) in filteredLogs"
          :key="i"
          class="log-line"
          :class="`log-line--${line.level}`"
        >
          <span class="log-ts">{{ formatTs(line.time) }}</span>
          <span class="log-level">{{ line.level.toUpperCase() }}</span>
          <span class="log-msg">{{ line.message }}</span>
        </div>
        <div v-if="filteredLogs.length === 0" class="log-empty">No entries match the current filter.</div>
      </div>
    </div>

    <!-- ══════════════ METRICS ══════════════ -->
    <div v-if="activeTab === 'metrics'" class="tab-content">

      <!-- Metrics sub-tab bar -->
      <div class="metrics-tab-bar">
        <button
          v-for="st in metricsTabs"
          :key="st.id"
          class="metrics-tab"
          :class="{ 'metrics-tab--active': metricsTab === st.id }"
          @click="metricsTab = st.id"
        >
          <Icon :name="st.icon" size="13" />
          {{ st.label }}
        </button>
        <div class="metrics-tab-bar__right">
          <button class="refresh-btn" :disabled="metricsLoading || k8sLoading" @click="refreshMetrics">
            <Icon name="lucide:refresh-cw" size="13" :class="{ spin: metricsLoading || k8sLoading }" /> Refresh
          </button>
        </div>
      </div>

      <!-- ── Docker sub-tab ── -->
      <template v-if="metricsTab === 'docker'">
        <div class="summary-strip">
          <div v-for="m in metricsSummary" :key="m.label" class="metric-card">
            <div class="metric-card__header">
              <Icon :name="m.icon" size="13" class="metric-card__icon" />
              <span class="metric-card__label">{{ m.label }}</span>
            </div>
            <div class="metric-card__value" :class="m.cls">{{ m.value }}</div>
            <div class="metric-card__sub">{{ m.sub }}</div>
          </div>
        </div>

        <UiCard title="Container Metrics">
          <template #header-right>
            <span class="count-label">{{ rawContainers.filter(c => c.State === 'running').length }}/{{ rawContainers.length }} running</span>
            <button class="hdr-refresh-btn" :disabled="statsLoading || metricsLoading" @click="refreshStats" title="Refresh stats">
              <Icon name="lucide:refresh-cw" size="11" :class="{ spin: statsLoading }" />
            </button>
          </template>
          <div v-if="metricsLoading" class="loading-hint">Loading…</div>
          <div v-else-if="rawContainers.length === 0" class="loading-hint">No containers found.</div>
          <table v-else class="stats-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Image</th>
                <th>State</th>
                <th>CPU</th>
                <th>Memory</th>
                <th>Net I/O</th>
                <th>Disk I/O</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in rawContainers" :key="c.Id" class="stats-table__row">
                <td class="ctr-name">{{ containerName(c) }}</td>
                <td class="ctr-img">{{ shortImage(c.Image) }}</td>
                <td><span class="ctr-state" :class="`ctr-state--${c.State}`">{{ c.State }}</span></td>
                <template v-if="stat(c.Id)">
                  <td>
                    <div class="stats-bar">
                      <div class="stats-bar-track">
                        <div class="stats-bar-fill"
                          :class="stat(c.Id).cpu_percent > 80 ? 'stats-bar-fill--danger' : stat(c.Id).cpu_percent > 50 ? 'stats-bar-fill--warn' : 'stats-bar-fill--ok'"
                          :style="{ width: Math.min(stat(c.Id).cpu_percent, 100) + '%' }"
                        />
                      </div>
                      <span class="stats-bar-val">{{ stat(c.Id).cpu_percent.toFixed(1) }}%</span>
                    </div>
                  </td>
                  <td>
                    <div class="stats-bar">
                      <div class="stats-bar-track">
                        <div class="stats-bar-fill"
                          :class="memPct(stat(c.Id)) > 80 ? 'stats-bar-fill--danger' : memPct(stat(c.Id)) > 60 ? 'stats-bar-fill--warn' : 'stats-bar-fill--ok'"
                          :style="{ width: memPct(stat(c.Id)) + '%' }"
                        />
                      </div>
                      <span class="stats-bar-val">
                        {{ fmtBytes(stat(c.Id).mem_usage) }}<span v-if="stat(c.Id).mem_limit > 0" class="stats-bar-limit"> / {{ fmtBytes(stat(c.Id).mem_limit) }}</span>
                      </span>
                    </div>
                  </td>
                  <td class="io-cell">
                    <span class="io-rx">↓ {{ fmtBytes(stat(c.Id).net_rx) }}</span>
                    <span class="io-sep">/</span>
                    <span class="io-tx">↑ {{ fmtBytes(stat(c.Id).net_tx) }}</span>
                  </td>
                  <td class="io-cell">
                    <span class="io-rx">R {{ fmtBytes(stat(c.Id).blk_read) }}</span>
                    <span class="io-sep">/</span>
                    <span class="io-tx">W {{ fmtBytes(stat(c.Id).blk_write) }}</span>
                  </td>
                </template>
                <template v-else>
                  <td colspan="4" class="stats-na">
                    {{ c.State === 'running' ? (statsLoading ? 'Loading…' : 'Stats unavailable') : '—' }}
                  </td>
                </template>
              </tr>
            </tbody>
          </table>
        </UiCard>

      </template>

      <!-- ── Kubernetes sub-tab ── -->
      <template v-if="metricsTab === 'kubernetes'">
        <div class="summary-strip">
          <div v-for="m in k8sSummaryCards" :key="m.label" class="metric-card">
            <div class="metric-card__header">
              <Icon :name="m.icon" size="13" class="metric-card__icon" />
              <span class="metric-card__label">{{ m.label }}</span>
            </div>
            <div class="metric-card__value" :class="m.cls">{{ m.value }}</div>
            <div class="metric-card__sub">{{ m.sub }}</div>
          </div>
        </div>

        <div class="k8s-row">
          <UiCard title="Pod Status">
            <template #header-right>
              <span class="count-label">{{ k8sSummary?.pods ?? 0 }} total</span>
            </template>
            <div v-if="k8sLoading" class="loading-hint">Loading…</div>
            <div v-else-if="!k8sSummary" class="loading-hint">Kubernetes not reachable.</div>
            <div v-else class="phase-bars">
              <div class="phase-row">
                <span class="phase-label">Running</span>
                <div class="phase-track">
                  <div class="phase-fill phase-fill--ok"
                    :style="{ width: k8sSummary.pods > 0 ? (k8sSummary.running / k8sSummary.pods * 100) + '%' : '0%' }" />
                </div>
                <span class="phase-val val--ok">{{ k8sSummary.running }}</span>
              </div>
              <div class="phase-row">
                <span class="phase-label">Pending</span>
                <div class="phase-track">
                  <div class="phase-fill phase-fill--warn"
                    :style="{ width: k8sSummary.pods > 0 ? (k8sSummary.pending / k8sSummary.pods * 100) + '%' : '0%' }" />
                </div>
                <span class="phase-val val--warn">{{ k8sSummary.pending }}</span>
              </div>
              <div class="phase-row">
                <span class="phase-label">Failed</span>
                <div class="phase-track">
                  <div class="phase-fill phase-fill--fail"
                    :style="{ width: k8sSummary.pods > 0 ? (k8sSummary.failed / k8sSummary.pods * 100) + '%' : '0%' }" />
                </div>
                <span class="phase-val val--fail">{{ k8sSummary.failed }}</span>
              </div>
              <div class="phase-row">
                <span class="phase-label">Succeeded</span>
                <div class="phase-track">
                  <div class="phase-fill phase-fill--muted"
                    :style="{ width: k8sSummary.pods > 0 ? (k8sSummary.succeeded / k8sSummary.pods * 100) + '%' : '0%' }" />
                </div>
                <span class="phase-val val--muted">{{ k8sSummary.succeeded }}</span>
              </div>
            </div>
          </UiCard>

          <UiCard title="Deployments">
            <template #header-right>
              <span class="count-label">{{ k8sSummary?.deployments ?? 0 }} total</span>
            </template>
            <div v-if="k8sLoading" class="loading-hint">Loading…</div>
            <div v-else-if="!k8sSummary" class="loading-hint">Kubernetes not reachable.</div>
            <div v-else class="phase-bars">
              <div class="phase-row">
                <span class="phase-label">Healthy</span>
                <div class="phase-track">
                  <div class="phase-fill phase-fill--ok"
                    :style="{ width: k8sSummary.deployments > 0 ? (k8sSummary.healthy_deploys / k8sSummary.deployments * 100) + '%' : '0%' }" />
                </div>
                <span class="phase-val val--ok">{{ k8sSummary.healthy_deploys }}</span>
              </div>
              <div class="phase-row">
                <span class="phase-label">Degraded</span>
                <div class="phase-track">
                  <div class="phase-fill phase-fill--fail"
                    :style="{ width: k8sSummary.deployments > 0 ? (k8sSummary.unhealthy_deploys / k8sSummary.deployments * 100) + '%' : '0%' }" />
                </div>
                <span class="phase-val val--fail">{{ k8sSummary.unhealthy_deploys }}</span>
              </div>
            </div>
          </UiCard>

          <UiCard title="Cluster">
            <dl class="info-list">
              <div class="info-list__row"><dt>Namespaces</dt><dd>{{ k8sSummary?.namespaces ?? '—' }}</dd></div>
              <div class="info-list__row"><dt>Nodes</dt>     <dd>{{ k8sSummary?.nodes ?? '—' }}</dd></div>
              <div class="info-list__row"><dt>Pods</dt>      <dd>{{ k8sSummary?.pods ?? '—' }}</dd></div>
              <div class="info-list__row"><dt>Deployments</dt><dd>{{ k8sSummary?.deployments ?? '—' }}</dd></div>
            </dl>
          </UiCard>
        </div>
      </template>

    </div>

    <!-- ══════════════ TRACES ══════════════ -->
    <div v-if="activeTab === 'traces'" class="tab-content">

      <UiCard title="Operation Traces">
        <template #header-right>
          <span class="count-label">{{ traces.length }} recorded</span>
          <button class="hdr-refresh-btn" :disabled="tracesLoading" @click="refreshTraces" title="Refresh">
            <Icon name="lucide:refresh-cw" size="11" :class="{ spin: tracesLoading }" />
          </button>
        </template>

        <div v-if="tracesLoading" class="loading-hint">Loading…</div>
        <div v-else-if="traces.length === 0" class="traces-empty">
          <Icon name="lucide:git-merge" size="32" class="traces-empty__icon" />
          <p class="traces-empty__title">No traces recorded yet</p>
          <p class="traces-empty__sub">Operations are traced automatically. Traces appear here once the server has processed requests.</p>
        </div>
        <table v-else class="stats-table">
          <thead>
            <tr>
              <th>Operation</th>
              <th>Status</th>
              <th>Duration</th>
              <th>Time</th>
              <th>Error</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(t, i) in traces.slice().reverse()" :key="i" class="stats-table__row">
              <td class="cell--name">{{ t.name }}</td>
              <td>
                <span class="status-badge" :class="t.status === 'ok' ? 'status-badge--ok' : 'status-badge--error'">
                  {{ t.status }}
                </span>
              </td>
              <td class="cell--mono">{{ t.duration_ms }}ms</td>
              <td class="cell--muted">{{ formatTs(t.time) }}</td>
              <td class="cell--muted trace-error">{{ t.error ?? '—' }}</td>
            </tr>
          </tbody>
        </table>
      </UiCard>

    </div>

  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()
const { getToken } = useAuth()

type TabId = 'logs' | 'metrics' | 'traces'
const tabs = [
  { id: 'logs'    as TabId, icon: 'lucide:scroll-text', label: 'Logs'    },
  { id: 'metrics' as TabId, icon: 'lucide:bar-chart-2', label: 'Metrics' },
  { id: 'traces'  as TabId, icon: 'lucide:git-merge',   label: 'Traces'  },
]
const activeTab = ref<TabId>('logs')

// ── Logs ──────────────────────────────────────────────────────────────────
interface LogLine { time: string; level: 'error' | 'warn' | 'info' | 'debug'; message: string }
type LogSourceMode = 'app' | 'k8s' | 'docker'

const logSourceMode       = ref<LogSourceMode>('app')
const logs                = ref<LogLine[]>([])
const logQuery            = ref('')
const logLevel            = ref('')
const logFollow           = ref(true)
const logLoading          = ref(false)
const logPanel            = ref<HTMLElement | null>(null)
const containers          = ref<DockerContainer[]>([])
const selectedContainerId = ref('')

function formatTs(iso: string): string {
  try { return new Date(iso).toLocaleTimeString() } catch { return iso }
}

async function loadAppLogs() {
  logLoading.value = true
  try {
    const entries = await api.listLogs()
    logs.value = entries.map(e => ({ time: e.time, level: e.level as LogLine['level'], message: e.message }))
  } catch { /* backend may not have entries yet */ } finally {
    logLoading.value = false
  }
}

let streamAbort: AbortController | null = null

async function streamLines(url: string, parseLine: (line: string) => LogLine | null) {
  if (streamAbort) { streamAbort.abort() }
  streamAbort = new AbortController()
  logLoading.value = true
  logs.value = []
  const token = getToken()
  try {
    const resp = await fetch(url, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      signal: streamAbort.signal,
    })
    if (!resp.ok) { logLoading.value = false; return }
    const reader = resp.body?.getReader()
    if (!reader) return
    logLoading.value = false
    const dec = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += dec.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        const entry = parseLine(line)
        if (entry) logs.value.push(entry)
      }
    }
  } catch (e: any) {
    if (e?.name !== 'AbortError') { /* closed */ }
  } finally {
    logLoading.value = false
  }
}

function parseK8sLine(line: string): LogLine | null {
  if (!line.trim()) return null
  try {
    const ev = JSON.parse(line)
    const obj = ev.object
    if (!obj) return null
    return {
      time:    obj.lastTimestamp || obj.firstTimestamp || new Date().toISOString(),
      level:   obj.type === 'Warning' ? 'warn' : 'info',
      message: `[${obj.reason ?? '?'}] ${obj.message ?? ''} — ${obj.involvedObject?.kind}/${obj.involvedObject?.name}`,
    }
  } catch { return null }
}

function parseDockerLine(line: string): LogLine | null {
  // Strip Docker multiplexed stream 8-byte header
  const clean = line.replace(/^[\x00-\x02][\x00]{3}[\x00-\xff]{4}/, '').trim()
  if (!clean) return null
  return { time: new Date().toISOString(), level: 'info', message: clean }
}

async function refreshLogs() {
  if (logSourceMode.value === 'app') {
    if (streamAbort) { streamAbort.abort(); streamAbort = null }
    await loadAppLogs()
  } else if (logSourceMode.value === 'k8s') {
    streamLines('/api/kubernetes/events', parseK8sLine)
  } else if (logSourceMode.value === 'docker' && selectedContainerId.value) {
    streamLines(`/api/docker/containers/${selectedContainerId.value}/logs`, parseDockerLine)
  }
}

watch(logSourceMode, () => { logs.value = []; logLevel.value = ''; logQuery.value = ''; refreshLogs() })
watch(selectedContainerId, (id) => { if (logSourceMode.value === 'docker' && id) refreshLogs() })

const filteredLogs = computed(() =>
  logs.value.filter(l => {
    if (logLevel.value && l.level !== logLevel.value) return false
    if (logQuery.value && !l.message.toLowerCase().includes(logQuery.value.toLowerCase())) return false
    return true
  })
)

watch(filteredLogs, async () => {
  if (!logFollow.value) return
  await nextTick()
  logPanel.value?.scrollTo({ top: logPanel.value.scrollHeight, behavior: 'smooth' })
})

// ── Metrics sub-tabs ──────────────────────────────────────────────────────
type MetricsTabId = 'docker' | 'kubernetes'
const metricsTabs = [
  { id: 'docker'     as MetricsTabId, icon: 'lucide:box',    label: 'Docker'     },
  { id: 'kubernetes' as MetricsTabId, icon: 'lucide:layers', label: 'Kubernetes' },
]
const metricsTab = ref<MetricsTabId>('docker')

// ── Metrics ───────────────────────────────────────────────────────────────
const rawContainers  = ref<DockerContainer[]>([])
const metricsLoading = ref(false)

interface K8sSummary {
  pods: number; running: number; pending: number; failed: number; succeeded: number
  namespaces: number; nodes: number; deployments: number
  healthy_deploys: number; unhealthy_deploys: number
}
const k8sSummary = ref<K8sSummary | null>(null)
const k8sLoading = ref(false)

interface ContainerStat {
  cpu_percent: number; mem_usage: number; mem_limit: number
  net_rx: number; net_tx: number; blk_read: number; blk_write: number
}
const containerStats = ref<Record<string, ContainerStat>>({})
const statsLoading   = ref(false)

function fmtBytes(n: number): string {
  if (n < 1024)       return `${n}B`
  if (n < 1048576)    return `${(n / 1024).toFixed(1)}K`
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)}M`
  return `${(n / 1073741824).toFixed(2)}G`
}

function memPct(s: ContainerStat): number {
  return s.mem_limit > 0 ? Math.min((s.mem_usage / s.mem_limit) * 100, 100) : 0
}

function stat(id: string): ContainerStat { return containerStats.value[id]! }

async function refreshStats() {
  const running = rawContainers.value.filter(c => c.State === 'running')
  if (running.length === 0) return
  statsLoading.value = true
  const results = await Promise.allSettled(
    running.map(c => api.getContainerStats(c.Id).then(s => [c.Id, s] as [string, ContainerStat]))
  )
  const map: Record<string, ContainerStat> = {}
  for (const r of results) {
    if (r.status === 'fulfilled') {
      const [id, stat] = r.value
      map[id] = stat
    }
  }
  containerStats.value = map
  statsLoading.value = false
}

const k8sSummaryCards = computed(() => [
  { label: 'Pods',        value: String(k8sSummary.value?.pods        ?? '—'), icon: 'lucide:box',          sub: 'total',     cls: '' },
  { label: 'Running',     value: String(k8sSummary.value?.running     ?? '—'), icon: 'lucide:circle-check', sub: 'pods',      cls: (k8sSummary.value?.running ?? 0) > 0 ? 'val--ok' : '' },
  { label: 'Deployments', value: String(k8sSummary.value?.deployments ?? '—'), icon: 'lucide:layers',       sub: 'total',     cls: '' },
  { label: 'Nodes',       value: String(k8sSummary.value?.nodes       ?? '—'), icon: 'lucide:server',       sub: 'k8s nodes', cls: '' },
])

async function refreshMetrics() {
  metricsLoading.value = true
  k8sLoading.value     = true
  try {
    const ctrs = await api.listContainers()
    rawContainers.value = ctrs
    containers.value    = ctrs
  } catch {} finally {
    metricsLoading.value = false
  }
  refreshStats()
  try {
    k8sSummary.value = await api.getK8sSummary()
  } catch {} finally {
    k8sLoading.value = false
  }
}

const metricsSummary = computed(() => {
  const running = rawContainers.value.filter(c => c.State === 'running').length
  const stopped = rawContainers.value.length - running
  return [
    { label: 'Containers', value: String(rawContainers.value.length), icon: 'lucide:box',          sub: 'total',       cls: ''                                   },
    { label: 'Running',    value: String(running),                    icon: 'lucide:circle-check', sub: 'containers',  cls: running > 0 ? 'val--ok' : ''         },
    { label: 'Stopped',    value: String(stopped),                    icon: 'lucide:circle-x',     sub: 'containers',  cls: stopped > 0 ? 'val--warn' : 'val--ok'},
  ]
})

function containerName(c: DockerContainer) {
  return c.Names?.[0]?.replace(/^\//, '') ?? c.Id.slice(0, 12)
}

function shortImage(image: string): string {
  return image.replace(/^[^/]+\/[^/]+\//, '').replace(/^[^/]+\//, '')
}

// ── Traces ────────────────────────────────────────────────────────────────
const traces        = ref<TraceEntry[]>([])
const tracesLoading = ref(false)

async function refreshTraces() {
  tracesLoading.value = true
  try { traces.value = await api.listTraces() } catch {} finally { tracesLoading.value = false }
}

watch(activeTab, tab => { if (tab === 'traces' && traces.value.length === 0) refreshTraces() })

onMounted(() => { loadAppLogs(); refreshMetrics() })
onUnmounted(() => { if (streamAbort) streamAbort.abort() })
</script>

<style scoped>
.obs-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

/* ── Tab bar ────────────────────────────────────────────────────────────── */
.tab-bar {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0 1.5rem;
  border-bottom: 1px solid var(--border);
  background: var(--bg-base);
  flex-shrink: 0;
}
.tab-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.625rem 1rem;
  font-size: 0.82rem;
  color: var(--text-muted);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
  white-space: nowrap;
}
.tab-btn:hover { color: var(--text-primary); }
.tab-btn--active { color: var(--accent-light); border-bottom-color: var(--accent); }

.tab-bar__right {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  margin-left: auto;
  padding: 0.5rem 0;
}

/* ── Tab content ────────────────────────────────────────────────────────── */
.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 1.75rem 2rem;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}
.tab-content--logs {
  padding: 0;
  overflow: hidden;
}

/* ── Toolbar inputs ─────────────────────────────────────────────────────── */
.search-wrap { position: relative; display: flex; align-items: center; }
.search-icon { position: absolute; left: 0.5rem; color: var(--text-muted); pointer-events: none; }
.search-input {
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.3rem 0.75rem 0.3rem 2rem;
  font-size: 0.78rem;
  color: var(--text-primary);
  width: 180px;
  outline: none;
}
.search-input::placeholder { color: var(--text-dim); }
.search-input:focus { border-color: var(--accent); }
.select-input {
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.3rem 0.5rem;
  font-size: 0.78rem;
  color: var(--text-primary);
  outline: none;
  cursor: pointer;
}
.select-input:focus { border-color: var(--accent); }
.follow-label { display: flex; align-items: center; gap: 0.3rem; font-size: 0.78rem; color: var(--text-tertiary); cursor: pointer; }
.follow-cb { accent-color: var(--accent); }
.icon-btn {
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.25rem;
  color: var(--text-muted);
  padding: 0.3rem 0.4rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  transition: all 0.15s;
}
.icon-btn:hover { color: var(--danger-light); border-color: #7f1d1d; }
.refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.3rem 0.625rem;
  font-size: 0.78rem;
  color: var(--text-tertiary);
  cursor: pointer;
  transition: all 0.15s;
}
.refresh-btn:hover { color: var(--text-primary); border-color: var(--border-hover); }

/* ── Log panel ──────────────────────────────────────────────────────────── */
.log-panel {
  flex: 1;
  overflow-y: auto;
  background: var(--bg-deep);
  padding: 0.75rem 0;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.78rem;
  line-height: 1.6;
}
.log-line {
  display: grid;
  grid-template-columns: 11rem 3.5rem 1fr;
  gap: 0.75rem;
  padding: 0.15rem 1.5rem;
}
.log-line:hover { background: var(--hover-bg); }
.log-ts     { color: var(--text-dim); }
.log-source { color: var(--accent); }
.log-msg    { color: var(--text-tertiary); word-break: break-word; }
.log-line--info  .log-level { color: #60a5fa; }
.log-line--debug .log-level { color: var(--text-dim); }
.log-line--warn  .log-level { color: var(--warning); }
.log-line--error .log-level { color: var(--danger-light); }
.log-line--warn  .log-msg   { color: #fcd34d; }
.log-line--error .log-msg   { color: var(--danger-lighter); }
.log-empty { text-align: center; color: var(--text-dim); padding: 2rem; font-family: inherit; }

/* ── Metrics sub-tabs ───────────────────────────────────────────────────── */
.metrics-tab-bar {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  border-bottom: 1px solid var(--border);
  margin: -1.75rem -2rem 0;
  padding: 0 2rem;
  background: var(--bg-base);
}
.metrics-tab {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.5rem 0.875rem;
  font-size: 0.8rem;
  color: var(--text-muted);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
}
.metrics-tab:hover { color: var(--text-primary); }
.metrics-tab--active { color: var(--accent-light); border-bottom-color: var(--accent); }
.metrics-tab-bar__right { margin-left: auto; }

/* ── Metrics ────────────────────────────────────────────────────────────── */
.summary-strip { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 1.25rem; }
.metric-card {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 1.25rem 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}
.metric-card__header { display: flex; align-items: center; gap: 0.4rem; margin-bottom: 0.25rem; }
.metric-card__icon   { color: var(--text-muted); }
.metric-card__label  { font-size: 0.72rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }
.metric-card__value  { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); line-height: 1; }
.metric-card__sub    { font-size: 0.72rem; color: var(--text-dim); }
.val--ok     { color: var(--success-light) !important; }
.val--danger { color: var(--danger-light) !important; }

.charts-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1.25rem; }

.chart-legend { display: flex; align-items: center; gap: 0.75rem; font-size: 0.72rem; color: var(--text-muted); }
.legend-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 0.25rem; }
.legend-dot--green  { background: var(--success); }
.legend-dot--orange { background: #f97316; }


.bar-fill { height: 100%; border-radius: 9999px; transition: width 0.4s; }
.bar-fill--ok     { background: var(--success); }
.bar-fill--warn   { background: var(--warning); }
.bar-fill--danger { background: var(--danger); }

.io-table { display: flex; flex-direction: column; gap: 0.625rem; }
.io-row { display: grid; grid-template-columns: 6rem 1fr 1fr; align-items: center; gap: 0.5rem; }
.io-val { font-size: 0.75rem; font-family: monospace; }
.io-val--rx { color: var(--success-light); }
.io-val--tx { color: #f97316; }

.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* ── Traces ─────────────────────────────────────────────────────────────── */
.traces-grid { display: grid; grid-template-columns: 1fr 380px; gap: 1.25rem; align-items: start; }
.count-label { font-size: 0.75rem; color: var(--text-muted); }

.data-table { width: 100%; border-collapse: collapse; font-size: 0.82rem; }
.data-table th {
  text-align: left;
  padding: 0.5rem 0.75rem;
  color: var(--text-muted);
  font-weight: 500;
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid var(--border);
}
.data-table td { padding: 0.6rem 0.75rem; border-bottom: 1px solid var(--border); color: var(--text-secondary); vertical-align: middle; }
.table-row { cursor: pointer; transition: background 0.12s; }
.table-row:hover { background: var(--hover-subtle); }
.table-row--selected { background: var(--accent-dim); }
.table-row:last-child td { border-bottom: none; }

.cell--mono  { font-family: monospace; font-size: 0.78rem; color: var(--text-tertiary); }
.cell--muted { color: var(--text-muted); }
.cell--id    { color: var(--accent); }
.cell--name  { color: var(--text-primary); font-weight: 500; }
.cell--ok    { color: var(--success-light); }
.cell--warn  { color: var(--warning); }

.status-badge {
  display: inline-flex;
  padding: 0.15rem 0.5rem;
  border-radius: 0.25rem;
  font-size: 0.7rem;
  font-weight: 600;
  text-transform: uppercase;
}
.status-badge--ok    { background: var(--role-dev-bg); color: var(--success-light); border: 1px solid var(--role-dev-border); }
.status-badge--error { background: var(--danger-bg); color: var(--danger-light); border: 1px solid var(--danger-border); }
.status-badge--slow  { background: var(--warning-bg); color: var(--warning); border: 1px solid var(--warning-border); }

.waterfall { margin-bottom: 1.25rem; display: flex; flex-direction: column; gap: 0.25rem; }
.waterfall__header {
  display: grid;
  grid-template-columns: 200px 1fr;
  font-size: 0.68rem;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--border);
  margin-bottom: 0.25rem;
}
.wf-timeline-label { text-align: right; padding-right: 3.5rem; }
.waterfall__row { display: grid; grid-template-columns: 200px 1fr 3.5rem; align-items: center; gap: 0.5rem; padding: 0.2rem 0; }
.wf-name { display: flex; align-items: center; gap: 0.3rem; overflow: hidden; min-width: 0; }
.wf-icon    { color: var(--text-dim); flex-shrink: 0; }
.wf-service { color: var(--accent-light); font-size: 0.72rem; white-space: nowrap; }
.wf-op      { color: var(--text-muted); font-size: 0.7rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.wf-bar-wrap { position: relative; height: 14px; background: var(--bg-overlay); border-radius: 3px; overflow: hidden; }
.wf-bar { position: absolute; top: 0; height: 100%; border-radius: 3px; min-width: 3px; }
.wf-bar--ok    { background: var(--success-glow); border: 1px solid var(--success); }
.wf-bar--slow  { background: var(--warning); border: 1px solid var(--warning); }
.wf-bar--error { background: var(--danger-glow); border: 1px solid var(--danger); }
.wf-dur { font-size: 0.7rem; color: var(--text-muted); text-align: right; white-space: nowrap; }

.span-meta { border-top: 1px solid var(--border); padding-top: 1rem; display: flex; flex-direction: column; gap: 0.5rem; }
.span-meta__row { display: flex; justify-content: space-between; align-items: center; }
.detail-label { font-size: 0.75rem; color: var(--text-muted); }
.detail-val   { font-size: 0.82rem; color: var(--text-secondary); }
.empty-hint   { color: var(--text-dim); font-size: 0.85rem; text-align: center; padding: 1rem 0; }

.traces-empty {
  display: flex; flex-direction: column; align-items: center;
  gap: 0.75rem; padding: 3rem 1rem; text-align: center;
}
.traces-empty__icon  { color: var(--border-strong); }
.traces-empty__title { margin: 0; font-size: 0.95rem; font-weight: 600; color: var(--text-secondary); }
.traces-empty__sub   { margin: 0; font-size: 0.8rem; color: var(--text-dim); max-width: 420px; line-height: 1.6; }

/* ── Kubernetes metrics ──────────────────────────────────────────────────── */
.section-heading {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding-top: 0.25rem;
}

.k8s-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 1.25rem;
}

.phase-bars { display: flex; flex-direction: column; gap: 0.75rem; }

.phase-row {
  display: grid;
  grid-template-columns: 5rem 1fr 2rem;
  align-items: center;
  gap: 0.5rem;
}

.phase-label { font-size: 0.75rem; color: var(--text-muted); }

.phase-track {
  height: 5px;
  background: var(--border);
  border-radius: 9999px;
  overflow: hidden;
}

.phase-fill { height: 100%; border-radius: 9999px; transition: width 0.4s; min-width: 0; }
.phase-fill--ok    { background: var(--success); }
.phase-fill--warn  { background: var(--warning); }
.phase-fill--fail  { background: var(--danger); }
.phase-fill--muted { background: var(--text-dim); }

.phase-val { font-size: 0.72rem; text-align: right; }

.val--warn  { color: var(--warning) !important; }
.val--fail  { color: var(--danger-light) !important; }
.val--muted { color: var(--text-dim) !important; }

.info-list { display: flex; flex-direction: column; gap: 0.5rem; margin: 0; padding: 0; }
.info-list__row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.8rem;
}
.info-list__row dt { color: var(--text-muted); font-weight: normal; }
.info-list__row dd { color: var(--text-secondary); margin: 0; font-weight: 600; }

/* ── Stats table ──────────────────────────────────────────────────────────── */
.stats-table { width: 100%; border-collapse: collapse; font-size: 0.8rem; }
.stats-table th {
  text-align: left; padding: 0 1rem 0.625rem;
  font-weight: 500; font-size: 0.68rem; text-transform: uppercase;
  letter-spacing: 0.04em; color: var(--text-dim);
  border-bottom: 1px solid var(--border);
}
.stats-table td { padding: 0.75rem 1rem; border-bottom: 1px solid var(--border-faint); vertical-align: middle; }
.stats-table tbody tr:last-child td { border-bottom: none; }
.stats-table__row:hover td { background: var(--hover-subtle); }

.stats-bar { display: flex; align-items: center; gap: 0.5rem; }
.stats-bar-track { flex: 1; height: 4px; background: var(--border); border-radius: 9999px; overflow: hidden; min-width: 50px; }
.stats-bar-fill { height: 100%; border-radius: 9999px; transition: width 0.4s; min-width: 0; }
.stats-bar-fill--ok     { background: var(--success); }
.stats-bar-fill--warn   { background: var(--warning); }
.stats-bar-fill--danger { background: var(--danger); }
.stats-bar-val   { font-family: monospace; font-size: 0.72rem; color: var(--text-secondary); white-space: nowrap; min-width: 3.5rem; text-align: right; }
.stats-bar-limit { color: var(--text-dim); }

.io-cell { white-space: nowrap; }
.io-rx  { font-family: monospace; font-size: 0.72rem; color: var(--success-light); }
.io-tx  { font-family: monospace; font-size: 0.72rem; color: #f97316; }
.io-sep { color: var(--border-strong); margin: 0 0.25rem; font-size: 0.72rem; }

.stats-na { color: var(--text-dim); font-size: 0.75rem; font-style: italic; }

.hdr-refresh-btn {
  background: none; border: 1px solid var(--border);
  border-radius: 0.25rem; padding: 0.2rem 0.35rem;
  color: var(--text-dim); cursor: pointer; display: flex; align-items: center;
  transition: all 0.15s; margin-left: 0.5rem;
}
.hdr-refresh-btn:hover:not(:disabled) { color: var(--text-primary); border-color: var(--border-strong); }
.hdr-refresh-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.loading-hint { padding: 1.5rem; text-align: center; color: var(--text-dim); font-size: 0.82rem; }
.trace-error  { font-family: monospace; font-size: 0.72rem; max-width: 20rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* ── Container / audit compact tables ────────────────────────────────────── */
.ctr-table { width: 100%; border-collapse: collapse; font-size: 0.8rem; }
.ctr-table th {
  text-align: left; padding: 0 0.75rem 0.5rem;
  font-weight: 500; font-size: 0.68rem; text-transform: uppercase;
  letter-spacing: 0.04em; color: var(--text-dim);
  border-bottom: 1px solid var(--border);
}
.ctr-table td { padding: 0.5rem 0.75rem; border-bottom: 1px solid var(--border-faint); vertical-align: middle; }
.ctr-table tbody tr:last-child td { border-bottom: none; }
.ctr-table__row:hover td { background: var(--hover-subtle); }

.ctr-name  { color: var(--text-primary); font-weight: 500; font-family: monospace; font-size: 0.75rem; white-space: nowrap; }
.ctr-img   { color: var(--text-muted); font-size: 0.75rem; max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ctr-ports { font-family: monospace; font-size: 0.72rem; color: var(--accent-light); }
.ctr-id    { font-family: monospace; font-size: 0.72rem; color: var(--text-dim); }

.ctr-state {
  display: inline-block;
  padding: 0.1rem 0.45rem; border-radius: 0.25rem;
  font-size: 0.68rem; font-weight: 600; text-transform: uppercase;
}
.ctr-state--running { background: var(--role-dev-bg); color: var(--success-light); border: 1px solid var(--role-dev-border); }
.ctr-state--exited  { background: var(--danger-bg); color: var(--danger-light); border: 1px solid #7f1d1d55; }
.ctr-state--paused  { background: var(--warning-bg); color: var(--warning); border: 1px solid var(--warning-border); }
.ctr-state--created { background: var(--bg-overlay); color: var(--text-muted); border: 1px solid var(--border); }

.method-badge {
  display: inline-block;
  padding: 0.1rem 0.35rem; border-radius: 0.2rem;
  font-size: 0.65rem; font-weight: 700; font-family: monospace;
}
.method-badge--get    { background: rgba(96,165,250,0.1); color: #60a5fa; }
.method-badge--post   { background: rgba(52,211,153,0.1); color: #34d399; }
.method-badge--put    { background: rgba(251,191,36,0.1); color: #fbbf24; }
.method-badge--delete { background: rgba(248,113,113,0.1); color: #f87171; }
.method-badge--patch  { background: rgba(167,139,250,0.1); color: #a78bfa; }

.audit-path { font-family: monospace; font-size: 0.72rem; color: var(--text-tertiary); }
.audit-action { font-size: 0.75rem; color: var(--text-secondary); }
.audit-ts { font-size: 0.72rem; color: var(--text-dim); white-space: nowrap; }
</style>
