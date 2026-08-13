import { FlaskConical, Plus } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { redactMonitoringJSON } from '../../monitoring'
import { buildTransformRuleListQuery } from '../../monitoringRecords'
import { DataTable, Drawer, FeedbackState, FilterToolbar, PageCanvas, PageHeader, PaginationControls, Section, StatusTag } from '../../ui'
import type { SourceDefinition, ConfigurationClient } from '../types'
import { buildRuleSavePayload, newRuleDraft, parseLegacyTransformRules, parseRuleTestContent, parseTransformRuleDetail, parseTransformRulePage, readRuleTestResult, ruleDraftFrom, type RuleDraft, type TransformRule } from '../ruleContracts'
import styles from './RulesPage.module.css'

export function RulesPage({ client, rules, sources, onRulesChange, refreshVersion }: { client: ConfigurationClient; rules: TransformRule[]; sources: SourceDefinition[]; onRulesChange: (rules: TransformRule[]) => void; refreshVersion: number }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [ruleType, setRuleType] = useState('all')
  const [applied, setApplied] = useState({ keyword: '', enabled: '' as '' | 'true' | 'false', ruleType: '' })
  const [page, setPage] = useState(1)
  const [reloadVersion, setReloadVersion] = useState(0)
  const [recordsPage, setRecordsPage] = useState<ReturnType<typeof parseTransformRulePage>>(null)
  const [loading, setLoading] = useState(true)
  const [listError, setListError] = useState('')
  const [notice, setNotice] = useState('')
  const [draft, setDraft] = useState<RuleDraft | null>(null)
  const [drawerError, setDrawerError] = useState('')
  const [rawContent, setRawContent] = useState('{}')
  const [testResult, setTestResult] = useState<unknown>(null)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [detailLoadingID, setDetailLoadingID] = useState<number | null>(null)
  const requestRef = useRef<AbortController | null>(null)
  const listQuery = useMemo(() => buildTransformRuleListQuery({ page, pageSize: 20, ...applied }), [applied, page])

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    async function load() {
      setLoading(true); setListError('')
      try {
        const response = await client(`/v1/transform-rules?${listQuery}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
        if (controller.signal.aborted) return
        const next = response.ok ? parseTransformRulePage(response.data) : null
        if (next) { setRecordsPage(next); return }
        const legacy = response.ok ? parseLegacyTransformRules(response.data) : null
        if (legacy) { const pageSize = 20; setRecordsPage({ list: legacy.slice(0, pageSize), pagination: { page: 1, pageSize, total: legacy.length, totalPages: legacy.length ? 1 : 0 } }); setListError('当前服务暂不支持规则分页或筛选，已显示未筛选的兼容数据。'); return }
        setListError(response.error?.message || '清洗规则列表暂时不可用，请稍后重试。')
      } catch {
        if (!controller.signal.aborted) setListError('清洗规则列表暂时不可用，请稍后重试。')
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    }
    void load()
    return () => controller.abort()
  }, [client, listQuery, refreshVersion, reloadVersion])

  function apply(nextQuery: string, nextStatus: string, nextType: string) {
    setPage(1); setApplied({ keyword: nextQuery, enabled: nextStatus === 'enabled' ? 'true' : nextStatus === 'disabled' ? 'false' : '', ruleType: nextType === 'all' ? '' : nextType }); setReloadVersion((v) => v + 1)
  }
  function openCreate() { if (detailLoadingID !== null) return; setNotice(''); setDrawerError(''); setTestResult(null); setDraft(newRuleDraft(sources[0]?.id)) }
  async function openDetail(id: number) {
    if (detailLoadingID !== null) return
    setNotice(''); setDetailLoadingID(id)
    try {
      const response = await client(`/v1/transform-rules/${id}`, { method: 'GET', showResult: false, silentLoading: true })
      const rule = response.ok ? parseTransformRuleDetail(response.data) : null
      if (!rule) { setNotice(response.error?.message || '规则详情暂时不可用，请稍后重试。'); return }
      setDrawerError(''); setTestResult(null); setDraft(ruleDraftFrom(rule))
    } catch { setNotice('规则详情暂时不可用，请稍后重试。') } finally { setDetailLoadingID(null) }
  }
  async function save() {
    if (!draft || saving || testing) return
    const validation = buildRuleSavePayload(draft)
    if (!validation.ok) { setDrawerError(validation.error); return }
    setSaving(true); setDrawerError('')
    try {
      const response = await client(draft.id ? `/v1/transform-rules/${draft.id}` : '/v1/transform-rules', { method: draft.id ? 'PUT' : 'POST', showResult: false, silentLoading: true, body: validation.payload })
      const saved = response.ok ? parseTransformRuleDetail(response.data) : null
      if (!saved || (draft.id !== null && saved.id !== draft.id)) { setDrawerError(response.error?.message || '规则保存未完成，请稍后重试。'); return }
      onRulesChange(draft.id ? rules.map((rule) => rule.id === saved.id ? saved : rule) : [...rules, saved].sort((a, b) => a.source_id - b.source_id || a.order_index - b.order_index || b.id - a.id))
      setDraft(null); setNotice('清洗规则已保存。'); setReloadVersion((v) => v + 1)
    } catch { setDrawerError('规则保存未完成，请稍后重试。') } finally { setSaving(false) }
  }
  async function runTest(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft || draft.ruleType !== 'mapping' || testing || saving) return
    const parsed = parseRuleTestContent(rawContent, draft.configJSON)
    if (!parsed.ok) { setDrawerError(parsed.error); return }
    setTesting(true); setDrawerError(''); setTestResult(null)
    try {
      const response = await client('/v1/transform-rules/test', { method: 'POST', body: { raw_content: parsed.rawContent, config_json: draft.configJSON }, showResult: false, silentLoading: true })
      const result = response.ok ? readRuleTestResult(response.data) : null
      if (result === null) { setDrawerError(response.error?.message || '规则测试未完成，请稍后重试。'); return }
      setTestResult(redactMonitoringJSON(result))
    } catch { setDrawerError('规则测试未完成，请稍后重试。') } finally { setTesting(false) }
  }
  const pagination = recordsPage?.pagination
  const listed = recordsPage?.list ?? []
  const busy = saving || testing
  return <PageCanvas>
    <PageHeader eyebrow="DATA CONFIGURATION" title="清洗规则" description="维护来源规则与执行顺序，并在保存前使用当前 Mapping 草稿做脱敏测试。" actions={<button className={styles.primary} type="button" disabled={detailLoadingID !== null} onClick={openCreate}><Plus aria-hidden="true" />{detailLoadingID ? '读取详情中…' : '新增规则'}</button>} />
    {notice ? <p className={styles.notice} role="status">{notice}</p> : null}
    <FilterToolbar summary={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}>
      <form className={styles.filters} onSubmit={(event) => { event.preventDefault(); apply(query, status, ruleType) }}>
        <label>名称 / 来源 ID / 配置<input type="search" value={query} onChange={(event) => setQuery(event.currentTarget.value)} /></label>
        <label>状态<select value={status} onChange={(event) => { const value = event.currentTarget.value; setStatus(value); apply(query, value, ruleType) }}><option value="all">全部</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label>
        <label>规则类型<select value={ruleType} onChange={(event) => { const value = event.currentTarget.value; setRuleType(value); apply(query, status, value) }}><option value="all">全部</option>{['mapping','validator','http_enrich','db_enrich','script'].map((value) => <option key={value}>{value}</option>)}</select></label>
        <button className={styles.primary} type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
    </FilterToolbar>
    {listError ? <FeedbackState kind="error" title="清洗规则查询提示" description={`${listError}${recordsPage && !listError.includes('兼容数据') ? ' 已保留最近一次成功数据。' : ''}`} /> : null}
    <Section title="规则列表" description="每页 20 条；规则详情始终从服务端重新读取。" flush>
      {loading && !recordsPage ? <FeedbackState kind="loading" title="正在加载清洗规则" /> : !listed.length ? <FeedbackState kind="empty" title="暂无清洗规则" /> : <RuleTable rules={listed} sources={sources} busy={detailLoadingID !== null} detailLoadingID={detailLoadingID} onDetail={openDetail} />}
      <PaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading || detailLoadingID !== null} onPrevious={() => setPage((v) => Math.max(1, v - 1))} onNext={() => setPage((v) => v + 1)} />
    </Section>
    <Drawer open={Boolean(draft)} title={draft?.id ? '清洗规则详情与编辑' : '新增清洗规则'} description="保存配置后才会影响正式清洗；测试只执行当前草稿。" size="medium" closeDisabled={busy} onClose={() => { if (!busy) { setDraft(null); setDrawerError('') } }} footer={<><button type="button" disabled={busy} onClick={() => setDraft(null)}>取消</button><button className={styles.primary} type="button" disabled={busy} onClick={() => void save()}>{saving ? '保存中…' : '保存规则'}</button></>}>
      {draft ? <div className={styles.drawerContent}>{draft.hasSecret ? <p className={styles.secretNotice}>配置中的敏感值已隐藏。保留“[已隐藏]”会保留原值；改为新值即可轮换。</p> : null}<form className={styles.form} onSubmit={(event) => { event.preventDefault(); void save() }}><label>来源<select required disabled={busy} value={draft.sourceID} onChange={(e) => setDraft({ ...draft, sourceID: e.currentTarget.value })}><option value="">选择数据源</option>{sources.map((s) => <option key={s.id} value={s.id}>#{s.id} {s.name}</option>)}</select></label><label>规则名称<input required maxLength={100} disabled={busy} value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.currentTarget.value })} /></label><label>规则类型<select disabled={busy} value={draft.ruleType} onChange={(e) => setDraft({ ...draft, ruleType: e.currentTarget.value })}>{['mapping','http_enrich','db_enrich','script','validator'].map((v) => <option key={v}>{v}</option>)}</select></label><label>执行顺序<input type="number" required disabled={busy} value={draft.orderIndex} onChange={(e) => setDraft({ ...draft, orderIndex: e.currentTarget.value })} /></label><label className={styles.checkbox}><input type="checkbox" disabled={busy} checked={draft.enabled} onChange={(e) => setDraft({ ...draft, enabled: e.currentTarget.checked })} />启用规则</label><label>规则配置 JSON<textarea className={styles.mono} rows={12} disabled={busy} value={draft.configJSON} onChange={(e) => setDraft({ ...draft, configJSON: e.currentTarget.value })} /></label></form><section className={styles.testSection}><div><h3>Mapping 规则测试</h3><p>仅 Mapping 类型可执行；返回内容会在展示前脱敏。</p></div><form className={styles.testForm} onSubmit={runTest}><label>测试原始内容 JSON<textarea className={styles.mono} rows={7} disabled={busy || draft.ruleType !== 'mapping'} value={rawContent} onChange={(e) => setRawContent(e.currentTarget.value)} /></label><button type="submit" disabled={busy || draft.ruleType !== 'mapping'}><FlaskConical aria-hidden="true" />{testing ? '测试中…' : '执行测试'}</button></form>{testResult !== null ? <pre className={styles.result}>{JSON.stringify(testResult, null, 2)}</pre> : null}</section>{drawerError ? <p className={styles.error} role="alert">{drawerError}</p> : null}</div> : null}
    </Drawer>
  </PageCanvas>
}

function RuleTable({ rules, sources, busy, detailLoadingID, onDetail }: { rules: TransformRule[]; sources: SourceDefinition[]; busy: boolean; detailLoadingID: number | null; onDetail: (id: number) => Promise<void> }) {
  return <DataTable containerClassName={styles.table} minWidth={860} scrollLabel="清洗规则列表"><thead><tr><th scope="col">规则名称</th><th scope="col">规则类型</th><th scope="col">来源</th><th scope="col">执行顺序</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead><tbody>{rules.map((rule) => { const source = sources.find((s) => s.id === rule.source_id); return <tr key={rule.id}><td><span className={styles.identity}><strong>{rule.name}</strong><code>#{rule.id}</code>{rule.has_secret ? <small>配置已脱敏</small> : null}</span></td><td><code>{rule.rule_type}</code></td><td>{source?.name ?? `#${rule.source_id}`}</td><td>{rule.order_index}</td><td><StatusTag tone={rule.enabled ? 'success' : 'neutral'}>{rule.enabled ? '启用' : '停用'}</StatusTag></td><td><button type="button" disabled={busy} onClick={() => void onDetail(rule.id)}>{detailLoadingID === rule.id ? '读取中…' : '详情'}</button></td></tr> })}</tbody></DataTable>
}
