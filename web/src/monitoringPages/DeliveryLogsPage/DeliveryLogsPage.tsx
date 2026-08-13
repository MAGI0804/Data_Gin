import { RefreshCcw } from 'lucide-react'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { buildDeliveryLogListQuery, parseMonitoringPage, type MonitoringPage } from '../../monitoringRecords'
import { DataTable, Dialog, Drawer, FeedbackState, FilterToolbar, PageCanvas, PageHeader, PaginationControls, Section, StatusTag } from '../../ui'
import { formatMonitoringDate, parseDeliveryLog } from '../contracts'
import type { DeliveryLog, MonitoringClient } from '../types'
import styles from './DeliveryLogsPage.module.css'

export interface DeliveryLogsPageProps {
  client: MonitoringClient
  onRetryLog: (logId: number) => Promise<void>
}

export function DeliveryLogsPage({ client, onRetryLog }: DeliveryLogsPageProps) {
  const [destination, setDestination] = useState('')
  const [source, setSource] = useState('')
  const [status, setStatus] = useState('all')
  const [businessKey, setBusinessKey] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [applied, setApplied] = useState({ destination: '', source: '', success: '' as '' | 'true' | 'false', businessKey: '', startTime: '', endTime: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<MonitoringPage<DeliveryLog> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selectedLogID, setSelectedLogID] = useState<number | null>(null)
  const [retryingLogID, setRetryingLogID] = useState<number | null>(null)
  const [pendingRetryLog, setPendingRetryLog] = useState<DeliveryLog | null>(null)
  const [reloadVersion, setReloadVersion] = useState(0)
  const requestRef = useRef<AbortController | null>(null)

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    const query = buildDeliveryLogListQuery({ page, pageSize: 20, ...applied })
    void client(`/v1/delivery-logs?${query}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }).then((response) => {
      if (controller.signal.aborted) return
      const parsed = response.ok ? parseMonitoringPage<unknown>(response.data, 'logs') : null
      const logs = parsed?.list.map(parseDeliveryLog) ?? []
      if (parsed && logs.every((log): log is DeliveryLog => log !== null)) {
        setRecordsPage({ ...parsed, list: logs })
        return
      }
      setError(response.error?.message || '推送日志查询暂时不可用，请稍后重试。')
    }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [applied, client, page, reloadVersion])

  const logs = recordsPage?.list
  const pagination = recordsPage?.pagination
  const selectedLog = logs?.find((log) => log.id === selectedLogID) ?? null

  useEffect(() => {
    setSelectedLogID((current) => logs?.some((log) => log.id === current) ? current : logs?.[0]?.id ?? null)
  }, [logs])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setApplied({ destination, source, success: status === 'success' ? 'true' : status === 'failed' ? 'false' : '', businessKey, startTime, endTime })
  }

  async function retryPending() {
    if (!pendingRetryLog || retryingLogID !== null) return
    setRetryingLogID(pendingRetryLog.id)
    try { await onRetryLog(pendingRetryLog.id) } finally {
      setRetryingLogID(null)
      setPendingRetryLog(null)
      setReloadVersion((version) => version + 1)
    }
  }

  const requestRetry = (log: DeliveryLog) => {
    if (!log.success && retryingLogID === null) setPendingRetryLog(log)
  }

  return <PageCanvas>
    <PageHeader eyebrow="OPERATIONS" title="推送日志" description="按目标、来源和业务键查询交付结果；异常详情受保护，不展示请求与响应正文。" />
    <FilterToolbar summary={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}>
      <form className={styles.filters} onSubmit={submit} aria-label="推送日志筛选">
        <label>推送目标编码<input name="delivery_destination" value={destination} onChange={(event) => setDestination(event.currentTarget.value)} /></label>
        <label>来源编码<input name="delivery_source" value={source} onChange={(event) => setSource(event.currentTarget.value)} /></label>
        <label>交付状态<select value={status} onChange={(event) => setStatus(event.currentTarget.value)}><option value="all">全部</option><option value="success">成功</option><option value="failed">失败</option></select></label>
        <label>业务键<input name="delivery_business_key" value={businessKey} onChange={(event) => setBusinessKey(event.currentTarget.value)} /></label>
        <label>开始时间<input name="delivery_start_time" type="datetime-local" value={startTime} onChange={(event) => setStartTime(event.currentTarget.value)} /></label>
        <label>结束时间<input name="delivery_end_time" type="datetime-local" value={endTime} onChange={(event) => setEndTime(event.currentTarget.value)} /></label>
        <button type="submit" disabled={loading || retryingLogID !== null}>{loading ? '查询中…' : '查询'}</button>
      </form>
    </FilterToolbar>
    {error ? <FeedbackState kind="error" title="推送日志查询失败" description={`${error}${recordsPage ? ' 已保留最近一次成功数据。' : ''}`} /> : null}
    <Section title="交付记录" description="每页 20 条；仅失败记录可发起重试。" flush>
      {loading && !recordsPage ? <FeedbackState kind="loading" title="正在加载推送日志" /> : !logs?.length ? <FeedbackState kind="empty" title="暂无推送日志" /> : <DeliveryLogTable logs={logs} selectedLogID={selectedLogID} retryingLogID={retryingLogID} onSelect={setSelectedLogID} onRetry={requestRetry} />}
      <PaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading || retryingLogID !== null} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} />
    </Section>
    <Drawer open={Boolean(selectedLog)} size="narrow" title={selectedLog ? `推送日志 #${selectedLog.id}` : '推送日志详情'} description="仅展示安全元数据，不展示请求或响应正文。" onClose={() => setSelectedLogID(null)} footer={selectedLog && !selectedLog.success ? <button className={styles.primary} type="button" disabled={retryingLogID !== null} onClick={() => requestRetry(selectedLog)}><RefreshCcw aria-hidden="true" />{retryingLogID === selectedLog.id ? '重试中…' : '重试推送'}</button> : undefined}>{selectedLog ? <DeliveryLogDetail log={selectedLog} /> : null}</Drawer>
    <Dialog open={Boolean(pendingRetryLog)} role="alertdialog" title="确认重试推送日志" description="操作会再次向原推送目标发起交付请求。" closeDisabled={retryingLogID !== null} onClose={() => { if (retryingLogID === null) setPendingRetryLog(null) }} footer={<><button type="button" disabled={retryingLogID !== null} onClick={() => setPendingRetryLog(null)}>取消</button><button className={styles.primary} type="button" disabled={retryingLogID !== null} onClick={() => void retryPending()}>{retryingLogID === pendingRetryLog?.id ? '重试中…' : '确认重试'}</button></>}><p className={styles.dialogCopy}>确认重试失败日志 #{pendingRetryLog?.id}？</p></Dialog>
  </PageCanvas>
}

function DeliveryLogTable({ logs, selectedLogID, retryingLogID, onSelect, onRetry }: { logs: DeliveryLog[]; selectedLogID: number | null; retryingLogID: number | null; onSelect: (id: number) => void; onRetry: (log: DeliveryLog) => void }) {
  return <DataTable containerClassName={styles.table} minWidth={940} scrollLabel="推送日志列表"><thead><tr><th scope="col">状态</th><th scope="col">业务键</th><th scope="col">推送目标</th><th scope="col">来源</th><th scope="col">HTTP</th><th scope="col">推送时间</th><th scope="col">重试</th><th scope="col">操作</th></tr></thead><tbody>{logs.map((log) => <tr className={log.id === selectedLogID ? styles.selectedRow : undefined} key={log.id}><td><StatusTag tone={log.success ? 'success' : 'danger'}>{log.success ? '成功' : '失败'}</StatusTag></td><td>{log.business_key || '-'}</td><td>{log.destination_name || log.destination_code || `目标 #${log.destination_id}`}</td><td>{log.source_code || '-'}</td><td>{log.http_status || '-'}</td><td>{formatMonitoringDate(log.sent_at)}</td><td>{log.retry_count}</td><td><div className={styles.actions}><button type="button" aria-pressed={log.id === selectedLogID} onClick={() => onSelect(log.id)}>查看</button>{!log.success ? <button type="button" disabled={retryingLogID !== null} onClick={() => onRetry(log)}>{retryingLogID === log.id ? '重试中…' : '重试'}</button> : null}</div></td></tr>)}</tbody></DataTable>
}

function DeliveryLogDetail({ log }: { log: DeliveryLog }) {
  return <div className={styles.detail}><StatusTag tone={log.success ? 'success' : 'danger'}>{log.success ? '成功' : '失败'}</StatusTag><dl><div><dt>业务键</dt><dd>{log.business_key || '-'}</dd></div><div><dt>推送目标</dt><dd>{log.destination_name || log.destination_code || `目标 #${log.destination_id}`}</dd></div><div><dt>来源</dt><dd>{log.source_code || '-'}</dd></div><div><dt>Trace ID</dt><dd>{log.trace_id || '-'}</dd></div><div><dt>HTTP 状态</dt><dd>{log.http_status || '-'}</dd></div><div><dt>推送时间</dt><dd>{formatMonitoringDate(log.sent_at)}</dd></div><div><dt>重试次数</dt><dd>{log.retry_count}</dd></div></dl>{!log.success ? <p>该记录存在交付异常。请求与响应内容受保护，不在管理端展示。</p> : null}</div>
}
