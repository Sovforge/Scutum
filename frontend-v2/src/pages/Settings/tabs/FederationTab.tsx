import { useState, type FormEvent } from 'react'
import { Network, Plus, Trash2 } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import styles from '../Settings.module.css'

// This is the REAL federation implementation (GET/POST/DELETE
// /federation/peers(/:id) — no update endpoint, matched here with no Edit
// action) — admin-only, same as SCIM. The Network page's own "Hub
// federation" section is a simpler addition from this rebuild, not
// something the old app has; hub-berlin's endpoint/public key/mesh CIDR
// here match that page's mock exactly since they're the same peer viewed
// from a fuller admin surface.
type FederationStatus = 'connected' | 'pending' | 'error'
type FederationPeer = {
  id: string
  name: string
  wgEndpoint: string
  wgPublicKey: string
  meshCidr: string
  allowedIps: string
  hubUrl: string
  status: FederationStatus
  lastSeen: string
}

const STATUS_VARIANT: Record<FederationStatus, 'success' | 'warning' | 'danger'> = {
  connected: 'success',
  pending: 'warning',
  error: 'danger',
}

const INITIAL_PEERS: FederationPeer[] = [
  {
    id: 'hub-berlin',
    name: 'hub-berlin',
    wgEndpoint: '203.0.113.10:51820',
    wgPublicKey: 'RtY6WsD4FgJ1AeB0CHZ8mQvX2N9Lp3K7=',
    meshCidr: '10.200.0.0/24',
    allowedIps: '10.200.0.0/24',
    hubUrl: 'https://hub-berlin.example.com',
    status: 'connected',
    lastSeen: '2026-07-09 09:10:00',
  },
]

function truncateKey(key: string) {
  return `${key.slice(0, 10)}…`
}

function FederationTab() {
  const toast = useToast()
  const [peers, setPeers] = useState(INITIAL_PEERS)
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [wgEndpoint, setWgEndpoint] = useState('')
  const [wgPublicKey, setWgPublicKey] = useState('')
  const [meshCidr, setMeshCidr] = useState('')
  const [allowedIps, setAllowedIps] = useState('')
  const [hubUrl, setHubUrl] = useState('')
  const [formError, setFormError] = useState('')

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!name || !wgEndpoint || !wgPublicKey || !meshCidr) {
      setFormError('Name, WireGuard endpoint, public key, and mesh CIDR are required.')
      return
    }
    setFormError('')
    setPeers((prev) => [
      ...prev,
      { id: name, name, wgEndpoint, wgPublicKey, meshCidr, allowedIps: allowedIps || meshCidr, hubUrl, status: 'pending', lastSeen: 'never' },
    ])
    toast(`${name} added — pending first handshake`)
    setName('')
    setWgEndpoint('')
    setWgPublicKey('')
    setMeshCidr('')
    setAllowedIps('')
    setHubUrl('')
    setShowForm(false)
  }

  function remove(p: FederationPeer) {
    setPeers((prev) => prev.filter((x) => x.id !== p.id))
    toast(`${p.name} unfederated`, 'danger')
  }

  return (
    <>
      <Section title="Federated hubs" action={<Button variant="ghost" onClick={() => setShowForm((v) => !v)}><Plus size={14} />Add peer</Button>}>
        {showForm && (
          <form className={styles.inlineForm} onSubmit={submit}>
            <div className={styles.formFields}>
              <TextField label="Name" id="fedName" value={name} onChange={(e) => setName(e.target.value)} placeholder="hub-tokyo" />
              <TextField label="WireGuard endpoint" id="fedEndpoint" value={wgEndpoint} onChange={(e) => setWgEndpoint(e.target.value)} placeholder="203.0.113.20:51820" />
              <TextField label="WireGuard public key" id="fedPubKey" value={wgPublicKey} onChange={(e) => setWgPublicKey(e.target.value)} placeholder="Base64-encoded public key" />
              <TextField label="Mesh CIDR" id="fedCidr" value={meshCidr} onChange={(e) => setMeshCidr(e.target.value)} placeholder="10.201.0.0/24" />
              <TextField label="Allowed IPs (optional)" id="fedAllowed" value={allowedIps} onChange={(e) => setAllowedIps(e.target.value)} placeholder="Defaults to mesh CIDR" />
              <TextField label="Hub URL (optional)" id="fedHubUrl" value={hubUrl} onChange={(e) => setHubUrl(e.target.value)} placeholder="https://hub-tokyo.example.com" />
            </div>
            {formError && <p className={styles.formError}>{formError}</p>}
            <div className={styles.formActions}>
              <Button type="submit">Add peer</Button>
            </div>
          </form>
        )}

        {peers.length === 0 ? (
          <EmptyState icon={Network} title="No federated hubs" description="Link an independent Scutum instance so nodes in each mesh can route to each other." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Endpoint</th>
                <th>Public key</th>
                <th>Mesh CIDR</th>
                <th>Status</th>
                <th>Last seen</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {peers.map((p) => (
                <tr key={p.id}>
                  <td className="cell-name">{p.name}</td>
                  <td className="cell-muted">{p.wgEndpoint}</td>
                  <td className="cell-muted" title={p.wgPublicKey}>
                    {truncateKey(p.wgPublicKey)}
                  </td>
                  <td className="cell-muted">{p.meshCidr}</td>
                  <td>
                    <Badge variant={STATUS_VARIANT[p.status]}>{p.status}</Badge>
                  </td>
                  <td className="cell-muted">{p.lastSeen}</td>
                  <td>
                    <div className={styles.rowActions}>
                      <button type="button" className={styles.rowActionDanger} onClick={() => remove(p)} aria-label={`Unfederate ${p.name}`}>
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Section>
    </>
  )
}

export default FederationTab
