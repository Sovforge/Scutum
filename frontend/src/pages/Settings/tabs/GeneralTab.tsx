import { useEffect, useState, type FormEvent } from 'react'
import { Database, Download, Lock, Plus, Radio, Save, Settings as SettingsIcon, Shield, Trash2, X } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import Select from '../../../components/ui/Select/Select'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import Tabs, { type TabItem } from '../../../components/ui/Tabs/Tabs'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import {
  ApiError,
  createAuditForwarder,
  deleteAuditForwarder,
  exportDatabase,
  getSSOProviders,
  getSystemSettings,
  getTLSMode,
  listAuditForwarders,
  updateAuditForwarder,
  updateSystemSettings,
  type AuditForwarder,
  type SSOProvider,
  type SystemSettings,
} from '../../../lib/api'
import styles from '../Settings.module.css'

// Real endpoints: GET/POST/PUT/DELETE /audit/forwarders (Forwarding), GET
// /system/tls-mode (TLS, read-only), GET /admin/export (Database), and
// GET/PUT /settings (General/Mesh/Nodes/Auth — a real persisted policy
// object, cmd/internal/store/store.go's SystemSettings). See lib/api.ts's
// getSystemSettings/updateSystemSettings comment for exactly which fields
// have a real, live effect versus which are just stored for display.
type SubTab = 'general' | 'mesh' | 'nodes' | 'database' | 'auth' | 'forwarding' | 'tls'

const SUB_TABS: TabItem[] = [
  { id: 'general', label: 'General', icon: SettingsIcon },
  { id: 'mesh', label: 'Mesh', icon: Radio },
  { id: 'nodes', label: 'Nodes', icon: SettingsIcon },
  { id: 'database', label: 'Database', icon: Database },
  { id: 'auth', label: 'Auth', icon: Lock },
  { id: 'forwarding', label: 'Forwarding', icon: Radio },
  { id: 'tls', label: 'TLS', icon: Shield },
]

const SETTINGS_TABS: SubTab[] = ['general', 'mesh', 'nodes', 'auth']

