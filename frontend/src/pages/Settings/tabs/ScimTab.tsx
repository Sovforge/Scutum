import { useEffect, useState, type FormEvent } from 'react'
import { Copy, KeyRound, Plus, Trash2 } from 'lucide-react'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import { ApiError, createSCIMToken, deleteSCIMToken, listSCIMTokens, type ScimToken } from '../../../lib/api'
import styles from '../Settings.module.css'

// Real endpoints: GET/POST/DELETE /scim/tokens(/:id) — admin-only. There's
// no endpoint/bearer-URL config beyond the fixed `${origin}/scim/v2` path
// the old app displays read-only. The raw token is only ever returned once,
// by the create call — GET never includes it (same one-time-reveal pattern
// as recovery codes elsewhere in this app).

function fmtDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
  } catch {
    return iso
  }
}

function ScimTab() {
  const toast = useToast()
  const [tokens, setTokens] = useState<ScimToken[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [deletingId, setDeletingId] = useState('')

  const [showForm, setShowForm] = useState(false)
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)
  const [revealedToken, setRevealedToken] = useState<string | null>(null)

  const endpoint = `${window.location.origin}/scim/v2`

  async function load() {
    setLoading(true)
    setApiError('')
    try {
      setTokens(await listSCIMTokens())
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load SCIM tokens')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!description) return
    setSaving(true)
    try {
      const res = await createSCIMToken(description)
      setRevealedToken(res.token)
      setDescription('')
      setShowForm(false)
      await load()
    } catch (e2) {
      toast(e2 instanceof ApiError ? e2.message : 'Failed to create token', 'danger')
    } finally {
      setSaving(false)
    }
  }

  async function remove(t: ScimToken) {
    setDeletingId(t.ID)
    try {
      await deleteSCIMToken(t.ID)
      await load()
      toast(`${t.Description || 'Token'} revoked`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Revoke failed', 'danger')
    } finally {
      setDeletingId('')
    }
  }

  return (
    <>
      <Section title="Endpoint">
        <div className={styles.endpointRow}>
          <span className={styles.mono}>{endpoint}</span>
        </div>
      </Section>

      <Section title="Tokens" action={<Button variant="ghost" onClick={() => setShowForm((v) => !v)}><Plus size={14} />Create token</Button>}>
        {showForm && (
          <form className={styles.inlineForm} onSubmit={submit}>
            <div className={styles.formFields}>
              <TextField label="Description" id="scimDescription" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Okta provisioning" />
            </div>
            <div className={styles.formActions}>
              <Button type="submit" disabled={!description || saving}>
                {saving ? 'Creating…' : 'Create'}
              </Button>
            </div>
          </form>
        )}

        {revealedToken && (
          <div className={styles.calloutWrap}>
            <Callout variant="danger">Copy this token now — it won't be shown again.</Callout>
            <div className={styles.tokenReveal}>
              <span className={styles.mono}>{revealedToken}</span>
              <button
                type="button"
                className={styles.copyBtn}
                onClick={async () => {
                  try {
                    await navigator.clipboard.writeText(revealedToken)
                    toast('Copied to clipboard')
                  } catch {
                    /* clipboard permission denied */
                  }
                }}
                aria-label="Copy token"
              >
                <Copy size={14} />
              </button>
              <Button variant="ghost" onClick={() => setRevealedToken(null)}>
                Done
              </Button>
            </div>
          </div>
        )}

        {apiError ? (
          <Callout variant="danger">{apiError}</Callout>
        ) : loading ? (
          <div className={styles.loadingRow}>Loading…</div>
        ) : tokens.length === 0 ? (
          <EmptyState icon={KeyRound} title="No SCIM tokens" description="Create a token to let an identity provider provision users automatically." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Description</th>
                <th>Created</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {tokens.map((t) => (
                <tr key={t.ID}>
                  <td className="cell-name">{t.Description || '—'}</td>
                  <td className="cell-muted">{fmtDate(t.CreatedAt)}</td>
                  <td>
                    <button
                      type="button"
                      className={styles.revokeBtn}
                      onClick={() => remove(t)}
                      disabled={deletingId === t.ID}
                      aria-label={`Revoke ${t.Description || 'token'}`}
                    >
                      <Trash2 size={14} />
                    </button>
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

export default ScimTab
