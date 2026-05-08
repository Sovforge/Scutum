export function useCurrentUser() {
  const roles    = useState<string[]>('current-user-roles', () => [])
  const loaded   = useState<boolean>('current-user-loaded', () => false)
  const isAdmin  = computed(() => roles.value.includes('admin'))

  async function loadIfNeeded() {
    if (loaded.value) return
    try {
      const me = await useApi().getMe()
      roles.value  = me.roles ?? []
      loaded.value = true
    } catch { /* not authenticated yet */ }
  }

  function clear() { roles.value = []; loaded.value = false }

  return { roles, isAdmin, loadIfNeeded, clear }
}