function GeneralTab() {
  const toast = useToast()
  const [subTab, setSubTab] = useState<SubTab>('general')

  // ── System settings (General / Mesh / Nodes / Auth share one object) ────
  const [settings, setSettings] = useState<SystemSettings | null>(null)
  const [draft, setDraft] = useState<SystemSettings | null>(null)
  const [settingsLoading, setSettingsLoading] = useState(true)
  const [settingsError, setSettingsError] = useState('')
  const [saving, setSaving] = useState(false)

  async function loadSettings() {
    setSettingsLoading(true)
    setSettingsError('')
    try {
      const s = await getSystemSettings()
      setSettings(s)
      setDraft(s)
    } catch (e) {
      setSettingsError(e instanceof ApiError ? e.message : 'Failed to load settings')
    } finally {
      setSettingsLoading(false)
    }
  }

  useEffect(() => {
    loadSettings()
  }, [])

  const dirty = !!settings && !!draft && JSON.stringify(settings) !== JSON.stringify(draft)

  function patchDraft(patch: Partial<SystemSettings>) {
    setDraft((prev) => (prev ? { ...prev, ...patch } : prev))
  }

  async function saveSettings() {
    if (!draft) return
    setSaving(true)
    try {
      const saved = await updateSystemSettings(draft)
      setSettings(saved)
      setDraft(saved)
      toast('Settings saved')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Failed to save settings', 'danger')
    } finally {
      setSaving(false)
    }
  }

  function discardSettings() {
    setDraft(settings)
  }

  // ── SSO providers (Auth tab) ─────────────────────────────────────────────
  const [ssoProviders, setSsoProviders] = useState<SSOProvider[] | null>(null)

  useEffect(() => {
    getSSOProviders()
      .then(setSsoProviders)
      .catch(() => setSsoProviders([]))
  }, [])

  // ── Forwarding ───────────────────────────────────────────────────
  const [forwarders, setForwarders] = useState<AuditForwarder[]>([])
  const [fwdLoading, setFwdLoading] = useState(true)
  const [fwdError, setFwdError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [format, setFormat] = useState<'json' | 'cef'>('json')
  const [formError, setFormError] = useState('')

  async function loadForwarders() {
    setFwdLoading(true)
    setFwdError('')
    try {
      setForwarders(await listAuditForwarders())
    } catch (e) {
      setFwdError(e instanceof ApiError ? e.message : 'Failed to load forwarders')
    } finally {
      setFwdLoading(false)
    }
  }

  useEffect(() => {
    loadForwarders()
  }, [])

  async function submitForwarder(e: FormEvent) {
    e.preventDefault()
    if (!name || !url) {
      setFormError('Name and URL are required.')
      return
    }
    setSaving(true)
    setFormError('')
    try {
      const created = await createAuditForwarder({ name, url, format })
      setForwarders((prev) => [...prev, created])
      toast(`${name} added`)
      setName('')
      setUrl('')
      setFormat('json')
      setShowForm(false)
    } catch (e2) {
      setFormError(e2 instanceof ApiError ? e2.message : 'Failed to add forwarder')
    } finally {
      setSaving(false)
    }
  }

  async function removeForwarder(f: AuditForwarder) {
    try {
      await deleteAuditForwarder(f.id)
      setForwarders((prev) => prev.filter((x) => x.id !== f.id))
      toast(`${f.name} removed`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    }
  }

  async function toggleForwarder(f: AuditForwarder) {
    try {
      const updated = await updateAuditForwarder(f.id, { enabled: !f.enabled })
      setForwarders((prev) => prev.map((x) => (x.id === f.id ? updated : x)))
      toast(`${f.name} ${f.enabled ? 'disabled' : 'enabled'}`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Update failed', 'danger')
    }
  }

  // ── TLS ──────────────────────────────────────────────────────────
  const [tlsMode, setTlsMode] = useState<{ mode: string; domain?: string; email?: string; staging?: boolean; cert_file?: string } | null>(null)
  const [tlsError, setTlsError] = useState('')

  useEffect(() => {
    getTLSMode()
      .then(setTlsMode)
      .catch((e) => setTlsError(e instanceof ApiError ? e.message : 'Failed to load TLS mode'))
  }, [])

  // ── Database ─────────────────────────────────────────────────────
  const [exporting, setExporting] = useState(false)

  async function handleExport() {
    setExporting(true)
    try {
      const blob = await exportDatabase()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `scutum-export-${new Date().toISOString().slice(0, 10)}.json`
      a.click()
      URL.revokeObjectURL(url)
      toast('Export downloaded')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Export failed', 'danger')
    } finally {
      setExporting(false)
    }
  }

  return (
    <>
      <Tabs tabs={SUB_TABS} active={subTab} onChange={(id) => setSubTab(id as SubTab)} variant="secondary" />

      {SETTINGS_TABS.includes(subTab) && settingsError && (
        <div className={styles.calloutWrap}>
          <Callout variant="danger">{settingsError}</Callout>
        </div>
      )}
      {SETTINGS_TABS.includes(subTab) && settingsLoading && <div className={styles.loadingRow}>Loading…</div>}

      {subTab === 'general' && draft && (
        <Section title="General">
          <div className={styles.formFields}>
            <TextField
              label="Cluster name"
              id="clusterName"
              value={draft.ClusterName}
              onChange={(e) => patchDraft({ ClusterName: e.target.value })}
              placeholder="prod-cluster"
            />
            <TextField
              label="Region"
              id="region"
              value={draft.Region}
              onChange={(e) => patchDraft({ Region: e.target.value })}
              placeholder="eu-west-1"
            />
            <Select
              label="Log level"
              id="logLevel"
              value={draft.LogLevel}
              onChange={(e) => patchDraft({ LogLevel: e.target.value as SystemSettings['LogLevel'] })}
              options={[
                { value: 'debug', label: 'debug' },
                { value: 'info', label: 'info' },
                { value: 'warn', label: 'warn' },
                { value: 'error', label: 'error' },
              ]}
              hint="Applied to the running server immediately on save — no restart needed."
            />
          </div>
        </Section>
      )}

      {subTab === 'mesh' && draft && (
        <Section title="WireGuard Mesh">
          <div className={styles.formFields}>
            <TextField
              label="MTU"
              id="meshMtu"
              type="number"
              value={draft.MeshMTU}
              onChange={(e) => patchDraft({ MeshMTU: Number(e.target.value) })}
              hint="Applied live to the wg0 interface on save."
            />
            <TextField
              label="Keepalive (s)"
              id="meshKeepalive"
              type="number"
              value={draft.MeshKeepaliveSeconds}
              onChange={(e) => patchDraft({ MeshKeepaliveSeconds: Number(e.target.value) })}
              hint="Default persistent-keepalive for newly added peers."
            />
          </div>
          <p className={styles.secretDetail}>
            The listen port and address are fixed at setup time and aren't editable here — changing them live would require
            tearing down every peer's tunnel. Reinstall to change them.
          </p>
        </Section>
      )}

      {subTab === 'nodes' && draft && (
        <Section title="Node Defaults">
          <div className={styles.formFields}>
            <Select
              label="Default role"
              id="nodeDefaultRole"
              value={draft.NodeDefaultRole}
              onChange={(e) => patchDraft({ NodeDefaultRole: e.target.value as SystemSettings['NodeDefaultRole'] })}
              options={[
                { value: 'remote', label: 'Remote' },
                { value: 'hub', label: 'Hub' },
              ]}
              hint="Pre-selected role when enrolling a new node."
            />
          </div>
          <div className={styles.mfaRow}>
            <div className={styles.mfaStatus}>
              <span>Require approval for new nodes</span>
            </div>
            <button
              type="button"
              className={styles.statusToggle}
              onClick={() => patchDraft({ NodeRequireApproval: !draft.NodeRequireApproval })}
            >
              <Badge variant={draft.NodeRequireApproval ? 'success' : 'neutral'}>
                {draft.NodeRequireApproval ? 'On' : 'Off'}
              </Badge>
            </button>
          </div>
          <p className={styles.secretDetail}>
            When on, nodes enrolled via <code>POST /nodes</code> start out <Badge variant="warning">pending</Badge> instead of{' '}
            <Badge variant="success">approved</Badge> — approve or reject them from the Nodes page.
          </p>
        </Section>
      )}

      {subTab === 'auth' && draft && (
        <>
          <Section title="Authentication">
            <div className={styles.formFields}>
              <div className="field">
                <label>Auth provider</label>
                <Badge variant="neutral">Local (username + password)</Badge>
              </div>
              <TextField
                label="Session timeout (min)"
                id="sessionTimeout"
                type="number"
                value={draft.AuthSessionTimeoutMin}
                onChange={(e) => patchDraft({ AuthSessionTimeoutMin: Number(e.target.value) })}
                hint="How long a login token stays valid. Takes effect on the next login."
              />
            </div>
            <div className={styles.mfaRow}>
              <div className={styles.mfaStatus}>
                <span>Require MFA for every user</span>
              </div>
              <button
                type="button"
                className={styles.statusToggle}
                onClick={() => patchDraft({ AuthRequireMFA: !draft.AuthRequireMFA })}
              >
                <Badge variant={draft.AuthRequireMFA ? 'success' : 'neutral'}>{draft.AuthRequireMFA ? 'On' : 'Off'}</Badge>
              </button>
            </div>
            <p className={styles.secretDetail}>
              Users without MFA enabled can still log in with their password, but are routed straight to the Account page's
              MFA setup instead of the rest of the app until they enable it — this avoids locking out the only admin account.
            </p>
          </Section>

          <Section title="Single Sign-On">
            {ssoProviders === null ? (
              <div className={styles.loadingRow}>Loading…</div>
            ) : ssoProviders.length === 0 ? (
              <EmptyState
                icon={Lock}
                title="No SSO providers configured"
                description="OIDC/OAuth2 login is supported (Entra ID, GitHub, Authentik, Keycloak, or any generic OIDC provider) — set SSO_<PROVIDER>_CLIENT_ID / CLIENT_SECRET / ISSUER_URL to enable one. None are set on this instance."
              />
            ) : (
              <div className={styles.roleBadges}>
                {ssoProviders.map((p) => (
                  <Badge variant="success" key={p.id}>
                    {p.name}
                  </Badge>
                ))}
              </div>
            )}
          </Section>
        </>
      )}

      {SETTINGS_TABS.includes(subTab) && draft && (
        <div className={styles.formActions}>
          <Button variant="ghost" onClick={discardSettings} disabled={!dirty || saving}>
            <X size={14} />
            Discard
          </Button>
          <Button onClick={saveSettings} disabled={!dirty || saving}>
            <Save size={14} />
            {saving ? 'Saving…' : 'Save changes'}
          </Button>
        </div>
      )}

      {subTab === 'forwarding' && (
        <Section action={<Button variant="ghost" onClick={() => setShowForm((v) => !v)}><Plus size={14} />Add forwarder</Button>}>
          {showForm && (
            <form className={styles.inlineForm} onSubmit={submitForwarder}>
              <div className={styles.formFields}>
                <TextField label="Name" id="fwdName" value={name} onChange={(e) => setName(e.target.value)} placeholder="siem-splunk" />
                <TextField label="URL" id="fwdUrl" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://splunk.internal:8088/services/collector" />
                <Select
                  label="Format"
                  id="fwdFormat"
                  value={format}
                  onChange={(e) => setFormat(e.target.value as 'json' | 'cef')}
                  options={[
                    { value: 'json', label: 'JSON' },
                    { value: 'cef', label: 'CEF' },
                  ]}
                />
              </div>
              {formError && <p className={styles.formError}>{formError}</p>}
              <div className={styles.formActions}>
                <Button type="submit" disabled={saving}>
                  {saving ? 'Adding…' : 'Add'}
                </Button>
              </div>
            </form>
          )}

          {fwdError ? (
            <Callout variant="danger">{fwdError}</Callout>
          ) : fwdLoading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : forwarders.length === 0 ? (
            <EmptyState icon={Radio} title="No audit forwarders" description="Add a forwarder to stream audit events to a SIEM in real time." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>URL</th>
                  <th>Format</th>
                  <th>Status</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {forwarders.map((f) => (
                  <tr key={f.id}>
                    <td className="cell-name">{f.name}</td>
                    <td className="cell-muted">{f.url}</td>
                    <td>
                      <Badge variant="neutral">{f.format.toUpperCase()}</Badge>
                    </td>
                    <td>
                      <button type="button" className={styles.statusToggle} onClick={() => toggleForwarder(f)}>
                        <Badge variant={f.enabled ? 'success' : 'neutral'}>{f.enabled ? 'Enabled' : 'Disabled'}</Badge>
                      </button>
                    </td>
                    <td>
                      <div className={styles.rowActions}>
                        <button type="button" className={styles.rowActionDanger} onClick={() => removeForwarder(f)} aria-label={`Remove ${f.name}`}>
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Section>
      )}

      {subTab === 'tls' && (
        <Section title="TLS">
          {tlsError ? (
            <Callout variant="danger">{tlsError}</Callout>
          ) : !tlsMode ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : (
            <dl className={styles.tlsList}>
              <div>
                <dt>Mode</dt>
                <dd>
                  <Badge variant="neutral">{tlsMode.mode}</Badge>
                </dd>
              </div>
              {tlsMode.domain && (
                <div>
                  <dt>Domain</dt>
                  <dd className={styles.mono}>{tlsMode.domain}</dd>
                </div>
              )}
              {tlsMode.email && (
                <div>
                  <dt>Email</dt>
                  <dd className={styles.mono}>{tlsMode.email}</dd>
                </div>
              )}
              {tlsMode.mode === 'acme' && (
                <div>
                  <dt>Staging</dt>
                  <dd>{tlsMode.staging ? 'Yes' : 'No'}</dd>
                </div>
              )}
              {tlsMode.cert_file && (
                <div>
                  <dt>Certificate file</dt>
                  <dd className={styles.mono}>{tlsMode.cert_file}</dd>
                </div>
              )}
            </dl>
          )}
        </Section>
      )}

      {subTab === 'database' && (
        <Section title="Database">
          <div className={styles.recoveryBody}>
            <p className={styles.recoveryIntro}>Export a full database snapshot as JSON.</p>
            <div className={styles.formActions}>
              <Button variant="ghost" onClick={handleExport} disabled={exporting}>
                <Download size={14} />
                {exporting ? 'Exporting…' : 'Export snapshot'}
              </Button>
            </div>
          </div>
        </Section>
      )}
    </>
  )
}

export default GeneralTab
