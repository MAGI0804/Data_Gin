import type { HTMLAttributes, ReactNode } from 'react'
import styles from './MetricStrip.module.css'

export interface MetricStripItem {
  key: string
  label: ReactNode
  value: ReactNode
  detail?: ReactNode
}

export interface MetricStripProps extends Omit<HTMLAttributes<HTMLDListElement>, 'children'> {
  items: readonly MetricStripItem[]
  label?: string
}

export function MetricStrip({
  className,
  items,
  label = '关键指标',
  ...props
}: MetricStripProps) {
  const classes = [styles.strip, className].filter(Boolean).join(' ')

  return (
    <dl className={classes} aria-label={label} {...props}>
      {items.map((item) => (
        <div className={styles.item} key={item.key}>
          <dt>{item.label}</dt>
          <dd>
            <strong>{item.value}</strong>
            {item.detail ? <span>{item.detail}</span> : null}
          </dd>
        </div>
      ))}
    </dl>
  )
}
