import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { RefreshCw, Search } from 'lucide-react'
import { getReportProcedureSignature, getReportProcedures, type ReportCenterClient } from '../../api'
import type { ReportDraft, ReportProcedureSignature, ReportProcedureSummary } from '../../types'
import styles from './ReportProcedureEditor.module.css'

export function ReportProcedureEditor({ client, draft, onChange }: { client: ReportCenterClient; draft: ReportDraft; onChange: (draft: ReportDraft) => void }) {
  const [filters, setFilters] = useState({ owner: draft.procedure.owner, search: '' })
  const [catalog, setCatalog] = useState<{ items: ReportProcedureSummary[]; hasMore: boolean; nextAfter: string }>({ items: [], hasMore: false, nextAfter: '' })
  const [signature, setSignature] = useState<ReportProcedureSignature | null>(null)
  const [state, setState] = useState({ loading: false, inspecting: false, error: '' })
  const catalogRequest = useRef(0)
  const signatureRequest = useRef(0)
  const signatureCache = useRef<ReportProcedureSignature | null>(null)
  const selectedKey = procedureKey(draft.procedure)
  const selectedOwner = draft.procedure.owner
  const selectedPackage = draft.procedure.package
  const selectedName = draft.procedure.name
  const selectedOverload = draft.procedure.overload

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
    onChange({
      ...draft,
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
        tableOwner: draft.result.tableOwner || response.data.procedure.owner,
        tableName: draft.result.tableName,
        runIdColumn: draft.result.runIdColumn || 'RUN_ID',
        rowIdColumn: draft.result.rowIdColumn || 'ROW_NO',
      },
      callTemplate: '',
      parameters: [],
    })
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

  const selectedLabel = useMemo(() => signature?.procedure.qualifiedName || qualifiedName(draft.procedure) || '尚未绑定存储过程', [draft.procedure, signature])
  if (!draft.datasourceId) return <div className={styles.empty} role="status">请先在“基本信息”中选择可用的 Oracle 数据源。</div>

  function updateFilter(key: keyof typeof filters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }))
  }

  function updateResult(key: keyof ReportDraft['result'], value: string) {
    onChange({ ...draft, result: { ...draft.result, [key]: value.toUpperCase() } })
  }

  return <div className={styles.editor}>
    <form className={styles.search} onSubmit={(event) => { event.preventDefault(); void load(false) }}>
      <label>Owner<input className={styles.mono} value={filters.owner} placeholder="可选，例如 REPORT" onChange={(event) => updateFilter('owner', event.currentTarget.value)} /></label>
      <label>过程名称<input value={filters.search} placeholder="搜索过程或包名" onChange={(event) => updateFilter('search', event.currentTarget.value)} /></label>
      <button type="submit" disabled={state.loading}><Search aria-hidden="true" />{state.loading ? '查询中…' : '查询 Oracle'}</button>
    </form>

    <div className={styles.catalog} aria-label="Oracle 存储过程查询结果">
      <div className={styles.catalogHeader}><strong>可见存储过程</strong><span>{catalog.items.length} 项</span></div>
      {catalog.items.length === 0 && !state.loading ? <p className={styles.empty}>当前条件下未查询到可见过程。</p> : null}
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
      <div><h3 id="report-result-table-title">Oracle 结果表绑定</h3><p>过程按系统注入的 run_id 写入持久化中间表；预览、Excel 和清理均只处理本次 run_id。</p></div>
      <div className={styles.resultFields}>
        <label>表 Owner<input className={styles.mono} value={draft.result.tableOwner} onChange={(event) => updateResult('tableOwner', event.currentTarget.value)} placeholder="例如 REPORT" /></label>
        <label>结果表名<input className={styles.mono} value={draft.result.tableName} onChange={(event) => updateResult('tableName', event.currentTarget.value)} placeholder="例如 SALES_REPORT_RESULT" /></label>
        <label>run_id 字段<input className={styles.mono} value={draft.result.runIdColumn} onChange={(event) => updateResult('runIdColumn', event.currentTarget.value)} placeholder="RUN_ID" /></label>
        <label>行游标字段<input className={styles.mono} value={draft.result.rowIdColumn} onChange={(event) => updateResult('rowIdColumn', event.currentTarget.value)} placeholder="ROW_NO" /></label>
      </div>
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
