import { useEffect, useState, type FormEvent } from 'react'
import { ArrowDown, ArrowUp, Globe, KeyRound, Network as NetworkIcon, Plus, Shield, ShieldAlert, Trash2 } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import Select from '../../components/ui/Select/Select'
import Badge from '../../components/ui/Badge/Badge'
import Callout from '../../components/ui/Callout/Callout'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import TopologyMap from '../../components/TopologyMap/TopologyMap'
import type { MeshNode } from '../../components/TopologyMap/TopologyMap'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import { positionFromMeshIp } from '../../lib/nodePosition'
import { formatBytes } from '../../lib/format'
import {
  ApiError,
  createFederationPeer,
  createNetworkPolicy,
  deleteFederationPeer,
  deleteNetworkPolicy,
  getHubKey,
  getMeshPeers,
  getNetworkPolicySettings,
  listFederationPeers,
  listNodeGroups,
  listNetworkPolicies,
  listNodes,
  setNetworkDefaultDeny,
  type FederationPeer,
  type NetworkPolicy,
  type NetworkPolicySettings,
  type NodeGroup,
  type NodeRecord,
  type PeerStatus,
} from '../../lib/api'
import styles from './Network.module.css'

type Quality = 'good' | 'degraded' | 'dead'

function truncateKey(key: string) {
  return key.length > 12 ? `${key.slice(0, 10)}…` : key
}

function qualityVariant(q: Quality) {
  return q === 'good' ? 'success' : q === 'degraded' ? 'warning' : 'danger'
}

function fmtHandshake(ts: number): string {
  if (!ts) return '—'
  const age = Math.floor(Date.now() / 1000) - ts
  if (age < 60) return `${age}s ago`
  if (age < 3600) return `${Math.floor(age / 60)}m ago`
  return `${Math.floor(age / 3600)}h ago`
}

