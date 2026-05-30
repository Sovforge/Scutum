<template>
  <div class="nodes">

    <!-- Stat strip -->
    <div class="stat-grid">
      <div v-for="s in stats" :key="s.label" class="stat-card">
        <span class="stat-card__value">{{ s.value }}</span>
        <span class="stat-card__label">{{ s.label }}</span>
      </div>
    </div>

    <!-- Table card -->
    <UiCard>
      <template #header-right>
        <div class="toolbar">
          <button class="toolbar__enroll" @click="showEnroll = true">
            <Icon name="lucide:user-plus" size="14" /> Enroll Peer
          </button>
          <div class="toolbar__search">
            <Icon name="lucide:search" size="14" class="toolbar__search-icon" />
            <input v-model="search" class="toolbar__input" placeholder="Search nodes…" />
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
        </div>
      </template>

      <div v-if="apiError" class="api-error">{{ apiError }}</div>
      <div v-else-if="loading" class="loading-row">Loading…</div>
      <table v-else class="data-table">
        <thead>
          <tr>
            <th>Node</th>
            <th>Type</th>
            <th>WireGuard address</th>
            <th>Public key</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="node in filtered"
            :key="node.id"
            class="data-table__row"
          >
            <td>
              <div class="node-name">
                <UiStatusDot status="pending" />
                <span class="node-name__text">{{ node.name }}</span>
              </div>
            </td>
            <td><UiBadge variant="info">{{ node.type }}</UiBadge></td>
            <td class="mono">{{ node.address }}</td>
            <td class="mono key">{{ node.public_key }}</td>
            <td class="cell--actions">
              <template v-if="pendingDelete === node.id">
                <span class="delete-confirm-label">Remove?</span>
                <button class="icon-btn" @click="pendingDelete = null">Cancel</button>
                <button class="icon-btn icon-btn--danger" @click="confirmDelete(node.id)">Confirm</button>
              </template>
              <button v-else class="icon-btn icon-btn--danger" title="Remove node" @click.stop="pendingDelete = node.id">
                <Icon name="lucide:trash-2" size="13" />
              </button>
            </td>
          </tr>
          <tr v-if="!filtered.length">
            <td colspan="5" class="data-table__empty">{{ nodes.length ? 'No nodes match your filter.' : 'No nodes enrolled yet.' }}</td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <!-- Node Groups card -->
    <UiCard title="Node Groups">
      <template #header-right>
        <button class="toolbar__enroll" @click="showGroupForm = true">
          <Icon name="lucide:plus" size="14" /> New Group
        </button>
      </template>

      <div v-if="groupsLoading" class="loading-row">Loading…</div>
      <div v-else-if="!groups.length" class="data-table__empty" style="padding:1.5rem;text-align:center">
        <Icon name="lucide:layers" size="20" style="opacity:0.3;display:block;margin:0 auto 0.5rem" />
        No groups yet. Create one to organise nodes by role or environment.
      </div>
      <template v-else>
        <div v-for="group in groups" :key="group.id" class="group-row">
          <div class="group-header" @click="toggleGroup(group.id)">
            <div class="group-header__left">
              <Icon :name="expandedGroup === group.id ? 'lucide:chevron-down' : 'lucide:chevron-right'" size="13" class="group-chevron" />
              <span class="group-name">{{ group.name }}</span>
              <span v-if="group.description" class="group-desc">{{ group.description }}</span>
            </div>
            <div class="group-header__right">
              <span class="group-count">{{ group.members?.length ?? 0 }} node{{ (group.members?.length ?? 0) !== 1 ? 's' : '' }}</span>
              <template v-if="pendingGroupDelete === group.id">
                <span class="delete-confirm-label">Delete?</span>
                <button class="icon-btn" @click.stop="pendingGroupDelete = null">Cancel</button>
                <button class="icon-btn icon-btn--danger" @click.stop="deleteGroup(group.id)">Confirm</button>
              </template>
              <button v-else class="icon-btn icon-btn--danger" title="Delete group" @click.stop="pendingGroupDelete = group.id">
                <Icon name="lucide:trash-2" size="13" />
              </button>
            </div>
          </div>

          <div v-if="expandedGroup === group.id" class="group-members">
            <div v-if="!groupNodes[group.id]?.length" class="group-empty">No nodes in this group.</div>
            <div v-for="n in groupNodes[group.id]" :key="n.id" class="group-member">
              <UiStatusDot status="pending" />
              <span class="group-member__name">{{ n.name }}</span>
              <UiBadge variant="info">{{ n.type }}</UiBadge>
              <button class="icon-btn icon-btn--danger" title="Remove from group" @click="removeMember(group.id, n.id)">
                <Icon name="lucide:x" size="12" />
              </button>
            </div>
            <div class="group-add-row">
              <select v-model="addMemberNode[group.id]" class="form-select form-select--sm">
                <option value="">Add a node…</option>
                <option
                  v-for="n in nodesNotInGroup(group)"
                  :key="n.id"
                  :value="n.id"
                >{{ n.name }} ({{ n.type }})</option>
              </select>
              <button
                class="toolbar__enroll"
                :disabled="!addMemberNode[group.id]"
                @click="addMember(group.id)"
              >Add</button>
            </div>
          </div>
        </div>
      </template>
    </UiCard>

  </div>

  <!-- New Group Modal -->
  <div v-if="showGroupForm" class="modal-backdrop" @click.self="showGroupForm = false">
    <div class="modal modal--sm">
      <div class="modal__header">
        <div class="modal__title">
          <Icon name="lucide:layers" size="15" class="modal__title-icon" />
          New Node Group
        </div>
        <button class="modal__close" @click="showGroupForm = false"><Icon name="lucide:x" size="15" /></button>
      </div>
      <div class="modal__body">
        <div class="form-grid">
          <div class="form-row">
            <label class="form-label">Name</label>
            <input v-model="groupForm.name" class="form-input" placeholder="production" />
          </div>
          <div class="form-row">
            <label class="form-label">Description</label>
            <input v-model="groupForm.description" class="form-input" placeholder="Optional description" />
          </div>
        </div>
      </div>
      <div class="modal__footer">
        <p v-if="groupError" class="enroll-error">{{ groupError }}</p>
        <button class="cancel-btn" @click="showGroupForm = false">Cancel</button>
        <button class="save-btn" @click="createGroup" :disabled="groupSaving || !groupForm.name">
          <Icon name="lucide:plus" size="14" /> {{ groupSaving ? 'Creating…' : 'Create Group' }}
        </button>
      </div>
    </div>
  </div>

  <!-- Manual Enrollment Modal -->
  <div v-if="showEnroll" class="modal-backdrop" @click.self="showEnroll = false">
    <div class="modal">
      <div class="modal__header">
        <div class="modal__title">
          <Icon name="lucide:shield" size="15" class="modal__title-icon" />
          Enroll Peer Node
        </div>
        <button class="modal__close" @click="showEnroll = false">
          <Icon name="lucide:x" size="15" />
        </button>
      </div>

      <div class="modal__body">
        <div class="enroll-note">
          <Icon name="lucide:info" size="13" class="enroll-note__icon" />
          Enter the remote node's WireGuard details. That node must have already been configured to point to this hub — enrollment here adds it to the mesh and authorises the connection.
        </div>

        <!-- Hub's own key — remote operators need this to configure their node -->
        <div class="hub-key-block">
          <span class="hub-key-label">This hub's public key</span>
          <div class="key-block">
            <code class="key-val">{{ localPubkey }}</code>
            <button class="copy-btn" @click="copyKey(localPubkey)" :class="{ 'copy-btn--done': copied === 'local' }">
              <Icon :name="copied === 'local' ? 'lucide:check' : 'lucide:copy'" size="12" />
            </button>
          </div>
          <span class="hub-key-hint">Give this key and endpoint <code class="inline-mono">{{ localEndpoint }}</code> to the remote node operator so they can point their node at this hub.</span>
        </div>

        <!-- Hub HMAC key — remote node needs this to accept proxied requests -->
        <div class="hub-key-block">
          <span class="hub-key-label">Hub proxy key <span style="font-weight:400;text-transform:none;letter-spacing:0">(enter this in the remote node's setup)</span></span>
          <div class="key-block">
            <code class="key-val">{{ hubHMACKey || 'loading…' }}</code>
            <button class="copy-btn" @click="copyKey(hubHMACKey, 'hmac')" :class="{ 'copy-btn--done': copied === 'hmac' }" :disabled="!hubHMACKey">
              <Icon :name="copied === 'hmac' ? 'lucide:check' : 'lucide:copy'" size="12" />
            </button>
          </div>
          <span class="hub-key-hint">The remote node needs this key so it can accept API requests proxied from this hub.</span>
        </div>

        <div class="form-grid" style="margin-top:0.25rem">
          <div class="form-row">
            <label class="form-label">Node name</label>
            <input v-model="enrollForm.name" class="form-input" placeholder="worker-03" />
          </div>
          <div class="form-row">
            <label class="form-label">Remote node's public key</label>
            <input v-model="enrollForm.pubkey" class="form-input font-mono" placeholder="Base64-encoded WireGuard public key" />
          </div>
          <div class="form-row">
            <label class="form-label">Remote node's endpoint <span class="form-label-hint">(host:port, if reachable)</span></label>
            <input v-model="enrollForm.endpoint" class="form-input font-mono" placeholder="1.2.3.4:51820" />
          </div>
          <div class="form-row">
            <label class="form-label">Allowed IPs</label>
            <input v-model="enrollForm.allowedIPs" class="form-input font-mono" placeholder="10.102.132.2/32" />
          </div>
          <div class="form-row">
            <label class="form-label">API address <span class="form-label-hint">(host:port)</span></label>
            <input v-model="enrollForm.apiAddress" class="form-input font-mono" placeholder="10.102.132.2:8086" />
          </div>
          <div class="form-row">
            <label class="form-label">Role</label>
            <select v-model="enrollForm.role" class="form-select">
              <option value="remote">Remote</option>
              <option value="hub">Hub</option>
            </select>
          </div>
        </div>
      </div>

      <div class="modal__footer">
        <p v-if="enrollError" class="enroll-error">{{ enrollError }}</p>
        <button class="cancel-btn" @click="showEnroll = false">Cancel</button>
        <button class="save-btn" @click="enroll" :disabled="enrollSaving || !enrollForm.pubkey || !enrollForm.endpoint || !enrollForm.name || !enrollForm.allowedIPs || !enrollForm.apiAddress">
          <Icon name="lucide:user-check" size="14" /> {{ enrollSaving ? 'Enrolling…' : 'Enroll Peer' }}
        </button>
      </div>
    </div>
  </div>

</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()

// ── Remote data ────────────────────────────────────────────────────────────
const nodes   = ref<NodeRecord[]>([])
const loading = ref(true)
const apiError = ref('')
const enrollError = ref('')
const enrollSaving = ref(false)

async function loadNodes() {
  loading.value = true
  apiError.value = ''
  try {
    nodes.value = await api.listNodes()
  } catch (e: any) {
    apiError.value = e?.data?.error ?? 'Failed to load nodes'
  } finally {
    loading.value = false
  }
}

onMounted(loadNodes)

// ── Enroll form ────────────────────────────────────────────────────────────
const showEnroll  = ref(false)
const copied      = ref<string | null>(null)
const hubHMACKey  = ref('')

const wgPubkeyCookie  = useCookie<string>('wg_pubkey')
const wgAddressCookie = useCookie<string>('wg_address')
const localPubkey   = computed(() => wgPubkeyCookie.value  || '(complete setup to get public key)')
const localEndpoint = computed(() => wgAddressCookie.value || '—')

watch(showEnroll, async (open) => {
  if (open && !hubHMACKey.value) {
    try {
      const res = await api.getHubKey()
      hubHMACKey.value = res.hmac_key
    } catch {}
  }
})

const enrollForm = reactive({ name: '', pubkey: '', endpoint: '', allowedIPs: '', apiAddress: '', role: 'remote' })

async function copyKey(key: string, slot = 'local') {
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(key)
    } else {
      const el = document.createElement('textarea')
      el.value = key
      el.style.cssText = 'position:fixed;opacity:0;pointer-events:none'
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
    }
    copied.value = slot
    setTimeout(() => { copied.value = null }, 2000)
  } catch {}
}

