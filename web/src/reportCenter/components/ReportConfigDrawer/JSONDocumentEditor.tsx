import { useEffect, useId, useState } from 'react'
import { Braces } from 'lucide-react'
import styles from './JSONDocumentEditor.module.css'

export function JSONDocumentEditor<T>({ label, description, value, parse, onChange }: { label: string; description: string; value: unknown; parse: (value: unknown) => T; onChange: (value: T) => void }) {
  const serialized = JSON.stringify(value, null, 2)
  const errorId = useId()
  const [text, setText] = useState(serialized)
  const [error, setError] = useState('')

  useEffect(() => {
    setText(serialized)
    setError('')
  }, [serialized])

  function apply() {
    try {
      const decoded = JSON.parse(text) as unknown
      onChange(parse(decoded))
      setError('')
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'JSON 格式不正确。')
    }
  }

  return <section className={styles.editor}>
    <div className={styles.heading}><div><h3><Braces aria-hidden="true" />{label}</h3><p>{description}</p></div><button type="button" onClick={apply}>应用 JSON</button></div>
    <label><span className={styles.srOnly}>{label}</span><textarea spellCheck={false} value={text} aria-invalid={Boolean(error) || undefined} aria-describedby={error ? errorId : undefined} onChange={(event) => setText(event.currentTarget.value)} onKeyDown={(event) => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); apply() } }} /></label>
    {error ? <p id={errorId} className={styles.error} role="alert">{error}</p> : <p className={styles.hint}>使用 Ctrl / Command + Enter 可立即应用。</p>}
  </section>
}
