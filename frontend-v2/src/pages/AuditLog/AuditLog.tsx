import { Fragment, useState } from 'react'
import { ChevronDown, Download, ScrollText } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import StatCard from '../../components/ui/StatCard/StatCard'
import StatGrid from '../../components/ui/StatCard/StatGrid'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import styles from './AuditLog.module.css'

// Shaped exactly like the real AuditEntry Go struct (cmd/internal/utils/logger.go)
// and the real GET /audit/logs JSON response — actor/actorId/outcome were
// added in the EU CRA compliance logging overhaul (Annex I requires actor
// identity + outcome on security events). The old Nuxt frontend's own
// AuditEntry TS type and its CSV export handler both predate that overhaul
// and are missing these three fields — built against the real JSON shape
// here instead of copying that stale interface. Action names below are
// real event names used by the actual audit() calls in the Go handlers.
// Not wired to a live backend yet.
type Outcome = 'success' | 'failure'
type AuditEntry = {
  time: string
  action: string
  actor: string
  actorId: string
  outcome: Outcome
  method: string
  path: string
  traceId: string
  clientIp: string
  extra: Record<string, string>
}

const ENTRIES: AuditEntry[] = [
  { time: '2026-07-09 09:14:03', action: 'LOGIN_SUCCESS', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/auth/login', traceId: 'a1b2c3d4', clientIp: '203.0.113.44', extra: {} },
  { time: '2026-07-09 09:12:51', action: 'LOGIN_FAILED', actor: 'unknown', actorId: '', outcome: 'failure', method: 'POST', path: '/auth/login', traceId: 'e5f6a7b8', clientIp: '203.0.113.44', extra: { reason: 'invalid_credentials' } },
  { time: '2026-07-09 08:58:20', action: 'MFA_ENABLED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/auth/mfa/enable', traceId: 'c9d0e1f2', clientIp: '10.100.0.1', extra: {} },
  { time: '2026-07-09 08:45:10', action: 'API_KEY_CREATED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/auth/tokens', traceId: 'f1a2b3c4', clientIp: '10.100.0.1', extra: { name: 'ci-deploy-key' } },
  { time: '2026-07-09 08:40:02', action: 'COMPOSE_DEPLOYED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/docker/deploy-compose', traceId: 'b8c9d0e1', clientIp: '10.100.0.1', extra: { stack: 'observability', node: 'hub-fra1' } },
  { time: '2026-07-09 08:20:44', action: 'TERMINAL_SESSION_STARTED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'GET', path: '/docker/containers/c1a2b3c4/terminal', traceId: 'd4e5f6a7', clientIp: '10.100.0.1', extra: { container: 'scutum-agent', node: 'hub-fra1' } },
  { time: '2026-07-08 22:10:05', action: 'ROLE_UPDATED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'PUT', path: '/roles/ops-oncall', traceId: '9f0a1b2c', clientIp: '10.100.0.1', extra: { role: 'ops-oncall' } },
  { time: '2026-07-08 09:15:00', action: 'PEER_ADDED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/network/peer', traceId: '3d4e5f6a', clientIp: '10.100.0.1', extra: { peer: 'edge-tokyo' } },
  { time: '2026-07-05 11:40:12', action: 'PLUGIN_LOADED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/plugins/upload', traceId: '7b8c9d0e', clientIp: '10.100.0.1', extra: { plugin: 'webhook-router' } },
  { time: '2026-07-02 09:14:00', action: 'PLUGIN_LOADED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/plugins/upload', traceId: '1a2b3c4d', clientIp: '10.100.0.1', extra: { plugin: 'slack-notifier' } },
  { time: '2026-06-28 14:22:10', action: 'USER_CREATED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/users', traceId: '5e6f7a8b', clientIp: '10.100.0.1', extra: { username: 'ops-oncall' } },
  { time: '2026-06-25 10:05:33', action: 'PASSWORD_RESET', actor: 'j.doe@example.com', actorId: 'usr_jdoe02', outcome: 'success', method: 'POST', path: '/auth/password-reset', traceId: '2c3d4e5f', clientIp: '198.51.100.9', extra: {} },
  { time: '2026-06-21 09:00:00', action: 'EDGE_REGISTERED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/nodes', traceId: '8d9e0f1a', clientIp: '10.100.0.1', extra: { node: 'build-runner' } },
  { time: '2026-06-15 16:40:02', action: 'RECOVERY_CODES_REGENERATED', actor: 'admin', actorId: 'usr_admin01', outcome: 'success', method: 'POST', path: '/auth/recovery/regenerate', traceId: '4f5a6b7c', clientIp: '10.100.0.1', extra: {} },
]

const METHODS = ['all', ...Array.from(new Set(ENTRIES.map((e) => e.method)))]
type OutcomeFilter = 'all' | Outcome

function downloadBlob(content: string, filename: string, type: string) {
  const blob = new Blob([content], { type })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

function toCsv(entries: AuditEntry[]) {
  const headers = ['time', 'action', 'actor', 'actor_id', 'outcome', 'method', 'path', 'client_ip', 'trace_id']
  const rows = entries.map((e) => [e.time, e.action, e.actor, e.actorId, e.outcome, e.method, e.path, e.clientIp, e.traceId])
  return [headers, ...rows].map((row) => row.map((v) => `"${String(v).replace(/"/g, '""')}"`).join(',')).join('\n')
}

function AuditLog() {
  const [search, setSearch] = useState('')
  const [method, setMethod] = useState('all')
  const [outcome, setOutcome] = useState<OutcomeFilter>('all')
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  const failures = ENTRIES.filter((e) => e.outcome === 'failure').length
  const uniqueActors = new Set(ENTRIES.map((e) => e.actor)).size

  const q = search.trim().toLowerCase()
  const filtered = ENTRIES.filter((e) => {
    if (method !== 'all' && e.method !== method) return false
    if (outcome !== 'all' && e.outcome !== outcome) return false
    if (
      q &&
      !e.action.toLowerCase().includes(q) &&
      !e.actor.toLowerCase().includes(q) &&
      !e.path.toLowerCase().includes(q) &&
      !e.clientIp.includes(q)
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
          <StatCard label="Events" value={ENTRIES.length} icon={<ScrollText size={16} />} />
          <StatCard label="Success" value={ENTRIES.length - failures} color="var(--blueprint)" />
          <StatCard label="Failure" value={failures} color="var(--danger)" />
          <StatCard label="Actors" value={uniqueActors} />
        </StatGrid>

        <Section
          action={
            <div className={styles.exportActions}>
              <Button variant="ghost" onClick={() => downloadBlob(toCsv(filtered), 'audit-log.csv', 'text/csv')}>
                <Download size={14} />
                CSV
              </Button>
              <Button variant="ghost" onClick={() => downloadBlob(JSON.stringify(filtered, null, 2), 'audit-log.json', 'application/json')}>
                <Download size={14} />
                JSON
              </Button>
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
              {METHODS.map((m) => (
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

          {filtered.length === 0 ? (
            <EmptyState
              icon={ScrollText}
              title="No events match"
              description="Nothing matches the current filters and search query."
              action={
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
                    <Fragment key={`${e.traceId}-${i}`}>
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
                        <td className="cell-muted">{e.time}</td>
                        <td className="cell-name">{e.action}</td>
                        <td className="cell-muted">{e.actor}</td>
                        <td>
                          <Badge variant={e.outcome === 'success' ? 'success' : 'danger'}>{e.outcome}</Badge>
                        </td>
                        <td className="cell-muted">{e.method}</td>
                        <td className="cell-muted">{e.path}</td>
                        <td className="cell-muted">{e.clientIp}</td>
                      </tr>
                      {isExpanded && (
                        <tr>
                          <td colSpan={8} className={styles.detailCell}>
                            <dl className={styles.detailList}>
                              <div>
                                <dt>Actor ID</dt>
                                <dd className={styles.mono}>{e.actorId || '—'}</dd>
                              </div>
                              <div>
                                <dt>Trace ID</dt>
                                <dd className={styles.mono}>{e.traceId}</dd>
                              </div>
                              {Object.entries(e.extra).map(([k, v]) => (
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
