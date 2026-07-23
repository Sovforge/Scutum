import { useState, type FormEvent } from 'react'
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
import styles from '../Settings.module.css'

// Unlike every other tab in Settings, this one has NO real backend at all
// — not "not wired up yet" (a real endpoint waiting to be connected), but
// genuinely nothing: the old Nuxt frontend's settings/secrets.vue is a
// hardcoded local array with dead View/Rotate/Delete buttons and no API
// calls anywhere. Built here with working local interactions instead
// (consistent with how every other mocked page in this app behaves), but
// disclosed honestly as a concept with nothing behind it server-side.
type SecretType = 'Opaque' | 'TLS' | 'DockerRegistry' | 'APIKey'
type Secret = {
  id: string
  name: string
  namespace: string
  type: SecretType
  createdAt: string
  usedBy: string[]
}

// grafana-admin-creds/minio-credentials/registry-pull-secret/ci-deploy-token
// tie to pods/containers/tokens already established on the Kubernetes,
// Containers, and Account pages.
const INITIAL_SECRETS: Secret[] = [
  { id: 'sec1', name: 'grafana-admin-creds', namespace: 'observability', type: 'Opaque', createdAt: '2026-07-05 11:00:00', usedBy: ['grafana-7c8b2'] },
  { id: 'sec2', name: 'minio-credentials', namespace: 'default', type: 'Opaque', createdAt: '2026-07-09 04:31:00', usedBy: ['minio'] },
  { id: 'sec3', name: 'registry-pull-secret', namespace: 'default', type: 'DockerRegistry', createdAt: '2026-07-08 09:00:00', usedBy: ['gitlab-runner'] },
  { id: 'sec4', name: 'wildcard-tls-cert', namespace: 'default', type: 'TLS', createdAt: '2026-07-08 21:10:00', usedBy: ['nginx-proxy'] },
  { id: 'sec5', name: 'ci-deploy-token', namespace: 'default', type: 'APIKey', createdAt: '2026-07-09 08:45:10', usedBy: ['gitlab-runner'] },
  { id: 'sec6', name: 'backup-restic-password', namespace: 'default', type: 'Opaque', createdAt: '2026-07-09 01:00:00', usedBy: ['backup-cronjob-28934720-lk9j2'] },
]

const TYPE_OPTIONS: { value: SecretType; label: string }[] = [
  { value: 'Opaque', label: 'Opaque' },
  { value: 'TLS', label: 'TLS' },
  { value: 'DockerRegistry', label: 'Docker Registry' },
  { value: 'APIKey', label: 'API Key' },
]

function SecretsTab() {
  const toast = useToast()
  const [secrets, setSecrets] = useState(INITIAL_SECRETS)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [namespace, setNamespace] = useState('default')
  const [type, setType] = useState<SecretType>('Opaque')
  const [formError, setFormError] = useState('')

  function toggleExpand(id: string) {
    setExpanded((prev) => (prev === id ? null : id))
  }

  function rotate(s: Secret) {
    setSecrets((prev) => prev.map((x) => (x.id === s.id ? { ...x, createdAt: 'just now' } : x)))
    toast(`${s.name} rotated`)
  }

  function remove(s: Secret) {
    setSecrets((prev) => prev.filter((x) => x.id !== s.id))
    toast(`${s.name} deleted`, 'danger')
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!name || !namespace) {
      setFormError('Name and namespace are required.')
      return
    }
    setFormError('')
    setSecrets((prev) => [...prev, { id: name, name, namespace, type, createdAt: 'just now', usedBy: [] }])
    toast(`${name} created`)
    setName('')
    setNamespace('default')
    setType('Opaque')
    setShowForm(false)
  }

  return (
    <>
      <div className={styles.calloutWrap}>
        <Callout variant="warning">
          Secrets management has no real backend behind it at all — not here, not in the reference app either. Every action
          below works against local state only; nothing is encrypted, stored, or retrievable, and no actual secret material
          exists anywhere in this preview.
        </Callout>
      </div>

      <Section action={<Button variant="ghost" onClick={() => setShowForm((v) => !v)}><Plus size={14} />Create secret</Button>}>
        {showForm && (
          <form className={styles.inlineForm} onSubmit={submit}>
            <div className={styles.formFields}>
              <TextField label="Name" id="secretName" value={name} onChange={(e) => setName(e.target.value)} placeholder="db-credentials" />
              <TextField label="Namespace" id="secretNamespace" value={namespace} onChange={(e) => setNamespace(e.target.value)} />
              <Select label="Type" id="secretType" value={type} onChange={(e) => setType(e.target.value as SecretType)} options={TYPE_OPTIONS} />
            </div>
            {formError && <p className={styles.formError}>{formError}</p>}
            <div className={styles.formActions}>
              <Button type="submit">Create</Button>
            </div>
          </form>
        )}

        {secrets.length === 0 ? (
          <EmptyState icon={KeyRound} title="No secrets" description="Create a secret to reference from containers or pods." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Namespace</th>
                <th>Type</th>
                <th>Created</th>
                <th>Used by</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {secrets.map((s) => (
                <tr key={s.id}>
                  <td className="cell-name">{s.name}</td>
                  <td className="cell-muted">{s.namespace}</td>
                  <td>
                    <Badge variant="neutral">{s.type}</Badge>
                  </td>
                  <td className="cell-muted">{s.createdAt}</td>
                  <td className="cell-muted">{s.usedBy.length > 0 ? s.usedBy.join(', ') : '—'}</td>
                  <td>
                    <div className={styles.rowActions}>
                      <button type="button" onClick={() => toggleExpand(s.id)} aria-label={`View ${s.name}`}>
                        <Eye size={14} />
                      </button>
                      <button type="button" onClick={() => rotate(s)} aria-label={`Rotate ${s.name}`}>
                        <RotateCw size={14} />
                      </button>
                      <button type="button" className={styles.rowActionDanger} onClick={() => remove(s)} aria-label={`Delete ${s.name}`}>
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

      {expanded && (
        <Section title={`${secrets.find((s) => s.id === expanded)?.name} details`}>
          <p className={styles.secretDetail}>
            No value is shown here — this preview never generates or stores actual secret material, only the metadata a real
            Secret resource would carry (name, namespace, type, and what references it).
          </p>
        </Section>
      )}
    </>
  )
}

export default SecretsTab
