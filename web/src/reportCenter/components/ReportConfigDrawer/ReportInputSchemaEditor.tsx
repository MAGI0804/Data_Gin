import { useEffect, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { newReportInputField, parseReportInputSchemaDocument, parseReportInputSchemaText, renameReportInputField, reportDateFormats, reportDateTimeFormats, reportInputControls, reportInputSchemaDocument, reportInputTypes, reportJSONContainsUnsafeNumber } from '../../refCursorConfig'
import type { ReportInputControl, ReportInputField, ReportInputFormat, ReportInputSchema, ReportInputType } from '../../types'
import { JSONDocumentEditor } from './JSONDocumentEditor'
import { getReportInputQueries, type ReportCenterClient } from '../../api'
import styles from './ReportInputSchemaEditor.module.css'

export function ReportInputSchemaEditor({ client, schema, onChange }: { client: ReportCenterClient; schema: ReportInputSchema; onChange: (schema: ReportInputSchema) => void }) {
  const entries = Object.entries(schema)
	const [queries, setQueries] = useState<string[]>([])
	const [queryState, setQueryState] = useState({ loading: true, error: '' })
	useEffect(() => {
		const controller = new AbortController()
		void getReportInputQueries(client, controller.signal).then((response) => {
			if (controller.signal.aborted) return
			if (!response.ok) { setQueryState({ loading: false, error: response.error }); return }
			setQueries(response.data)
			setQueryState({ loading: false, error: '' })
		})
		return () => controller.abort()
	}, [client])
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
    <JSONDocumentEditor label="筛选条件 JSON" description="每个键就是传给存储过程 JSON 的一个 conditions 参数；系统仅自动传入 report_id（本次运行 ID）。日期字段可用 format 选择 YYYYMMDD 或 YYYY-MM-DD。" value={reportInputSchemaDocument(schema)} parse={parseReportInputSchemaDocument} parseText={parseReportInputSchemaText} onChange={onChange} />
    <div className={styles.tableHeader}><div><h3>表格编辑</h3><p>表格修改会立即回写上方 JSON；日期只选择到日，日期时间选择到秒，最终传值仍是所选格式的字符串。</p></div><button type="button" onClick={add}><Plus aria-hidden="true" />新增筛选</button></div>
    <div className={styles.rows}>
      {entries.map(([code, field]) => <InputFieldRow code={code} field={field} queries={queries} queryState={queryState} key={code} onRename={(nextCode) => onChange(renameReportInputField(schema, code, nextCode))} onChange={(nextField) => onChange({ ...schema, [code]: nextField })} onDelete={() => onChange(Object.fromEntries(entries.filter(([itemCode]) => itemCode !== code)))} />)}
      {entries.length === 0 ? <p className={styles.empty}>至少新增一个筛选条件，或在上方粘贴 JSON 后点击“应用 JSON”。</p> : null}
    </div>
  </div>
}

function InputFieldRow({ code, field, queries, queryState, onRename, onChange, onDelete }: { code: string; field: ReportInputField; queries: string[]; queryState: { loading: boolean; error: string }; onRename: (code: string) => void; onChange: (field: ReportInputField) => void; onDelete: () => void }) {
	const source = reportInputSource(field)
  return <div className={styles.row}>
    <EditorField label="条件字段"><input className={styles.mono} value={code} onChange={(event) => onRename(event.currentTarget.value)} /></EditorField>
    <EditorField label="筛选显示名"><input value={field.displayName} onChange={(event) => onChange({ ...field, displayName: event.currentTarget.value })} /></EditorField>
    <EditorField label="JSON 类型"><select value={field.type} onChange={(event) => onChange(changeInputType(field, event.currentTarget.value as ReportInputType))}>{reportInputTypes.map((type) => <option key={type}>{type}</option>)}</select></EditorField>
    <EditorField label="查询控件"><select value={field.control} onChange={(event) => onChange(changeInputControl(field, event.currentTarget.value as ReportInputControl | ''))}>{reportInputControls.map((control) => <option value={control} key={control || 'AUTO'}>{inputControlLabel(control)}</option>)}</select></EditorField>
		{field.type === 'str' || field.type === 'number' ? <EditorField className={source === 'QUERY' ? styles.queryField : ''} label="输入方式"><select value={source} onChange={(event) => onChange(changeInputSource(field, event.currentTarget.value as ReportInputSource, queries))}><option value="MANUAL">手工输入</option><option value="STATIC">静态选项</option><option value="QUERY">Oracle 查询选择</option></select>{source === 'QUERY' ? <small>页面显示 name，提交给存储过程的是 id；保存并发布后生效。</small> : null}</EditorField> : null}
		{source === 'QUERY' ? <EditorField className={styles.queryField} label="选项查询"><select value={field.queryName ?? ''} disabled={queryState.loading} onChange={(event) => onChange(withQueryName(field, event.currentTarget.value))}><option value="">{queryState.loading ? '正在加载查询…' : queries.length ? '请选择查询' : '请先在报表配置页新增查询'}</option>{field.queryName && !queries.includes(field.queryName) ? <option value={field.queryName}>{field.queryName}（当前未启用）</option> : null}{queries.map((query) => <option value={query} key={query}>{query}</option>)}</select>{queryState.error ? <small className={styles.queryError}>{queryState.error}</small> : null}</EditorField> : null}
    {field.control === 'DATE' || field.control === 'DATETIME' ? <EditorField className={styles.formatField} label="日期传值格式（必选）"><select className={styles.mono} value={field.format ?? defaultFormat(field.control)} onChange={(event) => onChange({ ...field, format: event.currentTarget.value as ReportInputFormat })}>{(field.control === 'DATE' ? reportDateFormats : reportDateTimeFormats).map((format) => <option value={format} key={format}>{inputFormatLabel(format)}</option>)}</select><small>传入示例：{inputFormatExample(field.format ?? defaultFormat(field.control))}</small></EditorField> : null}
    <JSONValueInput label="示例值" value={field.example} exists={Object.hasOwnProperty.call(field, 'example')} numeric={field.type === 'number' || field.type === 'list[number]'} onChange={(value, exists) => onChange(withOptional(field, 'example', value, exists))} />
    <JSONValueInput label="默认值" value={field.default} exists={Object.hasOwnProperty.call(field, 'default')} numeric={field.type === 'number' || field.type === 'list[number]'} onChange={(value, exists) => onChange(withOptional(field, 'default', value, exists))} />
		{source === 'STATIC' || field.type !== 'str' && field.type !== 'number' ? <JSONValueInput label="允许值" value={field.allowedValues} exists={Boolean(field.allowedValues)} array numeric={field.type === 'number' || field.type === 'list[number]'} onChange={(value, exists) => onChange(withOptional(field, 'allowedValues', value as unknown[], exists))} /> : null}
    <div className={styles.flags}><label><input type="checkbox" checked={field.required} onChange={(event) => onChange({ ...field, required: event.currentTarget.checked })} />必填</label></div>
    <button className={styles.delete} type="button" aria-label={`删除筛选条件 ${field.displayName || code}`} onClick={onDelete}><Trash2 aria-hidden="true" /></button>
  </div>
}

type ReportInputSource = 'MANUAL' | 'STATIC' | 'QUERY'

function reportInputSource(field: ReportInputField): ReportInputSource {
	if (field.queryName) return 'QUERY'
	if (field.allowedValues?.length) return 'STATIC'
	return 'MANUAL'
}

function changeInputSource(field: ReportInputField, source: ReportInputSource, queries: string[]): ReportInputField {
	const next = { ...field }
	delete next.queryName
	delete next.allowedValues
	if (source === 'QUERY') {
		next.control = 'SELECT'
		if (queries[0]) next.queryName = queries[0]
	} else if (source === 'STATIC') {
		next.control = 'SELECT'
	} else if (next.control === 'SELECT') {
		next.control = next.type === 'number' ? 'NUMBER' : 'TEXT'
	}
	return next
}

function EditorField({ label, className = '', children }: { label: string; className?: string; children: React.ReactNode }) {
  return <label className={[styles.field, className].filter(Boolean).join(' ')}><span>{label}</span>{children}</label>
}

function JSONValueInput({ label, value, exists, array = false, numeric = false, onChange }: { label: string; value: unknown; exists: boolean; array?: boolean; numeric?: boolean; onChange: (value: unknown, exists: boolean) => void }) {
  const serialized = exists ? JSON.stringify(value) : ''
  const [text, setText] = useState(serialized)
  const [invalid, setInvalid] = useState(false)
  useEffect(() => { setText(serialized); setInvalid(false) }, [serialized])
  return <EditorField label={`${label} JSON`}><input className={styles.mono} value={text} aria-invalid={invalid || undefined} placeholder={array ? '["A","B"]' : '"01"'} onChange={(event) => setText(event.currentTarget.value)} onBlur={() => {
    if (!text.trim()) { setInvalid(false); onChange(undefined, false); return }
    try {
      if (numeric && reportJSONContainsUnsafeNumber(text)) { setInvalid(true); return }
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

function changeInputType(field: ReportInputField, type: ReportInputType): ReportInputField {
	if (field.control !== 'DATE' && field.control !== 'DATETIME') {
		const next = { ...field, type }
		if (type !== 'str' && type !== 'number') delete next.queryName
		return next
	}
  const control: ReportInputControl = type === 'number' ? 'NUMBER' : type === 'bool' ? 'CHECKBOX' : type === 'str' ? field.control : 'TEXTAREA'
  const next = { ...field, type, control }
  if (control !== 'DATE' && control !== 'DATETIME') delete next.format
  return next
}

function changeInputControl(field: ReportInputField, control: ReportInputControl | ''): ReportInputField {
  const next = { ...field, control, ...((control === 'DATE' || control === 'DATETIME') ? { type: 'str' as const, format: defaultFormat(control) } : {}) }
  if (control !== 'DATE' && control !== 'DATETIME') delete next.format
	if (control !== 'SELECT') delete next.queryName
  return next
}

function withQueryName(field: ReportInputField, value: string): ReportInputField {
	const next = { ...field }
	const queryName = value.trim()
	if (!queryName) delete next.queryName
	else {
		next.queryName = queryName
		delete next.allowedValues
	}
	return next
}

function defaultFormat(control: 'DATE' | 'DATETIME'): ReportInputFormat {
  return control === 'DATE' ? 'YYYYMMDD' : 'YYYY-MM-DD HH:mm:ss'
}

function inputControlLabel(control: ReportInputControl | '') {
  if (!control) return '自动选择'
  if (control === 'DATE') return '日期（精确到日）'
  if (control === 'DATETIME') return '日期时间（精确到秒）'
  return control
}

function inputFormatLabel(format: ReportInputFormat) {
  return `${format}（${inputFormatExample(format)}）`
}

function inputFormatExample(format: ReportInputFormat) {
  const examples: Record<ReportInputFormat, string> = {
    YYYYMMDD: '20260504',
    'YYYY-MM-DD': '2026-05-04',
    YYYYMMDDHHmmss: '20260504132500',
    'YYYY-MM-DD HH:mm:ss': '2026-05-04 13:25:00',
    ISO8601: '2026-05-04T13:25:00',
  }
  return examples[format]
}
