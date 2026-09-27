import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, LogOut, Moon, Sun, User } from 'lucide-react'
import styles from './AccountCard.module.css'

function initials(name: string): string {
  const parts = name.trim().split(/\s+/)
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[1][0]).toUpperCase()
}

function AccountCard({
  name,
  role,
  theme,
  onToggleTheme,
  onLogout,
}: {
  name: string
  role?: string
  theme: 'light' | 'dark'
  onToggleTheme: () => void
  onLogout?: () => void
}) {
  const [open, setOpen] = useState(false)

  return (
    <div className={styles.wrap}>
      <button type="button" className={styles.trigger} onClick={() => setOpen((v) => !v)}>
        <span className={styles.avatar}>{initials(name)}</span>
        <span className={styles.info}>
          <span className={styles.name}>{name}</span>
          {role && <span className={styles.role}>{role}</span>}
        </span>
        <ChevronDown size={14} className={open ? styles.chevronOpen : styles.chevron} />
      </button>

      {open && (
        <>
          <div className={styles.scrim} onClick={() => setOpen(false)} />
          <div className={styles.menu}>
            <Link to="/account" className={styles.menuItem} onClick={() => setOpen(false)}>
              <User size={15} />
              <span className={styles.menuLabel}>Account</span>
            </Link>

            <button
              type="button"
              className={styles.menuItem}
              onClick={() => {
                onToggleTheme()
              }}
            >
              {theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}
              <span className={styles.menuLabel}>{theme === 'dark' ? 'Light mode' : 'Dark mode'}</span>
            </button>

            <div className={styles.menuDivider} />

            <button
              type="button"
              className={`${styles.menuItem} ${styles.menuItemDanger}`}
              onClick={() => {
                setOpen(false)
                onLogout?.()
              }}
            >
              <LogOut size={15} />
              <span className={styles.menuLabel}>Log out</span>
            </button>
          </div>
        </>
      )}
    </div>
  )
}

export default AccountCard
