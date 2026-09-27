import type { ReactNode } from 'react'
import styles from './Section.module.css'

// `frame` adds blueprint-style corner registration marks — reserved for
// panels that hold a diagram/schematic (e.g. the mesh topology), not
// applied everywhere, so it stays a deliberate accent rather than noise.
function Section({
  title,
  action,
  frame = false,
  children,
}: {
  title?: string
  action?: ReactNode
  frame?: boolean
  children: ReactNode
}) {
  return (
    <div className={frame ? `${styles.section} ${styles.schematic}` : styles.section}>
      {frame && (
        <>
          <span className={`${styles.corner} ${styles.cornerTL}`} aria-hidden="true" />
          <span className={`${styles.corner} ${styles.cornerTR}`} aria-hidden="true" />
          <span className={`${styles.corner} ${styles.cornerBL}`} aria-hidden="true" />
          <span className={`${styles.corner} ${styles.cornerBR}`} aria-hidden="true" />
        </>
      )}
      {(title || action) && (
        <div className={styles.header}>
          {title ? <h2 className={styles.title}>{title}</h2> : <span aria-hidden="true" />}
          {action}
        </div>
      )}
      {children}
    </div>
  )
}

export default Section
