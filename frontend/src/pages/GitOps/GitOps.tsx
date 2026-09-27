import { useEffect, useState, type FormEvent } from 'react'
import { GitBranch, Plus, RefreshCw, Trash2 } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import PasswordField from '../../components/ui/PasswordField/PasswordField'
import Callout from '../../components/ui/Callout/Callout'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import { ApiError, gitSync } from '../../lib/api'
import styles from './GitOps.module.css'

type SyncStatus = 'synced' | 'out-of-sync' | 'failed'
type SyncResult = 'ok' | 'fail'

// The real backend has exactly one stateless endpoint here — POST
// /git/sync (clone if the target dir doesn't exist, else pull) — there is
// no repo-registry, no sync history, and no webhook config server-side.
// The old Nuxt frontend keeps its repo list in localStorage and computes
// "sync status" purely client-side from whether the last sync call
// succeeded; this does the same.
type Repo = {
  id: string
  name: string
  repoUrl: string
  branch: string
  target: string
  username?: string
  token?: string
  syncStatus: SyncStatus
  lastSync: string | null
}

type SyncEvent = {
  id: number
  repo: string
  result: SyncResult
  message: string
  time: string
}

const STATUS_VARIANT: Record<SyncStatus, 'success' | 'warning' | 'danger'> = {
  synced: 'success',
  'out-of-sync': 'warning',
  failed: 'danger',
}

const STORAGE_KEY = 'scutum_gitops_repos'

function loadRepos(): Repo[] {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
  } catch {
    return []
  }
}

let nextEventId = 1

