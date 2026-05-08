<template>
  <div class="settings-page">
    <div class="settings-layout">

      <nav class="settings-nav">
        <NuxtLink to="/settings" class="settings-nav__item" exact-active-class="settings-nav__item--active">
          <Icon name="lucide:sliders-horizontal" size="15" /> General
        </NuxtLink>
        <div class="settings-nav__divider" />
        <NuxtLink to="/settings/rbac"    class="settings-nav__item" active-class="settings-nav__item--active">
          <Icon name="lucide:shield" size="15" /> RBAC
        </NuxtLink>
        <NuxtLink to="/settings/secrets" class="settings-nav__item" active-class="settings-nav__item--active">
          <Icon name="lucide:key-round" size="15" /> Secrets / KMS
        </NuxtLink>
      </nav>

      <div class="settings-content">

        <!-- KMS status -->
        <UiCard title="KMS Provider">
          <div class="kms-status">
            <div class="kms-icon">
              <Icon name="lucide:shield-check" size="20" class="kms-icon__svg" />
            </div>
            <div class="kms-info">
              <span class="kms-name">Scutum built-in KMS</span>
              <span class="kms-detail">Key ring: <code>scutum/cluster-keys</code> · 3 active keys · last rotation 7 days ago</span>
            </div>
            <span class="kms-badge">Active</span>
          </div>
          <div class="form-grid mt">
            <div class="form-row">
              <label class="form-label">KMS backend</label>
              <select v-model="kms.backend" class="form-select">
                <option value="builtin">Built-in (file-backed)</option>
                <option value="vault">HashiCorp Vault</option>
                <option value="aws">AWS KMS</option>
                <option value="gcp">GCP Cloud KMS</option>
              </select>
            </div>
            <div class="form-row form-row--toggle">
              <label class="form-label">Auto-rotate keys</label>
              <label class="toggle">
                <input v-model="kms.autoRotate" type="checkbox" />
                <span class="toggle__track"><span class="toggle__thumb" /></span>
              </label>
            </div>
            <div class="form-row">
              <label class="form-label">Rotation interval (days)</label>
              <input v-model="kms.rotationDays" type="number" class="form-input" />
            </div>
          </div>
        </UiCard>

        <!-- Secrets table -->
        <UiCard title="Secrets">
          <template #header-right>
            <button class="action-btn action-btn--primary" @click="showCreate = true">
              <Icon name="lucide:plus" size="13" /> New Secret
            </button>
          </template>

          <table class="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Namespace</th>
                <th>Type</th>
                <th>Created</th>
                <th>Used by</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in secrets" :key="s.name" class="table-row">
                <td class="cell--name">
                  <Icon name="lucide:key-round" size="13" class="row-icon" />
                  {{ s.name }}
                </td>
                <td class="cell--ns">{{ s.namespace }}</td>
                <td>
                  <span class="type-badge">{{ s.type }}</span>
                </td>
                <td class="cell--muted">{{ s.created }}</td>
                <td class="cell--muted">{{ s.usedBy }}</td>
                <td class="cell--actions">
                  <button class="icon-btn" title="View"><Icon name="lucide:eye" size="13" /></button>
                  <button class="icon-btn" title="Rotate"><Icon name="lucide:rotate-ccw" size="13" /></button>
                  <button class="icon-btn icon-btn--danger" title="Delete"><Icon name="lucide:trash-2" size="13" /></button>
                </td>
              </tr>
            </tbody>
          </table>
        </UiCard>

        <!-- Create secret modal -->
        <div v-if="showCreate" class="modal-backdrop" @click.self="showCreate = false">
          <div class="modal">
            <div class="modal__header">
              <span>New Secret</span>
              <button class="modal__close" @click="showCreate = false">
                <Icon name="lucide:x" size="15" />
              </button>
            </div>
            <div class="modal__body">
              <div class="form-grid">
                <div class="form-row">
                  <label class="form-label">Name</label>
                  <input v-model="newSecret.name" class="form-input" placeholder="my-secret" />
                </div>
                <div class="form-row">
                  <label class="form-label">Namespace</label>
                  <input v-model="newSecret.namespace" class="form-input" placeholder="default" />
                </div>
                <div class="form-row">
                  <label class="form-label">Type</label>
                  <select v-model="newSecret.type" class="form-select">
                    <option>Opaque</option>
                    <option>TLS</option>
                    <option>DockerRegistry</option>
                    <option>APIKey</option>
                  </select>
                </div>
                <div class="form-row">
                  <label class="form-label">Value</label>
                  <textarea v-model="newSecret.value" class="form-textarea" placeholder="Base64 or plain text…" rows="3" />
                </div>
              </div>
            </div>
            <div class="modal__footer">
              <button class="cancel-btn" @click="showCreate = false">Cancel</button>
              <button class="save-btn" @click="createSecret">Create</button>
            </div>
          </div>
        </div>

      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const kms = reactive({
  backend: 'builtin',
  autoRotate: true,
  rotationDays: 90,
})

