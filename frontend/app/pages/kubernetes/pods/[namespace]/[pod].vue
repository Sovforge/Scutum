<template>
  <div class="pod-detail">

    <div v-if="loading" class="state-msg">Loading…</div>
    <div v-else-if="error" class="state-msg state-msg--err">{{ error }}</div>

    <template v-else>

    <!-- Header -->
    <div class="page-header">
      <NuxtLink to="/kubernetes" class="back-link">
        <Icon name="lucide:arrow-left" size="14" /> Kubernetes
      </NuxtLink>

      <div class="page-header__title">
        <div class="pod-icon"><Icon name="lucide:box" size="14" /></div>
        <h2 class="page-header__name">{{ meta.name }}</h2>
        <UiBadge :variant="phaseVariant(meta.phase)">{{ meta.phase }}</UiBadge>
        <span class="page-header__ns">#{{ meta.namespace }}</span>
      </div>

      <div class="page-header__actions">
        <button class="action-btn action-btn--danger" @click="deletePod">
          <Icon name="lucide:trash-2" size="13" /> Delete
        </button>
      </div>
    </div>

    <!-- Top row: info + status + containers -->
    <div class="info-row">

      <UiCard title="Pod Details">
        <dl class="info-list">
          <div class="info-list__row"><dt>Pod IP</dt>        <dd class="mono">{{ meta.podIP }}</dd></div>
          <div class="info-list__row"><dt>Host IP</dt>       <dd class="mono">{{ meta.hostIP }}</dd></div>
          <div class="info-list__row"><dt>Node</dt>          <dd>{{ meta.nodeName }}</dd></div>
          <div class="info-list__row"><dt>Namespace</dt>     <dd><span class="ns-tag">#{{ meta.namespace }}</span></dd></div>
          <div class="info-list__row"><dt>Service account</dt><dd>{{ meta.serviceAccount }}</dd></div>
          <div class="info-list__row"><dt>UID</dt>           <dd class="mono small">{{ meta.uid }}</dd></div>
          <div class="info-list__row"><dt>Created</dt>       <dd>{{ meta.created }}</dd></div>
          <div class="info-list__row"><dt>Started</dt>       <dd>{{ meta.startTime }}</dd></div>
          <div class="info-list__row"><dt>Restart policy</dt><dd>{{ meta.restartPolicy }}</dd></div>
        </dl>
      </UiCard>

      <UiCard title="Conditions">
        <div class="condition-list">
          <div v-for="cond in conditions" :key="cond.type" class="condition-row">
            <span class="condition-dot" :class="cond.status === 'True' ? 'condition-dot--ok' : 'condition-dot--fail'" />
            <span class="condition-type">{{ cond.type }}</span>
            <span class="condition-status" :class="cond.status === 'True' ? 'val--ok' : 'val--fail'">{{ cond.status }}</span>
            <span v-if="cond.reason" class="condition-reason">{{ cond.reason }}</span>
          </div>
          <div v-if="!conditions.length" class="loading-hint">No conditions.</div>
        </div>
      </UiCard>

      <UiCard title="Labels">
        <div class="kv-list">
          <div v-for="(v, k) in meta.labels" :key="k" class="kv-row">
            <span class="kv-key">{{ k }}</span>
            <span class="kv-val">{{ v }}</span>
          </div>
          <div v-if="!Object.keys(meta.labels).length" class="loading-hint">No labels.</div>
        </div>
      </UiCard>

    </div>

    <!-- Containers -->
    <UiCard title="Containers">
      <template #header-right>
        <span class="count-label">{{ containers.length }} container{{ containers.length !== 1 ? 's' : '' }}</span>
      </template>
      <table class="data-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Image</th>
            <th>Ready</th>
            <th>State</th>
            <th>Restarts</th>
            <th>CPU req / lim</th>
            <th>Mem req / lim</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in containers" :key="c.name" class="data-table__row" @click="selectedContainer = c.name">
            <td><span class="res-name" :class="{ 'res-name--active': selectedContainer === c.name }">{{ c.name }}</span></td>
            <td class="mono">{{ c.image }}</td>
            <td>
              <span class="ready-dot" :class="c.ready ? 'ready-dot--ok' : 'ready-dot--fail'" />
              {{ c.ready ? 'Ready' : 'Not Ready' }}
            </td>
            <td><UiBadge :variant="stateVariant(c.state)">{{ c.state }}</UiBadge></td>
            <td :class="c.restarts > 0 ? 'val--warn' : ''">{{ c.restarts }}</td>
            <td class="mono">{{ c.cpuReq }} / {{ c.cpuLim }}</td>
            <td class="mono">{{ c.memReq }} / {{ c.memLim }}</td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <!-- Volumes -->
    <UiCard v-if="volumes.length" title="Volumes">
      <table class="data-table">
        <thead>
          <tr><th>Name</th><th>Type</th><th>Source</th></tr>
        </thead>
        <tbody>
          <tr v-for="v in volumes" :key="v.name">
            <td class="mono">{{ v.name }}</td>
            <td><UiBadge variant="neutral">{{ v.type }}</UiBadge></td>
            <td class="mono muted">{{ v.source }}</td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <!-- Logs -->
    <UiCard title="Logs">
      <template #header-right>
        <div class="log-toolbar">
          <select v-model="selectedContainer" class="log-select">
            <option v-for="c in containers" :key="c.name" :value="c.name">{{ c.name }}</option>
          </select>
          <button class="icon-btn" title="Refresh" @click="loadLogs">
            <Icon name="lucide:refresh-cw" size="13" :class="{ spin: logsLoading }" />
          </button>
        </div>
      </template>
      <div class="log-output" ref="logPanel">
        <div v-if="logsLoading" class="log-empty">Loading…</div>
        <template v-else-if="logLines.length">
          <div v-for="(line, i) in logLines" :key="i" class="log-line">
            <span class="log-ts">{{ fmtTs(line.ts) }}</span>
            <span class="log-msg">{{ line.msg }}</span>
          </div>
        </template>
        <div v-else class="log-empty">No log output.</div>
      </div>
    </UiCard>

    </template>
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api   = useApi()
const route = useRoute()

