<template>
  <div class="containers">

    <!-- Stat strip -->
    <div class="stat-grid">
      <div v-for="s in stats" :key="s.label" class="stat-card">
        <span class="stat-card__value">{{ s.value }}</span>
        <span class="stat-card__label">{{ s.label }}</span>
      </div>
    </div>

    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar__search">
        <Icon name="lucide:search" size="14" class="toolbar__search-icon" />
        <input v-model="search" class="toolbar__input" placeholder="Search containers…" />
      </div>
      <div class="toolbar__filters">
        <button
          v-for="f in filters"
          :key="f.value"
          class="toolbar__filter"
          :class="{ 'toolbar__filter--active': activeFilter === f.value }"
          @click="activeFilter = f.value"
        >
          {{ f.label }}
        </button>
      </div>
      <button class="deploy-btn" @click="deployModal = true">
        <Icon name="lucide:upload" size="13" />
        Deploy
      </button>
    </div>

    <!-- Deploy compose modal -->
    <div v-if="deployModal" class="modal-backdrop" @click.self="closeDeployModal">
      <div class="modal">
        <div class="modal__header">
          <h3 class="modal__title">Deploy Compose</h3>
          <button class="modal__close" @click="closeDeployModal">
            <Icon name="lucide:x" size="14" />
          </button>
        </div>
        <div class="modal__body">
          <div class="form-row">
            <label class="form-label">Target node</label>
            <select v-model="deployNodeId" class="form-select">
              <option value="">Local (this node)</option>
              <option v-for="n in deployNodes" :key="n.id" :value="n.id">{{ n.name }} — {{ n.address }}</option>
            </select>
          </div>
          <label class="file-label">
            <input ref="fileInput" type="file" accept=".yml,.yaml" class="file-input" @change="onFileChange" />
            <div class="file-drop" :class="{ 'file-drop--has': !!yamlFile }">
              <Icon :name="yamlFile ? 'lucide:file-check' : 'lucide:file-up'" size="20" />
              <span>{{ yamlFile ? yamlFile.name : 'Click to choose .yml / .yaml' }}</span>
            </div>
          </label>
          <div v-if="deployError" class="modal__error">{{ deployError }}</div>
          <div v-if="deployOutput" class="modal__output">{{ deployOutput }}</div>
        </div>
        <div class="modal__footer">
          <button class="modal__cancel" @click="closeDeployModal">Cancel</button>
          <button class="modal__confirm" :disabled="!yamlFile || deploying" @click="runDeploy">
            <span v-if="deploying" class="btn-spinner" />
            <span v-else>Deploy</span>
          </button>
        </div>
      </div>
    </div>

    <!-- One collapsible card per node -->
    <div v-for="group in filteredGroups" :key="group.node.id" class="group-card">

      <!-- Clickable header -->
      <button class="group-card__header" @click="toggle(group.node.id)">
        <UiStatusDot :status="group.node.status" />
        <span class="node-header__name">{{ group.node.name }}</span>
        <UiBadge variant="info">{{ group.node.role }}</UiBadge>
        <span class="node-header__count">
          {{ group.containers.length }} container{{ group.containers.length !== 1 ? 's' : '' }}
        </span>
        <Icon
          name="lucide:chevron-down"
          size="14"
          class="group-card__chevron"
          :class="{ 'group-card__chevron--collapsed': collapsed.has(group.node.id) }"
        />
      </button>

      <!-- Collapsible body using CSS grid trick -->
      <div class="group-card__body" :class="{ 'group-card__body--collapsed': collapsed.has(group.node.id) }">
        <div class="group-card__inner">
          <table class="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Image</th>
                <th>State</th>
                <th>Ports</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="c in group.containers"
                :key="c.Id"
                class="data-table__row"
                @click="openContainer(c.Id, group.node.nodeId)"
              >
                <td><span class="container-name">{{ containerName(c) }}</span></td>
                <td class="mono">{{ c.Image }}</td>
                <td><UiBadge :variant="statusVariant(c.State)">{{ c.State }}</UiBadge></td>
                <td class="mono muted">{{ containerPorts(c) }}</td>
                <td class="muted">{{ c.Status }}</td>
                <td><Icon name="lucide:chevron-right" size="14" class="row-arrow" /></td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

    </div>

    <div v-if="filteredGroups.length === 0" class="empty-state">
      <Icon name="lucide:box" size="32" class="empty-state__icon" />
      <span>No containers match your search.</span>
    </div>

  </div>
</template>

<script setup lang="ts">
import type { NodeStatus } from '~/components/ui/StatusDot.vue'

definePageMeta({ layout: 'default' })

const api        = useApi()
const nodesStore = useNodesStore()

function openContainer(id: string, nodeId?: string) {
  nodesStore.select(nodeId ?? null)
  navigateTo(`/containers/${id}`)
}

// ── Remote data ────────────────────────────────────────────────────────────
interface NodeGroup {
  node: { id: string; name: string; role: string; status: NodeStatus; nodeId?: string }
  containers: DockerContainer[]
}
const groups   = ref<NodeGroup[]>([])
const loading  = ref(true)
const apiError = ref('')

