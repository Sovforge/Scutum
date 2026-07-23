import { Fragment, useState, type FormEvent } from 'react'
import { ChevronDown, HardDrive, Plug, Plus, Trash2 } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import Checkbox from '../../components/ui/Checkbox/Checkbox'
import Select from '../../components/ui/Select/Select'
import TextField from '../../components/ui/TextField/TextField'
import PasswordField from '../../components/ui/PasswordField/PasswordField'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import styles from './Storage.module.css'

type Provider = 'minio' | 'r2' | 'aws' | 'b2' | 'ceph'
type BackendStatus = 'connected' | 'unreachable' | 'untested'
type Bucket = { name: string; createdAt: string }

// Mock data shaped like the real endpoints (GET/POST/DELETE
// /storage/backends, POST /storage/backends/{id}/test, GET
// /storage/backends/{id}/buckets) — there's no rename/edit endpoint on the
// real backend at all, and no object-browsing inside a bucket either
// (the old Nuxt frontend's "Browse" button is disabled with a "coming
// soon" tooltip, kept honest here the same way). `secret_key` is write-only
// on the real API — it's taken on the add form and never stored or shown
// again, matching how the backend never returns it either. Not wired to a
// live backend yet.
type Backend = {
  id: string
  name: string
  provider: Provider
  endpoint: string
  region: string
  accessKey: string
  pathStyle: boolean
  useSsl: boolean
  status: BackendStatus
  buckets: Bucket[]
}

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

// backup.internal is the same endpoint backup-job (on Containers) writes
// its restic repository to — RESTIC_REPOSITORY=s3:https://backup.internal/scutum.
const INITIAL_BACKENDS: Backend[] = [
  {
    id: 'hub-storage',
    name: 'hub-storage',
    provider: 'minio',
    endpoint: 'https://backup.internal:9000',
    region: 'us-east-1',
    accessKey: 'scutum-backup',
    pathStyle: true,
    useSsl: true,
    status: 'connected',
    buckets: [
      { name: 'scutum-backups', createdAt: '2026-06-01 00:00' },
      { name: 'grafana-snapshots', createdAt: '2026-06-15 00:00' },
    ],
  },
  {
    id: 'r2-offsite',
    name: 'r2-offsite',
    provider: 'r2',
    endpoint: 'https://a1b2c3d4.r2.cloudflarestorage.com',
    region: 'auto',
    accessKey: 'r2-scutum-key',
    pathStyle: false,
    useSsl: true,
    status: 'connected',
    buckets: [{ name: 'offsite-backups', createdAt: '2026-05-20 00:00' }],
  },
  {
    id: 'ceph-cold-archive',
    name: 'ceph-cold-archive',
    provider: 'ceph',
    endpoint: 'https://ceph-archive.internal:8080',
    region: '',
    accessKey: 'ceph-ro-key',
    pathStyle: true,
    useSsl: false,
    status: 'unreachable',
    buckets: [],
  },
]

function Storage() {
  const toast = useToast()
  const [backends, setBackends] = useState(INITIAL_BACKENDS)
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [provider, setProvider] = useState<Provider>('minio')
  const [endpoint, setEndpoint] = useState('')
  const [region, setRegion] = useState('')
  const [accessKey, setAccessKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [pathStyle, setPathStyle] = useState(false)
  const [useSsl, setUseSsl] = useState(true)
  const [formError, setFormError] = useState('')

  const totalBuckets = backends.reduce((sum, b) => sum + b.buckets.length, 0)
  const connected = backends.filter((b) => b.status === 'connected').length
  const unreachable = backends.filter((b) => b.status === 'unreachable').length

  function toggleExpand(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function testBackend(b: Backend) {
    if (b.status === 'unreachable') {
      toast(`${b.name}: connection failed — dial tcp: connection refused`, 'danger')
      return
    }
    setBackends((prev) => prev.map((x) => (x.id === b.id ? { ...x, status: 'connected' } : x)))
    toast(`${b.name}: connected, ${b.buckets.length} bucket${b.buckets.length !== 1 ? 's' : ''} found`)
  }

  function removeBackend(b: Backend) {
    setBackends((prev) => prev.filter((x) => x.id !== b.id))
    toast(`${b.name} removed`, 'danger')
  }

  function submitAdd(e: FormEvent) {
    e.preventDefault()
    if (!name || !endpoint || !accessKey || !secretKey) {
      setFormError('Name, endpoint, access key, and secret key are required.')
      return
    }
    setFormError('')
    setBackends((prev) => [
      ...prev,
      { id: name, name, provider, endpoint, region, accessKey, pathStyle, useSsl, status: 'untested', buckets: [] },
    ])
    toast(`${name} added — test the connection to confirm it's reachable`, 'info')
    setName('')
    setEndpoint('')
    setRegion('')
    setAccessKey('')
    setSecretKey('')
    setPathStyle(false)
    setUseSsl(true)
    setShowAdd(false)
  }

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
                <Button type="submit">Add backend</Button>
              </div>
            </form>
          )}

          {backends.length === 0 ? (
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
                          <Badge variant="neutral">{PROVIDER_LABEL[b.provider]}</Badge>
                        </td>
                        <td className="cell-muted">{b.endpoint}</td>
                        <td className="cell-muted">{b.region || '—'}</td>
                        <td>
                          <Badge variant={STATUS_VARIANT[b.status]}>{b.status}</Badge>
                        </td>
                        <td className="cell-muted">{b.buckets.length}</td>
                        <td>
                          <div className={styles.rowActions}>
                            <button type="button" onClick={() => testBackend(b)} aria-label={`Test ${b.name}`}>
                              <Plug size={14} />
                            </button>
                            <button
                              type="button"
                              className={styles.rowActionDanger}
                              onClick={() => removeBackend(b)}
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
                            {b.buckets.length === 0 ? (
                              <p className={styles.bucketEmpty}>
                                {b.status === 'unreachable' ? 'Unreachable — test the connection to list buckets.' : 'No buckets found.'}
                              </p>
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
                                  {b.buckets.map((bucket) => (
                                    <tr key={bucket.name}>
                                      <td className="cell-name">{bucket.name}</td>
                                      <td className="cell-muted">{bucket.createdAt}</td>
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
