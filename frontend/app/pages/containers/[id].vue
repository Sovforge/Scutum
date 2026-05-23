<template>
  <div class="container-detail">

    <div v-if="loading" class="loading-state">Loading…</div>
    <div v-else-if="notFound" class="loading-state">Container not found.</div>

    <template v-else>

    <!-- Header -->
    <div class="page-header">
      <NuxtLink to="/containers" class="back-link">
        <Icon name="lucide:arrow-left" size="14" />
        Containers
      </NuxtLink>

      <div class="page-header__title">
        <div class="container-icon">
          <Icon name="lucide:box" size="14" />
        </div>
        <h2 class="page-header__name">{{ c.name }}</h2>
        <UiBadge :variant="statusVariant(c.status)">{{ c.status }}</UiBadge>
        <span class="page-header__image">{{ c.image }}</span>
      </div>

      <div class="page-header__actions">
        <button class="action-btn" :disabled="c.status === 'running'" @click="start">
          <Icon name="lucide:play" size="13" />
          Start
        </button>
        <button class="action-btn" :disabled="c.status !== 'running'" @click="restart">
          <Icon name="lucide:rotate-cw" size="13" />
          Restart
        </button>
        <button class="action-btn action-btn--warn" :disabled="c.status !== 'running'" @click="stop">
          <Icon name="lucide:square" size="13" />
          Stop
        </button>
        <button class="action-btn action-btn--danger" @click="remove">
          <Icon name="lucide:trash-2" size="13" />
          Remove
        </button>
      </div>
    </div>

    <!-- Top info row -->
    <div class="info-row">

      <UiCard title="Details">
        <dl class="info-list">
          <div class="info-list__row"><dt>Container ID</dt><dd class="mono">{{ c.id }}</dd></div>
          <div class="info-list__row"><dt>Node</dt>
            <dd>
              <NuxtLink :to="`/nodes/${c.node}`" class="node-link">{{ c.node }}</NuxtLink>
            </dd>
          </div>
          <div class="info-list__row"><dt>Image</dt><dd class="mono">{{ c.image }}</dd></div>
          <div class="info-list__row"><dt>Image ID</dt><dd class="mono">{{ c.imageId }}</dd></div>
          <div class="info-list__row"><dt>Runtime</dt><dd>{{ c.runtime }}</dd></div>
          <div class="info-list__row"><dt>Created</dt><dd>{{ c.created }}</dd></div>
          <div class="info-list__row"><dt>Uptime</dt><dd>{{ c.uptime }}</dd></div>
          <div class="info-list__row"><dt>Restart policy</dt><dd>{{ c.restartPolicy }}</dd></div>
          <div class="info-list__row"><dt>Restarts</dt><dd>{{ c.restartCount }}</dd></div>
        </dl>
      </UiCard>

      <UiCard title="Resources">
        <template #header-right>
          <button class="stats-refresh" :disabled="statsLoading" @click="refreshStats">
            <Icon name="lucide:refresh-cw" size="13" :class="{ spin: statsLoading }" />
          </button>
        </template>
        <div class="resource-grid">
          <div class="resource-stat">
            <div class="resource-stat__label">CPU</div>
            <div class="resource-stat__value">{{ c.cpu }}</div>
            <div class="resource-bar">
              <div class="resource-bar__fill resource-bar__fill--cpu" :style="{ width: c.cpuRaw + '%' }" />
            </div>
            <div class="resource-stat__limit">Limit: {{ c.cpuLimit }}</div>
          </div>
          <div class="resource-stat">
            <div class="resource-stat__label">Memory</div>
            <div class="resource-stat__value">{{ c.memory }}</div>
            <div class="resource-bar">
              <div class="resource-bar__fill resource-bar__fill--mem" :style="{ width: c.memRaw + '%' }" />
            </div>
            <div class="resource-stat__limit">Limit: {{ c.memLimit }}</div>
          </div>
          <div class="resource-stat">
            <div class="resource-stat__label">Network Rx</div>
            <div class="resource-stat__value">{{ c.netRx }}</div>
          </div>
          <div class="resource-stat">
            <div class="resource-stat__label">Network Tx</div>
            <div class="resource-stat__value">{{ c.netTx }}</div>
          </div>
          <div class="resource-stat">
            <div class="resource-stat__label">Block Read</div>
            <div class="resource-stat__value">{{ c.blockRead }}</div>
          </div>
          <div class="resource-stat">
            <div class="resource-stat__label">Block Write</div>
            <div class="resource-stat__value">{{ c.blockWrite }}</div>
          </div>
        </div>
      </UiCard>

      <UiCard title="Network">
        <dl class="info-list">
          <div class="info-list__row"><dt>IP address</dt><dd class="mono">{{ c.ip }}</dd></div>
          <div class="info-list__row"><dt>Gateway</dt><dd class="mono">{{ c.gateway }}</dd></div>
          <div class="info-list__row"><dt>MAC</dt><dd class="mono">{{ c.mac }}</dd></div>
          <div class="info-list__row"><dt>Network mode</dt><dd>{{ c.networkMode }}</dd></div>
        </dl>

        <div v-if="c.ports.length" class="ports-section">
          <div class="ports-section__label">Port bindings</div>
          <div class="port-list">
            <div v-for="p in c.ports" :key="p.container" class="port-item">
              <span class="mono">0.0.0.0:{{ p.host }}</span>
              <Icon name="lucide:arrow-right" size="11" class="port-arrow" />
              <span class="mono">{{ p.container }}/{{ p.proto }}</span>
            </div>
          </div>
        </div>
      </UiCard>

    </div>

    <!-- Env vars + mounts row -->
    <div class="secondary-row">

      <UiCard title="Environment">
        <div class="env-list">
          <div v-for="env in c.env" :key="env.key" class="env-item">
            <span class="env-item__key">{{ env.key }}</span>
            <span class="env-item__eq">=</span>
            <span class="env-item__val">{{ env.value }}</span>
          </div>
        </div>
      </UiCard>

      <UiCard title="Mounts">
        <table class="data-table">
          <thead>
            <tr>
              <th>Type</th>
              <th>Source</th>
              <th>Destination</th>
              <th>Mode</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="m in c.mounts" :key="m.destination">
              <td><UiBadge variant="neutral">{{ m.type }}</UiBadge></td>
              <td class="mono">{{ m.source }}</td>
              <td class="mono">{{ m.destination }}</td>
              <td class="mono muted">{{ m.mode }}</td>
            </tr>
          </tbody>
        </table>
      </UiCard>

    </div>

    <!-- Logs -->
    <UiCard title="Logs">
      <template #header-right>
        <div class="log-toolbar">
          <label class="log-follow">
            <input v-model="followLogs" type="checkbox" class="log-follow__checkbox" />
            <span>Follow</span>
          </label>
          <button class="log-clear" @click="visibleLines = 50">Clear</button>
        </div>
      </template>
      <div class="log-output">
        <div v-for="(line, i) in c.logs.slice(-visibleLines)" :key="i" class="log-line">
          <span class="log-line__ts">{{ line.ts }}</span>
          <span class="log-line__stream" :class="`log-line__stream--${line.stream}`">{{ line.stream }}</span>
          <span class="log-line__msg">{{ line.msg }}</span>
        </div>
      </div>
    </UiCard>

    </template>

  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api   = useApi()
