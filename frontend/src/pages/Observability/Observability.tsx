import { useEffect, useRef, useState } from 'react'
import { Activity, Box, Download, Gauge, Layers, Route, ScrollText, SearchX, Zap } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import ResourceBar from '../../components/ui/ResourceBar/ResourceBar'
import Select from '../../components/ui/Select/Select'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import Tabs, { type TabItem } from '../../components/ui/Tabs/Tabs'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import { formatBytes } from '../../lib/format'
import {
  ApiError,
  getContainerStats,
  getContainerTraces,
  getK8sSummary,
  listContainers,
  listLogs,
  listMetrics,
  listNodes,
  listTraces,
  scrapeContainerMetrics,
  streamHeaders,
  streamUrl,
  type ContainerStats,
  type DockerContainer,
  type K8sSummary,
  type LogEntry,
  type MetricPoint,
  type NodeRecord,
  type TraceEntry,
} from '../../lib/api'
import styles from './Observability.module.css'

type Tab = 'logs' | 'metrics' | 'traces' | 'otel'
type LogLevel = 'debug' | 'info' | 'warn' | 'error'
type LogSource = 'app' | 'k8s' | 'docker'
type MetricsTab = 'docker' | 'kubernetes'

// "Metrics" (container/cluster resource stats) and "OTel Metrics" (raw
// OpenTelemetry data points) are two distinct real tabs — not the same
// thing. Application logs (this page) are separate from the CRA-compliant
// audit trail at GET /audit/logs (its own page).

type NodeContainer = DockerContainer & { nodeId: string | null; nodeName: string }
type NodeSummary = { nodeId: string | null; nodeName: string; summary: K8sSummary | null }

const LEVEL_VARIANT: Record<LogLevel, 'neutral' | 'success' | 'warning' | 'danger'> = {
  debug: 'neutral',
  info: 'success',
  warn: 'warning',
  error: 'danger',
}

const STATUS_VARIANT: Record<TraceEntry['status'], 'success' | 'danger' | 'neutral'> = {
  ok: 'success',
  error: 'danger',
  unset: 'neutral',
}

const TABS: TabItem[] = [
  { id: 'logs', label: 'Logs', icon: ScrollText },
  { id: 'metrics', label: 'Metrics', icon: Activity },
  { id: 'traces', label: 'Traces', icon: Route },
  { id: 'otel', label: 'OTel Metrics', icon: Gauge },
]

const METRICS_TABS: TabItem[] = [
  { id: 'docker', label: 'Docker', icon: Box },
  { id: 'kubernetes', label: 'Kubernetes', icon: Layers },
]

function fmtLabels(labels?: Record<string, string>) {
  if (!labels) return '—'
  return Object.entries(labels)
    .map(([k, v]) => `${k}=${v}`)
    .join(', ')
}

function fmtTime(iso: string): string {
  try {
    return new Date(iso).toLocaleTimeString()
  } catch {
    return iso
  }
}

