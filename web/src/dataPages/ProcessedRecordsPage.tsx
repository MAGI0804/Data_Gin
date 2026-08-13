import { Search } from 'lucide-react'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import type { ClientResponse } from '../api/client'
import { parseDataStatisticsSummary, type DataStatisticsSummary } from '../monitoring'
import { buildCleanRecordsQuery, buildProcessedRecordsQuery, parseProcessedRecordsPage, type ProcessedRecordsPage as RecordsPage } from '../processedRecords'
import { DataTable, Dialog, FeedbackState, FilterToolbar, MetricStrip, PageCanvas, PageHeader, PaginationControls, Section, StatusTag } from '../ui'
import { cleanRecordStatusLabel, cleanRecordStatusTone, formatQualityScore, formatUnixTime, processedFields, qualityTone, type CleanRecord, type ProcessedData, unixTimestamp } from './processedPageSupport'
import styles from './ProcessedRecordsPage.module.css'

type ProcessedClient = (path: string, options: { method: 'GET'; signal?: AbortSignal; showResult?: boolean; silentLoading?: boolean }) => Promise<ClientResponse>
type View = 'legacy' | 'clean'

export function ProcessedRecordsPage({ client }: { client: ProcessedClient }) {
  const [view, setView] = useState<View>('clean')
  const [statistics, setStatistics] = useState<DataStatisticsSummary | null>(null)
  const [statisticsLoading, setStatisticsLoading] = useState(true)
  const [statisticsError, setStatisticsError] = useState('')
  const [statisticsRequest, setStatisticsRequest] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    setStatisticsLoading(true)
    setStatisticsError('')
    void client('/v1/data/statistics', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }).then((response) => {
      if (controller.signal.aborted) return
      const parsed = response.ok ? parseDataStatisticsSummary(response.data) : null
      if (parsed) {
        setStatistics(parsed)
      } else {
        setStatisticsError('处理统计暂时不可用，记录列表仍可继续查询。')
      }
    }).catch(() => {
      if (!controller.signal.aborted) setStatisticsError('处理统计暂时不可用，记录列表仍可继续查询。')
    }).finally(() => {
      if (!controller.signal.aborted) setStatisticsLoading(false)
    })
    return () => controller.abort()
  }, [client, statisticsRequest])

  return <PageCanvas>
    <PageHeader
      eyebrow="DATA QUALITY"
      title="处理结果"
      description="按业务键、数据类型、质量和处理状态查询清洗结果；旧链路数据保留独立视图。"
      context={<ViewSwitch view={view} onChange={setView} />}
    />
    {statisticsError ? <FeedbackState kind="error" title="处理统计暂时不可用" description={`${statisticsError}${statistics ? ' 当前展示的是上一次成功数据。' : ''}`} action={<button type="button" onClick={() => setStatisticsRequest((current) => current + 1)} disabled={statisticsLoading}>重试统计</button>} /> : null}
    {view === 'legacy'
      ? <LegacyProcessedQueryPanel client={client} />
      : <CleanRecordsQueryPanel client={client} statistics={statistics} statisticsLoading={statisticsLoading} />}
  </PageCanvas>
}

function ViewSwitch({ view, onChange }: { view: View; onChange: (view: View) => void }) {
  return <div className={styles.viewSwitch} role="tablist" aria-label="处理结果数据视图">
    <button type="button" role="tab" aria-selected={view === 'clean'} className={view === 'clean' ? styles.activeTab : undefined} onClick={() => onChange('clean')}>清洗记录</button>
    <button type="button" role="tab" aria-selected={view === 'legacy'} className={view === 'legacy' ? styles.activeTab : undefined} onClick={() => onChange('legacy')}>旧处理结果</button>
  </div>
}

