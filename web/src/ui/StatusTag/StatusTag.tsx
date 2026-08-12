import type { HTMLAttributes, ReactNode } from 'react'
import styles from './StatusTag.module.css'

export type StatusTagTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger' | 'running'

export interface StatusTagProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: StatusTagTone
  children: ReactNode
  showDot?: boolean
}

export function StatusTag({
  children,
  className,
  showDot = true,
  tone = 'neutral',
  ...props
}: StatusTagProps) {
  const classes = [styles.tag, styles[tone], 'ui-pill-radius', className].filter(Boolean).join(' ')

  return (
    <span className={classes} {...props}>
      {showDot ? <span className={`${styles.dot} ui-pill-radius`} aria-hidden="true" /> : null}
      <span>{children}</span>
    </span>
  )
}
