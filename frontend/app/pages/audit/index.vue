<template>
  <div class="audit-page">

    <div class="page-header">
      <div>
        <h2 class="page-title">Audit Log</h2>
        <p class="page-sub">All privileged actions performed in this installation.</p>
      </div>
      <div class="header-actions">
        <a :href="api.auditExportUrl('csv')" download class="action-btn">
          <Icon name="lucide:download" size="13" /> Export CSV
        </a>
        <a :href="api.auditExportUrl('json')" download class="action-btn">
          <Icon name="lucide:file-json" size="13" /> Export JSON
        </a>
        <div class="report-group">
          <span class="report-label">CRA Report</span>
          <button class="action-btn action-btn--report" @click="api.downloadComplianceReport('json')">JSON</button>
          <button class="action-btn action-btn--report" @click="api.downloadComplianceReport('csv')">CSV</button>
          <button class="action-btn action-btn--report" @click="api.downloadComplianceReport('text')">Text</button>
        </div>
        <button class="action-btn" :disabled="loading" @click="load">
          <Icon name="lucide:refresh-cw" size="13" :class="{ spin: loading }" /> Refresh
        </button>
      </div>
    </div>

    <!-- Filters -->
    <div class="filter-bar">
      <div class="search-wrap">
        <Icon name="lucide:search" size="13" class="search-icon" />
        <input v-model="query" class="search-input" placeholder="Filter by action, path, IP…" />
      </div>
      <select v-model="methodFilter" class="select-input">
        <option value="">All methods</option>
        <option value="GET">GET</option>
        <option value="POST">POST</option>
        <option value="PUT">PUT</option>
        <option value="DELETE">DELETE</option>
      </select>
    </div>

    <div v-if="error" class="error-banner">
      <Icon name="lucide:alert-circle" size="13" /> {{ error }}
    </div>

    <UiCard>
      <div v-if="loading" class="loading-state">
        <Icon name="lucide:loader-circle" size="20" class="spin" />
        <span>Loading audit log…</span>
      </div>
      <div v-else-if="filtered.length === 0" class="empty-state">
        <Icon name="lucide:shield-check" size="28" />
        <p>No audit entries found.</p>
      </div>
      <table v-else class="audit-table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Action</th>
            <th>Method</th>
            <th>Path</th>
            <th>Client IP</th>
            <th>Details</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(e, i) in filtered" :key="i" class="audit-row" @click="selected = selected === e ? null : e">
            <td class="cell--time">{{ formatTime(e.time) }}</td>
            <td><span class="action-badge" :class="actionClass(e.action)">{{ e.action }}</span></td>
            <td class="cell--method">{{ e.method }}</td>
            <td class="cell--path cell--mono">{{ e.path }}</td>
            <td class="cell--muted cell--mono">{{ e.client_ip }}</td>
            <td class="cell--extras">
              <span v-for="(v, k) in e.extra" :key="k" class="extra-chip">
                {{ k }}={{ v }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <!-- Detail panel -->
    <UiCard v-if="selected" title="Entry detail">
      <div class="detail-grid">
        <div class="detail-row"><span class="detail-label">Time</span><span class="detail-val">{{ selected.time }}</span></div>
        <div class="detail-row"><span class="detail-label">Action</span><span class="detail-val">{{ selected.action }}</span></div>
        <div class="detail-row"><span class="detail-label">Method</span><span class="detail-val">{{ selected.method }}</span></div>
        <div class="detail-row"><span class="detail-label">Path</span><span class="detail-val cell--mono">{{ selected.path }}</span></div>
        <div class="detail-row"><span class="detail-label">Client IP</span><span class="detail-val cell--mono">{{ selected.client_ip }}</span></div>
        <div v-if="selected.trace_id" class="detail-row"><span class="detail-label">Trace ID</span><span class="detail-val cell--mono">{{ selected.trace_id }}</span></div>
        <template v-for="(v, k) in selected.extra" :key="k">
          <div class="detail-row"><span class="detail-label">{{ k }}</span><span class="detail-val cell--mono">{{ v }}</span></div>
        </template>
      </div>
    </UiCard>

  </div>
</template>

<script setup lang="ts">
import type { AuditEntry } from '~/composables/useApi'

definePageMeta({ layout: 'default' })

const api = useApi()

const entries = ref<AuditEntry[]>([])
const loading = ref(false)
const error   = ref('')
const query   = ref('')
const methodFilter = ref('')
const selected = ref<AuditEntry | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    entries.value = await api.listAuditLogs()
  } catch (e: any) {
    error.value = e?.data ?? e?.message ?? 'Failed to load audit log'
  } finally {
    loading.value = false
  }
}

