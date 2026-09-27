import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import styles from './AuthShell.module.css'

function AuthShell({
  eyebrow,
  title,
  subtitle,
  wide,
  children,
}: {
  eyebrow: string
  title: string
  subtitle?: string
  wide?: boolean
  children: ReactNode
}) {
  return (
    <div className={styles.shell}>
      <div className="grain" aria-hidden="true" />
      <header className={styles.nav}>
        <Link to="/" className="brand-mark">
          <img src="/logo.svg" alt="" className="brand-mark__logo" />
          <span className="brand-mark__word">SCUTUM</span>
        </Link>
      </header>
      <main className={styles.main}>
        <div className={wide ? `${styles.card} ${styles.cardWide}` : styles.card}>
          <p className="eyebrow">{eyebrow}</p>
          <h1 className={styles.title}>{title}</h1>
          {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
          {children}
        </div>
      </main>
    </div>
  )
}

export default AuthShell
