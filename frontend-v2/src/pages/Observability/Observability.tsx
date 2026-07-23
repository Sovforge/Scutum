import { useState } from 'react'
import { Activity, Box, Download, Gauge, Layers, Route, ScrollText, SearchX, Zap } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import ResourceBar from '../../components/ui/ResourceBar/ResourceBar'
import Select from '../../components/ui/Select/Select'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import Callout from '../../components/ui/Callout/Callout'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import Tabs, { type TabItem } from '../../components/ui/Tabs/Tabs'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import { formatBytes } from '../../lib/format'
import styles from './Observability.module.css'

type Tab = 'logs' | 'metrics' | 'traces' | 'otel'
type LogLevel = 'debug' | 'info' | 'warn' | 'error'
type LogSource = 'app' | 'k8s' | 'docker'
type MetricsTab = 'docker' | 'kubernetes'
type TraceStatus = 'ok' | 'error' | 'unset'

// Mock data shaped exactly like the real endpoints and types in
// frontend/app/composables/useApi.ts:
//   listLogs()               -> LogEntry[]     GET /observability/logs
//   listTraces()              -> TraceEntry[]   GET /observability/traces
//   listMetrics()              -> MetricPoint[]  GET /observability/metrics
//   getContainerStats() / getK8sSummary() back the Docker/Kubernetes metrics
//   sub-tabs (per-node resource stats, not OTel points).
// "Metrics" (container/cluster resource stats) and "OTel Metrics" (raw
// OpenTelemetry data points) are two distinct real tabs — not the same
// thing. Application logs (this page) are separate from the CRA-compliant
// audit trail at GET /audit/logs (its own page). Not wired to a live
// backend yet.

const LOGS: { time: string; level: LogLevel; message: string }[] = [
  { time: '14:32:11', level: 'info', message: 'wireguard: handshake completed with peer edge-london' },
  { time: '14:31:58', level: 'debug', message: 'docker-agent: healthcheck passed for scutum-agent (3ms)' },
  { time: '14:30:22', level: 'warn', message: 'healer: peer edge-nyc missed expected handshake window' },
  { time: '14:29:47', level: 'info', message: 'kubernetes-agent: reconciled deployment observability (3/3 pods ready)' },
  { time: '14:28:03', level: 'error', message: 'api-server: rejected request — invalid bearer token from 203.0.113.44' },
  { time: '14:26:40', level: 'info', message: 'gitops: synced manifest repo main@a3f9c12' },
  { time: '14:25:12', level: 'debug', message: 'wireguard: sent keepalive to build-runner' },
  { time: '14:20:05', level: 'info', message: 'storage: uploaded backup snapshot to S3 backend (412M)' },
]

const DOCKER_STATS = [
  {
    node: 'hub-fra1',
    name: 'scutum-agent',
    image: 'ghcr.io/sovforge/scutum:latest',
    state: 'running' as const,
    cpuPct: 4.1,
    memUsage: 134_000_000,
    memLimit: 1_073_741_824,
    netRx: 12_400_000,
    netTx: 8_900_000,
    blkRead: 2_100_000,
    blkWrite: 512_000,
  },
  {
    node: 'hub-fra1',
    name: 'postgres',
    image: 'postgres:16-alpine',
    state: 'running' as const,
    cpuPct: 18.6,
    memUsage: 512_000_000,
    memLimit: 2_147_483_648,
    netRx: 4_100_000,
    netTx: 6_700_000,
    blkRead: 18_900_000,
    blkWrite: 9_400_000,
  },
  {
    node: 'build-runner',
    name: 'grafana',
    image: 'grafana/grafana:11',
    state: 'running' as const,
    cpuPct: 62.0,
    memUsage: 933_888_000,
    memLimit: 1_073_741_824,
    netRx: 31_200_000,
    netTx: 52_800_000,
    blkRead: 4_300_000,
    blkWrite: 1_100_000,
  },
  {
    node: 'edge-london',
    name: 'minio',
    image: 'minio/minio:latest',
    state: 'running' as const,
    cpuPct: 8.2,
    memUsage: 356_000_000,
    memLimit: 1_073_741_824,
    netRx: 9_800_000,
    netTx: 14_200_000,
    blkRead: 61_400_000,
    blkWrite: 38_200_000,
  },
  {
    node: 'build-runner',
    name: 'backup-job',
    image: 'restic/restic:latest',
    state: 'exited' as const,
    cpuPct: 0,
    memUsage: 0,
    memLimit: 0,
    netRx: 0,
    netTx: 0,
    blkRead: 0,
    blkWrite: 0,
  },
]

