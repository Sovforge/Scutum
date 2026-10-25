import { useEffect, useState, type FormEvent } from 'react'
import { Bell, BellOff, Check, Pencil, Plus, Trash2 } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import Select from '../../../components/ui/Select/Select'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import {
  ApiError,
  acknowledgeAlertEvent,
  createAlertRule,
  deleteAlertRule,
  listAlertEvents,
  listAlertRules,
  silenceAlertRule,
  updateAlertRule,
  type AlertEvent,
  type AlertRule,
} from '../../../lib/api'
import styles from '../Settings.module.css'

// Real endpoints: GET/POST/PUT/DELETE /alerts/rules(/:id), PUT
// /alerts/rules/:id/silence {until}, GET /alerts/events?limit=, POST
// /alerts/events/:id/acknowledge.
type Condition = AlertRule['condition']
type Severity = AlertRule['severity']

const CONDITION_LABEL: Record<Condition, string> = {
  cpu_percent: 'CPU %',
  mem_percent: 'Memory %',
  disk_percent: 'Disk %',
  node_offline: 'Node offline',
  handshake_age: 'Handshake age',
}

const SEVERITY_VARIANT: Record<string, 'neutral' | 'warning' | 'danger'> = {
  info: 'neutral',
  warning: 'warning',
  critical: 'danger',
}

function fmtDate(iso?: string): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function isSilenced(r: AlertRule): boolean {
  return !!r.silenced_until && new Date(r.silenced_until) > new Date()
}