const secrets = ref([
  { name: 'mesh-ca-cert',       namespace: 'kube-system', type: 'TLS',            created: '2025-11-01', usedBy: 'wireguard-mesh'    },
  { name: 'registry-creds',     namespace: 'default',     type: 'DockerRegistry', created: '2025-11-12', usedBy: '4 workloads'       },
  { name: 'db-password',        namespace: 'database',    type: 'Opaque',         created: '2025-12-03', usedBy: 'postgres-ha'       },
  { name: 'api-gateway-token',  namespace: 'default',     type: 'APIKey',         created: '2026-01-07', usedBy: 'api-gateway'       },
  { name: 'grafana-secret-key', namespace: 'monitoring',  type: 'Opaque',         created: '2026-02-14', usedBy: 'grafana'           },
  { name: 'gitops-ssh-key',     namespace: 'gitops',      type: 'Opaque',         created: '2026-03-01', usedBy: 'gitops-controller' },
])

const showCreate = ref(false)
const newSecret  = reactive({ name: '', namespace: 'default', type: 'Opaque', value: '' })

function createSecret() {
  if (!newSecret.name.trim()) return
  secrets.value.push({
    name: newSecret.name,
    namespace: newSecret.namespace,
    type: newSecret.type,
    created: new Date().toISOString().slice(0, 10),
    usedBy: '—',
  })
  newSecret.name = ''
  newSecret.value = ''
  showCreate.value = false
}
</script>

<style scoped>
.settings-page { padding: 1.5rem; }
.settings-layout { display: grid; grid-template-columns: 200px 1fr; gap: 1.5rem; align-items: start; }
.settings-content { display: flex; flex-direction: column; gap: 1.25rem; }

.settings-nav {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}
.settings-nav__item {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  font-size: 0.82rem;
  color: var(--text-muted);
  cursor: pointer;
  background: none;
  border: none;
  text-align: left;
  text-decoration: none;
  transition: color 0.15s, background 0.15s;
  width: 100%;
}
.settings-nav__item:hover { color: var(--text-primary); background: var(--hover-bg); }
.settings-nav__item--active { color: var(--accent-light); background: var(--accent-dim); }
.settings-nav__divider { height: 1px; background: var(--border); margin: 0.25rem 0; }

/* ── KMS status ─────────────────────────────────────────────────────────── */
.kms-status {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem 0;
  margin-bottom: 1rem;
  border-bottom: 1px solid var(--border);
}
.kms-icon {
  width: 40px; height: 40px;
  background: var(--accent-subtle);
  border: 1px solid var(--accent-soft);
  border-radius: 0.5rem;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0;
}
.kms-icon__svg { color: var(--accent-light); }
.kms-info { display: flex; flex-direction: column; gap: 0.2rem; flex: 1; }
.kms-name { font-size: 0.9rem; font-weight: 600; color: var(--text-primary); }
.kms-detail { font-size: 0.75rem; color: var(--text-muted); }
.kms-detail code { font-family: monospace; color: var(--text-tertiary); }
.kms-badge {
  background: var(--role-dev-bg);
  color: var(--success-light);
  border: 1px solid var(--role-dev-border);
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0.2rem 0.6rem;
  border-radius: 0.25rem;
}

