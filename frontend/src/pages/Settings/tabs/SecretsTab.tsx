import { useEffect, useState, type FormEvent } from 'react'
import { Eye, KeyRound, Plus, RotateCw, Trash2 } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import Select from '../../../components/ui/Select/Select'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import { ApiError, createSecret, deleteSecret, listSecrets, rotateSecret, type K8sSecretInfo } from '../../../lib/api'
import styles from '../Settings.module.css'

// Real Kubernetes Secret objects — GET/POST/DELETE /kubernetes/{ns}/secrets,
// POST /kubernetes/{ns}/secrets/{name}/rotate — same cluster and client the
// Kubernetes page's Deployments tab uses. Values never leave the server:
// list/get only ever return key names, matching the "no value is shown
// here" behavior this tab always disclosed, even back when it was a mock.
//
// The mock's fourth type, "API Key", isn't a real Kubernetes secret type —
// kubectl only recognizes Opaque and a handful of kubernetes.io/* types —
// so it's dropped here rather than sent to a real API as a made-up type
// string. An API key/token is just a single-field Opaque secret; the type
// picker below reflects that.
const TYPE_OPTIONS = [
  { value: 'Opaque', label: 'Opaque' },
  { value: 'kubernetes.io/tls', label: 'TLS' },
  { value: 'kubernetes.io/dockerconfigjson', label: 'Docker Registry' },
] as const
type SecretType = (typeof TYPE_OPTIONS)[number]['value']

function fmtTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

// Builds a real .dockerconfigjson blob from server/username/password/email
// — the same structure `kubectl create secret docker-registry` produces.
function buildDockerConfigJSON(server: string, username: string, password: string, email: string): string {
  const auth = btoa(`${username}:${password}`)
  return JSON.stringify({ auths: { [server]: { username, password, email, auth } } })
}