const ns      = route.params.namespace as string
const podName = route.params.pod as string

const loading = ref(true)
const error   = ref('')
const raw     = ref<any>(null)

const selectedContainer = ref('')
const logLines   = ref<{ ts: string; msg: string }[]>([])
const logsLoading = ref(false)
const logPanel   = ref<HTMLElement | null>(null)

onMounted(async () => {
  try {
    raw.value = await api.getK8sPod(ns, podName)
  } catch (e: any) {
    error.value = e?.data ?? e?.message ?? 'Failed to load pod.'
  } finally {
    loading.value = false
  }
  if (containers.value.length) {
    selectedContainer.value = containers.value[0].name
    loadLogs()
  }
})

watch(selectedContainer, loadLogs)

async function loadLogs() {
  if (!selectedContainer.value) return
  logsLoading.value = true
  try {
    logLines.value = await api.getK8sPodLogsJSON(ns, podName, selectedContainer.value)
  } catch {
    logLines.value = []
  } finally {
    logsLoading.value = false
    await nextTick()
    logPanel.value?.scrollTo({ top: logPanel.value.scrollHeight })
  }
}

// ── Derived data ─────────────────────────────────────────────────────────

const meta = computed(() => {
  const r = raw.value
  if (!r) return { name: podName, namespace: ns, phase: '—', podIP: '—', hostIP: '—', nodeName: '—', serviceAccount: '—', uid: '—', created: '—', startTime: '—', restartPolicy: '—', labels: {} as Record<string, string> }
  const md = r.metadata ?? {}
  const spec = r.spec ?? {}
  const status = r.status ?? {}
  return {
    name:           md.name    ?? podName,
    namespace:      md.namespace ?? ns,
    phase:          status.phase ?? '—',
    podIP:          status.podIP  ?? '—',
    hostIP:         status.hostIP ?? '—',
    nodeName:       spec.nodeName ?? '—',
    serviceAccount: spec.serviceAccountName ?? 'default',
    uid:            md.uid ?? '—',
    created:        md.creationTimestamp ? new Date(md.creationTimestamp).toLocaleString() : '—',
    startTime:      status.startTime ? new Date(status.startTime).toLocaleString() : '—',
    restartPolicy:  spec.restartPolicy ?? '—',
    labels:         md.labels ?? {},
  }
})

