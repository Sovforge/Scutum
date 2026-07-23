import { useState, type FormEvent } from 'react'
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
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import { NODES } from '../../mockData'
import { formatBytes } from '../../lib/format'
import styles from './Network.module.css'

type Quality = 'good' | 'degraded' | 'dead'

// Mock data shaped like the real endpoints (see README.md § API Quick
// Reference: GET /network/peers, GET /network/mesh-summary, GET
// /network/hub-key, POST/DELETE /federation/peers) — not wired to a live
// backend yet.
type Peer = {
  nodeId: string
  nodeName: string
  endpoint: string
  allowedIps: string
  handshake: string
  rxBytes: number
  txBytes: number
  quality: Quality
}

const PEERS: Peer[] = [
  {
    nodeId: 'edge-london',
    nodeName: 'edge-london',
    endpoint: '203.0.113.22:51820',
    allowedIps: '10.100.0.4/32',
    handshake: '18s ago',
    rxBytes: 883_200_000,
    txBytes: 1_290_000_000,
    quality: 'good',
  },
  {
    nodeId: 'edge-nyc',
    nodeName: 'edge-nyc',
    endpoint: '198.51.100.9:51820',
    allowedIps: '10.100.0.7/32',
    handshake: '3m ago',
    rxBytes: 220_000_000,
    txBytes: 356_000_000,
    quality: 'degraded',
  },
  {
    nodeId: 'build-runner',
    nodeName: 'build-runner',
    endpoint: '203.0.113.40:51820',
    allowedIps: '10.100.0.9/32',
    handshake: '9s ago',
    rxBytes: 1_150_000_000,
    txBytes: 2_510_000_000,
    quality: 'good',
  },
]

type FederationPeer = {
  id: string
  name: string
  endpoint: string
  publicKey: string
  meshCidr: string
}

const INITIAL_FEDERATION: FederationPeer[] = [
  {
    id: 'hub-berlin',
    name: 'hub-berlin',
    endpoint: '203.0.113.10:51820',
    publicKey: 'RtY6WsD4FgJ1AeB0CHZ8mQvX2N9Lp3K7=',
    meshCidr: '10.200.0.0/24',
  },
]

const HUB_IDENTITY = {
  publicKey: 'HZ8mQvX2N9Lp3K7RtY6WsD4FgJ1AeB0C=',
  hmacKey: 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4',
}

function truncateKey(key: string) {
  return `${key.slice(0, 10)}…`
}

function qualityVariant(q: Quality) {
  return q === 'good' ? 'success' : q === 'degraded' ? 'warning' : 'danger'
}

function Network() {
  const toast = useToast()
  const [federation, setFederation] = useState(INITIAL_FEDERATION)
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [endpoint, setEndpoint] = useState('')
  const [publicKey, setPublicKey] = useState('')
  const [meshCidr, setMeshCidr] = useState('')
  const [formError, setFormError] = useState('')

  const healthy = PEERS.filter((p) => p.quality === 'good').length
  const rxTotal = PEERS.reduce((sum, p) => sum + p.rxBytes, 0)
  const txTotal = PEERS.reduce((sum, p) => sum + p.txBytes, 0)

  function removeFederationPeer(id: string) {
    setFederation((f) => f.filter((p) => p.id !== id))
    toast(`Unlinked hub ${id}`, 'danger')
  }

  function submitFederationPeer(e: FormEvent) {
    e.preventDefault()
    if (!name || !endpoint || !publicKey || !meshCidr) {
      setFormError('All fields are required.')
      return
    }
    setFormError('')
    setFederation((f) => [...f, { id: name, name, endpoint, publicKey, meshCidr }])
    toast(`Federation peer ${name} linked`)
    setName('')
    setEndpoint('')
    setPublicKey('')
    setMeshCidr('')
    setShowForm(false)
  }

  return (
    <AppShell title="Network">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">WireGuard Mesh</p>
            <h2 className={styles.reportTitle}>Link status</h2>
          </div>
          {healthy === PEERS.length ? (
            <span className="stamp">All peers healthy</span>
          ) : (
            <span className="stamp stamp--alt">{PEERS.length - healthy} degraded</span>
          )}
        </div>

        <StatGrid>
          <StatCard label="Peer health" value={`${healthy}/${PEERS.length}`} sub="WireGuard peers healthy" icon={<NetworkIcon size={16} />} />
          <StatCard label="Received" value={formatBytes(rxTotal)} sub="across all peers" icon={<ArrowDown size={16} />} />
          <StatCard label="Sent" value={formatBytes(txTotal)} sub="across all peers" icon={<ArrowUp size={16} />} />
          <StatCard label="Federated hubs" value={federation.length} sub="independent mesh(es) linked" icon={<KeyRound size={16} />} />
        </StatGrid>

        <Section title="Mesh topology" frame>
          <TopologyMap nodes={NODES} />
        </Section>

        <Section title="This hub's identity">
          <div className={styles.identityGrid}>
            <div>
              <span className={styles.identityLabel}>WireGuard public key</span>
              <span className={styles.identityVal} title={HUB_IDENTITY.publicKey}>
                <KeyRound size={13} />
                {truncateKey(HUB_IDENTITY.publicKey)}
              </span>
            </div>
            <div>
              <span className={styles.identityLabel}>Hub proxy key (HMAC)</span>
              <span className={styles.identityVal} title={HUB_IDENTITY.hmacKey}>
                <KeyRound size={13} />
                {truncateKey(HUB_IDENTITY.hmacKey)}
              </span>
            </div>
          </div>
        </Section>

        <Section title="WireGuard peers" action={<span className={styles.manifestNo}>Manifest No. {String(PEERS.length).padStart(3, '0')}</span>}>
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
              {PEERS.map((p, i) => (
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
                label="WireGuard endpoint"
                id="fedEndpoint"
                value={endpoint}
                onChange={(e) => setEndpoint(e.target.value)}
                placeholder="203.0.113.10:51820"
              />
              <TextField
                label="WireGuard public key"
                id="fedPublicKey"
                value={publicKey}
                onChange={(e) => setPublicKey(e.target.value)}
                placeholder="Base64-encoded public key"
              />
              <TextField
                label="Mesh CIDR"
                id="fedMeshCidr"
                value={meshCidr}
                onChange={(e) => setMeshCidr(e.target.value)}
                placeholder="10.200.0.0/24"
              />
              {formError && <Callout variant="danger">{formError}</Callout>}
              <Button type="submit">Add federation peer</Button>
            </form>
          )}

          {federation.length === 0 ? (
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
                  <th>Endpoint</th>
                  <th>Mesh CIDR</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {federation.map((p) => (
                  <tr key={p.id}>
                    <td className="cell-name">{p.name}</td>
                    <td className="cell-muted">{p.endpoint}</td>
                    <td className="cell-muted">{p.meshCidr}</td>
                    <td>
                      <button
                        type="button"
                        className={styles.removeBtn}
                        onClick={() => removeFederationPeer(p.id)}
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
