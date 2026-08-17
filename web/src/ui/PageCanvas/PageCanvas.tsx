import type { HTMLAttributes, ReactNode } from 'react'
import styles from './PageCanvas.module.css'

export interface PageCanvasProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode
  density?: 'default' | 'compact'
}

export function PageCanvas({ children, className, density = 'default', ...props }: PageCanvasProps) {
  const classes = [styles.canvas, density === 'compact' && styles.compact, className].filter(Boolean).join(' ')

  return (
    <div className={classes} {...props}>
      {children}
    </div>
  )
}
