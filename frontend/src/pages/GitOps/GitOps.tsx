import { useEffect, useState, type FormEvent } from 'react'
import { Eye, GitBranch, Plus, RefreshCw, Trash2 } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import Select from '../../components/ui/Select/Select'
import PasswordField from '../../components/ui/PasswordField/PasswordField'
import Callout from '../../components/ui/Callout/Callout'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import {
  ApiError,
  createGitOpsSource,
  deleteGitOpsSource,
  listGitOpsSources,
  listGitOpsSyncEvents,
  listNodes,
  syncGitOpsSource,
  type GitOpsSource,
  type GitOpsSourceInput,
  type GitOpsSyncEvent,
  type NodeRecord,
} from '../../lib/api'
import styles from './GitOps.module.css'

// A real, persisted multi-source registry (GET|POST|PUT|DELETE
// /api/gitops/sources) — each source is reconciled automatically on its
// own poll interval by a background loop on the hub, and can also be
// synced on demand from here or via a signed push webhook from the git
// host (POST /api/gitops/sources/{id}/sync, X-Hub-Signature-256). "Target
// node" empty means the manifest is applied on the hub; set, it's relayed
// to that node the same way Docker/Kubernetes deploys already are.
const STATUS_VARIANT: Record<string, 'success' | 'warning' | 'danger' | 'neutral'> = {
  synced: 'success',
  skipped: 'neutral',
  error: 'danger',
  pending: 'warning',
  '': 'neutral',
}

