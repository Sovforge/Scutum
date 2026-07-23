import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Box, ChevronDown, ChevronRight, FileUp, Upload } from 'lucide-react'
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
  type DockerContainer,
  type NodeRecord,
} from '../../lib/api'
import { useNodeSelection } from '../../lib/nodeSelection'
import styles from './Containers.module.css'

type Filter = 'all' | 'running' | 'exited' | 'paused'
const FILTERS: Filter[] = ['all', 'running', 'exited', 'paused']

type NodeTarget = { id: string; name: string; role: string; nodeId?: string }
type Group = { node: NodeTarget; containers: DockerContainer[] }

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
  }, [])

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
      </div>
    </AppShell>
  )
}

export default Containers
