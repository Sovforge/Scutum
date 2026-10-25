import { useEffect, useState } from 'react'
import { Box, Boxes, Network, Server } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import ResourceBar from '../../components/ui/ResourceBar/ResourceBar'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import TopologyMap from '../../components/TopologyMap/TopologyMap'
import type { MeshNode } from '../../components/TopologyMap/TopologyMap'
import { positionFromMeshIp } from '../../lib/nodePosition'
import {
  getK8sSummary,
  getMeshSummary,
  listContainers,
  listNodes,
  getContainerStats,
  type ContainerStats,
  type DockerContainer,
  type K8sSummary,
  type MeshSummary,
  type NodeRecord,
} from '../../lib/api'
import styles from './Dashboard.module.css'

function roleVariant(role: string) {
  return role === 'hub' ? 'success' : role === 'combined' ? 'warning' : 'neutral'
}

function fmtBytes(n: number): string {
  if (n < 1048576) return `${(n / 1024).toFixed(0)}K`
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)}M`
  return `${(n / 1073741824).toFixed(2)}G`
}

function containerName(c: DockerContainer): string {
  return (c.Names?.[0] ?? c.Id.slice(0, 12)).replace(/^\//, '')
}

function Dashboard() {
  const [nodes, setNodes] = useState<NodeRecord[]>([])
  const [containers, setContainers] = useState<DockerContainer[]>([])
  const [containerStats, setContainerStats] = useState<Record<string, ContainerStats>>({})
  const [meshSummary, setMeshSummary] = useState<MeshSummary>({ total: 0, healthy: 0, degraded: 0 })
  const [k8s, setK8s] = useState<K8sSummary | null>(null)

  useEffect(() => {
    listNodes().then(setNodes).catch(() => {})
    listContainers()
      .then((ctrs) => {
        setContainers(ctrs)
        const running = ctrs.filter((c) => c.State === 'running')
        if (running.length === 0) return
        Promise.allSettled(running.map((c) => getContainerStats(c.Id).then((s) => [c.Id, s] as const))).then(
          (results) => {
            const map: Record<string, ContainerStats> = {}
            for (const r of results) {
              if (r.status === 'fulfilled') map[r.value[0]] = r.value[1]
            }
            setContainerStats(map)
          },
        )
      })
      .catch(() => {})
    getMeshSummary()
      .then(setMeshSummary)
      .catch(() => {})
    // Local explicitly — this is a hub-wide overview, not scoped to
    // whatever node the topbar switcher happens to be pointed at.
    getK8sSummary(null)
      .then(setK8s)
      .catch(() => {})
  }, [])

  const runningContainers = containers.filter((c) => c.State === 'running').length
  const meshTotal = meshSummary.total
  const meshHealthy = meshSummary.healthy === meshTotal

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
    <AppShell title="Dashboard">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Mesh Overview</p>
            <h2 className={styles.reportTitle}>Situation report</h2>
          </div>
          <span className="stamp">Operational</span>
        </div>

        <StatGrid>
          <StatCard label="Nodes" value={nodes.length} sub="enrolled in mesh" icon={<Server size={16} />} />
          <StatCard
            label="Containers"
            value={`${runningContainers}/${containers.length}`}
            sub="running"
            icon={<Box size={16} />}
          />
          <StatCard
            label="Mesh health"
            value={meshTotal === 0 ? '—' : `${meshSummary.healthy}/${meshTotal}`}
            sub={meshTotal === 0 ? 'no peers' : `${meshSummary.degraded} degraded`}
            icon={<Network size={16} />}
            color={meshTotal === 0 || meshSummary.degraded > 0 ? 'var(--rust)' : 'var(--blueprint)'}
          />
          <StatCard
            label="Kubernetes pods"
            value={k8s?.pods ?? '—'}
            sub={k8s ? `across ${k8s.deployments} deployments` : 'no cluster detected'}
            icon={<Boxes size={16} />}
          />
        </StatGrid>

        <div className={styles.topologyRow}>
          <Section title="Mesh topology" frame>
            {nodes.length === 0 ? (
              <EmptyState icon={Network} title="No nodes registered" />
            ) : (
              <TopologyMap nodes={graphNodes} />
            )}
          </Section>

          <Section title="Nodes">
            {nodes.length === 0 ? (
              <EmptyState icon={Server} title="No nodes registered" />
            ) : (
              <Table>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Role</th>
                    <th>Address</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {nodes.map((n) => (
                    <tr key={n.id}>
                      <td className="cell-name">{n.name}</td>
                      <td>
                        <Badge variant={roleVariant(n.type)}>{n.type}</Badge>
                      </td>
                      <td className="cell-muted">{n.address}</td>
                      <td>
                        <Badge variant={meshHealthy ? 'success' : 'warning'}>{meshHealthy ? 'healthy' : 'degraded'}</Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            )}
          </Section>
        </div>

        <Section title="Containers" action={<span className={styles.manifestNo}>Manifest No. {String(containers.length).padStart(3, '0')}</span>}>
          {containers.length === 0 ? (
            <EmptyState icon={Box} title="No containers found" />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>#</th>
                  <th>Name</th>
                  <th>Image</th>
                  <th>Status</th>
                  <th>CPU</th>
                  <th>Memory</th>
                </tr>
              </thead>
              <tbody>
                {containers.map((c, i) => {
                  const s = containerStats[c.Id]
                  const isRunning = c.State === 'running'
                  const cpuPct = s ? Math.min(s.cpu_percent, 100) : 0
                  const memPct = s && s.mem_limit > 0 ? Math.min((s.mem_usage / s.mem_limit) * 100, 100) : 0
                  return (
                    <tr key={c.Id}>
                      <td className="cell-muted">{String(i + 1).padStart(3, '0')}</td>
                      <td className="cell-name">{containerName(c)}</td>
                      <td className="cell-muted">{c.Image}</td>
                      <td>
                        <Badge variant={isRunning ? 'success' : 'neutral'}>{c.State}</Badge>
                      </td>
                      <td>
                        {s ? (
                          <ResourceBar pct={cpuPct} label={`${s.cpu_percent.toFixed(1)}%`} />
                        ) : (
                          <span className="cell-muted">{isRunning ? '…' : '—'}</span>
                        )}
                      </td>
                      <td>
                        {s ? (
                          <ResourceBar pct={memPct} label={fmtBytes(s.mem_usage)} />
                        ) : (
                          <span className="cell-muted">{isRunning ? '…' : '—'}</span>
                        )}
                      </td>
                    </tr>
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

export default Dashboard
