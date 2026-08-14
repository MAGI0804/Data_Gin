import { Plus, Trash2 } from 'lucide-react'
import { applyExcelMapping, excelMappingFromColumns, parseExcelMappingDocument, renameExcelMappingField } from '../../refCursorConfig'
import type { ReportColumn } from '../../types'
import { JSONDocumentEditor } from './JSONDocumentEditor'
import styles from './ReportExcelMappingEditor.module.css'

export function ReportExcelMappingEditor({ columns, onChange }: { columns: ReportColumn[]; onChange: (columns: ReportColumn[]) => void }) {
  const mapping = excelMappingFromColumns(columns)
  const entries = Object.entries(mapping)
  const setMapping = (next: Record<string, string>) => onChange(applyExcelMapping(columns, next))

  function add() {
    let index = entries.length + 1
    let databaseColumn = `FIELD_${index}`
    while (Object.keys(mapping).some((field) => field.toUpperCase() === databaseColumn)) {
      index += 1
      databaseColumn = `FIELD_${index}`
    }
    setMapping({ ...mapping, [databaseColumn]: `字段 ${index}` })
  }

  function updateField(currentField: string, nextField: string) {
    onChange(renameExcelMappingField(columns, currentField, nextField))
  }

  return <div className={styles.editor}>
    <JSONDocumentEditor label="Excel 字段映射 JSON" description={'键是 Oracle 结果表字段，值是 Excel 表头，例如 {"a":"id"}。'} value={mapping} parse={parseExcelMappingDocument} onChange={setMapping} />
    <div className={styles.tableHeader}><div><h3>表格编辑</h3><p>无需逐项维护完整字段契约；这里和上方 JSON 使用同一份 columns 数据。</p></div><button type="button" onClick={add}><Plus aria-hidden="true" />新增映射</button></div>
    <div className={styles.rows}>
      {entries.map(([databaseColumn, excelHeader], index) => <div className={styles.row} key={index}>
        <label><span>Oracle 结果表字段</span><input className={styles.mono} value={databaseColumn} onChange={(event) => updateField(databaseColumn, event.currentTarget.value)} /></label>
        <label><span>Excel 表头</span><input value={excelHeader} onChange={(event) => setMapping(Object.fromEntries(entries.map(([field, header]) => [field, field === databaseColumn ? event.currentTarget.value : header])))} /></label>
        <span className={styles.order}>第 {index + 1} 列</span>
        <button className={styles.delete} type="button" aria-label={`删除 ${databaseColumn} Excel 映射`} onClick={() => setMapping(Object.fromEntries(entries.filter(([field]) => field !== databaseColumn)))}><Trash2 aria-hidden="true" /></button>
      </div>)}
      {entries.length === 0 ? <p className={styles.empty}>可先粘贴字段映射 JSON，或手动新增一行；发布核验后也可依据结果表字段调整。</p> : null}
    </div>
  </div>
}
