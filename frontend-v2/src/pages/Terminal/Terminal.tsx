import { useRef, useState, type KeyboardEvent } from 'react'
import { Box, Layers, Search, Terminal as TerminalIcon, X } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import styles from './Terminal.module.css'

// Mock data grounded in the real picker structure (see the old app's
// terminal page: containers grouped by node, pods grouped by namespace).
// The real page execs into a container/pod over a WebSocket at
// /api/docker/containers/{id}/terminal or /api/k8s/{ns}/{pod}/terminal.
// There's no live backend here, so sessions simulate a shell locally
// instead of pretending to hold a real connection — the UI says so.
type ContainerTarget = { id: string; name: string; node: string; image: string; status: 'running' | 'exited' }
type PodTarget = { name: string; namespace: string; cluster: string; phase: 'Running' | 'Pending'; image: string }

const CONTAINERS: ContainerTarget[] = [
  { id: 'c1', name: 'scutum-agent', node: 'hub-fra1', image: 'ghcr.io/sovforge/scutum:latest', status: 'running' },
  { id: 'c2', name: 'postgres', node: 'hub-fra1', image: 'postgres:16-alpine', status: 'running' },
  { id: 'c3', name: 'grafana', node: 'build-runner', image: 'grafana/grafana:11', status: 'running' },
  { id: 'c4', name: 'minio', node: 'edge-london', image: 'minio/minio:latest', status: 'running' },
  { id: 'c5', name: 'backup-job', node: 'build-runner', image: 'restic/restic:latest', status: 'exited' },
]

const PODS: PodTarget[] = [
  { name: 'otel-collector-6d9f4', namespace: 'observability', cluster: 'build-runner', phase: 'Running', image: 'otel/opentelemetry-collector:latest' },
  { name: 'grafana-7c8b2', namespace: 'observability', cluster: 'build-runner', phase: 'Running', image: 'grafana/grafana:11' },
  { name: 'api-gateway-5f6a1', namespace: 'default', cluster: 'build-runner', phase: 'Pending', image: 'nginx:1.27' },
]

type Line = { type: 'cmd' | 'out' | 'err' | 'info'; text: string }
type Kind = 'container' | 'pod'
type Session = { id: number; name: string; kind: Kind; node: string; output: Line[]; connected: boolean }

const LINE_CLASS: Record<Line['type'], string> = {
  cmd: 'termLineCmd',
  out: 'termLineOut',
  err: 'termLineErr',
  info: 'termLineInfo',
}

const CANNED: Record<string, string> = {
  ls: 'app  data  secrets  scutum',
  pwd: '/app',
  whoami: 'root',
  'echo hello': 'hello',
  clear: '',
}

let nextId = 1