const conditions = computed(() => {
  return (raw.value?.status?.conditions ?? []).map((c: any) => ({
    type:   c.type,
    status: c.status,
    reason: c.reason ?? '',
  }))
})

const containers = computed(() => {
  const specContainers: any[]   = raw.value?.spec?.containers ?? []
  const statusContainers: any[] = raw.value?.status?.containerStatuses ?? []
  const statusMap = Object.fromEntries(statusContainers.map((s: any) => [s.name, s]))

  return specContainers.map((c: any) => {
    const s = statusMap[c.name] ?? {}
    const stateKeys = Object.keys(s.state ?? {})
    const state = stateKeys[0] ?? '—'
    const res = c.resources ?? {}
    return {
      name:     c.name,
      image:    c.image ?? '—',
      ready:    s.ready ?? false,
      restarts: s.restartCount ?? 0,
      state,
      cpuReq: res.requests?.cpu      ?? '—',
      cpuLim: res.limits?.cpu        ?? '—',
      memReq: res.requests?.memory   ?? '—',
      memLim: res.limits?.memory     ?? '—',
    }
  })
})

const volumes = computed(() => {
  return (raw.value?.spec?.volumes ?? []).map((v: any) => {
    const type   = Object.keys(v).find(k => k !== 'name') ?? 'unknown'
    const source = JSON.stringify(v[type] ?? {})
    return { name: v.name, type, source }
  })
})

// ── Helpers ───────────────────────────────────────────────────────────────

function phaseVariant(phase: string) {
  if (phase === 'Running')   return 'success' as const
  if (phase === 'Pending')   return 'warning' as const
  if (phase === 'Succeeded') return 'neutral' as const
  return 'danger' as const
}

function stateVariant(state: string) {
  if (state === 'running')    return 'success' as const
  if (state === 'waiting')    return 'warning' as const
  if (state === 'terminated') return 'neutral' as const
  return 'neutral' as const
}

function fmtTs(ts: string): string {
  if (!ts) return ''
  try { return new Date(ts).toLocaleTimeString() } catch { return ts }
}

async function deletePod() {
  if (!confirm(`Delete pod ${podName}?`)) return
  try {
    await api.deleteK8sPod(ns, podName)
    navigateTo('/kubernetes')
  } catch (e: any) {
    alert(e?.data ?? e?.message ?? 'Delete failed.')
  }
}
</script>

<style scoped>
.pod-detail { display: flex; flex-direction: column; gap: 1rem; }

/* Header */
.page-header { display: flex; align-items: center; gap: 1rem; flex-wrap: wrap; }
.back-link {
  display: flex; align-items: center; gap: 0.35rem;
  font-size: 0.8rem; color: var(--text-dim); text-decoration: none; transition: color 0.15s;
}
.back-link:hover { color: var(--accent-light); }
.page-header__title { display: flex; align-items: center; gap: 0.625rem; flex: 1; min-width: 0; }
.pod-icon {
  width: 28px; height: 28px; border-radius: 0.375rem;
  background: var(--border); border: 1px solid var(--border-strong);
  display: flex; align-items: center; justify-content: center;
  color: var(--accent); flex-shrink: 0;
}
.page-header__name { margin: 0; font-size: 1.1rem; font-weight: 700; color: var(--text-primary); }
.page-header__ns { font-size: 0.78rem; color: var(--text-dim); }
.page-header__actions { display: flex; gap: 0.5rem; margin-left: auto; }
.action-btn {
  display: flex; align-items: center; gap: 0.4rem;
  border-radius: 0.375rem; padding: 0.35rem 0.75rem;
  font-size: 0.8rem; font-family: inherit; cursor: pointer;
  background: none; border: 1px solid var(--border); color: var(--text-tertiary);
  transition: background 0.15s, color 0.15s;
}
.action-btn--danger { border-color: var(--danger-border); color: var(--danger-light); }
.action-btn--danger:hover { background: var(--danger-bg); }

/* Grid */
.info-row { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 1rem; align-items: start; }

