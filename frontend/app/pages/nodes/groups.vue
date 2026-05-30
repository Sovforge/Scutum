<template>
  <div class="nodes-page">
    <div class="page-header">
      <h1 class="page-title">Node Groups</h1>
      <button class="btn-primary" @click="showCreate = true">
        <Icon name="lucide:plus" size="14" /> New group
      </button>
    </div>

    <div v-if="loading" class="loading-row">Loading…</div>
    <div v-else-if="!groups.length" class="empty-state">
      <Icon name="lucide:layers" size="24" class="empty-state__icon" />
      <p>No groups yet. Create one to organise nodes by environment, region, or role.</p>
    </div>

    <div v-else class="group-list">
      <UiCard v-for="g in groups" :key="g.id" class="group-card">
        <div class="group-card__header">
          <div>
            <span class="group-name">{{ g.name }}</span>
            <span v-if="g.description" class="group-desc">{{ g.description }}</span>
          </div>
          <div class="group-actions">
            <button class="icon-btn" :title="expanded === g.id ? 'Collapse' : 'Expand'" @click="toggleExpand(g.id)">
              <Icon :name="expanded === g.id ? 'lucide:chevron-up' : 'lucide:chevron-down'" size="14" />
            </button>
            <template v-if="pendingDelete === g.id">
              <span class="delete-confirm-label">Delete?</span>
              <button class="icon-btn" @click="pendingDelete = null">Cancel</button>
              <button class="icon-btn icon-btn--danger" @click="deleteGroup(g.id)">Confirm</button>
            </template>
            <button v-else class="icon-btn icon-btn--danger" @click="pendingDelete = g.id">
              <Icon name="lucide:trash-2" size="13" />
            </button>
          </div>
        </div>

        <div v-if="expanded === g.id" class="group-members">
          <div v-if="membersLoading[g.id]" class="members-loading">Loading members…</div>
          <div v-else>
            <div v-if="!groupNodes[g.id]?.length" class="members-empty">No members. Add nodes below.</div>
            <div v-else class="members-table">
              <div v-for="n in groupNodes[g.id]" :key="n.id" class="member-row">
                <span class="member-name">{{ n.name }}</span>
                <UiBadge variant="info">{{ n.type }}</UiBadge>
                <span class="member-addr mono">{{ n.address }}</span>
                <button class="icon-btn icon-btn--danger" title="Remove from group" @click="removeMember(g.id, n.id)">
                  <Icon name="lucide:x" size="12" />
                </button>
              </div>
            </div>
            <div class="add-member-row">
              <select v-model="addNodeId[g.id]" class="form-select-sm">
                <option value="">— add node —</option>
                <option v-for="n in availableNodes(g.id)" :key="n.id" :value="n.id">{{ n.name }}</option>
              </select>
              <button class="btn-sm-primary" :disabled="!addNodeId[g.id]" @click="addMember(g.id)">Add</button>
            </div>
          </div>
        </div>

        <div class="group-card__footer">
          <span class="member-count">{{ g.members?.length ?? 0 }} node{{ g.members?.length !== 1 ? 's' : '' }}</span>
        </div>
      </UiCard>
    </div>

    <!-- Create group modal -->
    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h2 class="modal__title">New group</h2>
        <div class="form-grid">
          <div class="form-row"><label class="form-label">Name</label><input v-model="newForm.name" class="form-input" placeholder="prod-eu" /></div>
          <div class="form-row"><label class="form-label">Description</label><input v-model="newForm.desc" class="form-input" placeholder="optional" /></div>
        </div>
        <div class="modal__footer">
          <button class="btn-ghost" @click="showCreate = false">Cancel</button>
          <button class="btn-primary" @click="createGroup">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })
const api = useApi()

const groups = ref<any[]>([])
const allNodes = ref<any[]>([])
const loading = ref(true)
const showCreate = ref(false)
const expanded = ref<string | null>(null)
const pendingDelete = ref<string | null>(null)
const groupNodes = ref<Record<string, any[]>>({})
const membersLoading = ref<Record<string, boolean>>({})
const addNodeId = ref<Record<string, string>>({})
const newForm = reactive({ name: '', desc: '' })

async function load() {
  loading.value = true
  const [g, n] = await Promise.all([
    api.listNodeGroups().catch(() => []),
    api.listNodes().catch(() => []),
  ])
  groups.value = g
  allNodes.value = n
  loading.value = false
}

async function toggleExpand(id: string) {
  if (expanded.value === id) { expanded.value = null; return }
  expanded.value = id
  if (!groupNodes.value[id]) {
    membersLoading.value[id] = true
    groupNodes.value[id] = await api.getGroupNodes(id).catch(() => [])
    membersLoading.value[id] = false
  }
}

function availableNodes(groupId: string) {
  const memberIds = new Set((groupNodes.value[groupId] ?? []).map((n: any) => n.id))
  return allNodes.value.filter(n => !memberIds.has(n.id))
}

