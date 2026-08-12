import type { HTMLAttributes, ReactNode } from 'react'
import styles from './PageHeader.module.css'

export interface PageHeaderProps extends Omit<HTMLAttributes<HTMLElement>, 'title'> {
  title: ReactNode
  description?: ReactNode
  eyebrow?: ReactNode
  actions?: ReactNode
  context?: ReactNode
}

export function PageHeader({
  actions,
  className,
  context,
  description,
  eyebrow,
  title,
  ...props
}: PageHeaderProps) {
  const classes = [styles.header, className].filter(Boolean).join(' ')

  return (
    <header className={classes} {...props}>
      <div className={styles.headingGroup}>
        {eyebrow ? <div className={styles.eyebrow}>{eyebrow}</div> : null}
        <h1 className={styles.title}>{title}</h1>
        {description ? <div className={styles.description}>{description}</div> : null}
      </div>
      {actions ? <div className={styles.actions}>{actions}</div> : null}
      {context ? <div className={styles.context}>{context}</div> : null}
    </header>
  )
}