function SecretsTab() {
  const toast = useToast()
  const [secrets, setSecrets] = useState<K8sSecretInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [expanded, setExpanded] = useState<string | null>(null)
  const [busyKey, setBusyKey] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [namespace, setNamespace] = useState('default')
  const [type, setType] = useState<SecretType>('Opaque')
  const [opaquePairs, setOpaquePairs] = useState<{ key: string; value: string }[]>([{ key: '', value: '' }])
  const [tlsCert, setTlsCert] = useState('')
  const [tlsKey, setTlsKey] = useState('')
  const [dockerServer, setDockerServer] = useState('')
  const [dockerUsername, setDockerUsername] = useState('')
  const [dockerPassword, setDockerPassword] = useState('')
  const [dockerEmail, setDockerEmail] = useState('')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  function rowKey(s: K8sSecretInfo): string {
    return `${s.namespace}/${s.name}`
  }

  async function load() {
    setLoading(true)
    setApiError('')
    try {
      setSecrets(await listSecrets())
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load secrets')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  function toggleExpand(key: string) {
    setExpanded((prev) => (prev === key ? null : key))
  }

  function resetForm() {
    setName('')
    setNamespace('default')
    setType('Opaque')
    setOpaquePairs([{ key: '', value: '' }])
    setTlsCert('')
    setTlsKey('')
    setDockerServer('')
    setDockerUsername('')
    setDockerPassword('')
    setDockerEmail('')
    setFormError('')
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!name || !namespace) {
      setFormError('Name and namespace are required.')
      return
    }

    let data: Record<string, string> = {}
    if (type === 'Opaque') {
      for (const p of opaquePairs) {
        if (p.key) data[p.key] = p.value
      }
      if (Object.keys(data).length === 0) {
        setFormError('At least one key/value pair is required.')
        return
      }
    } else if (type === 'kubernetes.io/tls') {
      if (!tlsCert || !tlsKey) {
        setFormError('Certificate and private key are both required.')
        return
      }
      data = { 'tls.crt': tlsCert, 'tls.key': tlsKey }
    } else {
      if (!dockerServer || !dockerUsername || !dockerPassword) {
        setFormError('Registry server, username, and password are required.')
        return
      }
      data = { '.dockerconfigjson': buildDockerConfigJSON(dockerServer, dockerUsername, dockerPassword, dockerEmail) }
    }

    setFormError('')
    setSaving(true)
    try {
      await createSecret(namespace, name, type, data)
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

  async function rotate(s: K8sSecretInfo) {
    const key = rowKey(s)
    setBusyKey(key)
    try {
      await rotateSecret(s.namespace, s.name)
      toast(`${s.name} rotated`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Rotate failed', 'danger')
    } finally {
      setBusyKey(null)
    }
  }

  async function remove(s: K8sSecretInfo) {
    const key = rowKey(s)
    setBusyKey(key)
    try {
      await deleteSecret(s.namespace, s.name)
      setSecrets((prev) => prev.filter((x) => rowKey(x) !== key))
      setExpanded((prev) => (prev === key ? null : prev))
      toast(`${s.name} deleted`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    } finally {
      setBusyKey(null)
    }
  }

  function addPair() {
    setOpaquePairs((prev) => [...prev, { key: '', value: '' }])
  }
  function removePair(i: number) {
    setOpaquePairs((prev) => prev.filter((_, idx) => idx !== i))
  }
  function updatePair(i: number, field: 'key' | 'value', v: string) {
    setOpaquePairs((prev) => prev.map((p, idx) => (idx === i ? { ...p, [field]: v } : p)))
  }

  return (
    <>
      <Section action={<Button variant="ghost" onClick={() => setShowForm((v) => !v)}><Plus size={14} />Create secret</Button>}>
        {showForm && (
          <form className={styles.inlineForm} onSubmit={submit}>
            <div className={styles.formFields}>
              <TextField label="Name" id="secretName" value={name} onChange={(e) => setName(e.target.value)} placeholder="db-credentials" />
              <TextField label="Namespace" id="secretNamespace" value={namespace} onChange={(e) => setNamespace(e.target.value)} />
              <Select label="Type" id="secretType" value={type} onChange={(e) => setType(e.target.value as SecretType)} options={TYPE_OPTIONS} />
            </div>

            {type === 'Opaque' && (
              <div className={styles.secretPairs}>
                {opaquePairs.map((p, i) => (
                  <div className={styles.secretPairRow} key={i}>
                    <TextField label={i === 0 ? 'Key' : ''} id={`secretKey${i}`} value={p.key} onChange={(e) => updatePair(i, 'key', e.target.value)} placeholder="username" />
                    <TextField label={i === 0 ? 'Value' : ''} id={`secretValue${i}`} value={p.value} onChange={(e) => updatePair(i, 'value', e.target.value)} placeholder="admin" />
                    {opaquePairs.length > 1 && (
                      <button type="button" className={styles.rowActionDanger} onClick={() => removePair(i)} aria-label="Remove pair">
                        <Trash2 size={14} />
                      </button>
                    )}
                  </div>
                ))}
                <Button type="button" variant="ghost" onClick={addPair}>
                  <Plus size={14} />
                  Add key
                </Button>
              </div>
            )}

            {type === 'kubernetes.io/tls' && (
              <div className={styles.secretPairs}>
                <div className="field">
                  <label htmlFor="tlsCert">Certificate (PEM)</label>
                  <textarea id="tlsCert" className={styles.textarea} rows={4} value={tlsCert} onChange={(e) => setTlsCert(e.target.value)} placeholder="-----BEGIN CERTIFICATE-----" />
                </div>
                <div className="field">
                  <label htmlFor="tlsKey">Private key (PEM)</label>
                  <textarea id="tlsKey" className={styles.textarea} rows={4} value={tlsKey} onChange={(e) => setTlsKey(e.target.value)} placeholder="-----BEGIN PRIVATE KEY-----" />
                </div>
              </div>
            )}

            {type === 'kubernetes.io/dockerconfigjson' && (
              <div className={styles.formFields}>
                <TextField label="Registry server" id="dockerServer" value={dockerServer} onChange={(e) => setDockerServer(e.target.value)} placeholder="registry.example.com" />
                <TextField label="Username" id="dockerUsername" value={dockerUsername} onChange={(e) => setDockerUsername(e.target.value)} />
                <TextField label="Password" id="dockerPassword" value={dockerPassword} onChange={(e) => setDockerPassword(e.target.value)} type="password" />
                <TextField label="Email (optional)" id="dockerEmail" value={dockerEmail} onChange={(e) => setDockerEmail(e.target.value)} />
              </div>
            )}

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
          <EmptyState icon={KeyRound} title="No secrets" description="Create a secret to reference from containers or pods." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Namespace</th>
                <th>Type</th>
                <th>Created</th>
                <th>Keys</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {secrets.map((s) => {
                const key = rowKey(s)
                const busy = busyKey === key
                const canRotate = s.type === 'Opaque' || s.type === ''
                return (
                  <tr key={key}>
                    <td className="cell-name">{s.name}</td>
                    <td className="cell-muted">{s.namespace}</td>
                    <td>
                      <Badge variant="neutral">{s.type || 'Opaque'}</Badge>
                    </td>
                    <td className="cell-muted">{fmtTime(s.created_at)}</td>
                    <td className="cell-muted">{s.keys.length}</td>
                    <td>
                      <div className={styles.rowActions}>
                        <button type="button" onClick={() => toggleExpand(key)} aria-label={`View ${s.name}`}>
                          <Eye size={14} />
                        </button>
                        <button
                          type="button"
                          onClick={() => rotate(s)}
                          disabled={!canRotate || busy}
                          title={canRotate ? 'Rotate to fresh random values' : 'Rotation only supports Opaque secrets'}
                          aria-label={`Rotate ${s.name}`}
                        >
                          <RotateCw size={14} />
                        </button>
                        <button type="button" className={styles.rowActionDanger} onClick={() => remove(s)} disabled={busy} aria-label={`Delete ${s.name}`}>
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </Table>
        )}
      </Section>

      {expanded && (
        <Section title={`${secrets.find((s) => rowKey(s) === expanded)?.name} details`}>
          <p className={styles.secretDetail}>
            Value contents are never returned by the API — only the data key names a real Secret resource carries:
          </p>
          <div className={styles.codeGrid}>
            {secrets
              .find((s) => rowKey(s) === expanded)
              ?.keys.map((k) => (
                <span className={styles.codeChip} key={k}>
                  {k}
                </span>
              ))}
          </div>
        </Section>
      )}
    </>
  )
}

export default SecretsTab
