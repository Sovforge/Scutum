import type { ComponentPropsWithoutRef, ReactNode } from 'react'
import styles from './Checkbox.module.css'

type CheckboxProps = Omit<ComponentPropsWithoutRef<'input'>, 'type'> & {
  children: ReactNode
}

function Checkbox({ children, ...rest }: CheckboxProps) {
  return (
    <label className={styles.row}>
      <input type="checkbox" {...rest} />
      <span>{children}</span>
    </label>
  )
}

export default Checkbox
