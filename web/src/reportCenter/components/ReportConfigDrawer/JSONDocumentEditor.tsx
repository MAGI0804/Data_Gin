import { useEffect, useId, useRef, useState } from 'react'
import { Braces, RotateCcw, WandSparkles } from 'lucide-react'
import styles from './JSONDocumentEditor.module.css'

export function JSONDocumentEditor<T>({ label, description, value, parse, parseText, onChange }: { label: string; description: string; value: unknown; parse: (value: unknown) => T; parseText?: (source: string) => T; onChange: (value: T) => void }) {
  const serialized = JSON.stringify(value, null, 2)
  const errorId = useId()
  const [text, setText] = useState(serialized)
  const [error, setError] = useState('')
  const previousSerialized = useRef(serialized)

  useEffect(() => {
    setText((current) => current === previousSerialized.current ? serialized : current)
    previousSerialized.current = serialized
    setError('')
  }, [serialized])

  const dirty = text !== serialized

  function apply() {
    try {
      const decoded = JSON.parse(text) as unknown
      onChange(parseText ? parseText(text) : parse(decoded))
      setError('')
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'JSON 格式不正确。')
    }
  }

  function format() {
    try {
      setText(JSON.stringify(JSON.parse(text) as unknown, null, 2))
      setError('')
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'JSON 格式不正确。')
    }
  }

  return <section className={styles.editor}>
    <div className={styles.heading}><div><h3><Braces aria-hidden="true" />{label}{dirty ? <span className={styles.dirty}>未应用</span> : null}</h3><p>{description}</p></div><div className={styles.actions}><button type="button" onClick={format}><WandSparkles aria-hidden="true" />格式化</button><button type="button" disabled={!dirty} onClick={() => { setText(serialized); setError('') }}><RotateCcw aria-hidden="true" />恢复</button><button type="button" disabled={!dirty} onClick={apply}>应用 JSON</button></div></div>
    <label><span className={styles.srOnly}>{label}</span><textarea spellCheck={false} value={text} aria-invalid={Boolean(error) || undefined} aria-describedby={error ? errorId : undefined} onChange={(event) => setText(event.currentTarget.value)} onKeyDown={(event) => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); apply() } }} /></label>
    {error ? <p id={errorId} className={styles.error} role="alert">{error}</p> : <p className={styles.hint}>{dirty ? 'JSON 草稿尚未应用；切换为表格编辑不会覆盖这份草稿。' : '使用 Ctrl / Command + Enter 可立即应用。'}</p>}
  </section>
}
