import { useEffect, useState, type FormEvent } from 'react'
import { ArrowDown, ArrowUp, Globe, KeyRound, Network as NetworkIcon, Plus, Trash2 } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
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
  deleteFederationPeer,
  getHubKey,
  getMeshPeers,
  listFederationPeers,
  listNodes,
  type FederationPeer,
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
      </div>
    </AppShell>
  )
}

export default Network
