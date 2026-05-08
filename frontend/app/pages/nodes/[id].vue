<template>
  <div class="node-detail">

    <div v-if="loading" class="loading-state">Loading…</div>
    <div v-else-if="notFound" class="empty-state">Node not found.</div>

    <div v-else class="node-content">

      <!-- Back + header -->
      <div class="page-header">
        <NuxtLink to="/nodes" class="back-link">
          <Icon name="lucide:arrow-left" size="14" />
          Nodes
        </NuxtLink>
        <div class="page-header__title">
          <UiStatusDot status="healthy" />
          <h2 class="page-header__name">{{ node.name }}</h2>
          <UiBadge variant="info">{{ node.role }}</UiBadge>
          <UiBadge variant="success">registered</UiBadge>
        </div>
        <div class="page-header__actions">
          <button class="action-btn action-btn--ghost">
            <Icon name="lucide:refresh-cw" size="14" />
            Sync
          </button>
          <button class="action-btn action-btn--danger">
            <Icon name="lucide:unplug" size="14" />
            Disconnect
          </button>
        </div>
      </div>

      <!-- Info grid -->
      <div class="info-grid">

        <UiCard title="Identity">
          <dl class="info-list">
            <div class="info-list__row">
              <dt>Hostname</dt>
              <dd>{{ node.name }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Node ID</dt>
              <dd class="mono">{{ node.id }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Role</dt>
              <dd><UiBadge variant="info">{{ node.role }}</UiBadge></dd>
            </div>
            <div class="info-list__row">
              <dt>OS</dt>
              <dd>{{ node.os }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Arch</dt>
              <dd>{{ node.arch }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Agent version</dt>
              <dd class="mono">{{ node.agentVersion }}</dd>
            </div>
          </dl>
        </UiCard>

        <UiCard title="WireGuard">
          <dl class="info-list">
            <div class="info-list__row">
              <dt>Endpoint</dt>
              <dd class="mono">{{ node.endpoint }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Mesh IP</dt>
              <dd class="mono">{{ node.meshIp }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Public key</dt>
              <dd class="mono key-full">{{ node.pubkey }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Listen port</dt>
              <dd class="mono">51820</dd>
            </div>
            <div class="info-list__row">
              <dt>Last handshake</dt>
              <dd>{{ node.lastHandshake }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Latency</dt>
              <dd class="mono">{{ node.latency }}</dd>
            </div>
          </dl>
        </UiCard>

        <UiCard title="Traffic">
          <dl class="info-list">
            <div class="info-list__row">
              <dt>Received</dt>
              <dd class="mono">{{ node.rxBytes }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Transmitted</dt>
              <dd class="mono">{{ node.txBytes }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Packets in</dt>
              <dd class="mono">{{ node.rxPackets }}</dd>
            </div>
            <div class="info-list__row">
              <dt>Packets out</dt>
              <dd class="mono">{{ node.txPackets }}</dd>
            </div>
          </dl>
        </UiCard>

      </div>

    </div>

  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api   = useApi()
const route = useRoute()

const loading  = ref(true)
const notFound = ref(false)
const raw      = ref<NodeRecord | null>(null)

onMounted(async () => {
  try {
    raw.value = await api.getNode(route.params.id as string)
  } catch {
    notFound.value = true
  } finally {
    loading.value = false
  }
})

const node = computed(() => ({
  id:            raw.value?.id            ?? '',
  name:          raw.value?.name          ?? '',
  role:          raw.value?.type          ?? '',
  endpoint:      raw.value?.address       ?? '—',
  pubkey:        raw.value?.public_key    ?? '—',
  os:            '—',
  arch:          '—',
  agentVersion:  '—',
  meshIp:        '—',
  latency:       '—',
  lastHandshake: '—',
  rxBytes:       '—',
  txBytes:       '—',
  rxPackets:     '—',
  txPackets:     '—',
}))
</script>

<style scoped>
.node-detail { display: flex; flex-direction: column; gap: 1rem; }
.node-content { display: flex; flex-direction: column; gap: 1rem; }

/* Header */
.page-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}
.back-link {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.8rem;
  color: var(--text-dim);
  text-decoration: none;
  transition: color 0.15s;
}
.back-link:hover { color: var(--accent-light); }
.page-header__title {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  flex: 1;
}
.page-header__name {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--text-primary);
}
.page-header__actions { display: flex; gap: 0.5rem; margin-left: auto; }
.action-btn {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  border-radius: 0.375rem;
  padding: 0.35rem 0.75rem;
  font-size: 0.8rem;
  font-family: inherit;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.action-btn--ghost {
  background: none;
  border: 1px solid var(--border);
  color: var(--text-tertiary);
}
.action-btn--ghost:hover { background: var(--border); color: var(--text-primary); }
.action-btn--danger {
  background: none;
  border: 1px solid var(--danger-border);
  color: var(--danger-light);
}
.action-btn--danger:hover { background: var(--danger-bg); }

/* Info grid */
.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1rem;
}
.info-list { margin: 0; display: flex; flex-direction: column; gap: 0.1rem; }
.info-list__row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  padding: 0.45rem 0;
  border-bottom: 1px solid var(--border-subtle);
  gap: 1rem;
}
.info-list__row:last-child { border-bottom: none; }
dt { font-size: 0.75rem; color: var(--text-dim); white-space: nowrap; }
dd { margin: 0; font-size: 0.8rem; color: var(--text-secondary); text-align: right; }

.mono  { font-family: monospace; font-size: 0.75rem; color: var(--text-tertiary); }
.key-full { word-break: break-all; text-align: right; }
.muted { color: var(--text-dim); }

.loading-state { color: var(--text-dim); font-size: 0.875rem; padding: 2rem; text-align: center; }
.empty-state   { color: var(--text-dim); font-size: 0.875rem; padding: 2rem; text-align: center; }
</style>
