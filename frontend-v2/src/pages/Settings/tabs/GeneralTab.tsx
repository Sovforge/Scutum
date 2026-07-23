import { useState, type FormEvent } from 'react'
import { Database, Download, Lock, Plus, Radio, Settings as SettingsIcon, Shield, Trash2 } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import Select from '../../../components/ui/Select/Select'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import Tabs, { type TabItem } from '../../../components/ui/Tabs/Tabs'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import styles from '../Settings.module.css'

// The old app's settings/index.vue has 7 sub-tabs. Only three call real
// endpoints: Forwarding (GET/POST/PUT/DELETE /audit/forwarders — full
// CRUD), TLS (GET /system/tls-mode — read-only), and Database (GET
// /admin/export — blob download). General/Mesh/Nodes/Auth are local
// `reactive()` state with a no-op Save/Discard bar and no API calls at
// all in the old app either — not built out here for the same reason
// Secrets got a disclosure instead of fabricated forms: a Save button
// with nothing behind it doesn't add anything.
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

const PLACEHOLDER_TABS: SubTab[] = ['general', 'mesh', 'nodes', 'auth']

type Forwarder = { id: string; name: string; url: string; format: 'json' | 'cef'; enabled: boolean }

const INITIAL_FORWARDERS: Forwarder[] = [{ id: 'fwd1', name: 'siem-splunk', url: 'https://splunk.internal:8088/services/collector', format: 'json', enabled: true }]

const TLS_MODE = {
  mode: 'acme' as const,
  domain: 'scutum.example.com',
  email: 'admin@example.com',
  staging: false,
}

function GeneralTab() {
  const toast = useToast()
  const [subTab, setSubTab] = useState<SubTab>('forwarding')

  // ── Forwarding ───────────────────────────────────────────────────
  const [forwarders, setForwarders] = useState(INITIAL_FORWARDERS)
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [format, setFormat] = useState<'json' | 'cef'>('json')
  const [formError, setFormError] = useState('')

  function submitForwarder(e: FormEvent) {
    e.preventDefault()
    if (!name || !url) {
      setFormError('Name and URL are required.')
      return
    }
    setFormError('')
    setForwarders((prev) => [...prev, { id: name, name, url, format, enabled: true }])
    toast(`${name} added`)
    setName('')
    setUrl('')
    setFormat('json')
    setShowForm(false)
  }

  function removeForwarder(f: Forwarder) {
    setForwarders((prev) => prev.filter((x) => x.id !== f.id))
    toast(`${f.name} removed`, 'danger')
  }

  function toggleForwarder(f: Forwarder) {
    setForwarders((prev) => prev.map((x) => (x.id === f.id ? { ...x, enabled: !x.enabled } : x)))
    toast(`${f.name} ${f.enabled ? 'disabled' : 'enabled'}`)
  }

  // ── Database ─────────────────────────────────────────────────────
  function exportDatabase() {
    const blob = new Blob(
      [JSON.stringify({ note: 'Preview build — this is a stub, not a real database export.', exported_at: new Date().toISOString() }, null, 2)],
      { type: 'application/json' },
    )
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'scutum-export.json'
    a.click()
    URL.revokeObjectURL(url)
    toast('Export downloaded')
  }

  return (
    <>
      <Tabs tabs={SUB_TABS} active={subTab} onChange={(id) => setSubTab(id as SubTab)} variant="secondary" />

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
                <Button type="submit">Add</Button>
              </div>
            </form>
          )}

          {forwarders.length === 0 ? (
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
          <dl className={styles.tlsList}>
            <div>
              <dt>Mode</dt>
              <dd>
                <Badge variant="neutral">{TLS_MODE.mode}</Badge>
              </dd>
            </div>
            <div>
              <dt>Domain</dt>
              <dd className={styles.mono}>{TLS_MODE.domain}</dd>
            </div>
            <div>
              <dt>Email</dt>
              <dd className={styles.mono}>{TLS_MODE.email}</dd>
            </div>
            <div>
              <dt>Staging</dt>
              <dd>{TLS_MODE.staging ? 'Yes' : 'No'}</dd>
            </div>
          </dl>
        </Section>
      )}

      {subTab === 'database' && (
        <Section title="Database">
          <div className={styles.recoveryBody}>
            <p className={styles.recoveryIntro}>Export a full database snapshot as JSON.</p>
            <div className={styles.formActions}>
              <Button variant="ghost" onClick={exportDatabase}>
                <Download size={14} />
                Export snapshot
              </Button>
            </div>
          </div>
        </Section>
      )}

      {PLACEHOLDER_TABS.includes(subTab) && (
        <Section>
          <EmptyState
            icon={SUB_TABS.find((t) => t.id === subTab)?.icon}
            title={`${SUB_TABS.find((t) => t.id === subTab)?.label} has no real backend`}
            description="Local-only placeholder configuration in the reference app too — no API call exists to wire up here."
          />
        </Section>
      )}
    </>
  )
}

export default GeneralTab
