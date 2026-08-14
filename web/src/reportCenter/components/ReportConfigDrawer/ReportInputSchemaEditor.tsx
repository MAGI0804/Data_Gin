import { useEffect, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { newReportInputField, parseReportInputSchemaDocument, renameReportInputField, reportInputControls, reportInputSchemaDocument, reportInputTypes } from '../../refCursorConfig'
import type { ReportInputField, ReportInputSchema } from '../../types'
import { JSONDocumentEditor } from './JSONDocumentEditor'
import styles from './ReportInputSchemaEditor.module.css'

export function ReportInputSchemaEditor({ schema, onChange }: { schema: ReportInputSchema; onChange: (schema: ReportInputSchema) => void }) {
  const entries = Object.entries(schema)
  function add() {
    let index = entries.length
    let next = newReportInputField(index)
    while (Object.hasOwnProperty.call(schema, next[0])) {
      index += 1
      next = newReportInputField(index)
    }
    onChange({ ...schema, [next[0]]: next[1] })
  }
  return <div className={styles.editor}>
    <JSONDocumentEditor label="筛选条件 JSON" description="每个键会写入运行 payload 的 conditions；displayName 是查询页显示的筛选名称。" value={reportInputSchemaDocument(schema)} parse={parseReportInputSchemaDocument} onChange={onChange} />
    <div className={styles.tableHeader}><div><h3>表格编辑</h3><p>表格修改会立即回写上方 JSON；示例值、默认值和允许值均填写合法 JSON。</p></div><button type="button" onClick={add}><Plus aria-hidden="true" />新增筛选</button></div>
    <div className={styles.rows}>
      {entries.map(([code, field], index) => <InputFieldRow code={code} field={field} key={index} onRename={(nextCode) => onChange(renameReportInputField(schema, code, nextCode))} onChange={(nextField) => onChange({ ...schema, [code]: nextField })} onDelete={() => onChange(Object.fromEntries(entries.filter(([itemCode]) => itemCode !== code)))} />)}
      {entries.length === 0 ? <p className={styles.empty}>至少新增一个筛选条件，或在上方粘贴 JSON 后点击“应用 JSON”。</p> : null}
    </div>
  </div>
}

function InputFieldRow({ code, field, onRename, onChange, onDelete }: { code: string; field: ReportInputField; onRename: (code: string) => void; onChange: (field: ReportInputField) => void; onDelete: () => void }) {
  return <div className={styles.row}>
    <EditorField label="条件字段"><input className={styles.mono} value={code} onChange={(event) => onRename(event.currentTarget.value)} /></EditorField>
    <EditorField label="筛选显示名"><input value={field.displayName} onChange={(event) => onChange({ ...field, displayName: event.currentTarget.value })} /></EditorField>
    <EditorField label="Oracle 类型"><select value={field.type} onChange={(event) => onChange({ ...field, type: event.currentTarget.value })}>{reportInputTypes.map((type) => <option key={type}>{type}</option>)}</select></EditorField>
    <EditorField label="查询控件"><select value={field.control} onChange={(event) => onChange({ ...field, control: event.currentTarget.value as ReportInputField['control'] })}>{reportInputControls.map((control) => <option value={control} key={control || 'AUTO'}>{control || '自动选择'}</option>)}</select></EditorField>
    <JSONValueInput label="示例值" value={field.example} exists={Object.hasOwnProperty.call(field, 'example')} onChange={(value, exists) => onChange(withOptional(field, 'example', value, exists))} />
    <JSONValueInput label="默认值" value={field.default} exists={Object.hasOwnProperty.call(field, 'default')} onChange={(value, exists) => onChange(withOptional(field, 'default', value, exists))} />
    <JSONValueInput label="允许值" value={field.allowedValues} exists={Boolean(field.allowedValues)} array onChange={(value, exists) => onChange(withOptional(field, 'allowedValues', value as unknown[], exists))} />
    <div className={styles.flags}><label><input type="checkbox" checked={field.required} onChange={(event) => onChange({ ...field, required: event.currentTarget.checked })} />必填</label><label><input type="checkbox" checked={field.multiple} onChange={(event) => onChange({ ...field, multiple: event.currentTarget.checked })} />多值</label></div>
    <button className={styles.delete} type="button" aria-label={`删除筛选条件 ${field.displayName || code}`} onClick={onDelete}><Trash2 aria-hidden="true" /></button>
  </div>
}

function EditorField({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className={styles.field}><span>{label}</span>{children}</label>
}

function JSONValueInput({ label, value, exists, array = false, onChange }: { label: string; value: unknown; exists: boolean; array?: boolean; onChange: (value: unknown, exists: boolean) => void }) {
  const serialized = exists ? JSON.stringify(value) : ''
  const [text, setText] = useState(serialized)
  const [invalid, setInvalid] = useState(false)
  useEffect(() => { setText(serialized); setInvalid(false) }, [serialized])
  return <EditorField label={`${label} JSON`}><input className={styles.mono} value={text} aria-invalid={invalid || undefined} placeholder={array ? '["A","B"]' : '"01"'} onChange={(event) => setText(event.currentTarget.value)} onBlur={() => {
    if (!text.trim()) { setInvalid(false); onChange(undefined, false); return }
    try {
      const decoded = JSON.parse(text) as unknown
      if (array && (!Array.isArray(decoded) || decoded.length === 0)) { setInvalid(true); return }
      setInvalid(false)
      onChange(decoded, true)
    } catch { setInvalid(true) }
  }} /></EditorField>
}

function withOptional<K extends 'example' | 'default' | 'allowedValues'>(field: ReportInputField, key: K, value: ReportInputField[K], exists: boolean): ReportInputField {
  const next = { ...field }
  if (exists) next[key] = value
  else delete next[key]
  return next
}
