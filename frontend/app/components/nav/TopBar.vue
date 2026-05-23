<template>
  <header class="topbar">
    <div class="topbar__left">
      <button class="topbar__icon-btn" @click="toggle" aria-label="Toggle menu">
        <Icon :name="open ? 'lucide:panel-left-close' : 'lucide:panel-left-open'" size="18" />
      </button>
      <h1 class="topbar__title">{{ pageTitle }}</h1>
    </div>
    <div class="topbar__right">

      <!-- Node context selector -->
      <div class="node-select" ref="nodeSelectEl">
        <button class="node-select__btn" @click="nodeDropOpen = !nodeDropOpen">
          <Icon name="lucide:server" size="12" class="node-select__icon" />
          <span class="node-select__label">{{ nodesStore.selected?.name ?? 'Local' }}</span>
          <Icon name="lucide:chevron-down" size="11" class="node-select__chevron" :class="{ 'node-select__chevron--open': nodeDropOpen }" />
        </button>
        <div v-if="nodeDropOpen" class="node-select__dropdown">
          <button
            class="node-select__option"
            :class="{ 'node-select__option--active': nodesStore.selectedId === null }"
            @click="selectNode(null)"
          >
            <Icon name="lucide:home" size="12" />
            <span>Local (this hub)</span>
          </button>
          <div v-if="nodesStore.nodes.some(n => n.type !== 'hub')" class="node-select__divider" />
          <button
            v-for="n in nodesStore.nodes.filter(n => n.type !== 'hub')"
            :key="n.id"
            class="node-select__option"
            :class="{ 'node-select__option--active': nodesStore.selectedId === n.id }"
            @click="selectNode(n.id)"
          >
            <Icon name="lucide:server" size="12" />
            <span>{{ n.name }}</span>
            <span class="node-select__role">{{ n.type }}</span>
          </button>
        </div>
      </div>

      <!-- Mesh status pill -->
      <NuxtLink to="/network" class="mesh-pill" :class="`mesh-pill--${meshStatus}`" :title="`${mesh.healthy}/${mesh.total} nodes reachable`">
        <span class="mesh-pill__dot" :class="`mesh-pill__dot--${meshStatus}`" />
        <Icon name="lucide:network" size="12" class="mesh-pill__icon" />
        <span class="mesh-pill__count">{{ mesh.healthy }}/{{ mesh.total }}</span>
        <span class="mesh-pill__sep" />
        <span class="mesh-pill__label">{{ meshStatus }}</span>
      </NuxtLink>

      <!-- Theme toggle -->
      <button
        class="topbar__icon-btn topbar__theme-btn"
        @click="toggleTheme"
        :title="theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
      >
        <Icon :name="theme === 'dark' ? 'lucide:sun' : 'lucide:moon'" size="16" />
      </button>

      <NavUserCard />
    </div>
  </header>
</template>

<script setup lang="ts">
import { onClickOutside } from '@vueuse/core'

const route = useRoute()
const { open, toggle } = useSidebar()
const { theme, toggle: toggleTheme } = useTheme()

const titleMap: Record<string, string> = {
  '/':             'Dashboard',
  '/nodes':        'Nodes',
  '/network':      'Network',
  '/containers':   'Containers',
  '/kubernetes':   'Kubernetes',
  '/storage':      'Storage',
  '/observability':'Observability',
  '/terminal':     'Terminal',
  '/gitops':       'GitOps',
  '/settings':     'Settings',
  '/account':      'My Account',
  '/plugins':      'Plugins',
  '/about':        'About',
  '/setup':        'Setup',
}

const pageTitle = computed(() => {
  const base = '/' + route.path.split('/')[1]
  return titleMap[base] ?? 'Scutum'
})

const api        = useApi()
const nodesStore = useNodesStore()
const router     = useRouter()
const mesh = reactive({ healthy: 0, total: 0 })

// ── Node context selector ──────────────────────────────────────────────────
const nodeDropOpen = ref(false)
const nodeSelectEl = ref<HTMLElement | null>(null)

function selectNode(id: string | null) {
  nodesStore.select(id)
  nodeDropOpen.value = false
  router.go(0)
}

onClickOutside(nodeSelectEl, () => { nodeDropOpen.value = false })
const meshStatus = computed<'healthy' | 'degraded' | 'offline'>(() => {
  if (mesh.total === 0) return 'offline'
  if (mesh.healthy === mesh.total) return 'healthy'
  return 'degraded'
})

