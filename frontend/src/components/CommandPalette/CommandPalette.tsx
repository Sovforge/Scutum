import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { LogOut, MoonStar, Sun, Tv, User, type LucideIcon } from 'lucide-react'
import { NAV_ITEMS } from '../Sidebar/Sidebar'
import styles from './CommandPalette.module.css'

type Group = 'Navigate' | 'Actions'
type Entry = { id: string; label: string; icon: LucideIcon; group: Group; run: () => void }

function CommandPalette({
  open,
  onClose,
  theme,
  onToggleTheme,
  onLogout,
}: {
  open: boolean
  onClose: () => void
  theme: 'light' | 'dark'
  onToggleTheme: () => void
  onLogout: () => void
}) {
  const navigate = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState(0)

  useEffect(() => {
    if (!open) return
    setQuery('')
    setSelected(0)
    const id = requestAnimationFrame(() => inputRef.current?.focus())
    return () => cancelAnimationFrame(id)
  }, [open])

  const entries = useMemo<Entry[]>(() => {
    const nav: Entry[] = NAV_ITEMS.filter((item) => item.to).map((item) => ({
      id: item.to!,
      label: item.label,
      icon: item.icon,
      group: 'Navigate',
      run: () => navigate(item.to!),
    }))
    nav.push({ id: '/tv', label: 'TV Mode', icon: Tv, group: 'Navigate', run: () => navigate('/tv') })
    nav.push({ id: '/account', label: 'Account', icon: User, group: 'Navigate', run: () => navigate('/account') })

    const actions: Entry[] = [
      {
        id: 'theme',
        label: theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode',
        icon: theme === 'dark' ? Sun : MoonStar,
        group: 'Actions',
        run: onToggleTheme,
      },
      { id: 'logout', label: 'Log out', icon: LogOut, group: 'Actions', run: onLogout },
    ]
    return [...nav, ...actions]
  }, [navigate, theme, onToggleTheme, onLogout])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return entries
    return entries.filter((entry) => entry.label.toLowerCase().includes(q))
  }, [entries, query])

  function execute(entry?: Entry) {
    if (!entry) return
    entry.run()
    onClose()
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelected((s) => Math.min(s + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelected((s) => Math.max(s - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      execute(filtered[selected])
    } else if (e.key === 'Escape') {
      onClose()
    }
  }

  if (!open) return null

  let rowIndex = -1

  return (
    <div className={styles.backdrop} onClick={onClose}>
      <div className={styles.panel} onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="Command palette">
        <div className={styles.bar}>
          <span className={styles.dot} />
          <span className={styles.dot} />
          <span className={styles.dot} />
          <span className={styles.barLabel}>SCUTUM // COMMAND</span>
        </div>

        <div className={styles.promptRow}>
          <span className={styles.prompt}>&gt;</span>
          <input
            ref={inputRef}
            className={styles.input}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setSelected(0)
            }}
            onKeyDown={onKeyDown}
            placeholder="jump to a page, or run a command…"
            autoComplete="off"
            spellCheck={false}
          />
          <kbd className={styles.esc}>ESC</kbd>
        </div>

        <div className={styles.results}>
          {(['Navigate', 'Actions'] as const).map((group) => {
            const groupEntries = filtered.filter((entry) => entry.group === group)
            if (groupEntries.length === 0) return null
            return (
              <div key={group}>
                <div className={styles.groupLabel}>{group}</div>
                {groupEntries.map((entry) => {
                  rowIndex += 1
                  const idx = rowIndex
                  const Icon = entry.icon
                  return (
                    <button
                      type="button"
                      key={entry.id}
                      className={idx === selected ? `${styles.result} ${styles.resultActive}` : styles.result}
                      onMouseEnter={() => setSelected(idx)}
                      onClick={() => execute(entry)}
                    >
                      <Icon size={15} />
                      <span>{entry.label}</span>
                    </button>
                  )
                })}
              </div>
            )
          })}
          {filtered.length === 0 && <div className={styles.noResults}>No matches.</div>}
        </div>
      </div>
    </div>
  )
}

export default CommandPalette
