import { useCallback, useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import { RefreshCw, Search } from 'lucide-react'
import { getReportProcedureSignature, getReportProcedures, getReportResultTableSchema, getReportResultTables, type ReportCenterClient } from '../../api'
import { reconcileReportColumnsWithResultSchema, refreshReportColumnMetadata, reportColumnsFromResultSchema } from '../../refCursorConfig'
import type { ReportDraft, ReportProcedureSignature, ReportProcedureSummary, ReportResultTablePage, ReportResultTableSchema, ReportResultTableSummary } from '../../types'
import styles from './ReportProcedureEditor.module.css'

export function ReportProcedureEditor({ client, draft, onChange }: { client: ReportCenterClient; draft: ReportDraft; onChange: Dispatch<SetStateAction<ReportDraft>> }) {
  const [filters, setFilters] = useState({ owner: draft.procedure.owner, search: '' })
  const [catalog, setCatalog] = useState<{ items: ReportProcedureSummary[]; hasMore: boolean; nextAfter: string }>({ items: [], hasMore: false, nextAfter: '' })
  const [signature, setSignature] = useState<ReportProcedureSignature | null>(null)
  const [state, setState] = useState({ loading: false, inspecting: false, error: '' })
  const [tableFilters, setTableFilters] = useState({ owner: draft.result.tableOwner || draft.procedure.owner, search: '' })
  const [tableCatalog, setTableCatalog] = useState<ReportResultTablePage>({ items: [], hasMore: false, nextAfter: '' })
  const [tableSchema, setTableSchema] = useState<ReportResultTableSchema | null>(null)
  const [tableState, setTableState] = useState({ loading: false, inspecting: false, error: '' })
  const catalogRequest = useRef(0)
  const signatureRequest = useRef(0)
  const tableCatalogRequest = useRef(0)
  const tableSchemaRequest = useRef(0)
  const signatureCache = useRef<ReportProcedureSignature | null>(null)
  const selectedKey = procedureKey(draft.procedure)
  const selectedOwner = draft.procedure.owner
  const selectedPackage = draft.procedure.package
  const selectedName = draft.procedure.name
  const selectedOverload = draft.procedure.overload
  const selectedTableKey = resultTableKey(draft.result)

  const inspect = useCallback(async (procedure: ReportProcedureSummary | ReportDraft['procedure'], signal?: AbortSignal) => {
    if (!draft.datasourceId || !procedure.owner || !procedure.name) return
    const request = ++signatureRequest.current
    setState((current) => ({ ...current, inspecting: true, error: '' }))
    const response = await getReportProcedureSignature(client, draft.datasourceId, procedure, signal)
    if (signal?.aborted || request !== signatureRequest.current) return
    if (!response.ok) {
      setSignature(null)
      setState((current) => ({ ...current, inspecting: false, error: response.error }))
      return
    }
    signatureCache.current = response.data
    setSignature(response.data)
    setState((current) => ({ ...current, inspecting: false }))
    if (!response.data.protocolReady) {
      setState((current) => ({ ...current, error: response.data.blockingReasons[0] || '所选过程不符合唯一 JSON 输入协议。' }))
      return
    }
    onChange((currentDraft) => ({
      ...currentDraft,
      executionMode: 'TABLE_SNAPSHOT',
      procedure: {
        owner: response.data.procedure.owner,
        package: response.data.procedure.package,
        name: response.data.procedure.name,
        overload: response.data.procedure.overload,
        jsonInputArgName: response.data.inputArgName,
        resultCursorArgName: '',
      },
      result: {
        tableOwner: currentDraft.result.tableOwner || response.data.procedure.owner,
        tableName: currentDraft.result.tableName,
      },
      callTemplate: '',
      parameters: [],
    }))
  }, [client, draft, onChange])

  const load = useCallback(async (append = false, signal?: AbortSignal) => {
    if (!draft.datasourceId) return
    const request = ++catalogRequest.current
    setState((current) => ({ ...current, loading: true, error: '' }))
    const response = await getReportProcedures(client, draft.datasourceId, {
      owner: filters.owner,
      search: filters.search,
      after: append ? catalog.nextAfter : '',
      limit: 30,
    }, signal)
    if (signal?.aborted || request !== catalogRequest.current) return
    if (!response.ok) {
      setState((current) => ({ ...current, loading: false, error: response.error }))
      return
    }
    setCatalog((current) => ({
      items: append ? deduplicateProcedures([...current.items, ...response.data.items]) : response.data.items,
      hasMore: response.data.hasMore,
      nextAfter: response.data.nextAfter,
    }))
    setState((current) => ({ ...current, loading: false }))
  }, [catalog.nextAfter, client, draft.datasourceId, filters.owner, filters.search])

  useEffect(() => {
    if (!draft.datasourceId) return
    signatureCache.current = null
    setSignature(null)
    setFilters({ owner: '', search: '' })
    const controller = new AbortController()
    const request = ++catalogRequest.current
    setState((current) => ({ ...current, loading: true, error: '' }))
    void getReportProcedures(client, draft.datasourceId, { limit: 30 }, controller.signal).then((response) => {
      if (controller.signal.aborted || request !== catalogRequest.current) return
      if (response.ok) {
        setCatalog(response.data)
        setState((current) => ({ ...current, loading: false }))
      } else setState((current) => ({ ...current, loading: false, error: response.error }))
    })
    return () => { controller.abort(); catalogRequest.current += 1 }
  }, [client, draft.datasourceId])

  useEffect(() => {
    if (!draft.datasourceId) return
    setTableFilters({ owner: '', search: '' })
    setTableSchema(null)
    const controller = new AbortController()
    const request = ++tableCatalogRequest.current
    setTableState((current) => ({ ...current, loading: true, error: '' }))
    void getReportResultTables(client, draft.datasourceId, { limit: 30 }, controller.signal).then((response) => {
      if (controller.signal.aborted || request !== tableCatalogRequest.current) return
      if (response.ok) {
        setTableCatalog(response.data)
        setTableState((current) => ({ ...current, loading: false }))
      } else setTableState((current) => ({ ...current, loading: false, error: response.error }))
    })
    return () => { controller.abort(); tableCatalogRequest.current += 1 }
  }, [client, draft.datasourceId])

  useEffect(() => {
    if (!draft.datasourceId || !selectedOwner || !selectedName) {
      signatureCache.current = null
      setSignature(null)
      return
    }
    if (signatureCache.current && procedureKey(signatureCache.current.procedure) === selectedKey) {
      setSignature(signatureCache.current)
      return
    }
    const controller = new AbortController()
    const request = ++signatureRequest.current
    setState((current) => ({ ...current, inspecting: true, error: '' }))
    void getReportProcedureSignature(client, draft.datasourceId, { owner: selectedOwner, package: selectedPackage, name: selectedName, overload: selectedOverload }, controller.signal).then((response) => {
      if (controller.signal.aborted || request !== signatureRequest.current) return
      if (!response.ok) {
        setSignature(null)
        setState((current) => ({ ...current, inspecting: false, error: response.error }))
        return
      }
      signatureCache.current = response.data
      setSignature(response.data)
      setState((current) => ({ ...current, inspecting: false }))
    })
    return () => { controller.abort(); signatureRequest.current += 1 }
  }, [client, draft.datasourceId, selectedKey, selectedName, selectedOwner, selectedOverload, selectedPackage])

  useEffect(() => {
    if (!draft.datasourceId || !draft.result.tableOwner || !draft.result.tableName) {
      setTableSchema(null)
      return
    }
    if (tableSchema && resultTableKey(tableSchema.table) === selectedTableKey) return
    const controller = new AbortController()
    const request = ++tableSchemaRequest.current
    setTableState((current) => ({ ...current, inspecting: true, error: '' }))
    void getReportResultTableSchema(client, draft.datasourceId, { owner: draft.result.tableOwner, name: draft.result.tableName }, controller.signal).then((response) => {
      if (controller.signal.aborted || request !== tableSchemaRequest.current) return
      if (response.ok) {
        setTableSchema(response.data)
        setTableState((current) => ({ ...current, inspecting: false }))
        onChange((currentDraft) => resultTableKey(currentDraft.result) === resultTableKey(response.data.table)
          ? { ...currentDraft, columns: refreshReportColumnMetadata(response.data.columns, currentDraft.columns) }
          : currentDraft)
      } else {
        setTableSchema(null)
        setTableState((current) => ({ ...current, inspecting: false, error: response.error }))
      }
    })
    return () => { controller.abort(); tableSchemaRequest.current += 1 }
  }, [client, draft.datasourceId, draft.result.tableName, draft.result.tableOwner, onChange, selectedTableKey, tableSchema])

  const selectedLabel = useMemo(() => signature?.procedure.qualifiedName || qualifiedName(draft.procedure) || '尚未绑定存储过程', [draft.procedure, signature])
  if (!draft.datasourceId) return <div className={styles.empty} role="status">请先在“基本信息”中选择可用的 Oracle 数据源。</div>

  function updateFilter(key: keyof typeof filters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }))
  }

  async function loadResultTables(append = false, signal?: AbortSignal) {
    if (!draft.datasourceId) return
    const request = ++tableCatalogRequest.current
    setTableState((current) => ({ ...current, loading: true, error: '' }))
    const response = await getReportResultTables(client, draft.datasourceId, {
      owner: tableFilters.owner,
      search: tableFilters.search,
      after: append ? tableCatalog.nextAfter : '',
      limit: 30,
    }, signal)
    if (signal?.aborted || request !== tableCatalogRequest.current) return
    if (!response.ok) {
      setTableState((current) => ({ ...current, loading: false, error: response.error }))
      return
    }
    setTableCatalog((current) => ({
      items: append ? deduplicateResultTables([...current.items, ...response.data.items]) : response.data.items,
      hasMore: response.data.hasMore,
      nextAfter: response.data.nextAfter,
    }))
    setTableState((current) => ({ ...current, loading: false }))
  }

  async function selectResultTable(table: ReportResultTableSummary) {
    if (!draft.datasourceId) return
    const request = ++tableSchemaRequest.current
    setTableState((current) => ({ ...current, inspecting: true, error: '' }))
    const response = await getReportResultTableSchema(client, draft.datasourceId, table)
    if (request !== tableSchemaRequest.current) return
    if (!response.ok) {
      setTableState((current) => ({ ...current, inspecting: false, error: response.error }))
      return
    }
    setTableSchema(response.data)
    setTableState((current) => ({ ...current, inspecting: false }))
    onChange((currentDraft) => {
      const sameTable = resultTableKey(currentDraft.result) === resultTableKey(table)
      const columns = sameTable
        ? reconcileReportColumnsWithResultSchema(response.data.columns, currentDraft.columns)
        : reportColumnsFromResultSchema(response.data.columns)
      return {
        ...currentDraft,
        result: { tableOwner: table.owner, tableName: table.name },
        columns,
      }
    })
  }

  return <div className={styles.editor}>
    <p className={styles.accessHint}>使用所选数据源的普通 Oracle 账号查询全部已授权对象；Owner 留空会跨 Schema 搜索，也可输入本地同义词。选中后统一绑定真实 OWNER 和对象名，不需要管理员账号密码。</p>
    <form className={styles.search} onSubmit={(event) => { event.preventDefault(); void load(false) }}>
      <label>真实 Owner（可选）<input className={styles.mono} value={filters.owner} placeholder="留空搜索全部授权 Schema" onChange={(event) => updateFilter('owner', event.currentTarget.value)} /></label>
      <label>过程名称<input value={filters.search} placeholder="搜索过程、包或同义词" onChange={(event) => updateFilter('search', event.currentTarget.value)} /></label>
      <button type="submit" disabled={state.loading}><Search aria-hidden="true" />{state.loading ? '查询中…' : '查询 Oracle'}</button>
    </form>

    <div className={styles.catalog} aria-label="Oracle 存储过程查询结果">
      <div className={styles.catalogHeader}><strong>可见存储过程</strong><span>{catalog.items.length} 项</span></div>
      {catalog.items.length === 0 && !state.loading ? <p className={styles.empty}>当前条件下未查询到已授权过程，请核对真实 Owner、EXECUTE 权限或同义词。</p> : null}
      {catalog.items.map((item) => <button type="button" className={procedureKey(item) === selectedKey ? styles.selected : ''} aria-pressed={procedureKey(item) === selectedKey} onClick={() => void inspect(item)} key={procedureKey(item)}><code>{item.qualifiedName}</code><span>{item.argumentCount} 个参数</span></button>)}
      {catalog.hasMore ? <button type="button" className={styles.more} disabled={state.loading} onClick={() => void load(true)}><RefreshCw aria-hidden="true" />加载更多</button> : null}
    </div>

    <section className={styles.signature} aria-labelledby="report-procedure-signature-title">
      <div className={styles.signatureHeader}><div><h3 id="report-procedure-signature-title">已绑定过程</h3><code>{selectedLabel}</code></div>{signature ? <span className={signature.protocolReady ? styles.ready : styles.blocked}>{signature.protocolReady ? '协议可用' : '协议不兼容'}</span> : null}</div>
      {state.inspecting ? <p role="status">正在读取 Oracle 参数签名…</p> : null}
      {signature ? <>
        <div className={styles.arguments}>{signature.arguments.map((argument) => <div key={`${argument.position}-${argument.name}`}><code>{argument.name}</code><span>{argument.direction}</span><span>{argument.oracleType}</span><span>{argument.role === 'JSON_INPUT' ? 'JSON 输入' : '不支持'}</span></div>)}</div>
        {signature.protocolReady ? <dl className={styles.binding}><div><dt>JSON 输入参数</dt><dd><code>{signature.inputArgName}</code></dd></div><div><dt>结果获取方式</dt><dd>读取绑定结果表</dd></div></dl> : <ul className={styles.reasons}>{signature.blockingReasons.map((reason) => <li key={reason}>{reason}</li>)}</ul>}
      </> : <p className={styles.empty}>从上方 Oracle 查询结果中选择过程后，系统会自动绑定唯一 JSON 输入参数，不绑定任何出参。</p>}
    </section>
    <section className={styles.resultTable} aria-labelledby="report-result-table-title">
      <div><h3 id="report-result-table-title">Oracle 结果表绑定</h3><p>可绑定其他 Schema 已授权的物理表；系统读取完整结果并在 Excel 导出成功后清理数据，因此数据源账号需要 SELECT、DELETE 权限和稳定 ROWID。</p></div>
      <form className={styles.tableSearch} onSubmit={(event) => { event.preventDefault(); void loadResultTables(false) }}>
        <label>真实 Owner（可选）<input className={styles.mono} value={tableFilters.owner} placeholder="留空搜索全部授权 Schema" onChange={(event) => { const owner = event.currentTarget.value; setTableFilters((current) => ({ ...current, owner })) }} /></label>
        <label>结果表<input value={tableFilters.search} placeholder="搜索物理表或同义词" onChange={(event) => { const search = event.currentTarget.value; setTableFilters((current) => ({ ...current, search })) }} /></label>
        <button type="submit" disabled={tableState.loading}><Search aria-hidden="true" />{tableState.loading ? '查询中…' : '查询 Oracle'}</button>
      </form>
      <div className={styles.tableCatalog} aria-label="Oracle 结果表查询结果">
        <div className={styles.catalogHeader}><strong>可见结果表</strong><span>{tableCatalog.items.length} 项</span></div>
        {tableCatalog.items.length === 0 && !tableState.loading ? <p className={styles.empty}>当前条件下未查询到已授权物理表，请核对真实 Owner、SELECT/DELETE 权限或同义词。</p> : null}
        {tableCatalog.items.map((item) => <button type="button" className={resultTableKey(item) === selectedTableKey ? styles.selected : ''} aria-pressed={resultTableKey(item) === selectedTableKey} onClick={() => void selectResultTable(item)} key={resultTableKey(item)}><code>{item.qualifiedName}</code><span>{item.columnCount} 个字段</span></button>)}
        {tableCatalog.hasMore ? <button type="button" className={styles.more} disabled={tableState.loading} onClick={() => void loadResultTables(true)}><RefreshCw aria-hidden="true" />加载更多</button> : null}
      </div>
      <div className={styles.selectedTable}><strong>已选结果表</strong><code>{draft.result.tableOwner && draft.result.tableName ? `${draft.result.tableOwner}.${draft.result.tableName}` : '尚未选择'}</code>{tableState.inspecting ? <span role="status">正在读取字段…</span> : null}</div>
      {tableSchema ? <div className={styles.columnSummary}><div><strong>结果表字段</strong><span>已自动生成 {draft.columns.length} 个 Excel 字段，可到“Excel 映射”继续修改表头。</span></div>{tableSchema.columns.map((column) => <div key={column.name}><code>{column.name}</code><span>{column.oracleType}</span><span>{column.nullable ? '可空' : '必填'}</span></div>)}</div> : null}
      {tableState.error ? <div className={styles.error} role="alert">{tableState.error}</div> : null}
    </section>
    {state.error ? <div className={styles.error} role="alert">{state.error}</div> : null}
  </div>

}

function procedureKey(procedure: Pick<ReportDraft['procedure'], 'owner' | 'package' | 'name' | 'overload'>) {
  return [procedure.owner, procedure.package, procedure.name, procedure.overload].join('|').toUpperCase()
}

function qualifiedName(procedure: ReportDraft['procedure']) {
  if (!procedure.owner || !procedure.name) return ''
  return [procedure.owner, procedure.package, procedure.name].filter(Boolean).join('.') + (procedure.overload ? ` #${procedure.overload}` : '')
}

function deduplicateProcedures(items: ReportProcedureSummary[]) {
  return [...new Map(items.map((item) => [procedureKey(item), item])).values()]
}

function resultTableKey(table: { owner?: string; name?: string; tableOwner?: string; tableName?: string }) {
  return `${table.owner || table.tableOwner || ''}|${table.name || table.tableName || ''}`.toUpperCase()
}

function deduplicateResultTables(items: ReportResultTableSummary[]) {
  return [...new Map(items.map((item) => [resultTableKey(item), item])).values()]
}
