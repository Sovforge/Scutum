<template>
  <div class="callback-page">
    <span class="callback-page__spinner" />
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'auth' })

const auth = useAuth()

onMounted(async () => {
  const params = new URLSearchParams(window.location.search)
  let token = params.get('token') ?? ''
  if (!token) {
    const hash = window.location.hash
    if (hash.startsWith('#sso-token=')) token = hash.slice('#sso-token='.length)
  }
  if (token) {
    history.replaceState(null, '', window.location.pathname)
    auth.setToken(token)
  }
  await navigateTo('/')
})
</script>

<style scoped>
.callback-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-base);
}
.callback-page__spinner {
  width: 28px;
  height: 28px;
  border: 3px solid var(--border-strong);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
