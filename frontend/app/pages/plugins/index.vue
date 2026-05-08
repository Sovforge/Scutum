<template>
  <div class="plugins-page">

    <!-- Header row -->
    <div class="plugins-header">
      <div class="plugins-header__meta">
        <span class="runtime-badge">
          <Icon name="lucide:cpu" size="12" />
          WASM · wazero sandbox
        </span>
        <span class="plugin-count">{{ plugins.length }} loaded</span>
      </div>
      <button class="action-btn action-btn--primary" @click="openLoad">
        <Icon name="lucide:plus" size="13" /> Load plugin
      </button>
    </div>

    <!-- Plugin list -->
    <UiCard v-if="plugins.length > 0">
      <table class="data-table">
        <thead>
          <tr>
            <th>Plugin</th>
            <th>File</th>
            <th>Sandbox</th>
            <th>Loaded</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in plugins" :key="p.name" class="table-row">
            <td class="cell--name">
              <div class="plugin-icon">
                <Icon name="lucide:puzzle" size="13" />
              </div>
              <span class="plugin-name">{{ p.name }}</span>
            </td>
            <td class="cell--path">{{ p.path.split('/').pop() }}</td>
            <td>
              <span class="sandbox-badge">
                <Icon name="lucide:shield" size="11" />
                WASM
              </span>
            </td>
            <td class="cell--muted">{{ p.loadedAt ?? '—' }}</td>
            <td class="cell--actions">
              <template v-if="pendingUnload === p.name">
                <span class="confirm-label">Unload?</span>
                <button class="icon-btn" @click="pendingUnload = null">Cancel</button>
                <button class="icon-btn icon-btn--danger" @click="unload(p.name)">Confirm</button>
              </template>
              <template v-else>
                <button class="icon-btn icon-btn--danger" title="Unload" @click="pendingUnload = p.name">
                  <Icon name="lucide:power-off" size="13" />
                </button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
    </UiCard>

    <!-- Empty state -->
    <div v-else class="empty-state">
      <div class="empty-state__icon">
        <Icon name="lucide:puzzle" size="28" />
      </div>
      <p class="empty-state__title">No plugins loaded</p>
      <p class="empty-state__desc">
        Plugins are WASM modules that extend Scutum. Upload a <code class="inline-code">.wasm</code> file to get started.
      </p>
      <button class="action-btn action-btn--primary" @click="openLoad">
        <Icon name="lucide:plus" size="13" /> Load first plugin
      </button>
    </div>

    <!-- Info strip -->
    <div class="info-strip">
      <div class="info-strip__item">
        <Icon name="lucide:shield-check" size="13" class="info-strip__icon" />
        <span>Each plugin runs in an isolated WASM sandbox — no direct access to host memory, filesystem, or network beyond what the runtime explicitly grants.</span>
      </div>
      <div class="info-strip__item">
        <Icon name="lucide:database" size="13" class="info-strip__icon" />
        <span>Plugins may use the built-in KV store for persistent state and can make outbound HTTP calls through the controlled host bridge.</span>
      </div>
      <div class="info-strip__item">
        <Icon name="lucide:zap" size="13" class="info-strip__icon" />
        <span>Lifecycle hooks: <code class="inline-code">on_load</code> is called on instantiation, <code class="inline-code">on_unload</code> on removal.</span>
      </div>
    </div>

  </div>

  <!-- Load modal -->
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="showModal" class="modal-backdrop" @click.self="closeModal">
        <div class="modal">
          <div class="modal__header">
            <h3 class="modal__title">Load plugin</h3>
            <button class="modal__close" @click="closeModal">
              <Icon name="lucide:x" size="16" />
            </button>
          </div>
          <div class="modal__body">
            <div class="form-grid">
              <!-- Drop zone -->
              <div
                class="drop-zone"
                :class="{ 'drop-zone--over': dragOver, 'drop-zone--picked': !!form.file }"
                @dragover.prevent="dragOver = true"
                @dragleave="dragOver = false"
                @drop.prevent="onDrop"
                @click="fileInput?.click()"
              >
                <input ref="fileInput" type="file" accept=".wasm" class="drop-zone__input" @change="onFileChange" />
                <template v-if="form.file">
                  <Icon name="lucide:file-check" size="20" class="drop-zone__icon drop-zone__icon--ok" />
                  <span class="drop-zone__filename">{{ form.file.name }}</span>
                  <span class="drop-zone__size">{{ (form.file.size / 1024).toFixed(1) }} KB</span>
                </template>
                <template v-else>
                  <Icon name="lucide:upload" size="20" class="drop-zone__icon" />
                  <span class="drop-zone__hint">Drop <code class="inline-code">.wasm</code> file here or <span class="drop-zone__browse">browse</span></span>
                </template>
              </div>

              <div class="form-row">
                <label class="form-label">Plugin name</label>
                <input v-model="form.name" class="form-input" placeholder="my-plugin" />
              </div>
              <div class="form-row">
                <label class="form-label">Description <span class="form-label-hint">(optional)</span></label>
                <input v-model="form.description" class="form-input" placeholder="Short description of what this plugin does" />
              </div>
            </div>
            <p v-if="formError" class="form-error">{{ formError }}</p>
          </div>
          <div class="modal__footer">
            <button class="action-btn action-btn--ghost" @click="closeModal">Cancel</button>
            <button class="action-btn action-btn--primary" :disabled="loading" @click="load">
              <Icon v-if="loading" name="lucide:loader-circle" size="13" class="spin" />
              {{ loading ? 'Loading…' : 'Load plugin' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()

// ── Remote data ────────────────────────────────────────────────────────────
const plugins    = ref<PluginRecord[]>([])
const apiLoading = ref(true)
const apiError   = ref('')

async function loadPlugins() {
  apiLoading.value = true
  apiError.value = ''
  try {
    plugins.value = await api.listPlugins()
  } catch (e: any) {
    apiError.value = e?.data?.error ?? 'Failed to load plugins'
  } finally {
    apiLoading.value = false
  }
}

onMounted(loadPlugins)

// ── Unload ─────────────────────────────────────────────────────────────────
const pendingUnload = ref<string | null>(null)

async function unload(name: string) {
  try {
    await api.unloadPlugin(name)
    await loadPlugins()
  } catch (e: any) {
    apiError.value = e?.data?.error ?? 'Unload failed'
  }
  pendingUnload.value = null
}

// ── Load modal ─────────────────────────────────────────────────────────────
const showModal = ref(false)
const loading   = ref(false)
const formError = ref('')
const dragOver  = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const form = reactive<{ name: string; description: string; file: File | null }>({
  name: '', description: '', file: null,
})

function openLoad() {
  Object.assign(form, { name: '', description: '', file: null })
  formError.value = ''
  showModal.value = true
}
function closeModal() { showModal.value = false }

function pickFile(file: File) {
  if (!file.name.endsWith('.wasm')) { formError.value = 'Only .wasm files are accepted.'; return }
  formError.value = ''
  form.file = file
  if (!form.name) form.name = file.name.replace(/\.wasm$/, '')
}
function onFileChange(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (f) pickFile(f)
}
function onDrop(e: DragEvent) {
  dragOver.value = false
  const f = e.dataTransfer?.files?.[0]
  if (f) pickFile(f)
}

async function load() {
  formError.value = ''
  const name = form.name.trim()
  if (!form.file) { formError.value = 'Please select a .wasm file.'; return }
  if (!name)      { formError.value = 'Plugin name is required.'; return }

  loading.value = true
  try {
    const body = new FormData()
    body.append('name', name)
    body.append('file', form.file)
    if (form.description) body.append('description', form.description)
    await api.uploadPlugin(body)
    await loadPlugins()
    closeModal()
  } catch (e: any) {
    formError.value = e?.data?.error ?? 'Upload failed'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.plugins-page { display: flex; flex-direction: column; gap: 1.25rem; }

/* Header */
.plugins-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.plugins-header__meta { display: flex; align-items: center; gap: 0.75rem; }
.runtime-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.72rem;
  font-family: monospace;
  background: var(--accent-subtle);
  border: 1px solid var(--accent-soft);
  color: var(--accent-light);
  padding: 0.2rem 0.5rem;
  border-radius: 0.3rem;
}
.plugin-count { font-size: 0.78rem; color: var(--text-dim); }

/* Table */
.data-table { width: 100%; border-collapse: collapse; font-size: 0.82rem; }
.data-table th {
  text-align: left;
  padding: 0.5rem 0.75rem;
  color: var(--text-muted);
  font-weight: 500;
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid var(--border);
}
.data-table td { padding: 0.625rem 0.75rem; border-bottom: 1px solid var(--border-faint); color: var(--text-secondary); vertical-align: middle; }
.table-row:last-child td { border-bottom: none; }
.table-row:hover { background: var(--hover-subtle); }

.cell--name {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  min-width: 160px;
}
.cell--path { font-family: monospace; font-size: 0.72rem; color: var(--text-dim); }
.cell--exports { display: flex; flex-wrap: wrap; gap: 0.25rem; }
.cell--muted { color: var(--text-dim); white-space: nowrap; }
.cell--actions { display: flex; align-items: center; gap: 0.25rem; justify-content: flex-end; white-space: nowrap; }

.plugin-icon {
  width: 28px;
  height: 28px;
  background: var(--accent-subtle);
  border: 1px solid var(--accent-soft);
  border-radius: 0.4rem;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent-light);
  flex-shrink: 0;
}
.plugin-name { display: block; font-weight: 500; color: var(--text-primary); }
.plugin-desc { display: block; font-size: 0.72rem; color: var(--text-dim); }

.sandbox-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.68rem;
  font-weight: 600;
  font-family: monospace;
  background: rgba(34, 197, 94, 0.08);
  border: 1px solid rgba(34, 197, 94, 0.2);
  color: #4ade80;
  padding: 0.15rem 0.4rem;
  border-radius: 0.25rem;
}

.export-chip {
  font-size: 0.67rem;
  font-family: monospace;
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  color: var(--text-tertiary);
  padding: 0.1rem 0.35rem;
  border-radius: 0.2rem;
}

.confirm-label { font-size: 0.75rem; color: var(--danger-light); margin-right: 0.25rem; }

/* Empty state */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  padding: 3.5rem 1rem;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  text-align: center;
}
.empty-state__icon {
  width: 52px;
  height: 52px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-dim);
}
.empty-state__title { margin: 0; font-size: 0.9rem; font-weight: 600; color: var(--text-primary); }
.empty-state__desc  { margin: 0; font-size: 0.8rem; color: var(--text-muted); max-width: 380px; line-height: 1.55; }

