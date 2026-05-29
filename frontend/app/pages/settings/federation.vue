<template>
  <div class="settings-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">Hub Federation</h1>
        <p class="page-sub">Link remote Scutum hubs so their meshes can route to each other</p>
      </div>
      <button class="btn-primary" @click="showForm = true">
        <Icon name="lucide:plus" size="14" /> Add peer
      </button>
    </div>

    <UiCard>
      <div v-if="loading" class="loading-row">Loading…</div>
      <div v-else-if="!peers.length" class="empty-state">
        <Icon name="lucide:network" size="24" class="empty-state__icon" />
        <p>No federation peers. Add a remote hub to extend your mesh.</p>
      </div>
      <table v-else class="data-table">
        <thead>
          <tr><th>Name</th><th>WireGuard endpoint</th><th>Mesh CIDR</th><th>Status</th><th>Last seen</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="p in peers" :key="p.id" class="data-table__row">
            <td class="fw-medium">{{ p.name }}</td>
            <td class="mono text-dim">{{ p.wg_endpoint }}</td>
            <td class="mono">{{ p.mesh_cidr }}</td>
            <td>
              <span class="status-dot" :class="`status-dot--${p.status}`" />
              <span class="status-label">{{ p.status }}</span>
            </td>
            <td class="text-dim">{{ p.last_seen ? fmtDate(p.last_seen) : '—' }}</td>
            <td class="cell--actions">
              <template v-if="pendingDelete === p.id">
                <span class="delete-confirm-label">Remove?</span>
                <button class="icon-btn" @click="pendingDelete = null">Cancel</button>
                <button class="icon-btn icon-btn--danger" @click="confirmDelete(p.id)">Confirm</button>
              </template>
              <button v-else class="icon-btn icon-btn--danger" @click="pendingDelete = p.id">
                <Icon name="lucide:trash-2" size="13" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <div v-if="showForm" class="modal-overlay" @click.self="closeForm">
      <div class="modal">
        <h2 class="modal__title">Add federation peer</h2>
        <div v-if="formError" class="form-error">{{ formError }}</div>
        <div class="form-grid">
          <div class="form-row"><label class="form-label">Name</label><input v-model="form.name" class="form-input" placeholder="hub-london" /></div>
          <div class="form-row"><label class="form-label">Hub API URL</label><input v-model="form.hub_url" class="form-input font-mono" placeholder="https://hub-b.example.com (optional)" /></div>
          <div class="form-row"><label class="form-label">WG endpoint</label><input v-model="form.wg_endpoint" class="form-input font-mono" placeholder="203.0.113.10:51820" /></div>
          <div class="form-row"><label class="form-label">WG public key</label><input v-model="form.wg_public_key" class="form-input font-mono" placeholder="base64 public key" /></div>
          <div class="form-row"><label class="form-label">Mesh CIDR</label><input v-model="form.mesh_cidr" class="form-input font-mono" placeholder="10.200.0.0/24" /></div>
          <div class="form-row"><label class="form-label">Allowed IPs</label><input v-model="form.allowed_ips" class="form-input font-mono" placeholder="defaults to mesh CIDR" /></div>
        </div>
        <div class="modal__footer">
          <button class="btn-ghost" @click="closeForm">Cancel</button>
          <button class="btn-primary" :disabled="saving" @click="save">
            <span v-if="saving" class="btn-spinner" />
            <span v-else>Add peer</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })
const api = useApi()
const peers = ref<any[]>([])
const loading = ref(true)
const showForm = ref(false)
const saving = ref(false)
const pendingDelete = ref<string | null>(null)
const formError = ref('')
const form = reactive({ name: '', hub_url: '', wg_endpoint: '', wg_public_key: '', mesh_cidr: '', allowed_ips: '' })

async function load() {
  loading.value = true
  peers.value = await api.listFederationPeers().catch(() => [])
  loading.value = false
}

function closeForm() {
  showForm.value = false; formError.value = ''
  Object.assign(form, { name: '', hub_url: '', wg_endpoint: '', wg_public_key: '', mesh_cidr: '', allowed_ips: '' })
}

