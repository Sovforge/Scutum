<template>
  <div class="settings-page">
    <div class="settings-layout">

      <nav class="settings-nav">
        <NuxtLink to="/settings" class="settings-nav__item">
          <Icon name="lucide:sliders-horizontal" size="15" /> General
        </NuxtLink>
        <div class="settings-nav__divider" />
        <NuxtLink to="/settings/rbac"       class="settings-nav__item">
          <Icon name="lucide:shield" size="15" /> RBAC
        </NuxtLink>
        <NuxtLink to="/settings/secrets"    class="settings-nav__item">
          <Icon name="lucide:key-round" size="15" /> Secrets / KMS
        </NuxtLink>
        <NuxtLink to="/settings/federation" class="settings-nav__item">
          <Icon name="lucide:git-merge" size="15" /> Federation
        </NuxtLink>
        <NuxtLink to="/settings/webhooks"   class="settings-nav__item">
          <Icon name="lucide:webhook" size="15" /> Webhooks
        </NuxtLink>
        <NuxtLink to="/settings/scim"       class="settings-nav__item">
          <Icon name="lucide:users" size="15" /> SCIM
        </NuxtLink>
        <NuxtLink to="/settings/recovery"   class="settings-nav__item">
          <Icon name="lucide:usb" size="15" /> Recovery Keys
        </NuxtLink>
      </nav>

      <div class="settings-content">
        <slot />
      </div>

    </div>
  </div>
</template>

<style scoped>
.settings-page { width: 100%; }

.settings-layout {
  display: grid;
  grid-template-columns: 200px 1fr;
  gap: 1.5rem;
  align-items: start;
}

.settings-nav {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  position: sticky;
  top: 1rem;
}
.settings-nav__item {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  font-size: 0.82rem;
  color: var(--text-muted);
  cursor: pointer;
  background: none;
  border: none;
  text-align: left;
  text-decoration: none;
  transition: color 0.15s, background 0.15s;
  width: 100%;
}
.settings-nav__item:hover { color: var(--text-primary); background: var(--hover-bg); }
.router-link-active.settings-nav__item { color: var(--accent-light); background: var(--accent-dim); }
.settings-nav__divider { height: 1px; background: var(--border); margin: 0.25rem 0; }

.settings-content { min-width: 0; display: flex; flex-direction: column; gap: 1.25rem; }
</style>
