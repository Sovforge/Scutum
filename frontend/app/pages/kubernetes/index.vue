<template>
  <div class="kubernetes">

    <!-- ── Cluster header ─────────────────────────────────────────────────── -->
    <div class="cluster-header">
      <div class="cluster-header__left">
        <UiStatusDot :status="clusterStatus" />
        <span class="cluster-header__title">Kubernetes</span>
        <span class="cluster-header__subtitle">{{ reachableNodes }} / {{ nodeSummaries.length }} nodes reachable</span>
        <span v-if="loading" class="cluster-header__hint">Loading…</span>
      </div>

      <div v-if="summary" class="cluster-header__stats">
        <div class="cstat">
          <span class="cstat__val">{{ summary.pods }}</span>
          <span class="cstat__label">pods</span>
        </div>
        <div class="cstat__div" />
        <div class="cstat">
          <span class="cstat__val">{{ summary.nodes }}</span>
          <span class="cstat__label">k8s nodes</span>
        </div>
        <div class="cstat__div" />
        <div class="cstat">
          <span class="cstat__val">{{ summary.namespaces }}</span>
          <span class="cstat__label">namespaces</span>
        </div>
        <div class="cstat__div" />
        <div class="cstat">
          <span
            class="cstat__val"
            :class="summary.unhealthy_deploys > 0 ? 'cstat__val--warn' : 'cstat__val--ok'"
          >
            {{ summary.healthy_deploys }}/{{ summary.deployments }}
          </span>
          <span class="cstat__label">deploys ok</span>
        </div>
      </div>

      <div class="cluster-header__right">
        <button class="action-btn" :disabled="loading" @click="loadData">
          <Icon name="lucide:refresh-cw" size="13" :class="{ spin: loading }" />
          Refresh
        </button>
        <button class="apply-btn" @click="applyModal = true">
          <Icon name="lucide:upload" size="13" />
          Apply YAML
        </button>
      </div>
    </div>

    <!-- Apply YAML modal -->
    <div v-if="applyModal" class="modal-backdrop" @click.self="closeApplyModal">
      <div class="modal">
        <div class="modal__header">
          <h3 class="modal__title">Apply Kubernetes Manifest</h3>
          <button class="modal__close" @click="closeApplyModal"><Icon name="lucide:x" size="14" /></button>
        </div>
        <div class="modal__body">
          <div class="form-row">
            <label class="form-label">Target node</label>
            <select v-model="applyNodeId" class="form-select">
              <option value="">Local (this node)</option>
              <option v-for="n in applyNodes" :key="n.id" :value="n.id">{{ n.name }} — {{ n.address }}</option>
            </select>
          </div>
          <label class="file-label">
            <input ref="k8sFileInput" type="file" accept=".yml,.yaml" class="file-input" @change="onK8sFileChange" />
            <div class="file-drop" :class="{ 'file-drop--has': !!k8sFile }">
              <Icon :name="k8sFile ? 'lucide:file-check' : 'lucide:file-up'" size="20" />
              <span>{{ k8sFile ? k8sFile.name : 'Click to choose .yml / .yaml' }}</span>
            </div>
          </label>
          <div v-if="applyError"  class="modal__error">{{ applyError }}</div>
          <div v-if="applyOutput" class="modal__output">{{ applyOutput }}</div>
        </div>
        <div class="modal__footer">
          <button class="modal__cancel" @click="closeApplyModal">Cancel</button>
          <button class="modal__confirm" :disabled="!k8sFile || applying" @click="runApply">
            <span v-if="applying" class="btn-spinner" />
            <span v-else>Apply</span>
          </button>
        </div>
      </div>
    </div>

    <!-- ── Tab bar ──────────────────────────────────────────────────────── -->
    <div class="tabs-row">
      <div class="tabs">
        <button
          v-for="t in tabs"
          :key="t.value"
          class="tab"
          :class="{ 'tab--active': activeTab === t.value }"
          @click="activeTab = t.value"
        >
          {{ t.label }}
          <span class="tab__count">{{ t.count }}</span>
        </button>
      </div>
      <div class="tabs-row__meta">
        <span class="muted">{{ rawPods.length }} pods</span>
        <span class="sep">·</span>
        <span class="muted">{{ nodeSummaries.length }} cluster{{ nodeSummaries.length !== 1 ? 's' : '' }}</span>
      </div>
    </div>

    <!-- ── Pods ──────────────────────────────────────────────────────────── -->
    <template v-if="activeTab === 'pods'">
      <UiCard>
        <template #header-right>
          <div class="toolbar">
            <span class="toolbar__count">{{ filteredPods.length }} / {{ rawPods.length }} pods</span>
            <div class="toolbar__search">
              <Icon name="lucide:search" size="13" class="toolbar__search-icon" />
              <input v-model="search" class="toolbar__input" placeholder="Search…" />
            </div>
            <div class="ns-wrap">
              <select v-model="meshNodeFilter" class="ns-select">
                <option value="all">All clusters</option>
                <option v-for="ns in nodeSummaries" :key="ns.nodeId ?? '__local__'" :value="ns.nodeId ?? '__local__'">{{ ns.nodeName }}</option>
              </select>
            </div>
            <div class="ns-wrap">
              <select v-model="activeNs" class="ns-select">
                <option value="all">All namespaces</option>
                <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
              </select>
            </div>
            <div class="pill-group">
              <button
                v-for="f in podFilters"
                :key="f.value"
                class="pill"
                :class="{ 'pill--active': podFilter === f.value }"
                @click="podFilter = f.value"
              >{{ f.label }}</button>
            </div>
          </div>
        </template>

        <div v-if="loading" class="empty-state">
          <Icon name="lucide:loader" size="24" class="empty-state__icon spin" />
          <p>Loading pods…</p>
        </div>
        <div v-else-if="!summary" class="empty-state">
          <Icon name="lucide:wifi-off" size="28" class="empty-state__icon" />
          <p>Kubernetes cluster is not reachable.</p>
          <p class="empty-state__sub">Ensure your kubeconfig is configured and the API server is accessible.</p>
        </div>
        <table v-else class="data-table">
          <thead>
            <tr>
              <th>Cluster</th>
              <th>Pod</th>
              <th>Namespace</th>
              <th>K8s Node</th>
              <th>Phase</th>
              <th>Ready</th>
              <th>Restarts</th>
              <th>Age</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="pod in filteredPods"
              :key="pod.uid"
              class="data-table__row"
              style="cursor:pointer"
              @click="openPod(pod)"
            >
              <td>
                <span class="cluster-chip" :class="pod.meshNodeId ? 'cluster-chip--remote' : 'cluster-chip--local'">
                  {{ pod.meshNodeName }}
                </span>
              </td>
              <td>
                <div class="res-name">
                  <span class="res-dot" :class="`res-dot--${phaseColor(pod.phase)}`" />
                  <span class="res-name__text">{{ pod.name }}</span>
                </div>
              </td>
              <td><span class="ns-tag">#{{ pod.namespace }}</span></td>
              <td class="muted">{{ pod.node }}</td>
              <td><UiBadge :variant="phaseVariant(pod.phase)">{{ pod.phase }}</UiBadge></td>
              <td class="mono">{{ pod.ready }}</td>
              <td>
                <div class="restarts" :class="{ 'restarts--warn': pod.restarts > 0 }">
                  <Icon v-if="pod.restarts > 0" name="lucide:refresh-cw" size="11" />
                  {{ pod.restarts }}
                </div>
              </td>
              <td class="muted">{{ pod.age }}</td>
            </tr>
            <tr v-if="filteredPods.length === 0 && !loading">
              <td colspan="8" class="data-table__empty">No pods match your filter.</td>
            </tr>
          </tbody>
        </table>
      </UiCard>
    </template>

    <!-- ── Deployments ────────────────────────────────────────────────────── -->
    <template v-if="activeTab === 'deployments'">
      <div v-if="summary" class="deploy-summary-row">
        <div class="deploy-stat">
          <span class="deploy-stat__val">{{ summary.deployments }}</span>
          <span class="deploy-stat__label">Total</span>
        </div>
        <div class="cstat__div" style="height:32px" />
        <div class="deploy-stat deploy-stat--ok">
          <span class="deploy-stat__val">{{ summary.healthy_deploys }}</span>
          <span class="deploy-stat__label">Healthy</span>
        </div>
        <div class="cstat__div" style="height:32px" />
        <div class="deploy-stat" :class="summary.unhealthy_deploys > 0 ? 'deploy-stat--warn' : ''">
          <span class="deploy-stat__val">{{ summary.unhealthy_deploys }}</span>
          <span class="deploy-stat__label">Degraded</span>
        </div>
      </div>
      <UiCard title="Deployments">
        <div class="empty-state">
          <Icon name="lucide:layers" size="28" class="empty-state__icon" />
          <p>Detailed deployment listing is not yet available via the API.</p>
          <p class="empty-state__sub">Use <code>kubectl get deployments -A</code> to inspect deployments directly.</p>
        </div>
      </UiCard>
    </template>

    <!-- ── Services ──────────────────────────────────────────────────────── -->
    <template v-if="activeTab === 'services'">
      <UiCard title="Services">
        <div class="empty-state">
          <Icon name="lucide:network" size="28" class="empty-state__icon" />
          <p>Service listing is not yet available via the API.</p>
          <p class="empty-state__sub">Use <code>kubectl get services -A</code> to inspect cluster services.</p>
        </div>
      </UiCard>
    </template>

    <!-- ── Config ────────────────────────────────────────────────────────── -->
    <template v-if="activeTab === 'config'">
      <UiCard title="Configuration">
        <div class="empty-state">
          <Icon name="lucide:key-round" size="28" class="empty-state__icon" />
          <p>ConfigMap and Secret listing is not yet available via the API.</p>
          <p class="empty-state__sub">Use <code>kubectl get configmaps,secrets -A</code> to inspect configuration.</p>
        </div>
      </UiCard>
    </template>

  </div>
