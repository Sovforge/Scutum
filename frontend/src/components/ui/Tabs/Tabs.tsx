import type { LucideIcon } from 'lucide-react'
import styles from './Tabs.module.css'

export type TabItem = { id: string; label: string; icon?: LucideIcon }

function Tabs({
  tabs,
  active,
  onChange,
  variant = 'primary',
}: {
  tabs: TabItem[]
  active: string
  onChange: (id: string) => void
  variant?: 'primary' | 'secondary'
}) {
  const isSecondary = variant === 'secondary'
  const barClass = isSecondary ? `${styles.bar} ${styles.barSecondary}` : styles.bar
  const activeClass = isSecondary ? styles.tabActiveSecondary : styles.tabActivePrimary

  return (
    <div className={barClass}>
      {tabs.map((t) => {
        const Icon = t.icon
        const isActive = t.id === active
        const classes = [styles.tab, isSecondary && styles.tabSecondary, isActive && activeClass].filter(Boolean).join(' ')
        return (
          <button key={t.id} type="button" className={classes} onClick={() => onChange(t.id)}>
            {Icon && <Icon size={isSecondary ? 14 : 15} />}
            {t.label}
          </button>
        )
      })}
    </div>
  )
}

export default Tabs