async function enroll() {
  if (!enrollForm.pubkey || !enrollForm.endpoint || !enrollForm.name || !enrollForm.allowedIPs || !enrollForm.apiAddress) return
  enrollError.value = ''
  enrollSaving.value = true
  try {
    const node = await api.createNode({
      name:       enrollForm.name,
      type:       enrollForm.role,
      address:    enrollForm.apiAddress,
      public_key: enrollForm.pubkey,
    })
    await api.addPeer({
      public_key:  enrollForm.pubkey,
      endpoint:    enrollForm.endpoint,
      allowed_ips: enrollForm.allowedIPs,
      node_id:     node.id,
    })
    await loadNodes()
    showEnroll.value = false
    Object.assign(enrollForm, { name: '', pubkey: '', endpoint: '', allowedIPs: '', apiAddress: '', role: 'remote' })
  } catch (e: any) {
    enrollError.value = e?.data?.error ?? 'Enrollment failed'
  } finally {
    enrollSaving.value = false
  }
}

// ── Delete ─────────────────────────────────────────────────────────────────
const pendingDelete = ref<string | null>(null)

async function confirmDelete(id: string) {
  try {
    await api.deleteNode(id)
    await loadNodes()
  } catch {
    apiError.value = 'Delete failed'
  }
  pendingDelete.value = null
}

