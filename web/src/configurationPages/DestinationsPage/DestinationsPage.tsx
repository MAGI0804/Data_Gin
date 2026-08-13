import { Plus, Send, Unplug } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { buildDestinationListQuery } from '../../monitoringRecords'
import { DataTable, Dialog, Drawer, FeedbackState, FilterToolbar, PageCanvas, PageHeader, PaginationControls, Section, StatusTag } from '../../ui'
import { buildDestinationSavePayload, destinationDraftFrom, newDestinationDraft, parseDestinationDetail, parseDestinationPage, parseDestinationTestResult, parseLegacyDestinationList } from '../destinationContracts'
import type { ConfigurationClient, DestinationDefinition, DestinationDraft } from '../types'
import styles from './DestinationsPage.module.css'

export interface DestinationsPageProps {
  client: ConfigurationClient
  refreshVersion: number
}

export function DestinationsPage({ client, refreshVersion }: DestinationsPageProps) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [destinationType, setDestinationType] = useState('')
  const [applied, setApplied] = useState({ keyword: '', enabled: '' as '' | 'true' | 'false', destinationType: '' })
  const [page, setPage] = useState(1)
  const [reloadVersion, setReloadVersion] = useState(0)
  const [recordsPage, setRecordsPage] = useState<ReturnType<typeof parseDestinationPage>>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [draft, setDraft] = useState<DestinationDraft | null>(null)
  const [draftError, setDraftError] = useState('')
  const [saving, setSaving] = useState(false)
  const [detailLoadingID, setDetailLoadingID] = useState<number | null>(null)
  const [pendingTest, setPendingTest] = useState<DestinationDefinition | null>(null)
  const [testingID, setTestingID] = useState<number | null>(null)
  const listRequestRef = useRef<AbortController | null>(null)
  const detailRequestRef = useRef<AbortController | null>(null)
  const listQuery = useMemo(() => buildDestinationListQuery({ page, pageSize: 20, ...applied }), [applied, page])
  const testing = testingID !== null

  useEffect(() => {
    listRequestRef.current?.abort()
    const controller = new AbortController()
    listRequestRef.current = controller

    async function load() {
      setLoading(true)
      setError('')
      try {
        const response = await client(`/v1/destinations?${listQuery}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
        if (controller.signal.aborted) return
        const nextPage = response.ok ? parseDestinationPage(response.data) : null
        if (nextPage) {
          setRecordsPage(nextPage)
          return
        }
        const legacyDestinations = response.ok ? parseLegacyDestinationList(response.data) : null
        if (legacyDestinations) {
          const pageSize = 20
          setRecordsPage({ list: legacyDestinations.slice(0, pageSize), pagination: { page: 1, pageSize, total: legacyDestinations.length, totalPages: legacyDestinations.length ? 1 : 0 } })
          setError('当前服务暂不支持推送目标分页或筛选，已显示未筛选的兼容数据。')
          return
        }
        setError(response.error?.message || '推送目标列表暂时不可用，请稍后重试。')
      } catch {
        if (!controller.signal.aborted) setError('推送目标列表暂时不可用，请稍后重试。')
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    }

    void load()
    return () => controller.abort()
  }, [client, listQuery, refreshVersion, reloadVersion])

  useEffect(() => () => detailRequestRef.current?.abort(), [])

  function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (testing) return
    setPage(1)
    setApplied({ keyword: query, enabled: status === 'enabled' ? 'true' : status === 'disabled' ? 'false' : '', destinationType })
    setReloadVersion((version) => version + 1)
  }

  function resetQuery() {
    if (testing) return
    setQuery('')
    setStatus('all')
    setDestinationType('')
    setPage(1)
    setApplied({ keyword: '', enabled: '', destinationType: '' })
    setReloadVersion((version) => version + 1)
  }

  async function openDetail(destinationID: number) {
    if (detailLoadingID !== null || testing) return
    detailRequestRef.current?.abort()
    const controller = new AbortController()
    detailRequestRef.current = controller
    setNotice('')
    setDetailLoadingID(destinationID)
    try {
      const response = await client(`/v1/destinations/${destinationID}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
      if (controller.signal.aborted || detailRequestRef.current !== controller) return
      const destination = response.ok ? parseDestinationDetail(response.data) : null
      if (!destination) {
        setNotice(response.error?.message || '推送目标详情暂时不可用。')
        return
      }
      setDraftError('')
      setDraft(destinationDraftFrom(destination))
    } catch {
      if (!controller.signal.aborted) setNotice('推送目标详情暂时不可用。')
    } finally {
      if (detailRequestRef.current === controller) {
        detailRequestRef.current = null
        setDetailLoadingID(null)
      }
    }
  }

  function openCreate() {
    if (testing) return
    detailRequestRef.current?.abort()
    detailRequestRef.current = null
    setDetailLoadingID(null)
    setNotice('')
    setDraftError('')
    setDraft(newDestinationDraft())
  }

  function closeDrawer() {
    if (saving) return
    setDraft(null)
    setDraftError('')
  }

  async function saveDraft() {
    if (!draft || saving) return
    const validation = buildDestinationSavePayload(draft)
    if (!validation.ok) {
      setNotice('')
      setDraftError(validation.error)
      return
    }
    setSaving(true)
    setDraftError('')
    try {
      const response = await client(draft.id ? `/v1/destinations/${draft.id}` : '/v1/destinations', {
        method: draft.id ? 'PUT' : 'POST',
        showResult: false,
        silentLoading: true,
        body: validation.payload,
      })
      const savedDestination = response.ok ? parseDestinationDetail(response.data) : null
      if (!savedDestination || (draft.id !== null && savedDestination.id !== draft.id)) {
        setDraftError(response.error?.message || '推送目标保存未完成。')
        return
      }
      setDraft(null)
      setNotice('推送目标已保存。')
      setReloadVersion((version) => version + 1)
    } catch {
      setDraftError('推送目标保存未完成。')
    } finally {
      setSaving(false)
    }
  }

  async function confirmTest() {
    const target = pendingTest
    if (!target || testing) return
    setTestingID(target.id)
    setNotice('')
    try {
      const response = await client(`/v1/destinations/${target.id}/test`, { method: 'POST', showResult: false, silentLoading: true })
      const passed = response.ok && parseDestinationTestResult(response.data, target.id)
      setNotice(passed ? '连通性测试通过；仅发送了无业务载荷的 HEAD 或 GET 请求。' : response.error?.message || '连通性测试未完成。')
    } catch {
      setNotice('连通性测试未完成。')
    } finally {
      setTestingID(null)
      setPendingTest(null)
    }
  }

  const pagination = recordsPage?.pagination
  const destinations = recordsPage?.list ?? []
  const pageBusy = loading || detailLoadingID !== null || testing

  return <PageCanvas>
    <PageHeader eyebrow="DELIVERY CONFIGURATION" title="推送目标" description="维护目标系统、接口协议与连接配置；测试连接会访问真实地址，但不会发送业务载荷。" actions={<button className={styles.primary} type="button" disabled={testing || detailLoadingID !== null} onClick={openCreate}><Plus aria-hidden="true" />{detailLoadingID !== null ? '读取详情中…' : '新增目标'}</button>} />
    {notice ? <p className={styles.notice} role="status" aria-live="polite">{notice}</p> : null}
    <FilterToolbar summary={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}>
      <form className={styles.filters} onSubmit={submitQuery} aria-label="推送目标筛选">
        <label>名称 / 编码<input name="destination_query" value={query} disabled={testing} onChange={(event) => setQuery(event.currentTarget.value)} /></label>
        <label>状态<select value={status} disabled={testing} onChange={(event) => setStatus(event.currentTarget.value)}><option value="all">全部</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label>
        <label>接口类型<input name="destination_type" value={destinationType} disabled={testing} placeholder="例如 http、soap" onChange={(event) => setDestinationType(event.currentTarget.value)} /></label>
        <div className={styles.filterActions}><button type="button" onClick={resetQuery} disabled={pageBusy}>重置筛选</button><button className={styles.primary} type="submit" disabled={pageBusy}>{loading ? '查询中…' : '查询'}</button></div>
      </form>
    </FilterToolbar>
    {error ? <FeedbackState kind="error" title="推送目标查询提示" description={`${error}${recordsPage && !error.includes('兼容数据') ? ' 已保留最近一次成功数据。' : ''}`} /> : null}
    <Section title="目标配置" description="敏感配置不会回显；编辑时保留“[已隐藏]”即可沿用原值。" flush>
      {loading && !recordsPage ? <FeedbackState kind="loading" title="正在加载推送目标" /> : destinations.length === 0 ? <FeedbackState kind="empty" title="暂无推送目标" description="可从右上角新增第一个推送目标。" /> : <DestinationTable destinations={destinations} detailLoadingID={detailLoadingID} testingID={testingID} onDetail={openDetail} onTest={setPendingTest} />}
      <PaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={pageBusy} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} />
    </Section>
    <DestinationDrawer draft={draft} error={draftError} saving={saving} onChange={setDraft} onClose={closeDrawer} onSave={saveDraft} />
    <Dialog open={Boolean(pendingTest)} role="alertdialog" title="确认测试推送目标" description="此操作会访问目标系统的真实网络地址。" closeDisabled={testing} closeOnBackdrop={!testing} onClose={() => { if (!testing) setPendingTest(null) }} footer={<><button type="button" disabled={testing} onClick={() => setPendingTest(null)}>取消</button><button className={styles.primary} type="button" disabled={testing} onClick={() => void confirmTest()}>{testing ? '测试中…' : '确认测试'}</button></>}>
      {pendingTest ? <p className={styles.dialogCopy}>将向“{pendingTest.name}”配置的目标地址发起真实连通性请求。系统只允许无业务载荷的 HEAD 或 GET；请先确认该 GET 端点没有副作用。</p> : null}
    </Dialog>
  </PageCanvas>
}

function DestinationTable({ destinations, detailLoadingID, testingID, onDetail, onTest }: { destinations: DestinationDefinition[]; detailLoadingID: number | null; testingID: number | null; onDetail: (destinationID: number) => Promise<void>; onTest: (destination: DestinationDefinition) => void }) {
  const operationRunning = detailLoadingID !== null || testingID !== null
  return <DataTable containerClassName={styles.table} minWidth={860} scrollLabel="推送目标列表">
    <thead><tr><th scope="col">目标系统</th><th scope="col">目标编码</th><th scope="col">接口类型</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
    <tbody>{destinations.map((destination) => <tr key={destination.id}>
      <td><span className={styles.identity}><Send aria-hidden="true" /><span><strong>{destination.name}</strong><code>#{destination.id}</code>{destination.has_secret ? <small>连接配置已脱敏</small> : null}</span></span></td>
      <td><code>{destination.code}</code></td>
      <td><code>{destination.destination_type || '-'}</code></td>
      <td><StatusTag tone={destination.enabled ? 'success' : 'neutral'}>{destination.enabled ? '启用' : '停用'}</StatusTag></td>
      <td><div className={styles.actions}><button type="button" disabled={operationRunning} onClick={() => void onDetail(destination.id)}>{detailLoadingID === destination.id ? '读取中…' : '详情'}</button><button type="button" disabled={operationRunning} onClick={() => onTest(destination)}><Unplug aria-hidden="true" />{testingID === destination.id ? '测试中…' : '测试连接'}</button></div></td>
    </tr>)}</tbody>
  </DataTable>
}

function DestinationDrawer({ draft, error, saving, onChange, onClose, onSave }: { draft: DestinationDraft | null; error: string; saving: boolean; onChange: (draft: DestinationDraft) => void; onClose: () => void; onSave: () => Promise<void> }) {
  const set = <K extends keyof DestinationDraft>(key: K, value: DestinationDraft[K]) => { if (draft) onChange({ ...draft, [key]: value }) }
  return <Drawer open={Boolean(draft)} title={draft?.id ? '推送目标详情与编辑' : '新增推送目标'} description="连接配置保存在服务端；配置内容必须为 JSON 对象。" size="medium" closeDisabled={saving} onClose={onClose} footer={<><button type="button" disabled={saving} onClick={onClose}>取消</button><button className={styles.primary} type="button" disabled={saving} onClick={() => void onSave()}>{saving ? '保存中…' : '保存目标'}</button></>}>
    {draft ? <form className={styles.form} onSubmit={(event) => { event.preventDefault(); void onSave() }}>
      {draft.hasSecret ? <p className={styles.secretNotice} role="status">配置中的敏感值已隐藏。保留“[已隐藏]”会保留原值；改为新值即可轮换，且不会回显旧值。</p> : null}
      <label>目标名称<input required maxLength={100} disabled={saving} value={draft.name} onChange={(event) => set('name', event.currentTarget.value)} /></label>
      <label>目标编码<input required maxLength={100} className={styles.mono} disabled={saving} value={draft.code} onChange={(event) => set('code', event.currentTarget.value)} /></label>
      <label>目标类型<select disabled={saving} value={draft.destinationType} onChange={(event) => set('destinationType', event.currentTarget.value)}><option value="http">HTTP</option><option value="soap">SOAP</option></select></label>
      <label className={styles.checkbox}><input type="checkbox" checked={draft.enabled} disabled={saving} onChange={(event) => set('enabled', event.currentTarget.checked)} /><span>启用目标</span></label>
      <label>配置 JSON<textarea rows={12} className={styles.mono} disabled={saving} spellCheck={false} value={draft.configJSON} onChange={(event) => set('configJSON', event.currentTarget.value)} /></label>
      <p className={styles.contractNote}>连接测试仅允许无业务载荷的 HEAD 或 GET。请确保所配置的 GET 端点没有写入、触发任务等副作用。</p>
      {error ? <p className={styles.formError} role="alert">{error}</p> : null}
    </form> : null}
  </Drawer>
}