async function addMember(groupId: string) {
  const nodeId = addNodeId.value[groupId]
  if (!nodeId) return
  await api.addNodeToGroup(groupId, nodeId).catch(() => {})
  addNodeId.value[groupId] = ''
  groupNodes.value[groupId] = await api.getGroupNodes(groupId).catch(() => [])
  await load()
}

async function removeMember(groupId: string, nodeId: string) {
  await api.removeNodeFromGroup(groupId, nodeId).catch(() => {})
  groupNodes.value[groupId] = await api.getGroupNodes(groupId).catch(() => [])
  await load()
}

async function createGroup() {
  if (!newForm.name) return
  await api.createNodeGroup({ name: newForm.name, description: newForm.desc }).catch(() => {})
  showCreate.value = false
  Object.assign(newForm, { name: '', desc: '' })
  await load()
}

async function deleteGroup(id: string) {
  await api.deleteNodeGroup(id).catch(() => {})
  pendingDelete.value = null
  if (expanded.value === id) expanded.value = null
  await load()
}

onMounted(load)
</script>

<style scoped>
.nodes-page { padding: 1.5rem; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.25rem; }
.page-title { font-size: 1.15rem; font-weight: 700; color: var(--text-primary); margin: 0; }
.btn-primary { display: inline-flex; align-items: center; gap: 0.4rem; background: var(--accent); color: #fff; border: none; border-radius: 0.375rem; padding: 0.5rem 0.875rem; font-size: 0.8rem; font-weight: 600; cursor: pointer; }
.btn-primary:hover:not(:disabled) { background: var(--accent-hover); }
.btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }
.btn-ghost { background: none; border: 1px solid var(--border-strong); color: var(--text-muted); border-radius: 0.375rem; padding: 0.5rem 0.875rem; font-size: 0.8rem; cursor: pointer; }
.btn-sm-primary { background: var(--accent); color: #fff; border: none; border-radius: 0.3rem; padding: 0.375rem 0.75rem; font-size: 0.78rem; font-weight: 600; cursor: pointer; }
.btn-sm-primary:hover:not(:disabled) { background: var(--accent-hover); }
.btn-sm-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.loading-row, .empty-state { padding: 2rem; text-align: center; color: var(--text-dim); font-size: 0.875rem; }
.empty-state__icon { margin: 0 auto 0.75rem; display: block; color: var(--text-subtle); }
.group-list { display: flex; flex-direction: column; gap: 0.875rem; }
.group-card { padding: 1rem 1.25rem; }
.group-card__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.group-name { font-size: 0.9rem; font-weight: 600; color: var(--text-primary); }
.group-desc { font-size: 0.78rem; color: var(--text-dim); margin-left: 0.5rem; }
.group-actions { display: flex; align-items: center; gap: 0.25rem; }
.group-card__footer { margin-top: 0.5rem; }
.member-count { font-size: 0.75rem; color: var(--text-dim); }
.group-members { margin-top: 0.875rem; padding-top: 0.875rem; border-top: 1px solid var(--border); }
.members-loading, .members-empty { font-size: 0.8rem; color: var(--text-dim); padding: 0.5rem 0; }
.members-table { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 0.75rem; }
.member-row { display: flex; align-items: center; gap: 0.75rem; padding: 0.375rem 0.5rem; border-radius: 0.3rem; background: var(--bg-elevated); }
.member-name { font-size: 0.82rem; font-weight: 500; color: var(--text-primary); flex: 1; }
.member-addr { font-size: 0.75rem; color: var(--text-dim); }
.add-member-row { display: flex; align-items: center; gap: 0.5rem; }
.form-select-sm { background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 0.3rem; padding: 0.35rem 0.625rem; font-size: 0.8rem; color: var(--text-primary); outline: none; }
.icon-btn { background: none; border: none; color: var(--text-dim); cursor: pointer; padding: 0.25rem; border-radius: 0.25rem; display: inline-flex; align-items: center; }
.icon-btn:hover { background: var(--hover-bg); color: var(--text-primary); }
.icon-btn--danger:hover { color: var(--danger-lighter); background: var(--danger-bg); }
.delete-confirm-label { font-size: 0.75rem; color: var(--text-dim); }
.mono { font-family: monospace; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--bg-surface); border: 1px solid var(--border); border-radius: 0.75rem; padding: 1.5rem; width: 420px; max-width: 95vw; }
.modal__title { margin: 0 0 1.25rem; font-size: 1rem; font-weight: 700; color: var(--text-primary); }
.modal__footer { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 1.25rem; }
.form-grid { display: flex; flex-direction: column; gap: 0.75rem; }
.form-row { display: flex; align-items: center; gap: 1rem; }
.form-label { font-size: 0.78rem; color: var(--text-tertiary); flex-shrink: 0; width: 90px; }
.form-input { flex: 1; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 0.375rem; padding: 0.5rem 0.75rem; font-size: 0.875rem; color: var(--text-primary); font-family: inherit; outline: none; }
.form-input:focus { border-color: var(--accent); }
</style>
