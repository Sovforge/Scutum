import { Fragment, useEffect, useState } from 'react'
import { ChevronDown, Download, ScrollText } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import Callout from '../../components/ui/Callout/Callout'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import { ApiError, auditExportUrl, downloadComplianceReport, listAuditLogs, type AuditEntry } from '../../lib/api'
import styles from './AuditLog.module.css'

// Shaped exactly like the real AuditEntry Go struct (cmd/internal/utils/logger.go)
// and the real GET /audit/logs JSON response. Export (GET /audit/logs/export)
// is a plain download link, not a filtered client-side export — the real
// backend has no filter params, so CSV/JSON export always contains the last
// N entries server-side, same as the old Nuxt frontend (it doesn't respect
// the on-page search/method filters either).
type OutcomeFilter = 'all' | 'success' | 'failure'

function fmtTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function AuditLog() {
  const toast = useToast()
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [reportLoading, setReportLoading] = useState('')

  const [search, setSearch] = useState('')
  const [method, setMethod] = useState('all')
  const [outcome, setOutcome] = useState<OutcomeFilter>('all')
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  async function load() {
    setLoading(true)
    setApiError('')
    try {
      setEntries(await listAuditLogs())
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load audit log')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function getReport(format: 'json' | 'csv' | 'text') {
    setReportLoading(format)
    try {
      await downloadComplianceReport(format)
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Failed to generate compliance report', 'danger')
    } finally {
      setReportLoading('')
    }
  }

  const failures = entries.filter((e) => e.outcome === 'failure').length
  const uniqueActors = new Set(entries.map((e) => e.actor).filter(Boolean)).size
  const methods = ['all', ...Array.from(new Set(entries.map((e) => e.method)))]

  const q = search.trim().toLowerCase()
  // Most-recent-first, matching the old app's [...entries].reverse().
  const filtered = [...entries].reverse().filter((e) => {
    if (method !== 'all' && e.method !== method) return false
    if (outcome !== 'all' && (e.outcome ?? 'success') !== outcome) return false
    if (
      q &&
      !e.action.toLowerCase().includes(q) &&
      !(e.actor ?? '').toLowerCase().includes(q) &&
      !e.path.toLowerCase().includes(q) &&
      !(e.client_ip ?? '').includes(q)
    )
      return false
    return true
  })

  function toggle(i: number) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(i)) next.delete(i)
      else next.add(i)
      return next
    })
  }

  return (
    <AppShell title="Audit Log">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Security Events</p>
            <h2 className={styles.reportTitle}>Audit trail</h2>
          </div>
          <span className={failures === 0 ? 'stamp' : 'stamp stamp--alt'}>{failures === 0 ? 'All clear' : `${failures} failures`}</span>
        </div>

        <StatGrid minWidth={140}>
          <StatCard label="Events" value={entries.length} icon={<ScrollText size={16} />} />
          <StatCard label="Success" value={entries.length - failures} color="var(--blueprint)" />
          <StatCard label="Failure" value={failures} color="var(--danger)" />
          <StatCard label="Actors" value={uniqueActors} />
        </StatGrid>

        <Section
          action={
            <div className={styles.exportActions}>
              <Button as="a" href={auditExportUrl('csv')} download variant="ghost">
                <Download size={14} />
                CSV
              </Button>
              <Button as="a" href={auditExportUrl('json')} download variant="ghost">
                <Download size={14} />
                JSON
              </Button>
              <div className={styles.reportGroup}>
                <span className={styles.reportLabel}>CRA report</span>
                <Button variant="ghost" onClick={() => getReport('json')} disabled={!!reportLoading}>
                  {reportLoading === 'json' ? '…' : 'JSON'}
                </Button>
                <Button variant="ghost" onClick={() => getReport('csv')} disabled={!!reportLoading}>
                  {reportLoading === 'csv' ? '…' : 'CSV'}
                </Button>
                <Button variant="ghost" onClick={() => getReport('text')} disabled={!!reportLoading}>
                  {reportLoading === 'text' ? '…' : 'Text'}
                </Button>
              </div>
            </div>
          }
        >
          <div className={styles.toolbar}>
            <input
              className={styles.toolbarInput}
              placeholder="Filter by action, actor, path, or IP…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <select className={styles.toolbarSelect} value={method} onChange={(e) => setMethod(e.target.value)}>
              {methods.map((m) => (
                <option key={m} value={m}>
                  {m === 'all' ? 'All methods' : m}
                </option>
              ))}
            </select>
            <div className={styles.filterChips}>
              {(['all', 'success', 'failure'] as OutcomeFilter[]).map((f) => (
                <button
                  key={f}
                  type="button"
                  className={outcome === f ? `${styles.chip} ${styles.chipActive}` : styles.chip}
                  onClick={() => setOutcome(f)}
                >
                  {f === 'all' ? 'All' : f[0].toUpperCase() + f.slice(1)}
                </button>
              ))}
            </div>
          </div>

          {apiError ? (
            <Callout variant="danger">{apiError}</Callout>
          ) : loading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : filtered.length === 0 ? (
            <EmptyState
              icon={ScrollText}
              title={entries.length ? 'No events match' : 'No audit entries found'}
              description={entries.length ? 'Nothing matches the current filters and search query.' : undefined}
              action={
                entries.length && (search || method !== 'all' || outcome !== 'all') ? (
                  <Button
                    variant="ghost"
                    onClick={() => {
                      setSearch('')
                      setMethod('all')
                      setOutcome('all')
                    }}
                  >
                    Clear filters
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th />
                  <th>Time</th>
                  <th>Action</th>
                  <th>Actor</th>
                  <th>Outcome</th>
                  <th>Method</th>
                  <th>Path</th>
                  <th>Client IP</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((e, i) => {
                  const isExpanded = expanded.has(i)
                  return (
                    <Fragment key={`${e.trace_id ?? ''}-${i}`}>
                      <tr>
                        <td>
                          <button
                            type="button"
                            className={styles.expandBtn}
                            onClick={() => toggle(i)}
                            aria-label={isExpanded ? 'Collapse' : 'Expand'}
                          >
                            <ChevronDown size={14} className={isExpanded ? undefined : styles.chevronCollapsed} />
                          </button>
                        </td>
                        <td className="cell-muted">{fmtTime(e.time)}</td>
                        <td className="cell-name">{e.action}</td>
                        <td className="cell-muted">{e.actor || '—'}</td>
                        <td>
                          <Badge variant={(e.outcome ?? 'success') === 'success' ? 'success' : 'danger'}>{e.outcome ?? 'success'}</Badge>
                        </td>
                        <td className="cell-muted">{e.method}</td>
                        <td className="cell-muted">{e.path}</td>
                        <td className="cell-muted">{e.client_ip || '—'}</td>
                      </tr>
                      {isExpanded && (
                        <tr>
                          <td colSpan={8} className={styles.detailCell}>
                            <dl className={styles.detailList}>
                              <div>
                                <dt>Actor ID</dt>
                                <dd className={styles.mono}>{e.actor_id || '—'}</dd>
                              </div>
                              <div>
                                <dt>Trace ID</dt>
                                <dd className={styles.mono}>{e.trace_id || '—'}</dd>
                              </div>
                              {Object.entries(e.extra ?? {}).map(([k, v]) => (
                                <div key={k}>
                                  <dt>{k}</dt>
                                  <dd className={styles.mono}>{v}</dd>
                                </div>
                              ))}
                            </dl>
                          </td>
                        </tr>
                      )}
                    </Fragment>
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

export default AuditLog