</template>

<script setup lang="ts">
import type { NodeStatus } from '~/components/ui/StatusDot.vue'

definePageMeta({ layout: 'default' })

const api        = useApi()
const nodesStore = useNodesStore()

const loading        = ref(false)
const activeTab      = ref<'pods' | 'deployments' | 'services' | 'config'>('pods')
const activeNs       = ref('all')
const meshNodeFilter = ref('all')
const podFilter      = ref<'all' | 'Running' | 'Pending' | 'Failed'>('all')
const search         = ref('')

const podFilters = [
  { label: 'All',     value: 'all'     },
  { label: 'Running', value: 'Running' },
  { label: 'Pending', value: 'Pending' },
  { label: 'Failed',  value: 'Failed'  },
] as const

// ── Data types ────────────────────────────────────────────────────────────
interface K8sSummary {
  pods: number; running: number; pending: number; failed: number; succeeded: number
  namespaces: number; nodes: number; deployments: number
  healthy_deploys: number; unhealthy_deploys: number
}

interface NodeSummary {
  nodeId:   string | null   // null = local hub
  nodeName: string
  summary:  K8sSummary | null
}

interface PodRow {
  uid:          string
  name:         string
  namespace:    string
  node:         string   // k8s node name within the cluster
  phase:        string
  ready:        string
  restarts:     number
  age:          string
  meshNodeId:   string | null  // null = local
  meshNodeName: string
}