// ── Filter / search ────────────────────────────────────────────────────────
const search       = ref('')
const activeFilter = ref<'all' | 'hub' | 'remote'>('all')

const filters = [
  { label: 'All',    value: 'all'    },
  { label: 'Hub',    value: 'hub'    },
  { label: 'Remote', value: 'remote' },
] as const

const filtered = computed(() =>
  nodes.value.filter(n => {
    const matchesFilter = activeFilter.value === 'all' || n.type === activeFilter.value
    const q = search.value.toLowerCase()
    const matchesSearch = !q || n.name.toLowerCase().includes(q) || n.address.includes(q)
    return matchesFilter && matchesSearch
  })
)

const stats = computed(() => [
  { label: 'Total',  value: nodes.value.length },
  { label: 'Hub',    value: nodes.value.filter(n => n.type === 'hub').length },
  { label: 'Remote', value: nodes.value.filter(n => n.type === 'remote').length },
])

// ── Node Groups ────────────────────────────────────────────────────────────
interface NodeGroup { id: string; name: string; description: string; members?: string[] }

const groups        = ref<NodeGroup[]>([])
const groupsLoading = ref(true)
const expandedGroup = ref<string | null>(null)
const groupNodes    = ref<Record<string, any[]>>({})
const addMemberNode = ref<Record<string, string>>({})
const pendingGroupDelete = ref<string | null>(null)
const showGroupForm  = ref(false)
const groupSaving    = ref(false)
const groupError     = ref('')
const groupForm      = reactive({ name: '', description: '' })

