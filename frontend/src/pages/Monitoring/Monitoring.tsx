import { useEffect, useState } from 'react'
import { Cpu, RefreshCw, WifiOff } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import ResourceBar from '../../components/ui/ResourceBar/ResourceBar'
import Section from '../../components/ui/Section/Section'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { getSystemStats, getSystemStatsHistory, listNodes, type NodeStatRecord, type SystemStats } from '../../lib/api'
import styles from './Monitoring.module.css'

// Real endpoints: GET /system/stats (live snapshot) and GET
// /system/stats/history?limit= (rolling 24h history, 1/min), both proxyable
// via X-Target-Node. Same "Local + filter out the hub row" node targeting
// as Dashboard/Kubernetes — passing the hub's own id as X-Target-Node would
// make it proxy an HTTPS request to itself instead of answering locally.
const REFRESH_INTERVAL_MS = 30_000

function fmtBytes(bytes: number): string {
  if (bytes >= 1_073_741_824) return (bytes / 1_073_741_824).toFixed(1) + ' GB'
  if (bytes >= 1_048_576) return (bytes / 1_048_576).toFixed(1) + ' MB'
  return (bytes / 1024).toFixed(0) + ' KB'
}

function relativeTime(iso: string): string {
  const secs = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (secs < 5) return 'just now'
  if (secs < 90) return `${secs}s ago`
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`
  return `${Math.floor(secs / 3600)}h ago`
}

// Renders oldest-to-newest left-to-right; history comes back newest-first.
function sparklinePoints(values: number[]): string {
  if (values.length < 2) return ''
  const w = 200
  const h = 40
  const pad = 2
  const max = Math.max(...values, 1)
  return values
    .slice()
    .reverse()
    .map((v, i) => {
      const x = pad + (i / (values.length - 1)) * (w - pad * 2)
      const y = h - pad - (v / max) * (h - pad * 2)
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}

type NodeEntry = {
  nodeId: string | null
  nodeName: string
  nodeType: string
  stats: SystemStats | null
  history: NodeStatRecord[]
  error: boolean
}

function Monitoring() {
  const [entries, setEntries] = useState<NodeEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null)
  const [, forceTick] = useState(0)

  async function refresh(isStale: () => boolean = () => false) {
    setRefreshing(true)
    try {
      const nodes = await listNodes().catch(() => [])
      const targets = [
        { id: null as string | null, name: 'Local', type: 'hub' },
        ...nodes.filter((n) => n.type !== 'hub').map((n) => ({ id: n.id as string | null, name: n.name, type: n.type })),
      ]
      const results = await Promise.all(
        targets.map(async (t) => {
          try {
            const [stats, history] = await Promise.all([
              getSystemStats(t.id),
              getSystemStatsHistory(t.id, 60),
            ])
            return { nodeId: t.id, nodeName: t.name, nodeType: t.type, stats, history, error: false }
          } catch {
            return { nodeId: t.id, nodeName: t.name, nodeType: t.type, stats: null, history: [], error: true }
          }
        }),
      )
      if (isStale()) return
      setEntries(results)
      setLastRefresh(new Date())
    } finally {
      if (!isStale()) {
        setRefreshing(false)
        setLoading(false)
      }
    }
  }

  // StrictMode double-invokes effects in dev (mount → cleanup → mount) — a
  // `useRef` flag set true by the first cleanup would stay true forever and
  // wedge every later refresh, so each effect invocation gets its own local
  // `stale` closure instead (same pattern as Kubernetes.tsx's loadData).
  useEffect(() => {
    let stale = false
    refresh(() => stale)
    const id = setInterval(() => refresh(() => stale), REFRESH_INTERVAL_MS)
    // Re-render every 5s so the "Updated Xs ago" label keeps ticking between fetches.
    const tick = setInterval(() => forceTick((n) => n + 1), 5000)
    return () => {
      stale = true
      clearInterval(id)
      clearInterval(tick)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <AppShell title="Monitoring">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Fleet Health</p>
            <h2 className={styles.reportTitle}>Resource monitoring</h2>
          </div>
          <div className={styles.headerActions}>
            <span className="stamp">{lastRefresh ? `Updated ${relativeTime(lastRefresh.toISOString())}` : 'Loading…'}</span>
            <button type="button" className={styles.refreshBtn} onClick={() => refresh()} disabled={refreshing} aria-label="Refresh">
              <RefreshCw size={14} className={refreshing ? styles.spin : undefined} />
            </button>
          </div>
        </div>

        <Section>
          {loading ? (
            <div className={styles.loadingRow}>Loading stats…</div>
          ) : entries.length === 0 ? (
            <EmptyState icon={Cpu} title="No monitoring data yet" description="Stats are collected every 60 seconds." />
          ) : (
            <div className={styles.nodeGrid}>
              {entries.map((entry) => (
                <div className={styles.nodeCard} key={entry.nodeId ?? 'local'}>
                  <div className={styles.nodeCardHeader}>
                    <div className={styles.nodeCardTitle}>
                      <Badge variant={entry.error ? 'danger' : 'success'}>{entry.error ? 'Offline' : 'Healthy'}</Badge>
                      <span className={styles.nodeName}>{entry.nodeName}</span>
                      <Badge variant="neutral">{entry.nodeType}</Badge>
                    </div>
                    {entry.stats && <span className={styles.nodeTs}>{relativeTime(entry.stats.recorded_at)}</span>}
                  </div>

                  {entry.error ? (
                    <div className={styles.nodeError}>
                      <WifiOff size={13} />
                      Unreachable
                    </div>
                  ) : entry.stats ? (
                    <>
                      <div className={styles.gaugeRow}>
                        <span className={styles.gaugeLabel}>CPU</span>
                        <ResourceBar pct={entry.stats.cpu_percent} label={`${entry.stats.cpu_percent.toFixed(1)}%`} />
                      </div>
                      <div className={styles.gaugeRow}>
                        <span className={styles.gaugeLabel}>Memory</span>
                        <ResourceBar
                          pct={entry.stats.mem_percent}
                          label={`${fmtBytes(entry.stats.mem_used)} / ${fmtBytes(entry.stats.mem_total)}`}
                        />
                      </div>
                      <div className={styles.gaugeRow}>
                        <span className={styles.gaugeLabel}>Disk</span>
                        <ResourceBar
                          pct={entry.stats.disk_percent}
                          label={`${fmtBytes(entry.stats.disk_used)} / ${fmtBytes(entry.stats.disk_total)}`}
                        />
                      </div>

                      <div className={styles.loadRow}>
                        <span className={styles.loadLabel}>Load avg</span>
                        <span className={styles.loadVals}>
                          {entry.stats.load_1.toFixed(2)} &nbsp; {entry.stats.load_5.toFixed(2)} &nbsp;{' '}
                          {entry.stats.load_15.toFixed(2)}
                        </span>
                        <span className={styles.loadPeriods}>(1m 5m 15m)</span>
                      </div>

                      {entry.history.length > 1 && (
                        <div className={styles.sparklineWrap}>
                          <span className={styles.sparklineLabel}>CPU (last {entry.history.length}m)</span>
                          <svg className={styles.sparkline} viewBox="0 0 200 40" preserveAspectRatio="none">
                            <polyline
                              points={sparklinePoints(entry.history.map((h) => h.cpu_percent))}
                              fill="none"
                              stroke="var(--blueprint-bright)"
                              strokeWidth={1.5}
                              strokeLinejoin="round"
                              strokeLinecap="round"
                            />
                          </svg>
                        </div>
                      )}
                    </>
                  ) : null}
                </div>
              ))}
            </div>
          )}
        </Section>
      </div>
    </AppShell>
  )
}

export default Monitoring
