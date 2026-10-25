import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Box, Layers, Search, Server, Terminal as TerminalIcon, Trash2, X } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { getToken, listAllK8sPods, listContainers, listNodes, wsUrl, type DockerContainer, type NodeRecord } from '../../lib/api'
import styles from './Terminal.module.css'

// Real endpoints: WS /api/docker/containers/{id}/terminal and
// WS /api/k8s/{namespace}/{pod}/terminal, both raw byte streams (no JSON
// envelope) — stdin goes out, stdout/stderr comes back on the same socket.
// Auth travels as a `token` query param (the WebSocket API can't set
// headers), and a `nodeId` query param tells the hub to proxy to a remote
// node instead of execing locally, mirroring the X-Target-Node header used
// everywhere else in this app.

type Kind = 'container' | 'pod'
type ContainerItem = { id: string; name: string; status: string; image: string }
type ContainerGroup = { node: string; nodeId?: string; containers: ContainerItem[] }
type PodItem = { name: string; namespace: string; phase: string; image: string }
type PodGroup = { cluster: string; nodeId?: string; pods: PodItem[] }

type Target = { id: string; name: string; kind: Kind; node: string; nodeId?: string; image: string; namespace?: string }

type TermLine = { type: 'cmd' | 'out' | 'err' | 'info'; html: string; cwd?: string; promptChar?: string }
type Session = {
  id: number
  name: string
  kind: Kind
  node: string
  nodeId?: string
  image: string
  namespace?: string
  cwd: string
  promptChar: string
  output: TermLine[]
  connected: boolean
}

const LINE_CLASS: Record<TermLine['type'], string> = {
  cmd: 'termLineCmd',
  out: 'termLineOut',
  err: 'termLineErr',
  info: 'termLineInfo',
}