async function loadContainers() {
  loading.value = true
  apiError.value = ''
  try {
    const nodes = await api.listNodes().catch(() => [] as NodeRecord[])

    const targets: Array<{ id: string; name: string; role: string; status: NodeStatus; nodeId?: string }> = [
      { id: 'local', name: 'Local', role: 'hub', status: 'healthy' },
      ...nodes
        .filter(n => n.type !== 'hub')
        .map(n => ({
          id: n.id, name: n.name, role: n.type,
          status: 'pending' as NodeStatus,
          nodeId: n.id,
        })),
    ]

    const results = await Promise.allSettled(
      targets.map(t => api.listContainers(t.nodeId))
    )

    groups.value = targets.map((t, i) => {
      const res = results[i]
      return { node: t, containers: res?.status === 'fulfilled' ? res.value : [] }
    })
  } catch (e: any) {
    apiError.value = e?.data?.error ?? 'Failed to load containers'
  } finally {
    loading.value = false
  }
}

onMounted(loadContainers)

// ── Normalise Docker API fields ────────────────────────────────────────────
function containerName(c: DockerContainer): string {
  return (c.Names?.[0] ?? c.Id.slice(0, 12)).replace(/^\//, '')
}
function containerPorts(c: DockerContainer): string {
  const mapped = (c.Ports ?? [])
    .filter(p => p.PublicPort)
    .map(p => p.PublicPort)
  return mapped.length ? mapped.join(', ') : '—'
}

// ── Search / filter ────────────────────────────────────────────────────────
const search       = ref('')
const activeFilter = ref<'all' | 'running' | 'exited' | 'paused'>('all')
const collapsed    = ref(new Set<string>())

function toggle(id: string) {
  collapsed.value = collapsed.value.has(id)
    ? new Set([...collapsed.value].filter(x => x !== id))
    : new Set([...collapsed.value, id])
}

const filters = [
  { label: 'All',     value: 'all'     },
  { label: 'Running', value: 'running' },
  { label: 'Exited',  value: 'exited'  },
  { label: 'Paused',  value: 'paused'  },
] as const

function filterContainers(containers: DockerContainer[]): DockerContainer[] {
  return containers.filter((c: DockerContainer) => {
    const state = c.State?.toLowerCase() ?? ''
    if (activeFilter.value !== 'all' && state !== activeFilter.value) return false
    if (search.value) {
      const q = search.value.toLowerCase()
      return containerName(c).includes(q) || (c.Image ?? '').toLowerCase().includes(q)
    }
    return true
  })
}

const filteredGroups = computed(() =>
  groups.value
    .map(g => ({ node: g.node, containers: filterContainers(g.containers) }))
    .filter(g => g.containers.length > 0)
)

const allContainers = computed(() => groups.value.flatMap(g => g.containers))

const stats = computed(() => {
  const all = allContainers.value
  return [
    { label: 'Total',   value: all.length },
    { label: 'Running', value: all.filter((c: DockerContainer) => c.State === 'running').length },
    { label: 'Exited',  value: all.filter((c: DockerContainer) => c.State === 'exited').length },
    { label: 'Paused',  value: all.filter((c: DockerContainer) => c.State === 'paused').length },
  ]
})

function statusVariant(state: string) {
  if (state === 'running') return 'success' as const
  if (state === 'paused')  return 'warning' as const
  return 'neutral' as const
}

// ── Deploy modal ───────────────────────────────────────────────────────────
const deployModal  = ref(false)
const deployNodeId = ref('')
const deployNodes  = ref<NodeRecord[]>([])
const yamlFile     = ref<File | null>(null)
const deploying    = ref(false)
const deployError  = ref('')
const deployOutput = ref('')
const fileInput    = ref<HTMLInputElement | null>(null)

watch(deployModal, async (open) => {
  if (open && deployNodes.value.length === 0) {
    try { deployNodes.value = (await api.listNodes()).filter(n => n.type !== 'hub') } catch {}
  }
})

function onFileChange(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0] ?? null
  yamlFile.value    = f
  deployError.value = ''
  deployOutput.value = ''
}

function closeDeployModal() {
  deployModal.value  = false
  deployNodeId.value = ''
  yamlFile.value     = null
  deployError.value  = ''
  deployOutput.value = ''
  deploying.value    = false
  if (fileInput.value) fileInput.value.value = ''
}

async function runDeploy() {
  if (!yamlFile.value) return
  deploying.value   = true
  deployError.value = ''
  deployOutput.value = ''
  try {
    const text = await yamlFile.value.text()
    const res  = await api.deployCompose(text, deployNodeId.value || undefined)
    deployOutput.value = res.output ?? 'Deployed.'
    await loadContainers()
  } catch (e: any) {
    deployError.value = e?.data?.error ?? e?.data ?? e?.message ?? 'Deploy failed.'
  } finally {
    deploying.value = false
  }
}

</script>

<style scoped>
.containers { display: flex; flex-direction: column; gap: 1rem; }