function GitOps() {
  const toast = useToast()
  const [repos, setRepos] = useState<Repo[]>(loadRepos)
  const [history, setHistory] = useState<SyncEvent[]>([])
  const [syncingId, setSyncingId] = useState('')
  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [repoUrl, setRepoUrl] = useState('')
  const [branch, setBranch] = useState('main')
  const [target, setTarget] = useState('')
  const [username, setUsername] = useState('')
  const [token, setToken] = useState('')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(repos))
    } catch {
      /* storage unavailable — repo list just won't survive a reload */
    }
  }, [repos])

  const synced = repos.filter((r) => r.syncStatus === 'synced').length
  const outOfSync = repos.filter((r) => r.syncStatus === 'out-of-sync').length
  const failed = repos.filter((r) => r.syncStatus === 'failed').length

  function logEvent(repo: string, result: SyncResult, message: string) {
    setHistory((prev) => [{ id: nextEventId++, repo, result, message, time: new Date().toLocaleTimeString() }, ...prev])
  }

  async function sync(repo: Repo) {
    setSyncingId(repo.id)
    try {
      const message = await gitSync({
        repo_url: repo.repoUrl,
        username: repo.username || undefined,
        token: repo.token || undefined,
        target_dir: repo.target,
      })
      const now = new Date().toLocaleTimeString()
      setRepos((prev) => prev.map((r) => (r.id === repo.id ? { ...r, syncStatus: 'synced', lastSync: now } : r)))
      logEvent(repo.name, 'ok', message)
      toast(`${repo.name} synced`)
    } catch (e) {
      const message = e instanceof ApiError ? e.message : 'Sync failed. Please check the server logs.'
      setRepos((prev) => prev.map((r) => (r.id === repo.id ? { ...r, syncStatus: 'failed' } : r)))
      logEvent(repo.name, 'fail', message)
      toast(`${repo.name}: ${message}`, 'danger')
    } finally {
      setSyncingId('')
    }
  }

  function remove(repo: Repo) {
    setRepos((prev) => prev.filter((r) => r.id !== repo.id))
    toast(`${repo.name} removed`, 'danger')
  }

  async function submitAdd(e: FormEvent) {
    e.preventDefault()
    if (!name || !repoUrl || !target) {
      setFormError('Display name, repo URL, and target directory are required.')
      return
    }
    if (!repoUrl.startsWith('https://')) {
      setFormError('Repo URL must start with https:// — the sync endpoint rejects anything else.')
      return
    }
    setSaving(true)
    setFormError('')
    const repo: Repo = {
      id: crypto.randomUUID(),
      name,
      repoUrl,
      branch: branch || 'main',
      target,
      username: username || undefined,
      token: token || undefined,
      syncStatus: 'out-of-sync',
      lastSync: null,
    }
    try {
      const message = await gitSync({
        repo_url: repo.repoUrl,
        username: repo.username,
        token: repo.token,
        target_dir: repo.target,
      })
      repo.syncStatus = 'synced'
      repo.lastSync = new Date().toLocaleTimeString()
      setRepos((prev) => [...prev, repo])
      logEvent(name, 'ok', message)
      toast(`${name} added and synced`)
      setName('')
      setRepoUrl('')
      setBranch('main')
      setTarget('')
      setUsername('')
      setToken('')
      setShowAdd(false)
    } catch (e2) {
      const message = e2 instanceof ApiError ? e2.message : 'Sync failed. Please check the server logs.'
      setRepos((prev) => [...prev, repo])
      logEvent(name, 'fail', message)
      setFormError(message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <AppShell title="GitOps">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Declarative Stacks</p>
            <h2 className={styles.reportTitle}>GitOps registry</h2>
          </div>
          <span className={outOfSync === 0 && failed === 0 ? 'stamp' : 'stamp stamp--alt'}>
            {synced}/{repos.length} synced
          </span>
        </div>

        <StatGrid minWidth={140}>
          <StatCard label="Repositories" value={repos.length} icon={<GitBranch size={16} />} />
          <StatCard label="Synced" value={synced} color="var(--blueprint)" />
          <StatCard label="Out of sync" value={outOfSync} color="var(--rust)" />
          <StatCard label="Failed" value={failed} color="var(--danger)" />
        </StatGrid>

        <Section
          action={
            <Button variant="ghost" onClick={() => setShowAdd((v) => !v)}>
              <Plus size={14} />
              Add repo
            </Button>
          }
        >
          {showAdd && (
            <form className={styles.addForm} onSubmit={submitAdd}>
              <div className={styles.calloutWrap}>
                <Callout variant="info">
                  Adding a repo clones or pulls it into the target directory right away — there's no scheduled/periodic sync, only
                  manual and on-add.
                </Callout>
              </div>
              <div className={styles.addFields}>
                <TextField label="Display name" id="repoName" value={name} onChange={(e) => setName(e.target.value)} placeholder="observability-stack" />
                <TextField
                  label="Repo URL"
                  id="repoUrl"
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  placeholder="https://git.internal/scutum/observability-stack.git"
                />
                <TextField label="Branch" id="repoBranch" value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="main" />
                <TextField label="Target directory" id="repoTarget" value={target} onChange={(e) => setTarget(e.target.value)} placeholder="observability" />
                <TextField label="Username (optional)" id="repoUsername" value={username} onChange={(e) => setUsername(e.target.value)} />
                <PasswordField label="Token (optional)" id="repoToken" value={token} onChange={(e) => setToken(e.target.value)} />
              </div>
              {formError && <p className={styles.formError}>{formError}</p>}
              <div className={styles.addActions}>
                <Button type="submit" disabled={saving}>
                  {saving ? 'Adding…' : 'Add and sync'}
                </Button>
              </div>
            </form>
          )}

          {repos.length === 0 ? (
            <EmptyState icon={GitBranch} title="No repositories linked" description="Add a Git repo to start reconciling stacks onto this mesh." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Repo</th>
                  <th>Branch</th>
                  <th>Target</th>
                  <th>Status</th>
                  <th>Last synced</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {repos.map((r) => (
                  <tr key={r.id}>
                    <td className="cell-name">{r.name}</td>
                    <td className="cell-muted">{r.repoUrl}</td>
                    <td className="cell-muted">{r.branch}</td>
                    <td className="cell-muted">{r.target}</td>
                    <td>
                      <Badge variant={STATUS_VARIANT[r.syncStatus]}>{r.syncStatus}</Badge>
                    </td>
                    <td className="cell-muted">{r.lastSync ?? '—'}</td>
                    <td>
                      <div className={styles.rowActions}>
                        <button type="button" onClick={() => sync(r)} disabled={syncingId === r.id} aria-label={`Sync ${r.name}`}>
                          <RefreshCw size={14} className={syncingId === r.id ? styles.spin : undefined} />
                        </button>
                        <button type="button" className={styles.rowActionDanger} onClick={() => remove(r)} aria-label={`Remove ${r.name}`}>
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

        <Section title="Sync history">
          {history.length === 0 ? (
            <EmptyState title="No sync events yet" description="Sync events will show up here once a repo is added or synced." />
          ) : (
            <div className={styles.historyList}>
              {history.map((h) => (
                <div className={styles.historyRow} key={h.id}>
                  <span className={styles.historyTime}>{h.time}</span>
                  <Badge variant={h.result === 'ok' ? 'success' : 'danger'}>{h.result}</Badge>
                  <span className={styles.historyRepo}>{h.repo}</span>
                  <span className={styles.historyMessage}>{h.message}</span>
                </div>
              ))}
            </div>
          )}
        </Section>
      </div>
    </AppShell>
  )
}

export default GitOps