function Network() {
  const toast = useToast()

  // ── Nodes + live WireGuard peer status ──────────────────────────────────
  const [nodes, setNodes] = useState<NodeRecord[]>([])
  const [wgPeers, setWgPeers] = useState<PeerStatus[]>([])
  const [hubHmacKey, setHubHmacKey] = useState('')
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')

  async function loadNetwork() {
    setLoading(true)
    setApiError('')
    try {
      const [n, p] = await Promise.all([listNodes(), getMeshPeers()])
      setNodes(n)
      setWgPeers(p)
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load network status')
    } finally {
      setLoading(false)
    }
    getHubKey()
      .then((k) => setHubHmacKey(k.hmac_key))
      .catch(() => {})
  }

  useEffect(() => {
    loadNetwork()
  }, [])

  const hubNode = nodes.find((n) => n.type === 'hub' || n.type === 'combined') ?? nodes[0]

  const peerByKey: Record<string, PeerStatus> = {}
  for (const p of wgPeers) peerByKey[p.public_key] = p
  const peerByNodeId: Record<string, PeerStatus> = {}
  for (const p of wgPeers) if (p.node_id) peerByNodeId[p.node_id] = p

  const peers = nodes.map((n) => {
    const live = peerByNodeId[n.id] ?? peerByKey[n.public_key]
    return {
      nodeId: n.id,
      nodeName: n.name,
      endpoint: live?.endpoint || n.address,
      allowedIps: live?.allowed_ips || n.address,
      handshake: live ? fmtHandshake(live.last_handshake) : '—',
      rxBytes: live?.rx_bytes ?? 0,
      txBytes: live?.tx_bytes ?? 0,
      quality: (live?.quality ?? 'good') as Quality,
    }
  })

  const healthy = peers.filter((p) => p.quality === 'good').length
  const rxTotal = peers.reduce((sum, p) => sum + p.rxBytes, 0)
  const txTotal = peers.reduce((sum, p) => sum + p.txBytes, 0)

  const graphNodes: MeshNode[] = nodes.map((n) => {
    const { lat, lng } = positionFromMeshIp(n.address)
    const peer = peers.find((p) => p.nodeId === n.id)
    return {
      id: n.id,
      name: n.name,
      role: (n.type as MeshNode['role']) ?? 'remote',
      status: peer?.quality === 'good' ? 'healthy' : peer?.quality === 'degraded' ? 'degraded' : 'offline',
      lat,
      lng,
    }
  })

  // ── Federation ───────────────────────────────────────────────────────────
  const [federation, setFederation] = useState<FederationPeer[]>([])
  const [fedLoading, setFedLoading] = useState(true)
  const [fedError, setFedError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [hubUrl, setHubUrl] = useState('')
  const [wgEndpoint, setWgEndpoint] = useState('')
  const [wgPublicKey, setWgPublicKey] = useState('')
  const [meshCidr, setMeshCidr] = useState('')
  const [allowedIps, setAllowedIps] = useState('')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  async function loadFederation() {
    setFedLoading(true)
    setFedError('')
    try {
      setFederation(await listFederationPeers())
    } catch (e) {
      setFedError(e instanceof ApiError ? e.message : 'Failed to load federation peers')
    } finally {
      setFedLoading(false)
    }
  }

  useEffect(() => {
    loadFederation()
  }, [])

  function resetForm() {
    setName('')
    setHubUrl('')
    setWgEndpoint('')
    setWgPublicKey('')
    setMeshCidr('')
    setAllowedIps('')
    setFormError('')
  }

  async function removeFederationPeer(peer: FederationPeer) {
    try {
      await deleteFederationPeer(peer.id)
      await loadFederation()
      toast(`Unlinked hub ${peer.name}`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Failed to unlink hub', 'danger')
    }
  }

  async function submitFederationPeer(e: FormEvent) {
    e.preventDefault()
    if (!name || !wgEndpoint || !wgPublicKey || !meshCidr) {
      setFormError('Name, WireGuard endpoint, public key, and mesh CIDR are required.')
      return
    }
    setSaving(true)
    setFormError('')
    try {
      await createFederationPeer({
        name,
        hub_url: hubUrl || undefined,
        wg_endpoint: wgEndpoint,
        wg_public_key: wgPublicKey,
        mesh_cidr: meshCidr,
        allowed_ips: allowedIps || undefined,
      })
      await loadFederation()
      toast(`Federation peer ${name} linked`)
      resetForm()
      setShowForm(false)
    } catch (e) {
      setFormError(e instanceof ApiError ? e.message : 'Failed to add federation peer')
    } finally {
      setSaving(false)
    }
  }

  // ── Network policies ─────────────────────────────────────────────────────
  const [policies, setPolicies] = useState<NetworkPolicy[]>([])
  const [policiesLoading, setPoliciesLoading] = useState(true)
  const [policiesError, setPoliciesError] = useState('')
  const [policySettings, setPolicySettings] = useState<NetworkPolicySettings | null>(null)
  const [groups, setGroups] = useState<NodeGroup[]>([])
  const [busyPolicyId, setBusyPolicyId] = useState<string | null>(null)
  const [defaultDenyBusy, setDefaultDenyBusy] = useState(false)

  const [showPolicyForm, setShowPolicyForm] = useState(false)
  const [policyName, setPolicyName] = useState('')
  const [policyDescription, setPolicyDescription] = useState('')
  const [policyPriority, setPolicyPriority] = useState('100')
  const [policyAction, setPolicyAction] = useState<'allow' | 'deny'>('deny')
  const [policyProtocol, setPolicyProtocol] = useState<'any' | 'tcp' | 'udp' | 'icmp'>('any')
  const [policyPort, setPolicyPort] = useState('')
  const [srcType, setSrcType] = useState<'any' | 'node' | 'group'>('any')
  const [srcId, setSrcId] = useState('')
  const [dstType, setDstType] = useState<'any' | 'node' | 'group'>('any')
  const [dstId, setDstId] = useState('')
  const [policyFormError, setPolicyFormError] = useState('')
  const [policySaving, setPolicySaving] = useState(false)

  async function loadPolicies() {
    setPoliciesLoading(true)
    setPoliciesError('')
    try {
      const [p, s, g] = await Promise.all([listNetworkPolicies(), getNetworkPolicySettings(), listNodeGroups()])
      setPolicies(p)
      setPolicySettings(s)
      setGroups(g)
    } catch (e) {
      setPoliciesError(e instanceof ApiError ? e.message : 'Failed to load network policies')
    } finally {
      setPoliciesLoading(false)
    }
  }

  useEffect(() => {
    loadPolicies()
  }, [])

  function targetLabel(type: string, id: string): string {
    if (type !== 'node' && type !== 'group') return 'any'
    if (type === 'node') return nodes.find((n) => n.id === id)?.name ?? `node:${id.slice(0, 8)}`
    return groups.find((g) => g.id === id)?.name ?? `group:${id.slice(0, 8)}`
  }

  function resetPolicyForm() {
    setPolicyName('')
    setPolicyDescription('')
    setPolicyPriority('100')
    setPolicyAction('deny')
    setPolicyProtocol('any')
    setPolicyPort('')
    setSrcType('any')
    setSrcId('')
    setDstType('any')
    setDstId('')
    setPolicyFormError('')
  }

  async function submitPolicy(e: FormEvent) {
    e.preventDefault()
    if (!policyName) {
      setPolicyFormError('Name is required.')
      return
    }
    if (srcType !== 'any' && !srcId) {
      setPolicyFormError('Select a source node/group, or set source to "any".')
      return
    }
    if (dstType !== 'any' && !dstId) {
      setPolicyFormError('Select a destination node/group, or set destination to "any".')
      return
    }
    setPolicyFormError('')
    setPolicySaving(true)
    try {
      await createNetworkPolicy({
        name: policyName,
        description: policyDescription,
        enabled: true,
        priority: Number(policyPriority) || 100,
        action: policyAction,
        protocol: policyProtocol,
        port: policyProtocol === 'icmp' ? '' : policyPort,
        src_type: srcType,
        src_id: srcType === 'any' ? '' : srcId,
        dst_type: dstType,
        dst_id: dstType === 'any' ? '' : dstId,
      })
      toast(`Policy "${policyName}" created`)
      resetPolicyForm()
      setShowPolicyForm(false)
      loadPolicies()
    } catch (e2) {
      setPolicyFormError(e2 instanceof ApiError ? e2.message : 'Failed to create policy')
    } finally {
      setPolicySaving(false)
    }
  }

  async function removePolicy(p: NetworkPolicy) {
    setBusyPolicyId(p.id)
    try {
      await deleteNetworkPolicy(p.id)
      setPolicies((prev) => prev.filter((x) => x.id !== p.id))
      toast(`Policy "${p.name}" deleted`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    } finally {
      setBusyPolicyId(null)
    }
  }

  async function toggleDefaultDeny() {
    if (!policySettings) return
    const next = !policySettings.default_deny
    setDefaultDenyBusy(true)
    try {
      await setNetworkDefaultDeny(next)
      setPolicySettings({ ...policySettings, default_deny: next })
      toast(next ? 'Default-deny enabled — unmatched mesh traffic is now dropped' : 'Default-deny disabled')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Failed to change default-deny', 'danger')
    } finally {
      setDefaultDenyBusy(false)
    }
  }

  const nodeOptions = [{ value: '', label: 'Select a node…' }, ...nodes.map((n) => ({ value: n.id, label: n.name }))]
  const groupOptions = [{ value: '', label: 'Select a group…' }, ...groups.map((g) => ({ value: g.id, label: g.name }))]

  return (
    <AppShell title="Network">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">WireGuard Mesh</p>
            <h2 className={styles.reportTitle}>Link status</h2>
          </div>
          {peers.length === 0 || healthy === peers.length ? (
            <span className="stamp">All peers healthy</span>
          ) : (
            <span className="stamp stamp--alt">{peers.length - healthy} degraded</span>
          )}
        </div>

        <StatGrid>
          <StatCard label="Peer health" value={`${healthy}/${peers.length}`} sub="WireGuard peers healthy" icon={<NetworkIcon size={16} />} />
          <StatCard label="Received" value={formatBytes(rxTotal)} sub="across all peers" icon={<ArrowDown size={16} />} />
          <StatCard label="Sent" value={formatBytes(txTotal)} sub="across all peers" icon={<ArrowUp size={16} />} />
          <StatCard label="Federated hubs" value={federation.length} sub="independent mesh(es) linked" icon={<KeyRound size={16} />} />
        </StatGrid>

        <Section title="Mesh topology" frame>
          <TopologyMap nodes={graphNodes} />
        </Section>

        <Section title="This hub's identity">
          <div className={styles.identityGrid}>
            <div>
              <span className={styles.identityLabel}>WireGuard public key</span>
              <span className={styles.identityVal} title={hubNode?.public_key || '—'}>
                <KeyRound size={13} />
                {hubNode?.public_key ? truncateKey(hubNode.public_key) : '—'}
              </span>
            </div>
            <div>
              <span className={styles.identityLabel}>Hub proxy key (HMAC)</span>
              <span className={styles.identityVal} title={hubHmacKey || '—'}>
                <KeyRound size={13} />
                {hubHmacKey ? truncateKey(hubHmacKey) : '—'}
              </span>
            </div>
          </div>
        </Section>

        <Section title="WireGuard peers" action={<span className={styles.manifestNo}>Manifest No. {String(peers.length).padStart(3, '0')}</span>}>
          {apiError ? (
            <Callout variant="danger">{apiError}</Callout>
          ) : loading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : peers.length === 0 ? (
            <EmptyState icon={NetworkIcon} title="No nodes enrolled yet" description="Enroll a node from the Nodes page to see it here." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>#</th>
                  <th>Node</th>
                  <th>Endpoint</th>
                  <th>Allowed IPs</th>
                  <th>Handshake</th>
                  <th>RX</th>
                  <th>TX</th>
                  <th>Quality</th>
                </tr>
              </thead>
              <tbody>
                {peers.map((p, i) => (
                  <tr key={p.nodeId}>
                    <td className="cell-muted">{String(i + 1).padStart(3, '0')}</td>
                    <td className="cell-name">{p.nodeName}</td>
                    <td className="cell-muted">{p.endpoint}</td>
                    <td className="cell-muted">{p.allowedIps}</td>
                    <td className="cell-muted">{p.handshake}</td>
                    <td className="cell-muted">{formatBytes(p.rxBytes)}</td>
                    <td className="cell-muted">{formatBytes(p.txBytes)}</td>
                    <td>
                      <Badge variant={qualityVariant(p.quality)}>{p.quality}</Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Section>

        <Section
          title="Hub federation"
          action={
            <Button variant="ghost" onClick={() => setShowForm((v) => !v)}>
              <Plus size={14} />
              Link hub
            </Button>
          }
        >
          <div className={styles.calloutWrap}>
            <Callout variant="info">
              Connect an independent Scutum instance so nodes in each mesh can route to nodes in
              the other. Federation adds a WireGuard peer between the two hub interfaces — repeat
              this symmetrically on the other hub.
            </Callout>
          </div>

          {showForm && (
            <form className={styles.form} onSubmit={submitFederationPeer}>
              <TextField label="Name" id="fedName" value={name} onChange={(e) => setName(e.target.value)} placeholder="hub-b" />
              <TextField
                label="Hub API URL (optional)"
                id="fedHubUrl"
                value={hubUrl}
                onChange={(e) => setHubUrl(e.target.value)}
                placeholder="https://hub-b.example.com"
              />
              <TextField
                label="WireGuard endpoint"
                id="fedEndpoint"
                value={wgEndpoint}
                onChange={(e) => setWgEndpoint(e.target.value)}
                placeholder="203.0.113.10:51820"
              />
              <TextField
                label="WireGuard public key"
                id="fedPublicKey"
                value={wgPublicKey}
                onChange={(e) => setWgPublicKey(e.target.value)}
                placeholder="Base64-encoded public key"
              />
              <TextField
                label="Mesh CIDR"
                id="fedMeshCidr"
                value={meshCidr}
                onChange={(e) => setMeshCidr(e.target.value)}
                placeholder="10.200.0.0/24"
              />
              <TextField
                label="Allowed IPs (optional)"
                id="fedAllowedIps"
                value={allowedIps}
                onChange={(e) => setAllowedIps(e.target.value)}
                placeholder="defaults to mesh CIDR"
              />
              {formError && <Callout variant="danger">{formError}</Callout>}
              <Button type="submit" disabled={saving}>
                {saving ? 'Adding…' : 'Add federation peer'}
              </Button>
            </form>
          )}

          {fedError ? (
            <Callout variant="danger">{fedError}</Callout>
          ) : fedLoading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : federation.length === 0 ? (
            <EmptyState
              icon={Globe}
              title="No federated hubs linked"
              description="Link an independent Scutum hub above so nodes in each mesh can route to each other."
            />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>WireGuard endpoint</th>
                  <th>Mesh CIDR</th>
                  <th>Status</th>
                  <th>Last seen</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {federation.map((p) => (
                  <tr key={p.id}>
                    <td className="cell-name">{p.name}</td>
                    <td className="cell-muted">{p.wg_endpoint}</td>
                    <td className="cell-muted">{p.mesh_cidr}</td>
                    <td>
                      <Badge variant={p.status === 'connected' ? 'success' : p.status === 'error' ? 'danger' : 'warning'}>{p.status}</Badge>
                    </td>
                    <td className="cell-muted">{p.last_seen ? new Date(p.last_seen).toLocaleString() : '—'}</td>
                    <td>
                      <button
                        type="button"
                        className={styles.removeBtn}
                        onClick={() => removeFederationPeer(p)}
                        aria-label={`Unlink ${p.name}`}
                      >
                        <Trash2 size={15} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Section>

        <Section
          title="Network policies"
          action={
            <Button variant="ghost" onClick={() => setShowPolicyForm((v) => !v)}>
              <Plus size={14} />
              New policy
            </Button>
          }
        >
          <div className={styles.calloutWrap}>
            <Callout variant="info">
              Rules apply only to traffic entering and leaving the WireGuard interface — never to SSH, the API
              itself, or anything else on the host, so a bad policy can't lock you out. Rules run in priority order
              (lowest first); the first match wins.
            </Callout>
          </div>

          {policySettings && (
            <div className={styles.calloutWrap}>
              <Callout variant={policySettings.default_deny ? 'warning' : 'info'}>
                <span>
                  Default-deny is <strong>{policySettings.default_deny ? 'ON' : 'off'}</strong> — unmatched mesh
                  traffic is currently {policySettings.default_deny ? 'dropped' : 'allowed'}.
                  {!policySettings.iptables_available && ' iptables is not available on this hub; policies are stored but not enforced.'}
                </span>{' '}
                <Button variant="ghost" onClick={toggleDefaultDeny} disabled={defaultDenyBusy}>
                  {policySettings.default_deny ? <Shield size={14} /> : <ShieldAlert size={14} />}
                  {policySettings.default_deny ? 'Disable default-deny' : 'Enable default-deny'}
                </Button>
              </Callout>
            </div>
          )}

          {showPolicyForm && (
            <form className={styles.form} onSubmit={submitPolicy}>
              <TextField label="Name" id="policyName" value={policyName} onChange={(e) => setPolicyName(e.target.value)} placeholder="allow ssh from prod" />
              <TextField
                label="Description (optional)"
                id="policyDescription"
                value={policyDescription}
                onChange={(e) => setPolicyDescription(e.target.value)}
              />
              <TextField label="Priority (lower runs first)" id="policyPriority" type="number" value={policyPriority} onChange={(e) => setPolicyPriority(e.target.value)} />
              <Select
                label="Action"
                id="policyAction"
                value={policyAction}
                onChange={(e) => setPolicyAction(e.target.value as 'allow' | 'deny')}
                options={[{ value: 'allow', label: 'Allow' }, { value: 'deny', label: 'Deny' }]}
              />
              <Select
                label="Protocol"
                id="policyProtocol"
                value={policyProtocol}
                onChange={(e) => setPolicyProtocol(e.target.value as 'any' | 'tcp' | 'udp' | 'icmp')}
                options={[
                  { value: 'any', label: 'Any' },
                  { value: 'tcp', label: 'TCP' },
                  { value: 'udp', label: 'UDP' },
                  { value: 'icmp', label: 'ICMP' },
                ]}
              />
              {policyProtocol !== 'icmp' && policyProtocol !== 'any' && (
                <TextField label="Port (optional, e.g. 22 or 1000-2000)" id="policyPort" value={policyPort} onChange={(e) => setPolicyPort(e.target.value)} />
              )}
              <Select
                label="Source"
                id="policySrcType"
                value={srcType}
                onChange={(e) => {
                  setSrcType(e.target.value as 'any' | 'node' | 'group')
                  setSrcId('')
                }}
                options={[{ value: 'any', label: 'Any' }, { value: 'node', label: 'Node' }, { value: 'group', label: 'Group' }]}
              />
              {srcType !== 'any' && (
                <Select label="Source target" id="policySrcId" value={srcId} onChange={(e) => setSrcId(e.target.value)} options={srcType === 'node' ? nodeOptions : groupOptions} />
              )}
              <Select
                label="Destination"
                id="policyDstType"
                value={dstType}
                onChange={(e) => {
                  setDstType(e.target.value as 'any' | 'node' | 'group')
                  setDstId('')
                }}
                options={[{ value: 'any', label: 'Any' }, { value: 'node', label: 'Node' }, { value: 'group', label: 'Group' }]}
              />
              {dstType !== 'any' && (
                <Select label="Destination target" id="policyDstId" value={dstId} onChange={(e) => setDstId(e.target.value)} options={dstType === 'node' ? nodeOptions : groupOptions} />
              )}
              {policyFormError && <Callout variant="danger">{policyFormError}</Callout>}
              <Button type="submit" disabled={policySaving}>
                {policySaving ? 'Creating…' : 'Create policy'}
              </Button>
            </form>
          )}

          {policiesError ? (
            <Callout variant="danger">{policiesError}</Callout>
          ) : policiesLoading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : policies.length === 0 ? (
            <EmptyState icon={Shield} title="No network policies" description="Create a policy to allow or deny traffic between nodes or groups on the mesh." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Pri</th>
                  <th>Name</th>
                  <th>Source</th>
                  <th>Destination</th>
                  <th>Proto/Port</th>
                  <th>Action</th>
                  <th>Enabled</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {policies.map((p) => {
                  const busy = busyPolicyId === p.id
                  return (
                    <tr key={p.id}>
                      <td className="cell-muted">{p.priority}</td>
                      <td className="cell-name">{p.name}</td>
                      <td className="cell-muted">{targetLabel(p.src_type, p.src_id)}</td>
                      <td className="cell-muted">{targetLabel(p.dst_type, p.dst_id)}</td>
                      <td className="cell-muted">
                        {p.protocol}
                        {p.port ? `/${p.port}` : ''}
                      </td>
                      <td>
                        <Badge variant={p.action === 'allow' ? 'success' : 'danger'}>{p.action}</Badge>
                      </td>
                      <td>
                        <Badge variant={p.enabled ? 'success' : 'neutral'}>{p.enabled ? 'on' : 'off'}</Badge>
                      </td>
                      <td>
                        <button
                          type="button"
                          className={styles.removeBtn}
                          onClick={() => removePolicy(p)}
                          disabled={busy}
                          aria-label={`Delete ${p.name}`}
                        >
                          <Trash2 size={15} />
                        </button>
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

export default Network
