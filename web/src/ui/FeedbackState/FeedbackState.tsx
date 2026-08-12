import type { HTMLAttributes, ReactNode } from 'react'
import styles from './FeedbackState.module.css'

export type FeedbackStateKind = 'empty' | 'loading' | 'error'

export interface FeedbackStateProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  kind: FeedbackStateKind
  title: ReactNode
  description?: ReactNode
  action?: ReactNode
  icon?: ReactNode
}

export function FeedbackState({
  action,
  className,
  description,
  icon,
  kind,
  title,
  ...props
}: FeedbackStateProps) {
  const classes = [styles.state, styles[kind], className].filter(Boolean).join(' ')

  return (
    <div
      className={classes}
      role={kind === 'error' ? 'alert' : 'status'}
      aria-busy={kind === 'loading' || undefined}
      {...props}
    >
      <div className={styles.marker} aria-hidden="true">
        {icon ?? <span className={styles.fallbackMarker} />}
      </div>
      <div className={styles.copy}>
        <strong className={styles.title}>{title}</strong>
        {description ? <div className={styles.description}>{description}</div> : null}
      </div>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  )
}
