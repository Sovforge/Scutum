<template>
  <header class="topbar">
    <div class="topbar__left">
      <button class="topbar__icon-btn" @click="toggle" aria-label="Toggle menu">
        <Icon :name="open ? 'lucide:panel-left-close' : 'lucide:panel-left-open'" size="18" />
      </button>
      <h1 class="topbar__title">{{ pageTitle }}</h1>
    </div>
    <div class="topbar__right">

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

const api = useApi()
const mesh = reactive({ healthy: 0, total: 0 })
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
onMounted(() => { refreshMesh(); meshTimer = setInterval(refreshMesh, 30_000) })
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
