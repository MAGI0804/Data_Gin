import type { ButtonHTMLAttributes } from 'react'
import styles from './Button.module.css'

export type ButtonVariant = 'default' | 'primary' | 'danger'

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant
}

export function Button({ className, variant = 'default', type = 'button', ...props }: ButtonProps) {
  const variantClassName = variant === 'default' ? '' : styles[variant]
  return <button {...props} type={type} className={[variantClassName, className].filter(Boolean).join(' ')} />
}
