import { useState } from 'react'
import { Database, Play, Search } from 'lucide-react'
import type { WorkspaceApiClient } from '../appShell/WorkspaceRouter'
import { getOfficeResultColumns, searchOfficeProcedures, searchOfficeResultTables, testOfficeSelect } from './api'
import { mappingsFromColumns } from './contracts'
import { ColumnMappingEditor } from './ColumnMappingEditor'
import { QueryParameterEditor } from './QueryParameterEditor'
import type { OfficeMessageDraft, OfficeProcedureSummary, OfficeResultTableSummary } from './types'
import styles from './OfficeMessage.module.css'

type Props = { client: WorkspaceApiClient; draft: OfficeMessageDraft; disabled: boolean; onChange: (draft: OfficeMessageDraft) => void; onNotice: (notice: string, error?: boolean) => void }

export function ProcedureSourceEditor({ client, draft, disabled, onChange, onNotice }: Props) {
  const [procedures, setProcedures] = useState<OfficeProcedureSummary[]>([])
  const [tables, setTables] = useState<OfficeResultTableSummary[]>([])
  const [procedureSearch, setProcedureSearch] = useState('')
  const [tableSearch, setTableSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const busy = disabled || loading
  async function findProcedures() {
    setLoading(true)
    const result = await searchOfficeProcedures(client, draft.procedureOwner, procedureSearch)
    setLoading(false)
    if (!result.ok) return onNotice(result.error, true)
    setProcedures(result.data)
    onNotice(`找到 ${result.data.length} 个存储过程。`)
  }
  async function findTables() {
    setLoading(true)
    const result = await searchOfficeResultTables(client, draft.resultTableOwner, tableSearch)
    setLoading(false)
    if (!result.ok) return onNotice(result.error, true)
    setTables(result.data)
    onNotice(`找到 ${result.data.length} 个结果表。`)
  }
  async function loadColumns() {
    if (!draft.resultTableOwner.trim() || !draft.resultTableName.trim()) return onNotice('请先选择结果表。', true)
    setLoading(true)
    const result = await getOfficeResultColumns(client, draft.resultTableOwner, draft.resultTableName)
    setLoading(false)
    if (!result.ok) return onNotice(result.error, true)
    onChange({ ...draft, columnMapping: mappingsFromColumns(result.data) })
    onNotice(`已读取 ${result.data.length} 个结果列并生成列名对照。`)
  }
  return <>
    <div className={styles.sourceGrid}>
      <div className={styles.lookupPanel}><h3>存储过程</h3><div className={styles.twoColumns}><label>Owner<input className={styles.mono} value={draft.procedureOwner} disabled={busy} onChange={(event) => onChange({ ...draft, procedureOwner: event.currentTarget.value.toUpperCase() })} /></label><label>搜索<input value={procedureSearch} disabled={busy} placeholder="过程名或包名" onChange={(event) => setProcedureSearch(event.currentTarget.value)} /></label></div><button type="button" disabled={busy} onClick={() => void findProcedures()}><Search aria-hidden="true" />查询过程</button>
        {procedures.length ? <label>查询结果<select size={Math.min(5, procedures.length)} disabled={busy} value="" onChange={(event) => { const item = procedures[Number(event.currentTarget.value)]; if (item) onChange({ ...draft, procedureOwner: item.owner, packageName: item.packageName, procedureName: item.name, procedureOverload: item.overload }) }}><option value="" disabled>选择存储过程</option>{procedures.map((item, index) => <option value={index} key={`${item.owner}.${item.packageName}.${item.name}.${item.overload}`}>{[item.owner, item.packageName, item.name].filter(Boolean).join('.')} {item.overload ? `#${item.overload}` : ''}</option>)}</select></label> : null}
        <div className={styles.twoColumns}><label>Package<input className={styles.mono} value={draft.packageName} disabled={busy} onChange={(event) => onChange({ ...draft, packageName: event.currentTarget.value.toUpperCase() })} /></label><label>Procedure<input className={styles.mono} value={draft.procedureName} disabled={busy} onChange={(event) => onChange({ ...draft, procedureName: event.currentTarget.value.toUpperCase() })} /></label></div>
      </div>
      <div className={styles.lookupPanel}><h3>结果表</h3><div className={styles.twoColumns}><label>Owner<input className={styles.mono} value={draft.resultTableOwner} disabled={busy} onChange={(event) => onChange({ ...draft, resultTableOwner: event.currentTarget.value.toUpperCase() })} /></label><label>搜索<input value={tableSearch} disabled={busy} placeholder="结果表名" onChange={(event) => setTableSearch(event.currentTarget.value)} /></label></div><button type="button" disabled={busy} onClick={() => void findTables()}><Search aria-hidden="true" />查询结果表</button>
        {tables.length ? <label>查询结果<select size={Math.min(5, tables.length)} disabled={busy} value="" onChange={(event) => { const item = tables[Number(event.currentTarget.value)]; if (item) onChange({ ...draft, resultTableOwner: item.owner, resultTableName: item.name }) }}><option value="" disabled>选择结果表</option>{tables.map((item, index) => <option value={index} key={`${item.owner}.${item.name}`}>{item.owner}.{item.name}（{item.columnCount} 列）</option>)}</select></label> : null}
        <label>结果表名<input className={styles.mono} value={draft.resultTableName} disabled={busy} onChange={(event) => onChange({ ...draft, resultTableName: event.currentTarget.value.toUpperCase() })} /></label><button type="button" disabled={busy} onClick={() => void loadColumns()}><Database aria-hidden="true" />读取字段并生成映射</button>
      </div>
    </div>
    <ColumnMappingEditor value={draft.columnMapping} disabled={busy} onChange={(columnMapping) => onChange({ ...draft, columnMapping })} />
  </>
}
export function QuerySourceEditor({ client, draft, disabled, onChange, onNotice }: Props) {
  const [testValues, setTestValues] = useState<Record<string, string>>({})
  const [testing, setTesting] = useState(false)
  async function testQuery() {
    setTesting(true)
    const result = await testOfficeSelect(client, draft.selectSql, draft.parameters, testValues)
    setTesting(false)
    if (!result.ok) return onNotice(result.error, true)
    onChange({ ...draft, columnMapping: mappingsFromColumns(result.data.map((column) => ({ name: column.name, dataType: column.databaseType }))) })
    onNotice(`SELECT 测试成功，已识别 ${result.data.length} 列并生成列名对照。`)
  }
  const busy = disabled || testing
  return <>
    <label>Oracle SELECT<textarea className={styles.sqlEditor} rows={10} spellCheck={false} value={draft.selectSql} disabled={busy} onChange={(event) => onChange({ ...draft, selectSql: event.currentTarget.value })} /></label>
    <p className={styles.contractNote}>仅允许一条 SELECT，不支持注释、分号和 FOR UPDATE。日期参数直接写 <code>:参数名</code>，不要再套 TO_DATE。</p>
    <QueryParameterEditor value={draft.parameters} testValues={testValues} disabled={busy} onChange={(parameters) => onChange({ ...draft, parameters })} onTestValuesChange={setTestValues} />
    <div className={styles.testBar}><button type="button" className={styles.primary} disabled={busy} onClick={() => void testQuery()}><Play aria-hidden="true" />{testing ? '测试中…' : '测试 SELECT 并读取列'}</button><span>测试只读取列元数据；执行受服务端 30 秒超时限制。</span></div>
    <ColumnMappingEditor value={draft.columnMapping} disabled={busy} onChange={(columnMapping) => onChange({ ...draft, columnMapping })} />
  </>
}