// ── State ─────────────────────────────────────────────────────────────────
const nodeSummaries = ref<NodeSummary[]>([])
const rawPods       = ref<PodRow[]>([])

const summary = computed<K8sSummary | null>(() => {
  const valid = nodeSummaries.value.map(n => n.summary).filter(Boolean) as K8sSummary[]
  if (valid.length === 0) return null
  return {
    pods:             valid.reduce((a, s) => a + s.pods,             0),
    running:          valid.reduce((a, s) => a + s.running,          0),
    pending:          valid.reduce((a, s) => a + s.pending,          0),
    failed:           valid.reduce((a, s) => a + s.failed,           0),
    succeeded:        valid.reduce((a, s) => a + s.succeeded,        0),
    namespaces:       valid.reduce((a, s) => a + s.namespaces,       0),
    nodes:            valid.reduce((a, s) => a + s.nodes,            0),
    deployments:      valid.reduce((a, s) => a + s.deployments,      0),
    healthy_deploys:  valid.reduce((a, s) => a + s.healthy_deploys,  0),
    unhealthy_deploys:valid.reduce((a, s) => a + s.unhealthy_deploys,0),
  }
})

const reachableNodes = computed(() => nodeSummaries.value.filter(n => n.summary !== null).length)

const clusterStatus = computed((): NodeStatus => {
  if (loading.value)           return 'degraded'
  if (reachableNodes.value > 0) return 'healthy'
  return 'offline'
})

