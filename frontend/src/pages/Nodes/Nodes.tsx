import { useEffect, useState, type FormEvent } from 'react'
import { Check, ChevronDown, ChevronRight, Copy, FileUp, Layers, Plus, Search, Send, Server, Trash2, UserPlus, X } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import Select from '../../components/ui/Select/Select'
import Badge from '../../components/ui/Badge/Badge'
import Callout from '../../components/ui/Callout/Callout'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import {
  ApiError,
  addNodeToGroup,
  addPeer,
  approveNode,
  createNode,
  createNodeGroup,
  deleteNode,
  deleteNodeGroup,
  distributeFileToGroup,
  distributeFileToNode,
  getGroupNodes,
  getHubKey,
  listFileTransfers,
  listNodeGroups,
  listNodes,
  rejectNode,
  removeNodeFromGroup,
  type FileTransfer,
  type NodeGroup,
  type NodeRecord,
} from '../../lib/api'
import styles from './Nodes.module.css'

const ROLE_OPTIONS = [
  { value: 'remote', label: 'Remote' },
  { value: 'hub', label: 'Hub' },
] as const

function roleVariant(role: string) {
  return role === 'hub' ? 'success' : role === 'combined' ? 'warning' : 'neutral'
}

function truncateKey(key: string) {
  return key.length > 12 ? `${key.slice(0, 10)}…` : key
}

// Turns "10.102.132.2/32" into "10.102.132.2" — only single-host masks are a
// usable API address; wider CIDRs don't identify one node.
function meshIPFromAllowedIPs(allowedIPs: string): string {
  const first = (allowedIPs.split(',')[0] ?? '').trim()
  const slash = first.indexOf('/')
  if (slash === -1) return first
  const mask = first.slice(slash + 1)
  if (mask !== '32' && mask !== '128') return ''
  return first.slice(0, slash)
}

