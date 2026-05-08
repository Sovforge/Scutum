<template>
  <div class="gitops-page">

    <!-- Add repo modal -->
    <Teleport to="body">
      <div v-if="showAddModal" class="modal-backdrop" @click.self="showAddModal = false">
        <div class="modal">
          <div class="modal__header">
            <span class="modal__title">Add Repository</span>
            <button class="modal__close" @click="showAddModal = false">
              <Icon name="lucide:x" size="14" />
            </button>
          </div>
          <div class="modal__body">
            <div class="field">
              <label class="field__label">Display name</label>
              <input v-model="form.name" class="field__input" placeholder="my-app" />
            </div>
            <div class="field">
              <label class="field__label">Repository URL</label>
              <input v-model="form.repoUrl" class="field__input field__input--mono" placeholder="https://gitea.local/org/repo.git" />
            </div>
            <div class="field">
              <label class="field__label">Branch</label>
              <input v-model="form.branch" class="field__input field__input--mono" placeholder="main" />
            </div>
            <div class="field">
              <label class="field__label">Target path</label>
              <input v-model="form.target" class="field__input field__input--mono" placeholder="my-app" />
            </div>
            <div class="field">
              <label class="field__label">Username <span class="field__optional">(optional)</span></label>
              <input v-model="form.username" class="field__input" autocomplete="off" />
            </div>
            <div class="field">
              <label class="field__label">Token / password <span class="field__optional">(optional)</span></label>
              <input v-model="form.token" type="password" class="field__input" autocomplete="off" />
            </div>
          </div>
          <div class="modal__footer">
            <button class="action-btn action-btn--ghost" @click="showAddModal = false">Cancel</button>
            <button class="action-btn action-btn--primary" :disabled="!form.repoUrl || !form.name" @click="addRepo">
              <Icon name="lucide:plus" size="13" /> Add &amp; Sync
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Stat strip -->
    <div class="stat-strip">
      <div class="stat-card">
        <span class="stat-card__value">{{ apps.length }}</span>
        <span class="stat-card__label">Repositories</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__value stat-card__value--ok">{{ synced }}</span>
        <span class="stat-card__label">Synced</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__value stat-card__value--warn">{{ outOfSync }}</span>
        <span class="stat-card__label">Out of Sync</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__value stat-card__value--danger">{{ failed }}</span>
        <span class="stat-card__label">Failed</span>
      </div>
    </div>

    <!-- Main split -->
    <div class="gitops-grid">

      <!-- App list -->
      <UiCard title="Repositories">
        <template #header-right>
          <div class="toolbar">
            <div class="search-wrap">
              <Icon name="lucide:search" size="13" class="search-icon" />
              <input v-model="search" class="search-input" placeholder="Search…" />
            </div>
            <button class="action-btn action-btn--primary" @click="showAddModal = true">
              <Icon name="lucide:plus" size="13" /> Add Repo
            </button>
          </div>
        </template>

        <div v-if="apps.length === 0" class="empty-hint">
          No repositories configured. Click <strong>Add Repo</strong> to add one.
        </div>
        <div class="app-list">
          <div
            v-for="app in filteredApps"
            :key="app.id"
            class="app-item"
            :class="{ 'app-item--selected': selectedApp?.id === app.id }"
            @click="selectedApp = app"
          >
            <div class="app-item__left">
              <div class="app-item__icon">
                <Icon name="lucide:git-branch" size="14" />
              </div>
              <div class="app-item__info">
                <span class="app-item__name">{{ app.name }}</span>
                <span class="app-item__repo">{{ app.repoUrl }} @ {{ app.branch }}</span>
              </div>
            </div>
            <div class="app-item__right">
              <span class="sync-badge" :class="`sync-badge--${app.syncStatus}`">{{ app.syncStatus }}</span>
            </div>
          </div>
        </div>
      </UiCard>

      <!-- Right column -->
      <div class="right-col">

        <!-- App detail -->
        <UiCard v-if="selectedApp" :title="selectedApp.name">
          <template #header-right>
            <div class="action-group">
              <button class="action-btn action-btn--ghost" :disabled="syncing" @click="syncApp(selectedApp)">
                <Icon name="lucide:refresh-cw" size="13" :class="{ spin: syncing }" />
                {{ syncing ? 'Syncing…' : 'Sync' }}
              </button>
              <button class="action-btn action-btn--ghost action-btn--danger" @click="removeApp(selectedApp)">
                <Icon name="lucide:trash-2" size="13" /> Remove
              </button>
            </div>
          </template>

          <div class="detail-grid">
            <div class="detail-row">
              <span class="detail-label">Repository</span>
              <span class="detail-val cell--mono">{{ selectedApp.repoUrl }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Branch</span>
              <span class="detail-val cell--mono">{{ selectedApp.branch }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Target path</span>
              <span class="detail-val cell--mono">{{ selectedApp.target }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Last sync</span>
              <span class="detail-val cell--muted">{{ selectedApp.lastSync ?? 'never' }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Status</span>
              <span class="detail-val">
                <span class="sync-badge" :class="`sync-badge--${selectedApp.syncStatus}`">{{ selectedApp.syncStatus }}</span>
              </span>
            </div>
          </div>

          <p v-if="syncError" class="sync-error">{{ syncError }}</p>
        </UiCard>

        <UiCard v-else title="Repository Detail">
          <p class="empty-hint">Select a repository to view details.</p>
        </UiCard>

        <!-- Sync history -->
        <UiCard title="Sync History">
          <div v-if="syncHistory.length === 0" class="empty-hint">No syncs recorded yet.</div>
          <div class="sync-history">
            <div v-for="ev in syncHistory" :key="ev.id" class="sync-event">
              <div class="sync-event__dot" :class="`dot--${ev.result}`" />
              <div class="sync-event__body">
                <div class="sync-event__top">
                  <span class="sync-event__app">{{ ev.app }}</span>
                  <span class="sync-event__time">{{ ev.time }}</span>
                </div>
                <div class="sync-event__msg">{{ ev.message }}</div>
              </div>
            </div>
          </div>
        </UiCard>

      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()

type SyncStatus = 'Synced' | 'OutOfSync' | 'Failed' | 'Unknown'

interface App {
  id:         string
  name:       string
  repoUrl:    string
  branch:     string
  target:     string
  username?:  string
  token?:     string
  syncStatus: SyncStatus
  lastSync:   string | null
}

interface SyncEvent {
  id:      number
  app:     string
  result:  'ok' | 'fail'
  message: string
  time:    string
}

// ── Persisted repo list (localStorage) ────────────────────────────────────
const STORAGE_KEY = 'scutum_gitops_repos'

function loadApps(): App[] {
  if (!import.meta.client) return []
  try { return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]') } catch { return [] }
}

function saveApps(list: App[]) {
  if (!import.meta.client) return
  localStorage.setItem(STORAGE_KEY, JSON.stringify(list))
}

const apps = ref<App[]>(loadApps())
watch(apps, v => saveApps(v), { deep: true })

// ── State ──────────────────────────────────────────────────────────────────
const search      = ref('')
const selectedApp = ref<App | null>(apps.value[0] ?? null)
const syncing     = ref(false)
const syncError   = ref('')
const syncHistory = ref<SyncEvent[]>([])
let   nextEventId = 1

const showAddModal = ref(false)
const form = reactive({
  name: '', repoUrl: '', branch: 'main', target: '', username: '', token: '',
})

// ── Computed ───────────────────────────────────────────────────────────────
const filteredApps = computed(() => {
  if (!search.value) return apps.value
  const q = search.value.toLowerCase()
  return apps.value.filter(a => a.name.includes(q) || a.repoUrl.includes(q))
})

const synced    = computed(() => apps.value.filter(a => a.syncStatus === 'Synced').length)
const outOfSync = computed(() => apps.value.filter(a => a.syncStatus === 'OutOfSync' || a.syncStatus === 'Unknown').length)
const failed    = computed(() => apps.value.filter(a => a.syncStatus === 'Failed').length)

// ── Actions ────────────────────────────────────────────────────────────────
function addRepo() {
  const app: App = {
    id:         crypto.randomUUID(),
    name:       form.name.trim(),
    repoUrl:    form.repoUrl.trim(),
    branch:     form.branch.trim() || 'main',
    target:     form.target.trim() || form.name.trim(),
    username:   form.username || undefined,
    token:      form.token || undefined,
    syncStatus: 'Unknown',
    lastSync:   null,
  }
  apps.value.push(app)
  selectedApp.value = app
  showAddModal.value = false
  Object.assign(form, { name: '', repoUrl: '', branch: 'main', target: '', username: '', token: '' })
  syncApp(app)
}

async function syncApp(app: App) {
  syncing.value   = true
  syncError.value = ''
  try {
    await api.gitSync({
      repo_url:   app.repoUrl,
      username:   app.username,
      token:      app.token,
      target_dir: app.target,
    })
    const idx = apps.value.findIndex(a => a.id === app.id)
    if (idx !== -1) {
      apps.value[idx]!.syncStatus = 'Synced'
      apps.value[idx]!.lastSync   = new Date().toLocaleTimeString()
      if (selectedApp.value?.id === app.id) selectedApp.value = apps.value[idx]!
    }
    syncHistory.value.unshift({ id: nextEventId++, app: app.name, result: 'ok', message: 'Sync successful', time: new Date().toLocaleTimeString() })
  } catch (e: any) {
    const msg = typeof e?.data === 'string' ? e.data.trim() : 'Sync failed. Please check the server logs.'
    syncError.value = msg
    const idx = apps.value.findIndex(a => a.id === app.id)
    if (idx !== -1) {
      apps.value[idx]!.syncStatus = 'Failed'
      if (selectedApp.value?.id === app.id) selectedApp.value = apps.value[idx]!
    }
    syncHistory.value.unshift({ id: nextEventId++, app: app.name, result: 'fail', message: msg, time: new Date().toLocaleTimeString() })
  } finally {
    syncing.value = false
  }
}

function removeApp(app: App) {
  apps.value = apps.value.filter(a => a.id !== app.id)
  if (selectedApp.value?.id === app.id) selectedApp.value = apps.value[0] ?? null
}
</script>

<style scoped>
.gitops-page {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.5rem;
}

/* ── Stat strip ─────────────────────────────────────────────────────────── */
.stat-strip {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
}
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
.stat-card__value--ok     { color: var(--success-light); }
.stat-card__value--warn   { color: var(--warning); }
.stat-card__value--danger { color: var(--danger-light); }
.stat-card__label { font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }

/* ── Grid ───────────────────────────────────────────────────────────────── */
.gitops-grid {
  display: grid;
  grid-template-columns: 1fr 360px;
  gap: 1.25rem;
  align-items: start;
}
.right-col { display: flex; flex-direction: column; gap: 1.25rem; }

/* ── Toolbar ────────────────────────────────────────────────────────────── */
.toolbar { display: flex; align-items: center; gap: 0.625rem; }
.search-wrap { position: relative; display: flex; align-items: center; }
.search-icon { position: absolute; left: 0.5rem; color: var(--text-muted); pointer-events: none; }
.search-input {
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.35rem 0.75rem 0.35rem 2rem;
  font-size: 0.8rem;
  color: var(--text-primary);
  width: 180px;
  outline: none;
}
.search-input::placeholder { color: var(--text-dim); }
.search-input:focus { border-color: var(--accent); }

.action-group { display: flex; gap: 0.5rem; }
.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.3rem 0.625rem;
  border-radius: 0.375rem;
  font-size: 0.75rem;
  cursor: pointer;
  transition: all 0.15s;
  border: 1px solid transparent;
}
.action-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.action-btn--ghost {
  background: none;
  border-color: var(--border-strong);
  color: var(--text-tertiary);
}
.action-btn--ghost:hover:not(:disabled) { background: #ffffff08; color: var(--text-primary); border-color: var(--border-hover); }
.action-btn--danger { color: var(--danger-light) !important; border-color: var(--danger-border) !important; }
.action-btn--danger:hover { background: var(--danger-bg) !important; }
.action-btn--primary {
  background: var(--accent-subtle);
  border-color: var(--accent-soft);
  color: var(--accent-light);
}
.action-btn--primary:hover:not(:disabled) { background: var(--accent-tint); }

/* ── App list ───────────────────────────────────────────────────────────── */
.app-list { display: flex; flex-direction: column; gap: 0.25rem; }
.app-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.625rem 0.75rem;
  border-radius: 0.375rem;
  cursor: pointer;
  transition: background 0.12s;
  border: 1px solid transparent;
}
.app-item:hover { background: var(--hover-subtle); }
.app-item--selected { background: var(--accent-dim); border-color: var(--accent-tint); }

.app-item__left { display: flex; align-items: center; gap: 0.75rem; min-width: 0; }
.app-item__icon {
  width: 30px; height: 30px;
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  display: flex; align-items: center; justify-content: center;
  color: var(--accent); flex-shrink: 0;
}
.app-item__info { display: flex; flex-direction: column; gap: 0.1rem; min-width: 0; }
.app-item__name { font-size: 0.85rem; font-weight: 500; color: var(--text-primary); }
.app-item__repo { font-size: 0.72rem; color: var(--text-muted); font-family: monospace; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.app-item__right { display: flex; align-items: center; gap: 0.5rem; flex-shrink: 0; }

/* ── Badges ─────────────────────────────────────────────────────────────── */
.sync-badge {
  display: inline-flex; align-items: center;
  padding: 0.15rem 0.5rem; border-radius: 0.25rem;
  font-size: 0.68rem; font-weight: 600;
}
.sync-badge--Synced  { background: var(--role-dev-bg); color: var(--success-light); border: 1px solid var(--role-dev-border); }
.sync-badge--Failed  { background: var(--danger-bg); color: var(--danger-light); border: 1px solid var(--danger-border); }
.sync-badge--OutOfSync,
.sync-badge--Unknown { background: var(--role-gray-bg); color: var(--text-muted); border: 1px solid var(--border-strong); }

/* ── Detail ─────────────────────────────────────────────────────────────── */
.detail-grid { display: flex; flex-direction: column; gap: 0.5rem; }
.detail-row { display: flex; justify-content: space-between; align-items: center; gap: 1rem; }
.detail-label { font-size: 0.75rem; color: var(--text-muted); flex-shrink: 0; }
.detail-val { font-size: 0.82rem; color: var(--text-secondary); text-align: right; }
.cell--mono  { font-family: monospace; font-size: 0.78rem; color: var(--text-tertiary); }
.cell--muted { color: var(--text-muted); }

.sync-error { margin-top: 0.75rem; padding: 0.5rem 0.75rem; background: var(--danger-bg); border: 1px solid var(--danger-border); border-radius: 0.375rem; font-size: 0.78rem; color: var(--danger-light); }

/* ── Sync history ───────────────────────────────────────────────────────── */
.sync-history { display: flex; flex-direction: column; gap: 0.75rem; }
.sync-event { display: flex; gap: 0.75rem; align-items: flex-start; }
.sync-event__dot {
  width: 8px; height: 8px; border-radius: 50%;
  margin-top: 0.3rem; flex-shrink: 0;
}
.dot--ok   { background: var(--success); }
.dot--fail { background: var(--danger); }

.sync-event__body { display: flex; flex-direction: column; gap: 0.2rem; min-width: 0; }
.sync-event__top { display: flex; justify-content: space-between; align-items: center; }
.sync-event__app  { font-size: 0.8rem; font-weight: 500; color: var(--text-primary); }
.sync-event__time { font-size: 0.72rem; color: var(--text-muted); flex-shrink: 0; }
.sync-event__msg  { font-size: 0.75rem; color: var(--text-tertiary); }

.empty-hint { color: var(--text-dim); font-size: 0.85rem; text-align: center; padding: 1rem 0; }

/* ── Modal ──────────────────────────────────────────────────────────────── */
.modal-backdrop {
  position: fixed; inset: 0;
  background: rgba(0,0,0,0.55);
  display: flex; align-items: center; justify-content: center;
  z-index: 100;
}
.modal {
  background: var(--bg-surface);
  border: 1px solid var(--border-strong);
  border-radius: 0.625rem;
  width: 440px;
  max-height: 90vh;
  display: flex; flex-direction: column;
  overflow: hidden;
}
.modal__header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border);
}
.modal__title { font-size: 0.9rem; font-weight: 600; color: var(--text-primary); }
.modal__close {
  background: none; border: none; color: var(--text-muted);
  cursor: pointer; display: flex; align-items: center;
  border-radius: 0.25rem; padding: 0.25rem;
}
.modal__close:hover { color: var(--text-primary); }

.modal__body {
  padding: 1.25rem;
  display: flex; flex-direction: column; gap: 0.875rem;
  overflow-y: auto;
}
.field { display: flex; flex-direction: column; gap: 0.35rem; }
.field__label { font-size: 0.78rem; color: var(--text-muted); }
.field__optional { color: var(--text-dim); font-size: 0.72rem; }
.field__input {
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.45rem 0.75rem;
  font-size: 0.82rem;
  color: var(--text-primary);
  outline: none;
}
.field__input--mono { font-family: monospace; font-size: 0.78rem; }
.field__input:focus { border-color: var(--accent); }

.modal__footer {
  display: flex; justify-content: flex-end; gap: 0.5rem;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--border);
}

.spin { animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
