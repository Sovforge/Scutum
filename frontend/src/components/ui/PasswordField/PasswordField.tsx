import { useState, type ComponentPropsWithoutRef, type ReactNode } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import styles from './PasswordField.module.css'

type PasswordFieldProps = Omit<ComponentPropsWithoutRef<'input'>, 'type'> & {
  label: ReactNode
  id: string
  hint?: ReactNode
}

function PasswordField({ label, id, hint, ...rest }: PasswordFieldProps) {
  const [show, setShow] = useState(false)

  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <div className={styles.wrap}>
        <input id={id} type={show ? 'text' : 'password'} {...rest} />
        <button
          type="button"
          className={styles.eye}
          onClick={() => setShow((v) => !v)}
          tabIndex={-1}
          aria-label={show ? 'Hide password' : 'Show password'}
        >
          {show ? <EyeOff size={14} /> : <Eye size={14} />}
        </button>
      </div>
      {hint}
    </div>
  )
}

export default PasswordField
