<template>
  <div class="storage-page">

    <!-- Stat strip -->
    <div class="stat-strip">
      <div class="stat-card">
        <span class="stat-card__value">{{ backends.length }}</span>
        <span class="stat-card__label">Backends</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__value">{{ totalBuckets }}</span>
        <span class="stat-card__label">Buckets</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__value">{{ backends.filter(b => b._connected).length }}</span>
        <span class="stat-card__label">Connected</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__value">{{ backends.filter(b => !b._connected && b._tested).length }}</span>
        <span class="stat-card__label">Unreachable</span>
      </div>
    </div>

    <!-- Main layout -->
    <div class="storage-grid">

      <!-- Backend list -->
      <UiCard title="S3 Backends">
        <template #header-right>
          <button class="action-btn action-btn--primary" @click="showAddBackend = true">
            <Icon name="lucide:plus" size="13" /> Add Backend
          </button>
        </template>

        <div v-if="loading" class="empty-state">
          <Icon name="lucide:loader" size="20" class="spin" />
          <span>Loading…</span>
        </div>
        <div v-else-if="backends.length === 0" class="empty-state">
          <Icon name="lucide:database" size="24" />
          <span>No backends configured.</span>
        </div>
        <div v-else class="backend-list">
          <div
            v-for="b in backends"
            :key="b.id"
            class="backend-item"
            :class="{ 'backend-item--active': selectedBackend?.id === b.id }"
            @click="selectBackend(b)"
          >
            <div class="backend-item__left">
              <div class="backend-icon">
                <Icon :name="providerIcon(b.provider)" size="16" />
              </div>
              <div class="backend-info">
                <span class="backend-name">{{ b.name }}</span>
                <span class="backend-endpoint">{{ b.endpoint }}</span>
              </div>
            </div>
            <div class="backend-item__right">
              <span class="provider-badge" :class="`provider-badge--${b.provider}`">{{ providerLabel(b.provider) }}</span>
              <span class="backend-dot" :class="b._connected ? 'dot--ok' : b._tested ? 'dot--off' : 'dot--unknown'" />
            </div>
          </div>
        </div>
      </UiCard>

      <!-- Bucket list -->
      <UiCard v-if="selectedBackend" :title="selectedBackend.name + ' — Buckets'">
        <template #header-right>
          <button class="action-btn action-btn--ghost" :disabled="bucketsLoading" @click="loadBuckets(selectedBackend!)">
            <Icon name="lucide:refresh-cw" size="13" :class="{ spin: bucketsLoading }" /> Refresh
          </button>
        </template>
        <div v-if="bucketsLoading" class="empty-state">
          <Icon name="lucide:loader" size="20" class="spin" />
          <span>Loading buckets…</span>
        </div>
        <div v-else-if="bucketsError" class="error-banner">
          <Icon name="lucide:alert-circle" size="13" /> {{ bucketsError }}
        </div>
        <div v-else-if="buckets.length === 0" class="empty-state">
          <Icon name="lucide:folder" size="22" />
          <span>No buckets found.</span>
        </div>
        <table v-else class="data-table">
          <thead>
            <tr>
              <th>Bucket</th>
              <th>Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="bkt in buckets"
              :key="bkt.name"
              class="table-row"
              :class="{ 'table-row--selected': selectedBucket?.name === bkt.name }"
              @click="selectedBucket = bkt"
            >
              <td class="cell--name">
                <Icon name="lucide:folder" size="13" class="row-icon" />
                {{ bkt.name }}
              </td>
              <td class="cell--muted">{{ fmtDate(bkt.created_at) }}</td>
              <td class="cell--actions">
                <button class="icon-btn" title="Browse (coming soon)" disabled>
                  <Icon name="lucide:folder-open" size="13" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </UiCard>

      <!-- Right column: backend detail -->
      <div class="right-col">
        <UiCard v-if="selectedBackend" :title="selectedBackend.name">
          <template #header-right>
            <div class="action-group">
              <button class="action-btn action-btn--ghost" :disabled="testingId === selectedBackend.id" @click="testBackend(selectedBackend!)">
                <Icon name="lucide:plug-zap" size="13" :class="{ spin: testingId === selectedBackend.id }" /> Test
              </button>
              <button class="action-btn action-btn--danger" :disabled="deletingId === selectedBackend.id" @click="removeBackend(selectedBackend!)">
                <Icon name="lucide:trash-2" size="13" /> Remove
              </button>
            </div>
          </template>

          <div class="detail-grid">
            <div class="detail-row">
              <span class="detail-label">Provider</span>
              <span class="detail-val">
                <span class="provider-badge" :class="`provider-badge--${selectedBackend.provider}`">
                  {{ providerLabel(selectedBackend.provider) }}
                </span>
              </span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Endpoint</span>
              <span class="detail-val cell--mono">{{ selectedBackend.endpoint }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Region</span>
              <span class="detail-val cell--mono">{{ selectedBackend.region }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Access key</span>
              <span class="detail-val cell--mono">{{ selectedBackend.access_key || '—' }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Secret key</span>
              <span class="detail-val cell--mono">••••••••••••••••</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Path-style</span>
              <span class="detail-val">{{ selectedBackend.path_style ? 'Yes' : 'No' }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">TLS</span>
              <span class="detail-val">
                <Icon v-if="selectedBackend.use_ssl" name="lucide:shield-check" size="13" class="icon-ok" />
                <Icon v-else name="lucide:shield-off" size="13" class="icon-off" />
                {{ selectedBackend.use_ssl ? 'Enabled' : 'Disabled' }}
              </span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Status</span>
              <span class="detail-val" :class="selectedBackend._connected ? 'icon-ok' : selectedBackend._tested ? 'icon-off' : ''">
                {{ selectedBackend._connected ? 'Connected' : selectedBackend._tested ? 'Unreachable' : 'Not tested' }}
              </span>
            </div>
          </div>

          <div v-if="testResult && testResultId === selectedBackend.id" class="test-result" :class="`test-result--${testResult.ok ? 'ok' : 'fail'}`">
            <Icon :name="testResult.ok ? 'lucide:check-circle' : 'lucide:x-circle'" size="13" />
            {{ testResult.ok ? `Connected — ${testResult.buckets} bucket${testResult.buckets === 1 ? '' : 's'}` : testResult.error }}
          </div>
        </UiCard>

        <!-- Bucket detail -->
        <UiCard v-if="selectedBucket" :title="selectedBucket.name">
          <div class="detail-grid">
            <div class="detail-row">
              <span class="detail-label">Name</span>
              <span class="detail-val cell--mono">{{ selectedBucket.name }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Created</span>
              <span class="detail-val cell--muted">{{ fmtDate(selectedBucket.created_at) }}</span>
            </div>
          </div>
        </UiCard>
      </div>
    </div>

    <!-- Add Backend modal -->
    <div v-if="showAddBackend" class="modal-backdrop" @click.self="closeModal">
      <div class="modal">
        <div class="modal__header">
          <span>Add S3 Backend</span>
          <button class="modal__close" @click="closeModal"><Icon name="lucide:x" size="15" /></button>
        </div>
        <div class="modal__body">
          <div class="form-grid">
            <div class="form-row">
              <label class="form-label">Name</label>
              <input v-model="form.name" class="form-input" placeholder="my-minio" />
            </div>
            <div class="form-row">
              <label class="form-label">Provider</label>
              <select v-model="form.provider" class="form-select">
                <option value="minio">MinIO</option>
                <option value="r2">Cloudflare R2</option>
                <option value="aws">AWS S3</option>
                <option value="b2">Backblaze B2</option>
                <option value="ceph">Ceph RGW</option>
              </select>
            </div>
            <div class="form-row">
              <label class="form-label">Endpoint</label>
              <input v-model="form.endpoint" class="form-input font-mono" placeholder="https://s3.example.com" />
            </div>
            <div class="form-row">
              <label class="form-label">Region</label>
              <input v-model="form.region" class="form-input font-mono" placeholder="us-east-1" />
            </div>
            <div class="form-row">
              <label class="form-label">Access key ID</label>
              <input v-model="form.access_key" class="form-input font-mono" />
            </div>
            <div class="form-row">
              <label class="form-label">Secret access key</label>
              <input v-model="form.secret_key" type="password" class="form-input font-mono" />
            </div>
            <div class="form-row form-row--toggle">
              <label class="form-label">Force path-style</label>
              <label class="toggle">
                <input v-model="form.path_style" type="checkbox" />
                <span class="toggle__track"><span class="toggle__thumb" /></span>
              </label>
            </div>
            <div class="form-row form-row--toggle">
              <label class="form-label">TLS / HTTPS</label>
              <label class="toggle">
                <input v-model="form.use_ssl" type="checkbox" />
                <span class="toggle__track"><span class="toggle__thumb" /></span>
              </label>
            </div>
          </div>
          <p class="form-hint">Credentials are stored encrypted via the Scutum KMS.</p>
          <div v-if="addError" class="error-banner mt-sm">
            <Icon name="lucide:alert-circle" size="13" /> {{ addError }}
          </div>
        </div>
        <div class="modal__footer">
          <button class="cancel-btn" @click="closeModal">Cancel</button>
          <button class="save-btn" :disabled="saving || !form.name || !form.endpoint" @click="addBackend">
            <span v-if="saving" class="btn-spinner" />
            <span v-else>Add Backend</span>
          </button>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import type { StorageBackend, BucketInfo } from '~/composables/useApi'

definePageMeta({ layout: 'default' })

const api = useApi()

type BackendWithStatus = StorageBackend & { _connected: boolean; _tested: boolean }

const backends        = ref<BackendWithStatus[]>([])
const loading         = ref(false)
const selectedBackend = ref<BackendWithStatus | null>(null)
const selectedBucket  = ref<BucketInfo | null>(null)
const buckets         = ref<BucketInfo[]>([])
const bucketsLoading  = ref(false)
const bucketsError    = ref('')
const testingId       = ref('')
const deletingId      = ref('')
const testResult      = ref<{ ok: boolean; buckets?: number; error?: string } | null>(null)
const testResultId    = ref('')
const showAddBackend  = ref(false)
const saving          = ref(false)
const addError        = ref('')

const totalBuckets = computed(() => backends.value.reduce((a, b) => a + (b._connected ? 1 : 0), 0))

const form = reactive({
  name: '', provider: 'minio', endpoint: '', region: 'us-east-1',
  access_key: '', secret_key: '', path_style: true, use_ssl: true,
})

async function load() {
  loading.value = true
  try {
    const list = await api.listStorageBackends()
    backends.value = (list ?? []).map(b => ({ ...b, _connected: false, _tested: false }))
    if (backends.value.length > 0 && !selectedBackend.value) {
      selectBackend(backends.value[0]!)
    }
  } catch { /* backend may not be configured yet */ } finally {
    loading.value = false
  }
}

async function selectBackend(b: BackendWithStatus) {
  selectedBackend.value = b
  selectedBucket.value  = null
  buckets.value         = []
  bucketsError.value    = ''
  testResult.value      = null
  await loadBuckets(b)
}

async function loadBuckets(b: BackendWithStatus) {
  bucketsLoading.value = true
  bucketsError.value   = ''
  try {
    buckets.value     = await api.listStorageBuckets(b.id)
    b._connected      = true
    b._tested         = true
  } catch (e: any) {
    bucketsError.value = e?.data?.error ?? e?.message ?? 'Failed to reach backend'
    b._connected       = false
    b._tested          = true
  } finally {
    bucketsLoading.value = false
  }
}

async function testBackend(b: BackendWithStatus) {
  testingId.value    = b.id
  testResult.value   = null
  testResultId.value = b.id
  try {
    const res      = await api.testStorageBackend(b.id)
    testResult.value = res
    b._connected     = res.ok
    b._tested        = true
  } catch (e: any) {
    testResult.value = { ok: false, error: e?.data?.error ?? e?.message ?? 'Test failed' }
    b._connected     = false
    b._tested        = true
  } finally {
    testingId.value = ''
  }
}

async function removeBackend(b: BackendWithStatus) {
  if (!confirm(`Remove backend "${b.name}"? This cannot be undone.`)) return
  deletingId.value = b.id
  try {
    await api.deleteStorageBackend(b.id)
    backends.value = backends.value.filter(x => x.id !== b.id)
    if (selectedBackend.value?.id === b.id) {
      selectedBackend.value = backends.value[0] ?? null
      buckets.value         = []
      selectedBucket.value  = null
    }
  } catch (e: any) {
    alert(e?.data?.error ?? e?.message ?? 'Delete failed')
  } finally {
    deletingId.value = ''
  }
}

async function addBackend() {
  saving.value   = true
  addError.value = ''
  try {
    const b = await api.createStorageBackend({
      name:       form.name,
      provider:   form.provider,
      endpoint:   form.endpoint,
      region:     form.region || 'us-east-1',
      access_key: form.access_key,
      secret_key: form.secret_key,
      path_style: form.path_style,
      use_ssl:    form.use_ssl,
    })
    const entry: BackendWithStatus = { ...b, _connected: false, _tested: false }
    backends.value.push(entry)
    closeModal()
    selectBackend(entry)
  } catch (e: any) {
    addError.value = e?.data?.error ?? e?.message ?? 'Failed to add backend'
  } finally {
    saving.value = false
  }
}

function closeModal() {
  showAddBackend.value = false
  addError.value       = ''
  Object.assign(form, {
    name: '', provider: 'minio', endpoint: '', region: 'us-east-1',
    access_key: '', secret_key: '', path_style: true, use_ssl: true,
  })
}

function fmtDate(iso?: string): string {
  if (!iso) return '—'
  try { return new Date(iso).toLocaleDateString() } catch { return iso }
}

function providerIcon(p: string): string {
  const icons: Record<string, string> = {
    minio: 'lucide:server', r2: 'lucide:cloud', aws: 'lucide:cloud',
    b2: 'lucide:cloud', ceph: 'lucide:database',
  }
  return icons[p] ?? 'lucide:hard-drive'
}

function providerLabel(p: string): string {
  const labels: Record<string, string> = {
    minio: 'MinIO', r2: 'R2', aws: 'AWS S3', b2: 'Backblaze B2', ceph: 'Ceph RGW',
  }
  return labels[p] ?? p
}

onMounted(load)
</script>

<style scoped>
.storage-page {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.5rem;
}

/* ── Stat strip ─────────────────────────────────────────────────────────── */
.stat-strip { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1rem; }
.stat-card {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 1rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.stat-card__value { font-size: 1.5rem; font-weight: 700; color: var(--text-primary); }
.stat-card__label { font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }

/* ── Grid ───────────────────────────────────────────────────────────────── */
.storage-grid {
  display: grid;
  grid-template-columns: 280px 1fr 300px;
  gap: 1.25rem;
  align-items: start;
}
.right-col { display: flex; flex-direction: column; gap: 1.25rem; }

/* ── Empty / loading states ─────────────────────────────────────────────── */
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  gap: 0.5rem; padding: 2rem; color: var(--text-muted); font-size: 0.82rem;
}

.error-banner {
  display: flex; align-items: center; gap: 0.5rem;
  background: var(--danger-bg); border: 1px solid #7f1d1d55;
  border-radius: 0.375rem; padding: 0.6rem 0.875rem;
  font-size: 0.78rem; color: var(--danger-lighter);
}

/* ── Backend list ───────────────────────────────────────────────────────── */
.backend-list { display: flex; flex-direction: column; gap: 0.25rem; }
.backend-item {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0.625rem 0.75rem; border-radius: 0.375rem; cursor: pointer;
  transition: background 0.12s; border: 1px solid transparent;
}
.backend-item:hover { background: var(--hover-bg); }
.backend-item--active { background: var(--accent-dim); border-color: var(--accent-tint); }

.backend-item__left { display: flex; align-items: center; gap: 0.75rem; min-width: 0; }
.backend-icon {
  width: 32px; height: 32px; background: var(--bg-overlay);
  border: 1px solid var(--border-strong); border-radius: 0.375rem;
  display: flex; align-items: center; justify-content: center;
  color: var(--accent); flex-shrink: 0;
}
.backend-info { display: flex; flex-direction: column; gap: 0.1rem; min-width: 0; }
.backend-name     { font-size: 0.82rem; font-weight: 500; color: var(--text-primary); }
.backend-endpoint { font-size: 0.68rem; color: var(--text-dim); font-family: monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 160px; }

.backend-item__right { display: flex; align-items: center; gap: 0.5rem; flex-shrink: 0; }

.backend-dot { width: 6px; height: 6px; border-radius: 50%; }
.dot--ok      { background: var(--success); }
.dot--off     { background: var(--danger); }
.dot--unknown { background: var(--text-dim); }

/* ── Provider badges ────────────────────────────────────────────────────── */
.provider-badge {
  display: inline-flex; padding: 0.15rem 0.45rem; border-radius: 0.25rem;
  font-size: 0.68rem; font-weight: 600;
}
.provider-badge--minio { background: var(--role-op-bg); color: #60a5fa; border: 1px solid var(--role-op-border); }
.provider-badge--r2    { background: var(--warning-bg); color: #fb923c; border: 1px solid var(--warning-border); }
.provider-badge--aws   { background: var(--warning-bg); color: #fb923c; border: 1px solid var(--warning-border); }
.provider-badge--b2    { background: var(--role-dev-bg); color: var(--success-light); border: 1px solid var(--role-dev-border); }
.provider-badge--ceph  { background: var(--role-admin-bg); color: #c084fc; border: 1px solid var(--role-admin-border); }

/* ── Table ──────────────────────────────────────────────────────────────── */
.data-table { width: 100%; border-collapse: collapse; font-size: 0.82rem; }
.data-table th {
  text-align: left; padding: 0.5rem 0.75rem; color: var(--text-muted);
  font-weight: 500; font-size: 0.72rem; text-transform: uppercase;
  letter-spacing: 0.04em; border-bottom: 1px solid var(--border);
}
.data-table td { padding: 0.6rem 0.75rem; border-bottom: 1px solid var(--border); color: var(--text-secondary); vertical-align: middle; }
.table-row { cursor: pointer; transition: background 0.12s; }
.table-row:hover { background: var(--hover-subtle); }
.table-row--selected { background: var(--accent-dim); }
.table-row:last-child td { border-bottom: none; }

.cell--name { display: flex; align-items: center; gap: 0.5rem; color: var(--text-primary); font-weight: 500; }
.row-icon   { color: var(--text-muted); flex-shrink: 0; }
.cell--mono  { font-family: monospace; font-size: 0.78rem; color: var(--text-tertiary); }
.cell--muted { color: var(--text-muted); font-size: 0.78rem; }
.cell--actions { display: flex; gap: 0.25rem; justify-content: flex-end; }

/* ── Detail ─────────────────────────────────────────────────────────────── */
.detail-grid { display: flex; flex-direction: column; gap: 0.5rem; }
.detail-row  { display: flex; justify-content: space-between; align-items: center; gap: 0.75rem; }
.detail-label { font-size: 0.75rem; color: var(--text-muted); flex-shrink: 0; }
.detail-val   { font-size: 0.82rem; color: var(--text-secondary); display: flex; align-items: center; gap: 0.3rem; }
.icon-ok  { color: var(--success-light); }
.icon-off { color: var(--text-muted); }

.test-result {
  display: flex; align-items: center; gap: 0.5rem;
  margin-top: 1rem; padding: 0.5rem 0.75rem;
  border-radius: 0.375rem; font-size: 0.78rem; font-family: monospace;
}
.test-result--ok   { background: #15803d14; color: var(--success-light); border: 1px solid var(--role-dev-border); }
.test-result--fail { background: #7f1d1d14; color: var(--danger-light); border: 1px solid var(--danger-border); }

/* ── Action buttons ─────────────────────────────────────────────────────── */
.action-group { display: flex; gap: 0.5rem; }
.action-btn {
  display: inline-flex; align-items: center; gap: 0.35rem;
  padding: 0.3rem 0.625rem; border-radius: 0.375rem;
  font-size: 0.75rem; cursor: pointer; transition: all 0.15s;
  border: 1px solid transparent; font-family: inherit;
}
.action-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.action-btn--primary { background: var(--accent-subtle); border-color: var(--accent-soft); color: var(--accent-light); }
.action-btn--primary:hover { background: var(--accent-tint); }
.action-btn--ghost { background: none; border-color: var(--border-strong); color: var(--text-tertiary); }
.action-btn--ghost:hover:not(:disabled) { color: var(--text-primary); border-color: var(--border-hover); background: var(--hover-bg); }
.action-btn--danger { background: none; border-color: var(--danger-border); color: var(--danger-light); }
.action-btn--danger:hover:not(:disabled) { background: var(--danger-bg); }

.icon-btn {
  background: none; border: 1px solid var(--border-strong); border-radius: 0.25rem;
  color: var(--text-muted); padding: 0.25rem; cursor: pointer;
  display: flex; align-items: center; transition: all 0.15s;
}
.icon-btn:hover:not(:disabled) { color: var(--text-primary); border-color: var(--border-hover); }
.icon-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.mt-sm { margin-top: 0.75rem; }

/* ── Modal ──────────────────────────────────────────────────────────────── */
.modal-backdrop {
  position: fixed; inset: 0; background: #00000088;
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.modal {
  background: var(--bg-surface); border: 1px solid var(--border-strong);
  border-radius: 0.75rem; width: 480px; max-width: 95vw;
  display: flex; flex-direction: column; overflow: hidden;
}
.modal__header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1rem 1.25rem; border-bottom: 1px solid var(--border);
  font-size: 0.9rem; font-weight: 600; color: var(--text-primary);
}
.modal__close { background: none; border: none; color: var(--text-muted); cursor: pointer; display: flex; align-items: center; }
.modal__close:hover { color: var(--text-primary); }
.modal__body { padding: 1.25rem; display: flex; flex-direction: column; gap: 0; }
.modal__footer {
  display: flex; justify-content: flex-end; gap: 0.75rem;
  padding: 0.875rem 1.25rem; border-top: 1px solid var(--border);
}

.form-grid { display: flex; flex-direction: column; gap: 0.875rem; }
.form-row { display: grid; grid-template-columns: 140px 1fr; align-items: center; gap: 0.75rem; }
.form-row--toggle { align-items: center; }
.form-label { font-size: 0.82rem; color: var(--text-tertiary); }
.form-hint  { font-size: 0.72rem; color: var(--text-dim); margin-top: 0.75rem; }
.form-input, .form-select {
  background: var(--bg-overlay); border: 1px solid var(--border-strong); border-radius: 0.375rem;
  padding: 0.4rem 0.75rem; font-size: 0.82rem; color: var(--text-primary);
  outline: none; width: 100%; font-family: inherit;
}
.form-input:focus, .form-select:focus { border-color: var(--accent); }
.font-mono { font-family: monospace; font-size: 0.78rem; }

.toggle { display: inline-flex; align-items: center; cursor: pointer; }
.toggle input { display: none; }
.toggle__track {
  width: 36px; height: 20px; background: var(--border); border: 1px solid var(--border-strong);
  border-radius: 9999px; position: relative; transition: background 0.2s, border-color 0.2s;
}
.toggle input:checked ~ .toggle__track { background: var(--accent); border-color: var(--accent); }
.toggle__thumb {
  position: absolute; top: 2px; left: 2px; width: 14px; height: 14px;
  background: var(--text-muted); border-radius: 50%; transition: transform 0.2s, background 0.2s;
}
.toggle input:checked ~ .toggle__track .toggle__thumb { transform: translateX(16px); background: #fff; }

.save-btn {
  display: inline-flex; align-items: center; gap: 0.4rem;
  background: var(--accent); border: none; border-radius: 0.375rem;
  padding: 0.45rem 1.25rem; font-size: 0.82rem; color: #fff;
  cursor: pointer; min-width: 100px; justify-content: center;
}
.save-btn:hover:not(:disabled) { background: var(--accent-hover); }
.save-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.cancel-btn {
  background: none; border: 1px solid var(--border-strong); border-radius: 0.375rem;
  padding: 0.45rem 1rem; font-size: 0.82rem; color: var(--text-muted); cursor: pointer; font-family: inherit;
}
.cancel-btn:hover { color: var(--text-primary); border-color: var(--border-hover); }

.btn-spinner {
  width: 12px; height: 12px; border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff; border-radius: 50%; animation: spin 0.7s linear infinite;
}
.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
