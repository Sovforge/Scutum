import { Fragment, useEffect, useState, type FormEvent } from 'react'
import { ChevronDown, HardDrive, Plug, Plus, Trash2 } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import Checkbox from '../../components/ui/Checkbox/Checkbox'
import Select from '../../components/ui/Select/Select'
import TextField from '../../components/ui/TextField/TextField'
import PasswordField from '../../components/ui/PasswordField/PasswordField'
import Callout from '../../components/ui/Callout/Callout'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import {
  ApiError,
  createStorageBackend,
  deleteStorageBackend,
  listStorageBackends,
  listStorageBuckets,
  testStorageBackend,
  type BucketInfo,
  type StorageBackend,
} from '../../lib/api'
import styles from './Storage.module.css'

type Provider = 'minio' | 'r2' | 'aws' | 'b2' | 'ceph'
type BackendStatus = 'connected' | 'unreachable' | 'untested'

// There's no rename/edit endpoint on the real backend at all, and no
// object-browsing inside a bucket either (the old Nuxt frontend's "Browse"
// button is disabled with a "coming soon" tooltip, kept honest here the
// same way). `secret_key` is write-only on the real API — taken on the add
// form and never stored or shown again, matching how the backend never
// returns it either. "Connected"/"unreachable" is a client-side overlay
// derived from testing/listing buckets, not a field the backend persists.

const PROVIDER_LABEL: Record<Provider, string> = {
  minio: 'MinIO',
  r2: 'Cloudflare R2',
  aws: 'AWS S3',
  b2: 'Backblaze B2',
  ceph: 'Ceph',
}

const STATUS_VARIANT: Record<BackendStatus, 'success' | 'danger' | 'neutral'> = {
  connected: 'success',
  unreachable: 'danger',
  untested: 'neutral',
}

function fmtDate(iso?: string): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleDateString()
  } catch {
    return iso
  }
}

