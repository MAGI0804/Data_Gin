import type { HTMLAttributes, ReactNode, RefObject } from 'react'
import { Menu } from 'lucide-react'
import styles from './WorkspaceHeader.module.css'

export interface WorkspaceHeaderProps extends Omit<HTMLAttributes<HTMLElement>, 'title'> {
  title?: ReactNode
  description?: ReactNode
  context?: ReactNode
  actions?: ReactNode
  menuButtonRef?: RefObject<HTMLButtonElement>
  onOpenNavigation?: () => void
}

export function WorkspaceHeader({
  actions,
  className,
  context,
  description,
  menuButtonRef,
  onOpenNavigation,
  title,
  ...props
}: WorkspaceHeaderProps) {
  const classes = [styles.header, title ? styles.withTitle : styles.toolbarOnly, className].filter(Boolean).join(' ')

  return (
    <header className={classes} {...props}>
      <div className={styles.heading}>
        <button
          ref={menuButtonRef}
          className={styles.menuButton}
          type="button"
          aria-label="打开主导航"
          onClick={onOpenNavigation}
        >
          <Menu aria-hidden="true" />
        </button>
        {title ? (
          <div>
            <h1>{title}</h1>
            {description ? <p>{description}</p> : null}
          </div>
        ) : (
          <span className={styles.productContext}>{context ?? '数据协同工作台'}</span>
        )}
      </div>
      {actions ? <div className={styles.actions}>{actions}</div> : null}
    </header>
  )
}
