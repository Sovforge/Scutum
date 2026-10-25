import styles from './ResourceBar.module.css'

function resourceColor(pct: number) {
  if (pct >= 80) return 'var(--danger)'
  if (pct >= 50) return 'var(--rust-bright)'
  return 'var(--blueprint-bright)'
}

function ResourceBar({ pct, label }: { pct: number; label: string }) {
  return (
    <div className={styles.resource}>
      <div className={styles.track}>
        <div className={styles.fill} style={{ width: `${pct}%`, background: resourceColor(pct) }} />
      </div>
      <span className={styles.val}>{label}</span>
    </div>
  )
}

export default ResourceBar
