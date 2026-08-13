import { Copy, Search } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { redactMonitoringJSON } from '../../monitoring'
import { buildRunListQuery, parseMonitoringPage, type MonitoringPage } from '../../monitoringRecords'
import { FeedbackState, FilterToolbar, MasterDetail, PageCanvas, PageHeader, PaginationControls, Section, StatusTag } from '../../ui'
import { formatMonitoringDate, monitoringDurationLabel, monitoringStatusTone, parseMonitoringJSON, parsePipelineRun, parseStepRun, parseStepRunsResponse, pipelineRunStatusLabel, stepRunStatusLabel } from '../contracts'
import type { MonitoringClient, PipelineRun, StepRun } from '../types'
import styles from './StepRunsPage.module.css'

export interface StepRunsPageProps {
  client: MonitoringClient
  focusRunID: number | null
}

export function StepRunsPage({ client, focusRunID }: StepRunsPageProps) {
  const [runQuery, setRunQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [runType, setRunType] = useState('all')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [applied, setApplied] = useState({ traceID: '', status: '', runType: '', startTime: '', endTime: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<MonitoringPage<PipelineRun> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selectedRunID, setSelectedRunID] = useState<number | null>(null)
  const [stepRuns, setStepRuns] = useState<StepRun[]>([])
  const [stepLoading, setStepLoading] = useState(false)
  const [stepError, setStepError] = useState('')
  const [stepQuery, setStepQuery] = useState('')
  const [selectedStepID, setSelectedStepID] = useState<number | null>(null)
  const requestRef = useRef<AbortController | null>(null)
  const stepRequestRef = useRef<AbortController | null>(null)

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    const query = buildRunListQuery({ page, pageSize: 20, ...applied })
    void client(`/v1/runs?${query}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }).then((response) => {
      if (controller.signal.aborted) return
      const parsed = response.ok ? parseMonitoringPage<unknown>(response.data, 'runs') : null
      const runs = parsed?.list.map(parsePipelineRun) ?? []
      if (parsed && runs.every((run): run is PipelineRun => run !== null)) {
        setRecordsPage({ ...parsed, list: runs })
        return
      }
      setError(response.error?.message || '步骤运行查询暂时不可用，请稍后重试。')
    }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [applied, client, page])

  useEffect(() => () => stepRequestRef.current?.abort(), [])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setApplied({ traceID: runQuery, status: status === 'all' ? '' : status, runType: runType === 'all' ? '' : runType, startTime, endTime })
  }

  const selectRun = useCallback(async (runID: number) => {
    stepRequestRef.current?.abort()
    const controller = new AbortController()
    stepRequestRef.current = controller
    setSelectedRunID(runID)
    setStepRuns([])
    setSelectedStepID(null)
    setStepError('')
    setStepLoading(true)
    const response = await client(`/v1/pipeline-runs/${runID}/steps`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
    if (controller.signal.aborted) return
    const rawStepRuns = response.ok ? parseStepRunsResponse(response.data) : null
    const parsedStepRuns = rawStepRuns?.map(parseStepRun) ?? []
    if (!response.ok) setStepError(response.error?.message || '步骤详情暂时不可用，请稍后重试。')
    else if (!rawStepRuns || !parsedStepRuns.every((step): step is StepRun => step !== null)) setStepError('步骤详情返回格式无效，已拒绝展示。')
    else setStepRuns(parsedStepRuns)
    setStepLoading(false)
  }, [client])

  useEffect(() => {
    if (!focusRunID || selectedRunID === focusRunID || !recordsPage?.list.some((run) => run.id === focusRunID)) return
    void selectRun(focusRunID)
  }, [focusRunID, recordsPage, selectRun, selectedRunID])

  const runs = recordsPage?.list ?? []
  const pagination = recordsPage?.pagination
  const visibleRuns = runs.filter((run) => includesQuery([run.id, run.trace_id, run.run_type], runQuery))
  const visibleSteps = stepRuns.filter((step) => includesQuery([step.id, step.run_id, step.step_code, step.method_type, step.status, step.error_message], stepQuery))
  const selectedStep = visibleSteps.find((step) => step.id === selectedStepID) ?? visibleSteps[0] ?? null

  useEffect(() => {
    setSelectedStepID((current) => visibleSteps.some((step) => step.id === current) ? current : visibleSteps[0]?.id ?? null)
  }, [visibleSteps])

  return <PageCanvas>
    <PageHeader eyebrow="OPERATIONS" title="步骤运行" description="从当前运行页选择任务，查看脱敏后的步骤时间线、输入和输出。" />
    <FilterToolbar summary={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条运行`}>
      <form className={styles.filters} onSubmit={submit} aria-label="步骤运行筛选">
        <label>运行 / Trace ID<input name="step_run_query" value={runQuery} onChange={(event) => setRunQuery(event.currentTarget.value)} /></label>
        <SelectField label="状态" value={status} onChange={setStatus} options={[['running', '运行中'], ['success', '成功'], ['failed', '失败'], ['partial_success', '部分成功']]} />
        <SelectField label="运行类型" value={runType} onChange={setRunType} options={[['fetch', '拉取'], ['ingest', '接收'], ['transform', '清洗'], ['delivery', '推送']]} />
        <label>开始时间<input name="step_run_start_time" type="datetime-local" value={startTime} onChange={(event) => setStartTime(event.currentTarget.value)} /></label>
        <label>结束时间<input name="step_run_end_time" type="datetime-local" value={endTime} onChange={(event) => setEndTime(event.currentTarget.value)} /></label>
        <button type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
    </FilterToolbar>
    {error ? <FeedbackState kind="error" title="运行记录查询失败" description={`${error}${recordsPage ? ' 已保留最近一次成功数据。' : ''}`} /> : null}
    <Section title="运行与步骤" description="运行关键字会即时过滤当前页，提交查询后同时作为服务端 Trace ID 条件。" flush>
      <MasterDetail className={styles.explorer} masterWidth="34%" masterLabel="流水线运行" detailLabel="运行步骤与详情"
        master={<div className={styles.pane}><PaneHeading title="选择运行" meta={loading && !recordsPage ? '正在加载…' : `当前页 ${visibleRuns.length} 条`} />{visibleRuns.length === 0 ? <FeedbackState kind={loading ? 'loading' : 'empty'} title={loading ? '正在加载运行记录' : '暂无匹配运行'} /> : <div className={styles.list} role="list">{visibleRuns.map((run) => <button className={run.id === selectedRunID ? styles.selectedItem : undefined} type="button" key={run.id} aria-pressed={run.id === selectedRunID} onClick={() => void selectRun(run.id)}><span><strong>#{run.id}</strong><small>{run.trace_id || '-'} · {formatMonitoringDate(run.started_at)}</small></span><StatusTag tone={monitoringStatusTone(run.status)}>{pipelineRunStatusLabel(run.status)}</StatusTag></button>)}</div>}<PaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} /></div>}
        detail={<div className={styles.pane}><PaneHeading title="步骤时间线" meta={selectedRunID ? `运行 #${selectedRunID}` : '请选择运行'} /><label className={styles.stepSearch}><span>编码 / 类型 / 状态</span><span><Search aria-hidden="true" /><input value={stepQuery} onChange={(event) => setStepQuery(event.currentTarget.value)} /></span></label>{stepError ? <FeedbackState kind="error" title="步骤详情加载失败" description={stepError} /> : null}{visibleSteps.length === 0 ? <FeedbackState kind={stepLoading ? 'loading' : 'empty'} title={stepLoading ? '正在加载步骤详情' : selectedRunID ? '当前运行没有匹配步骤' : '请选择一个运行'} /> : <div className={`${styles.list} ${styles.stepList}`} role="list">{visibleSteps.map((step) => <button className={step.id === selectedStep?.id ? styles.selectedItem : undefined} type="button" key={step.id} aria-pressed={step.id === selectedStep?.id} onClick={() => setSelectedStepID(step.id)}><span><strong>{step.step_code || `步骤 #${step.id}`}</strong><small>{step.method_type || '-'} · {formatMonitoringDate(step.started_at)} · {monitoringDurationLabel(step.started_at, step.finished_at)}</small></span><StatusTag tone={monitoringStatusTone(step.status)}>{stepRunStatusLabel(step.status)}</StatusTag></button>)}</div>}<StepDetail step={selectedStep} /></div>} />
    </Section>
  </PageCanvas>
}

function SelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: Array<[string, string]> }) {
  return <label>{label}<select value={value} onChange={(event) => onChange(event.currentTarget.value)}><option value="all">全部</option>{options.map(([optionValue, optionLabel]) => <option value={optionValue} key={optionValue}>{optionLabel}</option>)}</select></label>
}

function PaneHeading({ title, meta }: { title: string; meta: string }) {
  return <header className={styles.paneHeading}><h3>{title}</h3><span>{meta}</span></header>
}

function StepDetail({ step }: { step: StepRun | null }) {
  return <aside className={styles.detail} aria-live="polite" aria-label="已选步骤详情">{step ? <><div className={styles.detailHeading}><div><strong>{step.step_code || `步骤 #${step.id}`}</strong><span>{step.method_type || '未声明方法类型'} · 运行 #{step.run_id}</span></div><StatusTag tone={monitoringStatusTone(step.status)}>{stepRunStatusLabel(step.status)}</StatusTag></div>{step.error_message ? <p className={styles.protectedError} role="alert">该步骤执行失败，错误详情已受保护。</p> : null}<div className={styles.jsonGrid}><CopyableRedactedJSON label="输入（脱敏）" value={parseMonitoringJSON(step.input_json)} /><CopyableRedactedJSON label="输出（脱敏）" value={parseMonitoringJSON(step.output_json)} /></div></> : <FeedbackState kind="empty" title="选择步骤后查看安全详情" />}</aside>
}

function CopyableRedactedJSON({ label, value }: { label: string; value: unknown }) {
  const [message, setMessage] = useState('')
  const redacted = redactMonitoringJSON(value)
  async function copy() {
    if (!navigator.clipboard?.writeText) { setMessage('当前浏览器不支持复制，请手动选择内容。'); return }
    try { await navigator.clipboard.writeText(jsonText(redacted)); setMessage('已复制脱敏内容。') } catch { setMessage('复制失败，请手动选择内容。') }
  }
  return <section><div className={styles.jsonHeading}><h3>{label}</h3><button type="button" onClick={() => void copy()}><Copy aria-hidden="true" />复制</button></div>{message ? <small role="status" aria-live="polite">{message}</small> : null}<pre className={styles.json} aria-label="只读 JSON">{jsonText(redacted)}</pre></section>
}

function includesQuery(values: Array<string | number | null | undefined>, query: string) {
  const normalized = query.trim().toLowerCase()
  return !normalized || values.some((value) => String(value ?? '').toLowerCase().includes(normalized))
}

function jsonText(value: unknown) {
  return typeof value === 'string' ? value : JSON.stringify(value ?? {}, null, 2)
}