async function loadGroups() {
  groupsLoading.value = true
  try {
    groups.value = await api.listNodeGroups()
  } catch {}
  groupsLoading.value = false
}

onMounted(loadGroups)

async function toggleGroup(id: string) {
  if (expandedGroup.value === id) {
    expandedGroup.value = null
    return
  }
  expandedGroup.value = id
  if (!groupNodes.value[id]) {
    try {
      groupNodes.value[id] = await api.getGroupNodes(id)
    } catch {
      groupNodes.value[id] = []
    }
  }
}

function nodesNotInGroup(group: NodeGroup) {
  const memberIds = new Set(groupNodes.value[group.id]?.map((n: any) => n.id) ?? [])
  return nodes.value.filter(n => !memberIds.has(n.id))
}

async function addMember(groupId: string) {
  const nodeId = addMemberNode.value[groupId]
  if (!nodeId) return
  try {
    await api.addNodeToGroup(groupId, nodeId)
    groupNodes.value[groupId] = await api.getGroupNodes(groupId)
    addMemberNode.value[groupId] = ''
    await loadGroups()
  } catch {}
}

async function removeMember(groupId: string, nodeId: string) {
  try {
    await api.removeNodeFromGroup(groupId, nodeId)
    groupNodes.value[groupId] = await api.getGroupNodes(groupId)
    await loadGroups()
  } catch {}
}

async function deleteGroup(id: string) {
  try {
    await api.deleteNodeGroup(id)
    if (expandedGroup.value === id) expandedGroup.value = null
    delete groupNodes.value[id]
    await loadGroups()
  } catch {}
  pendingGroupDelete.value = null
}

async function createGroup() {
  if (!groupForm.name) return
  groupError.value = ''
  groupSaving.value = true
  try {
    await api.createNodeGroup({ name: groupForm.name, description: groupForm.description })
    await loadGroups()
    showGroupForm.value = false
    Object.assign(groupForm, { name: '', description: '' })
  } catch (e: any) {
    groupError.value = e?.data?.error ?? 'Failed to create group'
  } finally {
    groupSaving.value = false
  }
}

</script>

<style scoped>
.nodes { display: flex; flex-direction: column; gap: 1rem; }

.api-error, .loading-row {
  padding: 1rem 0.75rem;
  font-size: 0.82rem;
  color: var(--text-muted);
}
.api-error { color: var(--danger-light); }

