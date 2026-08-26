import { useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
import DatePicker from 'antd/es/date-picker'
import dayjs from 'dayjs'
import { ChevronDown, ChevronLeft, ChevronRight, Download, Filter, Info, Play, Plus, Square, Trash2 } from 'lucide-react'
import { Button, DataTable, Dialog, FeedbackState, FilterToolbar, MetricStrip, PageCanvas, PageHeader, Section, StatusTag, type StatusTagTone } from '../../../ui'
import { cancelReportRun, createReportExport, createReportRun, getReportExport, getReportExportDownload, queryReportResults, getReportRun, getReportRunContract, type ReportCenterClient } from '../../api'
import { buildNewReportRunState, canStartNewReportRun, initialReportParameterValues, terminalReportExportStatuses, terminalReportRunStatuses, visibleReportParameters } from '../../queryParameters'
import { buildReportConditions, editableReportConditionValue, initialReportConditionValues, isReportInputListType, orderedReportInputEntries } from '../../refCursorConfig'
import type { ReportExport, ReportFilterOperator, ReportInputField, ReportParameter, ReportResultColumn, ReportResultFilter, ReportResultPage, ReportResultQuery, ReportRun, ReportRunContract } from '../../types'
import { ReportFieldDetailDrawer } from '../../components/ReportFieldDetailDrawer/ReportFieldDetailDrawer'
import { ReportInputQuerySelect } from '../../components/ReportInputQuerySelect/ReportInputQuerySelect'
import { useReportCatalog } from '../../useReportCatalog'
import styles from './ReportQueryPage.module.css'

const emptyResultQuery: ReportResultQuery = { filters: [], sort: [] }

export function ReportQueryPage({ client, navigation }: { client: ReportCenterClient; navigation?: ReactNode }) {
  const query = useMemo(() => ({ limit: 100 }), [])
  const { items, loading, loadingMore, error, hasMore, reload, loadMore } = useReportCatalog(client, query)
  const published = items.filter((report) => report.status === 'ACTIVE')
  const [selectedId, setSelectedId] = useState('')
  const [contract, setContract] = useState<ReportRunContract | null>(null)
  const [values, setValues] = useState<Record<string, unknown>>({})
  const [contractState, setContractState] = useState<{ loading: boolean; error: string }>({ loading: false, error: '' })
  const [run, setRun] = useState<ReportRun | null>(null)
  const [result, setResult] = useState<ReportResultPage | null>(null)
  const [cursorHistory, setCursorHistory] = useState<string[]>([''])
  const [cursorIndex, setCursorIndex] = useState(0)
  const [operation, setOperation] = useState<{ busy: boolean; error: string }>({ busy: false, error: '' })
  const [reportExport, setReportExport] = useState<ReportExport | null>(null)
	const [resultQuery, setResultQuery] = useState<ReportResultQuery>(emptyResultQuery)
	const [appliedQuery, setAppliedQuery] = useState<ReportResultQuery>(emptyResultQuery)
	const [filtersOpen, setFiltersOpen] = useState(false)
  const [parametersOpen, setParametersOpen] = useState(true)
  const [cancelState, setCancelState] = useState({ open: false, busy: false, error: '' })
  const [detailColumn, setDetailColumn] = useState<ReportResultColumn | null>(null)
  const pollAbortRef = useRef<AbortController | null>(null)
  const keepRunningRef = useRef<HTMLButtonElement>(null)

  useEffect(() => () => pollAbortRef.current?.abort(), [])

  useEffect(() => {
    pollAbortRef.current?.abort()
    setContract(null)
    setRun(null)
    setResult(null)
    setReportExport(null)
		setResultQuery(emptyResultQuery)
		setAppliedQuery(emptyResultQuery)
    setCursorHistory([''])
    setCursorIndex(0)
    setOperation({ busy: false, error: '' })
    if (!selectedId) return
    const reportId = Number(selectedId)
    const controller = new AbortController()
    setContractState({ loading: true, error: '' })
    void getReportRunContract(client, reportId, controller.signal).then((response) => {
      if (controller.signal.aborted) return
      if (!response.ok) {
        setContractState({ loading: false, error: response.error })
        return
      }
      setContract(response.data)
      setValues(response.data.jsonInput ? initialReportConditionValues(response.data.inputSchema) : initialReportParameterValues(response.data.parameters))
      setContractState({ loading: false, error: '' })
    })
    return () => controller.abort()
  }, [client, selectedId])

  async function submitRun(event: FormEvent) {
    event.preventDefault()
    if (!contract || operation.busy) return
    let runInput: Record<string, unknown>
    if (contract.jsonInput) {
      const normalized = buildReportConditions(contract.inputSchema, values)
      if (!normalized.ok) { setOperation({ busy: false, error: normalized.error }); return }
      runInput = normalized.conditions
    } else {
      const normalized = buildRunParameters(contract.parameters, values)
      if (!normalized.ok) { setOperation({ busy: false, error: normalized.error }); return }
      runInput = normalized.parameters
    }
    setOperation({ busy: true, error: '' })
    const response = await createReportRun(client, contract.definitionId, runInput, contract.executionMode, contract.jsonInput)
    if (!response.ok) {
      setOperation({ busy: false, error: response.error })
      return
    }
    setRun(response.data)
    setResult(null)
    setReportExport(null)
    setCursorHistory([''])
    setCursorIndex(0)
    await pollRun(response.data.id)
  }

  async function pollRun(runId: number) {
    pollAbortRef.current?.abort()
    const controller = new AbortController()
    pollAbortRef.current = controller
    while (!controller.signal.aborted) {
      const response = await getReportRun(client, runId, controller.signal)
      if (!response.ok) {
        if (!controller.signal.aborted) setOperation({ busy: false, error: response.error })
        return
      }
      setRun(response.data)
      if (terminalReportRunStatuses.has(response.data.status)) {
        setOperation({ busy: false, error: response.data.errorMessage })
				if (response.data.resultAvailable) await loadResults(runId, emptyResultQuery, '', 0, controller.signal)
        return
      }
      await wait(1500, controller.signal)
    }
  }

  async function resumeRun() {
    if (!run || operation.busy) return
    setOperation({ busy: true, error: '' })
    await pollRun(run.id)
  }

	async function loadResults(runId: number, query: ReportResultQuery, cursor: string, pageIndex: number, signal?: AbortSignal) {
    setOperation((current) => ({ ...current, busy: true, error: '' }))
		const response = await queryReportResults(client, runId, query, cursor, 100, signal)
    if (!response.ok) {
      if (!signal?.aborted) setOperation({ busy: false, error: response.error })
      return
    }
    setResult(response.data)
    setRun(response.data.run)
    setCursorIndex(pageIndex)
    setOperation({ busy: false, error: '' })
  }

  async function nextPage() {
    if (!run || !result?.pagination.nextCursor) return
    const nextIndex = cursorIndex + 1
    const nextHistory = [...cursorHistory.slice(0, nextIndex), result.pagination.nextCursor]
    setCursorHistory(nextHistory)
		await loadResults(run.id, appliedQuery, result.pagination.nextCursor, nextIndex)
  }

  async function previousPage() {
    if (!run || cursorIndex === 0) return
    const previousIndex = cursorIndex - 1
		await loadResults(run.id, appliedQuery, cursorHistory[previousIndex], previousIndex)
  }

	async function applyResultQuery() {
		if (!run?.resultAvailable || operation.busy || reportExport) return
		const normalized = normalizeResultQuery(resultQuery)
		setAppliedQuery(normalized)
		setCursorHistory([''])
		setCursorIndex(0)
		await loadResults(run.id, normalized, '', 0)
	}

  async function cancelRun() {
    if (!run?.canCancel || cancelState.busy) return
    setCancelState({ open: true, busy: true, error: '' })
    const response = await cancelReportRun(client, run.id)
    if (!response.ok) {
      setCancelState({ open: true, busy: false, error: response.error })
      return
    }
    setCancelState({ open: false, busy: false, error: '' })
    setRun(response.data)
    if (!terminalReportRunStatuses.has(response.data.status)) await pollRun(response.data.id)
  }

  async function startExport() {
    if (!run?.resultAvailable || reportExport) return
    setOperation({ busy: true, error: '' })
		const response = await createReportExport(client, run.id, appliedQuery)
    if (!response.ok) {
      setOperation({ busy: false, error: response.error })
      return
    }
    setReportExport(response.data)
    await pollExport(response.data.id)
  }

  async function pollExport(exportId: number) {
    const controller = new AbortController()
    pollAbortRef.current = controller
    while (!controller.signal.aborted) {
      const response = await getReportExport(client, exportId, controller.signal)
      if (!response.ok) {
        setOperation({ busy: false, error: response.error })
        return
      }
      setReportExport(response.data)
      if (terminalReportExportStatuses.has(response.data.status)) {
        setOperation({ busy: false, error: response.data.errorMessage })
        return
      }
      await wait(1800, controller.signal)
    }
  }

  async function downloadExport() {
    if (!reportExport?.canDownload) return
    const response = await getReportExportDownload(client, reportExport.id)
    if (!response.ok) {
      setOperation({ busy: false, error: response.error })
      return
    }
    window.location.assign(response.data.url)
  }

  function startNewRun() {
    if (!contract || !run || !canStartNewReportRun(run.status, reportExport?.status ?? null, operation.busy)) return
    pollAbortRef.current?.abort()
    pollAbortRef.current = null
    const next = buildNewReportRunState(contract.parameters)
    setRun(next.run)
    setResult(next.result)
    setReportExport(next.reportExport)
    setResultQuery(next.resultQuery)
    setAppliedQuery(next.appliedQuery)
    setCursorHistory(next.cursorHistory)
    setCursorIndex(next.cursorIndex)
    setFiltersOpen(next.filtersOpen)
    setParametersOpen(next.parametersOpen)
    setValues(contract.jsonInput ? initialReportConditionValues(contract.inputSchema) : next.values)
    setOperation(next.operation)
  }

  const frozen = Boolean(run)
  const canStartNewRun = run ? canStartNewReportRun(run.status, reportExport?.status ?? null, operation.busy) : false
  const activeStage = !selectedId ? 0 : !run ? 1 : !result ? 2 : 3
  return (
    <PageCanvas density="compact">
      {navigation}
      <PageHeader eyebrow="ORACLE EXECUTION" title="报表查询" description="系统自动传入 report_id（本次运行 ID），conditions 只包含已发布契约中配置的筛选字段；结果分页和正式导出复用同一次运行快照。" actions={run ? <div className={styles.pageActions}>{run.canCancel ? <button type="button" onClick={() => setCancelState({ open: true, busy: false, error: '' })}><Square aria-hidden="true" />取消运行</button> : null}<button type="button" onClick={startNewRun} disabled={!canStartNewRun}><Plus aria-hidden="true" />新建运行</button></div> : undefined} />
      <FilterToolbar summary={run ? <StatusTag tone={runTone(run)}>{runLabel(run.status)}</StatusTag> : <StatusTag tone="neutral">等待选择报表</StatusTag>}>
        <div className={styles.catalogSelector}><label className={styles.selector}>选择报表<select value={selectedId} onChange={(event) => setSelectedId(event.currentTarget.value)} disabled={loading || frozen || published.length === 0}><option value="">请选择已发布报表</option>{published.map((report) => <option value={report.id} key={report.id}>{report.name}</option>)}</select></label>{hasMore ? <button type="button" onClick={() => void loadMore()} disabled={loadingMore || frozen}>{loadingMore ? '正在加载…' : '加载更多'}</button> : null}</div>
      </FilterToolbar>
      <ol className={styles.runStages} aria-label="报表执行流程">
        {['选择报表', '配置条件', 'Oracle 执行', '预览与导出'].map((label, index) => <li className={index === activeStage ? styles.currentStage : index < activeStage ? styles.completedStage : undefined} aria-current={index === activeStage ? 'step' : undefined} key={label}><span>{String(index + 1).padStart(2, '0')}</span><strong>{label}</strong><small>{index < activeStage ? '已完成' : index === activeStage ? '当前步骤' : '等待'}</small></li>)}
      </ol>
      <Section title="筛选条件" description={contract ? `发布版本 #${contract.versionId} · 条件会统一写入存储过程 JSON` : '选择报表后读取已发布筛选契约。'} actions={<button type="button" aria-expanded={parametersOpen} onClick={() => setParametersOpen((open) => !open)}><ChevronDown className={parametersOpen ? styles.chevronOpen : undefined} aria-hidden="true" />{parametersOpen ? '收起条件' : '展开条件'}</button>}>
        {parametersOpen ? <>
        {contractState.loading ? <FeedbackState kind="loading" title="正在读取已发布筛选契约" /> : null}
        {contractState.error ? <FeedbackState kind="error" title="筛选契约加载失败" description={contractState.error} /> : null}
        {contract ? <form className={styles.parameterForm} onSubmit={(event) => void submitRun(event)}>{contract.jsonInput ? orderedReportInputEntries(contract.inputSchema).map(([code, field]) => <ConditionField client={client} reportId={contract.definitionId} code={code} disabled={frozen || operation.busy} field={field} key={code} value={values[code]} onChange={(value) => setValues((current) => ({ ...current, [code]: value }))} />) : visibleReportParameters(contract.parameters).map((parameter) => <ParameterField disabled={frozen || operation.busy} key={parameter.code} parameter={parameter} value={values[parameter.code]} onChange={(value) => setValues((current) => ({ ...current, [parameter.code]: value }))} />)}<div className={styles.runActions}><span>{frozen ? '本次条件已冻结；点击页头“新建运行”可恢复默认条件并重新查询。' : `${reportConditionCount(contract)} 个可填写筛选条件。`}</span><Button variant="primary" type="submit" disabled={frozen || operation.busy}><Play aria-hidden="true" />运行报表</Button></div></form> : null}
        {!contract && !contractState.loading && !contractState.error ? <FeedbackState kind="empty" title="尚未选择报表" description="请选择一份已发布且有查询权限的报表。" /> : null}
        </> : <div className={styles.collapsedParameters}>{contract ? `${reportConditionCount(contract)} 个筛选条件${frozen ? ' · 本次运行条件已冻结' : ''}` : '筛选区已收起'}</div>}
      </Section>
      {run ? <MetricStrip role="status" label="本次报表运行概览" items={[{ key: 'status', label: '运行状态', value: runLabel(run.status), detail: operation.busy ? '状态同步中' : '已同步' }, { key: 'run', label: '运行编号', value: `#${run.id}`, detail: contract ? `版本 #${contract.versionId}` : undefined }, { key: 'rows', label: '结果行数', value: run.rowCount.toLocaleString('zh-CN'), detail: result ? `当前页 ${result.rows.length} 行` : '等待结果' }, { key: 'retention', label: '结果保留', value: run.resultExpiresAt ? formatDateShort(run.resultExpiresAt) : '-', detail: run.errorMessage || '按运行快照管理' }]} /> : null}
      {operation.error ? <FeedbackState kind="error" title="操作未完成" description={operation.error} action={run && !terminalReportRunStatuses.has(run.status) ? <button type="button" onClick={() => void resumeRun()}>恢复状态查询</button> : undefined} /> : null}
      <Section title="结果预览" description={contract?.jsonInput ? '读取存储过程写入的结果表快照；分页和导出不会重新执行存储过程。' : '使用签名 Cursor 进行 Oracle Keyset 分页，不会重新执行存储过程。'} actions={run?.resultAvailable ? <div className={styles.resultActions}>{reportExport ? <StatusTag tone={exportTone(reportExport)}>{exportLabel(reportExport.status)}</StatusTag> : null}<button type="button" onClick={() => void startExport()} disabled={operation.busy || Boolean(reportExport)}><Download aria-hidden="true" />生成正式 Excel</button>{reportExport?.canDownload ? <Button variant="primary" onClick={() => void downloadExport()}>下载文件</Button> : null}</div> : undefined} flush>
		{result && contract?.executionMode !== 'REF_CURSOR' ? <ResultQueryToolbar page={result} query={resultQuery} open={filtersOpen} disabled={operation.busy || Boolean(reportExport)} onToggle={() => setFiltersOpen((value) => !value)} onChange={setResultQuery} onApply={() => void applyResultQuery()} /> : null}
        {operation.busy && !result ? <FeedbackState kind="loading" title={reportExport ? '正在生成并校验正式 Excel' : '正在执行报表'} description={reportExport ? exportProgress(reportExport) : 'Oracle 存储过程只会执行一次，请稍候。'} /> : null}
        {!run && !loading ? <FeedbackState kind="empty" title={published.length === 0 ? '暂无已发布报表' : '尚未执行报表'} description={published.length === 0 ? '发布版本可用后会出现在上方选择器。' : '填写筛选条件并运行后，结果将在这里分页展示。'} /> : null}
        {loading ? <FeedbackState kind="loading" title="正在读取可用报表" /> : error ? <FeedbackState kind="error" title="可用报表加载失败" description={error} action={<button type="button" onClick={reload}>重试</button>} /> : null}
        {result ? <ResultTable page={result} onInspect={setDetailColumn} /> : null}
        {result ? <div className={styles.pagination}><span>第 {cursorIndex + 1} 页 · 每页 {result.pagination.pageSize} 行</span><div><button type="button" onClick={() => void previousPage()} disabled={operation.busy || cursorIndex === 0}><ChevronLeft aria-hidden="true" />上一页</button><button type="button" onClick={() => void nextPage()} disabled={operation.busy || !result.pagination.hasMore}>下一页<ChevronRight aria-hidden="true" /></button></div></div> : null}
      </Section>
      <Dialog open={cancelState.open && Boolean(run?.canCancel)} role="alertdialog" title="确认取消报表运行" description={run ? `运行 #${run.id} 的取消请求提交后不能撤回。` : undefined} closeDisabled={cancelState.busy} closeOnBackdrop={!cancelState.busy} initialFocusRef={keepRunningRef} onClose={() => setCancelState({ open: false, busy: false, error: '' })} footer={<><button ref={keepRunningRef} type="button" disabled={cancelState.busy} onClick={() => setCancelState({ open: false, busy: false, error: '' })}>继续运行</button><Button variant="danger" disabled={cancelState.busy} onClick={() => void cancelRun()}>{cancelState.busy ? '正在提交…' : '确认取消运行'}</Button></>}><p className={styles.cancelWarning}>系统会请求 Oracle 侧停止当前执行；已经写入结果表的数据仍按运行清理策略处理。</p>{cancelState.error ? <p className={styles.cancelError} role="alert">{cancelState.error}</p> : null}</Dialog>
      <ReportFieldDetailDrawer column={detailColumn} onClose={() => setDetailColumn(null)} />
    </PageCanvas>
  )
}

function ParameterField({ parameter, value, disabled, onChange }: { parameter: ReportParameter; value: unknown; disabled: boolean; onChange: (value: unknown) => void }) {
  const id = `report-parameter-${parameter.code}`
  const hint = [`{{${parameter.code}}}`, parameter.oracleType, parameter.required ? '必填' : '选填', parameter.sensitive ? '敏感' : ''].filter(Boolean).join(' · ')
  if (parameter.controlType === 'CHECKBOX') return <label className={styles.field} htmlFor={id}><span>{parameter.label}{parameter.required ? ' *' : ''}</span><select id={id} required={parameter.required} disabled={disabled} value={value === true ? 'true' : value === false ? 'false' : ''} onChange={(event) => onChange(event.currentTarget.value === '' ? '' : event.currentTarget.value === 'true')}><option value="">{parameter.required ? '请选择' : '不传入 / NULL'}</option><option value="true">是</option><option value="false">否</option></select><small>{hint}</small></label>
  if (parameter.controlType === 'SELECT' || parameter.controlType === 'MULTI_SELECT') return <label className={styles.field} htmlFor={id}><span>{parameter.label}{parameter.required ? ' *' : ''}</span><select id={id} multiple={parameter.controlType === 'MULTI_SELECT'} required={parameter.required} disabled={disabled} value={parameter.controlType === 'MULTI_SELECT' ? toStringArray(value) : String(value ?? '')} onChange={(event) => onChange(parameter.controlType === 'MULTI_SELECT' ? Array.from(event.currentTarget.selectedOptions, (option) => option.value) : event.currentTarget.value)}><option value="" disabled={parameter.required}>请选择</option>{parameter.allowedValues.map((option) => <option value={String(option)} key={String(option)}>{String(option)}</option>)}</select><small>{hint}</small></label>
  if (parameter.controlType === 'DATE' || parameter.controlType === 'DATETIME') return <ReportDateField id={id} label={`${parameter.label}${parameter.required ? ' *' : ''}`} hint={parameter.errorMessage || hint} value={value} disabled={disabled} required={parameter.required} dateTime={parameter.controlType === 'DATETIME'} onChange={onChange} />
  const type = parameter.sensitive ? 'password' : 'text'
  const rules = parameter.validation
  const common = { id, required: parameter.required, disabled, value: String(value ?? ''), minLength: safeIntegerRule(rules.minLength), maxLength: parameter.maxLength ?? safeIntegerRule(rules.maxLength), pattern: typeof rules.pattern === 'string' ? rules.pattern : undefined, onChange: (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => onChange(event.currentTarget.value) }
  return <label className={styles.field} htmlFor={id}><span>{parameter.label}{parameter.required ? ' *' : ''}</span>{parameter.controlType === 'TEXTAREA' ? <textarea {...common} rows={3} /> : <input {...common} type={type} inputMode={parameter.controlType === 'NUMBER' ? 'decimal' : undefined} />}<small>{parameter.errorMessage || hint}</small></label>
}

function ConditionField({ client, reportId, code, field, value, disabled, onChange }: { client: ReportCenterClient; reportId: number; code: string; field: ReportInputField; value: unknown; disabled: boolean; onChange: (value: unknown) => void }) {
  const id = `report-condition-${code}`
  const label = `${field.displayName}${field.required ? ' *' : ''}`
  const list = isReportInputListType(field.type)
  const hint = [code, field.type, field.format, field.required ? '必填' : '选填'].filter(Boolean).join(' · ')
	if (field.queryName) return <label className={styles.field} htmlFor={id}><span>{label}</span><ReportInputQuerySelect client={client} reportId={reportId} conditionCode={code} inputId={id} value={value} required={field.required} disabled={disabled} multiple={list} numeric={field.type === 'number' || field.type === 'list[number]'} onChange={onChange} /><small>{hint} · 显示 name，提交 id{list ? ' 列表' : ''}</small></label>
  if (field.allowedValues?.length) {
    const options = field.allowedValues.map((item, index) => ({ key: String(index), value: editableReportConditionValue(item, { ...field, allowedValues: undefined }), label: displayConditionOption(item) }))
    const selected = list ? options.filter((option) => (Array.isArray(value) ? value : []).some((item) => comparableConditionValue(item) === comparableConditionValue(option.value))).map((option) => option.key) : options.find((option) => comparableConditionValue(option.value) === comparableConditionValue(value))?.key ?? ''
    return <label className={styles.field} htmlFor={id}><span>{label}</span><select id={id} multiple={list} required={field.required} disabled={disabled} value={selected} onChange={(event) => onChange(list ? Array.from(event.currentTarget.selectedOptions, (option) => option.value).filter(Boolean).map((key) => options[Number(key)]?.value).filter((item) => item !== undefined) : event.currentTarget.value === '' ? '' : options[Number(event.currentTarget.value)]?.value)}>{list ? null : <option value="" disabled={field.required}>请选择</option>}{options.map((option) => <option value={option.key} key={option.key}>{option.label}</option>)}</select><small>{hint}</small></label>
  }
  if (list || field.type === 'json') return <label className={styles.field} htmlFor={id}><span>{label}</span><textarea id={id} rows={3} required={field.required} disabled={disabled} value={Array.isArray(value) || (field.type === 'json' && value && typeof value === 'object') ? JSON.stringify(value) : String(value ?? '')} placeholder={list ? '["a","b"]' : '{"key":"value"}'} onChange={(event) => onChange(event.currentTarget.value)} /><small>{hint}</small></label>
  if (field.type === 'bool' || field.control === 'CHECKBOX') return <label className={styles.field} htmlFor={id}><span>{label}</span><select id={id} required={field.required} disabled={disabled} value={value === true ? 'true' : value === false ? 'false' : ''} onChange={(event) => onChange(event.currentTarget.value === '' ? '' : event.currentTarget.value === 'true')}><option value="">请选择</option><option value="true">是</option><option value="false">否</option></select><small>{hint}</small></label>
	if (field.control === 'DATE' || field.control === 'DATETIME') return <ReportDateField id={id} label={label} hint={hint} value={value} disabled={disabled} required={field.required} dateTime={field.control === 'DATETIME'} onChange={onChange} />
  const common = { id, required: field.required, disabled, value: String(value ?? ''), placeholder: field.example === undefined ? undefined : String(field.example), onChange: (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => onChange(event.currentTarget.value) }
  return <label className={styles.field} htmlFor={id}><span>{label}</span>{field.control === 'TEXTAREA' ? <textarea {...common} rows={3} /> : <input {...common} type="text" inputMode={field.type === 'number' || field.control === 'NUMBER' ? 'decimal' : undefined} />}<small>{hint}</small></label>
}

function ReportDateField({ id, label, hint, value, disabled, required, dateTime, onChange }: { id: string; label: string; hint: string; value: unknown; disabled: boolean; required: boolean; dateTime: boolean; onChange: (value: unknown) => void }) {
	const text = String(value ?? '')
	const parsed = text ? dayjs(text) : null
	const pickerValue = parsed?.isValid() ? parsed : null
	return <label className={`${styles.field} ${styles.dateField}`} htmlFor={id}><span>{label}</span><DatePicker id={id} aria-required={required} className={styles.datePicker} allowClear={!required} disabled={disabled} format={dateTime ? 'YYYY-MM-DD HH:mm:ss' : 'YYYY-MM-DD'} placeholder={dateTime ? '选择日期和时间' : '选择日期'} showTime={dateTime ? { format: 'HH:mm:ss' } : false} value={pickerValue} onChange={(next) => onChange(next ? next.format(dateTime ? 'YYYY-MM-DDTHH:mm:ss' : 'YYYY-MM-DD') : '')} /><small>{hint}</small></label>
}

function ResultTable({ page, onInspect }: { page: ReportResultPage; onInspect: (column: ReportResultColumn) => void }) {
  return <DataTable scrollLabel="报表查询结果"><thead><tr>{page.columns.map((column) => <th key={column.fieldId} scope="col"><span className={styles.columnHeader}>{column.header}<button type="button" aria-label={`查看 ${column.header} 字段详情`} onClick={() => onInspect(column)}><Info aria-hidden="true" /></button></span></th>)}</tr></thead><tbody>{page.rows.map((row) => <tr key={row.key}>{page.columns.map((column) => <td className={numericValueType(column.valueType) ? styles.numericCell : undefined} key={column.fieldId}>{displayCell(row.values[column.code], column.nullDisplay)}</td>)}</tr>)}</tbody></DataTable>
}

function ResultQueryToolbar({ page, query, open, disabled, onToggle, onChange, onApply }: { page: ReportResultPage; query: ReportResultQuery; open: boolean; disabled: boolean; onToggle: () => void; onChange: (query: ReportResultQuery) => void; onApply: () => void }) {
	const filterable = page.columns.filter((column) => column.filterable && column.allowedOperators.length > 0)
	const sortable = page.columns.filter((column) => column.sortable)
	return <div className={styles.queryToolbar}>
		<div className={styles.querySummary}><button type="button" onClick={onToggle} aria-expanded={open}><Filter aria-hidden="true" />结果筛选<ChevronDown aria-hidden="true" className={open ? styles.expandedIcon : undefined} /></button><span>{query.filters.length} 个条件 · {query.sort.length ? '已排序' : '默认顺序'}{disabled ? ' · 导出条件已冻结' : ''}</span></div>
		{open ? <div className={styles.queryEditor}>
			{query.filters.map((filter, index) => <ResultFilterField key={`${filter.field}-${index}`} columns={filterable} filter={filter} disabled={disabled} onChange={(next) => onChange({ ...query, filters: replaceAt(query.filters, index, next) })} onDelete={() => onChange({ ...query, filters: query.filters.filter((_, itemIndex) => itemIndex !== index) })} />)}
			<div className={styles.queryControls}><button type="button" disabled={disabled || filterable.length === 0 || query.filters.length >= 8} onClick={() => { const column = filterable[0]; if (column) onChange({ ...query, filters: [...query.filters, { field: column.fieldId, operator: column.allowedOperators[0] ?? 'EQ', value: '' }] }) }}><Plus aria-hidden="true" />添加条件</button><label>排序字段<select value={query.sort[0]?.field ?? ''} disabled={disabled} onChange={(event) => onChange({ ...query, sort: event.currentTarget.value ? [{ field: event.currentTarget.value, direction: query.sort[0]?.direction ?? 'ASC' }] : [] })}><option value="">默认顺序</option>{sortable.map((column) => <option key={column.fieldId} value={column.fieldId}>{column.header}</option>)}</select></label><label>方向<select value={query.sort[0]?.direction ?? 'ASC'} disabled={disabled || query.sort.length === 0} onChange={(event) => onChange({ ...query, sort: query.sort.length ? [{ ...query.sort[0], direction: event.currentTarget.value as 'ASC' | 'DESC' }] : [] })}><option value="ASC">升序</option><option value="DESC">降序</option></select></label><Button variant="primary" disabled={disabled} onClick={onApply}>应用筛选</Button></div>
		</div> : null}
	</div>
}

function ResultFilterField({ columns, filter, disabled, onChange, onDelete }: { columns: ReportResultPage['columns']; filter: ReportResultFilter; disabled: boolean; onChange: (filter: ReportResultFilter) => void; onDelete: () => void }) {
	const column = columns.find((item) => item.fieldId === filter.field) ?? columns[0]
	if (!column) return null
	const operator = column.allowedOperators.includes(filter.operator) ? filter.operator : column.allowedOperators[0]
	const noValue = operator === 'IS_NULL' || operator === 'IS_NOT_NULL'
	return <div className={styles.filterRow}><label>字段<select value={column.fieldId} disabled={disabled} onChange={(event) => { const next = columns.find((item) => item.fieldId === event.currentTarget.value); if (next) onChange({ field: next.fieldId, operator: next.allowedOperators[0] ?? 'EQ', value: '' }) }}>{columns.map((item) => <option key={item.fieldId} value={item.fieldId}>{item.header}</option>)}</select></label><label>条件<select value={operator} disabled={disabled} onChange={(event) => onChange({ field: column.fieldId, operator: event.currentTarget.value as ReportFilterOperator, value: '' })}>{column.allowedOperators.map((item) => <option key={item} value={item}>{operatorLabel(item)}</option>)}</select></label>{noValue ? <span className={styles.noValue}>无需填写值</span> : <label>值<input disabled={disabled} value={filterValue(filter.value)} placeholder={operator === 'IN' || operator === 'NOT_IN' || operator === 'BETWEEN' ? '多个值用英文逗号分隔' : '请输入筛选值'} onChange={(event) => onChange({ ...filter, field: column.fieldId, operator, value: event.currentTarget.value })} /></label>}<button type="button" aria-label={`删除 ${column.header} 条件`} disabled={disabled} onClick={onDelete}><Trash2 aria-hidden="true" /></button></div>
}

function normalizeResultQuery(query: ReportResultQuery): ReportResultQuery {
	return { sort: query.sort, filters: query.filters.map((filter) => {
		if (filter.operator === 'IS_NULL' || filter.operator === 'IS_NOT_NULL') return { field: filter.field, operator: filter.operator }
		const text = String(filter.value ?? '').trim()
		if (filter.operator === 'IN' || filter.operator === 'NOT_IN' || filter.operator === 'BETWEEN') return { ...filter, value: text.split(',').map((item) => item.trim()).filter(Boolean) }
		return { ...filter, value: typedFilterValue(text) }
	}) }
}
function typedFilterValue(value: string): unknown { if (/^-?\d+(?:\.\d+)?$/.test(value)) return value; if (value === 'true') return true; if (value === 'false') return false; return value }
function filterValue(value: unknown) { return Array.isArray(value) ? value.join(',') : String(value ?? '') }
function replaceAt<T>(items: T[], index: number, value: T) { return items.map((item, itemIndex) => itemIndex === index ? value : item) }
function operatorLabel(operator: ReportFilterOperator) { return ({ EQ: '等于', NE: '不等于', GT: '大于', GTE: '大于等于', LT: '小于', LTE: '小于等于', IN: '属于集合', NOT_IN: '不属于集合', IS_NULL: '为空', IS_NOT_NULL: '不为空', CONTAINS: '包含', STARTS_WITH: '开头为', BETWEEN: '区间' })[operator] }

function buildRunParameters(parameters: ReportParameter[], values: Record<string, unknown>): { ok: true; parameters: Record<string, unknown> } | { ok: false; error: string } {
  const result: Record<string, unknown> = {}
  for (const parameter of visibleReportParameters(parameters)) {
    const value = values[parameter.code]
    if (value === '' || value === undefined || (Array.isArray(value) && value.length === 0)) {
      if (parameter.required) return { ok: false, error: `${parameter.label} 为必填筛选条件。` }
      continue
    }
    if ((parameter.logicalType === 'integer' || parameter.logicalType === 'decimal') && (typeof value !== 'string' || !/^-?\d+(?:\.\d+)?$/.test(value))) return { ok: false, error: `${parameter.label} 必须填写有效数字。` }
    if (parameter.logicalType === 'integer' && typeof value === 'string' && !/^-?\d+$/.test(value)) return { ok: false, error: `${parameter.label} 必须填写整数。` }
    if (parameter.logicalType === 'datetime' && typeof value === 'string') {
      const dateTime = zonedDateTimeToRFC3339(value, parameter.timezone)
      if (!dateTime) return { ok: false, error: `${parameter.label} 不是有效日期时间，或时区配置无效。` }
      result[parameter.code] = dateTime
      continue
    }
    if (parameter.logicalType === 'json' && typeof value === 'string') {
      try { result[parameter.code] = JSON.parse(value) as unknown } catch { return { ok: false, error: `${parameter.label} 必须填写有效 JSON。` } }
      continue
    }
    result[parameter.code] = value
  }
  return { ok: true, parameters: result }
}
function toStringArray(value: unknown) { return Array.isArray(value) ? value.map(String) : [] }
function reportConditionCount(contract: ReportRunContract) { return contract.jsonInput ? Object.keys(contract.inputSchema).length : visibleReportParameters(contract.parameters).length }
function displayConditionOption(value: unknown) { return typeof value === 'string' ? value : JSON.stringify(value) }
function comparableConditionValue(value: unknown) { return JSON.stringify(value) }
function displayCell(value: unknown, nullDisplay: string) { if (value === null || value === undefined) return nullDisplay || '-'; if (typeof value === 'object') return JSON.stringify(value); return String(value) }
function numericValueType(valueType: string) { return valueType === 'integer' || valueType === 'decimal' || valueType === 'number' }
function wait(milliseconds: number, signal: AbortSignal) { return new Promise<void>((resolve) => { const finish = () => { window.clearTimeout(timer); signal.removeEventListener('abort', finish); resolve() }; const timer = window.setTimeout(finish, milliseconds); signal.addEventListener('abort', finish, { once: true }) }) }
function runTone(run: ReportRun): StatusTagTone { return ['SUCCEEDED', 'EXPORTED', 'RESULT_PURGED'].includes(run.status) ? 'success' : run.status === 'FAILED' || run.status === 'CANCELLED' ? 'danger' : run.status === 'UNKNOWN' || run.status === 'RECONCILING' || run.status === 'SUPERSEDED' ? 'warning' : 'running' }
function runLabel(status: ReportRun['status']) { return ({ QUEUED: '等待执行', RUNNING: '正在执行', CANCEL_REQUESTED: '正在取消', SUCCEEDED: '运行成功', FAILED: '运行失败', CANCELLED: '已取消', UNKNOWN: '状态待确认', RECONCILING: '正在对账', EXPORTING: '正在导出', EXPORTED: '已导出', RESULT_PURGING: '正在清理结果', RESULT_PURGED: '结果已清理', SUPERSEDED: '已被新运行替代' })[status] }
function exportTone(item: ReportExport): StatusTagTone { return item.status === 'READY' ? 'success' : item.status === 'FAILED' || item.status === 'CANCELLED' || item.status === 'EXPIRED' ? 'danger' : 'running' }
function exportLabel(status: ReportExport['status']) { return ({ PENDING: '等待导出', RUNNING: '生成并上传 Excel', READY: '文件就绪', FAILED: '导出失败', CANCELLED: '导出取消', EXPIRED: '文件已过期' })[status] }
function exportProgress(item: ReportExport) { return `${exportLabel(item.status)} · 已处理 ${item.processedRows.toLocaleString('zh-CN')} 行${item.currentSheet ? ` · ${item.currentSheet}` : ''}` }
function formatDateShort(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? '-' : new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date) }
function safeIntegerRule(value: unknown) { return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 ? value : undefined }
function zonedDateTimeToRFC3339(value: string, timezone: string) {
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2})?(?:Z|[+-]\d{2}:\d{2})$/.test(value)) {
    const parsed = new Date(value)
    return Number.isNaN(parsed.getTime()) ? '' : parsed.toISOString()
  }
  const matched = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(value)
  if (!matched) return ''
  const zone = timezone || Intl.DateTimeFormat().resolvedOptions().timeZone
  try {
    const expected = { year: Number(matched[1]), month: Number(matched[2]), day: Number(matched[3]), hour: Number(matched[4]), minute: Number(matched[5]), second: Number(matched[6] ?? 0) }
    let instant = Date.UTC(expected.year, expected.month - 1, expected.day, expected.hour, expected.minute, expected.second)
    const formatter = new Intl.DateTimeFormat('en-CA', { timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' })
    for (let attempt = 0; attempt < 3; attempt += 1) {
      const parts = Object.fromEntries(formatter.formatToParts(new Date(instant)).filter((part) => part.type !== 'literal').map((part) => [part.type, Number(part.value)]))
      const rendered = Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute, parts.second)
      const target = Date.UTC(expected.year, expected.month - 1, expected.day, expected.hour, expected.minute, expected.second)
      instant += target - rendered
    }
    const confirmed = Object.fromEntries(formatter.formatToParts(new Date(instant)).filter((part) => part.type !== 'literal').map((part) => [part.type, Number(part.value)]))
    if (confirmed.year !== expected.year || confirmed.month !== expected.month || confirmed.day !== expected.day || confirmed.hour !== expected.hour || confirmed.minute !== expected.minute || confirmed.second !== expected.second) return ''
    return new Date(instant).toISOString()
  } catch { return '' }
}