function fmtTime(iso?: string): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function GitOps() {
  const toast = useToast()
  const [sources, setSources] = useState<GitOpsSource[]>([])
  const [nodes, setNodes] = useState<NodeRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [syncingId, setSyncingId] = useState('')
  const [busyId, setBusyId] = useState('')

  const [expanded, setExpanded] = useState<string | null>(null)
  const [events, setEvents] = useState<GitOpsSyncEvent[]>([])
  const [eventsLoading, setEventsLoading] = useState(false)

  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [repoUrl, setRepoUrl] = useState('')
  const [branch, setBranch] = useState('main')
  const [path, setPath] = useState('')
  const [manifestType, setManifestType] = useState<'compose' | 'kubernetes'>('compose')
  const [targetNodeId, setTargetNodeId] = useState('')
  const [pollInterval, setPollInterval] = useState('60')
  const [username, setUsername] = useState('')
  const [token, setToken] = useState('')
  const [webhookSecret, setWebhookSecret] = useState('')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  async function load() {
    setLoading(true)
    setApiError('')
    try {
      const [s, n] = await Promise.all([listGitOpsSources(), listNodes()])
      setSources(s)
      setNodes(n)
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load GitOps sources')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const synced = sources.filter((s) => s.last_status === 'synced' || s.last_status === 'skipped').length
  const failed = sources.filter((s) => s.last_status === 'error').length
  const pending = sources.filter((s) => s.last_status === '' || s.last_status === 'pending').length

  function targetLabel(s: GitOpsSource): string {
    if (!s.target_node_id) return 'hub'
    return nodes.find((n) => n.id === s.target_node_id)?.name ?? `node:${s.target_node_id.slice(0, 8)}`
  }

  function resetForm() {
    setName('')
    setRepoUrl('')
    setBranch('main')
    setPath('')
    setManifestType('compose')
    setTargetNodeId('')
    setPollInterval('60')
    setUsername('')
    setToken('')
    setWebhookSecret('')
    setFormError('')
  }

  async function submitAdd(e: FormEvent) {
    e.preventDefault()
    if (!name || !repoUrl || !path) {
      setFormError('Name, repo URL, and manifest path are required.')
      return
    }
    setSaving(true)
    setFormError('')
    const payload: GitOpsSourceInput = {
      name,
      repo_url: repoUrl,
      branch: branch || 'main',
      path,
      manifest_type: manifestType,
      target_node_id: targetNodeId || undefined,
      poll_interval_seconds: Number(pollInterval) || 60,
      username: username || undefined,
      token: token || undefined,
      webhook_secret: webhookSecret || undefined,
    }
    try {
      await createGitOpsSource(payload)
      toast(`${name} added — syncing now`)
      resetForm()
      setShowAdd(false)
      load()
    } catch (e2) {
      setFormError(e2 instanceof ApiError ? e2.message : 'Failed to add source')
    } finally {
      setSaving(false)
    }
  }

  async function sync(s: GitOpsSource) {
    setSyncingId(s.id)
    try {
      await syncGitOpsSource(s.id)
      toast(`${s.name}: sync triggered`)
      setTimeout(load, 1500)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Sync trigger failed', 'danger')
    } finally {
      setSyncingId('')
    }
  }

  async function remove(s: GitOpsSource) {
    setBusyId(s.id)
    try {
      await deleteGitOpsSource(s.id)
      setSources((prev) => prev.filter((x) => x.id !== s.id))
      setExpanded((prev) => (prev === s.id ? null : prev))
      toast(`${s.name} removed`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    } finally {
      setBusyId('')
    }
  }

  async function toggleExpand(s: GitOpsSource) {
    if (expanded === s.id) {
      setExpanded(null)
      return
    }
    setExpanded(s.id)
    setEventsLoading(true)
    try {
      setEvents(await listGitOpsSyncEvents(s.id))
    } catch {
      setEvents([])
    } finally {
      setEventsLoading(false)
    }
  }

  const nodeOptions = [{ value: '', label: 'Hub (local)' }, ...nodes.map((n) => ({ value: n.id, label: n.name }))]

  return (
    <AppShell title="GitOps">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Declarative Stacks</p>
            <h2 className={styles.reportTitle}>GitOps sources</h2>
          </div>
          <span className={failed === 0 ? 'stamp' : 'stamp stamp--alt'}>
            {synced}/{sources.length} synced
          </span>
        </div>

        <StatGrid minWidth={140}>
          <StatCard label="Sources" value={sources.length} icon={<GitBranch size={16} />} />
          <StatCard label="Synced" value={synced} color="var(--blueprint)" />
          <StatCard label="Pending" value={pending} color="var(--rust)" />
          <StatCard label="Failed" value={failed} color="var(--danger)" />
        </StatGrid>

        <Section
          action={
            <Button variant="ghost" onClick={() => setShowAdd((v) => !v)}>
              <Plus size={14} />
              Add source
            </Button>
          }
        >
          {showAdd && (
            <form className={styles.addForm} onSubmit={submitAdd}>
              <div className={styles.calloutWrap}>
                <Callout variant="info">
                  Adding a source syncs it immediately, then reconciles it automatically on its own poll interval.
                  A source can also be synced on push via a webhook — set a webhook secret below, then point your
                  git host's push webhook at{' '}
                  <code className={styles.mono}>POST /api/gitops/sources/&lt;id&gt;/sync</code> with a GitHub-style{' '}
                  <code className={styles.mono}>X-Hub-Signature-256</code> header.
                </Callout>
              </div>
              <div className={styles.addFields}>
                <TextField label="Display name" id="gitopsName" value={name} onChange={(e) => setName(e.target.value)} placeholder="observability-stack" />
                <TextField
                  label="Repo URL"
                  id="gitopsRepoUrl"
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  placeholder="https://git.internal/scutum/observability-stack.git"
                />
                <TextField label="Branch" id="gitopsBranch" value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="main" />
                <TextField label="Manifest path" id="gitopsPath" value={path} onChange={(e) => setPath(e.target.value)} placeholder="docker-compose.yml" />
                <Select
                  label="Manifest type"
                  id="gitopsManifestType"
                  value={manifestType}
                  onChange={(e) => setManifestType(e.target.value as 'compose' | 'kubernetes')}
                  options={[
                    { value: 'compose', label: 'Docker Compose' },
                    { value: 'kubernetes', label: 'Kubernetes YAML' },
                  ]}
                />
                <Select label="Apply on" id="gitopsTargetNode" value={targetNodeId} onChange={(e) => setTargetNodeId(e.target.value)} options={nodeOptions} />
                <TextField
                  label="Poll interval (seconds)"
                  id="gitopsPollInterval"
                  type="number"
                  value={pollInterval}
                  onChange={(e) => setPollInterval(e.target.value)}
                />
                <TextField label="Username (optional, private repos)" id="gitopsUsername" value={username} onChange={(e) => setUsername(e.target.value)} />
                <PasswordField label="Token (optional, private repos)" id="gitopsToken" value={token} onChange={(e) => setToken(e.target.value)} />
                <PasswordField label="Webhook secret (optional)" id="gitopsWebhookSecret" value={webhookSecret} onChange={(e) => setWebhookSecret(e.target.value)} />
              </div>
              {formError && <p className={styles.formError}>{formError}</p>}
              <div className={styles.addActions}>
                <Button type="submit" disabled={saving}>
                  {saving ? 'Adding…' : 'Add and sync'}
                </Button>
              </div>
            </form>
          )}

          {apiError ? (
            <Callout variant="danger">{apiError}</Callout>
          ) : loading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : sources.length === 0 ? (
            <EmptyState icon={GitBranch} title="No sources linked" description="Add a Git repo to start reconciling stacks onto this mesh." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Repo</th>
                  <th>Branch / Path</th>
                  <th>Apply on</th>
                  <th>Status</th>
                  <th>Last synced</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {sources.map((s) => {
                  const busy = busyId === s.id
                  return (
                    <tr key={s.id} className={styles.clickableRow} onClick={() => toggleExpand(s)}>
                      <td className="cell-name">{s.name}</td>
                      <td className="cell-muted">{s.repo_url}</td>
                      <td className="cell-muted">
                        {s.branch} / {s.path}
                      </td>
                      <td className="cell-muted">{targetLabel(s)}</td>
                      <td>
                        <span title={s.last_error}>
                          <Badge variant={STATUS_VARIANT[s.last_status] ?? 'neutral'}>{s.last_status || 'pending'}</Badge>
                        </span>
                      </td>
                      <td className="cell-muted">{fmtTime(s.last_synced_at)}</td>
                      <td onClick={(e) => e.stopPropagation()}>
                        <div className={styles.rowActions}>
                          <button type="button" onClick={() => sync(s)} disabled={syncingId === s.id || busy} aria-label={`Sync ${s.name}`}>
                            <RefreshCw size={14} className={syncingId === s.id ? styles.spin : undefined} />
                          </button>
                          <button type="button" className={styles.rowActionDanger} onClick={() => remove(s)} disabled={busy} aria-label={`Remove ${s.name}`}>
                            <Trash2 size={14} />
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

        {expanded && (
          <Section title={`${sources.find((s) => s.id === expanded)?.name} — sync history`}>
            {eventsLoading ? (
              <div className={styles.loadingRow}>Loading…</div>
            ) : events.length === 0 ? (
              <EmptyState icon={Eye} title="No sync events yet" description="Events show up here after the first sync attempt." />
            ) : (
              <div className={styles.historyList}>
                {events.map((ev) => (
                  <div className={styles.historyRow} key={ev.id}>
                    <span className={styles.historyTime}>{fmtTime(ev.started_at)}</span>
                    <Badge variant={STATUS_VARIANT[ev.status] ?? 'neutral'}>{ev.status}</Badge>
                    <span className={styles.historyRepo}>{ev.commit_sha ? ev.commit_sha.slice(0, 8) : '—'}</span>
                    <span className={styles.historyRepo}>{ev.triggered_by}</span>
                    <span className={styles.historyMessage} title={ev.message}>
                      {ev.message}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </Section>
        )}
      </div>
    </AppShell>
  )
}

export default GitOps
