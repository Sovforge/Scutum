import type { ComponentPropsWithoutRef, ElementType } from 'react'

type OwnProps<T extends ElementType> = {
  as?: T
  variant?: 'primary' | 'ghost'
  block?: boolean
}

type ButtonProps<T extends ElementType> = OwnProps<T> &
  Omit<ComponentPropsWithoutRef<T>, keyof OwnProps<T>>

// Renders as a native <button> by default, or any other element/component
// (typically react-router's <Link>) via the `as` prop — e.g.
// <Button as={Link} to="/setup" variant="primary">Get Started</Button>
function Button<T extends ElementType = 'button'>({
  as,
  variant = 'primary',
  block = false,
  className,
  ...rest
}: ButtonProps<T>) {
  const Component = as || 'button'
  const classes = [
    'btn',
    variant === 'primary' ? 'btn--primary' : 'btn--ghost',
    block ? 'btn--block' : '',
    className,
  ]
    .filter(Boolean)
    .join(' ')

  return <Component className={classes} {...rest} />
}

export default Button
