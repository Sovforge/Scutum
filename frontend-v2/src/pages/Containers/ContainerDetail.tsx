import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, Play, RefreshCw, RotateCw, Square, Trash2 } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import Checkbox from '../../components/ui/Checkbox/Checkbox'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import ResourceBar from '../../components/ui/ResourceBar/ResourceBar'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { formatBytes } from '../../lib/format'
import { useNodeSelection } from '../../lib/nodeSelection'
import {
  getContainerInspect,
  getContainerLogsJSON,
  getContainerStats,
  listContainers,
  listNodes,
  removeContainer,
  restartContainer,
  startContainer,
  stopContainer,
  type ContainerLogLine,
  type ContainerStats,
  type DockerContainer,
} from '../../lib/api'
import styles from './ContainerDetail.module.css'

function stateVariant(s: string): 'success' | 'warning' | 'neutral' {
  if (s === 'running') return 'success'
  if (s === 'paused') return 'warning'
  return 'neutral'
}

function ContainerDetail() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { selectedNodeId } = useNodeSelection()

  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)
  const [raw, setRaw] = useState<DockerContainer | null>(null)
  const [inspect, setInspect] = useState<any>(null)
  const [stats, setStats] = useState<ContainerStats | null>(null)
  const [statsLoading, setStatsLoading] = useState(false)
  const [logs, setLogs] = useState<ContainerLogLine[]>([])
  const [follow, setFollow] = useState(true)
  const [nodeName, setNodeName] = useState('Local')

  async function refreshStats() {
    setStatsLoading(true)
    try {
      setStats(await getContainerStats(id))
    } catch {
      // container may not be running
    } finally {
      setStatsLoading(false)
    }
  }

  async function load() {
    setLoading(true)
    try {
      const [list, insp, logList] = await Promise.allSettled([
        listContainers(),
        getContainerInspect(id),
        getContainerLogsJSON(id),
      ])
      if (list.status === 'fulfilled') setRaw(list.value.find((c) => c.Id === id) ?? null)
      if (insp.status === 'fulfilled') setInspect(insp.value)
      if (logList.status === 'fulfilled') setLogs(logList.value)
      if (list.status !== 'fulfilled' && insp.status !== 'fulfilled') setNotFound(true)
      else if (list.status === 'fulfilled' && !list.value.find((c) => c.Id === id) && insp.status !== 'fulfilled') setNotFound(true)
    } catch {
      setNotFound(true)
    } finally {
      setLoading(false)
    }
    refreshStats()
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  useEffect(() => {
    if (!selectedNodeId) {
      setNodeName('Local')
      return
    }
    listNodes()
      .then((nodes) => setNodeName(nodes.find((n) => n.id === selectedNodeId)?.name ?? 'Local'))
      .catch(() => {})
  }, [selectedNodeId])

  const c = useMemo(() => {
    const r = raw
    const i = inspect

    const env: string[] = i?.Config?.Env ?? []

    const mounts: { type: string; source: string; destination: string; mode: string }[] = (i?.Mounts ?? []).map((m: any) => ({
      type: m.Type ?? 'bind',
      source: m.Source ?? '',
      destination: m.Destination ?? '',
      mode: m.Mode ?? (m.RW ? 'rw' : 'ro'),
    }))

    const ports: { host: number; container: number; proto: string }[] = Object.entries(i?.NetworkSettings?.Ports ?? {}).flatMap(
      ([key, bindings]: [string, any]) => {
        const [portStr, proto] = key.split('/')
        return (bindings ?? []).map((b: any) => ({
          host: parseInt(b.HostPort || '0'),
          container: parseInt(String(portStr ?? '0')),
          proto: proto ?? 'tcp',
        }))
      },
    )

    const networks = Object.values(i?.NetworkSettings?.Networks ?? {}) as any[]
    const net = networks[0] ?? {}

    return {
      id: i?.Id ?? r?.Id ?? '',
      name: (i?.Name ?? r?.Names?.[0] ?? r?.Id?.slice(0, 12) ?? '').replace(/^\//, ''),
      image: i?.Config?.Image ?? r?.Image ?? '—',
      imageId: (i?.Image ?? '').replace(/^sha256:/, '').slice(0, 12) || '—',
      runtime: i?.GraphDriver?.Name ?? '—',
      created: i?.Created ? new Date(i.Created).toLocaleString() : '—',
      uptime: r?.Status ?? i?.State?.Status ?? '—',
      status: i?.State?.Status ?? r?.State ?? '—',
      restartPolicy: i?.HostConfig?.RestartPolicy?.Name || '—',
      restartCount: i?.RestartCount ?? 0,
      cpu: stats ? stats.cpu_percent.toFixed(2) + '%' : '—',
      cpuRaw: stats ? Math.min(stats.cpu_percent, 100) : 0,
      cpuLimit: 'no limit',
      memory: stats ? formatBytes(stats.mem_usage) : '—',
      memRaw: stats && stats.mem_limit > 0 ? Math.min((stats.mem_usage / stats.mem_limit) * 100, 100) : 0,
      memLimit: stats && stats.mem_limit > 0 ? formatBytes(stats.mem_limit) : 'no limit',
      netRx: stats ? formatBytes(stats.net_rx) : '—',
      netTx: stats ? formatBytes(stats.net_tx) : '—',
      blockRead: stats ? formatBytes(stats.blk_read) : '—',
      blockWrite: stats ? formatBytes(stats.blk_write) : '—',
      ip: net.IPAddress || i?.NetworkSettings?.IPAddress || '—',
      gateway: net.Gateway || i?.NetworkSettings?.Gateway || '—',
      mac: net.MacAddress || '—',
      networkMode: i?.HostConfig?.NetworkMode ?? '—',
      ports:
        ports.length > 0
          ? ports
          : (r?.Ports ?? [])
              .filter((p) => p.PublicPort)
              .map((p) => ({ host: p.PublicPort!, container: p.PrivatePort ?? p.PublicPort!, proto: p.Type ?? 'tcp' })),
      env,
      mounts,
    }
  }, [raw, inspect, stats])

  if (loading) {
    return (
      <AppShell title="Containers">
        <div className={styles.page}>Loading…</div>
      </AppShell>
    )
  }

  if (notFound || !c.id) {
    return (
      <AppShell title="Containers">
        <EmptyState
          title="Container not found"
          description="It may have been removed, or this link is out of date."
          action={
            <Button as={Link} to="/containers" variant="ghost">
              <ArrowLeft size={14} />
              Back to containers
            </Button>
          }
        />
      </AppShell>
    )
  }

  async function start() {
    await startContainer(id)
    await load()
  }
  async function restart() {
    await restartContainer(id)
    await load()
  }
  async function stop() {
    await stopContainer(id)
    await load()
  }
  async function remove() {
    await removeContainer(id)
    navigate('/containers')
  }

  return (
    <AppShell title={c.name}>
      <div className={styles.page}>
        <Link to="/containers" className={styles.backLink}>
          <ArrowLeft size={14} />
          Back to containers
        </Link>

        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">{nodeName}</p>
            <h2 className={styles.reportTitle}>{c.name}</h2>
          </div>
          <div className={styles.headerRight}>
            <Badge variant={stateVariant(c.status)}>{c.status}</Badge>
            <div className={styles.actions}>
              <Button variant="ghost" disabled={c.status === 'running'} onClick={start}>
                <Play size={14} />
                Start
              </Button>
              <Button variant="ghost" disabled={c.status !== 'running'} onClick={restart}>
                <RotateCw size={14} />
                Restart
              </Button>
              <Button variant="ghost" disabled={c.status !== 'running'} onClick={stop}>
                <Square size={14} />
                Stop
              </Button>
              <Button variant="ghost" onClick={remove}>
                <Trash2 size={14} />
                Remove
              </Button>
            </div>
          </div>
        </div>

        <div className={styles.infoGrid}>
          <Section title="Details">
            <dl className={styles.detailList}>
              <div>
                <dt>Container ID</dt>
                <dd className={styles.mono}>{c.id.slice(0, 12)}</dd>
              </div>
              <div>
                <dt>Node</dt>
                <dd>{nodeName}</dd>
              </div>
              <div>
                <dt>Image</dt>
                <dd className={styles.mono}>{c.image}</dd>
              </div>
              <div>
                <dt>Image ID</dt>
                <dd className={styles.mono}>{c.imageId}</dd>
              </div>
              <div>
                <dt>Runtime</dt>
                <dd>{c.runtime}</dd>
              </div>
              <div>
                <dt>Created</dt>
                <dd className={styles.mono}>{c.created}</dd>
              </div>
              <div>
                <dt>Uptime</dt>
                <dd>{c.uptime}</dd>
              </div>
              <div>
                <dt>Restart policy</dt>
                <dd>{c.restartPolicy}</dd>
              </div>
              <div>
                <dt>Restarts</dt>
                <dd>{c.restartCount}</dd>
              </div>
            </dl>
          </Section>

          <Section
            title="Resources"
            action={
              <button type="button" className={styles.refreshBtn} onClick={refreshStats} disabled={statsLoading} aria-label="Refresh stats">
                <RefreshCw size={14} />
              </button>
            }
          >
            <div className={styles.resourceList}>
              <div className={styles.resourceRow}>
                <span className={styles.resourceLabel}>CPU</span>
                <ResourceBar pct={c.cpuRaw} label={c.cpu} />
              </div>
              <div className={styles.resourceRow}>
                <span className={styles.resourceLabel}>Memory</span>
                <ResourceBar pct={c.memRaw} label={c.memory} />
              </div>
              <div className={styles.statLine}>
                <span>Network</span>
                <span className="cell-muted">
                  <span className={styles.arrowDown}>↓</span>
                  {c.netRx} / <span className={styles.arrowUp}>↑</span>
                  {c.netTx}
                </span>
              </div>
              <div className={styles.statLine}>
                <span>Disk</span>
                <span className="cell-muted">
                  <span className={styles.arrowDown}>R</span>
                  {c.blockRead} / <span className={styles.arrowUp}>W</span>
                  {c.blockWrite}
                </span>
              </div>
            </div>
          </Section>

          <Section title="Network">
            <dl className={styles.detailList}>
              <div>
                <dt>IP address</dt>
                <dd className={styles.mono}>{c.ip}</dd>
              </div>
              <div>
                <dt>Gateway</dt>
                <dd className={styles.mono}>{c.gateway}</dd>
              </div>
              <div>
                <dt>MAC address</dt>
                <dd className={styles.mono}>{c.mac}</dd>
              </div>
              <div>
                <dt>Network mode</dt>
                <dd>{c.networkMode}</dd>
              </div>
            </dl>
            {c.ports.length > 0 && (
              <dl className={styles.detailList}>
                <div>
                  <dt>Ports</dt>
                  <dd>
                    {c.ports.map((p, i) => (
                      <div className={styles.mono} key={i}>
                        0.0.0.0:{p.host} → {p.container}/{p.proto}
                      </div>
                    ))}
                  </dd>
                </div>
              </dl>
            )}
          </Section>
        </div>

        <div className={styles.secondRow}>
          <Section title="Environment">
            {c.env.length === 0 ? (
              <EmptyState title="No environment variables" description="This container was started without any custom env vars." />
            ) : (
              <ul className={styles.envList}>
                {c.env.map((e) => (
                  <li key={e}>{e}</li>
                ))}
              </ul>
            )}
          </Section>

          <Section title="Mounts">
            {c.mounts.length === 0 ? (
              <EmptyState title="No mounts" description="This container doesn't bind-mount or attach any volumes." />
            ) : (
              <Table>
                <thead>
                  <tr>
                    <th>Type</th>
                    <th>Source</th>
                    <th>Destination</th>
                    <th>Mode</th>
                  </tr>
                </thead>
                <tbody>
                  {c.mounts.map((m, i) => (
                    <tr key={i}>
                      <td>
                        <Badge variant="neutral">{m.type}</Badge>
                      </td>
                      <td className="cell-muted">{m.source}</td>
                      <td className="cell-muted">{m.destination}</td>
                      <td className="cell-muted">{m.mode}</td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            )}
          </Section>
        </div>

        <Section
          title="Logs"
          action={
            <div className={styles.logActions}>
              <div className={styles.followToggle}>
                <Checkbox checked={follow} onChange={(e) => setFollow(e.target.checked)}>
                  Follow
                </Checkbox>
              </div>
              <Button variant="ghost" onClick={() => setLogs([])}>
                Clear
              </Button>
            </div>
          }
        >
          {logs.length === 0 ? (
            <EmptyState title="No log lines" description="Logs were cleared, or this container hasn't emitted anything yet." />
          ) : (
            <div className={styles.logList}>
              {logs.map((l, i) => (
                <div className={styles.logRow} key={i}>
                  <span className={styles.logTime}>{l.ts}</span>
                  <span className={l.stream === 'stderr' ? styles.logStreamErr : styles.logStreamOut}>{l.stream}</span>
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

export default ContainerDetail
