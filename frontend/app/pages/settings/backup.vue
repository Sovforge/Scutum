<template>
  <SettingsShell>
    <div class="page-header">
      <div>
        <h1 class="page-title">Backup &amp; Restore</h1>
        <p class="page-sub">Create on-demand database backups and restore from a previous snapshot</p>
      </div>
      <button class="btn-primary" :disabled="creating" @click="createBackup">
        <Icon :name="creating ? 'lucide:loader-2' : 'lucide:database-backup'" size="14" :class="{ spin: creating }" />
        {{ creating ? 'Creating…' : 'Create backup' }}
      </button>
    </div>

    <div v-if="createError" class="alert alert--danger">{{ createError }}</div>

    <UiCard title="Backup history">
      <div v-if="loading" class="loading-row">Loading…</div>
      <div v-else-if="!backups.length" class="empty-state">
        <Icon name="lucide:database-backup" size="24" class="empty-state__icon" />
        <p>No backups yet. Click "Create backup" to take a snapshot now.</p>
      </div>
      <table v-else class="data-table">
        <thead>
          <tr>
            <th>Filename</th>
            <th>Driver</th>
            <th>Size</th>
            <th>Created</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="b in backups" :key="b.id" class="data-table__row">
            <td class="mono text-dim">{{ b.filename }}</td>
            <td><UiBadge variant="info">{{ b.driver }}</UiBadge></td>
            <td class="text-dim">{{ fmtSize(b.size_bytes) }}</td>
            <td class="text-dim">{{ fmtDate(b.created_at) }}</td>
            <td class="cell--actions">
              <a
                class="icon-btn"
                :href="api.downloadBackupUrl(b.id)"
                download
                title="Download"
              >
                <Icon name="lucide:download" size="13" />
              </a>

              <template v-if="pendingRestore === b.id">
                <span class="confirm-label">Restore?</span>
                <button class="icon-btn" @click="pendingRestore = null">Cancel</button>
                <button class="icon-btn icon-btn--warn" @click="confirmRestore(b.id)" :disabled="restoring">
                  {{ restoring ? '…' : 'Confirm' }}
                </button>
              </template>
              <button
                v-else
                class="icon-btn"
                title="Restore from this backup"
                @click="pendingRestore = b.id"
              >
                <Icon name="lucide:rotate-ccw" size="13" />
              </button>

              <template v-if="pendingDelete === b.id">
                <span class="confirm-label confirm-label--danger">Delete?</span>
                <button class="icon-btn" @click="pendingDelete = null">Cancel</button>
                <button class="icon-btn icon-btn--danger" @click="confirmDelete(b.id)">Confirm</button>
              </template>
              <button
                v-else
                class="icon-btn icon-btn--danger"
                title="Delete backup"
                @click="pendingDelete = b.id"
              >
                <Icon name="lucide:trash-2" size="13" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <!-- Restore result notice -->
    <div v-if="restoreResult" class="alert" :class="restoreResult.requires_restart ? 'alert--warn' : 'alert--success'">
      <Icon :name="restoreResult.requires_restart ? 'lucide:alert-triangle' : 'lucide:check-circle'" size="14" />
      {{ restoreResult.message }}
    </div>
  </SettingsShell>
</template>

<script setup lang="ts">
import type { BackupRecord } from '~/composables/useApi'

definePageMeta({ layout: 'default' })

const api = useApi()

const backups      = ref<BackupRecord[]>([])
const loading      = ref(true)
const creating     = ref(false)
const createError  = ref('')
const pendingDelete  = ref<string | null>(null)
const pendingRestore = ref<string | null>(null)
const restoring    = ref(false)
const restoreResult = ref<{ message: string; requires_restart: boolean } | null>(null)

