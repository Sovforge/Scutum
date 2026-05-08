<template>
  <nav class="sidebar" :class="{ 'sidebar--open': open }">

    <!-- Logo -->
    <div class="sidebar__logo">
      <span class="sidebar__icon-wrap">
        <img src="/logo.svg" class="sidebar__logo-img" alt="Scutum" />
      </span>
      <span class="sidebar__label sidebar__logo-name">Scutum</span>
    </div>

    <!-- Nav items -->
    <ul class="sidebar__nav">
      <li v-for="item in navItems" :key="item.to">
        <NuxtLink
          :to="item.to"
          class="sidebar__link"
          active-class="sidebar__link--active"
          :title="!open ? item.label : undefined"
        >
          <span class="sidebar__icon-wrap">
            <Icon :name="item.icon" size="16" />
          </span>
          <span class="sidebar__label">{{ item.label }}</span>
        </NuxtLink>
      </li>
    </ul>

    <!-- Footer -->
    <div class="sidebar__footer">
      <NuxtLink
        v-if="isAdmin"
        to="/audit"
        class="sidebar__link"
        active-class="sidebar__link--active"
        :title="!open ? 'Audit Log' : undefined"
      >
        <span class="sidebar__icon-wrap">
          <Icon name="lucide:shield-alert" size="16" />
        </span>
        <span class="sidebar__label">Audit Log</span>
      </NuxtLink>
      <NuxtLink
        to="/settings"
        class="sidebar__link"
        active-class="sidebar__link--active"
        :title="!open ? 'Settings' : undefined"
      >
        <span class="sidebar__icon-wrap">
          <Icon name="lucide:settings" size="16" />
        </span>
        <span class="sidebar__label">Settings</span>
      </NuxtLink>

      <!-- Version chip -->
      <div class="sidebar__version sidebar__label" :title="`build ${version.build} · ${version.commit}`">
        {{ version.version }}
      </div>
    </div>

  </nav>
</template>

<script setup lang="ts">
const { open } = useSidebar()
const { getVersion } = useApi()
const { isAdmin, loadIfNeeded } = useCurrentUser()

const version = ref({ version: '…', build: '', commit: '' })
onMounted(async () => {
  version.value = await getVersion()
  await loadIfNeeded()
})

const navItems = [
  { to: '/',             icon: 'lucide:layout-dashboard', label: 'Dashboard'    },
  { to: '/nodes',        icon: 'lucide:server',           label: 'Nodes'        },
  { to: '/network',      icon: 'lucide:network',          label: 'Network'      },
  { to: '/containers',   icon: 'lucide:box',              label: 'Containers'   },
  { to: '/kubernetes',   icon: 'lucide:layers',           label: 'Kubernetes'   },
  { to: '/storage',      icon: 'lucide:hard-drive',       label: 'Storage'      },
  { to: '/observability',icon: 'lucide:activity',         label: 'Observability'},
  { to: '/terminal',     icon: 'lucide:terminal',         label: 'Terminal'     },
  { to: '/gitops',       icon: 'lucide:git-branch',       label: 'GitOps'       },
  { to: '/plugins',      icon: 'lucide:puzzle',           label: 'Plugins'      },
  { to: '/about',        icon: 'lucide:info',             label: 'About'        },
]
</script>

<style scoped>
/* ── Shell ─────────────────────────────────────────────────────────────── */
.sidebar {
  --rail: 56px;
  --full: 220px;

  width: var(--rail);
  flex-shrink: 0;
  background: var(--bg-surface);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: width 0.22s cubic-bezier(0.4, 0, 0.2, 1);
}
.sidebar--open { width: var(--full); }

/* ── Logo ──────────────────────────────────────────────────────────────── */
.sidebar__logo-img {
  width: 26px;
  height: 26px;
  object-fit: contain;
  flex-shrink: 0;
}
.sidebar__logo {
  display: flex;
  align-items: center;
  height: 52px;
  flex-shrink: 0;
  border-bottom: 1px solid var(--border);
  margin-bottom: 0.5rem;
}
.sidebar__logo-name {
  font-weight: 700;
  font-size: 1rem;
  color: var(--text-primary);
}

/* ── Nav ───────────────────────────────────────────────────────────────── */
.sidebar__nav {
  list-style: none;
  margin: 0;
  padding: 0;
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}

/* ── Links ─────────────────────────────────────────────────────────────── */
.sidebar__link {
  display: flex;
  align-items: center;
  width: 100%;
  color: var(--text-muted);
  text-decoration: none;
  font-size: 0.875rem;
  background: none;
  border: none;
  cursor: pointer;
  font-family: inherit;
  position: relative;
  transition: color 0.15s, background 0.15s;
  white-space: nowrap;
}
.sidebar__link::before {
  content: '';
  position: absolute;
  left: 0; top: 6px; bottom: 6px;
  width: 2px;
  background: transparent;
  border-radius: 0 2px 2px 0;
  transition: background 0.15s;
}
.sidebar__link:hover { color: var(--text-primary); background: var(--hover-bg); }
.sidebar__link--active { color: var(--accent-light); }
.sidebar__link--active::before { background: var(--accent); }

/* ── Icon cell ─────────────────────────────────────────────────────────── */
.sidebar__icon-wrap {
  width: var(--rail);
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

/* ── Label (hidden when collapsed) ────────────────────────────────────── */
.sidebar__label {
  overflow: hidden;
  opacity: 1;
  transition: opacity 0.15s;
  white-space: nowrap;
}
.sidebar:not(.sidebar--open) .sidebar__label { opacity: 0; }

/* ── Footer ────────────────────────────────────────────────────────────── */
.sidebar__footer {
  border-top: 1px solid var(--border);
  padding: 0.5rem 0;
  flex-shrink: 0;
}
.sidebar__version {
  font-size: 0.68rem;
  font-family: monospace;
  color: var(--text-dim);
  padding: 0.25rem 0 0.1rem var(--rail);
  margin-top: 0.125rem;
}
</style>
