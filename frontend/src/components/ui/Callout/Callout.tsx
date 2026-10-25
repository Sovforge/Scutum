import type { ReactNode } from 'react'
import { AlertCircle, CheckCircle2, Info, TriangleAlert, type LucideIcon } from 'lucide-react'
import styles from './Callout.module.css'

type Variant = 'info' | 'warning' | 'danger' | 'success'

const ICONS: Record<Variant, LucideIcon> = {
  info: Info,
  warning: TriangleAlert,
  danger: AlertCircle,
  success: CheckCircle2,
}

function Callout({ variant = 'info', children }: { variant?: Variant; children: ReactNode }) {
  const Icon = ICONS[variant]
  return (
    <div className={`${styles.callout} ${styles[variant]}`}>
      <Icon size={14} />
      <span>{children}</span>
    </div>
  )
}

export default Callout
