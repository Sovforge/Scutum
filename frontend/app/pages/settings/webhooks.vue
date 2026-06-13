<template>
  <SettingsShell>
    <div class="page-header">
      <h1 class="page-title">Webhook Notifications</h1>
      <button class="btn-primary" @click="showForm = true">
        <Icon name="lucide:plus" size="14" /> Add webhook
      </button>
    </div>

    <UiCard>
      <div v-if="loading" class="loading-row">Loading…</div>
      <div v-else-if="!hooks.length" class="empty-state">
        <Icon name="lucide:webhook" size="24" class="empty-state__icon" />
        <p>No webhooks configured. Add one to receive event notifications.</p>
      </div>
      <table v-else class="data-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>URL</th>
            <th>Events</th>
            <th>Status</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="hook in hooks" :key="hook.id" class="data-table__row">
            <td class="fw-medium">{{ hook.name }}</td>
            <td class="mono text-dim">{{ hook.url }}</td>
            <td>
              <div class="badge-row">
                <UiBadge v-for="e in hook.events" :key="e" variant="info">{{ e }}</UiBadge>
              </div>
            </td>
            <td>
              <UiBadge :variant="hook.enabled ? 'success' : 'neutral'">
                {{ hook.enabled ? 'Enabled' : 'Disabled' }}
              </UiBadge>
            </td>
            <td class="cell--actions">
              <button class="icon-btn" title="Test delivery" @click="testHook(hook.id)">
                <Icon name="lucide:send" size="13" />
              </button>
              <button class="icon-btn" title="Edit" @click="startEdit(hook)">
                <Icon name="lucide:pencil" size="13" />
              </button>
              <template v-if="pendingDelete === hook.id">
                <span class="delete-confirm-label">Remove?</span>
                <button class="icon-btn" @click="pendingDelete = null">Cancel</button>
                <button class="icon-btn icon-btn--danger" @click="confirmDelete(hook.id)">Confirm</button>
              </template>
              <button v-else class="icon-btn icon-btn--danger" @click="pendingDelete = hook.id">
                <Icon name="lucide:trash-2" size="13" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <!-- Create / Edit form modal -->
    <div v-if="showForm" class="modal-overlay" @click.self="closeForm">
      <div class="modal">
        <h2 class="modal__title">{{ editingId ? 'Edit webhook' : 'Add webhook' }}</h2>
        <div v-if="formError" class="form-error">{{ formError }}</div>
        <div class="form-grid">
          <div class="form-row">
            <label class="form-label">Name</label>
            <input v-model="form.name" class="form-input" placeholder="Slack alerts" />
          </div>
          <div class="form-row">
            <label class="form-label">URL</label>
            <input v-model="form.url" class="form-input font-mono" placeholder="https://hooks.example.com/..." />
          </div>
          <div class="form-row">
            <label class="form-label">Secret <span class="form-hint-inline">(for HMAC signing)</span></label>
            <input v-model="form.secret" class="form-input font-mono" placeholder="optional" />
          </div>
          <div class="form-row form-row--col">
            <label class="form-label">Events</label>
            <div class="checkbox-grid">
              <label v-for="ev in allEvents" :key="ev" class="checkbox-row">
                <input type="checkbox" :value="ev" v-model="form.events" />
                <span class="mono">{{ ev }}</span>
              </label>
            </div>
          </div>
          <div class="form-row form-row--toggle">
            <label class="form-label">Enabled</label>
            <label class="toggle">
              <input v-model="form.enabled" type="checkbox" />
              <span class="toggle__track"><span class="toggle__thumb" /></span>
            </label>
          </div>
        </div>
        <div class="modal__footer">
          <button class="btn-ghost" @click="closeForm">Cancel</button>
          <button class="btn-primary" :disabled="saving" @click="save">
            <span v-if="saving" class="btn-spinner" />
            <span v-else>{{ editingId ? 'Save' : 'Create' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Toast -->
    <div v-if="toast" class="toast" :class="`toast--${toast.type}`">{{ toast.msg }}</div>
  </SettingsShell>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()

const hooks = ref<any[]>([])
const loading = ref(true)
const saving = ref(false)
const showForm = ref(false)
const editingId = ref<string | null>(null)
const pendingDelete = ref<string | null>(null)
const formError = ref('')
const toast = ref<{ msg: string; type: 'success' | 'error' } | null>(null)

const allEvents = [
  'node.enrolled', 'node.offline', 'node.online',
  'healer.service_restart', 'audit.critical', 'user.created', 'auth.sso_login',
]

const form = reactive({ name: '', url: '', secret: '', events: [] as string[], enabled: true })

async function load() {
  loading.value = true
  hooks.value = await api.listWebhooks().catch(() => [])
  loading.value = false
}

function startEdit(hook: any) {
  editingId.value = hook.id
  form.name = hook.name
  form.url = hook.url
  form.secret = ''
  form.events = [...hook.events]
  form.enabled = hook.enabled
  showForm.value = true
}

function closeForm() {
  showForm.value = false
  editingId.value = null
  formError.value = ''
  Object.assign(form, { name: '', url: '', secret: '', events: [], enabled: true })
}

async function save() {
  if (!form.name || !form.url) { formError.value = 'Name and URL are required.'; return }
  if (!form.events.length) { formError.value = 'Select at least one event.'; return }
  saving.value = true
  formError.value = ''
  try {
    if (editingId.value) {
      await api.updateWebhook(editingId.value, form)
    } else {
      await api.createWebhook(form)
    }
    showToast('Webhook saved.', 'success')
    closeForm()
    await load()
  } catch {
    formError.value = 'Failed to save webhook.'
  } finally {
    saving.value = false
  }
}

async function confirmDelete(id: string) {
  await api.deleteWebhook(id).catch(() => {})
  pendingDelete.value = null
  await load()
}

async function testHook(id: string) {
  try {
    await api.testWebhook(id)
    showToast('Test delivery sent.', 'success')
  } catch {
    showToast('Test delivery failed.', 'error')
  }
}

function showToast(msg: string, type: 'success' | 'error') {
  toast.value = { msg, type }
  setTimeout(() => { toast.value = null }, 3000)
}

onMounted(load)
</script>

<style scoped>
.settings-page { padding: 1.5rem; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.25rem; }
.page-title { font-size: 1.15rem; font-weight: 700; color: var(--text-primary); margin: 0; }
.btn-primary {
  display: inline-flex; align-items: center; gap: 0.4rem;
  background: var(--accent); color: #fff; border: none; border-radius: 0.375rem;
  padding: 0.5rem 0.875rem; font-size: 0.8rem; font-weight: 600; cursor: pointer;
}
.btn-primary:hover:not(:disabled) { background: var(--accent-hover); }
.btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }
.btn-ghost {
  background: none; border: 1px solid var(--border-strong); color: var(--text-muted);
  border-radius: 0.375rem; padding: 0.5rem 0.875rem; font-size: 0.8rem; cursor: pointer;
}
.btn-ghost:hover { background: var(--hover-bg); }
.loading-row, .empty-state { padding: 2rem; text-align: center; color: var(--text-dim); font-size: 0.875rem; }
.empty-state__icon { margin: 0 auto 0.75rem; display: block; color: var(--text-subtle); }
.data-table { width: 100%; border-collapse: collapse; font-size: 0.82rem; }
.data-table th { text-align: left; padding: 0.5rem 0.75rem; color: var(--text-dim); font-weight: 500; border-bottom: 1px solid var(--border); }
.data-table td { padding: 0.6rem 0.75rem; border-bottom: 1px solid var(--border-subtle); }
.data-table__row:last-child td { border-bottom: none; }
.cell--actions { display: flex; align-items: center; gap: 0.25rem; justify-content: flex-end; }
.icon-btn { background: none; border: none; color: var(--text-dim); cursor: pointer; padding: 0.25rem; border-radius: 0.25rem; display: flex; align-items: center; }
.icon-btn:hover { background: var(--hover-bg); color: var(--text-primary); }
.icon-btn--danger:hover { color: var(--danger-lighter); background: var(--danger-bg); }
.delete-confirm-label { font-size: 0.75rem; color: var(--text-dim); }
.badge-row { display: flex; flex-wrap: wrap; gap: 0.25rem; }
.fw-medium { font-weight: 500; }
.mono { font-family: monospace; font-size: 0.78rem; }
.text-dim { color: var(--text-dim); }
.form-hint-inline { font-size: 0.72rem; color: var(--text-subtle); font-weight: 400; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--bg-surface); border: 1px solid var(--border); border-radius: 0.75rem; padding: 1.5rem; width: 480px; max-width: 95vw; }
.modal__title { margin: 0 0 1.25rem; font-size: 1rem; font-weight: 700; color: var(--text-primary); }
.modal__footer { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 1.25rem; }
.form-grid { display: flex; flex-direction: column; gap: 0.875rem; }
.form-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.form-row--col { flex-direction: column; align-items: flex-start; }
.form-row--toggle { justify-content: space-between; }
.form-label { font-size: 0.78rem; color: var(--text-tertiary); flex-shrink: 0; }
.form-input { flex: 1; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 0.375rem; padding: 0.5rem 0.75rem; font-size: 0.875rem; color: var(--text-primary); font-family: inherit; outline: none; }
.form-input:focus { border-color: var(--accent); }
.form-error { background: var(--danger-bg); color: var(--danger-lighter); padding: 0.5rem 0.75rem; border-radius: 0.375rem; font-size: 0.8rem; margin-bottom: 0.75rem; }
.checkbox-grid { display: flex; flex-direction: column; gap: 0.375rem; width: 100%; }
.checkbox-row { display: flex; align-items: center; gap: 0.5rem; font-size: 0.8rem; cursor: pointer; color: var(--text-secondary); }
.toggle { position: relative; display: inline-block; }
.toggle input { opacity: 0; width: 0; height: 0; }
.toggle__track { display: block; width: 36px; height: 20px; background: var(--border-strong); border-radius: 10px; position: relative; cursor: pointer; transition: background 0.2s; }
.toggle input:checked + .toggle__track { background: var(--accent); }
.toggle__thumb { position: absolute; top: 2px; left: 2px; width: 16px; height: 16px; background: #fff; border-radius: 50%; transition: transform 0.2s; }
.toggle input:checked + .toggle__track .toggle__thumb { transform: translateX(16px); }
.btn-spinner { width: 12px; height: 12px; border: 2px solid rgba(255,255,255,0.3); border-top-color: #fff; border-radius: 50%; animation: spin 0.7s linear infinite; display: inline-block; }
@keyframes spin { to { transform: rotate(360deg); } }
.toast { position: fixed; bottom: 1.5rem; right: 1.5rem; padding: 0.625rem 1rem; border-radius: 0.375rem; font-size: 0.82rem; font-weight: 500; z-index: 200; }
.toast--success { background: var(--success-bg); color: var(--success-light); }
.toast--error { background: var(--danger-bg); color: var(--danger-lighter); }
</style>
