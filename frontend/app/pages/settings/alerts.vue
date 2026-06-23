<template>
  <SettingsShell>
    <div class="page-header">
      <div>
        <h1 class="page-title">Alert Rules</h1>
        <p class="page-sub">Define conditions that fire webhook notifications and appear in the event log</p>
      </div>
      <button class="btn-primary" @click="openCreate">
        <Icon name="lucide:plus" size="14" />
        New rule
      </button>
    </div>

    <div v-if="saveError" class="alert alert--danger">{{ saveError }}</div>

    <!-- Rule list -->
    <UiCard title="Rules">
      <div v-if="loadingRules" class="loading-row">Loading…</div>
      <div v-else-if="!rules.length" class="empty-state">
        <Icon name="lucide:bell-off" size="24" class="empty-state__icon" />
        <p>No alert rules yet. Add one to start monitoring thresholds.</p>
      </div>
      <table v-else class="data-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Condition</th>
            <th>Threshold</th>
            <th>Severity</th>
            <th>Status</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in rules" :key="r.id" class="data-table__row">
            <td>{{ r.name }}</td>
            <td class="mono text-dim">{{ r.condition }}</td>
            <td class="text-dim">{{ fmtThreshold(r) }}</td>
            <td>
              <UiBadge :variant="severityVariant(r.severity)">{{ r.severity }}</UiBadge>
            </td>
            <td>
              <span v-if="isSilenced(r)" class="badge badge--muted">silenced</span>
              <span v-else-if="r.enabled" class="badge badge--success">active</span>
              <span v-else class="badge badge--muted">disabled</span>
            </td>
            <td class="cell--actions">
              <button class="icon-btn" title="Edit" @click="openEdit(r)">
                <Icon name="lucide:pencil" size="14" />
              </button>
              <button class="icon-btn" title="Silence 4 h" @click="silence(r, 4)">
                <Icon name="lucide:bell-minus" size="14" />
              </button>
              <button
                class="icon-btn"
                :title="r.enabled ? 'Disable' : 'Enable'"
                @click="toggleEnabled(r)"
              >
                <Icon :name="r.enabled ? 'lucide:toggle-right' : 'lucide:toggle-left'" size="14" />
              </button>
              <button
                v-if="confirmDelete !== r.id"
                class="icon-btn icon-btn--danger"
                title="Delete"
                @click="confirmDelete = r.id"
              >
                <Icon name="lucide:trash-2" size="14" />
              </button>
              <template v-else>
                <button class="icon-btn icon-btn--danger" @click="deleteRule(r.id)">Confirm</button>
                <button class="icon-btn" @click="confirmDelete = null">Cancel</button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <!-- Event log -->
    <UiCard title="Recent events" class="mt-6">
      <div v-if="loadingEvents" class="loading-row">Loading…</div>
      <div v-else-if="!events.length" class="empty-state">
        <Icon name="lucide:bell" size="24" class="empty-state__icon" />
        <p>No alert events recorded yet.</p>
      </div>
      <table v-else class="data-table">
        <thead>
          <tr>
            <th>Rule</th>
            <th>Severity</th>
            <th>Message</th>
            <th>Fired</th>
            <th>Resolved</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="e in events"
            :key="e.id"
            :class="['data-table__row', { 'row--open': !e.resolved_at }]"
          >
            <td>{{ e.rule_name }}</td>
            <td>
              <UiBadge :variant="severityVariant(e.severity)">{{ e.severity }}</UiBadge>
            </td>
            <td class="text-dim">{{ e.message }}</td>
            <td class="text-dim">{{ fmtDate(e.fired_at) }}</td>
            <td class="text-dim">{{ e.resolved_at ? fmtDate(e.resolved_at) : '—' }}</td>
            <td class="cell--actions">
              <button
                v-if="!e.acknowledged_at"
                class="icon-btn"
                title="Acknowledge"
                @click="acknowledge(e.id)"
              >
                <Icon name="lucide:check" size="14" />
              </button>
              <Icon v-else name="lucide:check-check" size="14" class="text-dim" title="Acknowledged" />
            </td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <!-- Create / Edit modal -->
    <Teleport to="body">
      <div v-if="showForm" class="modal-backdrop" @click.self="closeForm">
        <div class="modal">
          <div class="modal__header">
            <h2>{{ editing ? 'Edit rule' : 'New alert rule' }}</h2>
            <button class="icon-btn" @click="closeForm"><Icon name="lucide:x" size="16" /></button>
          </div>
          <div class="modal__body">
            <div class="form-group">
              <label>Name</label>
              <input v-model="form.name" class="input" placeholder="High CPU" />
            </div>
            <div class="form-group">
              <label>Condition</label>
              <select v-model="form.condition" class="input">
                <option value="cpu_percent">CPU %</option>
                <option value="mem_percent">Memory %</option>
                <option value="disk_percent">Disk %</option>
                <option value="node_offline">Node offline (minutes since last handshake)</option>
                <option value="handshake_age">WireGuard handshake age (minutes)</option>
              </select>
            </div>
            <div class="form-group">
              <label>{{ thresholdLabel }}</label>
              <input v-model.number="form.threshold" type="number" min="0" max="100" class="input" />
            </div>
            <div class="form-group">
              <label>Severity</label>
              <select v-model="form.severity" class="input">
                <option value="info">Info</option>
                <option value="warning">Warning</option>
                <option value="critical">Critical</option>
              </select>
            </div>
            <div class="form-group form-group--inline">
              <label>
                <input v-model="form.enabled" type="checkbox" />
                Enabled
              </label>
            </div>
          </div>
          <div class="modal__footer">
            <button class="btn-secondary" @click="closeForm">Cancel</button>
            <button class="btn-primary" :disabled="saving" @click="save">
              {{ saving ? 'Saving…' : editing ? 'Update' : 'Create' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </SettingsShell>
</template>

<script setup lang="ts">
import type { AlertRule, AlertEvent } from '~/composables/useApi'

const api = useApi()

const rules = ref<AlertRule[]>([])
const events = ref<AlertEvent[]>([])
const loadingRules = ref(true)
const loadingEvents = ref(true)
const saveError = ref('')
const confirmDelete = ref<string | null>(null)
const showForm = ref(false)
const saving = ref(false)
const editing = ref<AlertRule | null>(null)

const form = reactive<{
  name: string
  condition: AlertRule['condition']
  threshold: number
  severity: AlertRule['severity']
  enabled: boolean
}>({
  name: '',
  condition: 'cpu_percent',
  threshold: 90,
  severity: 'warning',
  enabled: true,
})

const thresholdLabel = computed(() => {
  if (form.condition === 'node_offline' || form.condition === 'handshake_age') return 'Threshold (minutes)'
  return 'Threshold (%)'
})

async function load() {
  try {
    rules.value = await api.listAlertRules()
  } finally {
    loadingRules.value = false
  }
  try {
    events.value = await api.listAlertEvents(200)
  } finally {
    loadingEvents.value = false
  }
}

function openCreate() {
  editing.value = null
  Object.assign(form, { name: '', condition: 'cpu_percent', threshold: 90, severity: 'warning', enabled: true })
  saveError.value = ''
  showForm.value = true
}

function openEdit(r: AlertRule) {
  editing.value = r
  Object.assign(form, { name: r.name, condition: r.condition, threshold: r.threshold, severity: r.severity, enabled: r.enabled })
  saveError.value = ''
  showForm.value = true
}

function closeForm() {
  showForm.value = false
}

async function save() {
  saving.value = true
  saveError.value = ''
  try {
    if (editing.value) {
      const updated = await api.updateAlertRule(editing.value.id, { ...form })
      const idx = rules.value.findIndex(r => r.id === editing.value!.id)
      if (idx !== -1) rules.value[idx] = updated
    } else {
      const created = await api.createAlertRule({ ...form, silenced_until: '' })
      rules.value.unshift(created)
    }
    closeForm()
  } catch (e: any) {
    saveError.value = e?.message ?? 'Failed to save rule'
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(r: AlertRule) {
  try {
    const updated = await api.updateAlertRule(r.id, { ...r, enabled: !r.enabled })
    const idx = rules.value.findIndex(x => x.id === r.id)
    if (idx !== -1) rules.value[idx] = updated
  } catch (e: any) {
    saveError.value = e?.message ?? 'Failed to update rule'
  }
}

async function silence(r: AlertRule, hours: number) {
  const until = new Date(Date.now() + hours * 3600 * 1000).toISOString()
  try {
    const updated = await api.silenceAlertRule(r.id, until)
    const idx = rules.value.findIndex(x => x.id === r.id)
    if (idx !== -1) rules.value[idx] = updated
  } catch (e: any) {
    saveError.value = e?.message ?? 'Failed to silence rule'
  }
}

async function deleteRule(id: string) {
  try {
    await api.deleteAlertRule(id)
    rules.value = rules.value.filter(r => r.id !== id)
  } catch (e: any) {
    saveError.value = e?.message ?? 'Failed to delete rule'
  } finally {
    confirmDelete.value = null
  }
}

async function acknowledge(id: string) {
  try {
    await api.acknowledgeAlertEvent(id)
    const ev = events.value.find(e => e.id === id)
    if (ev) ev.acknowledged_at = new Date().toISOString()
  } catch (e: any) {
    saveError.value = e?.message ?? 'Failed to acknowledge event'
  }
}

function isSilenced(r: AlertRule) {
  if (!r.silenced_until) return false
  return new Date(r.silenced_until) > new Date()
}

function fmtThreshold(r: AlertRule) {
  if (r.condition === 'node_offline' || r.condition === 'handshake_age') return `${r.threshold} min`
  return `${r.threshold}%`
}

function fmtDate(s: string) {
  if (!s) return '—'
  return new Date(s).toLocaleString()
}

function severityVariant(s: string): 'success' | 'warning' | 'danger' | 'info' {
  if (s === 'critical') return 'danger'
  if (s === 'warning') return 'warning'
  return 'info'
}

onMounted(load)
</script>

<style scoped>
.mt-6 { margin-top: 1.5rem; }

.row--open td { color: var(--color-text); }

.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 600;
}
.badge--success { background: var(--color-success-bg, #d1fae5); color: var(--color-success, #065f46); }
.badge--muted   { background: var(--color-surface-2, #f3f4f6); color: var(--color-text-dim, #6b7280); }
</style>