function Storage() {
  const toast = useToast()
  const [backends, setBackends] = useState<StorageBackend[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')

  const [status, setStatus] = useState<Record<string, BackendStatus>>({})
  const [buckets, setBuckets] = useState<Record<string, BucketInfo[]>>({})
  const [bucketsLoading, setBucketsLoading] = useState<Record<string, boolean>>({})
  const [bucketsError, setBucketsError] = useState<Record<string, string>>({})
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [testingId, setTestingId] = useState('')
  const [deletingId, setDeletingId] = useState('')

  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [provider, setProvider] = useState<Provider>('minio')
  const [endpoint, setEndpoint] = useState('')
  const [region, setRegion] = useState('')
  const [accessKey, setAccessKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [pathStyle, setPathStyle] = useState(false)
  const [useSsl, setUseSsl] = useState(true)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState('')

  async function loadBackends() {
    setLoading(true)
    setApiError('')
    try {
      const list = await listStorageBackends()
      setBackends(list)
      if (list.length > 0) {
        loadBuckets(list[0].id)
        setExpanded(new Set([list[0].id]))
      }
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load storage backends')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadBackends()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function loadBuckets(id: string) {
    setBucketsLoading((prev) => ({ ...prev, [id]: true }))
    setBucketsError((prev) => ({ ...prev, [id]: '' }))
    try {
      const list = await listStorageBuckets(id)
      setBuckets((prev) => ({ ...prev, [id]: list }))
      setStatus((prev) => ({ ...prev, [id]: 'connected' }))
    } catch (e) {
      setBucketsError((prev) => ({ ...prev, [id]: e instanceof ApiError ? e.message : 'Failed to reach backend' }))
      setStatus((prev) => ({ ...prev, [id]: 'unreachable' }))
    } finally {
      setBucketsLoading((prev) => ({ ...prev, [id]: false }))
    }
  }

  function toggleExpand(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
        if (!(id in buckets)) loadBuckets(id)
      }
      return next
    })
  }

  async function testBackend(b: StorageBackend) {
    setTestingId(b.id)
    try {
      const res = await testStorageBackend(b.id)
      setStatus((prev) => ({ ...prev, [b.id]: res.ok ? 'connected' : 'unreachable' }))
      toast(res.ok ? `${b.name}: connected, ${res.buckets ?? 0} bucket${res.buckets === 1 ? '' : 's'} found` : `${b.name}: ${res.error}`, res.ok ? 'success' : 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Test failed', 'danger')
    } finally {
      setTestingId('')
    }
  }

  async function removeBackend(b: StorageBackend) {
    setDeletingId(b.id)
    try {
      await deleteStorageBackend(b.id)
      setBackends((prev) => prev.filter((x) => x.id !== b.id))
      setBuckets((prev) => {
        const next = { ...prev }
        delete next[b.id]
        return next
      })
      setStatus((prev) => {
        const next = { ...prev }
        delete next[b.id]
        return next
      })
      setExpanded((prev) => {
        const next = new Set(prev)
        next.delete(b.id)
        return next
      })
      toast(`${b.name} removed`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    } finally {
      setDeletingId('')
    }
  }

  async function submitAdd(e: FormEvent) {
    e.preventDefault()
    if (!name || !endpoint || !accessKey || !secretKey) {
      setFormError('Name, endpoint, access key, and secret key are required.')
      return
    }
    setSaving(true)
    setFormError('')
    try {
      const b = await createStorageBackend({
        name,
        provider,
        endpoint,
        region: region || 'us-east-1',
        access_key: accessKey,
        secret_key: secretKey,
        path_style: pathStyle,
        use_ssl: useSsl,
      })
      setBackends((prev) => [...prev, b])
      toast(`${b.name} added — test the connection to confirm it's reachable`, 'info')
      setName('')
      setEndpoint('')
      setRegion('')
      setAccessKey('')
      setSecretKey('')
      setPathStyle(false)
      setUseSsl(true)
      setShowAdd(false)
    } catch (e) {
      setFormError(e instanceof ApiError ? e.message : 'Failed to add backend')
    } finally {
      setSaving(false)
    }
  }

  const totalBuckets = Object.values(buckets).reduce((sum, list) => sum + list.length, 0)
  const connected = backends.filter((b) => status[b.id] === 'connected').length
  const unreachable = backends.filter((b) => status[b.id] === 'unreachable').length

  return (
    <AppShell title="Storage">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Object Storage</p>
            <h2 className={styles.reportTitle}>Backend registry</h2>
          </div>
          <span className={unreachable === 0 ? 'stamp' : 'stamp stamp--alt'}>
            {connected}/{backends.length} connected
          </span>
        </div>

        <StatGrid minWidth={140}>
          <StatCard label="Backends" value={backends.length} icon={<HardDrive size={16} />} />
          <StatCard label="Buckets" value={totalBuckets} />
          <StatCard label="Connected" value={connected} color="var(--blueprint)" />
          <StatCard label="Unreachable" value={unreachable} color="var(--danger)" />
        </StatGrid>

        <Section action={<Button variant="ghost" onClick={() => setShowAdd((v) => !v)}><Plus size={14} />Add backend</Button>}>
          {showAdd && (
            <form className={styles.addForm} onSubmit={submitAdd}>
              <div className={styles.addFields}>
                <TextField label="Name" id="beName" value={name} onChange={(e) => setName(e.target.value)} placeholder="hub-storage" />
                <Select
                  label="Provider"
                  id="beProvider"
                  value={provider}
                  onChange={(e) => setProvider(e.target.value as Provider)}
                  options={Object.entries(PROVIDER_LABEL).map(([value, label]) => ({ value, label }))}
                />
                <TextField
                  label="Endpoint"
                  id="beEndpoint"
                  value={endpoint}
                  onChange={(e) => setEndpoint(e.target.value)}
                  placeholder="https://minio.internal:9000"
                />
                <TextField label="Region" id="beRegion" value={region} onChange={(e) => setRegion(e.target.value)} placeholder="us-east-1" />
                <TextField label="Access key" id="beAccessKey" value={accessKey} onChange={(e) => setAccessKey(e.target.value)} />
                <PasswordField label="Secret key" id="beSecretKey" value={secretKey} onChange={(e) => setSecretKey(e.target.value)} />
              </div>
              <div className={styles.addToggles}>
                <Checkbox checked={pathStyle} onChange={(e) => setPathStyle(e.target.checked)}>
                  Path-style addressing
                </Checkbox>
                <Checkbox checked={useSsl} onChange={(e) => setUseSsl(e.target.checked)}>
                  Use SSL
                </Checkbox>
              </div>
              {formError && <p className={styles.formError}>{formError}</p>}
              <div className={styles.addActions}>
                <Button type="submit" disabled={saving}>
                  {saving ? 'Adding…' : 'Add backend'}
                </Button>
              </div>
            </form>
          )}

          {apiError ? (
            <Callout variant="danger">{apiError}</Callout>
          ) : loading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : backends.length === 0 ? (
            <EmptyState icon={HardDrive} title="No storage backends" description="Add an S3-compatible backend to start browsing buckets." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Provider</th>
                  <th>Endpoint</th>
                  <th>Region</th>
                  <th>Status</th>
                  <th>Buckets</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {backends.map((b) => {
                  const isExpanded = expanded.has(b.id)
                  const st = status[b.id] ?? 'untested'
                  const bucketList = buckets[b.id] ?? []
                  return (
                    <Fragment key={b.id}>
                      <tr>
                        <td>
                          <button type="button" className={styles.expandBtn} onClick={() => toggleExpand(b.id)}>
                            <ChevronDown size={14} className={isExpanded ? undefined : styles.chevronCollapsed} />
                            <span className="cell-name">{b.name}</span>
                          </button>
                        </td>
                        <td>
                          <Badge variant="neutral">{PROVIDER_LABEL[b.provider as Provider] ?? b.provider}</Badge>
                        </td>
                        <td className="cell-muted">{b.endpoint}</td>
                        <td className="cell-muted">{b.region || '—'}</td>
                        <td>
                          <Badge variant={STATUS_VARIANT[st]}>{st}</Badge>
                        </td>
                        <td className="cell-muted">{bucketList.length}</td>
                        <td>
                          <div className={styles.rowActions}>
                            <button type="button" onClick={() => testBackend(b)} disabled={testingId === b.id} aria-label={`Test ${b.name}`}>
                              <Plug size={14} />
                            </button>
                            <button
                              type="button"
                              className={styles.rowActionDanger}
                              onClick={() => removeBackend(b)}
                              disabled={deletingId === b.id}
                              aria-label={`Remove ${b.name}`}
                            >
                              <Trash2 size={14} />
                            </button>
                          </div>
                        </td>
                      </tr>
                      {isExpanded && (
                        <tr>
                          <td colSpan={7} className={styles.bucketCell}>
                            {bucketsLoading[b.id] ? (
                              <p className={styles.bucketEmpty}>Loading buckets…</p>
                            ) : bucketsError[b.id] ? (
                              <p className={styles.bucketEmpty}>{bucketsError[b.id]}</p>
                            ) : bucketList.length === 0 ? (
                              <p className={styles.bucketEmpty}>No buckets found.</p>
                            ) : (
                              <table className={styles.bucketTable}>
                                <thead>
                                  <tr>
                                    <th>Bucket</th>
                                    <th>Created</th>
                                    <th />
                                  </tr>
                                </thead>
                                <tbody>
                                  {bucketList.map((bucket) => (
                                    <tr key={bucket.name}>
                                      <td className="cell-name">{bucket.name}</td>
                                      <td className="cell-muted">{fmtDate(bucket.created_at)}</td>
                                      <td>
                                        <Button variant="ghost" disabled title="Object browsing isn't available yet">
                                          Browse
                                        </Button>
                                      </td>
                                    </tr>
                                  ))}
                                </tbody>
                              </table>
                            )}
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
      </div>
    </AppShell>
  )
}

export default Storage