function LegacyProcessedQueryPanel({ client }: { client: ProcessedClient }) {
  const [dataType, setDataType] = useState('')
  const [minQuality, setMinQuality] = useState('')
  const [maxQuality, setMaxQuality] = useState('')
  const [createdFrom, setCreatedFrom] = useState('')
  const [createdTo, setCreatedTo] = useState('')
  const [appliedQuery, setAppliedQuery] = useState({ dataType: '', minQuality: '', maxQuality: '', createdFrom: '', createdTo: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<RecordsPage<ProcessedData> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<ProcessedData | null>(null)
  const requestRef = useRef<AbortController | null>(null)

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    const query = buildProcessedRecordsQuery({ page, pageSize: 20, ...appliedQuery })
    void client(`/v1/data/processed/list?${query}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }).then((response) => {
      if (controller.signal.aborted) return
      const nextPage = response.ok ? parseProcessedRecordsPage<ProcessedData>(response.data) : null
      if (nextPage) setRecordsPage(nextPage)
      else setError(response.error?.message || '处理结果查询暂时不可用，请稍后重试。')
    }).catch(() => {
      if (!controller.signal.aborted) setError('处理结果查询暂时不可用，请稍后重试。')
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false)
    })
    return () => controller.abort()
  }, [appliedQuery, client, page])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setAppliedQuery({ dataType, minQuality, maxQuality, createdFrom: unixTimestamp(createdFrom), createdTo: unixTimestamp(createdTo) })
  }

  const records = recordsPage?.list ?? []
  return <>
    <FilterToolbar summary={loading && !recordsPage ? '正在加载…' : `共 ${recordsPage?.total ?? 0} 条`}>
      <form className={styles.filters} onSubmit={submit} aria-label="旧处理结果筛选">
        <TextField label="数据类型" name="processed_type" value={dataType} onChange={setDataType} />
        <TextField label="最低质量分" name="processed_min_quality" type="number" value={minQuality} onChange={setMinQuality} />
        <TextField label="最高质量分" name="processed_max_quality" type="number" value={maxQuality} onChange={setMaxQuality} />
        <TextField label="开始时间" name="processed_from" type="datetime-local" value={createdFrom} onChange={setCreatedFrom} />
        <TextField label="结束时间" name="processed_to" type="datetime-local" value={createdTo} onChange={setCreatedTo} />
        <button className={styles.primary} type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
    </FilterToolbar>
    <p className={styles.contractNote}>旧处理结果支持类型、质量和时间范围筛选；业务键属于清洗记录。</p>
    <MetricStrip label="旧处理结果指标" items={[
      { key: 'total', label: '命中记录', value: recordsPage?.total ?? 0 },
      { key: 'average', label: '平均质量', value: recordsPage ? recordsPage.averageQuality.toFixed(1) : '-' },
      { key: 'page-size', label: '当前页记录', value: records.length },
    ]} />
    {error ? <FeedbackState kind="error" title="旧处理结果查询失败" description={`${error} 已保留最近一次成功数据。`} /> : null}
    <Section title="旧处理结果" description="字段详情经过敏感信息过滤后展示。" flush>
      {loading && !recordsPage ? <FeedbackState kind="loading" title="正在加载旧处理结果" /> : records.length === 0 ? <FeedbackState kind="empty" title="暂无处理后数据" /> : <ProcessedDataList records={records} onSelect={setSelected} />}
      <PaginationControls page={recordsPage?.page ?? page} totalPages={recordsPage?.totalPages ?? 0} loading={loading} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} />
    </Section>
    <Dialog open={Boolean(selected)} title={selected ? `处理记录 #${selected.id}` : '处理记录'} description="字段内容只读，敏感键值已脱敏。" onClose={() => setSelected(null)}>
      {selected ? <pre className={styles.jsonPreview} aria-label="只读处理字段">{JSON.stringify(processedFields(selected.data_fields), null, 2)}</pre> : null}
    </Dialog>
  </>
}

function CleanRecordsQueryPanel({ client, statistics, statisticsLoading }: { client: ProcessedClient; statistics: DataStatisticsSummary | null; statisticsLoading: boolean }) {
  const [sourceID, setSourceID] = useState('')
  const [tableName, setTableName] = useState('')
  const [businessKey, setBusinessKey] = useState('')
  const [status, setStatus] = useState('')
  const [minQuality, setMinQuality] = useState('')
  const [maxQuality, setMaxQuality] = useState('')
  const [qualityBand, setQualityBand] = useState('all')
  const [createdFrom, setCreatedFrom] = useState('')
  const [createdTo, setCreatedTo] = useState('')
  const [appliedQuery, setAppliedQuery] = useState({ sourceID: '', tableName: '', businessKey: '', status: '', minQuality: '', maxQuality: '', createdFrom: '', createdTo: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<RecordsPage<CleanRecord> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<CleanRecord | null>(null)
  const requestRef = useRef<AbortController | null>(null)

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    const query = buildCleanRecordsQuery({ page, pageSize: 20, ...appliedQuery })
    void client(`/v1/data/clean-records/list?${query}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }).then((response) => {
      if (controller.signal.aborted) return
      const nextPage = response.ok ? parseProcessedRecordsPage<CleanRecord>(response.data) : null
      if (nextPage) setRecordsPage(nextPage)
      else setError(response.error?.message || '清洗记录查询暂时不可用，请稍后重试。')
    }).catch(() => {
      if (!controller.signal.aborted) setError('清洗记录查询暂时不可用，请稍后重试。')
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false)
    })
    return () => controller.abort()
  }, [appliedQuery, client, page])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setAppliedQuery({ sourceID, tableName, businessKey, status, minQuality, maxQuality, createdFrom: unixTimestamp(createdFrom), createdTo: unixTimestamp(createdTo) })
  }

  const records = recordsPage?.list ?? []
  return <>
    <FilterToolbar summary={loading && !recordsPage ? '正在加载…' : `当前页 ${records.length} / 共 ${recordsPage?.total ?? 0} 条`}>
      <form className={styles.filters} onSubmit={submit} aria-label="清洗记录筛选">
        <label className={styles.searchField}>业务键 / Raw ID / 内容<span><Search aria-hidden="true" /><input name="clean_business_key" type="search" value={businessKey} onChange={(event) => setBusinessKey(event.currentTarget.value)} placeholder="输入业务键" /></span></label>
        <TextField label="数据类型" name="clean_table_name" value={tableName} onChange={setTableName} placeholder="全部" />
        <label>质量<select name="clean_quality_band" value={qualityBand} onChange={(event) => { const next = event.currentTarget.value; setQualityBand(next); setMinQuality(next === 'high' ? '80' : ''); setMaxQuality(next === 'review' ? '79.99' : '') }}><option value="all">全部</option><option value="high">80 分及以上</option><option value="review">待复核（低于 80 分）</option></select></label>
        <details className={styles.advancedFilters}><summary>更多筛选</summary><div><TextField label="来源 ID" name="clean_source_id" type="number" value={sourceID} onChange={setSourceID} /><label>处理状态<select name="clean_status" value={status || 'all'} onChange={(event) => setStatus(event.currentTarget.value === 'all' ? '' : event.currentTarget.value)}><option value="all">全部</option><option value="ready">待推送</option><option value="invalid">无效</option><option value="delivered">已交付</option></select></label><TextField label="开始时间" name="clean_from" type="datetime-local" value={createdFrom} onChange={setCreatedFrom} /><TextField label="结束时间" name="clean_to" type="datetime-local" value={createdTo} onChange={setCreatedTo} /></div></details>
        <button className={styles.primary} type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
    </FilterToolbar>
    <MetricStrip aria-busy={statisticsLoading} label="处理统计" items={[
      { key: 'average', label: '平均质量', value: statistics?.averageQualityScore === null || statistics?.averageQualityScore === undefined ? '-' : statistics.averageQualityScore.toFixed(1) },
      { key: 'processed', label: '已处理', value: statistics?.processedCount ?? '-' },
      { key: 'failed', label: '处理失败', value: statistics?.errorCount ?? '-' },
    ]} />
    {error ? <FeedbackState kind="error" title="清洗记录查询失败" description={`${error} 已保留最近一次成功数据。`} /> : null}
    <Section title="清洗记录" description={loading && !recordsPage ? '正在加载…' : `查询命中 ${recordsPage?.total ?? 0} 条`} flush>
      {loading && !recordsPage ? <FeedbackState kind="loading" title="正在加载清洗记录" /> : records.length === 0 ? <FeedbackState kind="empty" title="暂无清洗记录" /> : <CleanRecordList records={records} onSelect={setSelected} />}
      <PaginationControls page={recordsPage?.page ?? page} totalPages={recordsPage?.totalPages ?? 0} loading={loading} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} />
    </Section>
    <Dialog open={Boolean(selected)} title={selected ? `清洗记录 #${selected.id}` : '清洗记录'} description="记录详情只读。" onClose={() => setSelected(null)}>
      {selected ? <dl className={styles.detailList}><div><dt>业务键</dt><dd>{selected.business_key || '-'}</dd></div><div><dt>数据类型</dt><dd>{selected.table_name || '-'}</dd></div><div><dt>Raw ID</dt><dd>#{selected.raw_record_id}</dd></div><div><dt>来源 ID</dt><dd>#{selected.source_id}</dd></div><div><dt>质量分数</dt><dd>{formatQualityScore(selected.quality_score)}</dd></div><div><dt>业务状态</dt><dd>{cleanRecordStatusLabel(selected.status)}</dd></div></dl> : null}
    </Dialog>
  </>
}

function ProcessedDataList({ records, onSelect }: { records: ProcessedData[]; onSelect: (record: ProcessedData) => void }) {
  return <DataTable minWidth={760} scrollLabel="旧处理结果列表">
    <thead><tr><th scope="col">数据类型</th><th scope="col">Raw ID</th><th scope="col">质量分数</th><th scope="col">处理时间</th><th scope="col">操作</th></tr></thead>
    <tbody>{records.map((record) => <tr key={record.id}><td><span className={styles.identity}><strong>{record.data_type || 'processed'}</strong><small>记录 #{record.id}</small></span></td><td>#{record.raw_data_id}</td><td>{formatQualityScore(record.quality_score)}</td><td>{formatUnixTime(record.created_at)}</td><td><button className={styles.linkButton} type="button" onClick={() => onSelect(record)}>查看字段</button></td></tr>)}</tbody>
  </DataTable>
}

function CleanRecordList({ records, onSelect }: { records: CleanRecord[]; onSelect: (record: CleanRecord) => void }) {
  return <DataTable containerClassName={styles.table} minWidth={980} scrollLabel="清洗记录列表">
    <thead><tr><th scope="col">业务键</th><th scope="col">数据类型</th><th scope="col">Raw ID</th><th scope="col">质量分数</th><th scope="col">质量</th><th scope="col">处理状态</th><th scope="col">处理时间</th><th scope="col">操作</th></tr></thead>
    <tbody>{records.map((record) => <tr key={record.id}><td><strong>{record.business_key || `#${record.id}`}</strong></td><td>{record.table_name || '-'}</td><td><span className={styles.identity}>#{record.raw_record_id}<small>来源 #{record.source_id}</small></span></td><td><span className={styles.qualityScore}><strong>{Math.round(record.quality_score)}</strong><progress className={record.quality_score >= 80 ? styles.highQuality : styles.reviewQuality} value={Math.max(0, Math.min(100, record.quality_score))} max="100" aria-label={`质量分 ${formatQualityScore(record.quality_score)}`} /></span></td><td><StatusTag tone={qualityTone(record.quality_score)}>{record.quality_score >= 80 ? '高质量' : '待复核'}</StatusTag></td><td><StatusTag tone={cleanRecordStatusTone(record.status)}>{cleanRecordStatusLabel(record.status)}</StatusTag></td><td>{formatUnixTime(record.created_at)}</td><td><button className={styles.linkButton} type="button" onClick={() => onSelect(record)}>查看</button></td></tr>)}</tbody>
  </DataTable>
}

function TextField({ label, name, value, onChange, type = 'text', placeholder }: { label: string; name: string; value: string; onChange: (value: string) => void; type?: string; placeholder?: string }) {
  return <label>{label}<input name={name} type={type} value={value} placeholder={placeholder} onChange={(event) => onChange(event.currentTarget.value)} /></label>
}
