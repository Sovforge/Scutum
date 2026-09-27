import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import styles from './StatCard.module.css'

// Only kicks in for plain numeric values (e.g. a node count) — strings like
// "3/5" or a pre-formatted byte size render as-is, unanimated.
function useCountUp(target: number, active: boolean, duration = 700) {
  const [display, setDisplay] = useState(active ? 0 : target)
  const frameRef = useRef<number>(0)

  useEffect(() => {
    if (!active) {
      setDisplay(target)
      return
    }
    const start = performance.now()
    function tick(now: number) {
      const t = Math.min((now - start) / duration, 1)
      const eased = 1 - (1 - t) ** 3
      setDisplay(target * eased)
      if (t < 1) frameRef.current = requestAnimationFrame(tick)
    }
    frameRef.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(frameRef.current)
  }, [target, active, duration])

  return display
}

function StatCard({
  label,
  value,
  sub,
  icon,
  color = 'var(--blueprint)',
  size = 'normal',
}: {
  label: ReactNode
  value: ReactNode
  sub?: ReactNode
  icon?: ReactNode
  color?: string
  size?: 'normal' | 'large'
}) {
  const isNumeric = typeof value === 'number'
  const animated = useCountUp(isNumeric ? value : 0, isNumeric)
  const displayValue = isNumeric ? Math.round(animated).toLocaleString() : value

  return (
    <div
      className={size === 'large' ? `${styles.stat} ${styles.statLarge}` : styles.stat}
      style={{ '--statColor': color } as CSSProperties}
    >
      <div className={styles.header}>
        <span className={styles.label}>{label}</span>
        {icon && <span className={styles.icon}>{icon}</span>}
      </div>
      <span className={styles.value}>{displayValue}</span>
      {sub && <span className={styles.sub}>{sub}</span>}
    </div>
  )
}

export default StatCard
