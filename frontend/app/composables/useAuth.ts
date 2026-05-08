const TOKEN_KEY = 'auth_token'

export function useAuth() {
  const token = useCookie<string | null>(TOKEN_KEY, {
    maxAge: 60 * 60 * 24,
    sameSite: 'strict',
    secure: false,
  })

  function setToken(t: string) { token.value = t }
  function clearToken()        { token.value = null }
  function isAuthenticated()   { return !!token.value }
  function getToken()          { return token.value ?? '' }

  return { token, setToken, clearToken, isAuthenticated, getToken }
}
