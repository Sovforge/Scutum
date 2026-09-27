import { useEffect, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { Menu, Search } from 'lucide-react'
import Sidebar from '../Sidebar/Sidebar'
import AccountCard from '../ui/AccountCard/AccountCard'
import CommandPalette from '../CommandPalette/CommandPalette'
import NodeSelect from './NodeSelect'
import { clearToken, getMe, getToken } from '../../lib/api'
import styles from './AppShell.module.css'

const THEME_KEY = 'scutum-app-theme'

function roleLabel(roles: string[]): string {
  if (roles.includes('admin')) return 'Administrator'
  const [first] = roles
  return first ? first[0].toUpperCase() + first.slice(1) : 'User'
}

function AppShell({
  title,
  accountName = 'admin',
  accountRole = 'Administrator',
  children,
}: {
  title: string
  accountName?: string
  accountRole?: string
  children: ReactNode
}) {
  const navigate = useNavigate()
  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [theme, setTheme] = useState<'light' | 'dark'>(() =>
    localStorage.getItem(THEME_KEY) === 'light' ? 'light' : 'dark',
  )
  const [me, setMe] = useState<{ name: string; role: string } | null>(null)

  // Every app-shell page requires a session — this is the one place that
  // check needs to live since every such page renders through here.
  useEffect(() => {
    if (!getToken()) {
      navigate('/login', { replace: true })
      return
    }
    getMe()
      .then((u) => setMe({ name: u.username, role: roleLabel(u.roles) }))
      .catch(() => {
        clearToken()
        navigate('/login', { replace: true })
      })
  }, [navigate])

  function toggleTheme() {
    setTheme((prev) => {
      const next = prev === 'dark' ? 'light' : 'dark'
      localStorage.setItem(THEME_KEY, next)
      return next
    })
  }

  function logout() {
    clearToken()
    navigate('/login')
  }

  // Cmd/Ctrl+K opens the command palette from anywhere in the app shell.
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPaletteOpen((v) => !v)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  return (
    // Theme is scoped to this subtree (not <html>) — the marketing pages
    // (Landing/Setup/Login/About/FAQ/TV) are dark-only by design and
    // shouldn't flip with the app's own light/dark toggle.
    <div className={styles.shell} data-theme={theme === 'light' ? 'light' : undefined}>
      <Sidebar
        collapsed={collapsed}
        onToggleCollapse={() => setCollapsed((c) => !c)}
        mobileOpen={mobileOpen}
        onCloseMobile={() => setMobileOpen(false)}
      />
      <div className={styles.content}>
        <header className={styles.topbar}>
          <button
            type="button"
            className={styles.menuBtn}
            onClick={() => setMobileOpen(true)}
            aria-label="Open menu"
          >
            <Menu size={20} />
          </button>
          <h1 className={styles.title}>{title}</h1>
          <button type="button" className={styles.searchBtn} onClick={() => setPaletteOpen(true)}>
            <Search size={14} />
            <span className={styles.searchBtnLabel}>Jump to…</span>
            <kbd className={styles.searchBtnKbd}>⌘K</kbd>
          </button>
          <NodeSelect />
          <AccountCard name={me?.name ?? accountName} role={me?.role ?? accountRole} theme={theme} onToggleTheme={toggleTheme} onLogout={logout} />
        </header>
        <main className={styles.main}>{children}</main>
      </div>
      <CommandPalette
        open={paletteOpen}
        onClose={() => setPaletteOpen(false)}
        theme={theme}
        onToggleTheme={toggleTheme}
        onLogout={logout}
      />
    </div>
  )
}

export default AppShell