/* Stats */
.stat-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1rem; }
.stat-card {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 1rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}
.stat-card__value { font-size: 1.5rem; font-weight: 700; color: var(--text-primary); }
.stat-card__label { font-size: 0.75rem; color: var(--text-muted); }

/* Toolbar */
.toolbar { display: flex; align-items: center; gap: 0.75rem; }
.toolbar__search {
  position: relative;
  display: flex;
  align-items: center;
}
.toolbar__search-icon { position: absolute; left: 0.6rem; color: var(--text-dim); pointer-events: none; }
.toolbar__input {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.375rem;
  padding: 0.35rem 0.75rem 0.35rem 2rem;
  color: var(--text-primary);
  font-size: 0.8rem;
  font-family: inherit;
  width: 200px;
  outline: none;
  transition: border-color 0.15s;
}
.toolbar__input:focus { border-color: var(--accent); }
.toolbar__input::placeholder { color: var(--text-subtle); }
.toolbar__filters { display: flex; gap: 0.25rem; }
.toolbar__filter {
  background: none;
  border: 1px solid var(--border);
  border-radius: 0.375rem;
  padding: 0.3rem 0.65rem;
  font-size: 0.75rem;
  color: var(--text-muted);
  cursor: pointer;
  font-family: inherit;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}
.toolbar__filter:hover { color: var(--text-primary); border-color: var(--text-subtle); }
.toolbar__filter--active { color: var(--accent-light); border-color: var(--accent); background: rgba(124,58,237,0.08); }

/* Collapsible group card */
.group-card {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  overflow: hidden;
}
.group-card__header {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  width: 100%;
  padding: 0.875rem 1.25rem;
  background: none;
  border: none;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  font-family: inherit;
  text-align: left;
  transition: background 0.15s;
}
.group-card__header:hover { background: var(--hover-subtle); }
.group-card__chevron {
  color: var(--text-dim);
  margin-left: auto;
  flex-shrink: 0;
  transition: transform 0.22s ease;
}
.group-card__chevron--collapsed { transform: rotate(-90deg); }

/* CSS grid height trick for smooth collapse */
.group-card__body {
  display: grid;
  grid-template-rows: 1fr;
  transition: grid-template-rows 0.22s ease;
}
.group-card__body--collapsed { grid-template-rows: 0fr; }
.group-card__inner { min-height: 0; overflow: hidden; padding: 0 1.25rem 1.25rem; }

/* Node header */
.node-header__name { font-weight: 600; color: var(--text-primary); font-size: 0.875rem; }
.node-header__meta { font-family: monospace; font-size: 0.72rem; color: var(--text-dim); flex: 1; }
.node-header__count { font-size: 0.72rem; color: var(--text-dim); }

/* Table */
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
.data-table__row { cursor: pointer; transition: background 0.1s; }
.data-table__row:hover td { background: var(--hover-bg); }
.data-table__row:hover .row-arrow { color: var(--accent-light); }
.data-table__empty { text-align: center; color: var(--text-subtle); padding: 1.5rem !important; }

.container-name { display: flex; align-items: center; gap: 0.5rem; }
.container-name__text { color: var(--text-primary); font-weight: 500; }

/* CPU bar */
.cpu-bar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.cpu-bar__fill {
  height: 4px;
  width: 40px;
  border-radius: 2px;
  min-width: 2px;
  transition: width 0.3s;
}
.cpu-bar__label { font-family: monospace; font-size: 0.75rem; color: var(--text-tertiary); }

.mono  { font-family: monospace; font-size: 0.75rem; color: var(--text-tertiary); }
.muted { color: var(--text-dim); }
.row-arrow { color: var(--border-strong); transition: color 0.15s; display: block; }

/* Empty */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  padding: 3rem;
  color: var(--text-subtle);
  font-size: 0.875rem;
}
.empty-state__icon { color: var(--border); }

/* Deploy button */
.deploy-btn {
  display: flex; align-items: center; gap: 0.4rem;
  background: var(--accent); color: #fff; border: none; border-radius: 0.375rem;
  padding: 0.35rem 0.75rem; font-size: 0.78rem; font-weight: 600;
  font-family: inherit; cursor: pointer; transition: background 0.15s;
  margin-left: auto;
}
.deploy-btn:hover { background: var(--accent-hover); }

/* Modal */
.modal-backdrop {
  position: fixed; inset: 0; background: rgba(0,0,0,0.6);
  display: flex; align-items: center; justify-content: center;
  z-index: 100;
}
.modal {
  background: var(--bg-surface); border: 1px solid var(--border);
  border-radius: 0.625rem; width: 480px; max-width: 95vw;
  display: flex; flex-direction: column; gap: 0;
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
  font-family: inherit; cursor: pointer; display: flex; align-items: center; gap: 0.4rem;
  min-width: 80px; justify-content: center; transition: background 0.15s;
}
.modal__confirm:hover:not(:disabled) { background: var(--accent-hover); }
.modal__confirm:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-spinner {
  width: 12px; height: 12px; border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff; border-radius: 50%; animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