function Terminal() {
  const [pickerTab, setPickerTab] = useState<'containers' | 'pods'>('containers')
  const [search, setSearch] = useState('')
  const [sessions, setSessions] = useState<Session[]>([])
  const [activeIndex, setActiveIndex] = useState(0)
  const [inputValue, setInputValue] = useState('')
  const historyRef = useRef<string[]>([])
  const historyIdxRef = useRef(-1)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const outputRef = useRef<HTMLDivElement | null>(null)

  const active = sessions[activeIndex] ?? null

  function scrollToBottom() {
    requestAnimationFrame(() => {
      if (outputRef.current) outputRef.current.scrollTop = outputRef.current.scrollHeight
    })
  }

  function pushLine(sessionId: number, line: Line) {
    setSessions((prev) => prev.map((s) => (s.id === sessionId ? { ...s, output: [...s.output, line] } : s)))
    scrollToBottom()
  }

  function openTarget(name: string, kind: Kind, node: string) {
    const existingIndex = sessions.findIndex((s) => s.name === name && s.kind === kind)
    if (existingIndex !== -1) {
      setActiveIndex(existingIndex)
      inputRef.current?.focus()
      return
    }
    const id = nextId++
    const session: Session = { id, name, kind, node, output: [], connected: false }
    setSessions((prev) => [...prev, session])
    setActiveIndex(sessions.length)
    pushLine(id, { type: 'info', text: `Connecting to ${name}…` })
    setTimeout(() => {
      setSessions((prev) => prev.map((s) => (s.id === id ? { ...s, connected: true } : s)))
      pushLine(id, { type: 'info', text: 'Connected — simulated shell, no live backend is wired up yet.' })
      inputRef.current?.focus()
    }, 350)
  }

  function closeSession(index: number) {
    setSessions((prev) => prev.filter((_, i) => i !== index))
    setActiveIndex((i) => Math.max(0, Math.min(i, sessions.length - 2)))
  }

  function runCommand(cmd: string) {
    if (!active) return
    pushLine(active.id, { type: 'cmd', text: cmd })
    const trimmed = cmd.trim()
    if (trimmed === 'clear') {
      setSessions((prev) => prev.map((s) => (s.id === active.id ? { ...s, output: [] } : s)))
      return
    }
    if (trimmed === 'hostname') {
      pushLine(active.id, { type: 'out', text: active.node })
      return
    }
    if (trimmed === '') return
    const canned = CANNED[trimmed]
    if (canned !== undefined) {
      pushLine(active.id, { type: 'out', text: canned })
    } else {
      pushLine(active.id, { type: 'err', text: `bash: ${trimmed}: command not found (simulated shell)` })
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (!active?.connected) return
    if (e.key === 'Enter') {
      e.preventDefault()
      const cmd = inputValue
      if (cmd.trim()) {
        historyRef.current.unshift(cmd)
        historyIdxRef.current = -1
      }
      runCommand(cmd)
      setInputValue('')
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      historyIdxRef.current = Math.min(historyIdxRef.current + 1, historyRef.current.length - 1)
      setInputValue(historyRef.current[historyIdxRef.current] ?? '')
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      historyIdxRef.current = Math.max(historyIdxRef.current - 1, -1)
      setInputValue(historyIdxRef.current >= 0 ? (historyRef.current[historyIdxRef.current] ?? '') : '')
    } else if (e.key === 'c' && e.ctrlKey) {
      e.preventDefault()
      pushLine(active.id, { type: 'info', text: '^C' })
      setInputValue('')
    } else if (e.key === 'l' && e.ctrlKey) {
      e.preventDefault()
      setSessions((prev) => prev.map((s) => (s.id === active.id ? { ...s, output: [] } : s)))
    }
  }

  const q = search.toLowerCase()
  const containerGroups = Object.entries(
    CONTAINERS.filter((c) => !q || c.name.toLowerCase().includes(q) || c.node.toLowerCase().includes(q)).reduce<Record<string, ContainerTarget[]>>(
      (acc, c) => {
        ;(acc[c.node] ??= []).push(c)
        return acc
      },
      {},
    ),
  )
  const podGroups = Object.entries(
    PODS.filter((p) => !q || p.name.toLowerCase().includes(q) || p.namespace.toLowerCase().includes(q)).reduce<Record<string, PodTarget[]>>(
      (acc, p) => {
        ;(acc[p.namespace] ??= []).push(p)
        return acc
      },
      {},
    ),
  )

  return (
    <AppShell title="Terminal">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Live Exec</p>
            <h2 className={styles.reportTitle}>Session console</h2>
          </div>
          <span className="stamp">
            {sessions.length} active session{sessions.length !== 1 ? 's' : ''}
          </span>
        </div>

        <div className={styles.console}>
        <div className={styles.picker}>
          <div className={styles.pickerHeader}>
            <div className={styles.searchWrap}>
              <Search size={13} className={styles.searchIcon} />
              <input className={styles.searchInput} placeholder="Filter…" value={search} onChange={(e) => setSearch(e.target.value)} />
            </div>
            <div className={styles.pickerTabs}>
              <button
                type="button"
                className={pickerTab === 'containers' ? `${styles.pTab} ${styles.pTabActive}` : styles.pTab}
                onClick={() => setPickerTab('containers')}
              >
                <Box size={12} />
                Containers
              </button>
              <button
                type="button"
                className={pickerTab === 'pods' ? `${styles.pTab} ${styles.pTabActive}` : styles.pTab}
                onClick={() => setPickerTab('pods')}
              >
                <Layers size={12} />
                Pods
              </button>
            </div>
          </div>

          <div className={styles.pickerList}>
            {pickerTab === 'containers' &&
              containerGroups.map(([node, containers]) => (
                <div key={node}>
                  <div className={styles.groupLabel}>{node}</div>
                  {containers.map((c) => (
                    <button
                      key={c.id}
                      type="button"
                      className={styles.pickerItem}
                      disabled={c.status !== 'running'}
                      onClick={() => openTarget(c.name, 'container', c.node)}
                    >
                      <span className={c.status === 'running' ? `${styles.dot} ${styles.dotRunning}` : styles.dot} />
                      <span className={styles.itemName}>{c.name}</span>
                      <span className={styles.itemSub}>{c.image.split('/').pop()}</span>
                    </button>
                  ))}
                </div>
              ))}

            {pickerTab === 'pods' &&
              podGroups.map(([ns, pods]) => (
                <div key={ns}>
                  <div className={styles.groupLabel}>{ns}</div>
                  {pods.map((p) => (
                    <button
                      key={p.name}
                      type="button"
                      className={styles.pickerItem}
                      disabled={p.phase !== 'Running'}
                      onClick={() => openTarget(p.name, 'pod', p.cluster)}
                    >
                      <span className={p.phase === 'Running' ? `${styles.dot} ${styles.dotRunning}` : styles.dot} />
                      <span className={styles.itemName}>{p.name}</span>
                      <span className={styles.itemSub}>{p.cluster}</span>
                    </button>
                  ))}
                </div>
              ))}
          </div>
        </div>

        <div className={styles.terminalArea}>
          {sessions.length > 0 && (
            <div className={styles.sessionBar}>
              <div className={styles.sessionTabs}>
                {sessions.map((s, i) => (
                  <button
                    key={s.id}
                    type="button"
                    className={i === activeIndex ? `${styles.sessionTab} ${styles.sessionTabActive}` : styles.sessionTab}
                    onClick={() => setActiveIndex(i)}
                  >
                    {s.kind === 'pod' ? <Layers size={11} /> : <Box size={11} />}
                    {s.name}
                    <span
                      className={styles.tabClose}
                      onClick={(e) => {
                        e.stopPropagation()
                        closeSession(i)
                      }}
                    >
                      <X size={10} />
                    </span>
                  </button>
                ))}
              </div>
            </div>
          )}

          {active ? (
            <div className={styles.termWrap} onClick={() => inputRef.current?.focus()}>
              <div className={styles.termOutput} ref={outputRef}>
                <div className={styles.termBanner}>
                  {active.kind === 'pod' ? <Layers size={12} /> : <Box size={12} />}
                  exec into <span className={styles.bannerTarget}>{active.name}</span>
                  <span className={styles.bannerSep}>·</span>
                  {active.node}
                </div>

                {active.output.map((line, i) => (
                  <div key={i} className={`${styles.termLine} ${styles[LINE_CLASS[line.type]]}`}>
                    {line.type === 'cmd' && (
                      <span className={styles.prompt}>
                        <span className={styles.promptTarget}>{active.name}</span>
                        <span className={styles.promptDollar}>$</span>
                      </span>
                    )}
                    <span className={styles.termText}>{line.text}</span>
                  </div>
                ))}

                {active.connected && (
                  <div className={styles.termInputRow}>
                    <span className={styles.prompt}>
                      <span className={styles.promptTarget}>{active.name}</span>
                      <span className={styles.promptDollar}>$</span>
                    </span>
                    <input
                      ref={inputRef}
                      className={styles.termInput}
                      value={inputValue}
                      onChange={(e) => setInputValue(e.target.value)}
                      onKeyDown={onKeyDown}
                      autoComplete="off"
                      spellCheck={false}
                      autoFocus
                    />
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className={styles.termEmpty}>
              <EmptyState
                icon={TerminalIcon}
                title="No session selected"
                description="Select a running container or pod to open a terminal session."
              />
            </div>
          )}
        </div>
      </div>
      </div>
    </AppShell>
  )
}

export default Terminal
