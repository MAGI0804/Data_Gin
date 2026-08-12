import type { HTMLAttributes, ReactNode } from 'react'
import styles from './FilterToolbar.module.css'

export interface FilterToolbarProps extends HTMLAttributes<HTMLDivElement> {
  children?: ReactNode
  actions?: ReactNode
  summary?: ReactNode
  label?: string
}

export function FilterToolbar({
  actions,
  children,
  className,
  label = '筛选与列表操作',
  summary,
  ...props
}: FilterToolbarProps) {
  const classes = [styles.toolbar, className].filter(Boolean).join(' ')

  return (
    <div className={classes} role="region" aria-label={label} {...props}>
      {children ? <div className={styles.filters}>{children}</div> : null}
      {summary ? <div className={styles.summary}>{summary}</div> : null}
      {actions ? <div className={styles.actions}>{actions}</div> : null}
    </div>
  )
}
