<template>
  <div class="settings-page">

    <!-- Side nav + content split -->
    <div class="settings-layout">

      <!-- Left nav -->
      <nav class="settings-nav">
        <button
          v-for="section in sections"
          :key="section.id"
          class="settings-nav__item"
          :class="{ 'settings-nav__item--active': activeSection === section.id }"
          @click="activeSection = section.id"
        >
          <Icon :name="section.icon" size="15" />
          {{ section.label }}
        </button>
        <div class="settings-nav__divider" />
        <NuxtLink to="/settings/rbac"     class="settings-nav__item">
          <Icon name="lucide:shield" size="15" /> RBAC
        </NuxtLink>
        <NuxtLink to="/settings/secrets"  class="settings-nav__item">
          <Icon name="lucide:key-round" size="15" /> Secrets / KMS
        </NuxtLink>
        <NuxtLink to="/settings/webhooks" class="settings-nav__item">
          <Icon name="lucide:webhook" size="15" /> Webhooks
        </NuxtLink>
        <NuxtLink to="/settings/scim"     class="settings-nav__item">
          <Icon name="lucide:users" size="15" /> SCIM
        </NuxtLink>
      </nav>

      <!-- Content -->
      <div class="settings-content">

        <!-- General -->
        <template v-if="activeSection === 'general'">
          <UiCard title="General">
            <div class="form-grid">
              <div class="form-row">
                <label class="form-label">Cluster name</label>
                <input v-model="general.clusterName" class="form-input" />
              </div>
              <div class="form-row">
                <label class="form-label">Region</label>
                <input v-model="general.region" class="form-input" />
              </div>
              <div class="form-row">
                <label class="form-label">API endpoint</label>
                <input v-model="general.apiEndpoint" class="form-input font-mono" />
              </div>
              <div class="form-row">
                <label class="form-label">Log level</label>
                <select v-model="general.logLevel" class="form-select">
                  <option>debug</option>
                  <option>info</option>
                  <option>warn</option>
                  <option>error</option>
                </select>
              </div>
            </div>
          </UiCard>
        </template>

        <!-- Mesh -->
        <template v-if="activeSection === 'mesh'">
          <UiCard title="WireGuard Mesh">
            <div class="form-grid">
              <div class="form-row">
                <label class="form-label">Listen port</label>
                <input v-model="mesh.port" type="number" class="form-input" />
              </div>
              <div class="form-row">
                <label class="form-label">MTU</label>
                <input v-model="mesh.mtu" type="number" class="form-input" />
              </div>
              <div class="form-row">
                <label class="form-label">DNS server</label>
                <input v-model="mesh.dns" class="form-input font-mono" />
              </div>
              <div class="form-row">
                <label class="form-label">Keepalive (s)</label>
                <input v-model="mesh.keepalive" type="number" class="form-input" />
              </div>
              <div class="form-row form-row--toggle">
                <label class="form-label">Auto-routing</label>
                <label class="toggle">
                  <input v-model="mesh.autoRouting" type="checkbox" />
                  <span class="toggle__track"><span class="toggle__thumb" /></span>
                </label>
              </div>
              <div class="form-row form-row--toggle">
                <label class="form-label">NAT traversal</label>
                <label class="toggle">
                  <input v-model="mesh.nat" type="checkbox" />
                  <span class="toggle__track"><span class="toggle__thumb" /></span>
                </label>
              </div>
            </div>
          </UiCard>

          <UiCard title="Public Key" class="mt">
            <div class="pubkey-block">
              <code class="pubkey-val">{{ meshPubkey }}</code>
              <button class="icon-btn" title="Copy">
                <Icon name="lucide:copy" size="13" />
              </button>
            </div>
            <button class="danger-btn mt-sm">
              <Icon name="lucide:rotate-ccw" size="13" /> Rotate keypair
            </button>
          </UiCard>
        </template>

        <!-- Nodes -->
        <template v-if="activeSection === 'nodes'">
          <UiCard title="Node Defaults">
            <div class="form-grid">
              <div class="form-row">
                <label class="form-label">Default role</label>
                <select v-model="nodeDefaults.role" class="form-select">
                  <option>edge</option>
                  <option>hub</option>
                  <option value="hub+edge">hub+edge</option>
                </select>
              </div>
              <div class="form-row form-row--toggle">
                <label class="form-label">Require approval</label>
                <label class="toggle">
                  <input v-model="nodeDefaults.requireApproval" type="checkbox" />
                  <span class="toggle__track"><span class="toggle__thumb" /></span>
                </label>
              </div>
              <div class="form-row">
                <label class="form-label">Heartbeat interval (s)</label>
                <input v-model="nodeDefaults.heartbeat" type="number" class="form-input" />
              </div>
            </div>
          </UiCard>

          <UiCard title="Enrollment Token" class="mt">
            <div class="pubkey-block">
              <code class="pubkey-val">{{ enrollToken }}</code>
              <button class="icon-btn" title="Copy">
                <Icon name="lucide:copy" size="13" />
              </button>
            </div>
            <p class="form-hint">Share this token with nodes to allow enrollment. Rotate regularly.</p>
            <button class="danger-btn mt-sm">
              <Icon name="lucide:rotate-ccw" size="13" /> Rotate token
            </button>
          </UiCard>
        </template>

        <!-- Database -->
        <template v-if="activeSection === 'database'">
          <UiCard title="State Database">
            <div class="db-note">
              <Icon name="lucide:info" size="13" class="db-note__icon" />
              Scutum stores encrypted cluster state in a database. Defaults to embedded SQLite — no external dependencies required. Switch to PostgreSQL or MySQL for HA or externalized state.
            </div>
            <div class="form-grid mt">
              <div class="form-row">
                <label class="form-label">Backend</label>
                <select v-model="database.backend" class="form-select">
                  <option value="sqlite">SQLite (embedded, default)</option>
                  <option value="postgres">PostgreSQL</option>
                  <option value="mysql">MySQL</option>
                </select>
              </div>
              <template v-if="database.backend !== 'sqlite'">
                <div class="form-row">
                  <label class="form-label">Host</label>
                  <input v-model="database.host" class="form-input font-mono" placeholder="localhost:5432" />
                </div>
                <div class="form-row">
                  <label class="form-label">Database name</label>
                  <input v-model="database.name" class="form-input font-mono" placeholder="scutum" />
                </div>
                <div class="form-row">
                  <label class="form-label">Username</label>
                  <input v-model="database.user" class="form-input font-mono" />
                </div>
                <div class="form-row">
                  <label class="form-label">Password</label>
                  <input v-model="database.password" type="password" class="form-input font-mono" />
                </div>
                <div class="form-row form-row--toggle">
                  <label class="form-label">TLS / SSL</label>
                  <label class="toggle">
                    <input v-model="database.tls" type="checkbox" />
                    <span class="toggle__track"><span class="toggle__thumb" /></span>
                  </label>
                </div>
              </template>
              <div class="form-row form-row--toggle">
                <label class="form-label">Encryption at rest</label>
                <label class="toggle">
                  <input v-model="database.encrypted" type="checkbox" />
                  <span class="toggle__track"><span class="toggle__thumb" /></span>
                </label>
              </div>
              <div class="form-row">
                <label class="form-label">Max connections</label>
                <input v-model="database.maxConns" type="number" class="form-input" />
              </div>
            </div>
          </UiCard>

          <UiCard title="Current State">
            <div class="form-grid">
              <div class="detail-row">
                <span class="detail-label">Backend</span>
                <span class="detail-val cell--mono">{{ database.backend === 'sqlite' ? 'SQLite (embedded)' : database.backend }}</span>
              </div>
              <div class="detail-row">
                <span class="detail-label">Encryption</span>
                <span class="detail-val" :class="database.encrypted ? 'val-ok' : 'val-warn'">
                  <Icon :name="database.encrypted ? 'lucide:shield-check' : 'lucide:shield-off'" size="13" />
                  {{ database.encrypted ? 'Enabled (AES-256-GCM via KMS)' : 'Disabled' }}
                </span>
              </div>
              <div class="detail-row">
                <span class="detail-label">Data path</span>
                <span class="detail-val cell--mono">/app/data/state.db</span>
              </div>
              <div class="detail-row">
                <span class="detail-label">Size</span>
                <span class="detail-val">12.4 MB</span>
              </div>
            </div>
            <button class="secondary-btn mt-sm" :disabled="exporting" @click="exportDB">
              <Icon :name="exporting ? 'lucide:loader' : 'lucide:download'" size="13" :class="{ spin: exporting }" />
              {{ exporting ? 'Exporting…' : 'Export snapshot' }}
            </button>
          </UiCard>
        </template>

        <!-- Auth -->
        <template v-if="activeSection === 'auth'">
          <UiCard title="Authentication">
            <div class="form-grid">
              <div class="form-row">
                <label class="form-label">Auth provider</label>
                <span class="auth-provider-badge">
                  <Icon name="lucide:user-round" size="13" />
                  Local (username + password)
                </span>
              </div>
              <div class="form-row form-row--toggle">
                <label class="form-label">Require MFA</label>
                <label class="toggle">
                  <input v-model="auth.mfa" type="checkbox" />
                  <span class="toggle__track"><span class="toggle__thumb" /></span>
                </label>
              </div>
              <div class="form-row">
                <label class="form-label">Session timeout (min)</label>
                <input v-model="auth.sessionTimeout" type="number" class="form-input" />
              </div>
            </div>
          </UiCard>

          <UiCard title="Single Sign-On" class="mt">
            <div class="sso-roadmap">
              <div class="sso-roadmap__icon">
                <Icon name="lucide:clock-4" size="18" />
              </div>
              <div class="sso-roadmap__body">
                <p class="sso-roadmap__title">Coming soon</p>
                <p class="sso-roadmap__text">
                  OIDC / SAML single sign-on is on the roadmap. Once available you will be able to
                  delegate authentication to your identity provider — Keycloak, Okta, GitHub, Azure AD, and others.
                </p>
              </div>
            </div>
          </UiCard>
        </template>

        <!-- Audit forwarding -->
        <template v-if="activeSection === 'audit'">
          <UiCard title="Audit Log Forwarders">
            <div class="fwd-toolbar">
              <button class="btn-sm-primary" @click="showFwdForm = true">
                <Icon name="lucide:plus" size="13" /> Add forwarder
              </button>
            </div>
            <div v-if="fwdLoading" class="loading-row">Loading…</div>
            <div v-else-if="!forwarders.length" class="empty-row">No forwarders configured.</div>
            <table v-else class="data-table mt-sm">
              <thead><tr><th>Name</th><th>URL</th><th>Format</th><th>Status</th><th></th></tr></thead>
              <tbody>
                <tr v-for="f in forwarders" :key="f.id" class="data-table__row">
                  <td class="fw-medium">{{ f.name }}</td>
                  <td class="mono text-dim">{{ f.url }}</td>
                  <td><UiBadge variant="info">{{ f.format }}</UiBadge></td>
                  <td><UiBadge :variant="f.enabled ? 'success' : 'neutral'">{{ f.enabled ? 'Active' : 'Paused' }}</UiBadge></td>
                  <td class="cell--actions">
                    <button class="icon-btn" @click="toggleFwd(f)"><Icon :name="f.enabled ? 'lucide:pause' : 'lucide:play'" size="13" /></button>
                    <template v-if="pendingFwdDelete === f.id">
                      <span class="delete-confirm-label">Remove?</span>
                      <button class="icon-btn" @click="pendingFwdDelete = null">Cancel</button>
                      <button class="icon-btn icon-btn--danger" @click="deleteFwd(f.id)">Confirm</button>
                    </template>
                    <button v-else class="icon-btn icon-btn--danger" @click="pendingFwdDelete = f.id"><Icon name="lucide:trash-2" size="13" /></button>
                  </td>
                </tr>
              </tbody>
            </table>
          </UiCard>
          <UiCard v-if="showFwdForm" title="New forwarder" class="mt">
            <div class="form-grid">
              <div class="form-row"><label class="form-label">Name</label><input v-model="fwdForm.name" class="form-input" /></div>
              <div class="form-row"><label class="form-label">URL</label><input v-model="fwdForm.url" class="form-input font-mono" /></div>
              <div class="form-row">
                <label class="form-label">Format</label>
                <select v-model="fwdForm.format" class="form-select">
                  <option value="json">JSON</option>
                  <option value="cef">CEF (ArcSight / QRadar)</option>
                </select>
              </div>
            </div>
            <div class="form-actions mt-sm">
              <button class="btn-sm-ghost" @click="showFwdForm = false">Cancel</button>
              <button class="btn-sm-primary" @click="saveFwd">Save</button>
            </div>
          </UiCard>
        </template>

        <!-- TLS -->
        <template v-if="activeSection === 'tls'">
          <UiCard title="TLS / HTTPS">
            <div class="form-grid">
              <div class="form-row">
                <label class="form-label">Mode</label>
                <span v-if="tlsMode.mode === 'acme'" class="tls-badge tls-badge--acme">
                  <Icon name="lucide:shield-check" size="13" /> ACME / Let&#39;s Encrypt
                </span>
                <span v-else-if="tlsMode.mode === 'manual'" class="tls-badge tls-badge--manual">
                  <Icon name="lucide:file-key" size="13" /> Manual certificate
                </span>
                <span v-else class="tls-badge tls-badge--none">
                  <Icon name="lucide:shield-off" size="13" /> Plain HTTP (no TLS)
                </span>
              </div>
              <template v-if="tlsMode.mode === 'acme'">
                <div class="form-row">
                  <label class="form-label">Domain</label>
                  <span class="form-value mono">{{ tlsMode.domain }}</span>
                </div>
                <div class="form-row">
                  <label class="form-label">Email</label>
                  <span class="form-value">{{ tlsMode.email }}</span>
                </div>
                <div class="form-row">
                  <label class="form-label">Staging</label>
                  <span class="form-value">{{ tlsMode.staging ? 'Yes (staging)' : 'No (production)' }}</span>
                </div>
              </template>
              <template v-else-if="tlsMode.mode === 'manual'">
                <div class="form-row">
                  <label class="form-label">Certificate file</label>
                  <span class="form-value mono">{{ tlsMode.cert_file }}</span>
                </div>
              </template>
            </div>
            <p class="tls-note">TLS mode is controlled by environment variables (<code>ACME_DOMAIN</code> / <code>CERT_FILE</code>). Restart the server after changing them.</p>
          </UiCard>
        </template>

        <!-- Save bar -->
        <div class="save-bar">
          <button class="save-btn">
            <Icon name="lucide:save" size="14" /> Save changes
          </button>
          <button class="cancel-btn">Discard</button>
        </div>

      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()
