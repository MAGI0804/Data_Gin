import { useEffect, useRef, useState, type FormEvent } from 'react'
import { PipelineRunPanel, type Pipeline } from '../../PipelineRunPanel'
import { buildRunListQuery, parseMonitoringPage, type MonitoringPage } from '../../monitoringRecords'
import { DataTable, FeedbackState, FilterToolbar, PageCanvas, PageHeader, PaginationControls, Section, StatusTag } from '../../ui'
import { formatMonitoringDate, monitoringDurationLabel, monitoringStatusTone, parsePipelineRun, pipelineRunStatusLabel } from '../contracts'
import type { MonitoringClient, PipelineRun } from '../types'
import styles from './PipelineRunsPage.module.css'

export interface PipelineRunsPageProps {
  client: MonitoringClient
  pipelines: Pipeline[]
  onLoadSteps: (runId: number) => void
  onPipelineRunCompleted: () => void
  refreshVersion: number
}

export function PipelineRunsPage({ client, pipelines, onLoadSteps, onPipelineRunCompleted, refreshVersion }: PipelineRunsPageProps) {
  const [traceID, setTraceID] = useState('')
  const [status, setStatus] = useState('all')
  const [runType, setRunType] = useState('all')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [applied, setApplied] = useState({ traceID: '', status: '', runType: '', startTime: '', endTime: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<MonitoringPage<PipelineRun> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const requestRef = useRef<AbortController | null>(null)

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
      setError(response.error?.message || '运行记录查询暂时不可用，请稍后重试。')
    }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [applied, client, page, refreshVersion])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setApplied({ traceID, status: status === 'all' ? '' : status, runType: runType === 'all' ? '' : runType, startTime, endTime })
  }

  const runs = recordsPage?.list ?? []
  const pagination = recordsPage?.pagination
  return <PageCanvas>
    <PageHeader eyebrow="OPERATIONS" title="流水线运行" description="按 Trace、状态和时间范围查询运行记录，并按需手动执行已启用流水线。" />
    <PipelineRunPanel pipelines={pipelines} client={client} onRunCompleted={onPipelineRunCompleted} />
    <FilterToolbar summary={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}>
      <form className={styles.filters} onSubmit={submit} aria-label="流水线运行筛选">
        <label>Trace ID<input name="run_trace_id" value={traceID} onChange={(event) => setTraceID(event.currentTarget.value)} /></label>
        <SelectField label="状态" value={status} onChange={setStatus} options={[['running', '运行中'], ['success', '成功'], ['failed', '失败'], ['partial_success', '部分成功']]} />
        <SelectField label="运行类型" value={runType} onChange={setRunType} options={[['fetch', '拉取'], ['ingest', '接收'], ['transform', '清洗'], ['delivery', '推送']]} />
        <label>开始时间<input name="run_start_time" type="datetime-local" value={startTime} onChange={(event) => setStartTime(event.currentTarget.value)} /></label>
        <label>结束时间<input name="run_end_time" type="datetime-local" value={endTime} onChange={(event) => setEndTime(event.currentTarget.value)} /></label>
        <button type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
    </FilterToolbar>
    {error ? <FeedbackState kind="error" title="运行记录查询失败" description={`${error}${recordsPage ? ' 已保留最近一次成功数据。' : ''}`} /> : null}
    <Section title="运行记录" description="每页 20 条；筛选条件在提交后冻结到当前查询。" flush>
      {loading && !recordsPage ? <FeedbackState kind="loading" title="正在加载运行记录" /> : runs.length === 0 ? <FeedbackState kind="empty" title="暂无运行记录" /> : <RunTable runs={runs} onLoadSteps={onLoadSteps} />}
      <PaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} />
    </Section>
  </PageCanvas>
}

function SelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: Array<[string, string]> }) {
  return <label>{label}<select value={value} onChange={(event) => onChange(event.currentTarget.value)}><option value="all">全部</option>{options.map(([optionValue, optionLabel]) => <option value={optionValue} key={optionValue}>{optionLabel}</option>)}</select></label>
}

function RunTable({ runs, onLoadSteps }: { runs: PipelineRun[]; onLoadSteps: (runId: number) => void }) {
  return <DataTable minWidth={940} scrollLabel="流水线运行记录"><thead><tr><th scope="col">ID / Trace ID</th><th scope="col">运行类型</th><th scope="col">触发方式</th><th scope="col">状态</th><th scope="col">成功 / 失败 / 总数</th><th scope="col">耗时</th><th scope="col">开始时间</th><th scope="col">明细</th></tr></thead><tbody>{runs.slice(0, 20).map((run) => <tr key={run.id}><td><strong>#{run.id}</strong><small>{run.trace_id || '-'}</small></td><td>{run.run_type}</td><td>{run.trigger_type || '-'}</td><td><StatusTag tone={monitoringStatusTone(run.status)}>{pipelineRunStatusLabel(run.status)}</StatusTag></td><td>{run.success_count} / {run.failed_count} / {run.total_count}</td><td>{monitoringDurationLabel(run.started_at, run.finished_at)}</td><td>{formatMonitoringDate(run.started_at)}</td><td><button type="button" onClick={() => onLoadSteps(run.id)}>查看步骤</button></td></tr>)}</tbody></DataTable>
}