function containerName(c: DockerContainer): string {
  return (c.Names?.[0] ?? c.Id.slice(0, 12)).replace(/^\//, '')
}

function shortImage(image: string): string {
  return image.replace(/^[^/]+\/[^/]+\//, '').replace(/^[^/]+\//, '')
}

// Strips Docker's 8-byte multiplexed-stream frame header from each line.
function parseDockerLine(line: string): LogEntry | null {
  const clean = line.replace(/^[\x00-\x02][\x00]{3}[\x00-\xff]{4}/, '').trim()
  if (!clean) return null
  return { time: new Date().toISOString(), level: 'info', message: clean }
}

function parseK8sLine(line: string): LogEntry | null {
  if (!line.trim()) return null
  try {
    const ev = JSON.parse(line)
    const obj = ev.object
    if (!obj) return null
    return {
      time: obj.lastTimestamp || obj.firstTimestamp || new Date().toISOString(),
      level: obj.type === 'Warning' ? 'warn' : 'info',
      message: `[${obj.reason ?? '?'}] ${obj.message ?? ''} — ${obj.involvedObject?.kind}/${obj.involvedObject?.name}`,
    }
  } catch {
    return null
  }
}

function Observability() {
  const toast = useToast()
  const [tab, setTab] = useState<Tab>('logs')

  // ── Cross-node containers (backs Logs' docker source, Metrics' docker
  // sub-tab, and the Traces/OTel "collect from container" selectors) ───────
  const [containers, setContainers] = useState<NodeContainer[]>([])
  const [containerStats, setContainerStats] = useState<Record<string, ContainerStats>>({})
  const [metricsLoading, setMetricsLoading] = useState(false)
  const [k8sNodeSummaries, setK8sNodeSummaries] = useState<NodeSummary[]>([])
  const [k8sLoading, setK8sLoading] = useState(false)

  async function loadMetrics(isStale: () => boolean) {
    setMetricsLoading(true)
    setK8sLoading(true)
    setContainers([])
    setK8sNodeSummaries([])
    const nodes = await listNodes().catch(() => [] as NodeRecord[])
    if (isStale()) return
    const edgeNodes = nodes.filter((n) => n.type !== 'hub')
    const allNodes = [{ id: null as string | null, name: 'Local' }, ...edgeNodes.map((n) => ({ id: n.id, name: n.name }))]

    try {
      const all: NodeContainer[] = []
      const local = await listContainers().catch(() => [] as DockerContainer[])
      all.push(...local.map((c) => ({ ...c, nodeId: null, nodeName: 'Local' })))
      for (const n of edgeNodes) {
        const ctrs = await listContainers(n.id).catch(() => [] as DockerContainer[])
        all.push(...ctrs.map((c) => ({ ...c, nodeId: n.id, nodeName: n.name })))
      }
      if (isStale()) return
      setContainers(all)
      const running = all.filter((c) => c.State === 'running')
      if (running.length > 0) {
        Promise.allSettled(running.map((c) => getContainerStats(c.Id, c.nodeId).then((s) => [c.Id, s] as const))).then(
          (results) => {
            if (isStale()) return
            const map: Record<string, ContainerStats> = {}
            for (const r of results) if (r.status === 'fulfilled') map[r.value[0]] = r.value[1]
            setContainerStats(map)
          },
        )
      }
    } finally {
      if (!isStale()) setMetricsLoading(false)
    }

    try {
      const summaries = await Promise.all(
        allNodes.map((n): Promise<NodeSummary> =>
          getK8sSummary(n.id)
            .then((summary): NodeSummary => ({ nodeId: n.id, nodeName: n.name, summary }))
            .catch((): NodeSummary => ({ nodeId: n.id, nodeName: n.name, summary: null })),
        ),
      )
      if (!isStale()) setK8sNodeSummaries(summaries)
    } finally {
      if (!isStale()) setK8sLoading(false)
    }
  }

  // StrictMode double-invokes effects in dev; loadMetrics appends via
  // functional setState in places, so guard against a stale second run
  // clobbering or duplicating results (see Kubernetes.tsx for the same
  // pattern and the key bug it fixed).
  useEffect(() => {
    let stale = false
    loadMetrics(() => stale)
    return () => {
      stale = true
    }
  }, [])

  // ── Logs ─────────────────────────────────────────────────────────────────
  const [logSource, setLogSource] = useState<LogSource>('app')
  const [logLevel, setLogLevel] = useState<LogLevel | ''>('')
  const [logQuery, setLogQuery] = useState('')
  const [logFollow, setLogFollow] = useState(true)
  const [logLoading, setLogLoading] = useState(false)
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [selectedContainerId, setSelectedContainerId] = useState('')
  const streamAbort = useRef<AbortController | null>(null)
  const logPanelRef = useRef<HTMLDivElement | null>(null)

  async function loadAppLogs() {
    setLogLoading(true)
    try {
      setLogs(await listLogs())
    } catch {
      /* backend may not have entries yet */
    } finally {
      setLogLoading(false)
    }
  }

  async function streamLines(path: string, parseLine: (line: string) => LogEntry | null, nodeId?: string | null) {
    streamAbort.current?.abort()
    const controller = new AbortController()
    streamAbort.current = controller
    setLogLoading(true)
    setLogs([])
    try {
      const res = await fetch(streamUrl(path), { headers: streamHeaders(nodeId), signal: controller.signal })
      if (!res.ok || !res.body) {
        setLogLoading(false)
        return
      }
      const reader = res.body.getReader()
      setLogLoading(false)
      const decoder = new TextDecoder()
      let buf = ''
      for (;;) {
        const { done, value } = await reader.read()
        if (done) break
        buf += decoder.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() ?? ''
        for (const line of lines) {
          const entry = parseLine(line)
          if (entry) setLogs((prev) => [...prev, entry])
        }
      }
    } catch (e) {
      if ((e as Error)?.name !== 'AbortError') {
        /* stream closed */
      }
    } finally {
      setLogLoading(false)
    }
  }

  function refreshLogs() {
    if (logSource === 'app') {
      streamAbort.current?.abort()
      streamAbort.current = null
      loadAppLogs()
    } else if (logSource === 'k8s') {
      streamLines('/kubernetes/events', parseK8sLine)
    } else if (logSource === 'docker' && selectedContainerId) {
      const target = containers.find((c) => c.Id === selectedContainerId)
      streamLines(`/docker/containers/${selectedContainerId}/logs`, parseDockerLine, target?.nodeId ?? null)
    }
  }

  useEffect(() => {
    setLogs([])
    setLogLevel('')
    setLogQuery('')
    refreshLogs()
    return () => {
      streamAbort.current?.abort()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [logSource])

  useEffect(() => {
    if (logSource === 'docker' && selectedContainerId) refreshLogs()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedContainerId])

  const filteredLogs = logs.filter((l) => {
    if (logLevel && l.level !== logLevel) return false
    if (logQuery && !l.message.toLowerCase().includes(logQuery.toLowerCase())) return false
    return true
  })
  const errorCount = logs.filter((l) => l.level === 'error').length

  useEffect(() => {
    if (!logFollow) return
    const el = logPanelRef.current
    if (el) el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
  }, [filteredLogs.length, logFollow])

  const [metricsTab, setMetricsTab] = useState<MetricsTab>('docker')

  // ── Metrics derived totals ───────────────────────────────────────────────
  const dockerRunning = containers.filter((c) => c.State === 'running').length
  const k8sReachable = k8sNodeSummaries.filter((n) => n.summary).length
  const k8sTotals = k8sNodeSummaries.reduce(
    (acc, n) => {
      if (!n.summary) return acc
      acc.pods += n.summary.pods
      acc.running += n.summary.running
      acc.deployments += n.summary.deployments
      acc.nodes += n.summary.nodes
      return acc
    },
    { pods: 0, running: 0, deployments: 0, nodes: 0 },
  )

  // ── Traces ───────────────────────────────────────────────────────────────
  const [traces, setTraces] = useState<TraceEntry[]>([])
  const [tracesLoading, setTracesLoading] = useState(false)
  const [tracesLoaded, setTracesLoaded] = useState(false)
  const [collectContainer, setCollectContainer] = useState('')
  const [collecting, setCollecting] = useState(false)

  async function refreshTraces() {
    setTracesLoading(true)
    try {
      setTraces(await listTraces())
      setTracesLoaded(true)
    } catch {
      /* ignore */
    } finally {
      setTracesLoading(false)
    }
  }

  async function collectFromContainer() {
    if (!collectContainer) return
    setCollecting(true)
    try {
      const spans = await getContainerTraces(collectContainer)
      setTraces((t) => [...spans, ...t])
      const c = containers.find((d) => d.Id === collectContainer)
      toast(`Collected ${spans.length} span${spans.length === 1 ? '' : 's'} from ${c ? containerName(c) : collectContainer}`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Failed to collect spans', 'danger')
    } finally {
      setCollecting(false)
    }
  }

  // ── OTel Metrics ─────────────────────────────────────────────────────────
  const [otel, setOtel] = useState<MetricPoint[]>([])
  const [otelLoading, setOtelLoading] = useState(false)
  const [otelLoaded, setOtelLoaded] = useState(false)
  const [scrapeContainer, setScrapeContainer] = useState('')
  const [scrapePort, setScrapePort] = useState('9090')
  const [scraping, setScraping] = useState(false)

  async function refreshOtel() {
    setOtelLoading(true)
    try {
      setOtel(await listMetrics())
      setOtelLoaded(true)
    } catch {
      /* ignore */
    } finally {
      setOtelLoading(false)
    }
  }

  async function runScrape() {
    if (!scrapeContainer) return
    setScraping(true)
    try {
      const pts = await scrapeContainerMetrics(scrapeContainer, scrapePort)
      setOtel((m) => [...pts, ...m])
      const c = containers.find((d) => d.Id === scrapeContainer)
      toast(`Scraped ${pts.length} point${pts.length === 1 ? '' : 's'} from ${c ? containerName(c) : scrapeContainer}:${scrapePort}`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Scrape failed', 'danger')
    } finally {
      setScraping(false)
    }
  }

  useEffect(() => {
    if (tab === 'traces' && !tracesLoaded) refreshTraces()
    if (tab === 'otel' && !otelLoaded) refreshOtel()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab])

  const containerOptions = [
    { value: '', label: 'Select container…' },
    ...containers.map((c) => ({ value: c.Id, label: `${c.nodeName} · ${containerName(c)}` })),
  ]

  return (
    <AppShell title="Observability">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Telemetry</p>
            <h2 className={styles.reportTitle}>Signal report</h2>
          </div>
          {errorCount > 0 ? (
            <span className="stamp stamp--alt">{errorCount} error{errorCount !== 1 ? 's' : ''} logged</span>
          ) : (
            <span className="stamp">Nominal</span>
          )}
        </div>

        <Tabs tabs={TABS} active={tab} onChange={(id) => setTab(id as Tab)} />

        {tab === 'logs' && (
          <Section>
            <div className={styles.toolbar}>
              <select
                className={styles.toolbarSelect}
                value={logSource}
                onChange={(e) => setLogSource(e.target.value as LogSource)}
              >
                <option value="app">Application</option>
                <option value="k8s">Kubernetes Events</option>
                <option value="docker">Docker Container</option>
              </select>
              {logSource === 'docker' && (
                <select
                  className={styles.toolbarSelect}
                  value={selectedContainerId}
                  onChange={(e) => setSelectedContainerId(e.target.value)}
                >
                  {containerOptions.map((o) => (
                    <option key={o.value} value={o.value}>
                      {o.label}
                    </option>
                  ))}
                </select>
              )}
              <input
                className={styles.toolbarInput}
                placeholder="Filter logs…"
                value={logQuery}
                onChange={(e) => setLogQuery(e.target.value)}
              />
              <select className={styles.toolbarSelect} value={logLevel} onChange={(e) => setLogLevel(e.target.value as LogLevel | '')}>
                <option value="">All levels</option>
                <option value="error">Error</option>
                <option value="warn">Warn</option>
                <option value="info">Info</option>
                <option value="debug">Debug</option>
              </select>
              <label className={styles.followLabel}>
                <input type="checkbox" checked={logFollow} onChange={(e) => setLogFollow(e.target.checked)} /> Follow
              </label>
            </div>

            <div className={styles.logList} ref={logPanelRef}>
              {logLoading && logs.length === 0 ? (
                <div className={styles.loadingRow}>Loading…</div>
              ) : (
                <>
                  {filteredLogs.map((log, i) => (
                    <div className={styles.logRow} key={i}>
                      <span className={styles.logTime}>{fmtTime(log.time)}</span>
                      <Badge variant={LEVEL_VARIANT[log.level]}>{log.level}</Badge>
                      <span className={styles.logMessage}>{log.message}</span>
                    </div>
                  ))}
                  {filteredLogs.length === 0 && (
                    <EmptyState
                      icon={SearchX}
                      title={logSource === 'docker' && !selectedContainerId ? 'Select a container above' : 'No matching log entries'}
                      description={
                        logSource === 'docker' && !selectedContainerId
                          ? 'Pick a container to tail its logs.'
                          : 'Nothing matches the current filter and search query.'
                      }
                      action={
                        logQuery || logLevel ? (
                          <Button
                            variant="ghost"
                            onClick={() => {
                              setLogQuery('')
                              setLogLevel('')
                            }}
                          >
                            Clear filters
                          </Button>
                        ) : undefined
                      }
                    />
                  )}
                </>
              )}
            </div>
          </Section>
        )}

        {tab === 'metrics' && (
          <>
            <Tabs tabs={METRICS_TABS} active={metricsTab} onChange={(id) => setMetricsTab(id as MetricsTab)} variant="secondary" />

            {metricsTab === 'docker' && (
              <>
                <StatGrid minWidth={140}>
                  <StatCard label="Nodes" value={new Set(containers.map((c) => c.nodeName)).size} />
                  <StatCard label="Containers" value={containers.length} />
                  <StatCard label="Running" value={dockerRunning} />
                  <StatCard label="Stopped" value={containers.length - dockerRunning} />
                </StatGrid>
                <Section>
                  {metricsLoading ? (
                    <div className={styles.loadingRow}>Loading…</div>
                  ) : containers.length === 0 ? (
                    <EmptyState icon={Box} title="No containers found" />
                  ) : (
                    <Table>
                      <thead>
                        <tr>
                          <th>Node</th>
                          <th>Name</th>
                          <th>Image</th>
                          <th>State</th>
                          <th>CPU</th>
                          <th>Memory</th>
                          <th>Net I/O</th>
                          <th>Disk I/O</th>
                        </tr>
                      </thead>
                      <tbody>
                        {containers.map((c) => {
                          const stat = containerStats[c.Id]
                          return (
                            <tr key={`${c.nodeId ?? 'local'}-${c.Id}`}>
                              <td className="cell-muted">{c.nodeName}</td>
                              <td className="cell-name">{containerName(c)}</td>
                              <td className="cell-muted">{shortImage(c.Image)}</td>
                              <td>
                                <Badge variant={c.State === 'running' ? 'success' : 'neutral'}>{c.State}</Badge>
                              </td>
                              {stat ? (
                                <>
                                  <td>
                                    <ResourceBar pct={stat.cpu_percent} label={`${stat.cpu_percent.toFixed(1)}%`} />
                                  </td>
                                  <td>
                                    <ResourceBar
                                      pct={stat.mem_limit > 0 ? Math.min((stat.mem_usage / stat.mem_limit) * 100, 100) : 0}
                                      label={formatBytes(stat.mem_usage)}
                                    />
                                  </td>
                                  <td className="cell-muted">
                                    ↓{formatBytes(stat.net_rx)} / ↑{formatBytes(stat.net_tx)}
                                  </td>
                                  <td className="cell-muted">
                                    R{formatBytes(stat.blk_read)} / W{formatBytes(stat.blk_write)}
                                  </td>
                                </>
                              ) : (
                                <td colSpan={4} className="cell-muted">
                                  {c.State === 'running' ? 'Loading…' : '—'}
                                </td>
                              )}
                            </tr>
                          )
                        })}
                      </tbody>
                    </Table>
                  )}
                </Section>
              </>
            )}

            {metricsTab === 'kubernetes' && (
              <>
                <StatGrid minWidth={140}>
                  <StatCard label="Clusters reachable" value={`${k8sReachable}/${k8sNodeSummaries.length}`} />
                  <StatCard label="Pods" value={k8sTotals.pods} />
                  <StatCard label="Running" value={k8sTotals.running} />
                  <StatCard label="Deployments" value={k8sTotals.deployments} />
                  <StatCard label="K8s nodes" value={k8sTotals.nodes} />
                </StatGrid>
                <Section>
                  {k8sLoading ? (
                    <div className={styles.loadingRow}>Loading…</div>
                  ) : k8sNodeSummaries.length === 0 ? (
                    <EmptyState icon={Layers} title="No Kubernetes clusters found" />
                  ) : (
                    <Table>
                      <thead>
                        <tr>
                          <th>Cluster</th>
                          <th>Pods</th>
                          <th>Running</th>
                          <th>Pending</th>
                          <th>Failed</th>
                          <th>Deployments</th>
                          <th>Namespaces</th>
                          <th>K8s nodes</th>
                        </tr>
                      </thead>
                      <tbody>
                        {k8sNodeSummaries.map((n) => (
                          <tr key={n.nodeId ?? 'local'}>
                            <td className="cell-name">{n.nodeName}</td>
                            {n.summary ? (
                              <>
                                <td className="cell-muted">{n.summary.pods}</td>
                                <td className="cell-muted">{n.summary.running}</td>
                                <td className="cell-muted">{n.summary.pending}</td>
                                <td className="cell-muted">{n.summary.failed}</td>
                                <td className="cell-muted">{n.summary.deployments}</td>
                                <td className="cell-muted">{n.summary.namespaces}</td>
                                <td className="cell-muted">{n.summary.nodes}</td>
                              </>
                            ) : (
                              <td colSpan={7} className="cell-muted">
                                Not reachable
                              </td>
                            )}
                          </tr>
                        ))}
                      </tbody>
                    </Table>
                  )}
                </Section>
              </>
            )}
          </>
        )}

        {tab === 'traces' && (
          <>
            <Section
              action={
                <Button variant="ghost" onClick={refreshTraces} disabled={tracesLoading}>
                  Refresh
                </Button>
              }
            >
              {tracesLoading ? (
                <div className={styles.loadingRow}>Loading…</div>
              ) : traces.length === 0 ? (
                <EmptyState
                  icon={Route}
                  title="No spans match"
                  description="HTTP requests are automatically traced. Use the OTLP endpoint or collect from a container below to see service spans."
                />
              ) : (
                <Table>
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Service</th>
                      <th>Source</th>
                      <th>Kind</th>
                      <th>Status</th>
                      <th>Duration</th>
                      <th>Time</th>
                    </tr>
                  </thead>
                  <tbody>
                    {traces.map((t, i) => (
                      <tr key={t.trace_id ?? i}>
                        <td className="cell-name">
                          {t.name} {t.trace_id && <span className={styles.idChip}>{t.trace_id.slice(0, 8)}</span>}
                        </td>
                        <td className="cell-muted">{t.service ?? '—'}</td>
                        <td className={styles.sourceTag}>{t.source ?? 'internal'}</td>
                        <td className="cell-muted">{t.kind ?? '—'}</td>
                        <td>
                          <Badge variant={STATUS_VARIANT[t.status]}>{t.status}</Badge>
                        </td>
                        <td className="cell-muted">{t.duration_ms}ms</td>
                        <td className="cell-muted">{fmtTime(t.time)}</td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
              )}
            </Section>

            <Section title="Collect from container">
              <div className={styles.collectRow}>
                <Select
                  label="Container"
                  id="collectContainer"
                  value={collectContainer}
                  onChange={(e) => setCollectContainer(e.target.value)}
                  options={containerOptions}
                />
                <Button variant="ghost" onClick={collectFromContainer} disabled={!collectContainer || collecting}>
                  <Download size={14} />
                  {collecting ? 'Collecting…' : 'Collect'}
                </Button>
              </div>
              <p className={styles.hint}>Scans the last 200 log lines for structured JSON spans (OTEL, structured loggers).</p>
            </Section>
          </>
        )}

        {tab === 'otel' && (
          <>
            <Section
              action={
                <Button variant="ghost" onClick={refreshOtel} disabled={otelLoading}>
                  Refresh
                </Button>
              }
            >
              {otelLoading ? (
                <div className={styles.loadingRow}>Loading…</div>
              ) : otel.length === 0 ? (
                <EmptyState
                  icon={Gauge}
                  title="No metrics yet"
                  description="Push metrics via POST /api/otlp/v1/metrics or scrape a container's Prometheus endpoint below."
                />
              ) : (
                <Table>
                  <thead>
                    <tr>
                      <th>Metric</th>
                      <th>Service</th>
                      <th>Source</th>
                      <th>Type</th>
                      <th>Value</th>
                      <th>Labels</th>
                      <th>Time</th>
                    </tr>
                  </thead>
                  <tbody>
                    {otel.map((m, i) => (
                      <tr key={i}>
                        <td className="cell-name">{m.name}</td>
                        <td className="cell-muted">{m.service ?? '—'}</td>
                        <td className={styles.sourceTag}>{m.source ?? '—'}</td>
                        <td className="cell-muted">{m.type}</td>
                        <td className="cell-muted">{m.value.toLocaleString()}</td>
                        <td className="cell-muted">{fmtLabels(m.labels)}</td>
                        <td className="cell-muted">{fmtTime(m.time)}</td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
              )}
            </Section>

            <Section title="Scrape container metrics">
              <div className={styles.collectRow}>
                <Select
                  label="Container"
                  id="scrapeContainer"
                  value={scrapeContainer}
                  onChange={(e) => setScrapeContainer(e.target.value)}
                  options={containerOptions}
                />
                <TextField label="Port" id="scrapePort" value={scrapePort} onChange={(e) => setScrapePort(e.target.value)} />
                <Button variant="ghost" onClick={runScrape} disabled={!scrapeContainer || scraping}>
                  <Zap size={14} />
                  {scraping ? 'Scraping…' : 'Scrape'}
                </Button>
              </div>
              <p className={styles.hint}>
                Hits <code>http://&lt;container-ip&gt;:{scrapePort}/metrics</code> and ingests Prometheus-format data.
              </p>
            </Section>

            <Section title="OTLP endpoint">
              <dl className={styles.infoList}>
                <div className={styles.infoRow}>
                  <dt>Traces</dt>
                  <dd className={styles.mono}>POST /api/otlp/v1/traces</dd>
                </div>
                <div className={styles.infoRow}>
                  <dt>Logs</dt>
                  <dd className={styles.mono}>POST /api/otlp/v1/logs</dd>
                </div>
                <div className={styles.infoRow}>
                  <dt>Metrics</dt>
                  <dd className={styles.mono}>POST /api/otlp/v1/metrics</dd>
                </div>
                <div className={styles.infoRow}>
                  <dt>Auth</dt>
                  <dd>Bearer token (same as API)</dd>
                </div>
                <div className={styles.infoRow}>
                  <dt>Format</dt>
                  <dd>OTLP JSON (application/json)</dd>
                </div>
              </dl>
            </Section>
          </>
        )}
      </div>
    </AppShell>
  )
}

export default Observability
