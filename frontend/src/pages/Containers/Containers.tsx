import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Box, ChevronDown, ChevronRight, Database, Download, FileUp, Layers, Sparkles, Trash2, Upload } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import Select from '../../components/ui/Select/Select'
import TextField from '../../components/ui/TextField/TextField'
import PasswordField from '../../components/ui/PasswordField/PasswordField'
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
  listGroupImages,
  listImages,
  listNodeGroups,
  listNodes,
  listVolumes,
  pruneImages,
  pruneVolumes,
  pullImage,
  pullImageToGroup,
  removeImage,
  removeVolume,
  type DockerContainer,
  type DockerImage,
  type DockerVolume,
  type GroupNodeImages,
  type NodeGroup,
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
  const [pruningVolumes, setPruningVolumes] = useState<Set<string>>(new Set())

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
    setPruningVolumes((prev) => new Set(prev).add(groupId))
    setPruneConfirmFor(null)
    try {
      await pruneVolumes(nodeId)
      await loadVolumes()
    } catch (e) {
      const message = e instanceof ApiError ? e.message : 'Failed to prune volumes'
      setVolumeActionError((prev) => ({ ...prev, [`${groupId}:__prune`]: message }))
    } finally {
      setPruningVolumes((prev) => {
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

  // ── Images ───────────────────────────────────────────────────────────────
  type ImageGroup = { node: NodeTarget; images: DockerImage[]; error?: string }
  const [imageGroups, setImageGroups] = useState<ImageGroup[]>([])
  const [imagesLoading, setImagesLoading] = useState(true)
  const [imagesCollapsed, setImagesCollapsed] = useState<Set<string>>(new Set())
  const [pruning, setPruning] = useState('')

  const [showPull, setShowPull] = useState(false)
  const [pullNodeId, setPullNodeId] = useState('')
  const [pullRef, setPullRef] = useState('')
  const [pullUsername, setPullUsername] = useState('')
  const [pullPassword, setPullPassword] = useState('')
  const [pulling, setPulling] = useState(false)
  const [pullError, setPullError] = useState('')
  const [pullResult, setPullResult] = useState('')

  async function loadImages() {
    setImagesLoading(true)
    try {
      const nodes = await listNodes().catch(() => [] as NodeRecord[])
      const targets: NodeTarget[] = [
        { id: 'local', name: 'Local', role: 'hub' },
        ...nodes.filter((n) => n.type !== 'hub').map((n) => ({ id: n.id, name: n.name, role: n.type, nodeId: n.id })),
      ]
      const results = await Promise.allSettled(targets.map((t) => listImages(t.nodeId)))
      setImageGroups(
        targets.map((t, i) => {
          const res = results[i]
          return res.status === 'fulfilled'
            ? { node: t, images: res.value }
            : { node: t, images: [], error: res.reason instanceof ApiError ? res.reason.message : 'Failed to load images' }
        }),
      )
    } finally {
      setImagesLoading(false)
    }
  }

  useEffect(() => {
    loadImages()
  }, [])

  function toggleImageNode(id: string) {
    setImagesCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function tagOf(img: DockerImage): string {
    return img.RepoTags?.[0] ?? `${img.Id.replace('sha256:', '').slice(0, 12)} (untagged)`
  }
  function fmtSize(bytes: number): string {
    if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(2)} GB`
    if (bytes >= 1e6) return `${(bytes / 1e6).toFixed(1)} MB`
    return `${(bytes / 1e3).toFixed(0)} KB`
  }
  function fmtCreated(unixSeconds: number): string {
    try {
      return new Date(unixSeconds * 1000).toLocaleDateString()
    } catch {
      return '—'
    }
  }

  async function openPull() {
    setShowPull((v) => !v)
    if (deployNodes.length === 0) {
      listNodes()
        .then((nodes) => setDeployNodes(nodes.filter((n) => n.type !== 'hub')))
        .catch(() => {})
    }
  }

  async function submitPull(e: FormEvent) {
    e.preventDefault()
    if (!pullRef) {
      setPullError('Image reference is required (e.g. nginx:latest).')
      return
    }
    setPulling(true)
    setPullError('')
    setPullResult('')
    try {
      const res = await pullImage(
        { image: pullRef, username: pullUsername || undefined, password: pullPassword || undefined },
        pullNodeId || undefined,
      )
      if (!res.success) {
        setPullError(res.error || 'Pull failed.')
      } else {
        setPullResult(`${pullRef} pulled.`)
        setPullRef('')
        setPullUsername('')
        setPullPassword('')
        await loadImages()
      }
    } catch (e2) {
      setPullError(e2 instanceof ApiError ? e2.message : 'Pull failed.')
    } finally {
      setPulling(false)
    }
  }

  async function doRemoveImage(img: DockerImage, nodeId?: string) {
    const key = `${nodeId ?? 'local'}:${img.Id}`
    setBusyImageKey(key)
    try {
      await removeImage(img.Id, nodeId)
      await loadImages()
    } catch (e) {
      setImagesError(e instanceof ApiError ? e.message : 'Remove failed.')
    } finally {
      setBusyImageKey('')
    }
  }

  async function doPrune(nodeKey: string, nodeId?: string) {
    setPruning(nodeKey)
    try {
      await pruneImages(nodeId)
      await loadImages()
    } catch (e) {
      setImagesError(e instanceof ApiError ? e.message : 'Prune failed.')
    } finally {
      setPruning('')
    }
  }

  const [imagesError, setImagesError] = useState('')
  const [busyImageKey, setBusyImageKey] = useState('')

  // ── Image diff across a group ──────────────────────────────────────────
  const [groups2, setGroups2] = useState<NodeGroup[]>([])
  const [diffGroupId, setDiffGroupId] = useState('')
  const [diffData, setDiffData] = useState<GroupNodeImages[]>([])
  const [diffLoading, setDiffLoading] = useState(false)
  const [diffError, setDiffError] = useState('')

  useEffect(() => {
    listNodeGroups()
      .then(setGroups2)
      .catch(() => {})
  }, [])

  async function loadDiff(groupId: string) {
    setDiffGroupId(groupId)
    setDiffData([])
    setDiffError('')
    if (!groupId) return
    setDiffLoading(true)
    try {
      setDiffData(await listGroupImages(groupId))
    } catch (e) {
      setDiffError(e instanceof ApiError ? e.message : 'Failed to load group images')
    } finally {
      setDiffLoading(false)
    }
  }

  async function pullToGroupFromDiff() {
    if (!diffGroupId || !pullRef) return
    setPulling(true)
    setPullError('')
    try {
      await pullImageToGroup(diffGroupId, { image: pullRef, username: pullUsername || undefined, password: pullPassword || undefined })
      await loadDiff(diffGroupId)
      setPullResult(`${pullRef} pulled to group.`)
    } catch (e) {
      setPullError(e instanceof ApiError ? e.message : 'Group pull failed.')
    } finally {
      setPulling(false)
    }
  }

  const diffTags = Array.from(new Set(diffData.flatMap((d) => (d.images ?? []).flatMap((img) => img.RepoTags ?? [])))).sort()

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

        <Section
          title="Container images"
          action={
            <Button variant="ghost" onClick={openPull}>
              <Download size={14} />
              Pull image
            </Button>
          }
        >
          {showPull && (
            <form className={styles.deployForm} onSubmit={submitPull}>
              <div className={styles.deployFields}>
                <Select
                  label="Target node"
                  id="pullNode"
                  value={pullNodeId}
                  onChange={(e) => setPullNodeId(e.target.value)}
                  options={[
                    { value: '', label: 'Local (this node)' },
                    ...deployNodes.map((n) => ({ value: n.id, label: `${n.name} — ${n.address}` })),
                  ]}
                />
                <TextField label="Image reference" id="pullRef" value={pullRef} onChange={(e) => setPullRef(e.target.value)} placeholder="nginx:latest" />
                <TextField label="Registry username (optional)" id="pullUsername" value={pullUsername} onChange={(e) => setPullUsername(e.target.value)} />
                <PasswordField label="Registry password (optional)" id="pullPassword" value={pullPassword} onChange={(e) => setPullPassword(e.target.value)} />
              </div>
              {pullError && <Callout variant="danger">{pullError}</Callout>}
              {pullResult && <Callout variant="success">{pullResult}</Callout>}
              <div className={styles.deployActions}>
                <Button type="submit" disabled={!pullRef || pulling}>
                  <Download size={14} />
                  {pulling ? 'Pulling…' : 'Pull to node'}
                </Button>
              </div>
            </form>
          )}

          {imagesError && <Callout variant="danger">{imagesError}</Callout>}
        </Section>

        {imagesLoading ? (
          <Section>
            <div className={styles.toolbar}>Loading images…</div>
          </Section>
        ) : (
          imageGroups.map((group) => {
            const nodeKey = group.node.nodeId ?? 'local'
            const isCollapsed = imagesCollapsed.has(group.node.id)
            return (
              <Section
                key={`images-${group.node.id}`}
                title={`${group.node.name} — images`}
                action={
                  <div className={styles.sectionActions}>
                    <Badge variant="neutral">{group.images.length}</Badge>
                    <Button variant="ghost" onClick={() => doPrune(nodeKey, group.node.nodeId)} disabled={pruning === nodeKey}>
                      <Sparkles size={14} />
                      {pruning === nodeKey ? 'Pruning…' : 'Prune dangling'}
                    </Button>
                    <button
                      type="button"
                      className={styles.collapseToggle}
                      onClick={() => toggleImageNode(group.node.id)}
                      aria-label={isCollapsed ? `Expand ${group.node.name} images` : `Collapse ${group.node.name} images`}
                    >
                      <ChevronDown size={16} className={isCollapsed ? styles.chevronCollapsed : undefined} />
                    </button>
                  </div>
                }
              >
                {!isCollapsed &&
                  (group.error ? (
                    <Callout variant="danger">{group.error}</Callout>
                  ) : group.images.length === 0 ? (
                    <EmptyState icon={Box} title="No images" description="Pull an image above to see it here." />
                  ) : (
                    <Table>
                      <thead>
                        <tr>
                          <th>Tag</th>
                          <th>Size</th>
                          <th>Created</th>
                          <th />
                        </tr>
                      </thead>
                      <tbody>
                        {group.images.map((img) => {
                          const key = `${nodeKey}:${img.Id}`
                          return (
                            <tr key={img.Id}>
                              <td className="cell-name">{tagOf(img)}</td>
                              <td className="cell-muted">{fmtSize(img.Size)}</td>
                              <td className="cell-muted">{fmtCreated(img.Created)}</td>
                              <td>
                                <div className={styles.rowActions}>
                                  <button
                                    type="button"
                                    className={styles.rowActionDanger}
                                    onClick={() => doRemoveImage(img, group.node.nodeId)}
                                    disabled={busyImageKey === key}
                                    aria-label={`Remove ${tagOf(img)}`}
                                  >
                                    <Trash2 size={14} />
                                  </button>
                                </div>
                              </td>
                            </tr>
                          )
                        })}
                      </tbody>
                    </Table>
                  ))}
              </Section>
            )
          })
        )}

        <Section title="Compare images across a group">
          <div className={styles.toolbar}>
            <Select
              label="Node group"
              id="diffGroup"
              value={diffGroupId}
              onChange={(e) => loadDiff(e.target.value)}
              options={[{ value: '', label: 'Select a group…' }, ...groups2.map((g) => ({ value: g.id, label: g.name }))]}
            />
            {diffGroupId && pullRef && (
              <Button variant="ghost" onClick={pullToGroupFromDiff} disabled={pulling}>
                <Download size={14} />
                Pull "{pullRef}" to every node in group
              </Button>
            )}
          </div>

          {diffError ? (
            <Callout variant="danger">{diffError}</Callout>
          ) : !diffGroupId ? (
            <EmptyState icon={Layers} title="No group selected" description="Pick a node group above to compare which images are present on each member." />
          ) : diffLoading ? (
            <div className={styles.toolbar}>Loading…</div>
          ) : diffTags.length === 0 ? (
            <EmptyState icon={Database} title="No images found" description="No member node reported any images." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Image</th>
                  {diffData.map((d) => (
                    <th key={d.node_id}>{d.node_name}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {diffTags.map((tag) => (
                  <tr key={tag}>
                    <td className="cell-name">{tag}</td>
                    {diffData.map((d) => {
                      const present = (d.images ?? []).some((img) => (img.RepoTags ?? []).includes(tag))
                      return (
                        <td key={d.node_id}>
                          <Badge variant={d.error ? 'danger' : present ? 'success' : 'neutral'}>
                            {d.error ? 'error' : present ? 'present' : 'missing'}
                          </Badge>
                        </td>
                      )
                    })}
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Section>

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
            const isPruning = pruningVolumes.has(groupId)
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