.cell--actions {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  justify-content: flex-end;
  white-space: nowrap;
}
.delete-confirm-label {
  font-size: 0.75rem;
  color: var(--danger-light);
  margin-right: 0.25rem;
}
.icon-btn {
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.25rem;
  color: var(--text-muted);
  padding: 0.25rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  font-size: 0.72rem;
  font-family: inherit;
  transition: all 0.15s;
}
.icon-btn:hover           { color: var(--text-primary); border-color: var(--border-hover); }
.icon-btn--danger:hover   { color: var(--danger-light); border-color: #7f1d1d; }

.enroll-error {
  flex: 1;
  font-size: 0.75rem;
  color: var(--danger-light);
  margin: 0;
}

/* Stats */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1rem;
}
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
.toolbar {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}
.toolbar__search {
  position: relative;
  display: flex;
  align-items: center;
}
.toolbar__search-icon {
  position: absolute;
  left: 0.6rem;
  color: var(--text-dim);
  pointer-events: none;
}
.toolbar__input {
  background: var(--bg-base);
  border: 1px solid var(--border);
  border-radius: 0.375rem;
  padding: 0.35rem 0.75rem 0.35rem 2rem;
  color: var(--text-primary);
  font-size: 0.8rem;
  font-family: inherit;
  width: 180px;
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
.toolbar__filter--active {
  color: var(--accent-light);
  border-color: var(--accent);
  background: rgba(124, 58, 237, 0.08);
}

/* Table */
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
}
.data-table th {
  text-align: left;
  color: var(--text-dim);
  font-weight: 500;
  padding: 0 0.75rem 0.75rem;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
.data-table td {
  padding: 0.7rem 0.75rem;
  color: var(--text-secondary);
  border-bottom: 1px solid transparent;
}
.data-table__row { cursor: pointer; transition: background 0.1s; }
.data-table__row:hover td { background: var(--hover-bg); }
.data-table__row:hover .row-arrow { color: var(--accent-light); }
.data-table__empty {
  text-align: center;
  color: var(--text-subtle);
  padding: 2rem !important;
}

.node-name { display: flex; align-items: center; gap: 0.5rem; }
.node-name__text { color: var(--text-primary); font-weight: 500; }

.mono  { font-family: monospace; font-size: 0.75rem; color: var(--text-tertiary); }
.key   { max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.muted { color: var(--text-dim); }
.row-arrow { color: var(--border-strong); transition: color 0.15s; display: block; }

/* Enroll button */
.toolbar__enroll {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.3rem 0.75rem;
  background: var(--accent-subtle);
  border: 1px solid var(--accent-soft);
  border-radius: 0.375rem;
  font-size: 0.78rem;
  color: var(--accent-light);
  cursor: pointer;
  font-family: inherit;
  transition: all 0.15s;
}
.toolbar__enroll:hover { background: var(--accent-tint); }

/* Modal */
.modal-backdrop {
  position: fixed; inset: 0;
  background: #00000090;
  display: flex; align-items: center; justify-content: center;
  z-index: 200;
}
.modal {
  background: var(--bg-surface);
  border: 1px solid var(--border-strong);
  border-radius: 0.75rem;
  width: 540px; max-width: 95vw;
  display: flex; flex-direction: column;
  overflow: hidden;
  max-height: 90vh;
}
.modal__header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border);
}
.modal__title {
  display: flex; align-items: center; gap: 0.5rem;
  font-size: 0.9rem; font-weight: 600; color: var(--text-primary);
}
.modal__title-icon { color: var(--accent); }
.modal__close {
  background: none; border: none; color: var(--text-muted);
  cursor: pointer; display: flex; align-items: center;
  padding: 0.2rem; border-radius: 0.25rem;
}
.modal__close:hover { color: var(--text-primary); }
.modal__body { padding: 1.25rem; overflow-y: auto; display: flex; flex-direction: column; gap: 1rem; }
.modal__footer {
  display: flex; justify-content: flex-end; gap: 0.75rem;
  padding: 0.875rem 1.25rem;
  border-top: 1px solid var(--border);
}

