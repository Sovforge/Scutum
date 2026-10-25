import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react'
import { CheckCircle2, Info, TriangleAlert, X } from 'lucide-react'
import styles from './Toast.module.css'

type ToastVariant = 'success' | 'info' | 'danger'
type ToastItem = { id: number; message: string; variant: ToastVariant }
type ToastFn = (message: string, variant?: ToastVariant) => void

const ToastContext = createContext<ToastFn | null>(null)

const ICONS: Record<ToastVariant, typeof Info> = {
  success: CheckCircle2,
  info: Info,
  danger: TriangleAlert,
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within a ToastProvider')
  return ctx
}

function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const idRef = useRef(0)

  const dismiss = useCallback((id: number) => {
    setToasts((t) => t.filter((toast) => toast.id !== id))
  }, [])

  const toast = useCallback<ToastFn>(
    (message, variant = 'success') => {
      const id = idRef.current++
      setToasts((t) => [...t, { id, message, variant }])
      setTimeout(() => dismiss(id), 4000)
    },
    [dismiss],
  )

  return (
    <ToastContext.Provider value={toast}>
      {children}
      <div className={styles.viewport} role="status" aria-live="polite">
        {toasts.map((t) => {
          const Icon = ICONS[t.variant]
          return (
            <div className={`${styles.toast} ${styles[t.variant]}`} key={t.id}>
              <Icon size={16} className={styles.icon} />
              <span className={styles.message}>{t.message}</span>
              <button type="button" className={styles.close} onClick={() => dismiss(t.id)} aria-label="Dismiss">
                <X size={14} />
              </button>
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}

export default ToastProvider