function Nodes() {
  const toast = useToast()

  // ── Nodes ────────────────────────────────────────────────────────────────
  const [nodes, setNodes] = useState<NodeRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [search, setSearch] = useState('')
  const [activeFilter, setActiveFilter] = useState<'all' | 'hub' | 'remote'>('all')

  async function loadNodes() {
    setLoading(true)
    setApiError('')
    try {
      setNodes(await listNodes())
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load nodes')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadNodes()
  }, [])

  const filtered = nodes.filter((n) => {
    const matchesFilter = activeFilter === 'all' || n.type === activeFilter
    const q = search.toLowerCase()
    const matchesSearch = !q || n.name.toLowerCase().includes(q) || n.address.includes(q)
    return matchesFilter && matchesSearch
  })

  const stats = [
    { label: 'Total', value: nodes.length },
    { label: 'Hub', value: nodes.filter((n) => n.type === 'hub').length },
    { label: 'Remote', value: nodes.filter((n) => n.type === 'remote').length },
  ]

  async function removeNode(node: NodeRecord) {
    try {
      await deleteNode(node.id)
      await loadNodes()
      toast(`${node.name} removed from the mesh`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    }
  }

  // Only relevant when Settings → Nodes has "require approval" on — nodes
  // enrolled while it's off go straight to "approved" and never show this.
  async function approvePending(node: NodeRecord) {
    try {
      await approveNode(node.id)
      await loadNodes()
      toast(`${node.name} approved`)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Approve failed', 'danger')
    }
  }

  async function rejectPending(node: NodeRecord) {
    try {
      await rejectNode(node.id)
      await loadNodes()
      toast(`${node.name} rejected`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Reject failed', 'danger')
    }
  }

  // ── Manual enrollment ──────────────────────────────────────────────────
  const [showEnroll, setShowEnroll] = useState(false)
  const [hubHMACKey, setHubHMACKey] = useState('')
  const [copied, setCopied] = useState<string | null>(null)
  const [enrollName, setEnrollName] = useState('')
  const [enrollPubkey, setEnrollPubkey] = useState('')
  const [enrollEndpoint, setEnrollEndpoint] = useState('')
  const [enrollAllowedIPs, setEnrollAllowedIPs] = useState('')
  const [enrollApiAddress, setEnrollApiAddress] = useState('')
  const [enrollRole, setEnrollRole] = useState<'remote' | 'hub'>('remote')
  const [enrollError, setEnrollError] = useState('')
  const [enrollSaving, setEnrollSaving] = useState(false)

  // The hub's own identity comes straight from the node it registered for
  // itself at setup time — more reliable than caching it client-side, since
  // it works from any browser/admin session, not just the one that ran setup.
  const hubNode = nodes.find((n) => n.type === 'hub' || n.type === 'combined')
  const localPubkey = hubNode?.public_key ?? '(no hub node found)'
  const localEndpoint = hubNode?.address ?? '—'

  function openEnroll() {
    setShowEnroll((v) => !v)
    if (!hubHMACKey) {
      getHubKey()
        .then((res) => setHubHMACKey(res.hmac_key))
        .catch(() => {})
    }
  }

  async function copyKey(key: string, slot = 'local') {
    try {
      await navigator.clipboard.writeText(key)
      setCopied(slot)
      setTimeout(() => setCopied(null), 2000)
    } catch {
      // clipboard permission denied
    }
  }

  function resetEnrollForm() {
    setEnrollName('')
    setEnrollPubkey('')
    setEnrollEndpoint('')
    setEnrollAllowedIPs('')
    setEnrollApiAddress('')
    setEnrollRole('remote')
  }

  async function enroll(e: FormEvent) {
    e.preventDefault()
    if (!enrollPubkey || !enrollEndpoint || !enrollName || !enrollAllowedIPs) return
    setEnrollError('')
    setEnrollSaving(true)
    try {
      const address = enrollApiAddress || meshIPFromAllowedIPs(enrollAllowedIPs)
      const node = await createNode({ name: enrollName, type: enrollRole, address, public_key: enrollPubkey })
      await addPeer({ public_key: enrollPubkey, endpoint: enrollEndpoint, allowed_ips: enrollAllowedIPs, node_id: node.id })
      await loadNodes()
      setShowEnroll(false)
      resetEnrollForm()
      toast(`${enrollName} enrolled`)
    } catch (e2) {
      setEnrollError(e2 instanceof ApiError ? e2.message : 'Enrollment failed')
    } finally {
      setEnrollSaving(false)
    }
  }

  // ── Node groups ──────────────────────────────────────────────────────────
  const [groups, setGroups] = useState<NodeGroup[]>([])
  const [groupsLoading, setGroupsLoading] = useState(true)
  const [expandedGroup, setExpandedGroup] = useState<string | null>(null)
  const [groupNodes, setGroupNodes] = useState<Record<string, NodeRecord[]>>({})
  const [addMemberNode, setAddMemberNode] = useState<Record<string, string>>({})
  const [showGroupForm, setShowGroupForm] = useState(false)
  const [groupName, setGroupName] = useState('')
  const [groupDescription, setGroupDescription] = useState('')
  const [groupSaving, setGroupSaving] = useState(false)
  const [groupError, setGroupError] = useState('')

  async function loadGroups() {
    setGroupsLoading(true)
    try {
      setGroups(await listNodeGroups())
    } catch {
      // best-effort — groups section just stays empty
    } finally {
      setGroupsLoading(false)
    }
  }

  useEffect(() => {
    loadGroups()
  }, [])

  async function toggleGroup(id: string) {
    if (expandedGroup === id) {
      setExpandedGroup(null)
      return
    }
    setExpandedGroup(id)
    if (!groupNodes[id]) {
      try {
        const gn = await getGroupNodes(id)
        setGroupNodes((prev) => ({ ...prev, [id]: gn }))
      } catch {
        setGroupNodes((prev) => ({ ...prev, [id]: [] }))
      }
    }
  }

  function nodesNotInGroup(group: NodeGroup) {
    const memberIds = new Set((groupNodes[group.id] ?? []).map((n) => n.id))
    return nodes.filter((n) => !memberIds.has(n.id))
  }

  async function addMember(groupId: string) {
    const nodeId = addMemberNode[groupId]
    if (!nodeId) return
    try {
      await addNodeToGroup(groupId, nodeId)
      setGroupNodes((prev) => ({ ...prev, [groupId]: prev[groupId] ? [...prev[groupId]] : [] }))
      const refreshed = await getGroupNodes(groupId)
      setGroupNodes((prev) => ({ ...prev, [groupId]: refreshed }))
      setAddMemberNode((prev) => ({ ...prev, [groupId]: '' }))
      await loadGroups()
    } catch {
      toast('Failed to add member', 'danger')
    }
  }

  async function removeMember(groupId: string, nodeId: string) {
    try {
      await removeNodeFromGroup(groupId, nodeId)
      setGroupNodes((prev) => ({ ...prev, [groupId]: (prev[groupId] ?? []).filter((n) => n.id !== nodeId) }))
      await loadGroups()
    } catch {
      toast('Failed to remove member', 'danger')
    }
  }

  async function deleteGroup(id: string) {
    try {
      await deleteNodeGroup(id)
      if (expandedGroup === id) setExpandedGroup(null)
      setGroupNodes((prev) => {
        const next = { ...prev }
        delete next[id]
        return next
      })
      await loadGroups()
    } catch {
      toast('Failed to delete group', 'danger')
    }
  }

  async function createGroup(e: FormEvent) {
    e.preventDefault()
    if (!groupName) return
    setGroupError('')
    setGroupSaving(true)
    try {
      await createNodeGroup({ name: groupName, description: groupDescription || undefined })
      await loadGroups()
      setShowGroupForm(false)
      setGroupName('')
      setGroupDescription('')
    } catch (e2) {
      setGroupError(e2 instanceof ApiError ? e2.message : 'Failed to create group')
    } finally {
      setGroupSaving(false)
    }
  }

  // ── File distribution ─────────────────────────────────────────────────
  const [showDistribute, setShowDistribute] = useState(false)
  const [distTargetType, setDistTargetType] = useState<'node' | 'group'>('node')
  const [distTargetId, setDistTargetId] = useState('')
  const [distFile, setDistFile] = useState<File | null>(null)
  const [distDestPath, setDistDestPath] = useState('')
  const [distPermissions, setDistPermissions] = useState('0644')
  const [distUseTemplate, setDistUseTemplate] = useState(false)
  const [distTemplateVars, setDistTemplateVars] = useState('')
  const [distError, setDistError] = useState('')
  const [distSaving, setDistSaving] = useState(false)

  const [transfers, setTransfers] = useState<FileTransfer[]>([])
  const [transfersLoading, setTransfersLoading] = useState(true)

  async function loadTransfers() {
    setTransfersLoading(true)
    try {
      setTransfers(await listFileTransfers())
    } catch {
      // best-effort — history section just stays empty
    } finally {
      setTransfersLoading(false)
    }
  }

  useEffect(() => {
    loadTransfers()
  }, [])

  function resetDistributeForm() {
    setDistTargetType('node')
    setDistTargetId('')
    setDistFile(null)
    setDistDestPath('')
    setDistPermissions('0644')
    setDistUseTemplate(false)
    setDistTemplateVars('')
    setDistError('')
  }

  async function distribute(e: FormEvent) {
    e.preventDefault()
    if (!distTargetId || !distFile || !distDestPath) return
    setDistError('')

    let templateVars: Record<string, string> | undefined
    if (distUseTemplate) {
      try {
        templateVars = distTemplateVars.trim() ? JSON.parse(distTemplateVars) : {}
      } catch {
        setDistError('Template variables must be valid JSON, e.g. {"Environment":"production"}')
        return
      }
    }

    setDistSaving(true)
    try {
      if (distTargetType === 'node') {
        const t = await distributeFileToNode(distTargetId, distFile, {
          destinationPath: distDestPath,
          permissions: distPermissions || undefined,
          templateVars,
        })
        toast(t.status === 'done' ? `${distFile.name} delivered to ${t.node_name}` : `Delivery to ${t.node_name} failed: ${t.error}`, t.status === 'done' ? 'success' : 'danger')
      } else {
        const results = await distributeFileToGroup(distTargetId, distFile, {
          destinationPath: distDestPath,
          permissions: distPermissions || undefined,
          templateVars,
        })
        const failed = results.filter((r) => r.status !== 'done')
        toast(
          failed.length === 0 ? `${distFile.name} delivered to ${results.length} node${results.length !== 1 ? 's' : ''}` : `${results.length - failed.length}/${results.length} delivered — ${failed.length} failed`,
          failed.length === 0 ? 'success' : 'danger',
        )
      }
      await loadTransfers()
      resetDistributeForm()
      setShowDistribute(false)
    } catch (e2) {
      setDistError(e2 instanceof ApiError ? e2.message : 'Distribution failed')
    } finally {
      setDistSaving(false)
    }
  }

  function fmtTime(iso: string): string {
    try {
      return new Date(iso).toLocaleString()
    } catch {
      return iso
    }
  }

  function transferStatusVariant(status: FileTransfer['status']) {
    if (status === 'done') return 'success'
    if (status === 'failed') return 'danger'
    return 'warning'
  }

  return (
    <AppShell title="Nodes">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Enrollment Ledger</p>
            <h2 className={styles.reportTitle}>Node registry</h2>
          </div>
          <span className="stamp">Operational</span>
        </div>

        <div className={styles.statGrid}>
          {stats.map((s) => (
            <div className={styles.statCard} key={s.label}>
              <span className={styles.statValue}>{s.value}</span>
              <span className={styles.statLabel}>{s.label}</span>
            </div>
          ))}
        </div>

        <Section
          title="Nodes"
          action={
            <div className={styles.toolbar}>
              <div className={styles.searchWrap}>
                <Search size={13} className={styles.searchIcon} />
                <input
                  className={styles.searchInput}
                  placeholder="Search nodes…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <div className={styles.filterGroup}>
                {(['all', 'hub', 'remote'] as const).map((f) => (
                  <button
                    key={f}
                    type="button"
                    className={activeFilter === f ? `${styles.filterBtn} ${styles.filterBtnActive}` : styles.filterBtn}
                    onClick={() => setActiveFilter(f)}
                  >
                    {f === 'all' ? 'All' : f === 'hub' ? 'Hub' : 'Remote'}
                  </button>
                ))}
              </div>
              <Button variant="ghost" onClick={openEnroll}>
                <UserPlus size={14} />
                Enroll peer
              </Button>
            </div>
          }
        >
          {showEnroll && (
            <form className={styles.enrollForm} onSubmit={enroll}>
              <Callout variant="info">
                Enter the remote node's WireGuard details. That node must already be configured to
                point at this hub — enrollment here adds it to the mesh and authorises the connection.
              </Callout>

              <div className={styles.hubKeyBlock}>
                <span className={styles.hubKeyLabel}>This hub's public key</span>
                <div className={styles.keyBlock}>
                  <code className={styles.keyVal}>{localPubkey}</code>
                  <button type="button" className={styles.copyBtn} onClick={() => copyKey(localPubkey, 'local')} aria-label="Copy public key">
                    {copied === 'local' ? <Check size={12} /> : <Copy size={12} />}
                  </button>
                </div>
                <span className={styles.hubKeyHint}>
                  Give this key and endpoint <code>{localEndpoint}</code> to the remote node operator so
                  they can point their node at this hub.
                </span>
              </div>

              <div className={styles.hubKeyBlock}>
                <span className={styles.hubKeyLabel}>Hub proxy key (enter this in the remote node's setup)</span>
                <div className={styles.keyBlock}>
                  <code className={styles.keyVal}>{hubHMACKey || 'loading…'}</code>
                  <button
                    type="button"
                    className={styles.copyBtn}
                    onClick={() => copyKey(hubHMACKey, 'hmac')}
                    disabled={!hubHMACKey}
                    aria-label="Copy hub proxy key"
                  >
                    {copied === 'hmac' ? <Check size={12} /> : <Copy size={12} />}
                  </button>
                </div>
                <span className={styles.hubKeyHint}>The remote node needs this key to accept API requests proxied from this hub.</span>
              </div>

              <TextField label="Node name" id="enrollName" value={enrollName} onChange={(e) => setEnrollName(e.target.value)} placeholder="worker-03" />
              <TextField
                label="Remote node's public key"
                id="enrollPubkey"
                value={enrollPubkey}
                onChange={(e) => setEnrollPubkey(e.target.value)}
                placeholder="Base64-encoded WireGuard public key"
              />
              <TextField
                label="Remote node's endpoint (host:port)"
                id="enrollEndpoint"
                value={enrollEndpoint}
                onChange={(e) => setEnrollEndpoint(e.target.value)}
                placeholder="1.2.3.4:51820"
                hint="The node's current WireGuard UDP endpoint. WireGuard's keepalive keeps this up to date once the peer connects — you only need a reachable initial value."
              />
              <TextField
                label="Allowed IPs"
                id="enrollAllowedIPs"
                value={enrollAllowedIPs}
                onChange={(e) => setEnrollAllowedIPs(e.target.value)}
                placeholder="10.102.132.2/32"
              />
              <TextField
                label="API address (optional)"
                id="enrollApiAddress"
                value={enrollApiAddress}
                onChange={(e) => setEnrollApiAddress(e.target.value)}
                placeholder="auto-derived from Allowed IPs"
                hint="Leave blank to use the node's mesh IP from Allowed IPs. Only needed when the API runs on a different address."
              />
              <Select
                label="Role"
                id="enrollRole"
                value={enrollRole}
                onChange={(e) => setEnrollRole(e.target.value as 'remote' | 'hub')}
                options={ROLE_OPTIONS}
              />

              {enrollError && <Callout variant="danger">{enrollError}</Callout>}
              <div className={styles.sectionActions}>
                <Button type="button" variant="ghost" onClick={() => setShowEnroll(false)} disabled={enrollSaving}>
                  Cancel
                </Button>
                <Button type="submit" disabled={enrollSaving || !enrollPubkey || !enrollEndpoint || !enrollName || !enrollAllowedIPs}>
                  {enrollSaving ? 'Enrolling…' : 'Enroll peer'}
                </Button>
              </div>
            </form>
          )}

          {apiError ? (
            <Callout variant="danger">{apiError}</Callout>
          ) : loading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : filtered.length === 0 ? (
            <EmptyState
              icon={Server}
              title={nodes.length ? 'No nodes match your filter' : 'No nodes enrolled yet'}
              description={nodes.length ? undefined : 'Enroll a peer manually to add it to the mesh.'}
            />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Type</th>
                  <th>WireGuard address</th>
                  <th>Public key</th>
                  <th>Status</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {filtered.map((n) => {
                  const pending = n.status === 'pending'
                  return (
                    <tr key={n.id}>
                      <td className="cell-name">{n.name}</td>
                      <td>
                        <Badge variant={roleVariant(n.type)}>{n.type}</Badge>
                      </td>
                      <td className="cell-muted">{n.address}</td>
                      <td className="cell-muted" title={n.public_key}>
                        {truncateKey(n.public_key)}
                      </td>
                      <td>
                        <Badge variant={pending ? 'warning' : 'success'}>{pending ? 'Pending' : 'Approved'}</Badge>
                      </td>
                      <td>
                        <div className={styles.rowActions}>
                          {pending && (
                            <>
                              <button type="button" className={`${styles.removeBtn} ${styles.approveBtn}`} onClick={() => approvePending(n)} aria-label={`Approve ${n.name}`} title="Approve">
                                <Check size={15} />
                              </button>
                              <button type="button" className={styles.removeBtn} onClick={() => rejectPending(n)} aria-label={`Reject ${n.name}`} title="Reject">
                                <X size={15} />
                              </button>
                            </>
                          )}
                          <button type="button" className={styles.removeBtn} onClick={() => removeNode(n)} aria-label={`Remove ${n.name}`} title="Remove">
                            <Trash2 size={15} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </Table>
          )}
        </Section>

        <Section
          title="Node groups"
          action={
            <Button variant="ghost" onClick={() => setShowGroupForm((v) => !v)}>
              <Plus size={14} />
              New group
            </Button>
          }
        >
          {showGroupForm && (
            <form className={styles.enrollForm} onSubmit={createGroup}>
              <TextField label="Name" id="groupName" value={groupName} onChange={(e) => setGroupName(e.target.value)} placeholder="production" />
              <TextField
                label="Description"
                id="groupDescription"
                value={groupDescription}
                onChange={(e) => setGroupDescription(e.target.value)}
                placeholder="Optional description"
              />
              {groupError && <Callout variant="danger">{groupError}</Callout>}
              <div className={styles.sectionActions}>
                <Button type="button" variant="ghost" onClick={() => setShowGroupForm(false)} disabled={groupSaving}>
                  Cancel
                </Button>
                <Button type="submit" disabled={groupSaving || !groupName}>
                  {groupSaving ? 'Creating…' : 'Create group'}
                </Button>
              </div>
            </form>
          )}

          {groupsLoading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : groups.length === 0 ? (
            <EmptyState icon={Layers} title="No groups yet" description="Create one to organise nodes by role or environment." />
          ) : (
            <div className={styles.groupList}>
              {groups.map((group) => (
                <div className={styles.groupRow} key={group.id}>
                  <div className={styles.groupHeader} onClick={() => toggleGroup(group.id)}>
                    <div className={styles.groupHeaderLeft}>
                      {expandedGroup === group.id ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
                      <span className={styles.groupName}>{group.name}</span>
                      {group.description && <span className={styles.groupDesc}>{group.description}</span>}
                    </div>
                    <div className={styles.groupHeaderRight}>
                      <span className={styles.groupCount}>
                        {group.members?.length ?? 0} node{(group.members?.length ?? 0) !== 1 ? 's' : ''}
                      </span>
                      <button
                        type="button"
                        className={styles.removeBtn}
                        onClick={(e) => {
                          e.stopPropagation()
                          deleteGroup(group.id)
                        }}
                        aria-label={`Delete ${group.name}`}
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </div>

                  {expandedGroup === group.id && (
                    <div className={styles.groupMembers}>
                      {(groupNodes[group.id] ?? []).length === 0 ? (
                        <span className={styles.groupEmpty}>No nodes in this group.</span>
                      ) : (
                        (groupNodes[group.id] ?? []).map((n) => (
                          <div className={styles.groupMember} key={n.id}>
                            <span className={styles.groupMemberName}>{n.name}</span>
                            <Badge variant={roleVariant(n.type)}>{n.type}</Badge>
                            <button
                              type="button"
                              className={styles.removeBtn}
                              onClick={() => removeMember(group.id, n.id)}
                              aria-label={`Remove ${n.name} from ${group.name}`}
                            >
                              <X size={12} />
                            </button>
                          </div>
                        ))
                      )}
                      <div className={styles.groupAddRow}>
                        <select
                          className={styles.groupSelect}
                          value={addMemberNode[group.id] ?? ''}
                          onChange={(e) => setAddMemberNode((prev) => ({ ...prev, [group.id]: e.target.value }))}
                        >
                          <option value="">Add a node…</option>
                          {nodesNotInGroup(group).map((n) => (
                            <option key={n.id} value={n.id}>
                              {n.name} ({n.type})
                            </option>
                          ))}
                        </select>
                        <Button variant="ghost" disabled={!addMemberNode[group.id]} onClick={() => addMember(group.id)}>
                          Add
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </Section>

        <Section
          title="File distribution"
          action={
            <Button variant="ghost" onClick={() => setShowDistribute((v) => !v)}>
              <FileUp size={14} />
              Distribute file
            </Button>
          }
        >
          {showDistribute && (
            <form className={styles.enrollForm} onSubmit={distribute}>
              <Callout variant="info">
                Files are written under a managed directory on the target node
                (<code>&lt;DATA_DIR&gt;/distributed-files/…</code>) — not an arbitrary host path.
              </Callout>

              <Select
                label="Target"
                id="distTargetType"
                value={distTargetType}
                onChange={(e) => {
                  setDistTargetType(e.target.value as 'node' | 'group')
                  setDistTargetId('')
                }}
                options={[
                  { value: 'node', label: 'A single node' },
                  { value: 'group', label: 'A node group' },
                ]}
              />

              <div className="field">
                <label htmlFor="distTargetId">{distTargetType === 'node' ? 'Node' : 'Group'}</label>
                <select id="distTargetId" className={styles.groupSelect} value={distTargetId} onChange={(e) => setDistTargetId(e.target.value)}>
                  <option value="">Select…</option>
                  {distTargetType === 'node'
                    ? nodes.map((n) => (
                        <option key={n.id} value={n.id}>
                          {n.name} ({n.type})
                        </option>
                      ))
                    : groups.map((g) => (
                        <option key={g.id} value={g.id}>
                          {g.name}
                        </option>
                      ))}
                </select>
              </div>

              <div className="field">
                <label htmlFor="distFile">File</label>
                <input id="distFile" type="file" onChange={(e) => setDistFile(e.target.files?.[0] ?? null)} />
              </div>

              <TextField
                label="Destination path (relative)"
                id="distDestPath"
                value={distDestPath}
                onChange={(e) => setDistDestPath(e.target.value)}
                placeholder="app/config.yml"
                hint="Resolves under the managed directory on the target node — no absolute paths, no '..'."
              />
              <TextField
                label="Permissions (octal)"
                id="distPermissions"
                value={distPermissions}
                onChange={(e) => setDistPermissions(e.target.value)}
                placeholder="0644"
              />

              <label className={styles.groupEmpty} style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', cursor: 'pointer' }}>
                <input type="checkbox" checked={distUseTemplate} onChange={(e) => setDistUseTemplate(e.target.checked)} />
                Substitute template variables (e.g. <code>{'{{.NodeIP}}'}</code>, <code>{'{{.NodeName}}'}</code>)
              </label>
              {distUseTemplate && (
                <div className="field">
                  <label htmlFor="distTemplateVars">Extra variables (JSON, optional)</label>
                  <textarea
                    id="distTemplateVars"
                    className={styles.textarea}
                    rows={2}
                    value={distTemplateVars}
                    onChange={(e) => setDistTemplateVars(e.target.value)}
                    placeholder='{"Environment":"production"}'
                  />
                </div>
              )}

              {distError && <Callout variant="danger">{distError}</Callout>}
              <div className={styles.sectionActions}>
                <Button type="button" variant="ghost" onClick={() => setShowDistribute(false)} disabled={distSaving}>
                  Cancel
                </Button>
                <Button type="submit" disabled={distSaving || !distTargetId || !distFile || !distDestPath}>
                  <Send size={14} />
                  {distSaving ? 'Sending…' : 'Distribute'}
                </Button>
              </div>
            </form>
          )}

          {transfersLoading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : transfers.length === 0 ? (
            <EmptyState icon={FileUp} title="No transfers yet" description="Distribute a file to a node or group to see its history here." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Node</th>
                  <th>File</th>
                  <th>Destination</th>
                  <th>Status</th>
                  <th>When</th>
                </tr>
              </thead>
              <tbody>
                {transfers.map((t) => (
                  <tr key={t.id}>
                    <td className="cell-name">
                      {t.node_name}
                      {t.target_type === 'group' && <span className={styles.groupDesc}> (group)</span>}
                    </td>
                    <td className="cell-muted">{t.filename}</td>
                    <td className="cell-muted">{t.destination_path}</td>
                    <td>
                      <span title={t.error}>
                        <Badge variant={transferStatusVariant(t.status)}>{t.status}</Badge>
                      </span>
                    </td>
                    <td className="cell-muted">{fmtTime(t.created_at)}</td>
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

export default Nodes
