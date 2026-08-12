import type { HTMLAttributes, ReactNode, RefObject } from 'react'
import styles from './AppShell.module.css'

export interface AppShellProps extends Omit<HTMLAttributes<HTMLElement>, 'children'> {
  navigation: ReactNode
  header?: ReactNode
  notices?: ReactNode
  children: ReactNode
  overlay?: ReactNode
  navigationOpen?: boolean
  navigationRef?: RefObject<HTMLElement>
  navigationClassName?: string
  onDismissNavigation?: () => void
  flushWorkspace?: boolean
  workspaceClassName?: string
}

export function AppShell({
  children,
  className,
  flushWorkspace = false,
  header,
  navigation,
  navigationClassName,
  navigationOpen = false,
  navigationRef,
  notices,
  onDismissNavigation,
  overlay,
  workspaceClassName,
  ...props
}: AppShellProps) {
  const classes = [styles.shell, className].filter(Boolean).join(' ')
  const navigationClasses = [styles.navigation, navigationOpen && styles.navigationOpen, navigationClassName].filter(Boolean).join(' ')
  const workspaceClasses = [styles.workspace, flushWorkspace && styles.workspaceFlush, workspaceClassName].filter(Boolean).join(' ')

  return (
    <main className={classes} {...props}>
      {navigationOpen ? (
        <button
          className={styles.backdrop}
          type="button"
          aria-label="关闭导航抽屉"
          onClick={onDismissNavigation}
        />
      ) : null}
      <aside ref={navigationRef} className={navigationClasses} aria-label="主导航">
        {navigation}
      </aside>
      <section className={workspaceClasses}>
        {header}
        {notices}
        <div className={styles.content}>{children}</div>
      </section>
      {overlay}
    </main>
  )
}
