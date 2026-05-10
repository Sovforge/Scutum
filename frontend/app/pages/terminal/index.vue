<template>
  <div class="terminal-page">

    <!-- Left: container/pod picker -->
    <div class="picker">

      <div class="picker__header">
        <div class="search-wrap">
          <Icon name="lucide:search" size="13" class="search-icon" />
          <input v-model="pickerSearch" class="search-input" placeholder="Filter…" />
        </div>
        <div class="picker__tabs">
          <button class="ptab" :class="{ 'ptab--active': pickerTab === 'containers' }" @click="pickerTab = 'containers'">
            <Icon name="lucide:box" size="12" /> Containers
          </button>
          <button class="ptab" :class="{ 'ptab--active': pickerTab === 'pods' }" @click="pickerTab = 'pods'">
            <Icon name="lucide:layers" size="12" /> Pods
          </button>
        </div>
      </div>

      <!-- Container list -->
      <div v-if="pickerTab === 'containers'" class="picker__list">
        <div v-if="containersLoading" class="picker__hint">Loading…</div>
        <div v-else-if="containerGroups.length === 0" class="picker__hint">No containers found.</div>
        <template v-for="group in filteredContainerGroups" :key="group.node">
          <div class="picker__group-label">{{ group.node }}</div>
          <button
            v-for="c in group.containers"
            :key="c.id"
            class="picker__item"
            :class="{
              'picker__item--active': activeTarget?.id === c.id,
              'picker__item--running': c.status === 'running',
            }"
            :disabled="c.status !== 'running'"
            @click="openTarget({ id: c.id, name: c.name, kind: 'container', node: group.node, image: c.image })"
          >
            <span class="picker__dot" :class="`dot--${c.status}`" />
            <span class="picker__name">{{ c.name }}</span>
            <span class="picker__sub">{{ c.image.split('/').pop() }}</span>
          </button>
        </template>
      </div>

      <!-- Pod list -->
      <div v-if="pickerTab === 'pods'" class="picker__list">
        <div v-if="podsLoading" class="picker__hint">Loading…</div>
        <div v-else-if="podGroups.length === 0" class="picker__hint">No pods found.</div>
        <template v-for="group in filteredPodGroups" :key="group.cluster">
          <div class="picker__group-label">{{ group.cluster }}</div>
          <button
            v-for="p in group.pods"
            :key="p.name"
            class="picker__item"
            :class="{
              'picker__item--active': activeTarget?.id === p.name,
              'picker__item--running': p.phase === 'Running',
            }"
            :disabled="p.phase !== 'Running'"
            @click="openTarget({ id: p.name, name: p.name, kind: 'pod', node: group.cluster, image: p.image, namespace: p.namespace })"
          >
            <span class="picker__dot" :class="p.phase === 'Running' ? 'dot--running' : 'dot--stopped'" />
            <span class="picker__name">{{ p.name }}</span>
            <span class="picker__sub">{{ p.namespace }}</span>
          </button>
        </template>
      </div>

    </div>

    <!-- Right: terminal sessions -->
    <div class="terminal-area">

      <!-- Session tabs -->
      <div class="session-bar" v-if="sessions.length">
        <div class="session-tabs">
          <button
            v-for="(sess, i) in sessions"
            :key="sess.id"
            class="session-tab"
            :class="{ 'session-tab--active': activeSession === i }"
            @click="activeSession = i"
          >
            <Icon :name="sess.kind === 'pod' ? 'lucide:layers' : 'lucide:box'" size="11" />
            {{ sess.name }}
            <button class="tab-close" @click.stop="closeSession(i)">
              <Icon name="lucide:x" size="10" />
            </button>
          </button>
        </div>
        <div class="session-controls">
          <span v-if="currentSession" class="session-meta">
            <Icon name="lucide:server" size="12" class="meta-icon" />
            {{ currentSession.node }}
          </span>
          <button class="ctrl-btn" @click="clearOutput">
            <Icon name="lucide:trash-2" size="12" /> Clear
          </button>
        </div>
      </div>

      <!-- Terminal body -->
      <div v-if="currentSession" class="term-wrap" @click="focusInput">
        <div class="term-output" ref="outputEl">

          <!-- Connection banner -->
          <div class="term-banner">
            <Icon :name="currentSession.kind === 'pod' ? 'lucide:layers' : 'lucide:box'" size="12" class="banner-icon" />
            exec into <span class="banner-target">{{ currentSession.name }}</span>
            <span class="banner-sep">·</span>
            <span class="banner-node">{{ currentSession.node }}</span>
          </div>

          <div
            v-for="(line, i) in currentOutput"
            :key="i"
            class="term-line"
            :class="`term-line--${line.type}`"
          >
            <span v-if="line.type === 'cmd'" class="term-prompt">
              <span class="prompt-target">{{ currentSession.name }}</span>
              <span class="prompt-sep">:</span>
              <span class="prompt-dir">{{ line.cwd || '~' }}</span>
              <span class="prompt-dollar">{{ line.promptChar || '$' }}</span>
            </span>
            <span class="term-text" v-html="line.text" />
          </div>

          <!-- Input line (shown only when WS connected) -->
          <div v-if="isConnected" class="term-line term-line--input">
            <span class="term-prompt">
              <span class="prompt-target">{{ currentSession.name }}</span>
              <span class="prompt-sep">:</span>
              <span class="prompt-dir">{{ currentSession.cwd || '~' }}</span>
              <span class="prompt-dollar">{{ currentSession.promptChar || '$' }}</span>
            </span>
            <span class="term-input-wrap">
              <span class="term-input-text">{{ inputBefore }}</span>
              <span class="term-cursor" :class="{ 'term-cursor--blink': focused }" />
              <span class="term-input-text">{{ inputAfter }}</span>
            </span>
            <input
              ref="hiddenInput"
              class="term-hidden-input"
              v-model="inputLine"
              @keydown="onKey"
              @focus="focused = true"
              @blur="focused = false"
              autocomplete="off"
              spellcheck="false"
            />
          </div>
        </div>
      </div>

      <!-- Empty state -->
      <div v-else class="term-empty">
        <Icon name="lucide:terminal" size="32" class="empty-icon" />
        <p>Select a running container or pod to open a terminal session.</p>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api    = useApi()