async function refreshMesh() {
  try {
    const s = await api.getMeshSummary()
    mesh.total   = s.total
    mesh.healthy = s.healthy
  } catch { /* WireGuard not running — leave zeroes */ }
}

let meshTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  refreshMesh()
  nodesStore.load()
  meshTimer = setInterval(refreshMesh, 30_000)
})
onUnmounted(() => clearInterval(meshTimer))
</script>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1.5rem;
  border-bottom: 1px solid var(--border);
  background: var(--bg-base);
  flex-shrink: 0;
}
.topbar__left {
  display: flex;
  align-items: center;
  gap: 0.875rem;
}
.topbar__icon-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted);
  padding: 0.3rem;
  border-radius: 0.375rem;
  display: flex;
  align-items: center;
  transition: color 0.15s, background 0.15s;
}
.topbar__icon-btn:hover { color: var(--text-primary); background: var(--hover-bg); }
.topbar__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}
.topbar__right {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}
.topbar__theme-btn { color: var(--text-dim); }
.topbar__theme-btn:hover { color: var(--accent-light); }

/* ── Node context selector ─────────────────────────────────────────────── */
.node-select { position: relative; }

.node-select__btn {
  display: inline-flex; align-items: center; gap: 0.35rem;
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 0.375rem; padding: 0.3rem 0.6rem;
  font-size: 0.75rem; color: var(--text-tertiary);
  cursor: pointer; transition: border-color 0.15s, color 0.15s;
  white-space: nowrap;
}
.node-select__btn:hover { border-color: var(--accent); color: var(--text-primary); }
.node-select__icon  { color: var(--accent-light); opacity: 0.7; flex-shrink: 0; }
.node-select__label { max-width: 120px; overflow: hidden; text-overflow: ellipsis; }
.node-select__chevron { color: var(--text-dim); transition: transform 0.15s; }
.node-select__chevron--open { transform: rotate(180deg); }

.node-select__dropdown {
  position: absolute; top: calc(100% + 0.375rem); right: 0; z-index: 100;
  background: var(--bg-surface); border: 1px solid var(--border-strong);
  border-radius: 0.5rem; padding: 0.25rem;
  min-width: 180px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.35);
}
.node-select__option {
  display: flex; align-items: center; gap: 0.5rem;
  width: 100%; padding: 0.45rem 0.625rem;
  background: none; border: none; border-radius: 0.375rem;
  font-size: 0.78rem; color: var(--text-tertiary);
  cursor: pointer; text-align: left;
  transition: background 0.12s, color 0.12s;
}
.node-select__option:hover { background: var(--hover-bg); color: var(--text-primary); }
.node-select__option--active { color: var(--accent-light); background: var(--accent-subtle); }
.node-select__role {
  margin-left: auto; font-size: 0.68rem;
  color: var(--text-dim); font-family: monospace;
}
.node-select__divider {
  height: 1px; background: var(--border); margin: 0.25rem 0;
}

/* ── Mesh pill ─────────────────────────────────────────────────────────── */
.mesh-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.25rem 0.625rem;
  border-radius: 9999px;
  border: 1px solid;
  font-size: 0.72rem;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  transition: opacity 0.15s, filter 0.15s;
  white-space: nowrap;
}
.mesh-pill:hover { filter: brightness(1.15); }

.mesh-pill--healthy  {
  background: rgba(34, 197, 94, 0.08);
  border-color: rgba(34, 197, 94, 0.25);
  color: #4ade80;
}
.mesh-pill--degraded {
  background: rgba(245, 158, 11, 0.08);
  border-color: rgba(245, 158, 11, 0.25);
  color: #fbbf24;
}
.mesh-pill--offline {
  background: rgba(239, 68, 68, 0.08);
  border-color: rgba(239, 68, 68, 0.25);
  color: #f87171;
}

.mesh-pill__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.mesh-pill__dot--healthy  {
  background: #22c55e;
  box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.6);
  animation: mesh-pulse 2s ease-in-out infinite;
}
.mesh-pill__dot--degraded { background: #f59e0b; }
.mesh-pill__dot--offline  { background: #ef4444; }

@keyframes mesh-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.5); }
  50%       { box-shadow: 0 0 0 4px rgba(34, 197, 94, 0); }
}

.mesh-pill__icon { opacity: 0.6; flex-shrink: 0; }
.mesh-pill__count { font-family: monospace; letter-spacing: -0.02em; }
.mesh-pill__sep {
  width: 1px;
  height: 10px;
  background: currentColor;
  opacity: 0.2;
  flex-shrink: 0;
}
.mesh-pill__label { opacity: 0.8; }
</style>