function AlertsTab() {
  const toast = useToast()
  const [rules, setRules] = useState<AlertRule[]>([])
  const [events, setEvents] = useState<AlertEvent[]>([])
  const [rulesLoading, setRulesLoading] = useState(true)
  const [eventsLoading, setEventsLoading] = useState(true)
  const [rulesError, setRulesError] = useState('')
  const [eventsError, setEventsError] = useState('')

  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [condition, setCondition] = useState<Condition>('cpu_percent')
  const [threshold, setThreshold] = useState('80')
  const [severity, setSeverity] = useState<Severity>('warning')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  async function loadRules() {
    setRulesLoading(true)
    setRulesError('')
    try {
      setRules(await listAlertRules())
    } catch (e) {
      setRulesError(e instanceof ApiError ? e.message : 'Failed to load alert rules')
    } finally {
      setRulesLoading(false)
    }
  }

  async function loadEvents() {
    setEventsLoading(true)
    setEventsError('')
    try {
      setEvents(await listAlertEvents(200))
    } catch (e) {
      setEventsError(e instanceof ApiError ? e.message : 'Failed to load alert events')
    } finally {
      setEventsLoading(false)
    }
  }

  useEffect(() => {
    loadRules()
    loadEvents()
  }, [])

  function openAdd() {
    setEditingId(null)
    setName('')
    setCondition('cpu_percent')
    setThreshold('80')
    setSeverity('warning')
    setFormError('')
    setShowForm(true)
  }

  function openEdit(r: AlertRule) {
    setEditingId(r.id)
    setName(r.name)
    setCondition(r.condition)
    setThreshold(String(r.threshold))
    setSeverity(r.severity)
    setFormError('')
    setShowForm(true)
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    const n = Number(threshold)
    if (!name || Number.isNaN(n)) {
      setFormError('Name and a numeric threshold are required.')
      return
    }
    setSaving(true)
    setFormError('')
    try {
      if (editingId) {
        const updated = await updateAlertRule(editingId, { name, condition, threshold: n, severity })
        setRules((prev) => prev.map((r) => (r.id === editingId ? updated : r)))
        toast(`${name} updated`)
      } else {
        const created = await createAlertRule({ name, condition, threshold: n, severity, enabled: true, silenced_until: '' })
        setRules((prev) => [created, ...prev])
        toast(`${name} created`)
      }
      setShowForm(false)
    } catch (e2) {
      setFormError(e2 instanceof ApiError ? e2.message : 'Failed to save rule')
    } finally {
      setSaving(false)
    }
  }

  async function remove(r: AlertRule) {
    try {
      await deleteAlertRule(r.id)
      setRules((prev) => prev.filter((x) => x.id !== r.id))
      toast(`${r.name} removed`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    }
  }

  async function toggleEnabled(r: AlertRule) {
    try {
      const updated = await updateAlertRule(r.id, { ...r, enabled: !r.enabled })
      setRules((prev) => prev.map((x) => (x.id === r.id ? updated : x)))
      toast(`${r.name} ${r.enabled ? 'disabled' : 'enabled'}`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Update failed', 'danger')
    }
  }

  async function silence4h(r: AlertRule) {
    const until = new Date(Date.now() + 4 * 3600 * 1000).toISOString()
    try {
      const updated = await silenceAlertRule(r.id, until)
      setRules((prev) => prev.map((x) => (x.id === r.id ? updated : x)))
      toast(`${r.name} silenced for 4 hours`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Silence failed', 'danger')
    }
  }

  async function acknowledge(e: AlertEvent) {
    try {
      await acknowledgeAlertEvent(e.id)
      setEvents((prev) => prev.map((x) => (x.id === e.id ? { ...x, acknowledged_at: new Date().toISOString() } : x)))
      toast(`${e.rule_name} acknowledged`)
    } catch (err) {
      toast(err instanceof ApiError ? err.message : 'Acknowledge failed', 'danger')
    }
  }

  return (
    <>
      <Section action={<Button variant="ghost" onClick={openAdd}><Plus size={14} />Add rule</Button>}>
        {showForm && (
          <form className={styles.inlineForm} onSubmit={submit}>
            <div className={styles.formFields}>
              <TextField label="Name" id="ruleName" value={name} onChange={(e) => setName(e.target.value)} placeholder="high-cpu-hub" />
              <Select
                label="Condition"
                id="ruleCondition"
                value={condition}
                onChange={(e) => setCondition(e.target.value as Condition)}
                options={Object.entries(CONDITION_LABEL).map(([value, label]) => ({ value, label }))}
              />
              <TextField label="Threshold" id="ruleThreshold" value={threshold} onChange={(e) => setThreshold(e.target.value)} placeholder="85" />
              <Select
                label="Severity"
                id="ruleSeverity"
                value={severity}
                onChange={(e) => setSeverity(e.target.value as Severity)}
                options={[
                  { value: 'info', label: 'Info' },
                  { value: 'warning', label: 'Warning' },
                  { value: 'critical', label: 'Critical' },
                ]}
              />
            </div>
            {formError && <p className={styles.formError}>{formError}</p>}
            <div className={styles.formActions}>
              <Button type="submit" disabled={saving}>
                {saving ? 'Saving…' : editingId ? 'Save rule' : 'Create rule'}
              </Button>
            </div>
          </form>
        )}

        {rulesError ? (
          <Callout variant="danger">{rulesError}</Callout>
        ) : rulesLoading ? (
          <div className={styles.loadingRow}>Loading…</div>
        ) : rules.length === 0 ? (
          <EmptyState icon={Bell} title="No alert rules" description="Add a rule to get notified when a threshold is crossed." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Condition</th>
                <th>Threshold</th>
                <th>Severity</th>
                <th>Status</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {rules.map((r) => (
                <tr key={r.id}>
                  <td className="cell-name">{r.name}</td>
                  <td className="cell-muted">{CONDITION_LABEL[r.condition]}</td>
                  <td className="cell-muted">{r.condition === 'node_offline' || r.condition === 'handshake_age' ? `${r.threshold} min` : `${r.threshold}%`}</td>
                  <td>
                    <Badge variant={SEVERITY_VARIANT[r.severity]}>{r.severity}</Badge>
                  </td>
                  <td className="cell-muted">{!r.enabled ? 'Disabled' : isSilenced(r) ? `Silenced until ${fmtDate(r.silenced_until)}` : 'Active'}</td>
                  <td>
                    <div className={styles.rowActions}>
                      <button type="button" onClick={() => silence4h(r)} aria-label={`Silence ${r.name} for 4 hours`}>
                        <BellOff size={14} />
                      </button>
                      <button type="button" onClick={() => toggleEnabled(r)} aria-label={r.enabled ? `Disable ${r.name}` : `Enable ${r.name}`}>
                        <Bell size={14} />
                      </button>
                      <button type="button" onClick={() => openEdit(r)} aria-label={`Edit ${r.name}`}>
                        <Pencil size={14} />
                      </button>
                      <button type="button" className={styles.rowActionDanger} onClick={() => remove(r)} aria-label={`Remove ${r.name}`}>
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

      <Section title="Recent events">
        {eventsError ? (
          <Callout variant="danger">{eventsError}</Callout>
        ) : eventsLoading ? (
          <div className={styles.loadingRow}>Loading…</div>
        ) : events.length === 0 ? (
          <EmptyState title="No alert events" description="Fired alerts will show up here." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Rule</th>
                <th>Severity</th>
                <th>Message</th>
                <th>Fired</th>
                <th>Resolved</th>
                <th>Acknowledged</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {events.map((e) => (
                <tr key={e.id}>
                  <td className="cell-name">{e.rule_name}</td>
                  <td>
                    <Badge variant={SEVERITY_VARIANT[e.severity] ?? 'neutral'}>{e.severity}</Badge>
                  </td>
                  <td className="cell-muted">{e.message}</td>
                  <td className="cell-muted">{fmtDate(e.fired_at)}</td>
                  <td className="cell-muted">{fmtDate(e.resolved_at)}</td>
                  <td className="cell-muted">{fmtDate(e.acknowledged_at)}</td>
                  <td>
                    {!e.acknowledged_at && (
                      <div className={styles.rowActions}>
                        <button type="button" onClick={() => acknowledge(e)} aria-label={`Acknowledge ${e.rule_name}`}>
                          <Check size={14} />
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Section>
    </>
  )
}

export default AlertsTab
