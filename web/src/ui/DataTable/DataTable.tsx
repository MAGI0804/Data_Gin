import type { CSSProperties, ReactNode, TableHTMLAttributes } from 'react'
import styles from './DataTable.module.css'

export type DataTableDensity = 'compact' | 'default'

export interface DataTableProps extends TableHTMLAttributes<HTMLTableElement> {
  children: ReactNode
  containerClassName?: string
  density?: DataTableDensity
  minWidth?: number | string
  scrollLabel?: string
}

type DataTableStyle = CSSProperties & {
  '--data-table-min-width'?: string
}

export function DataTable({
  children,
  className,
  containerClassName,
  density = 'default',
  minWidth = 720,
  scrollLabel = '数据表格，可横向滚动',
  style,
  ...props
}: DataTableProps) {
  const tableClasses = [styles.table, styles[density], className].filter(Boolean).join(' ')
  const containerClasses = [styles.scrollArea, containerClassName].filter(Boolean).join(' ')
  const tableStyle: DataTableStyle = {
    ...style,
    '--data-table-min-width': typeof minWidth === 'number' ? `${minWidth}px` : minWidth,
  }

  return (
    <div className={containerClasses} role="region" aria-label={scrollLabel} tabIndex={0}>
      <table className={tableClasses} style={tableStyle} {...props}>
        {children}
      </table>
    </div>
  )
}