async function load() {
  loading.value = true
  try {
    backups.value = await api.listBackups()
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function createBackup() {
  createError.value = ''
  creating.value = true
  try {
    const record = await api.createBackup()
    backups.value = [record, ...backups.value]
  } catch (e: any) {
    createError.value = e?.data?.error ?? 'Backup failed'
  } finally {
    creating.value = false
  }
}

async function confirmDelete(id: string) {
  try {
    await api.deleteBackup(id)
    backups.value = backups.value.filter(b => b.id !== id)
  } catch {}
  pendingDelete.value = null
}

async function confirmRestore(id: string) {
  restoring.value = true
  restoreResult.value = null
  try {
    const res = await api.restoreBackup(id)
    restoreResult.value = res
  } catch (e: any) {
    restoreResult.value = { message: e?.data?.error ?? 'Restore failed', requires_restart: false }
  } finally {
    restoring.value = false
    pendingRestore.value = null
  }
}

function fmtSize(bytes: number): string {
  if (bytes >= 1_073_741_824) return (bytes / 1_073_741_824).toFixed(1) + ' GB'
  if (bytes >= 1_048_576)     return (bytes / 1_048_576).toFixed(1) + ' MB'
  if (bytes >= 1024)          return (bytes / 1024).toFixed(0) + ' KB'
  return bytes + ' B'
}

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleString()
}
</script>

<style scoped>
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 1.25rem;
  gap: 1rem;
}
.page-title { margin: 0 0 0.25rem; font-size: 1.1rem; font-weight: 700; color: var(--text-primary); }
.page-sub   { margin: 0; font-size: 0.82rem; color: var(--text-muted); }

.btn-primary {
  display: inline-flex; align-items: center; gap: 0.4rem;
  background: var(--accent); border: none; border-radius: 0.375rem;
  padding: 0.45rem 1.1rem; font-size: 0.82rem; color: #fff;
  cursor: pointer; transition: background 0.15s; white-space: nowrap; flex-shrink: 0;
}
.btn-primary:hover:not(:disabled) { background: var(--accent-hover); }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.loading-row, .empty-state {
  text-align: center; padding: 2rem 1rem;
  font-size: 0.82rem; color: var(--text-muted);
}
.empty-state__icon { display: block; margin: 0 auto 0.75rem; opacity: 0.25; }

.data-table { width: 100%; border-collapse: collapse; font-size: 0.8rem; }
.data-table th {
  text-align: left; color: var(--text-dim); font-weight: 500;
  padding: 0 0.75rem 0.75rem; border-bottom: 1px solid var(--border);
}
.data-table td { padding: 0.65rem 0.75rem; border-bottom: 1px solid transparent; color: var(--text-secondary); }
.data-table__row:hover td { background: var(--hover-bg); }

.mono     { font-family: monospace; font-size: 0.75rem; }
.text-dim { color: var(--text-dim); }

.cell--actions {
  display: flex; align-items: center; gap: 0.25rem; justify-content: flex-end;
}
.confirm-label {
  font-size: 0.72rem; color: var(--text-muted); margin-right: 0.15rem;
}
.confirm-label--danger { color: var(--danger-light); }

.icon-btn {
  background: none; border: 1px solid var(--border-strong); border-radius: 0.25rem;
  color: var(--text-muted); padding: 0.25rem 0.4rem; cursor: pointer;
  display: inline-flex; align-items: center; font-size: 0.72rem;
  font-family: inherit; text-decoration: none; transition: all 0.15s;
}
.icon-btn:hover           { color: var(--text-primary); border-color: var(--border-hover); }
.icon-btn--danger:hover   { color: var(--danger-light); border-color: #7f1d1d; }
.icon-btn--warn:hover     { color: #f59e0b; border-color: #92400e; }

.alert {
  display: flex; align-items: center; gap: 0.5rem;
  border-radius: 0.5rem; padding: 0.75rem 1rem;
  font-size: 0.82rem; margin-top: 0.75rem;
}
.alert--danger  { background: #1f0a0a; border: 1px solid #7f1d1d; color: var(--danger-light); }
.alert--warn    { background: #1c1400; border: 1px solid #92400e; color: #f59e0b; }
.alert--success { background: #0a1f0a; border: 1px solid #14532d; color: #4ade80; }

.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>