/* Info strip */
.info-strip {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 0.875rem 1rem;
}
.info-strip__item {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  font-size: 0.78rem;
  color: var(--text-muted);
  line-height: 1.5;
}
.info-strip__icon { color: var(--text-dim); margin-top: 1px; flex-shrink: 0; }

/* Action buttons */
.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.35rem 0.75rem;
  border-radius: 0.375rem;
  font-size: 0.78rem;
  font-family: inherit;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.15s;
}
.action-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.action-btn--primary { background: var(--accent-subtle); border-color: var(--accent-soft); color: var(--accent-light); }
.action-btn--primary:not(:disabled):hover { background: var(--accent-tint); }
.action-btn--ghost   { background: none; border-color: var(--border-strong); color: var(--text-tertiary); }
.action-btn--ghost:hover { background: var(--hover-bg); color: var(--text-primary); }

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
.icon-btn:hover         { color: var(--text-primary); border-color: var(--border-hover); }
.icon-btn--danger:hover { color: var(--danger-light); border-color: #7f1d1d; }

/* Modal */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.55);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
  padding: 1rem;
}
.modal {
  width: 100%;
  max-width: 480px;
  background: var(--bg-surface);
  border: 1px solid var(--border-strong);
  border-radius: 0.75rem;
  box-shadow: 0 24px 64px var(--shadow-lg);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border);
}
.modal__title { margin: 0; font-size: 0.95rem; font-weight: 700; color: var(--text-primary); }
.modal__close {
  background: none; border: none; color: var(--text-dim); cursor: pointer;
  padding: 0.2rem; border-radius: 0.25rem; display: flex; transition: color 0.15s, background 0.15s;
}
.modal__close:hover { color: var(--text-primary); background: var(--hover-bg); }
.modal__body   { padding: 1.25rem; }
.modal__footer { display: flex; justify-content: flex-end; gap: 0.5rem; padding: 0.875rem 1.25rem; border-top: 1px solid var(--border); }

