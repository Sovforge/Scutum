import { useState, type FormEvent } from 'react'
import { Copy, KeyRound, Plus, Trash2 } from 'lucide-react'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import styles from '../Settings.module.css'

// Real endpoints: GET/POST/DELETE /scim/tokens(/:id) — admin-only
// (auth.Require(store, "admin", "admin")) in the real backend. There's no
// endpoint/bearer-URL config beyond the fixed `${origin}/scim/v2` path the
// old app displays read-only — shown here via the real
// window.location.origin rather than a fake domain.
type ScimToken = { id: string; description: string; createdAt: string }

const INITIAL_TOKENS: ScimToken[] = [{ id: 'scim1', description: 'Okta provisioning', createdAt: '2026-06-01 10:00:00' }]

function randomToken() {
  return `scim_${Array.from({ length: 32 }, () => Math.floor(Math.random() * 16).toString(16)).join('')}`
}

function ScimTab() {
  const toast = useToast()
  const [tokens, setTokens] = useState(INITIAL_TOKENS)
  const [showForm, setShowForm] = useState(false)
  const [description, setDescription] = useState('')
  const [revealedToken, setRevealedToken] = useState<string | null>(null)

  const endpoint = `${window.location.origin}/scim/v2`

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!description) return
    setTokens((prev) => [...prev, { id: `scim_${prev.length + 1}`, description, createdAt: 'just now' }])
    setRevealedToken(randomToken())
    setDescription('')
    setShowForm(false)
  }

  function remove(t: ScimToken) {
    setTokens((prev) => prev.filter((x) => x.id !== t.id))
    toast(`${t.description} revoked`, 'danger')
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
              <Button type="submit" disabled={!description}>
                Create
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
                onClick={() => {
                  navigator.clipboard?.writeText(revealedToken)
                  toast('Copied to clipboard')
                  setRevealedToken(null)
                }}
                aria-label="Copy token"
              >
                <Copy size={14} />
              </button>
            </div>
          </div>
        )}

        {tokens.length === 0 ? (
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
                <tr key={t.id}>
                  <td className="cell-name">{t.description}</td>
                  <td className="cell-muted">{t.createdAt}</td>
                  <td>
                    <button type="button" className={styles.revokeBtn} onClick={() => remove(t)} aria-label={`Revoke ${t.description}`}>
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
