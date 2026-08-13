import type { HTMLAttributes } from 'react'
import styles from './PaginationControls.module.css'

export interface PaginationControlsProps extends HTMLAttributes<HTMLDivElement> {
  page: number
  totalPages: number
  loading?: boolean
  onPrevious: () => void
  onNext: () => void
}

export function PaginationControls({ className, loading = false, onNext, onPrevious, page, totalPages, ...props }: PaginationControlsProps) {
  return <div className={[styles.controls, className].filter(Boolean).join(' ')} role="status" aria-live="polite" {...props}>
    <span>第 {page} / {Math.max(totalPages, 1)} 页</span>
    <div>
      <button type="button" onClick={onPrevious} disabled={loading || page <= 1}>上一页</button>
      <button type="button" onClick={onNext} disabled={loading || totalPages === 0 || page >= totalPages}>下一页</button>
    </div>
  </div>
}
