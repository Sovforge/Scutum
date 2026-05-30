<template>
  <div class="settings-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">SCIM Provisioning</h1>
        <p class="page-sub">Manage tokens for IdP user provisioning (Entra ID, Okta, etc.)</p>
      </div>
      <button class="btn-primary" @click="showCreate = true">
        <Icon name="lucide:plus" size="14" /> New token
      </button>
    </div>

    <UiCard>
      <div v-if="loading" class="loading-row">Loading…</div>
      <div v-else-if="!tokens.length" class="empty-state">
        <Icon name="lucide:key" size="24" class="empty-state__icon" />
        <p>No SCIM tokens yet. Create one to connect your identity provider.</p>
      </div>
      <table v-else class="data-table">
        <thead>
          <tr><th>Description</th><th>Created</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="t in tokens" :key="t.id" class="data-table__row">
            <td>{{ t.description || '—' }}</td>
            <td class="text-dim">{{ formatDate(t.created_at) }}</td>
            <td class="cell--actions">
              <template v-if="pendingRevoke === t.id">
                <span class="delete-confirm-label">Revoke?</span>
                <button class="icon-btn" @click="pendingRevoke = null">Cancel</button>
                <button class="icon-btn icon-btn--danger" @click="revoke(t.id)">Confirm</button>
              </template>
              <button v-else class="icon-btn icon-btn--danger" @click="pendingRevoke = t.id">
                <Icon name="lucide:trash-2" size="13" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <UiCard title="SCIM Endpoint" class="mt">
      <div class="endpoint-block">
        <code class="endpoint-url">{{ scimEndpoint }}</code>
        <button class="icon-btn" @click="copy(scimEndpoint)">
          <Icon name="lucide:copy" size="13" />
        </button>
      </div>
      <p class="info-note">Use this URL and a SCIM token when configuring provisioning in your identity provider.</p>
    </UiCard>

    <!-- Create token modal -->
    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h2 class="modal__title">Create SCIM token</h2>
        <div class="form-grid">
          <div class="form-row">
            <label class="form-label">Description</label>
            <input v-model="newDesc" class="form-input" placeholder="Entra ID provisioning" />
          </div>
        </div>
        <div class="modal__footer">
          <button class="btn-ghost" @click="showCreate = false">Cancel</button>
          <button class="btn-primary" :disabled="creating" @click="create">
            <span v-if="creating" class="btn-spinner" />
            <span v-else>Generate</span>
          </button>
        </div>
      </div>
    </div>

    <!-- One-time token reveal modal -->
    <div v-if="newToken" class="modal-overlay">
      <div class="modal">
        <h2 class="modal__title">
          <Icon name="lucide:shield-check" size="16" class="success-icon" /> Token created
        </h2>
        <p class="token-warning">Copy this token now — it will not be shown again.</p>
        <div class="token-reveal">
          <code class="token-val">{{ newToken }}</code>
          <button class="icon-btn" @click="copy(newToken)"><Icon name="lucide:copy" size="13" /></button>
        </div>
        <div class="modal__footer">
          <button class="btn-primary" @click="newToken = ''">Done</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()
const tokens = ref<any[]>([])
const loading = ref(true)
const showCreate = ref(false)
const creating = ref(false)
const newDesc = ref('')
const newToken = ref('')
const pendingRevoke = ref<string | null>(null)

const scimEndpoint = computed(() => {
  if (process.client) return `${window.location.origin}/scim/v2`
  return '/scim/v2'
})

async function load() {
  loading.value = true
  tokens.value = await api.listSCIMTokens().catch(() => [])
  loading.value = false
}

async function create() {
  creating.value = true
  try {
    const res = await api.createSCIMToken(newDesc.value)
    newToken.value = res.token
    showCreate.value = false
    newDesc.value = ''
    await load()
  } finally {
    creating.value = false
  }
}

async function revoke(id: string) {
  await api.deleteSCIMToken(id).catch(() => {})
  pendingRevoke.value = null
  await load()
}

function copy(text: string) {
  navigator.clipboard.writeText(text).catch(() => {})
}

function formatDate(s: string) {
  return new Date(s).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

onMounted(load)
</script>

<style scoped>
.settings-page { padding: 1.5rem; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 1.25rem; }
.page-title { font-size: 1.15rem; font-weight: 700; color: var(--text-primary); margin: 0 0 0.2rem; }
.page-sub { font-size: 0.78rem; color: var(--text-dim); margin: 0; }
.mt { margin-top: 1rem; }
.btn-primary { display: inline-flex; align-items: center; gap: 0.4rem; background: var(--accent); color: #fff; border: none; border-radius: 0.375rem; padding: 0.5rem 0.875rem; font-size: 0.8rem; font-weight: 600; cursor: pointer; }
.btn-primary:hover:not(:disabled) { background: var(--accent-hover); }
.btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }
.btn-ghost { background: none; border: 1px solid var(--border-strong); color: var(--text-muted); border-radius: 0.375rem; padding: 0.5rem 0.875rem; font-size: 0.8rem; cursor: pointer; }
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
.text-dim { color: var(--text-dim); font-size: 0.78rem; }
.endpoint-block { display: flex; align-items: center; gap: 0.5rem; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 0.375rem; padding: 0.625rem 0.875rem; }
.endpoint-url { font-family: monospace; font-size: 0.82rem; color: var(--text-primary); flex: 1; overflow: hidden; text-overflow: ellipsis; }
.info-note { font-size: 0.75rem; color: var(--text-dim); margin: 0.75rem 0 0; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--bg-surface); border: 1px solid var(--border); border-radius: 0.75rem; padding: 1.5rem; width: 420px; max-width: 95vw; }
.modal__title { margin: 0 0 1.25rem; font-size: 1rem; font-weight: 700; color: var(--text-primary); display: flex; align-items: center; gap: 0.5rem; }
.modal__footer { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 1.25rem; }
.success-icon { color: var(--success-light); }
.token-warning { font-size: 0.8rem; color: var(--warning); margin: 0 0 0.875rem; }
.token-reveal { display: flex; align-items: center; gap: 0.5rem; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 0.375rem; padding: 0.625rem 0.875rem; }
.token-val { font-family: monospace; font-size: 0.78rem; color: var(--text-primary); flex: 1; word-break: break-all; }
.form-grid { display: flex; flex-direction: column; gap: 0.875rem; }
.form-row { display: flex; align-items: center; gap: 1rem; }
.form-label { font-size: 0.78rem; color: var(--text-tertiary); flex-shrink: 0; width: 90px; }
.form-input { flex: 1; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 0.375rem; padding: 0.5rem 0.75rem; font-size: 0.875rem; color: var(--text-primary); font-family: inherit; outline: none; }
.form-input:focus { border-color: var(--accent); }
.btn-spinner { width: 12px; height: 12px; border: 2px solid rgba(255,255,255,0.3); border-top-color: #fff; border-radius: 50%; animation: spin 0.7s linear infinite; display: inline-block; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
