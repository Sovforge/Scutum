import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, RefreshCw, Trash2 } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import Select from '../../components/ui/Select/Select'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import Callout from '../../components/ui/Callout/Callout'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { ApiError, deleteK8sPod, getK8sPod, getK8sPodLogsJSON, type K8sLogLine } from '../../lib/api'
import styles from './PodDetail.module.css'

function phaseVariant(phase: string): 'success' | 'warning' | 'danger' | 'neutral' {
  if (phase === 'Running') return 'success'
  if (phase === 'Pending') return 'warning'
  if (phase === 'Succeeded') return 'neutral'
  return 'danger'
}
function containerStateVariant(state: string): 'success' | 'warning' | 'neutral' {
  if (state === 'running') return 'success'
  if (state === 'waiting') return 'warning'
  return 'neutral'
}
function fmtTs(ts: string): string {
  if (!ts) return ''
  try {
    return new Date(ts).toLocaleTimeString()
  } catch {
    return ts
  }
}

function PodDetail() {
  const { namespace = '', name = '' } = useParams()
  const navigate = useNavigate()

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [raw, setRaw] = useState<any>(null)
  const [selectedContainer, setSelectedContainer] = useState('')
  const [logLines, setLogLines] = useState<K8sLogLine[]>([])
  const [logsLoading, setLogsLoading] = useState(false)

  useEffect(() => {
    getK8sPod(namespace, name)
      .then((r) => setRaw(r))
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Failed to load pod.'))
      .finally(() => setLoading(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [namespace, name])

  const meta = useMemo(() => {
    const r = raw
    if (!r) {
      return {
        name,
        namespace,
        phase: '—',
        podIP: '—',
        hostIP: '—',
        nodeName: '—',
        serviceAccount: '—',
        uid: '—',
        created: '—',
        startTime: '—',
        restartPolicy: '—',
        labels: {} as Record<string, string>,
      }
    }
    const md = r.metadata ?? {}
    const spec = r.spec ?? {}
    const status = r.status ?? {}
    return {
      name: md.name ?? name,
      namespace: md.namespace ?? namespace,
      phase: status.phase ?? '—',
      podIP: status.podIP ?? '—',
      hostIP: status.hostIP ?? '—',
      nodeName: spec.nodeName ?? '—',
      serviceAccount: spec.serviceAccountName ?? 'default',
      uid: md.uid ?? '—',
      created: md.creationTimestamp ? new Date(md.creationTimestamp).toLocaleString() : '—',
      startTime: status.startTime ? new Date(status.startTime).toLocaleString() : '—',
      restartPolicy: spec.restartPolicy ?? '—',
      labels: (md.labels ?? {}) as Record<string, string>,
    }
  }, [raw, name, namespace])

  const conditions = useMemo(
    () => (raw?.status?.conditions ?? []).map((c: any) => ({ type: c.type, status: c.status, reason: c.reason ?? '' })),
    [raw],
  )

  const containers = useMemo(() => {
    const specContainers: any[] = raw?.spec?.containers ?? []
    const statusContainers: any[] = raw?.status?.containerStatuses ?? []
    const statusMap = Object.fromEntries(statusContainers.map((s: any) => [s.name, s]))
    return specContainers.map((c: any) => {
      const s = statusMap[c.name] ?? {}
      const stateKeys = Object.keys(s.state ?? {})
      const state = stateKeys[0] ?? '—'
      const res = c.resources ?? {}
      return {
        name: c.name,
        image: c.image ?? '—',
        ready: s.ready ?? false,
        restarts: s.restartCount ?? 0,
        state,
        cpuReq: res.requests?.cpu ?? '—',
        cpuLim: res.limits?.cpu ?? '—',
        memReq: res.requests?.memory ?? '—',
        memLim: res.limits?.memory ?? '—',
      }
    })
  }, [raw])

  const volumes = useMemo(
    () =>
      (raw?.spec?.volumes ?? []).map((v: any) => {
        const type = Object.keys(v).find((k) => k !== 'name') ?? 'unknown'
        const source = JSON.stringify(v[type] ?? {})
        return { name: v.name, type, source }
      }),
    [raw],
  )

  useEffect(() => {
    if (containers.length > 0 && !selectedContainer) {
      setSelectedContainer(containers[0].name)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [containers])

  useEffect(() => {
    if (!selectedContainer) return
    setLogsLoading(true)
    getK8sPodLogsJSON(namespace, name, selectedContainer)
      .then(setLogLines)
      .catch(() => setLogLines([]))
      .finally(() => setLogsLoading(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedContainer])

  function refreshLogs() {
    if (!selectedContainer) return
    setLogsLoading(true)
    getK8sPodLogsJSON(namespace, name, selectedContainer)
      .then(setLogLines)
      .catch(() => setLogLines([]))
      .finally(() => setLogsLoading(false))
  }

  async function remove() {
    if (!confirm(`Delete pod ${name}?`)) return
    try {
      await deleteK8sPod(namespace, name)
      navigate('/kubernetes')
    } catch (e) {
      alert(e instanceof ApiError ? e.message : 'Delete failed.')
    }
  }

  if (loading) {
    return (
      <AppShell title="Kubernetes">
        <div className={styles.page}>Loading…</div>
      </AppShell>
    )
  }

  if (error) {
    return (
      <AppShell title="Kubernetes">
        <EmptyState
          title="Pod not found"
          description={error}
          action={
            <Button as={Link} to="/kubernetes" variant="ghost">
              <ArrowLeft size={14} />
              Back to Kubernetes
            </Button>
          }
        />
      </AppShell>
    )
  }

  return (
    <AppShell title={meta.name}>
      <div className={styles.page}>
        <Link to="/kubernetes" className={styles.backLink}>
          <ArrowLeft size={14} />
          Back to Kubernetes
        </Link>

        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">{meta.namespace}</p>
            <h2 className={styles.reportTitle}>{meta.name}</h2>
          </div>
          <div className={styles.headerRight}>
            <Badge variant={phaseVariant(meta.phase)}>{meta.phase}</Badge>
            <Button variant="ghost" onClick={remove}>
              <Trash2 size={14} />
              Delete
            </Button>
          </div>
        </div>

        <div className={styles.infoGrid}>
          <Section title="Pod details">
            <dl className={styles.detailList}>
              <div>
                <dt>Pod IP</dt>
                <dd className={styles.mono}>{meta.podIP}</dd>
              </div>
              <div>
                <dt>Host IP</dt>
                <dd className={styles.mono}>{meta.hostIP}</dd>
              </div>
              <div>
                <dt>Node</dt>
                <dd>{meta.nodeName}</dd>
              </div>
              <div>
                <dt>Namespace</dt>
                <dd>{meta.namespace}</dd>
              </div>
              <div>
                <dt>Service account</dt>
                <dd>{meta.serviceAccount}</dd>
              </div>
              <div>
                <dt>UID</dt>
                <dd className={styles.mono}>{meta.uid}</dd>
              </div>
              <div>
                <dt>Created</dt>
                <dd>{meta.created}</dd>
              </div>
              <div>
                <dt>Started</dt>
                <dd>{meta.startTime}</dd>
              </div>
              <div>
                <dt>Restart policy</dt>
                <dd>{meta.restartPolicy}</dd>
              </div>
            </dl>
          </Section>

          <Section title="Conditions">
            {conditions.length === 0 ? (
              <EmptyState title="No conditions reported" />
            ) : (
              <dl className={styles.detailList}>
                {conditions.map((c: any) => (
                  <div key={c.type}>
                    <dt>{c.type}</dt>
                    <dd>
                      <Badge variant={c.status === 'True' ? 'success' : c.status === 'False' ? 'danger' : 'neutral'}>{c.status}</Badge>
                      {c.reason && <span className={styles.conditionReason}> {c.reason}</span>}
                    </dd>
                  </div>
                ))}
              </dl>
            )}
          </Section>

          <Section title="Labels">
            {Object.keys(meta.labels).length === 0 ? (
              <EmptyState title="No labels" />
            ) : (
              <ul className={styles.labelList}>
                {Object.entries(meta.labels).map(([k, v]) => (
                  <li key={k}>
                    <span className={styles.labelKey}>{k}</span>
                    <span className={styles.labelVal}>{v}</span>
                  </li>
                ))}
              </ul>
            )}
          </Section>
        </div>

        <Section title="Containers">
          <Table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Image</th>
                <th>Ready</th>
                <th>State</th>
                <th>Restarts</th>
                <th>CPU req/lim</th>
                <th>Mem req/lim</th>
              </tr>
            </thead>
            <tbody>
              {containers.map((c: any) => (
                <tr key={c.name} onClick={() => setSelectedContainer(c.name)} className={styles.row}>
                  <td className={c.name === selectedContainer ? styles.nameActive : 'cell-name'}>{c.name}</td>
                  <td className="cell-muted">{c.image}</td>
                  <td>
                    <span className={c.ready ? styles.readyDot : styles.notReadyDot} aria-label={c.ready ? 'Ready' : 'Not ready'} />
                  </td>
                  <td>
                    <Badge variant={containerStateVariant(c.state)}>{c.state}</Badge>
                  </td>
                  <td className="cell-muted">{c.restarts}</td>
                  <td className="cell-muted">
                    {c.cpuReq} / {c.cpuLim}
                  </td>
                  <td className="cell-muted">
                    {c.memReq} / {c.memLim}
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
        </Section>

        <Section title="Volumes">
          {volumes.length === 0 ? (
            <EmptyState title="No volumes" description="This pod doesn't mount any volumes." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Type</th>
                  <th>Source</th>
                </tr>
              </thead>
              <tbody>
                {volumes.map((v: any) => (
                  <tr key={v.name}>
                    <td className="cell-name">{v.name}</td>
                    <td>
                      <Badge variant="neutral">{v.type}</Badge>
                    </td>
                    <td className="cell-muted">{v.source || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Section>

        <Section
          title="Logs"
          action={
            <div className={styles.logActions}>
              {containers.length > 1 && (
                <Select
                  label="Container"
                  id="logContainer"
                  value={selectedContainer}
                  onChange={(e) => setSelectedContainer(e.target.value)}
                  options={containers.map((c: any) => ({ value: c.name, label: c.name }))}
                />
              )}
              <button type="button" className={styles.refreshBtn} onClick={refreshLogs} disabled={logsLoading} aria-label="Refresh logs">
                <RefreshCw size={14} />
              </button>
            </div>
          }
        >
          <div className={styles.calloutWrap}>
            <Callout variant="info">Logs are fetched on demand, not streamed live — there's no tail-follow websocket wired up yet.</Callout>
          </div>
          {logsLoading ? (
            <div className={styles.logList}>Loading…</div>
          ) : logLines.length === 0 ? (
            <EmptyState title="No log lines" description="This container hasn't emitted anything yet." />
          ) : (
            <div className={styles.logList}>
              {logLines.map((l, i) => (
                <div className={styles.logRow} key={i}>
                  <span className={styles.logTime}>{fmtTs(l.ts)}</span>
                  <span className={styles.logMsg}>{l.msg}</span>
                </div>
              ))}
            </div>
          )}
        </Section>
      </div>
    </AppShell>
  )
}

export default PodDetail