const K8S_CLUSTERS: {
  node: string
  summary: { pods: number; running: number; pending: number; failed: number; deployments: number; namespaces: number; nodes: number } | null
}[] = [
  { node: 'build-runner', summary: { pods: 23, running: 21, pending: 1, failed: 1, deployments: 6, namespaces: 4, nodes: 3 } },
  { node: 'edge-nyc', summary: null },
]

const INITIAL_TRACES: {
  traceId: string
  spanId: string
  name: string
  service?: string
  source: string
  kind?: string
  durationMs: number
  status: TraceStatus
  time: string
}[] = [
  { traceId: 'f3a9c1e2', spanId: '00a1', name: 'docker.deploy-compose', service: 'hub-fra1', source: 'internal', kind: 'server', durationMs: 842, status: 'ok', time: '14:32:07' },
  { traceId: '7bd402aa', spanId: '00b2', name: 'kubernetes.apply', service: 'build-runner', source: 'internal', kind: 'server', durationMs: 1204, status: 'ok', time: '14:15:40' },
  { traceId: 'e19f88b0', spanId: '00c3', name: 'network.peer-handshake', service: 'edge-nyc', source: 'internal', kind: 'internal', durationMs: 3110, status: 'error', time: '13:58:02' },
  { traceId: '4c6e0d7f', spanId: '00d4', name: 'auth.login', service: 'hub-fra1', source: 'internal', kind: 'server', durationMs: 61, status: 'ok', time: '13:40:19' },
  { traceId: 'a08d31c4', spanId: '00e5', name: 'HTTP GET /metrics', service: 'grafana', source: 'docker', kind: 'client', durationMs: 118, status: 'unset', time: '13:22:55' },
  { traceId: '9911ffab', spanId: '00f6', name: 'otel.export', service: 'external-collector', source: 'otlp', kind: 'client', durationMs: 44, status: 'ok', time: '13:10:03' },
]

const INITIAL_OTEL: {
  time: string
  name: string
  service?: string
  source: string
  type: string
  value: number
  labels?: Record<string, string>
}[] = [
  { time: '14:32:00', name: 'system.cpu.utilization', service: 'hub-fra1', source: 'internal', type: 'gauge', value: 22.4 },
  { time: '14:32:00', name: 'system.memory.utilization', service: 'hub-fra1', source: 'internal', type: 'gauge', value: 41.1 },
  { time: '14:32:00', name: 'system.cpu.utilization', service: 'edge-nyc', source: 'internal', type: 'gauge', value: 71.3 },
  { time: '14:31:45', name: 'wireguard.peer.rx_bytes', service: 'edge-london', source: 'internal', type: 'counter', value: 883_200_000, labels: { peer: 'edge-london' } },
  { time: '14:20:10', name: 'http_requests_total', service: 'grafana', source: 'docker', type: 'counter', value: 48_213 },
  { time: '14:18:55', name: 'otel.exporter.queue_size', service: 'external-collector', source: 'otlp', type: 'gauge', value: 12 },
]

const LEVEL_VARIANT: Record<LogLevel, 'neutral' | 'success' | 'warning' | 'danger'> = {
  debug: 'neutral',
  info: 'success',
  warn: 'warning',
  error: 'danger',
}

