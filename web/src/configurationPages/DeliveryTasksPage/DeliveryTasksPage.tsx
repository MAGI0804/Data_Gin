import { ArrowUpFromLine, Play, Plus } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { buildDeliveryTaskListQuery } from '../../monitoringRecords'
import { DataTable, Dialog, Drawer, FeedbackState, FilterToolbar, PageCanvas, PageHeader, PaginationControls, Section, StatusTag } from '../../ui'
import { buildDeliveryTaskSavePayload, deliveryTaskDraftFrom, deliveryTaskTriggerLabel, newDeliveryTaskDraft, parseDeliveryRunResult, parseDeliveryTaskDetail, parseDeliveryTaskPage, parseLegacyDeliveryTasks } from '../deliveryTaskContracts'
import type { ConfigurationClient, DeliveryTask, DeliveryTaskDraft, DestinationDefinition, SourceDefinition } from '../types'
import styles from './DeliveryTasksPage.module.css'

export function DeliveryTasksPage({ client, canManage, sources, destinations, onRefresh, refreshVersion }: { client: ConfigurationClient; canManage: boolean; sources: SourceDefinition[]; destinations: DestinationDefinition[]; onRefresh: () => Promise<void>; refreshVersion: number }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [destinationID, setDestinationID] = useState('all')
  const [applied, setApplied] = useState({ keyword: '', enabled: '' as '' | 'true' | 'false', destinationID: '' })
  const [page, setPage] = useState(1)
  const [reloadVersion, setReloadVersion] = useState(0)
  const [recordsPage, setRecordsPage] = useState<ReturnType<typeof parseDeliveryTaskPage>>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [draft, setDraft] = useState<DeliveryTaskDraft | null>(null)
  const [draftError, setDraftError] = useState('')
  const [saving, setSaving] = useState(false)
  const [detailLoadingID, setDetailLoadingID] = useState<number | null>(null)
  const [pendingRun, setPendingRun] = useState<DeliveryTask | null>(null)
  const [runningID, setRunningID] = useState<number | null>(null)
  const listRequestRef = useRef<AbortController | null>(null)
  const detailRequestRef = useRef<AbortController | null>(null)
  const saveInFlightRef = useRef(false)
  const runInFlightRef = useRef(false)
  const listQuery = useMemo(() => buildDeliveryTaskListQuery({ page, pageSize: 20, ...applied }), [applied, page])
  const running = runningID !== null

  useEffect(() => {
    listRequestRef.current?.abort()
    const controller = new AbortController()
    listRequestRef.current = controller
    async function load() {
      setLoading(true); setError('')
      try {
        const response = await client(`/v1/delivery-tasks?${listQuery}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
        if (controller.signal.aborted) return
        const nextPage = response.ok ? parseDeliveryTaskPage(response.data) : null
        if (nextPage) { setRecordsPage(nextPage); return }
        const legacyTasks = response.ok ? parseLegacyDeliveryTasks(response.data) : null
        if (legacyTasks) { const pageSize = 20; setRecordsPage({ list: legacyTasks.slice(0, pageSize), pagination: { page: 1, pageSize, total: legacyTasks.length, totalPages: legacyTasks.length ? 1 : 0 } }); setError('当前服务暂不支持推送任务分页或筛选，已显示未筛选的兼容数据。'); return }
        setError(response.error?.message || '推送任务列表暂时不可用，请稍后重试。')
      } catch {
        if (!controller.signal.aborted) setError('推送任务列表暂时不可用，请稍后重试。')
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    }
    void load()
    return () => controller.abort()
  }, [client, listQuery, refreshVersion, reloadVersion])

  useEffect(() => () => detailRequestRef.current?.abort(), [])

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (running) return
    setPage(1)
    setApplied({ keyword: query, enabled: status === 'enabled' ? 'true' : status === 'disabled' ? 'false' : '', destinationID: destinationID === 'all' ? '' : destinationID })
    setReloadVersion((value) => value + 1)
  }

  function resetFilters() {
    if (running) return
    setQuery(''); setStatus('all'); setDestinationID('all'); setPage(1)
    setApplied({ keyword: '', enabled: '', destinationID: '' }); setReloadVersion((value) => value + 1)
  }

  function openCreate() {
    if (!canManage || running) return
    detailRequestRef.current?.abort(); detailRequestRef.current = null; setDetailLoadingID(null)
    setNotice(''); setDraftError(''); setDraft(newDeliveryTaskDraft(sources[0]?.id, destinations[0]?.id))
  }

  async function openDetail(taskID: number) {
    if (detailLoadingID !== null || running) return
    detailRequestRef.current?.abort()
    const controller = new AbortController()
    detailRequestRef.current = controller
    setNotice(''); setDetailLoadingID(taskID)
    try {
      const response = await client(`/v1/delivery-tasks/${taskID}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
      if (controller.signal.aborted || detailRequestRef.current !== controller) return
      const task = response.ok ? parseDeliveryTaskDetail(response.data) : null
      if (!task) { setNotice(response.error?.message || '推送任务详情暂时不可用，请稍后重试。'); return }
      setDraftError(''); setDraft(deliveryTaskDraftFrom(task))
    } catch {
      if (!controller.signal.aborted) setNotice('推送任务详情暂时不可用，请稍后重试。')
    } finally {
      if (detailRequestRef.current === controller) { detailRequestRef.current = null; setDetailLoadingID(null) }
    }
  }

  async function saveDraft() {
    if (!canManage || !draft || saving || running || saveInFlightRef.current) return
    const validation = buildDeliveryTaskSavePayload(draft)
    if (!validation.ok) { setDraftError(validation.error); return }
    saveInFlightRef.current = true; setSaving(true); setDraftError('')
    try {
      const response = await client(draft.id ? `/v1/delivery-tasks/${draft.id}` : '/v1/delivery-tasks', { method: draft.id ? 'PUT' : 'POST', showResult: false, silentLoading: true, body: validation.payload })
      const saved = response.ok ? parseDeliveryTaskDetail(response.data) : null
      if (!saved || (draft.id !== null && saved.id !== draft.id)) { setDraftError(response.error?.message || '推送任务保存未完成，请稍后重试。'); return }
      setDraft(null); setNotice('推送任务已保存。'); setReloadVersion((value) => value + 1)
      try { await onRefresh() } catch { setNotice('推送任务已保存；关联配置刷新未完成，可手动刷新页面。') }
    } catch { setDraftError('推送任务保存未完成，请稍后重试。') } finally { saveInFlightRef.current = false; setSaving(false) }
  }

  async function confirmRun() {
    const task = pendingRun
    if (!canManage || !task || running || runInFlightRef.current) return
    runInFlightRef.current = true; setRunningID(task.id); setNotice('')
    try {
      const response = await client(`/v1/delivery-tasks/${task.id}/run`, { method: 'POST', showResult: false, silentLoading: true })
      const result = response.ok ? parseDeliveryRunResult(response.data) : null
      if (!result) { setNotice(response.error?.message || '推送任务未完成。'); return }
      setNotice(`执行完成：总计 ${result.totalCount}，成功 ${result.successCount}，失败 ${result.failedCount}，跳过 ${result.skippedCount}；追踪 ${result.traceID}。`)
      setReloadVersion((value) => value + 1)
      try { await onRefresh() } catch { setNotice(`执行已完成，追踪 ${result.traceID}；关联配置刷新未完成。`) }
    } catch { setNotice('推送任务未完成。') } finally { runInFlightRef.current = false; setRunningID(null); setPendingRun(null) }
  }

  const pagination = recordsPage?.pagination
  const tasks = recordsPage?.list ?? []
  const pageBusy = loading || detailLoadingID !== null || running
  return <PageCanvas>
    <PageHeader eyebrow="DELIVERY CONFIGURATION" title="推送任务" description="配置清洗结果表、推送目标与触发方式；手动执行最多处理 100 条 ready 数据。" actions={canManage ? <button className={styles.primary} type="button" disabled={running || detailLoadingID !== null} onClick={openCreate}><Plus aria-hidden="true" />{detailLoadingID !== null ? '读取详情中…' : '新增任务'}</button> : <StatusTag tone="neutral">只读权限</StatusTag>} />
    {notice ? <p className={styles.notice} role="status" aria-live="polite">{notice}</p> : null}
    <FilterToolbar summary={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}><form className={styles.filters} onSubmit={applyFilters} aria-label="推送任务筛选"><label>名称 / 清洗表<input value={query} disabled={running} onChange={(event) => setQuery(event.currentTarget.value)} /></label><label>状态<select value={status} disabled={running} onChange={(event) => setStatus(event.currentTarget.value)}><option value="all">全部</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label><label>推送目标<select value={destinationID} disabled={running} onChange={(event) => setDestinationID(event.currentTarget.value)}><option value="all">全部</option>{destinations.map((destination) => <option key={destination.id} value={destination.id}>{destination.name || destination.code}</option>)}</select></label><div className={styles.filterActions}><button type="button" disabled={pageBusy} onClick={resetFilters}>重置筛选</button><button className={styles.primary} type="submit" disabled={pageBusy}>{loading ? '查询中…' : '查询'}</button></div></form></FilterToolbar>
    {error ? <FeedbackState kind="error" title="推送任务查询提示" description={`${error}${recordsPage && !error.includes('兼容数据') ? ' 已保留最近一次成功数据。' : ''}`} /> : null}
    <Section title="任务配置" description="手动运行需要再次确认；任务或目标停用时不能执行。" flush>{loading && !recordsPage ? <FeedbackState kind="loading" title="正在加载推送任务" /> : tasks.length === 0 ? <FeedbackState kind="empty" title="暂无推送任务" description={canManage ? '可从右上角新增第一个推送任务。' : '当前没有可查看的推送任务。'} /> : <TaskTable canManage={canManage} tasks={tasks} destinations={destinations} detailLoadingID={detailLoadingID} runningID={runningID} onDetail={openDetail} onRun={setPendingRun} />}<PaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={pageBusy} onPrevious={() => setPage((value) => Math.max(1, value - 1))} onNext={() => setPage((value) => value + 1)} /></Section>
    <TaskDrawer canManage={canManage} draft={draft} sources={sources} destinations={destinations} error={draftError} saving={saving} onChange={setDraft} onClose={() => { if (!saving) { setDraft(null); setDraftError('') } }} onSave={saveDraft} />
    <Dialog open={Boolean(pendingRun)} role="alertdialog" title="确认执行推送任务" description="此操作会向真实目标系统发送业务数据。" closeDisabled={running} closeOnBackdrop={!running} onClose={() => { if (!running) setPendingRun(null) }} footer={<><button type="button" disabled={running} onClick={() => setPendingRun(null)}>取消</button><button className={styles.primary} type="button" disabled={running} onClick={() => void confirmRun()}>{running ? '执行中…' : '确认执行'}</button></>}>
      {pendingRun ? <p className={styles.dialogCopy}>将执行“{pendingRun.name}”，最多向 {destinationLabel(pendingRun, destinations)} 处理 100 条 ready 记录；成功发送和策略跳过的记录都会标记为已交付。</p> : null}
    </Dialog>
  </PageCanvas>
}

function TaskTable({ canManage, tasks, destinations, detailLoadingID, runningID, onDetail, onRun }: { canManage: boolean; tasks: DeliveryTask[]; destinations: DestinationDefinition[]; detailLoadingID: number | null; runningID: number | null; onDetail: (taskID: number) => Promise<void>; onRun: (task: DeliveryTask) => void }) {
  const busy = detailLoadingID !== null || runningID !== null
  return <DataTable containerClassName={styles.table} minWidth={980} scrollLabel="推送任务列表"><thead><tr><th scope="col">任务名称</th><th scope="col">触发方式</th><th scope="col">清洗表</th><th scope="col">推送目标</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead><tbody>{tasks.map((task) => { const destination = destinations.find((item) => item.id === task.destination_id); const runnable = task.enabled && Boolean(destination?.enabled); return <tr key={task.id}><td><span className={styles.identity}><ArrowUpFromLine aria-hidden="true" /><span><strong>{task.name}</strong><code>#{task.id}</code></span></span></td><td>{deliveryTaskTriggerLabel(task.trigger_type)}{task.trigger_type === 'schedule' && task.cron_expr ? <small>{task.cron_expr}</small> : null}</td><td><code>{task.clean_table}</code></td><td>{destinationLabel(task, destinations)}</td><td><StatusTag tone={task.enabled ? 'success' : 'neutral'}>{task.enabled ? '启用' : '停用'}</StatusTag></td><td><div className={styles.actions}><button type="button" disabled={busy} onClick={() => void onDetail(task.id)}>{detailLoadingID === task.id ? '读取中…' : '详情'}</button>{canManage ? <button type="button" title={runnable ? undefined : '任务或推送目标已停用或不可用'} disabled={busy || !runnable} onClick={() => onRun(task)}><Play aria-hidden="true" />{runningID === task.id ? '推送中…' : '手动运行'}</button> : null}</div></td></tr> })}</tbody></DataTable>
}

function TaskDrawer({ canManage, draft, sources, destinations, error, saving, onChange, onClose, onSave }: { canManage: boolean; draft: DeliveryTaskDraft | null; sources: SourceDefinition[]; destinations: DestinationDefinition[]; error: string; saving: boolean; onChange: (draft: DeliveryTaskDraft) => void; onClose: () => void; onSave: () => Promise<void> }) {
  const set = <K extends keyof DeliveryTaskDraft>(key: K, value: DeliveryTaskDraft[K]) => { if (draft) onChange({ ...draft, [key]: value }) }
  const readOnly = !canManage
  return <Drawer open={Boolean(draft)} title={draft?.id ? (canManage ? '推送任务详情与编辑' : '推送任务详情') : '新增推送任务'} description="任务策略 JSON 只承载任务级少推设置，不执行通用字段筛选。" size="medium" closeDisabled={saving} onClose={onClose} footer={<><button type="button" disabled={saving} onClick={onClose}>{canManage ? '取消' : '关闭'}</button>{canManage ? <button className={styles.primary} type="button" disabled={saving} onClick={() => void onSave()}>{saving ? '保存中…' : '保存任务'}</button> : null}</>}>
    {draft ? <form className={styles.form} onSubmit={(event) => { event.preventDefault(); void onSave() }}><label>任务名称<input required maxLength={100} disabled={saving || readOnly} value={draft.name} onChange={(event) => set('name', event.currentTarget.value)} /></label><label>来源<select required disabled={saving || readOnly} value={draft.sourceID} onChange={(event) => set('sourceID', event.currentTarget.value)}><option value="">选择数据源</option>{sources.map((source) => <option key={source.id} value={source.id}>#{source.id} {source.name || source.code}{source.enabled ? '' : '（已停用）'}</option>)}</select></label><label>清洗结果表<input required maxLength={100} className={styles.mono} disabled={saving || readOnly} value={draft.cleanTable} onChange={(event) => set('cleanTable', event.currentTarget.value)} /></label><label>推送目标<select required disabled={saving || readOnly} value={draft.destinationID} onChange={(event) => set('destinationID', event.currentTarget.value)}><option value="">选择推送目标</option>{destinations.map((destination) => <option key={destination.id} value={destination.id}>#{destination.id} {destination.name || destination.code}{destination.enabled ? '' : '（已停用）'}</option>)}</select></label><label>触发方式<select disabled={saving || readOnly} value={draft.triggerType} onChange={(event) => set('triggerType', event.currentTarget.value as DeliveryTask['trigger_type'])}><option value="manual">手动</option><option value="schedule">定时</option><option value="event">事件</option></select></label><label>Cron 表达式<input maxLength={100} className={styles.mono} required={draft.triggerType === 'schedule'} disabled={saving || readOnly || draft.triggerType !== 'schedule'} value={draft.cronExpr} onChange={(event) => set('cronExpr', event.currentTarget.value)} /></label><label className={styles.checkbox}><input type="checkbox" checked={draft.enabled} disabled={saving || readOnly} onChange={(event) => set('enabled', event.currentTarget.checked)} /><span>启用任务</span></label><label>任务策略 JSON<textarea rows={8} className={styles.mono} disabled={saving || readOnly} spellCheck={false} value={draft.filterJSON} onChange={(event) => set('filterJSON', event.currentTarget.value)} /></label><label>推送载荷模板<textarea rows={8} className={styles.mono} disabled={saving || readOnly} spellCheck={false} value={draft.payloadTemplate} onChange={(event) => set('payloadTemplate', event.currentTarget.value)} /></label><p className={styles.contractNote}>手动执行按来源与清洗表读取最多 100 条 ready 数据；策略跳过和成功发送的记录都会标记为已交付。模板字段按纯文本替换，不做 JSON 转义，请勿在模板中存放密钥。定时与事件模式目前仅保存配置。</p>{error ? <p className={styles.formError} role="alert">{error}</p> : null}</form> : null}
  </Drawer>
}

function destinationLabel(task: DeliveryTask, destinations: DestinationDefinition[]): string {
  const destination = destinations.find((item) => item.id === task.destination_id)
  return destination ? `${destination.name || destination.code} (#${destination.id})` : `目标 #${task.destination_id}`
}
