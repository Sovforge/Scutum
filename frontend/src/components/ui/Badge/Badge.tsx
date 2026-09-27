import type { ReactNode } from 'react'
import styles from './Badge.module.css'

type Variant = 'success' | 'warning' | 'danger' | 'neutral'

function Badge({ variant = 'neutral', children }: { variant?: Variant; children: ReactNode }) {
  return <span className={`${styles.badge} ${styles[variant]}`}>{children}</span>
}

export default Badge
