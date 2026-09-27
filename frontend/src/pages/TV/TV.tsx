import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowLeft, TriangleAlert } from 'lucide-react'
import TopologyMap from '../../components/TopologyMap/TopologyMap'
import type { MeshNode } from '../../components/TopologyMap/TopologyMap'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import { positionFromMeshIp } from '../../lib/nodePosition'
import {
  getK8sSummary,
  getMeshSummary,
  listAuditLogs,
  listContainers,
  listNodes,
  type AuditEntry,
  type DockerContainer,
  type K8sSummary,
  type MeshSummary,
  type NodeRecord,
} from '../../lib/api'
import styles from './TV.module.css'

const SLIDE_INTERVAL_MS = 25_000
const REFRESH_INTERVAL_MS = 30_000
const CLOCK_FORMAT_KEY = 'tv-clock-format'

const SEVERITY_COLOR: Record<'info' | 'danger', string> = {
  info: 'var(--blueprint-bright)',
  danger: 'var(--danger)',
}

function fmtTime(iso: string): string {
  try {
    return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } catch {
    return iso
  }
}

function TV() {
  const [now, setNow] = useState(() => new Date())
  const [slide, setSlide] = useState(0)
  const [use24h, setUse24h] = useState(() => localStorage.getItem(CLOCK_FORMAT_KEY) !== '12h')

  const [nodes, setNodes] = useState<NodeRecord[]>([])
  const [containers, setContainers] = useState<DockerContainer[]>([])
  const [meshSummary, setMeshSummary] = useState<MeshSummary>({ total: 0, healthy: 0, degraded: 0 })
  const [k8s, setK8s] = useState<K8sSummary | null>(null)
  const [logEntries, setLogEntries] = useState<AuditEntry[]>([])

  function loadData() {
    listNodes().then(setNodes).catch(() => {})
    listContainers().then(setContainers).catch(() => {})
    getMeshSummary().then(setMeshSummary).catch(() => {})
    // Local explicitly — this is a hub-wide overview, not scoped to
    // whatever node the topbar switcher happens to be pointed at.
    getK8sSummary(null).then(setK8s).catch(() => {})
    listAuditLogs().then((entries) => setLogEntries(entries.slice(0, 12))).catch(() => {})
  }

  useEffect(() => {
    loadData()
    const id = setInterval(loadData, REFRESH_INTERVAL_MS)
    return () => clearInterval(id)
  }, [])

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 15_000)
    return () => clearInterval(id)
  }, [])

  function toggleClockFormat() {
    setUse24h((prev) => {
      const next = !prev
      localStorage.setItem(CLOCK_FORMAT_KEY, next ? '24h' : '12h')
      return next
    })
  }

  useEffect(() => {
    const id = setInterval(() => setSlide((s) => (s + 1) % 2), SLIDE_INTERVAL_MS)
    return () => clearInterval(id)
  }, [])

  const runningContainers = containers.filter((c) => c.State === 'running').length
  const meshHealthy = meshSummary.total === 0 || meshSummary.healthy === meshSummary.total

  // The real API only reports aggregate mesh health (MeshSummary), not a
  // per-node status, so — same as the Dashboard page — every node on the
  // globe below shares one healthy/degraded color instead of an individual
  // one. The attention banner is built from what's actually knowable: the
  // degraded count, plus any failed actions in the audit log.
  const failedEntries = logEntries.filter((e) => e.outcome === 'failure')
  const alertMessages = [
    ...(meshSummary.degraded > 0
      ? [`${meshSummary.degraded} node${meshSummary.degraded !== 1 ? 's' : ''} degraded`]
      : []),
    ...failedEntries.slice(0, 3).map((e) => `${e.action} failed`),
  ]
  const hasFatal = alertMessages.length > 0

  const graphNodes: MeshNode[] = nodes.map((n) => {
    const { lat, lng } = positionFromMeshIp(n.address)
    return {
      id: n.id,
      name: n.name,
      role: (n.type as MeshNode['role']) ?? 'remote',
      status: meshHealthy ? 'healthy' : 'degraded',
      lat,
      lng,
    }
  })

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div className={styles.headerLeft}>
          <Link to="/dashboard" className={styles.backBtn} aria-label="Back to dashboard">
            <ArrowLeft size={16} />
          </Link>
          <span className="brand-mark">
            <img src="/logo.svg" alt="" className="brand-mark__logo" />
            <span className="brand-mark__word">SCUTUM</span>
          </span>
        </div>
        {/* Locale-based hour12 detection isn't reliable (browser locale != OS
            clock-format preference), so this is an explicit, persisted toggle
            instead — click to switch between 24h and 12h. */}
        <button type="button" className={styles.clock} onClick={toggleClockFormat} title="Click to toggle 12h/24h">
          {now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: !use24h })}
        </button>
      </header>

      {hasFatal && (
        <div className={styles.alertBanner} role="alert">
          <TriangleAlert size={20} className={styles.alertIcon} />
          <span className={styles.alertTitle}>Attention required</span>
          <span className={styles.alertMessage}>{alertMessages.join(' · ')}</span>
        </div>
      )}

      <div className={styles.stage}>
        <section className={`${styles.slide} ${slide === 0 ? styles.slideActive : ''}`}>
          <h2 className={styles.slideTitle}>Mesh Status</h2>
          {/* Side-by-side, not stacked — stacking made the globe compete
              with the stat cards for the same limited vertical space, so it
              never got bigger than a leftover sliver. This way it gets the
              slide's full height. */}
          <div className={styles.meshLayout}>
            <div className={styles.statsColumn}>
              <StatGrid minWidth={200}>
                <StatCard label="Nodes" value={nodes.length} sub="enrolled in mesh" size="large" />
                <StatCard label="Containers" value={`${runningContainers}/${containers.length}`} sub="running" size="large" />
                <StatCard
                  label="Mesh health"
                  value={meshSummary.total === 0 ? '—' : `${meshSummary.healthy}/${meshSummary.total}`}
                  sub={meshSummary.total === 0 ? 'no peers' : `${meshSummary.degraded} degraded`}
                  size="large"
                  color={meshHealthy ? 'var(--blueprint)' : 'var(--rust)'}
                />
                <StatCard
                  label="Kubernetes pods"
                  value={k8s?.pods ?? '—'}
                  sub={k8s ? `across ${k8s.deployments} deployments` : 'no cluster detected'}
                  size="large"
                />
              </StatGrid>
            </div>
            <div className={styles.topologyColumn}>
              <TopologyMap nodes={graphNodes} maxWidth={1600} tvMode />
            </div>
          </div>
        </section>

        <section className={`${styles.slide} ${slide === 1 ? styles.slideActive : ''}`}>
          <h2 className={styles.slideTitle}>Recent Activity</h2>
          <div className={styles.logList}>
            {logEntries.map((entry, i) => (
              <div className={styles.logRow} key={i}>
                <span
                  className={styles.logDot}
                  style={{ background: SEVERITY_COLOR[entry.outcome === 'failure' ? 'danger' : 'info'] }}
                />
                <span className={styles.logTime}>{fmtTime(entry.time)}</span>
                <span className={styles.logEvent}>{entry.action}</span>
                <span className={styles.logMessage}>{entry.method} {entry.path}</span>
                <span className={styles.logActor}>{entry.actor ?? 'system'}</span>
              </div>
            ))}
          </div>
        </section>
      </div>
    </div>
  )
}

export default TV
