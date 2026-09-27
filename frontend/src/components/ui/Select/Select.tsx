import type { ComponentPropsWithoutRef, ReactNode } from 'react'
import styles from './Select.module.css'

type Option = { value: string; label: string }

type SelectProps = ComponentPropsWithoutRef<'select'> & {
  label: ReactNode
  id: string
  options: readonly Option[]
  hint?: ReactNode
}

function Select({ label, id, options, hint, ...rest }: SelectProps) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <select id={id} {...rest}>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
      {hint && <p className={styles.hint}>{hint}</p>}
    </div>
  )
}

export default Select