const STATUS_VARIANT: Record<TraceStatus, 'success' | 'danger' | 'neutral'> = {
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

function Observability() {
  const toast = useToast()
  const [tab, setTab] = useState<Tab>('logs')

  // ── Logs ─────────────────────────────────────────────────────────────────
  const [logSource, setLogSource] = useState<LogSource>('app')
  const [logLevel, setLogLevel] = useState<LogLevel | ''>('')
  const [logQuery, setLogQuery] = useState('')
  const filteredLogs = LOGS.filter((l) => {
    if (logLevel && l.level !== logLevel) return false
    if (logQuery && !l.message.toLowerCase().includes(logQuery.toLowerCase())) return false
    return true
  })
  const errorCount = LOGS.filter((l) => l.level === 'error').length

  // ── Metrics (Docker / Kubernetes resource stats) ────────────────────────
  const [metricsTab, setMetricsTab] = useState<MetricsTab>('docker')
  const dockerRunning = DOCKER_STATS.filter((c) => c.state === 'running').length
  const k8sReachable = K8S_CLUSTERS.filter((c) => c.summary).length
  const k8sTotals = K8S_CLUSTERS.reduce(
    (acc, c) => {
      if (!c.summary) return acc
      acc.pods += c.summary.pods
      acc.running += c.summary.running
      acc.deployments += c.summary.deployments
      acc.nodes += c.summary.nodes
      return acc
    },
    { pods: 0, running: 0, deployments: 0, nodes: 0 },
  )

  // ── Traces ───────────────────────────────────────────────────────────────
  const [traces, setTraces] = useState(INITIAL_TRACES)
  const [collectContainer, setCollectContainer] = useState('')

  function collectFromContainer() {
    if (!collectContainer) return
    const c = DOCKER_STATS.find((d) => d.name === collectContainer)
    if (!c) return
    setTraces((t) => [
      { traceId: Math.random().toString(16).slice(2, 10), spanId: '0000', name: 'container.collected-span', service: c.name, source: 'docker', kind: 'internal', durationMs: 0, status: 'unset', time: 'just now' },
      ...t,
    ])
    toast(`Collected spans from ${c.name}`)
  }

  // ── OTel Metrics ─────────────────────────────────────────────────────────
  const [otel, setOtel] = useState(INITIAL_OTEL)
  const [scrapeContainer, setScrapeContainer] = useState('')
  const [scrapePort, setScrapePort] = useState('9090')

  function runScrape() {
    if (!scrapeContainer) return
    setOtel((m) => [
      { time: 'just now', name: 'scrape.up', service: scrapeContainer, source: 'docker', type: 'gauge', value: 1 },
      ...m,
    ])
    toast(`Scraped metrics from ${scrapeContainer}:${scrapePort}`)
  }

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
              <select className={styles.toolbarSelect} value={logSource} onChange={(e) => setLogSource(e.target.value as LogSource)}>
                <option value="app">Application</option>
                <option value="k8s">Kubernetes Events</option>
                <option value="docker">Docker Container</option>
              </select>
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
            </div>

            {logSource !== 'app' ? (
              <div className={styles.calloutWrap}>
                <Callout variant="info">
                  {logSource === 'k8s' ? 'Kubernetes event' : 'Docker container'} log tailing streams live from the
                  target node and isn't available in this preview build.
                </Callout>
              </div>
            ) : (
              <div className={styles.logList}>
                {filteredLogs.map((log, i) => (
                  <div className={styles.logRow} key={i}>
                    <span className={styles.logTime}>{log.time}</span>
                    <Badge variant={LEVEL_VARIANT[log.level]}>{log.level}</Badge>
                    <span className={styles.logMessage}>{log.message}</span>
                  </div>
                ))}
                {filteredLogs.length === 0 && (
                  <EmptyState
                    icon={SearchX}
                    title="No matching log entries"
                    description="Nothing matches the current filter and search query."
                    action={
                      <Button
                        variant="ghost"
                        onClick={() => {
                          setLogQuery('')
                          setLogLevel('')
                        }}
                      >
                        Clear filters
                      </Button>
                    }
                  />
                )}
              </div>
            )}
          </Section>
        )}

        {tab === 'metrics' && (
          <>
            <Tabs tabs={METRICS_TABS} active={metricsTab} onChange={(id) => setMetricsTab(id as MetricsTab)} variant="secondary" />

            {metricsTab === 'docker' && (
              <>
                <StatGrid minWidth={140}>
                  <StatCard label="Nodes" value={new Set(DOCKER_STATS.map((c) => c.node)).size} />
                  <StatCard label="Containers" value={DOCKER_STATS.length} />
                  <StatCard label="Running" value={dockerRunning} />
                  <StatCard label="Stopped" value={DOCKER_STATS.length - dockerRunning} />
                </StatGrid>
                <Section>
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
                      {DOCKER_STATS.map((c) => (
                        <tr key={`${c.node}-${c.name}`}>
                          <td className="cell-muted">{c.node}</td>
                          <td className="cell-name">{c.name}</td>
                          <td className="cell-muted">{c.image}</td>
                          <td>
                            <Badge variant={c.state === 'running' ? 'success' : 'neutral'}>{c.state}</Badge>
                          </td>
                          {c.state === 'running' ? (
                            <>
                              <td>
                                <ResourceBar pct={c.cpuPct} label={`${c.cpuPct}%`} />
                              </td>
                              <td>
                                <ResourceBar
                                  pct={Math.min((c.memUsage / c.memLimit) * 100, 100)}
                                  label={formatBytes(c.memUsage)}
                                />
                              </td>
                              <td className="cell-muted">
                                ↓{formatBytes(c.netRx)} / ↑{formatBytes(c.netTx)}
                              </td>
                              <td className="cell-muted">
                                R{formatBytes(c.blkRead)} / W{formatBytes(c.blkWrite)}
                              </td>
                            </>
                          ) : (
                            <td colSpan={4} className="cell-muted">
                              —
                            </td>
                          )}
                        </tr>
                      ))}
                    </tbody>
                  </Table>
                </Section>
              </>
            )}

            {metricsTab === 'kubernetes' && (
              <>
                <StatGrid minWidth={140}>
                  <StatCard label="Clusters reachable" value={`${k8sReachable}/${K8S_CLUSTERS.length}`} />
                  <StatCard label="Pods" value={k8sTotals.pods} />
                  <StatCard label="Running" value={k8sTotals.running} />
                  <StatCard label="Deployments" value={k8sTotals.deployments} />
                  <StatCard label="K8s nodes" value={k8sTotals.nodes} />
                </StatGrid>
                <Section>
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
                      {K8S_CLUSTERS.map((c) => (
                        <tr key={c.node}>
                          <td className="cell-name">{c.node}</td>
                          {c.summary ? (
                            <>
                              <td className="cell-muted">{c.summary.pods}</td>
                              <td className="cell-muted">{c.summary.running}</td>
                              <td className="cell-muted">{c.summary.pending}</td>
                              <td className="cell-muted">{c.summary.failed}</td>
                              <td className="cell-muted">{c.summary.deployments}</td>
                              <td className="cell-muted">{c.summary.namespaces}</td>
                              <td className="cell-muted">{c.summary.nodes}</td>
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
                </Section>
              </>
            )}
          </>
        )}

        {tab === 'traces' && (
          <>
            <Section>
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
                  {traces.map((t) => (
                    <tr key={t.traceId}>
                      <td className="cell-name">
                        {t.name} <span className={styles.idChip}>{t.traceId.slice(0, 8)}</span>
                      </td>
                      <td className="cell-muted">{t.service ?? '—'}</td>
                      <td className={styles.sourceTag}>{t.source}</td>
                      <td className="cell-muted">{t.kind ?? '—'}</td>
                      <td>
                        <Badge variant={STATUS_VARIANT[t.status]}>{t.status}</Badge>
                      </td>
                      <td className="cell-muted">{t.durationMs}ms</td>
                      <td className="cell-muted">{t.time}</td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </Section>

            <Section title="Collect from container">
              <div className={styles.collectRow}>
                <Select
                  label="Container"
                  id="collectContainer"
                  value={collectContainer}
                  onChange={(e) => setCollectContainer(e.target.value)}
                  options={[
                    { value: '', label: 'Select container…' },
                    ...DOCKER_STATS.map((c) => ({ value: c.name, label: `${c.node} · ${c.name}` })),
                  ]}
                />
                <Button variant="ghost" onClick={collectFromContainer} disabled={!collectContainer}>
                  <Download size={14} />
                  Collect
                </Button>
              </div>
              <p className={styles.hint}>Scans the last 200 log lines for structured JSON spans (OTEL, structured loggers).</p>
            </Section>
          </>
        )}

        {tab === 'otel' && (
          <>
            <Section>
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
                      <td className={styles.sourceTag}>{m.source}</td>
                      <td className="cell-muted">{m.type}</td>
                      <td className="cell-muted">{m.value.toLocaleString()}</td>
                      <td className="cell-muted">{fmtLabels(m.labels)}</td>
                      <td className="cell-muted">{m.time}</td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </Section>

            <Section title="Scrape container metrics">
              <div className={styles.collectRow}>
                <Select
                  label="Container"
                  id="scrapeContainer"
                  value={scrapeContainer}
                  onChange={(e) => setScrapeContainer(e.target.value)}
                  options={[
                    { value: '', label: 'Select container…' },
                    ...DOCKER_STATS.map((c) => ({ value: c.name, label: `${c.node} · ${c.name}` })),
                  ]}
                />
                <TextField label="Port" id="scrapePort" value={scrapePort} onChange={(e) => setScrapePort(e.target.value)} />
                <Button variant="ghost" onClick={runScrape} disabled={!scrapeContainer}>
                  <Zap size={14} />
                  Scrape
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