const route = useRoute()

const loading      = ref(true)
const notFound     = ref(false)
const raw          = ref<DockerContainer | null>(null)
const rawInspect   = ref<any>(null)
const rawStats     = ref<{ cpu_percent: number; mem_usage: number; mem_limit: number; net_rx: number; net_tx: number; blk_read: number; blk_write: number } | null>(null)
const statsLoading = ref(false)
const rawLogs      = ref<{ ts: string; stream: string; msg: string }[]>([])
const followLogs   = ref(true)
const visibleLines = ref(50)

function fmtBytes(n: number): string {
  if (n === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(n) / Math.log(1024))
  return (n / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1) + ' ' + units[i]
}

async function refreshStats() {
  statsLoading.value = true
  try {
    rawStats.value = await api.getContainerStats(route.params.id as string)
  } catch { /* container may not be running */ } finally {
    statsLoading.value = false
  }
}

onMounted(async () => {
  const id = route.params.id as string
  try {
    const [list, inspect, logs] = await Promise.allSettled([
      api.listContainers(),
      api.getContainerInspect(id),
      api.getContainerLogsJSON(id),
    ])
    if (list.status === 'fulfilled') {
      raw.value = list.value.find(c => c.Id === id) ?? null
    }
    if (inspect.status === 'fulfilled') rawInspect.value = inspect.value
    if (logs.status === 'fulfilled')    rawLogs.value    = logs.value
    if (!raw.value && !rawInspect.value) notFound.value = true
  } catch {
    notFound.value = true
  } finally {
    loading.value = false
  }
  // Fetch stats after the page is visible (can be slow for stopped containers)
  refreshStats()
})

