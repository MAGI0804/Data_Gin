import { Database, Download, Plus, Unplug } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { buildSourceListQuery } from '../../monitoringRecords'
import { parseSourceFetchSummary } from '../../sourceOperations'
import { DataTable, Drawer, FeedbackState, FilterToolbar, PageCanvas, PageHeader, PaginationControls, Section, StatusTag } from '../../ui'
import { buildSourceSavePayload, newSourceDraft, parseLegacySourceList, parseSourceDetail, parseSourcePage, sourceDraftFrom } from '../sourceContracts'
import type { ConfigurationClient, SourceDefinition, SourceDraft } from '../types'
import styles from './SourcesPage.module.css'

export interface SourcesPageProps {
  client: ConfigurationClient
  onFetchSource: (sourceID: number) => Promise<{ ok: boolean; data: unknown; error?: { message?: string } }>
  onTestSource: (sourceID: number) => Promise<{ ok: boolean; error?: { message?: string } }>
  refreshVersion: number
}

export function SourcesPage({ client, onFetchSource, onTestSource, refreshVersion }: SourcesPageProps) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [sourceType, setSourceType] = useState('')
  const [applied, setApplied] = useState({ keyword: '', enabled: '' as '' | 'true' | 'false', sourceType: '' })
  const [page, setPage] = useState(1)
  const [reloadVersion, setReloadVersion] = useState(0)
  const [recordsPage, setRecordsPage] = useState<ReturnType<typeof parseSourcePage>>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [draft, setDraft] = useState<SourceDraft | null>(null)
  const [draftError, setDraftError] = useState('')
  const [saving, setSaving] = useState(false)
  const [detailLoadingID, setDetailLoadingID] = useState<number | null>(null)
  const requestRef = useRef<AbortController | null>(null)
  const listQuery = useMemo(() => buildSourceListQuery({ page, pageSize: 20, ...applied }), [applied, page])

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    void client(`/v1/sources?${listQuery}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }).then((response) => {
      if (controller.signal.aborted) return
      const nextPage = response.ok ? parseSourcePage(response.data) : null
      if (nextPage) {
        setRecordsPage(nextPage)
        return
      }
      const legacySources = response.ok ? parseLegacySourceList(response.data) : null
      if (legacySources) {
        const pageSize = 20
        setRecordsPage({ list: legacySources.slice(0, pageSize), pagination: { page: 1, pageSize, total: legacySources.length, totalPages: legacySources.length ? 1 : 0 } })
        setError('当前服务暂不支持数据源分页或筛选，已显示未筛选的兼容数据。')
        return
      }
      setError(response.error?.message || '数据源列表暂时不可用，请稍后重试。')
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false)
    })
    return () => controller.abort()
  }, [client, listQuery, refreshVersion, reloadVersion])

  function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setApplied({ keyword: query, enabled: status === 'enabled' ? 'true' : status === 'disabled' ? 'false' : '', sourceType })
    setReloadVersion((version) => version + 1)
  }

  function resetQuery() {
    setQuery('')
    setStatus('all')
    setSourceType('')
    setPage(1)
    setApplied({ keyword: '', enabled: '', sourceType: '' })
    setReloadVersion((version) => version + 1)
  }

  async function openDetail(sourceID: number) {
    if (detailLoadingID !== null) return
    setNotice('')
    setDetailLoadingID(sourceID)
    try {
      const response = await client(`/v1/sources/${sourceID}`, { method: 'GET', showResult: false, silentLoading: true })
      const source = response.ok ? parseSourceDetail(response.data) : null
      if (!source) {
        setNotice(response.error?.message || '数据源详情暂时不可用。')
        return
      }
      setDraftError('')
      setDraft(sourceDraftFrom(source))
    } catch {
      setNotice('数据源详情暂时不可用。')
    } finally {
      setDetailLoadingID(null)
    }
  }

  async function saveDraft() {
    if (!draft || saving) return
    const validation = buildSourceSavePayload(draft)
    if (!validation.ok) {
      setNotice('')
      setDraftError(validation.error)
      return
    }
    setSaving(true)
    setDraftError('')
    try {
      const response = await client(draft.id ? `/v1/sources/${draft.id}` : '/v1/sources', {
        method: draft.id ? 'PUT' : 'POST',
        showResult: false,
        silentLoading: true,
        body: validation.payload,
      })
      if (!response.ok) {
        setDraftError(response.error?.message || '数据源保存未完成。')
        return
      }
      setDraft(null)
      setNotice('数据源已保存。')
      setReloadVersion((version) => version + 1)
    } catch {
      setDraftError('数据源保存未完成。')
    } finally {
      setSaving(false)
    }
  }

  const pagination = recordsPage?.pagination
  const sources = recordsPage?.list ?? []

  function openCreate() {
    if (detailLoadingID !== null) return
    setNotice('')
    setDraftError('')
    setDraft(newSourceDraft())
  }

  function closeDrawer() {
    if (saving) return
    setDraft(null)
    setDraftError('')
  }

  return <PageCanvas>
    <PageHeader eyebrow="DATA CONFIGURATION" title="数据源" description="维护接入配置、鉴权方式和来源查询键；连接配置中的敏感值只允许保留或轮换，不会回显。" actions={<button className={styles.primary} type="button" disabled={detailLoadingID !== null} onClick={openCreate}><Plus aria-hidden="true" />{detailLoadingID !== null ? '读取详情中…' : '新增数据源'}</button>} />
    {notice ? <p className={styles.notice} role="status" aria-live="polite">{notice}</p> : null}
    <FilterToolbar summary={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}>
      <form className={styles.filters} onSubmit={submitQuery} aria-label="数据源筛选">
        <label>名称 / 编码 / 鉴权<input name="source_query" value={query} onChange={(event) => setQuery(event.currentTarget.value)} /></label>
        <label>状态<select value={status} onChange={(event) => setStatus(event.currentTarget.value)}><option value="all">全部</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label>
        <label>类型<select value={sourceType || 'all'} onChange={(event) => setSourceType(event.currentTarget.value === 'all' ? '' : event.currentTarget.value)}><option value="all">全部</option><option value="api_poll">API</option><option value="webhook">Webhook</option><option value="database">数据库</option><option value="file">文件</option></select></label>
        <div className={styles.filterActions}><button type="button" onClick={resetQuery} disabled={loading}>重置筛选</button><button className={styles.primary} type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button></div>
      </form>
    </FilterToolbar>
    {error ? <FeedbackState kind="error" title="数据源查询提示" description={`${error}${recordsPage && !error.includes('兼容数据') ? ' 已保留最近一次成功数据。' : ''}`} /> : null}
    <Section title="数据源配置" description="连接测试会发起真实请求；手动拉取会创建运行记录并写入原始数据。" flush>
      {loading && !recordsPage ? <FeedbackState kind="loading" title="正在加载数据源" /> : sources.length === 0 ? <FeedbackState kind="empty" title="暂无数据源配置" description="可从右上角新增第一个数据源。" /> : <SourceTable sources={sources} detailLoadingID={detailLoadingID} onDetail={openDetail} onFetchSource={onFetchSource} onTestSource={onTestSource} />}
      <PaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading || detailLoadingID !== null} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} />
    </Section>
    <SourceDrawer draft={draft} error={draftError} saving={saving} onChange={setDraft} onClose={closeDrawer} onSave={saveDraft} />
  </PageCanvas>
}

function SourceTable({ sources, detailLoadingID, onDetail, onFetchSource, onTestSource }: { sources: SourceDefinition[]; detailLoadingID: number | null; onDetail: (sourceID: number) => Promise<void>; onFetchSource: SourcesPageProps['onFetchSource']; onTestSource: SourcesPageProps['onTestSource'] }) {
  const [fetchingID, setFetchingID] = useState<number | null>(null)
  const [testingID, setTestingID] = useState<number | null>(null)
  const [messageByID, setMessageByID] = useState<Record<number, string>>({})
  const operationRunning = fetchingID !== null || testingID !== null || detailLoadingID !== null

  async function fetchSource(sourceID: number) {
    if (operationRunning) return
    setFetchingID(sourceID)
    try {
      const response = await onFetchSource(sourceID)
      const summary = response.ok ? parseSourceFetchSummary(response.data) : null
      setMessageByID((current) => ({ ...current, [sourceID]: summary ? `拉取完成：成功 ${summary.successCount}/${summary.totalCount}，失败 ${summary.failedCount}；追踪 ${summary.traceID}` : response.error?.message || '拉取完成，但未收到可验证的结果摘要。' }))
    } catch {
      setMessageByID((current) => ({ ...current, [sourceID]: '数据源拉取未完成，请稍后重试。' }))
    } finally {
      setFetchingID(null)
    }
  }

  async function testSource(sourceID: number) {
    if (operationRunning) return
    setTestingID(sourceID)
    try {
      const response = await onTestSource(sourceID)
      setMessageByID((current) => ({ ...current, [sourceID]: response.ok ? '连接测试通过。' : response.error?.message || '连接测试未完成，请稍后重试。' }))
    } catch {
      setMessageByID((current) => ({ ...current, [sourceID]: '连接测试未完成，请稍后重试。' }))
    } finally {
      setTestingID(null)
    }
  }

  return <DataTable containerClassName={styles.table} minWidth={1080} scrollLabel="数据源列表">
    <thead><tr><th scope="col">数据源</th><th scope="col">类型</th><th scope="col">鉴权方式</th><th scope="col">接收键</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
    <tbody>{sources.map((source) => <tr key={source.id}>
      <td><span className={styles.identity}><Database aria-hidden="true" /><span><strong>{source.name}</strong><code>#{source.id} · {source.code}</code>{source.has_secret ? <small>连接配置已脱敏</small> : null}</span></span></td>
      <td><code>{sourceTypeLabel(source.source_type)}</code></td>
      <td>{source.auth_type || 'none'}</td>
      <td><code>{source.source_query_key || '-'}</code></td>
      <td><StatusTag tone={source.enabled ? 'success' : 'neutral'}>{source.enabled ? '启用' : '停用'}</StatusTag></td>
      <td><div className={styles.operationCell}><div className={styles.actions}><button type="button" disabled={operationRunning} onClick={() => void onDetail(source.id)}>{detailLoadingID === source.id ? '读取中…' : '详情'}</button><button type="button" disabled={operationRunning || !source.enabled} onClick={() => void testSource(source.id)}><Unplug aria-hidden="true" />{testingID === source.id ? '测试中…' : '测试连接'}</button><button type="button" disabled={operationRunning || !source.enabled || source.source_type === 'webhook'} onClick={() => void fetchSource(source.id)}><Download aria-hidden="true" />{fetchingID === source.id ? '拉取中…' : '手动拉取'}</button></div>{messageByID[source.id] ? <small className={styles.operationMessage} role="status" aria-live="polite">{messageByID[source.id]}</small> : null}</div></td>
    </tr>)}</tbody>
  </DataTable>
}

function SourceDrawer({ draft, error, saving, onChange, onClose, onSave }: { draft: SourceDraft | null; error: string; saving: boolean; onChange: (draft: SourceDraft) => void; onClose: () => void; onSave: () => Promise<void> }) {
  const set = <K extends keyof SourceDraft>(key: K, value: SourceDraft[K]) => { if (draft) onChange({ ...draft, [key]: value }) }
  return <Drawer open={Boolean(draft)} title={draft?.id ? '数据源详情与编辑' : '新增数据源'} description="配置、Schema 和去重键保存在服务端；请使用合法 JSON。" size="medium" closeDisabled={saving} onClose={onClose} footer={<><button type="button" disabled={saving} onClick={onClose}>取消</button><button className={styles.primary} type="button" disabled={saving} onClick={() => void onSave()}>{saving ? '保存中…' : '保存数据源'}</button></>}>
    {draft ? <form className={styles.form} onSubmit={(event) => { event.preventDefault(); void onSave() }}>
      {draft.hasSecret ? <p className={styles.secretNotice} role="status">配置中的敏感值已隐藏。保留“[已隐藏]”会保留原值；改为新值即可轮换，且不会回显旧值。</p> : null}
      <label>数据源名称<input required maxLength={100} disabled={saving} value={draft.name} onChange={(event) => set('name', event.currentTarget.value)} /></label>
      <label>数据源编码<input required maxLength={100} className={styles.mono} disabled={saving} value={draft.code} onChange={(event) => set('code', event.currentTarget.value)} /></label>
      <label>数据源类型<select disabled={saving} value={draft.sourceType} onChange={(event) => set('sourceType', event.currentTarget.value)}><option value="api_poll">API 轮询</option><option value="database">数据库</option><option value="webhook">Webhook</option></select></label>
      <label>鉴权类型<input maxLength={50} className={styles.mono} disabled={saving} value={draft.authType} onChange={(event) => set('authType', event.currentTarget.value)} /></label>
      <label>来源查询键<input maxLength={100} className={styles.mono} disabled={saving} value={draft.sourceQueryKey} onChange={(event) => set('sourceQueryKey', event.currentTarget.value)} /></label>
      <label className={styles.checkbox}><input type="checkbox" checked={draft.enabled} disabled={saving} onChange={(event) => set('enabled', event.currentTarget.checked)} /><span>启用数据源</span></label>
      <label>连接配置 JSON<textarea rows={10} className={styles.mono} disabled={saving} spellCheck={false} value={draft.configJSON} onChange={(event) => set('configJSON', event.currentTarget.value)} /></label>
      <label>Schema JSON<textarea rows={5} className={styles.mono} disabled={saving} spellCheck={false} value={draft.schemaJSON} onChange={(event) => set('schemaJSON', event.currentTarget.value)} /></label>
      <label>去重键 JSON 数组<textarea rows={4} className={styles.mono} disabled={saving} spellCheck={false} value={draft.dedupeKeys} onChange={(event) => set('dedupeKeys', event.currentTarget.value)} /></label>
      <p className={styles.contractNote}>API 测试会发起真实连通性请求；Webhook 不支持主动拉取。Schema 与去重键目前由服务端保存，未参与拉取校验。</p>
      {error ? <p className={styles.formError} role="alert">{error}</p> : null}
    </form> : null}
  </Drawer>
}

function sourceTypeLabel(value: string) {
  return ({ api_poll: 'API', api: 'API', webhook: 'Webhook', database: '数据库', file: '文件' } as Record<string, string>)[value] ?? (value || '-')
}
