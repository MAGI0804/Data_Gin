import type { CSSProperties, HTMLAttributes, ReactNode } from 'react'
import styles from './MasterDetail.module.css'

export interface MasterDetailProps extends Omit<HTMLAttributes<HTMLDivElement>, 'children'> {
  master: ReactNode
  detail: ReactNode
  masterLabel?: string
  detailLabel?: string
  masterWidth?: number | string
}

type MasterDetailStyle = CSSProperties & {
  '--master-detail-master-width'?: string
}

export function MasterDetail({
  className,
  detail,
  detailLabel = '详情',
  master,
  masterLabel = '列表',
  masterWidth = 320,
  style,
  ...props
}: MasterDetailProps) {
  const classes = [styles.layout, className].filter(Boolean).join(' ')
  const layoutStyle: MasterDetailStyle = {
    ...style,
    '--master-detail-master-width': typeof masterWidth === 'number' ? `${masterWidth}px` : masterWidth,
  }

  return (
    <div className={classes} style={layoutStyle} {...props}>
      <aside className={styles.master} aria-label={masterLabel}>{master}</aside>
      <section className={styles.detail} aria-label={detailLabel}>{detail}</section>
    </div>
  )
}
