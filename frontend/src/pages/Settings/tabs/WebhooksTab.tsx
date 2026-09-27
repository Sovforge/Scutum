import { useEffect, useState, type FormEvent } from 'react'
import { Pencil, Plus, Send, Trash2, Webhook as WebhookIcon } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import PasswordField from '../../../components/ui/PasswordField/PasswordField'
import Checkbox from '../../../components/ui/Checkbox/Checkbox'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import { ApiError, createWebhook, deleteWebhook, listWebhooks, testWebhook, updateWebhook, type WebhookConfig } from '../../../lib/api'
import styles from '../Settings.module.css'

// Real endpoints: GET/POST/PUT/DELETE /webhooks(/:id) + POST
// /webhooks/:id/test. The event checklist is fixed to the real webhook
// events emitted by the Go handlers. There's no delivery-status/retry
// history in the real API — "Test" just fires once and reports success or
// failure via toast, matching the real endpoint's behavior exactly.
const EVENTS = ['node.enrolled', 'node.offline', 'node.online', 'healer.service_restart', 'audit.critical', 'user.created', 'auth.sso_login']

function WebhooksTab() {
  const toast = useToast()
  const [webhooks, setWebhooks] = useState<WebhookConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [testingId, setTestingId] = useState('')
  const [deletingId, setDeletingId] = useState('')

  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [events, setEvents] = useState<string[]>([])
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  async function load() {
    setLoading(true)
    setApiError('')
    try {
      setWebhooks(await listWebhooks())
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load webhooks')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

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

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!name || !url) {
      setFormError('Name and URL are required.')
      return
    }
    if (!url.startsWith('https://')) {
      setFormError('URL must start with https://.')
      return
    }
    if (!events.length) {
      setFormError('Select at least one event.')
      return
    }
    setSaving(true)
    setFormError('')
    try {
      const editing = editingId ? webhooks.find((w) => w.id === editingId) : undefined
      // The PUT endpoint has no "leave secret unchanged" semantics — an
      // empty secret field replaces the stored one with "". Carry the
      // existing secret forward when the field was left blank.
      const payload = { name, url, secret: secret || editing?.secret || '', events, enabled: editing?.enabled ?? true }
      if (editingId) {
        await updateWebhook(editingId, payload)
        toast(`${name} updated`)
      } else {
        await createWebhook(payload)
        toast(`${name} created`)
      }
      await load()
      setShowForm(false)
    } catch (e2) {
      setFormError(e2 instanceof ApiError ? e2.message : 'Failed to save webhook')
    } finally {
      setSaving(false)
    }
  }

  async function remove(w: WebhookConfig) {
    setDeletingId(w.id)
    try {
      await deleteWebhook(w.id)
      await load()
      toast(`${w.name} removed`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    } finally {
      setDeletingId('')
    }
  }

  async function toggleEnabled(w: WebhookConfig) {
    try {
      await updateWebhook(w.id, { name: w.name, url: w.url, secret: w.secret ?? '', events: w.events, enabled: !w.enabled })
      await load()
      toast(`${w.name} ${w.enabled ? 'disabled' : 'enabled'}`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Update failed', 'danger')
    }
  }

  async function test(w: WebhookConfig) {
    setTestingId(w.id)
    try {
      await testWebhook(w.id)
      toast(`Test delivery sent to ${w.name}`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Test delivery failed', 'danger')
    } finally {
      setTestingId('')
    }
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
            <Button type="submit" disabled={saving}>
              {saving ? 'Saving…' : editingId ? 'Save webhook' : 'Create webhook'}
            </Button>
          </div>
        </form>
      )}

      {apiError ? (
        <Callout variant="danger">{apiError}</Callout>
      ) : loading ? (
        <div className={styles.loadingRow}>Loading…</div>
      ) : webhooks.length === 0 ? (
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
                    <button type="button" onClick={() => test(w)} disabled={testingId === w.id} aria-label={`Test ${w.name}`}>
                      <Send size={14} />
                    </button>
                    <button type="button" onClick={() => openEdit(w)} aria-label={`Edit ${w.name}`}>
                      <Pencil size={14} />
                    </button>
                    <button
                      type="button"
                      className={styles.rowActionDanger}
                      onClick={() => remove(w)}
                      disabled={deletingId === w.id}
                      aria-label={`Remove ${w.name}`}
                    >
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
