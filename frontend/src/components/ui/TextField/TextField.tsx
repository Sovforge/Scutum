import type { ComponentPropsWithoutRef, ReactNode } from 'react'
import styles from './TextField.module.css'

type TextFieldProps = ComponentPropsWithoutRef<'input'> & {
  label: ReactNode
  id: string
  hint?: ReactNode
}

function TextField({ label, id, hint, ...rest }: TextFieldProps) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input id={id} {...rest} />
      {hint && <p className={styles.hint}>{hint}</p>}
    </div>
  )
}

export default TextField
