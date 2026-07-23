import { useState } from 'react'
import { Download, RotateCcw, Shield, Trash2 } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import { formatBytes } from '../../../lib/format'
import styles from '../Settings.module.css'

// Real endpoints: POST /admin/backups (create), GET /admin/backups
// (list), GET /admin/backups/:id/download, DELETE /admin/backups/:id,
// POST /admin/backups/:id/restore -> {status, requires_restart, message}.
// Manual-trigger only — no schedule/cron config exists in the real
// backend or old UI. `driver` is chosen server-side from config, not at
// creation time (matched here — no driver selector on the Create button).
type BackupRecord = {
  id: string
  filename: string
  driver: string
  sizeBytes: number
  createdAt: string
}

const INITIAL_BACKUPS: BackupRecord[] = [
  { id: 'bkp3', filename: 'scutum-backup-2026-07-09-0100.tar.gz', driver: 'local', sizeBytes: 41_200_000, createdAt: '2026-07-09 01:00:00' },
  { id: 'bkp2', filename: 'scutum-backup-2026-07-02-0100.tar.gz', driver: 'local', sizeBytes: 39_800_000, createdAt: '2026-07-02 01:00:00' },
  { id: 'bkp1', filename: 'scutum-backup-2026-06-25-0100.tar.gz', driver: 'local', sizeBytes: 38_100_000, createdAt: '2026-06-25 01:00:00' },
]

function BackupTab() {
  const toast = useToast()
  const [backups, setBackups] = useState(INITIAL_BACKUPS)
  const [restoreResult, setRestoreResult] = useState<{ filename: string; message: string } | null>(null)

  function createBackup() {
    const now = new Date()
    const filename = `scutum-backup-${now.toISOString().slice(0, 10)}-${now.toISOString().slice(11, 16).replace(':', '')}.tar.gz`
    setBackups((prev) => [{ id: `bkp${prev.length + 1}`, filename, driver: 'local', sizeBytes: 40_000_000 + Math.floor(Math.random() * 2_000_000), createdAt: 'just now' }, ...prev])
    toast('Backup created')
  }

  function download(b: BackupRecord) {
    const blob = new Blob(
      [JSON.stringify({ filename: b.filename, driver: b.driver, size_bytes: b.sizeBytes, created_at: b.createdAt, note: 'Preview build — this is metadata only, not a real database dump.' }, null, 2)],
      { type: 'application/json' },
    )
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${b.filename}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  function remove(b: BackupRecord) {
    setBackups((prev) => prev.filter((x) => x.id !== b.id))
    toast(`${b.filename} deleted`, 'danger')
  }

  function restore(b: BackupRecord) {
    setRestoreResult({ filename: b.filename, message: `Database restored from ${b.filename}. A restart is required to apply changes.` })
    toast(`Restored from ${b.filename}`, 'danger')
  }

  return (
    <>
      {restoreResult && (
        <div className={styles.calloutWrap}>
          <Callout variant="warning">
            <strong>{restoreResult.filename}:</strong> {restoreResult.message}
          </Callout>
        </div>
      )}

      <Section action={<Button variant="ghost" onClick={createBackup}><Shield size={14} />Create backup</Button>}>
        {backups.length === 0 ? (
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
                  <td className="cell-muted">{formatBytes(b.sizeBytes)}</td>
                  <td className="cell-muted">{b.createdAt}</td>
                  <td>
                    <div className={styles.rowActions}>
                      <button type="button" onClick={() => download(b)} aria-label={`Download ${b.filename}`}>
                        <Download size={14} />
                      </button>
                      <button type="button" onClick={() => restore(b)} aria-label={`Restore ${b.filename}`}>
                        <RotateCcw size={14} />
                      </button>
                      <button type="button" className={styles.rowActionDanger} onClick={() => remove(b)} aria-label={`Delete ${b.filename}`}>
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
