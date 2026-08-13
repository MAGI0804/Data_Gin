import { useEffect, useId, useRef, type HTMLAttributes, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import styles from './Dialog.module.css'
import controlStyles from '../Controls/Controls.module.css'
import { isolateModalLayer, isTopModalLayer } from '../modalIsolation'

const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

export interface DialogProps extends Omit<HTMLAttributes<HTMLElement>, 'title' | 'role'> {
  open: boolean
  title: ReactNode
  children: ReactNode
  onClose: () => void
  description?: ReactNode
  footer?: ReactNode
  role?: 'dialog' | 'alertdialog'
  closeLabel?: string
  closeDisabled?: boolean
  closeOnBackdrop?: boolean
  initialFocusRef?: React.RefObject<HTMLElement | null>
  returnFocus?: HTMLElement | null
}

function visibleFocusableElements(container: HTMLElement) {
  return Array.from(container.querySelectorAll<HTMLElement>(focusableSelector)).filter(
    (element) => !element.closest('[hidden], [aria-hidden="true"]'),
  )
}

export function Dialog({
  children,
  className,
  closeDisabled = false,
  closeLabel = '关闭对话框',
  closeOnBackdrop = true,
  description,
  footer,
  initialFocusRef,
  onClose,
  open,
  returnFocus,
  role = 'dialog',
  title,
  ...props
}: DialogProps) {
  const panelRef = useRef<HTMLElement>(null)
  const layerRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)
  const closeDisabledRef = useRef(closeDisabled)
  const titleId = useId()
  const descriptionId = useId()
  onCloseRef.current = onClose
  closeDisabledRef.current = closeDisabled

  useEffect(() => {
    if (!open) return

    const previouslyFocused = returnFocus ?? (document.activeElement instanceof HTMLElement ? document.activeElement : null)
    const previousOverflow = document.body.style.overflow
    const panel = panelRef.current
    const layer = layerRef.current
    document.body.style.overflow = 'hidden'
    const restoreIsolation = layer ? isolateModalLayer(layer) : () => undefined

    const requestedInitialFocus = initialFocusRef?.current
    const initialFocus = requestedInitialFocus && panel?.contains(requestedInitialFocus)
      ? requestedInitialFocus
      : (panel ? visibleFocusableElements(panel)[0] : null) ?? panel
    initialFocus?.focus()

    function handleKeyDown(event: KeyboardEvent) {
      const currentPanel = panelRef.current
      if (!currentPanel || !isTopModalLayer(layerRef.current)) return

      if (event.key === 'Escape' && !closeDisabledRef.current) {
        event.preventDefault()
        onCloseRef.current()
        return
      }

      if (event.key !== 'Tab') return
      const focusable = visibleFocusableElements(currentPanel)
      if (focusable.length === 0) {
        event.preventDefault()
        currentPanel.focus()
        return
      }

      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (!currentPanel.contains(document.activeElement)) {
        event.preventDefault()
        ;(event.shiftKey ? last : first).focus()
        return
      }
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.body.style.overflow = previousOverflow
      restoreIsolation()
      document.removeEventListener('keydown', handleKeyDown)
      previouslyFocused?.focus()
    }
  }, [initialFocusRef, open, returnFocus])

  if (!open || typeof document === 'undefined') return null

  const panelClasses = [styles.dialog, className].filter(Boolean).join(' ')

  return createPortal(
    <div ref={layerRef} className={`${styles.layer} ${controlStyles.surface}`}>
      <button
        className={styles.backdrop}
        type="button"
        aria-label={closeLabel}
        disabled={closeDisabled || !closeOnBackdrop}
        onClick={onClose}
      />
      <section
        ref={panelRef}
        className={panelClasses}
        {...props}
        role={role}
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
        tabIndex={-1}
      >
        <header className={styles.header}>
          <div className={styles.heading}>
            <h2 id={titleId}>{title}</h2>
            {description ? <div id={descriptionId} className={styles.description}>{description}</div> : null}
          </div>
          <button
            className={styles.close}
            type="button"
            aria-label={closeLabel}
            disabled={closeDisabled}
            onClick={onClose}
          >
            <X aria-hidden="true" />
          </button>
        </header>
        <div className={styles.body}>{children}</div>
        {footer ? <footer className={styles.footer}>{footer}</footer> : null}
      </section>
    </div>,
    document.body,
  )
}
