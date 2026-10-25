import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Box, ChevronDown, ChevronRight, Database, FileUp, Trash2, Upload } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import Select from '../../components/ui/Select/Select'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import Callout from '../../components/ui/Callout/Callout'
import {
  ApiError,
  deployCompose,
  listContainers,
  listNodes,
  listVolumes,
  pruneVolumes,
  removeVolume,
  type DockerContainer,
  type DockerVolume,
  type NodeRecord,
} from '../../lib/api'
import { useNodeSelection } from '../../lib/nodeSelection'
import styles from './Containers.module.css'

type Filter = 'all' | 'running' | 'exited' | 'paused'
const FILTERS: Filter[] = ['all', 'running', 'exited', 'paused']

type NodeTarget = { id: string; name: string; role: string; nodeId?: string }
type Group = { node: NodeTarget; containers: DockerContainer[] }
type VolumeGroup = { node: NodeTarget; volumes: DockerVolume[] }

function formatBytes(n: number): string {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v < 10 && i > 0 ? 1 : 0)} ${units[i]}`
}

function containerName(c: DockerContainer): string {
  return (c.Names?.[0] ?? c.Id.slice(0, 12)).replace(/^\//, '')
}
function containerPorts(c: DockerContainer): string {
  const mapped = (c.Ports ?? []).filter((p) => p.PublicPort).map((p) => p.PublicPort)
  return mapped.length ? mapped.join(', ') : '—'
}
function stateVariant(state: string): 'success' | 'warning' | 'neutral' {
  if (state === 'running') return 'success'
  if (state === 'paused') return 'warning'
  return 'neutral'
}

function Containers() {
  const navigate = useNavigate()
  const { selectNode } = useNodeSelection()

  const [groups, setGroups] = useState<Group[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())

  async function loadContainers() {
    setLoading(true)
    setApiError('')
    try {
      const nodes = await listNodes().catch(() => [] as NodeRecord[])
      const targets: NodeTarget[] = [
        { id: 'local', name: 'Local', role: 'hub' },
        ...nodes.filter((n) => n.type !== 'hub').map((n) => ({ id: n.id, name: n.name, role: n.type, nodeId: n.id })),
      ]
      const results = await Promise.allSettled(targets.map((t) => listContainers(t.nodeId)))
      setGroups(targets.map((t, i) => {
        const res = results[i]
        return { node: t, containers: res.status === 'fulfilled' ? res.value : [] }
      }))
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load containers')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadContainers()
    loadVolumes()
  }, [])

  // ── Volumes ───────────────────────────────────────────────────────────
  const [volumeGroups, setVolumeGroups] = useState<VolumeGroup[]>([])
  const [volumesLoading, setVolumesLoading] = useState(true)
  const [volumesError, setVolumesError] = useState('')
  const [volumeCollapsed, setVolumeCollapsed] = useState<Set<string>>(new Set())
  const [volumeActionError, setVolumeActionError] = useState<Record<string, string>>({})
  const [pruneConfirmFor, setPruneConfirmFor] = useState<string | null>(null)
  const [pruning, setPruning] = useState<Set<string>>(new Set())

  async function loadVolumes() {
    setVolumesLoading(true)
    setVolumesError('')
    try {
      const nodes = await listNodes().catch(() => [] as NodeRecord[])
      const targets: NodeTarget[] = [
        { id: 'local', name: 'Local', role: 'hub' },
        ...nodes.filter((n) => n.type !== 'hub').map((n) => ({ id: n.id, name: n.name, role: n.type, nodeId: n.id })),
      ]
      const results = await Promise.allSettled(targets.map((t) => listVolumes(t.nodeId)))
      setVolumeGroups(targets.map((t, i) => {
        const res = results[i]
        return { node: t, volumes: res.status === 'fulfilled' ? res.value : [] }
      }))
    } catch (e) {
      setVolumesError(e instanceof ApiError ? e.message : 'Failed to load volumes')
    } finally {
      setVolumesLoading(false)
    }
  }

  function toggleVolumeNode(id: string) {
    setVolumeCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleRemoveVolume(name: string, nodeId: string | undefined, groupId: string) {
    setVolumeActionError((prev) => ({ ...prev, [`${groupId}:${name}`]: '' }))
    try {
      await removeVolume(name, nodeId)
      await loadVolumes()
    } catch (e) {
      const message = e instanceof ApiError ? e.message : 'Failed to remove volume'
      setVolumeActionError((prev) => ({ ...prev, [`${groupId}:${name}`]: message }))
    }
  }

  async function handlePruneVolumes(nodeId: string | undefined, groupId: string) {
    setPruning((prev) => new Set(prev).add(groupId))
    setPruneConfirmFor(null)
    try {
      await pruneVolumes(nodeId)
      await loadVolumes()
    } catch (e) {
      const message = e instanceof ApiError ? e.message : 'Failed to prune volumes'
      setVolumeActionError((prev) => ({ ...prev, [`${groupId}:__prune`]: message }))
    } finally {
      setPruning((prev) => {
        const next = new Set(prev)
        next.delete(groupId)
        return next
      })
    }
  }

  function openContainer(id: string, nodeId?: string) {
    selectNode(nodeId ?? null)
    navigate(`/containers/${id}`)
  }

  function toggleNode(id: string) {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const q = search.trim().toLowerCase()
  function matches(c: DockerContainer) {
    const state = c.State?.toLowerCase() ?? ''
    if (filter !== 'all' && state !== filter) return false
    if (q) return containerName(c).toLowerCase().includes(q) || (c.Image ?? '').toLowerCase().includes(q)
    return true
  }

  const filteredGroups = groups
    .map((g) => ({ node: g.node, containers: g.containers.filter(matches) }))
    .filter((g) => g.containers.length > 0)

  const allContainers = groups.flatMap((g) => g.containers)
  const total = allContainers.length
  const running = allContainers.filter((c) => c.State === 'running').length
  const exited = allContainers.filter((c) => c.State === 'exited').length
  const paused = allContainers.filter((c) => c.State === 'paused').length

  // ── Deploy compose ───────────────────────────────────────────────────────
  const [showDeploy, setShowDeploy] = useState(false)
  const [deployNodes, setDeployNodes] = useState<NodeRecord[]>([])
  const [deployNodeId, setDeployNodeId] = useState('')
  const [deployFile, setDeployFile] = useState<File | null>(null)
  const [deploying, setDeploying] = useState(false)
  const [deployError, setDeployError] = useState('')
  const [deployOutput, setDeployOutput] = useState('')

  function openDeploy() {
    setShowDeploy((v) => !v)
    if (deployNodes.length === 0) {
      listNodes()
        .then((nodes) => setDeployNodes(nodes.filter((n) => n.type !== 'hub')))
        .catch(() => {})
    }
  }

  function onFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    setDeployFile(e.target.files?.[0] ?? null)
    setDeployError('')
    setDeployOutput('')
  }

  async function runDeploy(e: FormEvent) {
    e.preventDefault()
    if (!deployFile) return
    setDeploying(true)
    setDeployError('')
    setDeployOutput('')
    try {
      const text = await deployFile.text()
      const res = await deployCompose(text, deployNodeId || undefined)
      setDeployOutput(res.output ?? 'Deployed.')
      await loadContainers()
    } catch (e2) {
      setDeployError(e2 instanceof ApiError ? e2.message : 'Deploy failed.')
    } finally {
      setDeploying(false)
    }
  }

  return (
    <AppShell title="Containers">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Docker Fleet</p>
            <h2 className={styles.reportTitle}>Container registry</h2>
          </div>
          <span className="stamp">
            {running}/{total} running
          </span>
        </div>

        <StatGrid minWidth={140}>
          <StatCard label="Total" value={total} icon={<Box size={16} />} />
          <StatCard label="Running" value={running} color="var(--blueprint)" />
          <StatCard label="Exited" value={exited} color="var(--paper-dim)" />
          <StatCard label="Paused" value={paused} color="var(--rust)" />
        </StatGrid>

        <Section>
          <div className={styles.toolbar}>
            <input
              className={styles.toolbarInput}
              placeholder="Filter by name or image…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <div className={styles.filterChips}>
              {FILTERS.map((f) => (
                <button
                  key={f}
                  type="button"
                  className={filter === f ? `${styles.chip} ${styles.chipActive}` : styles.chip}
                  onClick={() => setFilter(f)}
                >
                  {f === 'all' ? 'All' : f[0].toUpperCase() + f.slice(1)}
                </button>
              ))}
            </div>
            <Button variant="ghost" onClick={openDeploy}>
              <Upload size={14} />
              Deploy
            </Button>
          </div>

          {showDeploy && (
            <form className={styles.deployForm} onSubmit={runDeploy}>
              <div className={styles.deployFields}>
                <Select
                  label="Target node"
                  id="deployNode"
                  value={deployNodeId}
                  onChange={(e) => setDeployNodeId(e.target.value)}
                  options={[
                    { value: '', label: 'Local (this node)' },
                    ...deployNodes.map((n) => ({ value: n.id, label: `${n.name} — ${n.address}` })),
                  ]}
                />
                <div className="field">
                  <label htmlFor="composeFile">Compose file</label>
                  <div className={deployFile ? `${styles.dropzone} ${styles.dropzoneActive}` : styles.dropzone}>
                    <input id="composeFile" type="file" accept=".yml,.yaml" className={styles.dropzoneInput} onChange={onFileChange} />
                    <div className={styles.dropzoneLabel}>
                      <FileUp size={16} />
                      <span className={styles.dropzoneFileName}>{deployFile?.name ?? 'Choose or drop a file…'}</span>
                      <span className={styles.dropzoneHint}>.yml / .yaml</span>
                    </div>
                  </div>
                </div>
              </div>
              {deployError && <Callout variant="danger">{deployError}</Callout>}
              {deployOutput && <Callout variant="success">{deployOutput}</Callout>}
              <div className={styles.deployActions}>
                <Button type="submit" disabled={!deployFile || deploying}>
                  <Upload size={14} />
                  {deploying ? 'Deploying…' : 'Deploy stack'}
                </Button>
              </div>
            </form>
          )}
        </Section>

        {apiError ? (
          <Section>
            <Callout variant="danger">{apiError}</Callout>
          </Section>
        ) : loading ? (
          <Section>
            <div className={styles.toolbar}>Loading…</div>
          </Section>
        ) : filteredGroups.length === 0 ? (
          <Section>
            <EmptyState
              icon={Box}
              title="No containers match"
              description="Nothing matches the current filter and search query."
              action={
                <Button
                  variant="ghost"
                  onClick={() => {
                    setSearch('')
                    setFilter('all')
                  }}
                >
                  Clear filters
                </Button>
              }
            />
          </Section>
        ) : (
          filteredGroups.map((group) => {
            const groupRunning = group.containers.filter((c) => c.State === 'running').length
            const isCollapsed = collapsed.has(group.node.id)
            return (
              <Section
                key={group.node.id}
                title={group.node.name}
                action={
                  <div className={styles.sectionActions}>
                    <Badge variant="neutral">
                      {groupRunning}/{group.containers.length} running
                    </Badge>
                    <button
                      type="button"
                      className={styles.collapseToggle}
                      onClick={() => toggleNode(group.node.id)}
                      aria-label={isCollapsed ? `Expand ${group.node.name}` : `Collapse ${group.node.name}`}
                    >
                      <ChevronDown size={16} className={isCollapsed ? styles.chevronCollapsed : undefined} />
                    </button>
                  </div>
                }
              >
                {!isCollapsed && (
                  <Table>
                    <thead>
                      <tr>
                        <th>Name</th>
                        <th>Image</th>
                        <th>State</th>
                        <th>Ports</th>
                        <th>Status</th>
                        <th />
                      </tr>
                    </thead>
                    <tbody>
                      {group.containers.map((c) => (
                        <tr key={c.Id} onClick={() => openContainer(c.Id, group.node.nodeId)} className={styles.row}>
                          <td className="cell-name">{containerName(c)}</td>
                          <td className="cell-muted">{c.Image}</td>
                          <td>
                            <Badge variant={stateVariant(c.State)}>{c.State}</Badge>
                          </td>
                          <td className="cell-muted">{containerPorts(c)}</td>
                          <td className="cell-muted">{c.Status}</td>
                          <td>
                            <ChevronRight size={14} className={styles.rowArrow} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </Table>
                )}
              </Section>
            )
          })
        )}

        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Docker Fleet</p>
            <h2 className={styles.reportTitle}>Volumes</h2>
          </div>
        </div>

        {volumesError ? (
          <Section>
            <Callout variant="danger">{volumesError}</Callout>
          </Section>
        ) : volumesLoading ? (
          <Section>
            <div className={styles.toolbar}>Loading…</div>
          </Section>
        ) : (
          volumeGroups.map((group) => {
            const groupId = group.node.id
            const isCollapsed = volumeCollapsed.has(groupId)
            const unused = group.volumes.filter((v) => !v.InUse)
            const pruneError = volumeActionError[`${groupId}:__prune`]
            const isPruning = pruning.has(groupId)
            return (
              <Section
                key={groupId}
                title={group.node.name}
                action={
                  <div className={styles.sectionActions}>
                    <Badge variant="neutral">{group.volumes.length} volumes</Badge>
                    <Button
                      variant="ghost"
                      onClick={() => setPruneConfirmFor(groupId)}
                      disabled={unused.length === 0 || isPruning}
                    >
                      <Trash2 size={14} />
                      {isPruning ? 'Pruning…' : 'Prune unused'}
                    </Button>
                    <button
                      type="button"
                      className={styles.collapseToggle}
                      onClick={() => toggleVolumeNode(groupId)}
                      aria-label={isCollapsed ? `Expand ${group.node.name}` : `Collapse ${group.node.name}`}
                    >
                      <ChevronDown size={16} className={isCollapsed ? styles.chevronCollapsed : undefined} />
                    </button>
                  </div>
                }
              >
                {pruneConfirmFor === groupId && (
                  <Callout variant="warning">
                    <p>
                      This will permanently delete {unused.length} unused volume{unused.length === 1 ? '' : 's'}:{' '}
                      {unused.map((v) => v.Name).join(', ')}
                    </p>
                    <div className={styles.sectionActions}>
                      <Button variant="ghost" onClick={() => handlePruneVolumes(group.node.nodeId, groupId)}>
                        Confirm prune
                      </Button>
                      <Button variant="ghost" onClick={() => setPruneConfirmFor(null)}>
                        Cancel
                      </Button>
                    </div>
                  </Callout>
                )}
                {pruneError && <Callout variant="danger">{pruneError}</Callout>}

                {!isCollapsed && (
                  group.volumes.length === 0 ? (
                    <EmptyState icon={Database} title="No volumes" description="This node has no Docker volumes." />
                  ) : (
                    <Table>
                      <thead>
                        <tr>
                          <th>Name</th>
                          <th>Driver</th>
                          <th>Mountpoint</th>
                          <th>Size</th>
                          <th>Created</th>
                          <th>Status</th>
                          <th />
                        </tr>
                      </thead>
                      <tbody>
                        {group.volumes.map((v) => {
                          const rowError = volumeActionError[`${groupId}:${v.Name}`]
                          return (
                            <tr key={v.Name}>
                              <td className="cell-name">{v.Name}</td>
                              <td className="cell-muted">{v.Driver}</td>
                              <td className="cell-muted">{v.Mountpoint}</td>
                              <td className="cell-muted">{formatBytes(v.Size)}</td>
                              <td className="cell-muted">{v.CreatedAt ? new Date(v.CreatedAt).toLocaleString() : '—'}</td>
                              <td>
                                <Badge variant={v.InUse ? 'warning' : 'neutral'}>{v.InUse ? 'In use' : 'Unused'}</Badge>
                                {rowError && <div className={styles.inlineError}>{rowError}</div>}
                              </td>
                              <td>
                                <Button
                                  variant="ghost"
                                  disabled={v.InUse}
                                  title={v.InUse ? 'Volume is in use by a container' : 'Remove volume'}
                                  onClick={() => handleRemoveVolume(v.Name, group.node.nodeId, groupId)}
                                >
                                  <Trash2 size={14} />
                                </Button>
                              </td>
                            </tr>
                          )
                        })}
                      </tbody>
                    </Table>
                  )
                )}
              </Section>
            )
          })
        )}
      </div>
    </AppShell>
  )
}

export default Containers
