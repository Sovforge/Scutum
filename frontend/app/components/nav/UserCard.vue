<template>
  <div ref="root" class="user-card">
    <button class="user-card__trigger" @click="menuOpen = !menuOpen">
      <span class="user-card__avatar">{{ initials }}</span>
      <div class="user-card__info">
        <span class="user-card__name">{{ name }}</span>
        <span class="user-card__role">{{ role }}</span>
      </div>
      <Icon
        name="lucide:chevron-down"
        size="14"
        class="user-card__chevron"
        :class="{ 'user-card__chevron--open': menuOpen }"
      />
    </button>

    <Transition name="dropdown">
      <div v-if="menuOpen" class="user-card__menu">
        <div class="user-card__menu-header">
          <span class="user-card__menu-name">{{ name }}</span>
          <span class="user-card__menu-email">{{ email }}</span>
        </div>
        <div class="user-card__menu-divider" />
        <NuxtLink to="/account" class="user-card__menu-item" @click="menuOpen = false">
          <Icon name="lucide:user" size="14" />
          My Account
        </NuxtLink>
        <div class="user-card__menu-divider" />
        <button class="user-card__menu-item user-card__menu-item--danger" @click="logout">
          <Icon name="lucide:log-out" size="14" />
          Logout
        </button>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { onClickOutside } from '@vueuse/core'

const name  = 'Andreas'
const role  = 'Administrator'
const email = 'admin@scutum.local'

const initials = computed(() =>
  name.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2)
)

const root     = ref<HTMLElement | null>(null)
const menuOpen = ref(false)

onClickOutside(root, () => { menuOpen.value = false })

function logout() {
  menuOpen.value = false
  navigateTo('/auth/login')
}
</script>

<style scoped>
.user-card {
  position: relative;
}

/* ── Trigger ────────────────────────────────────────────────────────────── */
.user-card__trigger {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  background: none;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  padding: 0.35rem 0.5rem 0.35rem 0.35rem;
  cursor: pointer;
  font-family: inherit;
  transition: background 0.15s, border-color 0.15s;
}
.user-card__trigger:hover {
  background: var(--border);
  border-color: var(--border-strong);
}

/* ── Avatar ─────────────────────────────────────────────────────────────── */
.user-card__avatar {
  width: 28px;
  height: 28px;
  border-radius: 0.375rem;
  background: linear-gradient(135deg, var(--accent), var(--accent-secondary));
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.7rem;
  font-weight: 700;
  color: #fff;
  flex-shrink: 0;
  letter-spacing: 0.03em;
}

/* ── Info ───────────────────────────────────────────────────────────────── */
.user-card__info {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.05rem;
}
.user-card__name {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-primary);
  line-height: 1;
}
.user-card__role {
  font-size: 0.68rem;
  color: var(--text-dim);
  line-height: 1;
}

/* ── Chevron ────────────────────────────────────────────────────────────── */
.user-card__chevron {
  color: var(--text-dim);
  transition: transform 0.2s;
  flex-shrink: 0;
}
.user-card__chevron--open { transform: rotate(180deg); }

/* ── Dropdown menu ──────────────────────────────────────────────────────── */
.user-card__menu {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  width: 210px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.5rem;
  box-shadow: 0 8px 32px rgba(0,0,0,0.5);
  z-index: 100;
  overflow: hidden;
}

.user-card__menu-header {
  padding: 0.75rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}
.user-card__menu-name {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-primary);
}
.user-card__menu-email {
  font-size: 0.72rem;
  color: var(--text-dim);
}

.user-card__menu-divider {
  height: 1px;
  background: var(--border);
  margin: 0.25rem 0;
}

.user-card__menu-item {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.55rem 1rem;
  font-size: 0.8rem;
  color: var(--text-tertiary);
  text-decoration: none;
  background: none;
  border: none;
  width: 100%;
  text-align: left;
  cursor: pointer;
  font-family: inherit;
  transition: color 0.15s, background 0.15s;
}
.user-card__menu-item:hover {
  color: var(--text-primary);
  background: var(--border);
}
.user-card__menu-item--danger { color: var(--danger-light); }
.user-card__menu-item--danger:hover { background: var(--danger-bg); color: var(--danger-lighter); }

/* ── Transition ─────────────────────────────────────────────────────────── */
.dropdown-enter-active,
.dropdown-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