const filtered = computed(() =>
  [...entries.value].reverse().filter(e => {
    if (methodFilter.value && e.method !== methodFilter.value) return false
    if (query.value) {
      const q = query.value.toLowerCase()
      const haystack = `${e.action} ${e.path} ${e.client_ip} ${JSON.stringify(e.extra ?? {})}`.toLowerCase()
      if (!haystack.includes(q)) return false
    }
    return true
  })
)

function formatTime(iso: string): string {
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

function actionClass(action: string): string {
  if (action.includes('FAIL') || action.includes('ERROR') || action.includes('DELETE')) return 'action-badge--danger'
  if (action.includes('LOGIN') || action.includes('REGISTER')) return 'action-badge--info'
  return 'action-badge--default'
}

onMounted(load)
</script>

<style scoped>
.audit-page { display: flex; flex-direction: column; gap: 1.25rem; padding: 1.5rem; }

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}
.header-actions { display: flex; align-items: center; gap: 0.5rem; }
.page-title { margin: 0; font-size: 1.1rem; font-weight: 700; color: var(--text-primary); }
.page-sub   { margin: 0.2rem 0 0; font-size: 0.78rem; color: var(--text-dim); }

.filter-bar { display: flex; align-items: center; gap: 0.75rem; }
.search-wrap { position: relative; display: flex; align-items: center; }
.search-icon { position: absolute; left: 0.5rem; color: var(--text-muted); pointer-events: none; }
.search-input {
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.35rem 0.75rem 0.35rem 2rem;
  font-size: 0.8rem;
  color: var(--text-primary);
  width: 260px;
  outline: none;
}
.search-input:focus { border-color: var(--accent); }
.select-input {
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.35rem 0.5rem;
  font-size: 0.8rem;
  color: var(--text-primary);
  outline: none;
}

.action-btn {
  display: inline-flex; align-items: center; gap: 0.4rem;
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 0.375rem; padding: 0.4rem 0.875rem;
  font-size: 0.8rem; color: var(--text-primary); cursor: pointer;
}
.action-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.error-banner {
  display: flex; align-items: center; gap: 0.5rem;
  background: var(--danger-bg); border: 1px solid #7f1d1d55;
  border-radius: 0.375rem; padding: 0.6rem 0.875rem;
  font-size: 0.8rem; color: var(--danger-lighter);
}

.loading-state, .empty-state {
  display: flex; flex-direction: column; align-items: center;
  gap: 0.5rem; padding: 2rem; color: var(--text-muted); font-size: 0.85rem;
}

.audit-table { width: 100%; border-collapse: collapse; font-size: 0.8rem; }
.audit-table th {
  text-align: left; padding: 0.45rem 0.75rem;
  color: var(--text-muted); font-weight: 500; font-size: 0.72rem;
  text-transform: uppercase; letter-spacing: 0.04em;
  border-bottom: 1px solid var(--border);
}
.audit-row { cursor: pointer; transition: background 0.1s; }
.audit-row:hover { background: var(--hover-bg); }
.audit-row td { padding: 0.45rem 0.75rem; border-bottom: 1px solid var(--border); vertical-align: top; }

.cell--time   { color: var(--text-dim); white-space: nowrap; font-size: 0.75rem; }
.cell--method { color: var(--text-tertiary); font-family: monospace; }
.cell--path   { max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cell--muted  { color: var(--text-muted); }
.cell--mono   { font-family: monospace; font-size: 0.78rem; }
.cell--extras { display: flex; flex-wrap: wrap; gap: 0.25rem; }

.action-badge {
  display: inline-block; padding: 0.1rem 0.45rem;
  border-radius: 0.25rem; font-size: 0.7rem; font-weight: 600;
  font-family: monospace;
}
.action-badge--danger  { background: var(--danger-bg);  color: var(--danger-lighter); }
.action-badge--info    { background: var(--accent-subtle); color: var(--accent-light); }
.action-badge--default { background: var(--bg-elevated); color: var(--text-tertiary); border: 1px solid var(--border); }

.extra-chip {
  background: var(--bg-overlay); border: 1px solid var(--border);
  border-radius: 0.2rem; padding: 0.05rem 0.35rem;
  font-size: 0.68rem; font-family: monospace; color: var(--text-dim);
}

.detail-grid { display: flex; flex-direction: column; gap: 0.25rem; }
.detail-row  { display: flex; gap: 1rem; font-size: 0.82rem; padding: 0.2rem 0; }
.detail-label { width: 90px; flex-shrink: 0; color: var(--text-muted); font-size: 0.75rem; }
.detail-val   { color: var(--text-primary); }

.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.report-group { display: flex; align-items: center; gap: 0.25rem; padding-left: 0.5rem; border-left: 1px solid var(--border); }
.report-label { font-size: 0.72rem; color: var(--text-dim); margin-right: 0.25rem; white-space: nowrap; }
.action-btn--report { padding: 0.35rem 0.5rem; font-size: 0.72rem; }
</style>
