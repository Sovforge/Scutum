export type Theme = 'dark' | 'light'

export function useTheme() {
  const theme = useState<Theme>('theme', () => 'dark')

  function apply(t: Theme) {
    theme.value = t
    if (import.meta.client) {
      document.documentElement.setAttribute('data-theme', t)
      localStorage.setItem('scutum-theme', t)
    }
  }

  function toggle() {
    apply(theme.value === 'dark' ? 'light' : 'dark')
  }

  function init() {
    if (import.meta.client) {
      const saved = localStorage.getItem('scutum-theme') as Theme | null
      apply(saved ?? 'dark')
    }
  }

  return { theme, apply, toggle, init }
}