.mt { margin-top: 0; }

/* ── Form ───────────────────────────────────────────────────────────────── */
.form-grid { display: flex; flex-direction: column; gap: 0.875rem; }
.form-row { display: grid; grid-template-columns: 180px 1fr; align-items: center; gap: 1rem; }
.form-row--toggle { align-items: center; }
.form-label { font-size: 0.82rem; color: var(--text-tertiary); }
.form-input, .form-select, .form-textarea {
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.4rem 0.75rem;
  font-size: 0.82rem;
  color: var(--text-primary);
  outline: none;
  width: 100%;
  font-family: inherit;
}
.form-input:focus, .form-select:focus, .form-textarea:focus { border-color: var(--accent); }
.form-textarea { resize: vertical; }

.toggle { display: inline-flex; align-items: center; cursor: pointer; }
.toggle input { display: none; }
.toggle__track {
  width: 36px; height: 20px;
  background: var(--border);
  border: 1px solid var(--border-strong);
  border-radius: 9999px;
  position: relative;
  transition: background 0.2s, border-color 0.2s;
}
.toggle input:checked ~ .toggle__track { background: var(--accent); border-color: var(--accent); }
.toggle__thumb {
  position: absolute;
  top: 2px; left: 2px;
  width: 14px; height: 14px;
  background: var(--text-muted);
  border-radius: 50%;
  transition: transform 0.2s, background 0.2s;
}
.toggle input:checked ~ .toggle__track .toggle__thumb { transform: translateX(16px); background: #fff; }

/* ── Table ──────────────────────────────────────────────────────────────── */
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
.data-table td { padding: 0.625rem 0.75rem; border-bottom: 1px solid var(--border); color: var(--text-secondary); vertical-align: middle; }
.table-row:last-child td { border-bottom: none; }
.table-row:hover { background: var(--hover-subtle); }

.cell--name { display: flex; align-items: center; gap: 0.5rem; color: var(--text-primary); font-weight: 500; }
.row-icon { color: var(--text-muted); flex-shrink: 0; }
.cell--muted { color: var(--text-muted); }
.cell--ns { font-family: monospace; font-size: 0.78rem; color: var(--accent-light); }
.cell--actions { display: flex; gap: 0.25rem; justify-content: flex-end; }

.type-badge {
  display: inline-flex;
  padding: 0.15rem 0.5rem;
  border-radius: 0.25rem;
  font-size: 0.7rem;
  font-weight: 600;
  background: var(--role-gray-bg);
  color: var(--text-muted);
  border: 1px solid var(--border-strong);
}

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
.action-btn--primary { background: var(--accent-subtle); border-color: var(--accent-soft); color: var(--accent-light); }
.action-btn--primary:hover { background: var(--accent-tint); }

.icon-btn {
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.25rem;
  color: var(--text-muted);
  padding: 0.25rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  transition: all 0.15s;
}
.icon-btn:hover { color: var(--text-primary); border-color: var(--border-hover); }
.icon-btn--danger:hover { color: var(--danger-light); border-color: #7f1d1d; }

/* ── Modal ──────────────────────────────────────────────────────────────── */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: #00000088;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.modal {
  background: var(--bg-surface);
  border: 1px solid var(--border-strong);
  border-radius: 0.75rem;
  width: 480px;
  max-width: 95vw;
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
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}
.modal__close {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  display: flex;
  align-items: center;
  padding: 0.2rem;
  border-radius: 0.25rem;
}
.modal__close:hover { color: var(--text-primary); }
.modal__body { padding: 1.25rem; }
.modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 0.875rem 1.25rem;
  border-top: 1px solid var(--border);
}

.save-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  background: var(--accent);
  border: none;
  border-radius: 0.375rem;
  padding: 0.45rem 1.25rem;
  font-size: 0.82rem;
  color: #fff;
  cursor: pointer;
  transition: background 0.15s;
}
.save-btn:hover { background: var(--accent-hover); }
.cancel-btn {
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.45rem 1rem;
  font-size: 0.82rem;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
}
.cancel-btn:hover { color: var(--text-primary); border-color: var(--border-hover); }
</style>
