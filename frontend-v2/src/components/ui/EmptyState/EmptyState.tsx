import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import styles from './EmptyState.module.css'

function EmptyState({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon?: LucideIcon
  title: string
  description?: string
  action?: ReactNode
}) {
  return (
    <div className={styles.wrap}>
      {Icon && (
        <span className={styles.iconWrap}>
          <Icon size={22} />
        </span>
      )}
      <p className={styles.title}>{title}</p>
      {description && <p className={styles.description}>{description}</p>}
      {action && <div className={styles.action}>{action}</div>}
    </div>
  )
}

export default EmptyState
