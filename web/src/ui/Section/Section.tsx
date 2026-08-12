import { useId, type HTMLAttributes, type ReactNode } from 'react'
import styles from './Section.module.css'

export interface SectionProps extends Omit<HTMLAttributes<HTMLElement>, 'title'> {
  title?: ReactNode
  description?: ReactNode
  actions?: ReactNode
  children: ReactNode
  flush?: boolean
}

export function Section({
  actions,
  children,
  className,
  description,
  flush = false,
  title,
  ...props
}: SectionProps) {
  const headingId = useId()
  const classes = [styles.section, flush && styles.flush, className].filter(Boolean).join(' ')

  return (
    <section className={classes} aria-labelledby={title ? headingId : undefined} {...props}>
      {title || description || actions ? (
        <div className={styles.header}>
          <div className={styles.headingGroup}>
            {title ? <h2 className={styles.title} id={headingId}>{title}</h2> : null}
            {description ? <div className={styles.description}>{description}</div> : null}
          </div>
          {actions ? <div className={styles.actions}>{actions}</div> : null}
        </div>
      ) : null}
      <div className={styles.content}>{children}</div>
    </section>
  )
}
