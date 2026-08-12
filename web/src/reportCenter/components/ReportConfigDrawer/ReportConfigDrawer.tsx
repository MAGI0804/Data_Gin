import { useEffect, useId, useRef } from 'react'
import type { ReportSummary } from '../../types'
import styles from './ReportConfigDrawer.module.css'

export function ReportConfigDrawer({ report, onClose }: { report: ReportSummary | null; onClose: () => void }) {
  const panelRef = useRef<HTMLElement>(null)
  const returnFocusRef = useRef<HTMLElement | null>(document.activeElement instanceof HTMLElement ? document.activeElement : null)
  const onCloseRef = useRef(onClose)
  const titleId = useId()
  onCloseRef.current = onClose

  useEffect(() => {
    const previousOverflow = document.body.style.overflow
    const returnFocus = returnFocusRef.current
    const selector = 'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onCloseRef.current()
        return
      }
      if (event.key !== 'Tab') return
      const focusable = Array.from(panelRef.current?.querySelectorAll<HTMLElement>(selector) ?? [])
      if (focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', handleKeyDown)
    panelRef.current?.querySelector<HTMLElement>(selector)?.focus()
    return () => {
      document.body.style.overflow = previousOverflow
      window.removeEventListener('keydown', handleKeyDown)
      returnFocus?.focus()
    }
  }, [])

  return (
    <div className={styles.layer}>
      <button className={styles.backdrop} type="button" aria-label="关闭报表配置侧栏" onClick={onClose} />
      <section ref={panelRef} className={styles.drawer} role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1}>
        <header className={styles.header}>
          <div><span>MYSQL CONFIGURATION</span><h2 id={titleId}>{report ? '编辑报表配置' : '创建报表配置'}</h2></div>
          <button className="ui-control-radius" type="button" onClick={onClose}>关闭</button>
        </header>
        <form className={styles.form} onSubmit={(event) => event.preventDefault()}>
          <label>报表名称<input className="ui-control-radius" name="name" defaultValue={report?.name ?? ''} readOnly={Boolean(report)} /></label>
          <label>报表编码<input className="ui-control-radius" name="code" defaultValue={report?.code ?? ''} readOnly={Boolean(report)} /></label>
          <label>分类<input className="ui-control-radius" name="category" defaultValue={report?.category ?? ''} readOnly={Boolean(report)} /></label>
          <label>Oracle 数据源<select className="ui-control-radius" name="datasource" disabled><option>数据源接口尚未接入</option></select></label>
          <label className={styles.wide}>存储过程调用模板<textarea className="ui-control-radius" name="callTemplate" rows={7} disabled placeholder="BEGIN REPORT_PKG.PROCEDURE({{runId}}); END;" /></label>
          <div className={styles.notice} role="note">本批仅建立配置界面骨架。保存、参数解析和 Oracle 契约探测接口接入后才可提交配置。</div>
        </form>
        <footer className={styles.footer}>
          <button className="ui-control-radius" type="button" onClick={onClose}>取消</button>
          <button className="ui-control-radius" type="button" disabled>保存接口尚未接入</button>
        </footer>
      </section>
    </div>
  )
}
