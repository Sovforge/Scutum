import { useEffect, useState } from 'react'
import { Download, RotateCcw, Shield, Trash2 } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import { formatBytes } from '../../../lib/format'
import {
  ApiError,
  createBackup,
  deleteBackup,
  downloadBackupUrl,
  listBackups,
  restoreBackup,
  type BackupRecord,
} from '../../../lib/api'
import styles from '../Settings.module.css'

// Real endpoints: POST /admin/backups (create), GET /admin/backups (list),
// GET /admin/backups/:id/download, DELETE /admin/backups/:id, POST
// /admin/backups/:id/restore -> {status, requires_restart, message}.
// Manual-trigger only — no schedule/cron config exists server-side.
// `driver` is chosen server-side from config, not at creation time.

function fmtDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function BackupTab() {
  const toast = useToast()
  const [backups, setBackups] = useState<BackupRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [creating, setCreating] = useState(false)
  const [restoringId, setRestoringId] = useState('')
  const [deletingId, setDeletingId] = useState('')
  const [restoreResult, setRestoreResult] = useState<{ filename: string; message: string; requiresRestart: boolean } | null>(null)

  async function load() {
    setLoading(true)
    setApiError('')
    try {
      setBackups(await listBackups())
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load backups')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function handleCreate() {
    setCreating(true)
    try {
      const record = await createBackup()
      setBackups((prev) => [record, ...prev])
      toast('Backup created')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Backup failed', 'danger')
    } finally {
      setCreating(false)
    }
  }

  async function remove(b: BackupRecord) {
    setDeletingId(b.id)
    try {
      await deleteBackup(b.id)
      setBackups((prev) => prev.filter((x) => x.id !== b.id))
      toast(`${b.filename} deleted`, 'danger')
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Delete failed', 'danger')
    } finally {
      setDeletingId('')
    }
  }

  async function restore(b: BackupRecord) {
    setRestoringId(b.id)
    setRestoreResult(null)
    try {
      const res = await restoreBackup(b.id)
      setRestoreResult({ filename: b.filename, message: res.message, requiresRestart: res.requires_restart })
    } catch (e) {
      setRestoreResult({
        filename: b.filename,
        message: e instanceof ApiError ? e.message : 'Restore failed',
        requiresRestart: false,
      })
    } finally {
      setRestoringId('')
    }
  }

  return (
    <>
      {restoreResult && (
        <div className={styles.calloutWrap}>
          <Callout variant={restoreResult.requiresRestart ? 'warning' : 'success'}>
            <strong>{restoreResult.filename}:</strong> {restoreResult.message}
          </Callout>
        </div>
      )}

      <Section
        action={
          <Button variant="ghost" onClick={handleCreate} disabled={creating}>
            <Shield size={14} />
            {creating ? 'Creating…' : 'Create backup'}
          </Button>
        }
      >
        {apiError ? (
          <Callout variant="danger">{apiError}</Callout>
        ) : loading ? (
          <div className={styles.loadingRow}>Loading…</div>
        ) : backups.length === 0 ? (
          <EmptyState icon={Shield} title="No backups" description="Create a backup to snapshot the current database." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Filename</th>
                <th>Driver</th>
                <th>Size</th>
                <th>Created</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {backups.map((b) => (
                <tr key={b.id}>
                  <td className="cell-name">{b.filename}</td>
                  <td>
                    <Badge variant="neutral">{b.driver}</Badge>
                  </td>
                  <td className="cell-muted">{formatBytes(b.size_bytes)}</td>
                  <td className="cell-muted">{fmtDate(b.created_at)}</td>
                  <td>
                    <div className={styles.rowActions}>
                      <a href={downloadBackupUrl(b.id)} download aria-label={`Download ${b.filename}`}>
                        <Download size={14} />
                      </a>
                      <button type="button" onClick={() => restore(b)} disabled={restoringId === b.id} aria-label={`Restore ${b.filename}`}>
                        <RotateCcw size={14} />
                      </button>
                      <button
                        type="button"
                        className={styles.rowActionDanger}
                        onClick={() => remove(b)}
                        disabled={deletingId === b.id}
                        aria-label={`Delete ${b.filename}`}
                      >
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

export default BackupTab