function mapContainers(ctrs: DockerContainer[]): ContainerItem[] {
  return ctrs.map((c) => ({
    id: c.Id,
    name: (c.Names?.[0] ?? c.Id.slice(0, 12)).replace(/^\//, ''),
    status: c.State ?? (c.Status?.toLowerCase().includes('up') ? 'running' : 'stopped'),
    image: c.Image,
  }))
}

function mapPods(raw: { items?: any[] } | null, nodeId?: string): PodGroup[] {
  const items: PodItem[] = (raw?.items ?? []).map((p: any) => ({
    name: p.metadata?.name ?? '',
    namespace: p.metadata?.namespace ?? 'default',
    phase: p.status?.phase ?? 'Unknown',
    image: p.spec?.containers?.[0]?.image ?? '',
  }))
  const byNs: Record<string, PodItem[]> = {}
  for (const p of items) (byNs[p.namespace] ??= []).push(p)
  return Object.entries(byNs).map(([ns, pods]) => ({ cluster: ns, nodeId, pods }))
}

const ANSI_COLORS: Record<string, string> = {
  '30': '#21222c', '31': '#ff5555', '32': '#50fa7b', '33': '#f1fa8c',
  '34': '#bd93f9', '35': '#ff79c6', '36': '#8be9fd', '37': '#f8f8f2',
  '90': '#6272a4', '91': '#ffb86c', '92': '#50fa7b', '93': '#f1fa8c',
  '94': '#bd93f9', '95': '#ff79c6', '96': '#8be9fd', '97': '#ffffff',
}

// Turns raw ANSI SGR escape codes into inline-styled <span>s so real shell
// output (ls colors, prompt colors, etc.) renders instead of showing control
// codes literally. Ported from the old app's exact same approach.
function ansiToHtml(str: string): string {
  let res = str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  let spanOpen = false

  res = res.replace(/\x1b\[([0-9;]*)m/g, (_, codes) => {
    let out = spanOpen ? '</span>' : ''
    spanOpen = false
    if (codes === '0' || codes === '' || codes === 'm') return out

    const styles: string[] = []
    for (const c of codes.split(';')) {
      if (ANSI_COLORS[c]) styles.push(`color: ${ANSI_COLORS[c]}`)
      if (c === '1') styles.push('font-weight: bold')
      if (c === '3') styles.push('font-style: italic')
      if (c === '4') styles.push('text-decoration: underline')
    }
    if (styles.length) {
      spanOpen = true
      return out + `<span style="${styles.join('; ')}">`
    }
    return out
  })

  res = res.replace(/\x1b\[[?0-9;]*[a-zA-Z]/g, '').replace(/[\x00-\x08\x0b-\x1f\x7f]/g, '')
  if (spanOpen) res += '</span>'
  return res
}

let nextId = 1

function Terminal() {
  const [pickerTab, setPickerTab] = useState<'containers' | 'pods'>('containers')
  const [search, setSearch] = useState('')

  const [containerGroups, setContainerGroups] = useState<ContainerGroup[]>([])
  const [containersLoading, setContainersLoading] = useState(false)
  const [podGroups, setPodGroups] = useState<PodGroup[]>([])
  const [podsLoading, setPodsLoading] = useState(false)

  const [sessions, setSessions] = useState<Session[]>([])
  const [activeIndex, setActiveIndex] = useState(0)
  const [inputValue, setInputValue] = useState('')

  const wsMap = useRef<Map<number, WebSocket>>(new Map())
  const lastSentCmd = useRef<string | null>(null)
  const historyRef = useRef<string[]>([])
  const historyIdxRef = useRef(-1)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const outputRef = useRef<HTMLDivElement | null>(null)

  const active = sessions[activeIndex] ?? null

  async function loadContainers() {
    setContainersLoading(true)
    try {
      const nodes = await listNodes().catch(() => [] as NodeRecord[])
      const groups: ContainerGroup[] = []

      const localCtrs = await listContainers().catch(() => [] as DockerContainer[])
      const localItems = mapContainers(localCtrs)
      if (localItems.length) groups.push({ node: 'Local', containers: localItems })

      for (const n of nodes.filter((n) => n.type !== 'hub')) {
        const ctrs = await listContainers(n.id).catch(() => [] as DockerContainer[])
        const items = mapContainers(ctrs)
        if (items.length) groups.push({ node: n.name, nodeId: n.id, containers: items })
      }
      setContainerGroups(groups)
    } catch {
      setContainerGroups([])
    } finally {
      setContainersLoading(false)
    }
  }

  async function loadPods() {
    setPodsLoading(true)
    try {
      const nodes = await listNodes().catch(() => [] as NodeRecord[])
      const groups: PodGroup[] = []

      const localRaw = await listAllK8sPods().catch(() => null)
      groups.push(...mapPods(localRaw))

      for (const n of nodes.filter((n) => n.type !== 'hub')) {
        const remoteRaw = await listAllK8sPods(n.id).catch(() => null)
        groups.push(...mapPods(remoteRaw, n.id))
      }
      setPodGroups(groups)
    } catch {
      setPodGroups([])
    } finally {
      setPodsLoading(false)
    }
  }

  useEffect(() => {
    loadContainers()
    return () => {
      for (const ws of wsMap.current.values()) ws.close()
      wsMap.current.clear()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function switchTab(tab: 'containers' | 'pods') {
    setPickerTab(tab)
    if (tab === 'pods' && podGroups.length === 0) loadPods()
  }

  function scrollToBottom() {
    requestAnimationFrame(() => {
      if (outputRef.current) outputRef.current.scrollTop = outputRef.current.scrollHeight
    })
  }

  function pushLine(sessionId: number, type: TermLine['type'], text: string, cwd?: string, promptChar?: string) {
    setSessions((prev) =>
      prev.map((s) =>
        s.id === sessionId
          ? { ...s, output: [...s.output, { type, html: ansiToHtml(text), cwd: type === 'cmd' ? cwd : undefined, promptChar: type === 'cmd' ? promptChar : undefined }] }
          : s,
      ),
    )
    scrollToBottom()
  }

  function connectWS(session: Session, target: Target) {
    const token = getToken() ?? ''
    const nodeParam = target.nodeId ? `&nodeId=${encodeURIComponent(target.nodeId)}` : ''
    const path =
      target.kind === 'container'
        ? `/docker/containers/${encodeURIComponent(target.id)}/terminal?token=${encodeURIComponent(token)}${nodeParam}`
        : `/k8s/${encodeURIComponent(target.namespace ?? 'default')}/${encodeURIComponent(target.id)}/terminal?token=${encodeURIComponent(token)}${nodeParam}`

    pushLine(session.id, 'info', `Connecting to ${target.name}…`)

    let ws: WebSocket
    try {
      ws = new WebSocket(wsUrl(path))
    } catch {
      pushLine(session.id, 'err', 'WebSocket connection failed.')
      return
    }
    wsMap.current.set(session.id, ws)
    ws.binaryType = 'arraybuffer'

    ws.onopen = () => {
      setSessions((prev) => prev.map((s) => (s.id === session.id ? { ...s, connected: true } : s)))
      pushLine(session.id, 'info', 'Connected. Type commands and press Enter.')
      inputRef.current?.focus()
    }

    ws.onmessage = (ev) => {
      const raw = ev.data instanceof ArrayBuffer ? new TextDecoder().decode(ev.data) : (ev.data as string)

      // Strip keep-alive bytes, carriage returns, and non-color escape codes
      // before prompt detection so the regex anchors reliably.
      let clean = raw.replace(/[\r\0]/g, '').replace(/\x1b\[[?0-9;]*[a-zA-Z]/g, (m) => (m.endsWith('m') ? m : ''))
      if (!clean.replace(/[\x00-\x1f\x7f-\x9f]/g, '').trim()) return

      // Heuristically detect a trailing shell prompt to track cwd/prompt char
      // and drop the prompt line itself from the visible scrollback.
      const promptMatch = clean.trimEnd().match(/(^|[\n])(.*?)\s?([#$])\s?$/)
      let cwd: string | undefined
      let promptChar: string | undefined
      setSessions((prev) => {
        const s = prev.find((x) => x.id === session.id)
        cwd = s?.cwd
        promptChar = s?.promptChar
        return prev
      })

      if (promptMatch) {
        let rawPath = (promptMatch[2] || '').replace(/\x1b\[[0-9;]*m/g, '').trim()
        if (rawPath.includes(':')) rawPath = rawPath.split(':').pop() || ''
        rawPath = rawPath.replace(/[[\]()]/g, '').trim()
        if (rawPath) cwd = rawPath
        promptChar = promptMatch[3]
        setSessions((prev) => prev.map((s) => (s.id === session.id ? { ...s, cwd: cwd ?? s.cwd, promptChar: promptChar ?? s.promptChar } : s)))
        const promptIdx = clean.lastIndexOf(promptMatch[0].trim())
        if (promptIdx !== -1) clean = clean.slice(0, promptIdx)
      }

      const lines = clean.split('\n')
      for (const line of lines) {
        if (!line.trim()) continue
        const nakedLine = line.replace(/\x1b\[[?0-9;]*[a-zA-Z]/g, '').trim()
        const nakedSent = (lastSentCmd.current || '').trim()

        if (nakedSent && (nakedLine === nakedSent || nakedLine.endsWith(nakedSent))) {
          if (nakedSent !== 'clear') pushLine(session.id, 'cmd', line, cwd, promptChar)
          lastSentCmd.current = null
        } else {
          pushLine(session.id, 'out', line)
        }
      }
    }

    ws.onerror = () => pushLine(session.id, 'err', 'Connection error.')
    ws.onclose = () => {
      setSessions((prev) => prev.map((s) => (s.id === session.id ? { ...s, connected: false } : s)))
      pushLine(session.id, 'info', 'Connection closed.')
      wsMap.current.delete(session.id)
    }
  }

  function openTarget(target: Target) {
    const existingIndex = sessions.findIndex((s) => s.name === target.name && s.kind === target.kind)
    if (existingIndex !== -1) {
      setActiveIndex(existingIndex)
      inputRef.current?.focus()
      return
    }
    const session: Session = {
      id: nextId++,
      name: target.name,
      kind: target.kind,
      node: target.node,
      nodeId: target.nodeId,
      image: target.image,
      namespace: target.namespace,
      cwd: '~',
      promptChar: '$',
      output: [],
      connected: false,
    }
    setSessions((prev) => [...prev, session])
    setActiveIndex(sessions.length)
    connectWS(session, target)
  }

  function closeSession(index: number) {
    const session = sessions[index]
    if (session) {
      wsMap.current.get(session.id)?.close()
      wsMap.current.delete(session.id)
    }
    setSessions((prev) => prev.filter((_, i) => i !== index))
    setActiveIndex((i) => Math.max(0, Math.min(i, sessions.length - 2)))
  }

  function clearOutput() {
    if (!active) return
    setSessions((prev) => prev.map((s) => (s.id === active.id ? { ...s, output: [] } : s)))
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (!active?.connected) return
    const ws = wsMap.current.get(active.id)

    if (e.key === 'Enter') {
      e.preventDefault()
      const cmd = inputValue
      if (cmd.trim() === 'clear') {
        clearOutput()
        lastSentCmd.current = 'clear'
        ws?.send('clear\n')
        setInputValue('')
        return
      }
      lastSentCmd.current = cmd
      if (cmd.trim()) {
        historyRef.current.unshift(cmd)
        historyIdxRef.current = -1
      }
      ws?.send(cmd + '\n')
      setInputValue('')
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      historyIdxRef.current = Math.min(historyIdxRef.current + 1, historyRef.current.length - 1)
      setInputValue(historyRef.current[historyIdxRef.current] ?? '')
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      historyIdxRef.current = Math.max(historyIdxRef.current - 1, -1)
      setInputValue(historyIdxRef.current >= 0 ? (historyRef.current[historyIdxRef.current] ?? '') : '')
    } else if (e.key === 'l' && e.ctrlKey) {
      e.preventDefault()
      clearOutput()
    } else if (e.key === 'c' && e.ctrlKey) {
      e.preventDefault()
      ws?.send('\x03')
      setInputValue('')
    }
  }

  const q = search.toLowerCase()
  const filteredContainerGroups = containerGroups
    .map((g) => ({ ...g, containers: g.containers.filter((c) => !q || c.name.toLowerCase().includes(q) || g.node.toLowerCase().includes(q)) }))
    .filter((g) => g.containers.length)
  const filteredPodGroups = podGroups
    .map((g) => ({ ...g, pods: g.pods.filter((p) => !q || p.name.toLowerCase().includes(q) || g.cluster.toLowerCase().includes(q)) }))
    .filter((g) => g.pods.length)

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
                  onClick={() => switchTab('containers')}
                >
                  <Box size={12} />
                  Containers
                </button>
                <button
                  type="button"
                  className={pickerTab === 'pods' ? `${styles.pTab} ${styles.pTabActive}` : styles.pTab}
                  onClick={() => switchTab('pods')}
                >
                  <Layers size={12} />
                  Pods
                </button>
              </div>
            </div>

            <div className={styles.pickerList}>
              {pickerTab === 'containers' &&
                (containersLoading ? (
                  <p className={styles.groupLabel}>Loading…</p>
                ) : filteredContainerGroups.length === 0 ? (
                  <p className={styles.groupLabel}>No containers found.</p>
                ) : (
                  filteredContainerGroups.map((g) => (
                    <div key={g.node}>
                      <div className={styles.groupLabel}>{g.node}</div>
                      {g.containers.map((c) => (
                        <button
                          key={c.id}
                          type="button"
                          className={styles.pickerItem}
                          disabled={c.status !== 'running'}
                          onClick={() => openTarget({ id: c.id, name: c.name, kind: 'container', node: g.node, nodeId: g.nodeId, image: c.image })}
                        >
                          <span className={c.status === 'running' ? `${styles.dot} ${styles.dotRunning}` : styles.dot} />
                          <span className={styles.itemName}>{c.name}</span>
                          <span className={styles.itemSub}>{c.image.split('/').pop()}</span>
                        </button>
                      ))}
                    </div>
                  ))
                ))}

              {pickerTab === 'pods' &&
                (podsLoading ? (
                  <p className={styles.groupLabel}>Loading…</p>
                ) : filteredPodGroups.length === 0 ? (
                  <p className={styles.groupLabel}>No pods found.</p>
                ) : (
                  filteredPodGroups.map((g) => (
                    <div key={g.cluster + (g.nodeId ?? '')}>
                      <div className={styles.groupLabel}>{g.cluster}</div>
                      {g.pods.map((p) => (
                        <button
                          key={p.name}
                          type="button"
                          className={styles.pickerItem}
                          disabled={p.phase !== 'Running'}
                          onClick={() =>
                            openTarget({ id: p.name, name: p.name, kind: 'pod', node: g.cluster, nodeId: g.nodeId, image: p.image, namespace: p.namespace })
                          }
                        >
                          <span className={p.phase === 'Running' ? `${styles.dot} ${styles.dotRunning}` : styles.dot} />
                          <span className={styles.itemName}>{p.name}</span>
                          <span className={styles.itemSub}>{p.namespace}</span>
                        </button>
                      ))}
                    </div>
                  ))
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
                <div className={styles.sessionControls}>
                  {active && (
                    <span className={styles.sessionMeta}>
                      <Server size={12} />
                      {active.node}
                    </span>
                  )}
                  <button type="button" className={styles.ctrlBtn} onClick={clearOutput}>
                    <Trash2 size={12} />
                    Clear
                  </button>
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
                          <span className={styles.promptSep}>:</span>
                          <span className={styles.promptDir}>{line.cwd || '~'}</span>
                          <span className={styles.promptDollar}>{line.promptChar || '$'}</span>
                        </span>
                      )}
                      <span className={styles.termText} dangerouslySetInnerHTML={{ __html: line.html }} />
                    </div>
                  ))}

                  {active.connected && (
                    <div className={styles.termInputRow}>
                      <span className={styles.prompt}>
                        <span className={styles.promptTarget}>{active.name}</span>
                        <span className={styles.promptSep}>:</span>
                        <span className={styles.promptDir}>{active.cwd}</span>
                        <span className={styles.promptDollar}>{active.promptChar}</span>
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