const exporting = ref(false)

async function exportDB() {
  exporting.value = true
  try {
    const blob = await api.exportDatabase()
    const url  = URL.createObjectURL(blob)
    const a    = document.createElement('a')
    a.href     = url
    a.download = `scutum-export-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
  } finally {
    exporting.value = false
  }
}

type SectionId = 'general' | 'mesh' | 'nodes' | 'database' | 'auth' | 'audit' | 'tls'

const sections = [
  { id: 'general'  as SectionId, icon: 'lucide:sliders-horizontal', label: 'General'   },
  { id: 'mesh'     as SectionId, icon: 'lucide:network',            label: 'Mesh'      },
  { id: 'nodes'    as SectionId, icon: 'lucide:server',             label: 'Nodes'     },
  { id: 'database' as SectionId, icon: 'lucide:database',           label: 'Database'  },
  { id: 'auth'     as SectionId, icon: 'lucide:lock',               label: 'Auth'      },
  { id: 'audit'    as SectionId, icon: 'lucide:send',               label: 'Forwarding'},
  { id: 'tls'      as SectionId, icon: 'lucide:shield',             label: 'TLS'       },
]

const activeSection = ref<SectionId>('general')

const general = reactive({
  clusterName: 'prod-cluster',
  region: 'eu-west-1',
  apiEndpoint: 'http://localhost:8080/api/v1',
  logLevel: 'info',
})

const wgPubkeyCookie = useCookie<string>('wg_pubkey')
const meshPubkey = computed(() => wgPubkeyCookie.value || '(complete setup to generate key)')
const mesh = reactive({
  port: 51820,
  mtu: 1420,
  dns: '',
  keepalive: 25,
  autoRouting: true,
  nat: true,
})

const nodeDefaults = reactive({
  role: 'edge',
  requireApproval: true,
  heartbeat: 30,
})

const enrollToken = 'sov-enroll-Gk9mR3xQ2p7LnYwHdTvBcF5jZ8sX1oAe'

const database = reactive({
  backend:   'sqlite',
  host:      'localhost:5432',
  name:      'scutum',
  user:      'scutum',
  password:  '',
  tls:       true,
  encrypted: true,
  maxConns:  20,
})

const auth = reactive({
  mfa: false,
  sessionTimeout: 60,
})

// ── Audit forwarders ────────────────────────────────────────────────────────
const forwarders = ref<any[]>([])
const fwdLoading = ref(false)
const showFwdForm = ref(false)
const pendingFwdDelete = ref<string | null>(null)
const fwdForm = reactive({ name: '', url: '', format: 'json' })

async function loadForwarders() {
  fwdLoading.value = true
  forwarders.value = await api.listAuditForwarders().catch(() => [])
  fwdLoading.value = false
}

async function saveFwd() {
  if (!fwdForm.name || !fwdForm.url) return
  await api.createAuditForwarder({ ...fwdForm }).catch(() => {})
  showFwdForm.value = false
  Object.assign(fwdForm, { name: '', url: '', format: 'json' })
  await loadForwarders()
}

async function toggleFwd(f: any) {
  await api.updateAuditForwarder(f.id, { enabled: !f.enabled }).catch(() => {})
  await loadForwarders()
}

async function deleteFwd(id: string) {
  await api.deleteAuditForwarder(id).catch(() => {})
  pendingFwdDelete.value = null
  await loadForwarders()
}

watch(activeSection, (s) => { if (s === 'audit') loadForwarders() })

// ── TLS mode ─────────────────────────────────────────────────────────────────
const tlsMode = ref<{ mode: string; domain?: string; email?: string; staging?: boolean; cert_file?: string }>({ mode: 'none' })
onMounted(async () => { tlsMode.value = await api.getTLSMode().catch(() => ({ mode: 'none' })) })
</script>

<style scoped>
.settings-page {
  padding: 1.5rem;
  height: 100%;
}

.settings-layout {
  display: grid;
  grid-template-columns: 200px 1fr;
  gap: 1.5rem;
  align-items: start;
}

/* ── Left nav ───────────────────────────────────────────────────────────── */
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
.settings-nav__item--active,
.router-link-active.settings-nav__item { color: var(--accent-light); background: var(--accent-dim); }
.settings-nav__divider {
  height: 1px;
  background: var(--border);
  margin: 0.25rem 0;
}

/* ── Content ────────────────────────────────────────────────────────────── */
.settings-content {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}
.mt    { margin-top: 0; }
.mt-sm { margin-top: 0.75rem; display: inline-flex; }

/* ── Form ───────────────────────────────────────────────────────────────── */
.form-grid { display: flex; flex-direction: column; gap: 1rem; }
.form-row {
  display: grid;
  grid-template-columns: 180px 1fr;
  align-items: center;
  gap: 1rem;
}
.form-row--toggle { align-items: center; }
.form-label { font-size: 0.82rem; color: var(--text-tertiary); }
.form-hint  { font-size: 0.75rem; color: var(--text-dim); margin-top: 0.5rem; }
.form-input, .form-select {
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
.form-input:focus, .form-select:focus { border-color: var(--accent); }
.font-mono { font-family: 'JetBrains Mono', monospace; font-size: 0.78rem; }

/* ── Toggle ─────────────────────────────────────────────────────────────── */
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
.toggle input:checked ~ .toggle__track {
  background: var(--accent);
  border-color: var(--accent);
}
.toggle__thumb {
  position: absolute;
  top: 2px; left: 2px;
  width: 14px; height: 14px;
  background: var(--text-muted);
  border-radius: 50%;
  transition: transform 0.2s, background 0.2s;
}
.toggle input:checked ~ .toggle__track .toggle__thumb {
  transform: translateX(16px);
  background: #fff;
}

/* ── Pubkey block ───────────────────────────────────────────────────────── */
.pubkey-block {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  background: var(--bg-deep);
  border: 1px solid var(--border);
  border-radius: 0.375rem;
  padding: 0.625rem 0.875rem;
}
.pubkey-val {
  font-family: monospace;
  font-size: 0.78rem;
  color: var(--text-tertiary);
  word-break: break-all;
  flex: 1;
}
.icon-btn {
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.25rem;
  color: var(--text-muted);
  padding: 0.3rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  flex-shrink: 0;
  transition: all 0.15s;
}
.icon-btn:hover { color: var(--text-primary); border-color: var(--border-hover); }

/* ── Save bar ───────────────────────────────────────────────────────────── */
.save-bar {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding-top: 0.5rem;
}
.save-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  background: var(--accent);
  border: none;
  border-radius: 0.375rem;
  padding: 0.5rem 1.25rem;
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
  padding: 0.5rem 1rem;
  font-size: 0.82rem;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
}
.cancel-btn:hover { color: var(--text-primary); border-color: var(--border-hover); }

.danger-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  background: none;
  border: 1px solid var(--danger-border);
  border-radius: 0.375rem;
  padding: 0.4rem 0.875rem;
  font-size: 0.78rem;
  color: var(--danger-light);
  cursor: pointer;
  transition: all 0.15s;
}
.danger-btn:hover { background: var(--danger-bg); }

.secondary-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.4rem 0.875rem;
  font-size: 0.78rem;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s;
  font-family: inherit;
}
.secondary-btn:hover:not(:disabled) { background: var(--hover-bg); color: var(--text-primary); border-color: var(--accent-soft); }
.secondary-btn:disabled { opacity: 0.6; cursor: default; }
@keyframes spin { to { transform: rotate(360deg); } }
.spin { animation: spin 0.8s linear infinite; }

/* ── Auth section ───────────────────────────────────────────────────────── */
.auth-provider-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.82rem;
  color: var(--text-secondary);
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.4rem 0.75rem;
}

.sso-roadmap {
  display: flex;
  align-items: flex-start;
  gap: 0.875rem;
  background: #1e3a5f18;
  border: 1px solid #1e3a5f40;
  border-radius: 0.5rem;
  padding: 0.875rem 1rem;
}
.sso-roadmap__icon {
  flex-shrink: 0;
  margin-top: 0.1rem;
  color: var(--text-muted);
}
.sso-roadmap__body { display: flex; flex-direction: column; gap: 0.25rem; }
.sso-roadmap__title {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0;
}
.sso-roadmap__text {
  font-size: 0.78rem;
  color: var(--text-muted);
  line-height: 1.55;
  margin: 0;
}

/* ── Database section ───────────────────────────────────────────────────── */
.db-note {
  display: flex;
  align-items: flex-start;
  gap: 0.625rem;
  background: #1e40af10;
  border: 1px solid #1e40af30;
  border-radius: 0.5rem;
  padding: 0.75rem 1rem;
  font-size: 0.8rem;
  color: #93c5fd;
  line-height: 1.5;
  margin-bottom: 0.25rem;
}
.db-note__icon { color: #60a5fa; flex-shrink: 0; margin-top: 0.1rem; }

.mt { margin-top: 1rem; }
.mt-sm { margin-top: 0.75rem; display: inline-flex; }

.detail-row  { display: flex; justify-content: space-between; align-items: center; padding: 0.4rem 0; border-bottom: 1px solid var(--border-faint); }
.detail-row:last-child { border-bottom: none; }
.detail-label { font-size: 0.78rem; color: var(--text-muted); }
.detail-val   { font-size: 0.82rem; color: var(--text-secondary); display: flex; align-items: center; gap: 0.35rem; }
.cell--mono   { font-family: monospace; font-size: 0.78rem; color: var(--text-tertiary); }
.val-ok   { color: var(--success-light) !important; }
.val-warn { color: var(--warning) !important; }

/* ── Audit forwarders ────────────────────────────────────────────────────── */
.fwd-toolbar { display: flex; justify-content: flex-end; margin-bottom: 0.75rem; }
.btn-sm-primary {
  display: inline-flex; align-items: center; gap: 0.35rem;
  background: var(--accent); color: #fff; border: none; border-radius: 0.3rem;
  padding: 0.375rem 0.75rem; font-size: 0.78rem; font-weight: 600; cursor: pointer;
}
.btn-sm-primary:hover { background: var(--accent-hover); }
.btn-sm-ghost {
  background: none; border: 1px solid var(--border-strong); color: var(--text-muted);
  border-radius: 0.3rem; padding: 0.375rem 0.75rem; font-size: 0.78rem; cursor: pointer;
}
.loading-row, .empty-row { padding: 1.5rem; text-align: center; color: var(--text-dim); font-size: 0.82rem; }
.data-table { width: 100%; border-collapse: collapse; font-size: 0.82rem; }
.data-table th { text-align: left; padding: 0.5rem 0.75rem; color: var(--text-dim); font-weight: 500; border-bottom: 1px solid var(--border); }
.data-table td { padding: 0.6rem 0.75rem; border-bottom: 1px solid var(--border-subtle, var(--border)); }
.data-table__row:last-child td { border-bottom: none; }
.cell--actions { display: flex; align-items: center; gap: 0.25rem; justify-content: flex-end; }
.icon-btn { background: none; border: none; color: var(--text-dim); cursor: pointer; padding: 0.25rem; border-radius: 0.25rem; display: inline-flex; align-items: center; }
.icon-btn:hover { background: var(--hover-bg); color: var(--text-primary); }
.icon-btn--danger:hover { color: var(--danger-lighter); background: var(--danger-bg); }
.delete-confirm-label { font-size: 0.75rem; color: var(--text-dim); margin-right: 0.25rem; }
.fw-medium { font-weight: 500; }
.mono { font-family: monospace; font-size: 0.78rem; }
.text-dim { color: var(--text-dim); }
.mt { margin-top: 1rem; }
.mt-sm { margin-top: 0.5rem; }
.form-actions { display: flex; justify-content: flex-end; gap: 0.5rem; }

/* ── TLS ─────────────────────────────────────────────────────────────────── */
.tls-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.25rem 0.625rem;
  border-radius: 0.25rem;
  font-size: 0.78rem;
  font-weight: 500;
}
.tls-badge--acme   { background: var(--success-bg, #14532d22); color: var(--success-light); }
.tls-badge--manual { background: var(--accent-subtle); color: var(--accent-light); }
.tls-badge--none   { background: var(--bg-elevated); color: var(--text-dim); }

.form-value { font-size: 0.875rem; color: var(--text-primary); }
.form-value.mono { font-family: monospace; }

.tls-note {
  margin-top: 1rem;
  font-size: 0.75rem;
  color: var(--text-dim);
  line-height: 1.5;
}
.tls-note code {
  font-family: monospace;
  background: var(--bg-elevated);
  padding: 0.1rem 0.3rem;
  border-radius: 0.2rem;
}
</style>
