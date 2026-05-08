// Global route guard: setup check → auth check
export default defineNuxtRouteMiddleware(async (to) => {
  const isSetupRoute = to.path.startsWith('/setup')
  const isAuthRoute  = to.path.startsWith('/auth')

  // ── 1. Setup gate ────────────────────────────────────────────────────────
  // Cache result in shared state so we only hit the API once per page load.
  const setupDone = useState('setup-complete', (): boolean | null => null)

  if (setupDone.value === null) {
    // Skip during SSG — the API isn't running at build time so any result
    // would be wrong. Leave the state as null so the browser re-evaluates.
    if (import.meta.server) return

    try {
      const { complete } = await useApi().setupStatus()
      setupDone.value = complete
    } catch {
      setupDone.value = false
    }
  }

  // Redirect to setup if not done yet.
  if (!setupDone.value && !isSetupRoute) return navigateTo('/setup')

  // Redirect away from setup if it's already been completed.
  if (setupDone.value && isSetupRoute) return navigateTo('/')

  // ── 2. Auth gate ─────────────────────────────────────────────────────────
  if (!isSetupRoute && !isAuthRoute) {
    const { isAuthenticated } = useAuth()
    if (!isAuthenticated()) return navigateTo('/auth/login')
  }
})