.form-grid  { display: flex; flex-direction: column; gap: 0.875rem; }
.form-row   { display: flex; flex-direction: column; gap: 0.35rem; }
.form-label { font-size: 0.75rem; color: var(--text-tertiary); }
.form-label-hint { color: var(--text-subtle); font-weight: 400; }
.form-error { font-size: 0.75rem; color: var(--danger-light); margin: 0.5rem 0 0; }
.form-input {
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.5rem 0.75rem;
  font-size: 0.82rem;
  color: var(--text-primary);
  font-family: inherit;
  outline: none;
  width: 100%;
  box-sizing: border-box;
  transition: border-color 0.15s;
}
.form-input:focus { border-color: var(--accent); }
.form-input::placeholder { color: var(--text-subtle); }
.font-mono { font-family: monospace; }

/* Drop zone */
.drop-zone {
  border: 2px dashed var(--border-strong);
  border-radius: 0.5rem;
  background: var(--bg-elevated);
  padding: 1.5rem 1rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
  text-align: center;
}
.drop-zone:hover, .drop-zone--over {
  border-color: var(--accent);
  background: var(--accent-dim);
}
.drop-zone--picked {
  border-color: var(--success);
  background: var(--success-dim);
}
.drop-zone__input { display: none; }
.drop-zone__icon { color: var(--text-dim); }
.drop-zone__icon--ok { color: var(--success-light); }
.drop-zone__hint { font-size: 0.8rem; color: var(--text-muted); }
.drop-zone__browse { color: var(--accent-light); text-decoration: underline; }
.drop-zone__filename { font-size: 0.82rem; font-family: monospace; font-weight: 500; color: var(--text-primary); }
.drop-zone__size { font-size: 0.72rem; color: var(--text-dim); }

.inline-code {
  font-family: monospace;
  font-size: 0.85em;
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.2rem;
  padding: 0.05em 0.3em;
  color: var(--text-tertiary);
}

@keyframes spin { to { transform: rotate(360deg); } }
.spin { animation: spin 0.8s linear infinite; }

.modal-enter-active, .modal-leave-active { transition: opacity 0.18s ease; }
.modal-enter-active .modal, .modal-leave-active .modal { transition: transform 0.18s ease, opacity 0.18s ease; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-from .modal, .modal-leave-to .modal { transform: translateY(-10px) scale(0.97); opacity: 0; }
</style>