async function save() {
  if (!form.name || !form.wg_endpoint || !form.wg_public_key || !form.mesh_cidr) {
    formError.value = 'Name, WG endpoint, WG public key, and mesh CIDR are required.'
    return
  }
  saving.value = true
  try {
    await api.createFederationPeer({ ...form })
    closeForm(); await load()
  } catch { formError.value = 'Failed to add peer.' }
  finally { saving.value = false }
}

async function confirmDelete(id: string) {
  await api.deleteFederationPeer(id).catch(() => {})
  pendingDelete.value = null; await load()
}

function fmtDate(s: string) { return new Date(s).toLocaleString() }

onMounted(load)
</script>

<style scoped>
.settings-page { padding: 1.5rem; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 1.25rem; }
.page-title { font-size: 1.15rem; font-weight: 700; color: var(--text-primary); margin: 0 0 0.2rem; }
.page-sub { font-size: 0.78rem; color: var(--text-dim); margin: 0; }
.btn-primary { display: inline-flex; align-items: center; gap: 0.4rem; background: var(--accent); color: #fff; border: none; border-radius: 0.375rem; padding: 0.5rem 0.875rem; font-size: 0.8rem; font-weight: 600; cursor: pointer; }
.btn-primary:hover:not(:disabled) { background: var(--accent-hover); }
.btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }
.btn-ghost { background: none; border: 1px solid var(--border-strong); color: var(--text-muted); border-radius: 0.375rem; padding: 0.5rem 0.875rem; font-size: 0.8rem; cursor: pointer; }
.loading-row, .empty-state { padding: 2rem; text-align: center; color: var(--text-dim); font-size: 0.875rem; }
.empty-state__icon { margin: 0 auto 0.75rem; display: block; color: var(--text-subtle); }
.data-table { width: 100%; border-collapse: collapse; font-size: 0.82rem; }
.data-table th { text-align: left; padding: 0.5rem 0.75rem; color: var(--text-dim); font-weight: 500; border-bottom: 1px solid var(--border); }
.data-table td { padding: 0.6rem 0.75rem; border-bottom: 1px solid var(--border-subtle, var(--border)); vertical-align: middle; }
.data-table__row:last-child td { border-bottom: none; }
.cell--actions { display: flex; align-items: center; gap: 0.25rem; justify-content: flex-end; }
.icon-btn { background: none; border: none; color: var(--text-dim); cursor: pointer; padding: 0.25rem; border-radius: 0.25rem; display: inline-flex; align-items: center; }
.icon-btn:hover { background: var(--hover-bg); color: var(--text-primary); }
.icon-btn--danger:hover { color: var(--danger-lighter); background: var(--danger-bg); }
.delete-confirm-label { font-size: 0.75rem; color: var(--text-dim); margin-right: 0.25rem; }
.fw-medium { font-weight: 500; }
.mono { font-family: monospace; font-size: 0.78rem; }
.text-dim { color: var(--text-dim); font-size: 0.78rem; }
.status-dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%; margin-right: 0.375rem; vertical-align: middle; }
.status-dot--connected { background: var(--success-light); }
.status-dot--pending   { background: var(--warning, #f59e0b); }
.status-dot--error     { background: var(--danger-lighter); }
.status-label { font-size: 0.78rem; color: var(--text-secondary); }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--bg-surface); border: 1px solid var(--border); border-radius: 0.75rem; padding: 1.5rem; width: 500px; max-width: 95vw; }
.modal__title { margin: 0 0 1.25rem; font-size: 1rem; font-weight: 700; color: var(--text-primary); }
.modal__footer { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 1.25rem; }
.form-grid { display: flex; flex-direction: column; gap: 0.75rem; }
.form-row { display: flex; align-items: center; gap: 1rem; }
.form-label { font-size: 0.78rem; color: var(--text-tertiary); flex-shrink: 0; width: 100px; }
.form-input { flex: 1; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 0.375rem; padding: 0.5rem 0.75rem; font-size: 0.875rem; color: var(--text-primary); font-family: inherit; outline: none; }
.form-input:focus { border-color: var(--accent); }
.form-error { background: var(--danger-bg); color: var(--danger-lighter); padding: 0.5rem 0.75rem; border-radius: 0.375rem; font-size: 0.8rem; margin-bottom: 0.75rem; }
.btn-spinner { width: 12px; height: 12px; border: 2px solid rgba(255,255,255,0.3); border-top-color: #fff; border-radius: 50%; animation: spin 0.7s linear infinite; display: inline-block; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
