import type { HTMLAttributes, ReactNode } from 'react'
import styles from './PageCanvas.module.css'

export interface PageCanvasProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode
}

export function PageCanvas({ children, className, ...props }: PageCanvasProps) {
  const classes = [styles.canvas, className].filter(Boolean).join(' ')

  return (
    <div className={classes} {...props}>
      {children}
    </div>
  )
}
