import type { CSSProperties, ReactNode } from 'react'
import styles from './StatCard.module.css'

function StatGrid({ children, minWidth = 190 }: { children: ReactNode; minWidth?: number }) {
  return (
    <div className={styles.grid} style={{ '--statMinWidth': `${minWidth}px` } as CSSProperties}>
      {children}
    </div>
  )
}

export default StatGrid