/* Info list */
.info-list { margin: 0; display: flex; flex-direction: column; }
.info-list__row {
  display: flex; justify-content: space-between; align-items: baseline;
  padding: 0.42rem 0; border-bottom: 1px solid var(--border-subtle); gap: 1rem;
}
.info-list__row:last-child { border-bottom: none; }
dt { font-size: 0.72rem; color: var(--text-dim); white-space: nowrap; flex-shrink: 0; }
dd { margin: 0; font-size: 0.78rem; color: var(--text-secondary); text-align: right; }

/* Conditions */
.condition-list { display: flex; flex-direction: column; gap: 0.5rem; }
.condition-row { display: flex; align-items: center; gap: 0.5rem; font-size: 0.8rem; }
.condition-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.condition-dot--ok   { background: var(--success); }
.condition-dot--fail { background: var(--danger); }
.condition-type   { color: var(--text-primary); font-weight: 500; flex: 1; }
.condition-status { font-size: 0.72rem; font-weight: 600; }
.condition-reason { font-size: 0.7rem; color: var(--text-dim); }

/* KV list */
.kv-list { display: flex; flex-direction: column; gap: 0.3rem; }
.kv-row {
  display: flex; align-items: baseline; gap: 0.5rem;
  padding: 0.28rem 0.5rem; background: var(--bg-base); border-radius: 0.25rem;
  font-family: monospace; font-size: 0.73rem;
}
.kv-key { color: var(--accent-light); flex-shrink: 0; }
.kv-val { color: var(--text-tertiary); word-break: break-all; }

/* Namespace tag */
.ns-tag { font-family: monospace; font-size: 0.72rem; color: var(--text-dim); }

/* Table */
.data-table { width: 100%; border-collapse: collapse; font-size: 0.8rem; }
.data-table th {
  text-align: left; color: var(--text-dim); font-weight: 500;
  padding: 0 0.75rem 0.75rem; border-bottom: 1px solid var(--border);
}
.data-table td { padding: 0.6rem 0.75rem; color: var(--text-secondary); border-bottom: 1px solid var(--border-faint); }
.data-table tbody tr:last-child td { border-bottom: none; }
.data-table__row { cursor: pointer; transition: background 0.1s; }
.data-table__row:hover td { background: var(--hover-bg); }

.res-name { color: var(--text-primary); font-weight: 500; }
.res-name--active { color: var(--accent-light); }
.ready-dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%; margin-right: 0.35rem; }
.ready-dot--ok   { background: var(--success); }
.ready-dot--fail { background: var(--danger); }

/* Logs */
.log-toolbar { display: flex; align-items: center; gap: 0.5rem; }
.log-select {
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 0.3rem; padding: 0.2rem 0.5rem;
  font-size: 0.75rem; color: var(--text-primary); font-family: inherit; outline: none;
}
.icon-btn {
  background: none; border: none; color: var(--text-dim); cursor: pointer;
  padding: 0.2rem; display: flex; align-items: center; transition: color 0.15s;
}
.icon-btn:hover { color: var(--accent-light); }
.log-output {
  background: #050508; border-radius: 0.375rem; border: 1px solid var(--border);
  padding: 0.75rem; font-family: monospace; font-size: 0.72rem;
  max-height: 360px; overflow-y: auto;
  display: flex; flex-direction: column; gap: 0.2rem;
}
.log-empty { color: var(--text-subtle); font-size: 0.8rem; padding: 0.5rem 0; }
.log-line { display: flex; gap: 0.75rem; line-height: 1.5; }
.log-ts  { color: var(--text-subtle); white-space: nowrap; flex-shrink: 0; }
.log-msg { color: var(--text-tertiary); word-break: break-all; }

/* Misc */
.mono  { font-family: monospace; font-size: 0.75rem; color: var(--text-tertiary); }
.muted { color: var(--text-dim); }
.small { font-size: 0.7rem; }
.val--ok   { color: var(--success); }
.val--fail { color: var(--danger); }
.val--warn { color: var(--warning); }
.count-label { font-size: 0.72rem; color: var(--text-dim); }
.loading-hint { color: var(--text-dim); font-size: 0.8rem; padding: 0.5rem 0; }
.state-msg { color: var(--text-dim); font-size: 0.875rem; padding: 2rem; text-align: center; }
.state-msg--err { color: var(--danger-lighter); }
.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
