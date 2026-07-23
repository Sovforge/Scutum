import { useState, type FormEvent } from 'react'
import { Pencil, Plus, Send, Trash2, Webhook as WebhookIcon } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import PasswordField from '../../../components/ui/PasswordField/PasswordField'
import Checkbox from '../../../components/ui/Checkbox/Checkbox'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import styles from '../Settings.module.css'

// Real endpoints: GET/POST/PUT/DELETE /webhooks(/:id) + POST
// /webhooks/:id/test. `secret` is write-only (HMAC signing, never
// returned by the API — same pattern as storage backends' secret_key).
// The event checklist is fixed to the real webhook events already used
// elsewhere in this app's mock data (mockData.ts's LOG_ENTRIES) — not
// invented. No delivery-status/retry history exists in the real UI.
type WebhookConfig = {
  id: string
  name: string
  url: string
  events: string[]
  enabled: boolean
}

const EVENTS = ['node.enrolled', 'node.offline', 'node.online', 'healer.service_restart', 'audit.critical', 'user.created', 'auth.sso_login']

const INITIAL_WEBHOOKS: WebhookConfig[] = [
  { id: 'wh1', name: 'slack-alerts', url: 'https://hooks.slack.com/services/T000/B000/xxxxxxxx', events: ['audit.critical', 'node.offline', 'healer.service_restart'], enabled: true },
  { id: 'wh2', name: 'siem-forwarder', url: 'https://siem.internal/ingest/scutum', events: ['audit.critical', 'auth.sso_login', 'user.created'], enabled: true },
]

function WebhooksTab() {
  const toast = useToast()
  const [webhooks, setWebhooks] = useState(INITIAL_WEBHOOKS)
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [events, setEvents] = useState<string[]>([])
  const [formError, setFormError] = useState('')

  function openAdd() {
    setEditingId(null)
    setName('')
    setUrl('')
    setSecret('')
    setEvents([])
    setFormError('')
    setShowForm(true)
  }

  function openEdit(w: WebhookConfig) {
    setEditingId(w.id)
    setName(w.name)
    setUrl(w.url)
    setSecret('')
    setEvents(w.events)
    setFormError('')
    setShowForm(true)
  }

  function toggleEvent(event: string) {
    setEvents((prev) => (prev.includes(event) ? prev.filter((e) => e !== event) : [...prev, event]))
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!name || !url) {
      setFormError('Name and URL are required.')
      return
    }
    if (!url.startsWith('https://')) {
      setFormError('URL must start with https://.')
      return
    }
    setFormError('')
    if (editingId) {
      setWebhooks((prev) => prev.map((w) => (w.id === editingId ? { ...w, name, url, events } : w)))
      toast(`${name} updated`)
    } else {
      setWebhooks((prev) => [...prev, { id: name, name, url, events, enabled: true }])
      toast(`${name} created`)
    }
    setShowForm(false)
  }

  function remove(w: WebhookConfig) {
    setWebhooks((prev) => prev.filter((x) => x.id !== w.id))
    toast(`${w.name} removed`, 'danger')
  }

  function toggleEnabled(w: WebhookConfig) {
    setWebhooks((prev) => prev.map((x) => (x.id === w.id ? { ...x, enabled: !x.enabled } : x)))
    toast(`${w.name} ${w.enabled ? 'disabled' : 'enabled'}`)
  }

  function test(w: WebhookConfig) {
    toast(`Test delivery sent to ${w.name}`)
  }

  return (
    <Section action={<Button variant="ghost" onClick={openAdd}><Plus size={14} />Add webhook</Button>}>
      {showForm && (
        <form className={styles.inlineForm} onSubmit={submit}>
          <div className={styles.formFields}>
            <TextField label="Name" id="whName" value={name} onChange={(e) => setName(e.target.value)} placeholder="slack-alerts" />
            <TextField label="URL" id="whUrl" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://hooks.slack.com/services/..." />
            <PasswordField
              label={editingId ? 'Secret (optional)' : 'Secret'}
              id="whSecret"
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
              hint={<p className={styles.fieldHint}>Used to HMAC-sign the payload{editingId ? ' — leave blank to keep unchanged' : ''}.</p>}
            />
          </div>
          <div className={styles.roleChecks}>
            <span className={styles.fieldLabel}>Events</span>
            <div className={styles.roleCheckList}>
              {EVENTS.map((event) => (
                <Checkbox key={event} checked={events.includes(event)} onChange={() => toggleEvent(event)}>
                  {event}
                </Checkbox>
              ))}
            </div>
          </div>
          {formError && <p className={styles.formError}>{formError}</p>}
          <div className={styles.formActions}>
            <Button type="submit">{editingId ? 'Save webhook' : 'Create webhook'}</Button>
          </div>
        </form>
      )}

      {webhooks.length === 0 ? (
        <EmptyState icon={WebhookIcon} title="No webhooks" description="Add a webhook to notify an external system when events happen." />
      ) : (
        <Table>
          <thead>
            <tr>
              <th>Name</th>
              <th>URL</th>
              <th>Events</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {webhooks.map((w) => (
              <tr key={w.id}>
                <td className="cell-name">{w.name}</td>
                <td className="cell-muted">{w.url}</td>
                <td>
                  <div className={styles.badgeRow}>
                    {w.events.map((e) => (
                      <Badge variant="neutral" key={e}>
                        {e}
                      </Badge>
                    ))}
                  </div>
                </td>
                <td>
                  <button type="button" className={styles.statusToggle} onClick={() => toggleEnabled(w)}>
                    <Badge variant={w.enabled ? 'success' : 'neutral'}>{w.enabled ? 'Enabled' : 'Disabled'}</Badge>
                  </button>
                </td>
                <td>
                  <div className={styles.rowActions}>
                    <button type="button" onClick={() => test(w)} aria-label={`Test ${w.name}`}>
                      <Send size={14} />
                    </button>
                    <button type="button" onClick={() => openEdit(w)} aria-label={`Edit ${w.name}`}>
                      <Pencil size={14} />
                    </button>
                    <button type="button" className={styles.rowActionDanger} onClick={() => remove(w)} aria-label={`Remove ${w.name}`}>
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
  )
}

export default WebhooksTab