const c = computed(() => {
  const r = raw.value
  const i = rawInspect.value

  const env: { key: string; value: string }[] = (i?.Config?.Env ?? []).map((e: string) => {
    const idx = e.indexOf('=')
    return idx >= 0 ? { key: e.slice(0, idx), value: e.slice(idx + 1) } : { key: e, value: '' }
  })

  const mounts: { type: string; source: string; destination: string; mode: string }[] =
    (i?.Mounts ?? []).map((m: any) => ({
      type:        m.Type        ?? 'bind',
      source:      m.Source      ?? '',
      destination: m.Destination ?? '',
      mode:        m.Mode        ?? (m.RW ? 'rw' : 'ro'),
    }))

  const ports: { host: number; container: number; proto: string }[] =
    Object.entries(i?.NetworkSettings?.Ports ?? {}).flatMap(([key, bindings]: [string, any]) => {
      const [portStr, proto] = key.split('/')
      return (bindings ?? []).map((b: any) => ({
        host:      parseInt(b.HostPort  || '0'),
        container: parseInt(String(portStr ?? '0')),
        proto:     proto ?? 'tcp',
      }))
    })

  const networks  = Object.values(i?.NetworkSettings?.Networks ?? {}) as any[]
  const net       = networks[0] ?? {}

  return {
    id:            i?.Id   ?? r?.Id   ?? '',
    name:          (i?.Name ?? r?.Names?.[0] ?? r?.Id?.slice(0, 12) ?? '').replace(/^\//, ''),
    image:         i?.Config?.Image ?? r?.Image ?? '—',
    imageId:       (i?.Image ?? '').replace(/^sha256:/, '').slice(0, 12) || '—',
    node:          'local',
    runtime:       i?.GraphDriver?.Name ?? '—',
    created:       i?.Created ? new Date(i.Created).toLocaleString() : '—',
    uptime:        r?.Status ?? i?.State?.Status ?? '—',
    status:        i?.State?.Status ?? r?.State ?? '—',
    restartPolicy: i?.HostConfig?.RestartPolicy?.Name || '—',
    restartCount:  i?.RestartCount ?? 0,
    cpu:      rawStats.value ? rawStats.value.cpu_percent.toFixed(2) + '%' : '—',
    cpuRaw:   rawStats.value ? Math.min(rawStats.value.cpu_percent, 100) : 0,
    cpuLimit: 'no limit',
    memory:   rawStats.value ? fmtBytes(rawStats.value.mem_usage) : '—',
    memRaw:   rawStats.value && rawStats.value.mem_limit > 0
                ? Math.min((rawStats.value.mem_usage / rawStats.value.mem_limit) * 100, 100)
                : 0,
    memLimit: rawStats.value && rawStats.value.mem_limit > 0
                ? fmtBytes(rawStats.value.mem_limit)
                : 'no limit',
    netRx:     rawStats.value ? fmtBytes(rawStats.value.net_rx) : '—',
    netTx:     rawStats.value ? fmtBytes(rawStats.value.net_tx) : '—',
    blockRead:  rawStats.value ? fmtBytes(rawStats.value.blk_read) : '—',
    blockWrite: rawStats.value ? fmtBytes(rawStats.value.blk_write) : '—',
    ip:          net.IPAddress || i?.NetworkSettings?.IPAddress || '—',
    gateway:     net.Gateway   || i?.NetworkSettings?.Gateway   || '—',
    mac:         net.MacAddress || '—',
    networkMode: i?.HostConfig?.NetworkMode ?? '—',
    ports: ports.length > 0 ? ports : (r?.Ports ?? [])
      .filter((p: any) => p.PublicPort)
      .map((p: any) => ({ host: p.PublicPort!, container: p.PrivatePort ?? p.PublicPort!, proto: p.Type ?? 'tcp' })),
    env,
    mounts,
    logs: rawLogs.value,
  }
})

function statusVariant(s: string) {
  if (s === 'running') return 'success' as const
  if (s === 'paused')  return 'warning' as const
  return 'neutral' as const
}

async function start()   { await api.startContainer(c.value.id); await reload() }
async function restart() { await api.restartContainer(c.value.id); await reload() }
async function stop()    { await api.stopContainer(c.value.id); await reload() }
async function remove()  { await api.removeContainer(c.value.id); navigateTo('/containers') }

async function reload() {
  const id = route.params.id as string
  const [list, inspect] = await Promise.allSettled([
    api.listContainers(),
    api.getContainerInspect(id),
  ])
  if (list.status === 'fulfilled') {
    raw.value = list.value.find(x => x.Id === id) ?? raw.value
  }
  if (inspect.status === 'fulfilled') rawInspect.value = inspect.value
}
</script>

<style scoped>
.container-detail { display: flex; flex-direction: column; gap: 1rem; }

/* Header */
.page-header { display: flex; align-items: center; gap: 1rem; flex-wrap: wrap; }
.back-link {
  display: flex; align-items: center; gap: 0.35rem;
  font-size: 0.8rem; color: var(--text-dim); text-decoration: none; transition: color 0.15s;
}
.back-link:hover { color: var(--accent-light); }

.page-header__title { display: flex; align-items: center; gap: 0.625rem; flex: 1; min-width: 0; }
.container-icon {
  width: 28px; height: 28px; border-radius: 0.375rem;
  background: var(--border); border: 1px solid var(--border-strong);
  display: flex; align-items: center; justify-content: center;
  color: var(--accent); flex-shrink: 0;
}
.page-header__name { margin: 0; font-size: 1.1rem; font-weight: 700; color: var(--text-primary); }
.page-header__image { font-family: monospace; font-size: 0.75rem; color: var(--text-dim); }

.page-header__actions { display: flex; gap: 0.5rem; margin-left: auto; }
.action-btn {
  display: flex; align-items: center; gap: 0.4rem;
  border-radius: 0.375rem; padding: 0.35rem 0.75rem;
  font-size: 0.8rem; font-family: inherit; cursor: pointer;
  background: none; border: 1px solid var(--border); color: var(--text-tertiary);
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.action-btn:not(:disabled):hover { background: var(--border); color: var(--text-primary); }
.action-btn:disabled { opacity: 0.35; cursor: not-allowed; }
.action-btn--warn  { border-color: #78350f44; color: var(--warning); }
.action-btn--warn:not(:disabled):hover  { background: #78350f22; }
.action-btn--danger { border-color: var(--danger-border); color: var(--danger-light); }
.action-btn--danger:not(:disabled):hover { background: var(--danger-bg); }

/* Layout rows */
.info-row      { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 1rem; align-items: start; }
.secondary-row { display: grid; grid-template-columns: 1fr 1.5fr;   gap: 1rem; align-items: start; }

/* Info list */
.info-list { margin: 0; display: flex; flex-direction: column; }
.info-list__row {
  display: flex; justify-content: space-between; align-items: baseline;
  padding: 0.42rem 0; border-bottom: 1px solid var(--border-subtle); gap: 1rem;
}
.info-list__row:last-child { border-bottom: none; }
dt { font-size: 0.72rem; color: var(--text-dim); white-space: nowrap; flex-shrink: 0; }
dd { margin: 0; font-size: 0.78rem; color: var(--text-secondary); text-align: right; }

/* Resources */
.resource-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
.resource-stat { display: flex; flex-direction: column; gap: 0.3rem; }
.resource-stat__label { font-size: 0.7rem; color: var(--text-dim); }
.resource-stat__value { font-size: 1rem; font-weight: 600; color: var(--text-primary); }
.resource-stat__limit { font-size: 0.68rem; color: var(--text-subtle); }
.resource-bar { height: 3px; background: var(--border); border-radius: 2px; overflow: hidden; }
.resource-bar__fill { height: 100%; border-radius: 2px; transition: width 0.4s; }
.resource-bar__fill--cpu { background: linear-gradient(90deg, var(--accent), var(--accent-light)); }
.resource-bar__fill--mem { background: linear-gradient(90deg, #0ea5e9, #38bdf8); }

/* Ports */
.ports-section { margin-top: 1rem; }
.ports-section__label { font-size: 0.7rem; color: var(--text-dim); margin-bottom: 0.5rem; }
.port-list { display: flex; flex-direction: column; gap: 0.35rem; }
.port-item { display: flex; align-items: center; gap: 0.5rem; font-size: 0.75rem; }
.port-arrow { color: var(--text-subtle); }

/* Env */
.env-list { display: flex; flex-direction: column; gap: 0.3rem; }
.env-item {
  display: flex; align-items: baseline; gap: 0.25rem;
  padding: 0.3rem 0.5rem; background: var(--bg-base); border-radius: 0.25rem;
  font-family: monospace; font-size: 0.75rem;
}
.env-item__key { color: var(--accent-light); }
.env-item__eq  { color: var(--text-dim); }
.env-item__val { color: var(--text-tertiary); word-break: break-all; }

/* Mounts table */
.data-table { width: 100%; border-collapse: collapse; font-size: 0.8rem; }
.data-table th {
  text-align: left; color: var(--text-dim); font-weight: 500;
  padding: 0 0.75rem 0.75rem; border-bottom: 1px solid var(--border);
}
.data-table td { padding: 0.6rem 0.75rem; color: var(--text-secondary); border-bottom: 1px solid var(--border-faint); }
.data-table tbody tr:last-child td { border-bottom: none; }

/* Logs */
.log-toolbar { display: flex; align-items: center; gap: 0.75rem; }
.log-follow { display: flex; align-items: center; gap: 0.4rem; font-size: 0.75rem; color: var(--text-muted); cursor: pointer; }
.log-follow__checkbox { accent-color: var(--accent); cursor: pointer; }
.log-clear {
  background: none; border: 1px solid var(--border); border-radius: 0.3rem;
  padding: 0.2rem 0.5rem; font-size: 0.72rem; color: var(--text-dim);
  cursor: pointer; font-family: inherit; transition: color 0.15s;
}
.log-clear:hover { color: var(--text-primary); }

.log-output {
  background: #050508; border-radius: 0.375rem; border: 1px solid var(--border);
  padding: 0.75rem; font-family: monospace; font-size: 0.72rem;
  max-height: 320px; overflow-y: auto;
  display: flex; flex-direction: column; gap: 0.2rem;
}
.log-line { display: flex; gap: 0.75rem; line-height: 1.5; }
.log-line__ts     { color: var(--text-subtle); white-space: nowrap; flex-shrink: 0; }
.log-line__stream { flex-shrink: 0; width: 42px; }
.log-line__stream--stdout { color: var(--text-dim); }
.log-line__stream--stderr { color: var(--danger); }
.log-line__msg    { color: var(--text-tertiary); word-break: break-all; }

/* Stats refresh */
.stats-refresh {
  background: none; border: none; color: var(--text-dim);
  cursor: pointer; padding: 0.2rem; display: flex; align-items: center;
  transition: color 0.15s;
}
.stats-refresh:hover:not(:disabled) { color: var(--accent-light); }
.stats-refresh:disabled { opacity: 0.4; cursor: not-allowed; }
.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* Misc */
.mono  { font-family: monospace; font-size: 0.75rem; color: var(--text-tertiary); }
.muted { color: var(--text-dim); }
.node-link { color: var(--accent-light); text-decoration: none; font-size: 0.78rem; }
.node-link:hover { text-decoration: underline; }
.loading-state { color: var(--text-dim); font-size: 0.875rem; padding: 2rem; text-align: center; }
</style>
