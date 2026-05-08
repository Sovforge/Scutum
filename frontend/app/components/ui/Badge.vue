<template>
  <span class="badge" :class="`badge--${variant}`">
    <slot />
  </span>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ variant?: 'success' | 'warning' | 'danger' | 'neutral' | 'info' }>(), {
  variant: 'neutral',
})
</script>

<style scoped>
.badge {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 500;
}
.badge--success { background: #14532d40; color: var(--success-light); }
.badge--warning { background: #78350f40; color: #fbbf24; }
.badge--danger  { background: #7f1d1d40; color: var(--danger-light); }
.badge--info    { background: #1e1b4b40; color: var(--accent-light); }
.badge--neutral { background: var(--role-gray-bg);   color: var(--text-tertiary); }
</style>