const { getToken } = useAuth()

// ── Derive WebSocket base URL from the current page origin ─────────────────
const wsBase = computed(() => {
  if (import.meta.server) return ''
  const { protocol, host } = window.location
  return protocol.replace('http', 'ws') + '//' + host
})

// ── Container data ─────────────────────────────────────────────────────────
interface ContainerItem { id: string; name: string; status: string; image: string }
interface ContainerGroup { node: string; containers: ContainerItem[] }

const containerGroups   = ref<ContainerGroup[]>([])
const containersLoading = ref(false)

async function loadContainers() {
  containersLoading.value = true
  try {
    const ctrs = await api.listContainers()
    const items = ctrs.map(c => ({
      id:     c.Id,
      name:   c.Names?.[0]?.replace(/^\//, '') ?? c.Id.slice(0, 12),
      status: (c.State ?? (c.Status.toLowerCase().includes('up') ? 'running' : 'stopped')),
      image:  c.Image,
    }))
    containerGroups.value = items.length > 0
      ? [{ node: 'Local Docker', containers: items }]
      : []
  } catch { containerGroups.value = [] } finally {
    containersLoading.value = false
  }
}

// ── Pod data ───────────────────────────────────────────────────────────────
interface PodItem { name: string; namespace: string; phase: string; image: string }
interface PodGroup { cluster: string; pods: PodItem[] }

const podGroups   = ref<PodGroup[]>([])
const podsLoading = ref(false)

async function loadPods() {
  podsLoading.value = true
  try {
    const raw: any = await api.listAllK8sPods()
    const items: PodItem[] = (raw?.items ?? []).map((p: any) => ({
      name:      p.metadata?.name ?? '',
      namespace: p.metadata?.namespace ?? 'default',
      phase:     p.status?.phase ?? 'Unknown',
      image:     p.spec?.containers?.[0]?.image ?? '',
    }))
    // Group by namespace (closest proxy for "cluster" without cluster info)
    const byNs: Record<string, PodItem[]> = {}
    for (const p of items) {
      ;(byNs[p.namespace] ??= []).push(p)
    }
    podGroups.value = Object.entries(byNs).map(([ns, pods]) => ({ cluster: ns, pods }))
  } catch { podGroups.value = [] } finally {
    podsLoading.value = false
  }
}

// ── Picker state ───────────────────────────────────────────────────────────
const pickerTab    = ref<'containers' | 'pods'>('containers')
const pickerSearch = ref('')

const filteredContainerGroups = computed(() => {
  const q = pickerSearch.value.toLowerCase()
  return containerGroups.value.map(g => ({
    ...g,
    containers: g.containers.filter(c => !q || c.name.toLowerCase().includes(q) || g.node.toLowerCase().includes(q)),
  })).filter(g => g.containers.length)
})

const filteredPodGroups = computed(() => {
  const q = pickerSearch.value.toLowerCase()
  return podGroups.value.map(g => ({
    ...g,
    pods: g.pods.filter(p => !q || p.name.toLowerCase().includes(q) || g.cluster.toLowerCase().includes(q)),
  })).filter(g => g.pods.length)
})

watch(pickerTab, tab => {
  if (tab === 'pods' && podGroups.value.length === 0) loadPods()
})

// ── Sessions ───────────────────────────────────────────────────────────────
interface TermLine { 
  type: 'cmd' | 'out' | 'err' | 'info'
  text: string
  cwd?: string
  promptChar?: string
}
interface Session  {
  id:        number
  name:      string
  kind:      'container' | 'pod'
  node:      string
  image:     string
  cwd?:      string
  promptChar?: string
  namespace?: string
  output:    TermLine[]
  ws:        WebSocket | null
}
const lastSentCmd = ref<string | null>(null)

interface Target {
  id:         string
  name:       string
  kind:       'container' | 'pod'
  node:       string
  image:      string
  namespace?: string
}

let nextId = 1
const sessions        = ref<Session[]>([])
const activeSession   = ref(0)
const activeTarget    = ref<Target | null>(null)
const connectedIds    = ref<number[]>([])
const wsMap           = new Map<number, WebSocket>()

const currentSession = computed(() => sessions.value[activeSession.value] ?? null)
const currentOutput  = computed(() => currentSession.value?.output ?? [])
const isConnected    = computed(() => {
  const sess = currentSession.value
  return sess ? connectedIds.value.includes(sess.id) : false
})

function openTarget(target: Target) {
  const existing = sessions.value.findIndex(s => s.name === target.name && s.kind === target.kind)
  if (existing !== -1) {
    activeSession.value = existing
    activeTarget.value  = target
    nextTick(focusInput)
    return
  }
  const sess: Session = {
    id:        nextId++,
    name:      target.name,
    kind:      target.kind,
    node:      target.node,
    image:     target.image,
    namespace: target.namespace,
    cwd:       '~',
    promptChar: '$',
    output:    [],
    ws:        null,
  }
  sessions.value.push(sess)
  activeSession.value = sessions.value.length - 1
  activeTarget.value  = target
  nextTick(() => { connectWS(sess, target); focusInput() })
}

function connectWS(sess: Session, target: Target) {
  const token = getToken() ?? ''
  let url: string
  if (target.kind === 'container') {
    url = `${wsBase.value}/api/docker/containers/${encodeURIComponent(target.id)}/terminal?token=${encodeURIComponent(token)}`
  } else {
    url = `${wsBase.value}/api/k8s/${encodeURIComponent(target.namespace ?? 'default')}/${encodeURIComponent(target.id)}/terminal?token=${encodeURIComponent(token)}`
  }

  pushToSession(sess, 'info', `Connecting to <span class="c-purple">${target.name}</span>…`)

  let ws: WebSocket
  try {
    ws = new WebSocket(url)
  } catch {
    pushToSession(sess, 'err', 'WebSocket connection failed.')
    return
  }

  wsMap.set(sess.id, ws)

  ws.binaryType = 'arraybuffer'

  ws.onopen = () => {
    connectedIds.value = [...connectedIds.value, sess.id]
    pushToSession(sess, 'info', 'Connected. Type commands and press Enter.')

    nextTick(focusInput)
  }

  ws.onmessage = (ev) => {
    const raw = ev.data instanceof ArrayBuffer ? new TextDecoder().decode(ev.data) : (ev.data as string)

    // 1. Filter out null bytes (keep-alives), carriage returns, and control escape codes (DSR reports, etc.)
    // This must happen before prompt detection so the regex anchors work reliably.
    let clean = raw.replace(/[\r\0]/g, '')
                   .replace(/\x1b\[[?0-9;]*[a-zA-Z]/g, (m) => m.endsWith('m') ? m : '')

    // Discard keep-alive / ping frames: if nothing printable remains after stripping
    // control characters, there is no terminal output to display.
    if (!clean.replace(/[\x00-\x1f\x7f-\x9f]/g, '').trim()) return

    // 2. Heuristic: Identify the shell prompt at the end of the stream
    const promptRegex = /(^|[\n])(.*?)\s?([#$])\s?$/
    const promptMatch = clean.trimEnd().match(promptRegex)
    
    if (promptMatch) {
      const rawGroup = (promptMatch[2] || '').replace(/\x1b\[[0-9;]*m/g, '').trim()
      let rawPath = rawGroup
      
      if (rawPath.includes(':')) {
        rawPath = rawPath.split(':').pop() || ''
      }
      rawPath = rawPath.replace(/[\[\]()]/g, '').trim()
      
      if (rawPath) sess.cwd = rawPath
      sess.promptChar = promptMatch[3]
      
      // Remove the prompt line entirely from the visible output scrollback
      const promptIdx = clean.lastIndexOf(promptMatch[0].trim())
      if (promptIdx !== -1) clean = clean.slice(0, promptIdx)
    }

    // 3. Process the remaining output as text lines or command echoes
    const lines = clean.split('\n')
    for (let line of lines) {
      if (!line.trim()) continue

      // Clean the line for comparison (strip ANSI and trim)
      const nakedLine = line.replace(/\x1b\[[?0-9;]*[a-zA-Z]/g, '').trim()
      const nakedSent = (lastSentCmd.value || '').trim()

      // If the line is an echo of the command (possibly prefixed by a prompt)
      if (nakedSent && (nakedLine === nakedSent || nakedLine.endsWith(nakedSent))) {
        // Only push to session if it's NOT a clear command to keep the screen clean
        if (nakedSent !== 'clear') {
          pushToSession(sess, 'cmd', ansiToHtml(line))
        }
        lastSentCmd.value = null 
      } else {
        pushToSession(sess, 'out', ansiToHtml(line))
      }
    }
  }

  ws.onerror = () => {
    pushToSession(sess, 'err', 'Connection error.')
  }

  ws.onclose = () => {
    connectedIds.value = connectedIds.value.filter(id => id !== sess.id)
    pushToSession(sess, 'info', 'Connection closed.')
    wsMap.delete(sess.id)
  }
}

function closeSession(i: number) {
  const sess = sessions.value[i]
  if (sess) {
    const ws = wsMap.get(sess.id)
    if (ws) { ws.close(); wsMap.delete(sess.id) }
    connectedIds.value = connectedIds.value.filter(id => id !== sess.id)
  }
  sessions.value.splice(i, 1)
  activeSession.value = Math.max(0, Math.min(i, sessions.value.length - 1))
  activeTarget.value  = sessions.value[activeSession.value]
    ? { id: '', name: sessions.value[activeSession.value]!.name, kind: sessions.value[activeSession.value]!.kind, node: sessions.value[activeSession.value]!.node, image: sessions.value[activeSession.value]!.image }
    : null
}

function clearOutput() {
  if (currentSession.value) currentSession.value.output = []
}

const ansiColors: Record<string, string> = {
  '30': '#21222c', '31': '#ff5555', '32': '#50fa7b', '33': '#f1fa8c',
  '34': '#bd93f9', '35': '#ff79c6', '36': '#8be9fd', '37': '#f8f8f2',
  '90': '#6272a4', '91': '#ffb86c', '92': '#50fa7b', '93': '#f1fa8c',
  '94': '#bd93f9', '95': '#ff79c6', '96': '#8be9fd', '97': '#ffffff'
}

function ansiToHtml(str: string) {
  let res = str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  let spanOpen = false

  // SGR (Select Graphic Rendition) for colors and styles
  res = res.replace(/\x1b\[([0-9;]*)m/g, (_, codes) => {
    let out = spanOpen ? '</span>' : ''
    spanOpen = false

    // 0, empty, or just 'm' resets all attributes
    if (codes === '0' || codes === '' || codes === 'm') return out

    const parts = codes.split(';')
    let styles = []
    for (const c of parts) {
      if (ansiColors[c]) styles.push(`color: ${ansiColors[c]}`)
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

  // Strip other unsupported escape sequences (cursor movement, etc.)
  res = res.replace(/\x1b\[[?0-9;]*[a-zA-Z]/g, '').replace(/[\x00-\x08\x0b-\x1f\x7f]/g, '')
  if (spanOpen) res += '</span>'
  return res
}

function pushToSession(sess: Session, type: TermLine['type'], text: string) {
  // Always mutate through the reactive proxy so Vue tracks the change
  const target = sessions.value.find(s => s.id === sess.id)
  if (target) {
    target.output.push({ 
      type, text,
      cwd: type === 'cmd' ? sess.cwd : undefined,
      promptChar: type === 'cmd' ? sess.promptChar : undefined
    })
  }
  nextTick(() => { if (outputEl.value) outputEl.value.scrollTop = outputEl.value.scrollHeight })
}

// ── Input ──────────────────────────────────────────────────────────────────
const inputLine   = ref('')
const cursorPos   = ref(0)
const focused     = ref(false)
const outputEl    = ref<HTMLElement | null>(null)
const hiddenInput = ref<HTMLInputElement | null>(null)
const history     = ref<string[]>([])
const historyIdx  = ref(-1)

const inputBefore = computed(() => inputLine.value.slice(0, cursorPos.value))
const inputAfter  = computed(() => inputLine.value.slice(cursorPos.value))

watch(inputLine, v => { cursorPos.value = v.length })

function focusInput() { hiddenInput.value?.focus() }

function onKey(e: KeyboardEvent) {
  if (!currentSession.value || !isConnected.value) return
  const ws = wsMap.get(currentSession.value.id)

  if (e.key === 'Enter') {
    const cmd = inputLine.value

    // Handle 'clear' command locally to wipe scrollback history
    if (cmd.trim() === 'clear') {
      clearOutput()
      lastSentCmd.value = 'clear'
      ws?.send('clear\n')
      inputLine.value = ''
      cursorPos.value = 0
      e.preventDefault()
      return
    }
    
    // Track what we sent so we can identify the echo coming back
    lastSentCmd.value = cmd
    
    if (cmd.trim()) { history.value.unshift(cmd); historyIdx.value = -1 }
    
    ws?.send(cmd + '\n')
    inputLine.value = ''
    cursorPos.value = 0
    e.preventDefault()
  } else if (e.key === 'ArrowUp') {
    historyIdx.value = Math.min(historyIdx.value + 1, history.value.length - 1)
    inputLine.value  = history.value[historyIdx.value] ?? ''
    e.preventDefault()
  } else if (e.key === 'ArrowDown') {
    historyIdx.value = Math.max(historyIdx.value - 1, -1)
    inputLine.value  = historyIdx.value >= 0 ? (history.value[historyIdx.value] ?? '') : ''
    e.preventDefault()
  } else if (e.key === 'l' && e.ctrlKey) {
    clearOutput(); e.preventDefault()
  } else if (e.key === 'c' && e.ctrlKey) {
    ws?.send('\x03')
    inputLine.value = ''
    e.preventDefault()
  }
}

function escHtml(s: string) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

onMounted(() => {
  loadContainers()
})

onUnmounted(() => {
  for (const ws of wsMap.values()) ws.close()
  wsMap.clear()
})
</script>

<style scoped>
.terminal-page {
  display: flex;
  height: 100%;
  overflow: hidden;
}

/* ── Picker ─────────────────────────────────────────────────────────────── */
.picker {
  width: 240px;
  flex-shrink: 0;
  background: var(--bg-surface);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.picker__header {
  padding: 0.75rem;
  border-bottom: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.search-wrap { position: relative; display: flex; align-items: center; }
.search-icon { position: absolute; left: 0.5rem; color: var(--text-muted); pointer-events: none; }
.search-input {
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.35rem 0.5rem 0.35rem 2rem;
  font-size: 0.78rem;
  color: var(--text-primary);
  width: 100%;
  outline: none;
}
.search-input::placeholder { color: var(--text-dim); }
.search-input:focus { border-color: var(--accent); }

.picker__tabs { display: flex; gap: 0.25rem; }
.ptab {
  flex: 1;
  display: inline-flex; align-items: center; justify-content: center;
  gap: 0.3rem; padding: 0.3rem 0.5rem;
  font-size: 0.72rem; color: var(--text-muted);
  background: none; border: 1px solid var(--border-strong);
  border-radius: 0.3rem; cursor: pointer; transition: all 0.15s;
}
.ptab:hover { color: var(--text-primary); }
.ptab--active { color: var(--accent-light); border-color: var(--accent-soft); background: var(--accent-dim); }

.picker__list { flex: 1; overflow-y: auto; padding: 0.5rem 0; }
.picker__hint { padding: 0.75rem 0.875rem; font-size: 0.78rem; color: var(--text-dim); }
.picker__group-label {
  padding: 0.4rem 0.875rem 0.2rem;
  font-size: 0.68rem; color: var(--text-dim);
  text-transform: uppercase; letter-spacing: 0.06em;
}
.picker__item {
  display: flex; flex-direction: column; align-items: flex-start;
  gap: 0.1rem; padding: 0.45rem 0.875rem;
  width: 100%; background: none; border: none;
  cursor: pointer; text-align: left; transition: background 0.12s;
  position: relative; padding-left: 1.75rem;
}
.picker__item:disabled { opacity: 0.45; cursor: not-allowed; }
.picker__item:not(:disabled):hover { background: var(--hover-bg); }
.picker__item--active { background: var(--accent-bg) !important; }
.picker__item--active::before {
  content: '';
  position: absolute; left: 0; top: 4px; bottom: 4px;
  width: 2px; background: var(--accent); border-radius: 0 2px 2px 0;
}

.picker__dot {
  position: absolute; left: 0.75rem; top: 50%;
  transform: translateY(-50%); width: 6px; height: 6px; border-radius: 50%;
}
.dot--running { background: var(--success); }
.dot--stopped { background: var(--text-dim); }

.picker__name { font-size: 0.8rem; color: var(--text-primary); font-weight: 500; }
.picker__sub  { font-size: 0.68rem; color: var(--text-dim); font-family: monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 180px; }

/* ── Terminal area ──────────────────────────────────────────────────────── */
.terminal-area {
  flex: 1; display: flex; flex-direction: column;
  overflow: hidden; background: var(--bg-deep);
}

.session-bar {
  display: flex; align-items: center; justify-content: space-between;
  background: var(--bg-surface); border-bottom: 1px solid var(--border);
  padding: 0 0.75rem; flex-shrink: 0;
}
.session-tabs { display: flex; align-items: center; gap: 0.25rem; }
.session-tab {
  display: inline-flex; align-items: center; gap: 0.35rem;
  padding: 0.5rem 0.75rem; font-size: 0.78rem; color: var(--text-muted);
  background: none; border: none; border-bottom: 2px solid transparent;
  cursor: pointer; transition: color 0.15s; margin-bottom: -1px;
}
.session-tab:hover { color: var(--text-primary); }
.session-tab--active { color: var(--accent-light); border-bottom-color: var(--accent); }
.tab-close {
  background: none; border: none; color: var(--text-dim); cursor: pointer;
  display: flex; align-items: center; padding: 0.1rem; border-radius: 0.2rem;
}
.tab-close:hover { color: var(--danger-light); }

.session-controls { display: flex; align-items: center; gap: 0.625rem; }
.session-meta { display: flex; align-items: center; gap: 0.3rem; font-size: 0.75rem; color: var(--text-dim); }
.meta-icon { color: var(--text-dim); }
.ctrl-btn {
  display: inline-flex; align-items: center; gap: 0.3rem;
  background: none; border: 1px solid var(--border-strong); border-radius: 0.25rem;
  padding: 0.25rem 0.5rem; font-size: 0.72rem; color: var(--text-muted);
  cursor: pointer; transition: all 0.15s;
}
.ctrl-btn:hover { color: var(--danger-light); border-color: var(--danger-border); }

/* ── Terminal ───────────────────────────────────────────────────────────── */
.term-wrap { flex: 1; overflow: hidden; cursor: text; }
.term-output {
  height: 100%; overflow-y: auto;
  padding: 0.5rem 1.25rem 1rem;
  display: flex; flex-direction: column; gap: 0.1rem;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 0.8rem; line-height: 1.55;
}
.term-output::-webkit-scrollbar { width: 5px; }
.term-output::-webkit-scrollbar-thumb { background: var(--border-strong); border-radius: 3px; }

.term-banner {
  display: flex; align-items: center; gap: 0.4rem;
  padding: 0.5rem 0 0.625rem; font-size: 0.72rem; color: var(--text-dim);
  border-bottom: 1px solid var(--border); margin-bottom: 0.5rem;
}
.banner-icon   { color: var(--text-dim); }
.banner-target { color: var(--accent-light); font-weight: 600; }
.banner-sep    { color: var(--border-strong); }
.banner-node   { color: var(--text-muted); }

.term-line { display: flex; align-items: baseline; gap: 0.5rem; }
.term-line--out  .term-text { color: var(--text-tertiary); }
.term-line--err  .term-text { color: var(--danger-light); }
.term-line--info .term-text { color: var(--text-dim); font-style: italic; }
.term-line--cmd  .term-text { color: var(--text-primary); }

.term-prompt { display: inline-flex; align-items: baseline; gap: 0.1rem; flex-shrink: 0; }
.prompt-target { color: var(--accent-light); font-weight: 600; }
.prompt-sep    { color: var(--text-dim); }
.prompt-dir    { color: var(--success); }
.prompt-dollar { color: var(--text-primary); margin-left: 0.2rem; }

.term-input-wrap { display: inline-flex; align-items: baseline; color: var(--text-primary); }
.term-input-text { white-space: pre; }
.term-cursor {
  display: inline-block; width: 0.55ch; height: 1.1em;
  background: var(--accent-light); vertical-align: text-bottom; border-radius: 1px;
}
.term-cursor--blink { animation: blink 1.1s step-end infinite; }
@keyframes blink { 0%, 100% { opacity: 1; } 50% { opacity: 0; } }
.term-hidden-input { position: absolute; opacity: 0; pointer-events: none; width: 1px; height: 1px; }

/* ── Empty state ────────────────────────────────────────────────────────── */
.term-empty {
  flex: 1; display: flex; flex-direction: column;
  align-items: center; justify-content: center;
  gap: 1rem; color: var(--text-dim);
}
.empty-icon { color: var(--border-strong); }
.term-empty p { font-size: 0.85rem; text-align: center; max-width: 300px; }

/* ── v-html colour helpers ──────────────────────────────────────────────── */
:deep(.c-purple) { color: var(--accent-light); }
:deep(.c-blue)   { color: #60a5fa; }
:deep(.c-mono)   { font-family: monospace; color: var(--text-tertiary); }
</style>
