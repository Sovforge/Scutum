import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowLeft, TriangleAlert } from 'lucide-react'
import TopologyMap from '../../components/TopologyMap/TopologyMap'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import { CONTAINERS, KUBERNETES, LOG_ENTRIES, NODES } from '../../mockData'
import styles from './TV.module.css'

const SLIDE_INTERVAL_MS = 25_000
const CLOCK_FORMAT_KEY = 'tv-clock-format'

const SEVERITY_COLOR: Record<string, string> = {
  info: 'var(--blueprint-bright)',
  warning: 'var(--rust-bright)',
  danger: 'var(--danger)',
}

function TV() {
  const [now, setNow] = useState(() => new Date())
  const [slide, setSlide] = useState(0)
  const [use24h, setUse24h] = useState(() => localStorage.getItem(CLOCK_FORMAT_KEY) !== '12h')

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

  const runningContainers = CONTAINERS.filter((c) => c.status === 'running').length
  const healthyNodes = NODES.filter((n) => n.status === 'healthy').length

  // Fatal = something that needs a human right now, not just "degraded".
  const offlineNodes = NODES.filter((n) => n.status === 'offline')
  const criticalEntries = LOG_ENTRIES.filter((e) => e.severity === 'danger')
  const alertMessages = [
    ...(offlineNodes.length
      ? [`${offlineNodes.length} node${offlineNodes.length !== 1 ? 's' : ''} offline (${offlineNodes.map((n) => n.name).join(', ')})`]
      : []),
    ...criticalEntries.map((e) => e.message),
  ]
  const hasFatal = alertMessages.length > 0

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
                <StatCard label="Nodes" value={NODES.length} sub="enrolled in mesh" size="large" />
                <StatCard label="Containers" value={`${runningContainers}/${CONTAINERS.length}`} sub="running" size="large" />
                <StatCard
                  label="Mesh health"
                  value={`${healthyNodes}/${NODES.length}`}
                  sub={`${NODES.length - healthyNodes} degraded`}
                  size="large"
                  color={healthyNodes === NODES.length ? 'var(--blueprint)' : 'var(--rust)'}
                />
                <StatCard label="Kubernetes pods" value={KUBERNETES.pods} sub={`across ${KUBERNETES.deployments} deployments`} size="large" />
              </StatGrid>
            </div>
            <div className={styles.topologyColumn}>
              <TopologyMap nodes={NODES} maxWidth={1600} tvMode />
            </div>
          </div>
        </section>

        <section className={`${styles.slide} ${slide === 1 ? styles.slideActive : ''}`}>
          <h2 className={styles.slideTitle}>Recent Activity</h2>
          <div className={styles.logList}>
            {LOG_ENTRIES.map((entry, i) => (
              <div className={styles.logRow} key={i}>
                <span className={styles.logDot} style={{ background: SEVERITY_COLOR[entry.severity] }} />
                <span className={styles.logTime}>{entry.time}</span>
                <span className={styles.logEvent}>{entry.event}</span>
                <span className={styles.logMessage}>{entry.message}</span>
                <span className={styles.logActor}>{entry.actor}</span>
              </div>
            ))}
          </div>
        </section>
      </div>
    </div>
  )
}

export default TV
