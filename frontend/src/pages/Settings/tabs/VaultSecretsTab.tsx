import { Fragment, useEffect, useState, type FormEvent } from 'react'
import { Eye, EyeOff, Lock, Pencil, Plus, Trash2 } from 'lucide-react'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import {
  ApiError,
  createVaultSecret,
  deleteVaultSecret,
  listVaultSecrets,
  updateVaultSecret,
  type VaultSecret,
} from '../../../lib/api'
import styles from '../Settings.module.css'

// KMS-backed named secrets. Values are write-only from here on out — create
// and update send a plaintext value, but nothing this page ever fetches back
// (including the list itself) contains one. Reference a secret from a
// container/pod env var, or a Compose/Kubernetes YAML manifest, with
// "secret://<name>"; the hub resolves it to the real value at deploy time,
// so it never crosses the wire to a target node as a bare reference.
function fmtTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function VaultSecretsTab() {
  const toast = useToast()
  const [secrets, setSecrets] = useState<VaultSecret[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [busyName, setBusyName] = useState<string | null>(null)
  const [editing, setEditing] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [value, setValue] = useState('')
  const [showValue, setShowValue] = useState(false)
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  const [editDescription, setEditDescription] = useState('')
  const [editValue, setEditValue] = useState('')
  const [editShowValue, setEditShowValue] = useState(false)

  async function load() {
    setLoading(true)
    setApiError('')
    try {
      setSecrets(await listVaultSecrets())
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load secrets')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  function resetForm() {
    setName('')
    setDescription('')
    setValue('')
    setShowValue(false)
    setFormError('')
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!name) {
      setFormError('Name is required.')
      return
    }
    if (!value) {
      setFormError('Value is required.')
      return
    }
    setFormError('')
    setSaving(true)
    try {
      await createVaultSecret(name, description, value)
      toast(`${name} created`)
      resetForm()
      setShowForm(false)
      load()
    } catch (e2) {
      setFormError(e2 instanceof ApiError ? e2.message : 'Failed to create secret')
    } finally {
      setSaving(false)
    }
  }

  function startEdit(s: VaultSecret) {
    setEditing((prev) => (prev === s.name ? null : s.name))
    setEditDescription(s.description)
    setEditValue('')
    setEditShowValue(false)
  }

  async function saveEdit(s: VaultSecret) {
    setBusyName(s.name)
    try {
      const fields: { description?: string; value?: string } = {}
      if (editDescription !== s.description) fields.description = editDescription
      if (editValue) fields.value = editValue
      if (Object.keys(fields).length === 0) {
        setEditing(null)
        return
      }
      await updateVaultSecret(s.name, fields)
      toast(`${s.name} updated`)
      setEditing(null)
      load()
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Update failed', 'danger')
    } finally {
      setBusyName(null)
    }
  }

  async function remove(s: VaultSecret) {
    setBusyName(s.name)
    try {
      await deleteVaultSecret(s.name)
      setSecrets((prev) => prev.filter((x) => x.name !== s.name))
      setEditing((prev) => (prev === s.name ? null : prev))
      toast(`${s.name} deleted`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    } finally {
      setBusyName(null)
    }
  }

  return (
    <>
      <Callout variant="info">
        Reference a secret from a container's env vars, or a Compose/Kubernetes manifest, with{' '}
        <code className={styles.mono}>secret://&lt;name&gt;</code> — it's resolved to the real value on the hub at
        deploy time and never sent to a node as a bare reference.
      </Callout>

      <Section action={<Button variant="ghost" onClick={() => setShowForm((v) => !v)}><Plus size={14} />New secret</Button>}>
        {showForm && (
          <form className={styles.inlineForm} onSubmit={submit}>
            <div className={styles.formFields}>
              <TextField label="Name" id="vaultName" value={name} onChange={(e) => setName(e.target.value)} placeholder="prod/db/password" />
              <TextField label="Description (optional)" id="vaultDescription" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Primary database password" />
            </div>
            <div className={styles.secretPairRow}>
              <TextField
                label="Value"
                id="vaultValue"
                type={showValue ? 'text' : 'password'}
                value={value}
                onChange={(e) => setValue(e.target.value)}
              />
              <Button type="button" variant="ghost" onClick={() => setShowValue((v) => !v)} aria-label={showValue ? 'Hide value' : 'Show value'}>
                {showValue ? <EyeOff size={14} /> : <Eye size={14} />}
              </Button>
            </div>
            {formError && <p className={styles.formError}>{formError}</p>}
            <div className={styles.formActions}>
              <Button type="submit" disabled={saving}>
                {saving ? 'Creating…' : 'Create'}
              </Button>
            </div>
          </form>
        )}

        {apiError ? (
          <Callout variant="danger">{apiError}</Callout>
        ) : loading ? (
          <div className={styles.loadingRow}>Loading…</div>
        ) : secrets.length === 0 ? (
          <EmptyState icon={Lock} title="No secrets" description="Create a secret to reference from containers, Compose, or Kubernetes manifests." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Description</th>
                <th>Updated</th>
                <th>By</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {secrets.map((s) => {
                const busy = busyName === s.name
                const isEditing = editing === s.name
                return (
                  <Fragment key={s.name}>
                    <tr>
                      <td className="cell-name">{s.name}</td>
                      <td className="cell-muted">{s.description || '—'}</td>
                      <td className="cell-muted">{fmtTime(s.updated_at)}</td>
                      <td className="cell-muted">{s.updated_by || '—'}</td>
                      <td>
                        <div className={styles.rowActions}>
                          <button type="button" onClick={() => startEdit(s)} disabled={busy} aria-label={`Edit ${s.name}`}>
                            <Pencil size={14} />
                          </button>
                          <button type="button" className={styles.rowActionDanger} onClick={() => remove(s)} disabled={busy} aria-label={`Delete ${s.name}`}>
                            <Trash2 size={14} />
                          </button>
                        </div>
                      </td>
                    </tr>
                    {isEditing && (
                      <tr key={`${s.name}-edit`}>
                        <td colSpan={5}>
                          <div className={styles.secretPairs}>
                            <TextField
                              label="Description"
                              id={`editDesc-${s.name}`}
                              value={editDescription}
                              onChange={(e) => setEditDescription(e.target.value)}
                            />
                            <div className={styles.secretPairRow}>
                              <TextField
                                label="New value (leave blank to keep current)"
                                id={`editValue-${s.name}`}
                                type={editShowValue ? 'text' : 'password'}
                                value={editValue}
                                onChange={(e) => setEditValue(e.target.value)}
                              />
                              <Button type="button" variant="ghost" onClick={() => setEditShowValue((v) => !v)} aria-label={editShowValue ? 'Hide value' : 'Show value'}>
                                {editShowValue ? <EyeOff size={14} /> : <Eye size={14} />}
                              </Button>
                            </div>
                            <div className={styles.formActions}>
                              <Button type="button" onClick={() => saveEdit(s)} disabled={busy}>
                                Save
                              </Button>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })}
            </tbody>
          </Table>
        )}
      </Section>
    </>
  )
}

export default VaultSecretsTab
