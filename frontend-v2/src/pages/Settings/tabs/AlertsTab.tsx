import { useState, type FormEvent } from 'react'
import { Bell, BellOff, Check, Pencil, Plus, Trash2 } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import Select from '../../../components/ui/Select/Select'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import styles from '../Settings.module.css'

// Real endpoints: GET/POST/PUT/DELETE /alerts/rules(/:id), PUT
// /alerts/rules/:id/silence {until}, GET /alerts/events?limit=, POST
// /alerts/events/:id/acknowledge. The node-offline event below matches
// the edge-nyc handshake entries already in mockData's LOG_ENTRIES.
type Condition = 'cpu_percent' | 'mem_percent' | 'disk_percent' | 'node_offline' | 'handshake_age'
type Severity = 'info' | 'warning' | 'critical'

type AlertRule = {
  id: string
  name: string
  condition: Condition
  threshold: number
  severity: Severity
  enabled: boolean
  silencedUntil: string | null
}

type AlertEvent = {
  id: string
  ruleName: string
  severity: Severity
  message: string
  firedAt: string
  resolvedAt: string | null
  acknowledgedAt: string | null
}

const CONDITION_LABEL: Record<Condition, string> = {
  cpu_percent: 'CPU %',
  mem_percent: 'Memory %',
  disk_percent: 'Disk %',
  node_offline: 'Node offline',
  handshake_age: 'Handshake age',
}

const SEVERITY_VARIANT: Record<Severity, 'neutral' | 'warning' | 'danger'> = {
  info: 'neutral',
  warning: 'warning',
  critical: 'danger',
}

const INITIAL_RULES: AlertRule[] = [
  { id: 'rule1', name: 'high-cpu-hub', condition: 'cpu_percent', threshold: 85, severity: 'warning', enabled: true, silencedUntil: null },
  { id: 'rule2', name: 'node-offline', condition: 'node_offline', threshold: 1, severity: 'critical', enabled: true, silencedUntil: null },
  { id: 'rule3', name: 'disk-pressure', condition: 'disk_percent', threshold: 90, severity: 'critical', enabled: true, silencedUntil: null },
]

const INITIAL_EVENTS: AlertEvent[] = [
  { id: 'evt1', ruleName: 'node-offline', severity: 'critical', message: 'edge-nyc missed 3 consecutive handshakes', firedAt: '2026-07-09 13:40:12', resolvedAt: '2026-07-09 14:32:07', acknowledgedAt: '2026-07-09 13:41:00' },
  { id: 'evt2', ruleName: 'high-cpu-hub', severity: 'warning', message: 'CPU on build-runner sustained above 85% for 5m', firedAt: '2026-07-09 09:02:44', resolvedAt: null, acknowledgedAt: null },
  { id: 'evt3', ruleName: 'disk-pressure', severity: 'critical', message: 'edge-london disk usage at 92%', firedAt: '2026-07-08 22:15:00', resolvedAt: null, acknowledgedAt: '2026-07-09 09:00:00' },
]

function AlertsTab() {
  const toast = useToast()
  const [rules, setRules] = useState(INITIAL_RULES)
  const [events, setEvents] = useState(INITIAL_EVENTS)

  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [condition, setCondition] = useState<Condition>('cpu_percent')
  const [threshold, setThreshold] = useState('80')
  const [severity, setSeverity] = useState<Severity>('warning')
  const [formError, setFormError] = useState('')

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

  function submit(e: FormEvent) {
    e.preventDefault()
    const n = Number(threshold)
    if (!name || Number.isNaN(n)) {
      setFormError('Name and a numeric threshold are required.')
      return
    }
    setFormError('')
    if (editingId) {
      setRules((prev) => prev.map((r) => (r.id === editingId ? { ...r, name, condition, threshold: n, severity } : r)))
      toast(`${name} updated`)
    } else {
      setRules((prev) => [...prev, { id: name, name, condition, threshold: n, severity, enabled: true, silencedUntil: null }])
      toast(`${name} created`)
    }
    setShowForm(false)
  }

  function remove(r: AlertRule) {
    setRules((prev) => prev.filter((x) => x.id !== r.id))
    toast(`${r.name} removed`, 'danger')
  }

  function toggleEnabled(r: AlertRule) {
    setRules((prev) => prev.map((x) => (x.id === r.id ? { ...x, enabled: !x.enabled } : x)))
    toast(`${r.name} ${r.enabled ? 'disabled' : 'enabled'}`)
  }

  function silence4h(r: AlertRule) {
    setRules((prev) => prev.map((x) => (x.id === r.id ? { ...x, silencedUntil: 'in 4 hours' } : x)))
    toast(`${r.name} silenced for 4 hours`)
  }

  function acknowledge(e: AlertEvent) {
    setEvents((prev) => prev.map((x) => (x.id === e.id ? { ...x, acknowledgedAt: 'just now' } : x)))
    toast(`${e.ruleName} acknowledged`)
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
              <Button type="submit">{editingId ? 'Save rule' : 'Create rule'}</Button>
            </div>
          </form>
        )}

        {rules.length === 0 ? (
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
                  <td className="cell-muted">{r.threshold}</td>
                  <td>
                    <Badge variant={SEVERITY_VARIANT[r.severity]}>{r.severity}</Badge>
                  </td>
                  <td className="cell-muted">
                    {!r.enabled ? 'Disabled' : r.silencedUntil ? `Silenced ${r.silencedUntil}` : 'Active'}
                  </td>
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
        {events.length === 0 ? (
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
                  <td className="cell-name">{e.ruleName}</td>
                  <td>
                    <Badge variant={SEVERITY_VARIANT[e.severity]}>{e.severity}</Badge>
                  </td>
                  <td className="cell-muted">{e.message}</td>
                  <td className="cell-muted">{e.firedAt}</td>
                  <td className="cell-muted">{e.resolvedAt ?? '—'}</td>
                  <td className="cell-muted">{e.acknowledgedAt ?? '—'}</td>
                  <td>
                    {!e.acknowledgedAt && (
                      <div className={styles.rowActions}>
                        <button type="button" onClick={() => acknowledge(e)} aria-label={`Acknowledge ${e.ruleName}`}>
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
