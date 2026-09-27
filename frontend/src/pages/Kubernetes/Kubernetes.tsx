import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Boxes, FileUp, Layers, RefreshCw, Upload } from 'lucide-react'
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
import Tabs, { type TabItem } from '../../components/ui/Tabs/Tabs'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import {
  ApiError,
  applyK8s,
  getK8sSummary,
  listAllK8sPods,
  listDeployments,
  listNodes,
  restartDeployment,
  scaleDeployment,
  type K8sDeployment,
  type K8sSummary,
  type NodeRecord,
} from '../../lib/api'
import { useNodeSelection } from '../../lib/nodeSelection'
import styles from './Kubernetes.module.css'

type Phase = 'Running' | 'Pending' | 'Failed' | 'Succeeded' | string
const PHASE_VARIANT: Record<string, 'success' | 'warning' | 'danger' | 'neutral'> = {
  Running: 'success',
  Pending: 'warning',
  Failed: 'danger',
  Succeeded: 'neutral',
}

type Tab = 'pods' | 'deployments' | 'services' | 'config'
const TABS: TabItem[] = [
  { id: 'pods', label: 'Pods', icon: Boxes },
  { id: 'deployments', label: 'Deployments', icon: Layers },
  { id: 'services', label: 'Services' },
  { id: 'config', label: 'Config' },
]

type PhaseFilter = 'all' | 'Running' | 'Pending' | 'Failed'
const PHASE_FILTERS: PhaseFilter[] = ['all', 'Running', 'Pending', 'Failed']

type NodeSummary = { nodeId: string | null; nodeName: string; summary: K8sSummary | null }
type DeploymentRow = K8sDeployment & { meshNodeId: string | null; meshNodeName: string }
type PodRow = {
  uid: string
  name: string
  namespace: string
  node: string
  phase: Phase
  ready: string
  restarts: number
  age: string
  meshNodeId: string | null
  meshNodeName: string
}

