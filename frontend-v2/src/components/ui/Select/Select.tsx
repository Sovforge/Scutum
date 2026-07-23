import type { ComponentPropsWithoutRef, ReactNode } from 'react'

type Option = { value: string; label: string }

type SelectProps = ComponentPropsWithoutRef<'select'> & {
  label: ReactNode
  id: string
  options: readonly Option[]
}

function Select({ label, id, options, ...rest }: SelectProps) {
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
    </div>
  )
}

export default Select