const namespaces = computed(() => [...new Set(rawPods.value.map(p => p.namespace))].sort())

// ── Helpers ───────────────────────────────────────────────────────────────
function parseAge(ts: string): string {
  const diff = Date.now() - new Date(ts).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h`
  return `${Math.floor(hrs / 24)}d`
}

function parsePods(items: any[], meshNodeId: string | null, meshNodeName: string): PodRow[] {
  return items.map((item: any) => {
    const statuses   = item.status?.containerStatuses ?? []
    const specCtrs   = item.spec?.containers ?? []
    const readyCount = statuses.filter((s: any) => s.ready).length
    const total      = statuses.length || specCtrs.length || 1
    const restarts   = statuses.reduce((acc: number, s: any) => acc + (s.restartCount ?? 0), 0)
    return {
      uid:          (meshNodeId ?? 'local') + '/' + (item.metadata?.uid ?? item.metadata?.name),
      name:         item.metadata?.name ?? '',
      namespace:    item.metadata?.namespace ?? 'default',
      node:         item.spec?.nodeName ?? '—',
      phase:        item.status?.phase ?? 'Unknown',
      ready:        `${readyCount}/${total}`,
      restarts,
      age:          parseAge(item.metadata?.creationTimestamp ?? new Date().toISOString()),
      meshNodeId,
      meshNodeName,
    }
  })
}

// ── Load data from all mesh nodes ──────────────────────────────────────────
async function loadData() {
  loading.value   = true
  rawPods.value   = []
  nodeSummaries.value = []
  try {
    const nodes = await api.listNodes().catch(() => [] as NodeRecord[])
    const allNodes = [
      { id: null as string | null, name: 'Local' },
      ...nodes.filter(n => n.type !== 'hub').map(n => ({ id: n.id, name: n.name })),
    ]

    await Promise.all(allNodes.map(async n => {
      const [summ, podsResp] = await Promise.all([
        api.getK8sSummary(n.id).catch(() => null),
        api.listAllK8sPods(n.id).catch(() => null),
      ])
      nodeSummaries.value.push({ nodeId: n.id, nodeName: n.name, summary: summ })
      if (podsResp?.items) {
        rawPods.value.push(...parsePods(podsResp.items, n.id, n.name))
      }
    }))
  } finally {
    loading.value = false
  }
}

onMounted(loadData)

// ── Filtering ─────────────────────────────────────────────────────────────
const filteredPods = computed(() =>
  rawPods.value.filter(p => {
    if (meshNodeFilter.value !== 'all') {
      const target = meshNodeFilter.value === '__local__' ? null : meshNodeFilter.value
      if (p.meshNodeId !== target) return false
    }
    if (activeNs.value !== 'all' && p.namespace !== activeNs.value) return false
    if (podFilter.value !== 'all' && p.phase !== podFilter.value) return false
    if (search.value) {
      const q = search.value.toLowerCase()
      return p.name.includes(q) || p.namespace.includes(q) || p.node.includes(q) || p.meshNodeName.toLowerCase().includes(q)
    }
    return true
  })
)

type TabValue = 'pods' | 'deployments' | 'services' | 'config'
const tabs = computed((): Array<{ label: string; value: TabValue; count: number }> => [
  { label: 'Pods',        value: 'pods',        count: rawPods.value.length },
  { label: 'Deployments', value: 'deployments', count: summary.value?.deployments ?? 0 },
  { label: 'Services',    value: 'services',    count: 0 },
  { label: 'Config',      value: 'config',      count: 0 },
])

// ── Navigation ────────────────────────────────────────────────────────────
function openPod(pod: PodRow) {
  nodesStore.select(pod.meshNodeId)
  navigateTo(`/kubernetes/pods/${pod.namespace}/${pod.name}`)
}

function phaseVariant(phase: string) {
  if (phase === 'Running')   return 'success' as const
  if (phase === 'Pending')   return 'warning' as const
  if (phase === 'Succeeded') return 'neutral' as const
  return 'danger' as const
}

function phaseColor(phase: string) {
  if (phase === 'Running')   return 'healthy'
  if (phase === 'Pending')   return 'degraded'
  if (phase === 'Succeeded') return 'degraded'
  return 'offline'
}

// ── Apply YAML modal ──────────────────────────────────────────────────────
const applyModal   = ref(false)
const applyNodeId  = ref('')
const applyNodes   = ref<NodeRecord[]>([])
const k8sFile      = ref<File | null>(null)
const applying     = ref(false)
const applyError   = ref('')
const applyOutput  = ref('')
const k8sFileInput = ref<HTMLInputElement | null>(null)

watch(applyModal, async (open) => {
  if (open && applyNodes.value.length === 0) {
    try { applyNodes.value = await api.listNodes() } catch {}
  }
})

function onK8sFileChange(e: Event) {
  k8sFile.value     = (e.target as HTMLInputElement).files?.[0] ?? null
  applyError.value  = ''
  applyOutput.value = ''
}

function closeApplyModal() {
  applyModal.value  = false
  applyNodeId.value = ''
  k8sFile.value     = null
  applyError.value  = ''
  applyOutput.value = ''
  applying.value    = false
  if (k8sFileInput.value) k8sFileInput.value.value = ''
}

async function runApply() {
  if (!k8sFile.value) return
  applying.value    = true
  applyError.value  = ''
  applyOutput.value = ''
  try {
    const text = await k8sFile.value.text()
    const res  = await api.applyK8s(text, applyNodeId.value || undefined)
    applyOutput.value = res.output ?? 'Applied.'
  } catch (e: any) {
    applyError.value = e?.data?.error ?? e?.data ?? e?.message ?? 'Apply failed.'
  } finally {
    applying.value = false
  }
}
</script>

<style scoped>
.kubernetes { display: flex; flex-direction: column; gap: 1rem; }

/* ── Cluster header ───────────────────────────────────────────────────── */
.cluster-header {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 0.875rem 1.25rem;
  flex-wrap: wrap;
}

.cluster-header__left {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cluster-header__title {
  font-weight: 600;
  font-size: 0.9rem;
  color: var(--text-primary);
}

.cluster-header__hint {
  font-size: 0.75rem;
  color: var(--text-dim);
}

.cluster-header__hint--warn {
  color: var(--warning);
}

.cluster-header__stats {
  display: flex;
  align-items: center;
  gap: 0.875rem;
  padding-left: 1rem;
  border-left: 1px solid var(--border);
}

.cluster-header__right {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-left: auto;
}

.cstat { display: flex; flex-direction: column; gap: 0.1rem; }
.cstat__val { font-size: 1.1rem; font-weight: 700; color: var(--text-primary); line-height: 1; }
.cstat__val--warn { color: var(--warning); }
.cstat__val--ok   { color: var(--success-light); }
.cstat__label { font-size: 0.65rem; color: var(--text-dim); text-transform: uppercase; letter-spacing: 0.04em; }
.cstat__div { width: 1px; height: 28px; background: var(--border); }

.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.35rem 0.75rem;
  font-size: 0.78rem;
  color: var(--text-tertiary);
  cursor: pointer;
  font-family: inherit;
  transition: all 0.15s;
}
.action-btn:hover:not(:disabled) { color: var(--text-primary); border-color: var(--border-hover); }
.action-btn:disabled { opacity: 0.5; cursor: not-allowed; }

/* ── Tabs row ─────────────────────────────────────────────────────────── */
.tabs-row {
  display: flex; align-items: flex-end; justify-content: space-between;
  border-bottom: 1px solid var(--border); gap: 1rem;
}
.tabs { display: flex; gap: 0; }
.tab {
  display: flex; align-items: center; gap: 0.5rem;
  padding: 0.6rem 1rem; background: none; border: none;
  border-bottom: 2px solid transparent; margin-bottom: -1px;
  font-size: 0.8rem; font-family: inherit; color: var(--text-muted);
  cursor: pointer; transition: color 0.15s, border-color 0.15s; white-space: nowrap;
}
.tab:hover { color: var(--text-primary); }
.tab--active { color: var(--accent-light); border-bottom-color: var(--accent); }
.tab__count {
  background: var(--border); border-radius: 999px;
  padding: 0.1rem 0.45rem; font-size: 0.68rem; color: var(--text-muted);
}
.tab--active .tab__count { background: rgba(124,58,237,0.18); color: var(--accent-light); }
.tabs-row__meta { display: flex; align-items: center; gap: 0.5rem; padding-bottom: 0.75rem; white-space: nowrap; }

/* ── Toolbar ──────────────────────────────────────────────────────────── */
.toolbar { display: flex; align-items: center; gap: 0.625rem; flex-wrap: wrap; }
.toolbar__count { font-size: 0.72rem; color: var(--text-dim); white-space: nowrap; }
.toolbar__search { position: relative; display: flex; align-items: center; }
.toolbar__search-icon { position: absolute; left: 0.55rem; color: var(--text-dim); pointer-events: none; }
.toolbar__input {
  background: var(--bg-base); border: 1px solid var(--border); border-radius: 0.375rem;
  padding: 0.3rem 0.75rem 0.3rem 1.8rem; color: var(--text-primary);
  font-size: 0.78rem; font-family: inherit; width: 160px; outline: none;
  transition: border-color 0.15s;
}
.toolbar__input:focus { border-color: var(--accent); }
.toolbar__input::placeholder { color: var(--border-strong); }

.ns-wrap { display: flex; }
.ns-select {
  background: var(--bg-base); border: 1px solid var(--border); border-radius: 0.375rem;
  padding: 0.3rem 0.6rem; color: var(--text-tertiary); font-size: 0.75rem;
  font-family: inherit; outline: none; cursor: pointer;
  transition: border-color 0.15s; appearance: none;
}
.ns-select:focus { border-color: var(--accent); }

.pill-group { display: flex; gap: 0.2rem; }
.pill {
  background: none; border: 1px solid var(--border); border-radius: 999px;
  padding: 0.2rem 0.6rem; font-size: 0.72rem; color: var(--text-muted);
  cursor: pointer; font-family: inherit;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}
.pill:hover { color: var(--text-primary); border-color: var(--text-subtle); }
.pill--active { color: var(--accent-light); border-color: var(--accent); background: rgba(124,58,237,0.08); }

/* ── Table ────────────────────────────────────────────────────────────── */
.data-table { width: 100%; border-collapse: collapse; font-size: 0.8rem; }
.data-table th {
  text-align: left; color: var(--text-dim); font-weight: 500;
  padding: 0 0.75rem 0.75rem; border-bottom: 1px solid var(--border); white-space: nowrap;
}
.data-table td { padding: 0.62rem 0.75rem; color: var(--text-secondary); border-bottom: 1px solid var(--border-faint); }
.data-table tbody tr:last-child td { border-bottom: none; }
.data-table__row { transition: background 0.1s; }
.data-table__row:hover td { background: var(--hover-subtle); }
.data-table__empty { text-align: center; color: var(--text-subtle); padding: 2.5rem !important; }

/* ── Resource name ────────────────────────────────────────────────────── */
.res-name { display: flex; align-items: center; gap: 0.5rem; }
.res-name__text { color: var(--text-primary); font-weight: 500; font-family: monospace; font-size: 0.75rem; }
.res-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.res-dot--healthy  { background: var(--success); box-shadow: 0 0 5px var(--success-glow); }
.res-dot--degraded { background: var(--warning); }
.res-dot--offline  { background: var(--danger); }

/* ── Namespace tag ────────────────────────────────────────────────────── */
.ns-tag { font-family: monospace; font-size: 0.72rem; color: var(--accent); }

/* ── Restarts ─────────────────────────────────────────────────────────── */
.restarts { display: flex; align-items: center; gap: 0.3rem; font-family: monospace; font-size: 0.75rem; color: var(--text-dim); }
.restarts--warn { color: var(--warning); }

/* ── Empty state ──────────────────────────────────────────────────────── */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 2.5rem 1rem;
  text-align: center;
}
.empty-state__icon { color: var(--border-strong); margin-bottom: 0.25rem; }
.empty-state p { margin: 0; font-size: 0.85rem; color: var(--text-muted); }
.empty-state__sub { font-size: 0.75rem !important; color: var(--text-dim) !important; }
.empty-state__sub code { font-family: monospace; color: var(--accent-light); }

/* ── Deployments summary ──────────────────────────────────────────────── */
.deploy-summary-row {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  padding: 0.75rem 1.25rem;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
}
.deploy-stat { display: flex; flex-direction: column; gap: 0.1rem; }
.deploy-stat__val { font-size: 1.4rem; font-weight: 700; color: var(--text-primary); line-height: 1; }
.deploy-stat__label { font-size: 0.65rem; color: var(--text-dim); text-transform: uppercase; letter-spacing: 0.04em; }
.deploy-stat--ok   .deploy-stat__val { color: var(--success-light); }
.deploy-stat--warn .deploy-stat__val { color: var(--warning); }

.mono  { font-family: monospace; font-size: 0.75rem; }
.muted { color: var(--text-dim); }
.sep   { color: var(--border-strong); }

.cluster-header__subtitle {
  font-size: 0.72rem;
  color: var(--text-dim);
  margin-left: 0.25rem;
}

.cluster-chip {
  display: inline-block;
  padding: 0.1rem 0.45rem;
  border-radius: 0.25rem;
  font-size: 0.68rem;
  font-weight: 600;
  white-space: nowrap;
}
.cluster-chip--local  { background: rgba(96,165,250,0.1);  color: #60a5fa; border: 1px solid rgba(96,165,250,0.3); }
.cluster-chip--remote { background: rgba(167,139,250,0.1); color: #a78bfa; border: 1px solid rgba(167,139,250,0.3); }

/* ── Apply bar ──────────────────────────────────────────────────────────── */
.apply-btn {
  display: flex; align-items: center; gap: 0.4rem;
  background: var(--accent); color: #fff; border: none; border-radius: 0.375rem;
  padding: 0.35rem 0.75rem; font-size: 0.78rem; font-weight: 600;
  font-family: inherit; cursor: pointer; transition: background 0.15s;
}
.apply-btn:hover { background: var(--accent-hover); }

/* ── Modal ──────────────────────────────────────────────────────────────── */
.modal-backdrop {
  position: fixed; inset: 0; background: rgba(0,0,0,0.6);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.modal {
  background: var(--bg-surface); border: 1px solid var(--border);
  border-radius: 0.625rem; width: 480px; max-width: 95vw;
  display: flex; flex-direction: column;
  box-shadow: 0 24px 64px rgba(0,0,0,0.5);
}
.modal__header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1rem 1.25rem; border-bottom: 1px solid var(--border);
}
.modal__title { margin: 0; font-size: 0.9rem; font-weight: 700; color: var(--text-primary); }
.modal__close {
  background: none; border: none; color: var(--text-dim);
  cursor: pointer; padding: 0.2rem; display: flex; align-items: center;
}
.modal__close:hover { color: var(--text-primary); }
.modal__body { padding: 1.25rem; display: flex; flex-direction: column; gap: 0.875rem; }
.modal__desc { margin: 0; font-size: 0.8rem; color: var(--text-muted); }
.modal__desc code { font-family: monospace; color: var(--accent-light); }
.form-row { display: flex; flex-direction: column; gap: 0.3rem; }
.form-label { font-size: 0.75rem; color: var(--text-muted); font-weight: 500; }
.form-select {
  background: var(--bg-overlay); border: 1px solid var(--border-strong);
  border-radius: 0.375rem; padding: 0.4rem 0.625rem;
  font-size: 0.8rem; color: var(--text-primary); outline: none; cursor: pointer; width: 100%;
}
.form-select:focus { border-color: var(--accent); }

.file-input { display: none; }
.file-drop {
  border: 2px dashed var(--border-strong); border-radius: 0.5rem;
  padding: 1.5rem; display: flex; flex-direction: column; align-items: center;
  gap: 0.5rem; color: var(--text-dim); font-size: 0.8rem; cursor: pointer;
  transition: border-color 0.15s, color 0.15s;
}
.file-drop:hover, .file-drop--has { border-color: var(--accent); color: var(--accent-light); }
.modal__error {
  background: var(--danger-bg); border: 1px solid #7f1d1d55;
  border-radius: 0.375rem; padding: 0.6rem 0.875rem;
  font-size: 0.78rem; color: var(--danger-lighter);
}
.modal__output {
  background: var(--bg-base); border: 1px solid var(--border);
  border-radius: 0.375rem; padding: 0.6rem 0.875rem;
  font-family: monospace; font-size: 0.72rem; color: var(--text-tertiary);
  white-space: pre-wrap; max-height: 140px; overflow-y: auto;
}
.modal__footer {
  display: flex; justify-content: flex-end; gap: 0.5rem;
  padding: 1rem 1.25rem; border-top: 1px solid var(--border);
}
.modal__cancel {
  background: none; border: 1px solid var(--border); border-radius: 0.375rem;
  padding: 0.4rem 0.875rem; font-size: 0.8rem; color: var(--text-muted);
  cursor: pointer; font-family: inherit;
}
.modal__cancel:hover { background: var(--hover-bg); color: var(--text-primary); }
.modal__confirm {
  background: var(--accent); color: #fff; border: none; border-radius: 0.375rem;
  padding: 0.4rem 0.875rem; font-size: 0.8rem; font-weight: 600;
  font-family: inherit; cursor: pointer; display: flex; align-items: center;
  gap: 0.4rem; min-width: 80px; justify-content: center; transition: background 0.15s;
}
.modal__confirm:hover:not(:disabled) { background: var(--accent-hover); }
.modal__confirm:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-spinner {
  width: 12px; height: 12px; border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff; border-radius: 50%; animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