function parseAge(ts: string): string {
  const diff = Date.now() - new Date(ts).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h`
  return `${Math.floor(hrs / 24)}d`
}

function parsePods(items: any[], meshNodeId: string | null, meshNodeName: string): PodRow[] {
  return items.map((item) => {
    const statuses = item.status?.containerStatuses ?? []
    const specCtrs = item.spec?.containers ?? []
    const readyCount = statuses.filter((s: any) => s.ready).length
    const total = statuses.length || specCtrs.length || 1
    const restarts = statuses.reduce((acc: number, s: any) => acc + (s.restartCount ?? 0), 0)
    return {
      uid: (meshNodeId ?? 'local') + '/' + (item.metadata?.uid ?? item.metadata?.name),
      name: item.metadata?.name ?? '',
      namespace: item.metadata?.namespace ?? 'default',
      node: item.spec?.nodeName ?? '—',
      phase: item.status?.phase ?? 'Unknown',
      ready: `${readyCount}/${total}`,
      restarts,
      age: parseAge(item.metadata?.creationTimestamp ?? new Date().toISOString()),
      meshNodeId,
      meshNodeName,
    }
  })
}

function Kubernetes() {
  const navigate = useNavigate()
  const { selectNode } = useNodeSelection()
  const toast = useToast()

  const [loading, setLoading] = useState(false)
  const [nodeSummaries, setNodeSummaries] = useState<NodeSummary[]>([])
  const [rawPods, setRawPods] = useState<PodRow[]>([])
  const [deployments, setDeployments] = useState<DeploymentRow[]>([])

  async function loadData(isStale: () => boolean) {
    setLoading(true)
    setRawPods([])
    setNodeSummaries([])
    setDeployments([])
    try {
      const nodes = await listNodes().catch(() => [] as NodeRecord[])
      if (isStale()) return
      const allNodes = [
        { id: null as string | null, name: 'Local' },
        ...nodes.filter((n) => n.type !== 'hub').map((n) => ({ id: n.id, name: n.name })),
      ]
      await Promise.all(
        allNodes.map(async (n) => {
          const [summ, podsResp, deploys] = await Promise.all([
            getK8sSummary(n.id).catch(() => null),
            listAllK8sPods(n.id).catch(() => null),
            listDeployments(n.id).catch(() => null),
          ])
          if (isStale()) return
          setNodeSummaries((prev) => [...prev, { nodeId: n.id, nodeName: n.name, summary: summ }])
          if (podsResp?.items) {
            setRawPods((prev) => [...prev, ...parsePods(podsResp.items!, n.id, n.name)])
          }
          if (deploys) {
            setDeployments((prev) => [...prev, ...deploys.map((d) => ({ ...d, meshNodeId: n.id, meshNodeName: n.name }))])
          }
        }),
      )
    } finally {
      if (!isStale()) setLoading(false)
    }
  }

  // StrictMode double-invokes effects in dev, and loadData isn't naturally
  // idempotent (it appends via functional state updates) — without this
  // guard, two concurrent runs both append a "Local" entry and every pod
  // twice, which also breaks the cluster-filter dropdown's assumption that
  // node ids are unique.
  useEffect(() => {
    let stale = false
    loadData(() => stale)
    return () => {
      stale = true
    }
  }, [])

  const validSummaries = nodeSummaries.map((n) => n.summary).filter((s): s is K8sSummary => s !== null)
  const summary: K8sSummary | null =
    validSummaries.length === 0
      ? null
      : {
          pods: validSummaries.reduce((a, s) => a + s.pods, 0),
          running: validSummaries.reduce((a, s) => a + s.running, 0),
          pending: validSummaries.reduce((a, s) => a + s.pending, 0),
          failed: validSummaries.reduce((a, s) => a + s.failed, 0),
          succeeded: validSummaries.reduce((a, s) => a + s.succeeded, 0),
          namespaces: validSummaries.reduce((a, s) => a + s.namespaces, 0),
          nodes: validSummaries.reduce((a, s) => a + s.nodes, 0),
          deployments: validSummaries.reduce((a, s) => a + s.deployments, 0),
          healthy_deploys: validSummaries.reduce((a, s) => a + s.healthy_deploys, 0),
          unhealthy_deploys: validSummaries.reduce((a, s) => a + s.unhealthy_deploys, 0),
        }

  const reachableNodes = nodeSummaries.filter((n) => n.summary !== null).length

  const [tab, setTab] = useState<Tab>('pods')
  const [search, setSearch] = useState('')
  const [namespace, setNamespace] = useState('all')
  const [clusterFilter, setClusterFilter] = useState('all')
  const [phase, setPhase] = useState<PhaseFilter>('all')

  const namespaceOptions = ['all', ...new Set(rawPods.map((p) => p.namespace))]

  const q = search.trim().toLowerCase()
  const filtered = rawPods.filter((p) => {
    if (clusterFilter !== 'all') {
      const target = clusterFilter === '__local__' ? null : clusterFilter
      if (p.meshNodeId !== target) return false
    }
    if (namespace !== 'all' && p.namespace !== namespace) return false
    if (phase !== 'all' && p.phase !== phase) return false
    if (q) return p.name.toLowerCase().includes(q) || p.namespace.toLowerCase().includes(q) || p.meshNodeName.toLowerCase().includes(q)
    return true
  })

  function openPod(pod: PodRow) {
    selectNode(pod.meshNodeId)
    navigate(`/kubernetes/pods/${pod.namespace}/${pod.name}`)
  }

  // ── Deployments (scale / restart) ─────────────────────────────────────────
  const filteredDeployments = deployments.filter((d) => {
    if (clusterFilter === 'all') return true
    const target = clusterFilter === '__local__' ? null : clusterFilter
    return d.meshNodeId === target
  })

  const [replicaDrafts, setReplicaDrafts] = useState<Record<string, string>>({})
  const [pendingAction, setPendingAction] = useState<string | null>(null)

  function deployKey(d: DeploymentRow): string {
    return `${d.meshNodeId ?? 'local'}/${d.namespace}/${d.name}`
  }

  async function doScale(d: DeploymentRow) {
    const key = deployKey(d)
    const draft = replicaDrafts[key]
    const replicas = draft !== undefined ? Number(draft) : d.replicas
    if (!Number.isInteger(replicas) || replicas < 0) {
      toast('Replica count must be a non-negative whole number', 'danger')
      return
    }
    setPendingAction(key)
    try {
      await scaleDeployment(d.namespace, d.name, replicas, d.meshNodeId)
      setDeployments((prev) => prev.map((x) => (deployKey(x) === key ? { ...x, replicas } : x)))
      toast(`${d.name} scaled to ${replicas}`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Scale failed', 'danger')
    } finally {
      setPendingAction(null)
    }
  }

  async function doRestart(d: DeploymentRow) {
    const key = deployKey(d)
    setPendingAction(key)
    try {
      await restartDeployment(d.namespace, d.name, d.meshNodeId)
      toast(`${d.name} restarting`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Restart failed', 'danger')
    } finally {
      setPendingAction(null)
    }
  }

  // ── Apply YAML ───────────────────────────────────────────────────────────
  const [showApply, setShowApply] = useState(false)
  const [applyNodes, setApplyNodes] = useState<NodeRecord[]>([])
  const [applyNodeId, setApplyNodeId] = useState('')
  const [applyFile, setApplyFile] = useState<File | null>(null)
  const [applying, setApplying] = useState(false)
  const [applyError, setApplyError] = useState('')
  const [applyOutput, setApplyOutput] = useState('')

  function openApply() {
    setShowApply((v) => !v)
    if (applyNodes.length === 0) {
      listNodes()
        .then(setApplyNodes)
        .catch(() => {})
    }
  }

  async function runApply(e: FormEvent) {
    e.preventDefault()
    if (!applyFile) return
    setApplying(true)
    setApplyError('')
    setApplyOutput('')
    try {
      const text = await applyFile.text()
      const res = await applyK8s(text, applyNodeId || undefined)
      setApplyOutput(res.output ?? 'Applied.')
    } catch (e2) {
      setApplyError(e2 instanceof ApiError ? e2.message : 'Apply failed.')
    } finally {
      setApplying(false)
    }
  }

  return (
    <AppShell title="Kubernetes">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">k3s Fleet</p>
            <h2 className={styles.reportTitle}>Cluster overview</h2>
          </div>
          <div className={styles.headerActions}>
            <span className={reachableNodes === nodeSummaries.length ? 'stamp' : 'stamp stamp--alt'}>
              {loading ? 'Loading…' : `${reachableNodes}/${nodeSummaries.length} clusters reachable`}
            </span>
            <button type="button" className={styles.refreshBtn} onClick={() => loadData(() => false)} disabled={loading} aria-label="Refresh">
              <RefreshCw size={14} className={loading ? styles.spin : undefined} />
            </button>
          </div>
        </div>

        <StatGrid minWidth={140}>
          <StatCard label="Pods" value={summary?.pods ?? '—'} icon={<Boxes size={16} />} />
          <StatCard label="Running" value={summary?.running ?? '—'} color="var(--blueprint)" />
          <StatCard label="Pending" value={summary?.pending ?? '—'} color="var(--rust)" />
          <StatCard label="Failed" value={summary?.failed ?? '—'} color="var(--danger)" />
          <StatCard label="Namespaces" value={summary?.namespaces ?? '—'} />
          <StatCard label="K8s nodes" value={summary?.nodes ?? '—'} />
        </StatGrid>

        <Tabs tabs={TABS} active={tab} onChange={(id) => setTab(id as Tab)} />

        {tab === 'pods' && (
          <>
            <Section>
              <div className={styles.toolbar}>
                <input
                  className={styles.toolbarInput}
                  placeholder="Filter by pod or namespace…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                <select className={styles.toolbarSelect} value={namespace} onChange={(e) => setNamespace(e.target.value)}>
                  {namespaceOptions.map((ns) => (
                    <option key={ns} value={ns}>
                      {ns === 'all' ? 'All namespaces' : ns}
                    </option>
                  ))}
                </select>
                <select className={styles.toolbarSelect} value={clusterFilter} onChange={(e) => setClusterFilter(e.target.value)}>
                  <option value="all">All clusters</option>
                  {nodeSummaries.map((n) => (
                    <option key={n.nodeId ?? '__local__'} value={n.nodeId ?? '__local__'}>
                      {n.nodeName}
                    </option>
                  ))}
                </select>
                <div className={styles.filterChips}>
                  {PHASE_FILTERS.map((f) => (
                    <button
                      key={f}
                      type="button"
                      className={phase === f ? `${styles.chip} ${styles.chipActive}` : styles.chip}
                      onClick={() => setPhase(f)}
                    >
                      {f === 'all' ? 'All' : f}
                    </button>
                  ))}
                </div>
                <Button variant="ghost" onClick={openApply}>
                  <Upload size={14} />
                  Apply YAML
                </Button>
              </div>

              {showApply && (
                <form className={styles.applyForm} onSubmit={runApply}>
                  <div className={styles.applyFields}>
                    <Select
                      label="Target node"
                      id="applyNode"
                      value={applyNodeId}
                      onChange={(e) => setApplyNodeId(e.target.value)}
                      options={[
                        { value: '', label: 'Local (this node)' },
                        ...applyNodes.map((n) => ({ value: n.id, label: `${n.name} — ${n.address}` })),
                      ]}
                    />
                    <div className="field">
                      <label htmlFor="applyManifest">Manifest</label>
                      <div className={applyFile ? `${styles.dropzone} ${styles.dropzoneActive}` : styles.dropzone}>
                        <input
                          id="applyManifest"
                          type="file"
                          accept=".yml,.yaml"
                          className={styles.dropzoneInput}
                          onChange={(e) => {
                            setApplyFile(e.target.files?.[0] ?? null)
                            setApplyError('')
                            setApplyOutput('')
                          }}
                        />
                        <div className={styles.dropzoneLabel}>
                          <FileUp size={16} />
                          <span className={styles.dropzoneFileName}>{applyFile?.name ?? 'Choose or drop a file…'}</span>
                          <span className={styles.dropzoneHint}>.yml / .yaml</span>
                        </div>
                      </div>
                    </div>
                  </div>
                  {applyError && <Callout variant="danger">{applyError}</Callout>}
                  {applyOutput && <Callout variant="success">{applyOutput}</Callout>}
                  <div className={styles.applyActions}>
                    <Button type="submit" disabled={!applyFile || applying}>
                      <Upload size={14} />
                      {applying ? 'Applying…' : 'Apply'}
                    </Button>
                  </div>
                </form>
              )}
            </Section>

            <Section>
              {!loading && summary === null ? (
                <EmptyState
                  icon={Boxes}
                  title="Kubernetes cluster is not reachable"
                  description="Ensure your kubeconfig is configured and the API server is accessible."
                />
              ) : filtered.length === 0 ? (
                <EmptyState
                  icon={Boxes}
                  title="No pods match"
                  description="Nothing matches the current filters and search query."
                  action={
                    <Button
                      variant="ghost"
                      onClick={() => {
                        setSearch('')
                        setNamespace('all')
                        setClusterFilter('all')
                        setPhase('all')
                      }}
                    >
                      Clear filters
                    </Button>
                  }
                />
              ) : (
                <Table>
                  <thead>
                    <tr>
                      <th>Cluster</th>
                      <th>Pod</th>
                      <th>Namespace</th>
                      <th>K8s node</th>
                      <th>Phase</th>
                      <th>Ready</th>
                      <th>Restarts</th>
                      <th>Age</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filtered.map((p) => (
                      <tr key={p.uid} onClick={() => openPod(p)} className={styles.row}>
                        <td>
                          <span className={p.meshNodeId ? `${styles.clusterChip} ${styles.clusterChipRemote}` : `${styles.clusterChip} ${styles.clusterChipLocal}`}>
                            {p.meshNodeName}
                          </span>
                        </td>
                        <td className={`cell-name ${styles.nameLink}`}>{p.name}</td>
                        <td className="cell-muted">{p.namespace}</td>
                        <td className="cell-muted">{p.node}</td>
                        <td>
                          <Badge variant={PHASE_VARIANT[p.phase] ?? 'neutral'}>{p.phase}</Badge>
                        </td>
                        <td className="cell-muted">{p.ready}</td>
                        <td className="cell-muted">{p.restarts}</td>
                        <td className="cell-muted">{p.age}</td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
              )}
            </Section>
          </>
        )}

        {tab === 'deployments' && (
          <>
            <StatGrid minWidth={140}>
              <StatCard label="Total" value={summary?.deployments ?? '—'} icon={<Layers size={16} />} />
              <StatCard label="Healthy" value={summary?.healthy_deploys ?? '—'} color="var(--blueprint)" />
              <StatCard label="Degraded" value={summary?.unhealthy_deploys ?? '—'} color="var(--rust)" />
            </StatGrid>
            <Section>
              {!loading && summary === null ? (
                <EmptyState
                  icon={Layers}
                  title="Kubernetes cluster is not reachable"
                  description="Ensure your kubeconfig is configured and the API server is accessible."
                />
              ) : filteredDeployments.length === 0 ? (
                <EmptyState icon={Layers} title="No deployments found" description="Nothing matches the current cluster filter." />
              ) : (
                <Table>
                  <thead>
                    <tr>
                      <th>Cluster</th>
                      <th>Namespace</th>
                      <th>Deployment</th>
                      <th>Ready</th>
                      <th>Replicas</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {filteredDeployments.map((d) => {
                      const key = deployKey(d)
                      const draft = replicaDrafts[key] ?? String(d.replicas)
                      const busy = pendingAction === key
                      return (
                        <tr key={key}>
                          <td>
                            <span className={d.meshNodeId ? `${styles.clusterChip} ${styles.clusterChipRemote}` : `${styles.clusterChip} ${styles.clusterChipLocal}`}>
                              {d.meshNodeName}
                            </span>
                          </td>
                          <td className="cell-muted">{d.namespace}</td>
                          <td className="cell-name">{d.name}</td>
                          <td>
                            <Badge variant={d.ready_replicas >= d.replicas && d.replicas > 0 ? 'success' : 'warning'}>
                              {d.ready_replicas}/{d.replicas}
                            </Badge>
                          </td>
                          <td>
                            <input
                              type="number"
                              min={0}
                              className={styles.replicaInput}
                              value={draft}
                              onChange={(e) => setReplicaDrafts((prev) => ({ ...prev, [key]: e.target.value }))}
                              aria-label={`Replica count for ${d.name}`}
                            />
                          </td>
                          <td>
                            <div className={styles.deployActions}>
                              <Button variant="ghost" onClick={() => doScale(d)} disabled={busy}>
                                {busy ? '…' : 'Scale'}
                              </Button>
                              <Button variant="ghost" onClick={() => doRestart(d)} disabled={busy}>
                                {busy ? '…' : 'Restart'}
                              </Button>
                            </div>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </Table>
              )}
            </Section>
          </>
        )}

        {tab === 'services' && (
          <Section>
            <EmptyState title="Services aren't available yet" description="Use kubectl to inspect Services until this is wired up." />
          </Section>
        )}

        {tab === 'config' && (
          <Section>
            <EmptyState title="Config isn't available yet" description="Use kubectl to inspect ConfigMaps and Secrets until this is wired up." />
          </Section>
        )}
      </div>
    </AppShell>
  )
}

export default Kubernetes
