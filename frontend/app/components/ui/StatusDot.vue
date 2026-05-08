<template>
  <span class="status-dot" :class="`status-dot--${status}`" />
</template>

<script setup lang="ts">
export type NodeStatus = 'healthy' | 'degraded' | 'offline' | 'pending'
defineProps<{ status: NodeStatus }>()
</script>

<style scoped>
.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.status-dot--healthy  { background: var(--success); box-shadow: 0 0 6px var(--success-glow); }
.status-dot--degraded { background: var(--warning); box-shadow: 0 0 6px var(--warning); }
.status-dot--offline  { background: var(--danger); }
.status-dot--pending  { background: var(--text-tertiary); }
</style>