/* Enrollment note */
.enroll-note {
  display: flex; align-items: flex-start; gap: 0.625rem;
  background: #1e40af12; border: 1px solid #1e40af33;
  border-radius: 0.5rem; padding: 0.75rem 1rem;
  font-size: 0.8rem; color: #93c5fd; line-height: 1.5;
}
.enroll-note__icon { color: #60a5fa; flex-shrink: 0; margin-top: 0.1rem; }
.enroll-note strong { color: #bfdbfe; }

.hub-key-block {
  display: flex; flex-direction: column; gap: 0.3rem;
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 0.5rem; padding: 0.625rem 0.75rem;
}
.hub-key-label { font-size: 0.68rem; font-weight: 600; color: var(--text-dim); text-transform: uppercase; letter-spacing: 0.04em; }
.hub-key-hint  { font-size: 0.72rem; color: var(--text-subtle); line-height: 1.5; }
.hub-key-hint .inline-mono { font-family: monospace; font-size: 0.7rem; }

/* Key block */
.key-block {
  display: flex; align-items: center; gap: 0.625rem;
  background: var(--bg-deep); border: 1px solid var(--border);
  border-radius: 0.375rem; padding: 0.5rem 0.75rem;
}
.key-val {
  font-family: monospace; font-size: 0.78rem; color: var(--text-tertiary);
  word-break: break-all; flex: 1;
}
.inline-mono { font-family: monospace; font-size: 0.78rem; color: var(--accent-light); }
.copy-btn {
  background: none; border: 1px solid var(--border-strong); border-radius: 0.25rem;
  color: var(--text-muted); padding: 0.25rem; cursor: pointer;
  display: flex; align-items: center; flex-shrink: 0; transition: all 0.15s;
}
.copy-btn:hover { color: var(--text-primary); }
.copy-btn--done { color: var(--success-light); border-color: var(--role-dev-border); }

/* Form */
.form-grid { display: flex; flex-direction: column; gap: 0.75rem; }
.form-row { display: grid; grid-template-columns: 110px 1fr; align-items: center; gap: 0.75rem; }
.form-label { font-size: 0.8rem; color: var(--text-tertiary); }
.form-input, .form-select {
  background: var(--bg-overlay); border: 1px solid var(--border-strong); border-radius: 0.375rem;
  padding: 0.38rem 0.625rem; font-size: 0.8rem; color: var(--text-primary);
  outline: none; width: 100%; font-family: inherit;
}
.form-input:focus, .form-select:focus { border-color: var(--accent); }
.font-mono { font-family: monospace; font-size: 0.75rem; }

/* Buttons */
.save-btn {
  display: inline-flex; align-items: center; gap: 0.4rem;
  background: var(--accent); border: none; border-radius: 0.375rem;
  padding: 0.45rem 1.25rem; font-size: 0.82rem; color: #fff;
  cursor: pointer; transition: background 0.15s;
}
.save-btn:hover:not(:disabled) { background: var(--accent-hover); }
.save-btn:disabled { opacity: 0.45; cursor: not-allowed; }
.cancel-btn {
  background: none; border: 1px solid var(--border-strong); border-radius: 0.375rem;
  padding: 0.45rem 1rem; font-size: 0.82rem; color: var(--text-muted); cursor: pointer;
}
.cancel-btn:hover { color: var(--text-primary); border-color: var(--border-hover); }

/* Groups */
.group-row {
  border-bottom: 1px solid var(--border);
}
.group-row:last-child { border-bottom: none; }

.group-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0.65rem 0.75rem;
  cursor: pointer;
  transition: background 0.1s;
}
.group-header:hover { background: var(--hover-bg); }
.group-header__left { display: flex; align-items: center; gap: 0.5rem; }
.group-header__right { display: flex; align-items: center; gap: 0.5rem; }
.group-chevron { color: var(--text-dim); flex-shrink: 0; }
.group-name { font-size: 0.85rem; font-weight: 500; color: var(--text-primary); }
.group-desc { font-size: 0.78rem; color: var(--text-muted); }
.group-count { font-size: 0.75rem; color: var(--text-dim); }

.group-members {
  padding: 0.5rem 0.75rem 0.75rem 2rem;
  display: flex; flex-direction: column; gap: 0.4rem;
  background: var(--bg-base);
}
.group-empty { font-size: 0.78rem; color: var(--text-subtle); padding: 0.25rem 0; }
.group-member {
  display: flex; align-items: center; gap: 0.5rem;
  padding: 0.3rem 0.5rem;
  background: var(--bg-surface); border: 1px solid var(--border);
  border-radius: 0.375rem; font-size: 0.8rem;
}
.group-member__name { flex: 1; color: var(--text-secondary); font-weight: 500; }

.group-add-row {
  display: flex; align-items: center; gap: 0.5rem;
  margin-top: 0.25rem;
}
.form-select--sm { padding: 0.3rem 0.5rem; font-size: 0.78rem; flex: 1; max-width: 260px; }

.modal--sm { width: 400px; }
</style>
