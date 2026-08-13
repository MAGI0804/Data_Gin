import { Braces, Settings2, Wrench } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { PipelineComposerPanel } from '../../PipelineComposerPanel'
import { parsePipelineDetail, type PipelineSummary } from '../../pipelineComposer'
import { DataTable, Dialog, Drawer, FeedbackState, FilterToolbar, MetricStrip, PageCanvas, PageHeader, Section, StatusTag } from '../../ui'
import { parseDeliveryTaskDetail, parseLegacyDeliveryTasks } from '../deliveryTaskContracts'
import { parseDestinationDetail, parseLegacyDestinationList } from '../destinationContracts'
import { parseLegacyTransformRules as parseConfiguredRules, parseTransformRuleDetail } from '../ruleContracts'
import { parseLegacySourceList, parseSourceDetail } from '../sourceContracts'
import type { ConfigurationClient, DeliveryTask, DestinationDefinition, SourceDefinition } from '../types'
import type { TransformRule } from '../ruleContracts'
import {
  buildConfiguredMethodDisplays,
  buildCoreMethods,
  buildLegacyMethodDisplays,
  builtinMethods,
  canToggleTarget,
  parseLegacyTasks,
  parseLegacyTransformRules,
  parsePipelineSummaries,
  pipelineStepMethodDisplay,
  updateTargetEnabled,
  type LegacyTask,
  type LegacyTransformRule,
  type MethodDisplay,
  type ToggleTarget,
} from './methodCatalog'
import styles from './MethodsPage.module.css'

type CatalogData = {
  pipelines: PipelineSummary[]
  sources: SourceDefinition[]
  rules: TransformRule[]
  destinations: DestinationDefinition[]
  tasks: DeliveryTask[]
  legacyTasks: LegacyTask[]
  legacyRules: LegacyTransformRule[]
  pipelineMethods: MethodDisplay[]
}

const emptyData: CatalogData = { pipelines: [], sources: [], rules: [], destinations: [], tasks: [], legacyTasks: [], legacyRules: [], pipelineMethods: [] }

export function MethodsPage({ client, permissions, refreshVersion }: { client: ConfigurationClient; permissions: readonly string[]; refreshVersion: number }) {
  const [data, setData] = useState<CatalogData>(emptyData)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [reloadVersion, setReloadVersion] = useState(0)
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('all')
  const [status, setStatus] = useState('all')
  const [applied, setApplied] = useState({ query: '', category: 'all', status: 'all' })
  const [composerOpen, setComposerOpen] = useState(false)
  const [composerBusy, setComposerBusy] = useState(false)
  const [pendingToggle, setPendingToggle] = useState<{ method: MethodDisplay; enabled: boolean } | null>(null)
  const [toggling, setToggling] = useState(false)
  const toggleInFlightRef = useRef(false)
  const canManagePipeline = permissions.includes('pipeline.manage')

  useEffect(() => {
    const controller = new AbortController()
    async function load() {
      setLoading(true); setError('')
      const requests: Array<Promise<{ label: string; ok: boolean; value: unknown }>> = [
        request(client, '/v1/pipelines', controller.signal, parsePipelineSummaries, '流水线'),
        request(client, '/v1/transform-rules', controller.signal, parseConfiguredRules, '清洗规则'),
        request(client, '/v1/legacy-transform-rules', controller.signal, parseLegacyTransformRules, '旧清洗规则'),
      ]
      if (permissions.includes('source.read') || permissions.includes('source.manage')) requests.push(request(client, '/v1/sources', controller.signal, parseLegacySourceList, '数据源'))
      if (permissions.includes('delivery.read') || permissions.includes('delivery.manage')) {
        requests.push(request(client, '/v1/destinations', controller.signal, parseLegacyDestinationList, '推送目标'))
        requests.push(request(client, '/v1/delivery-tasks', controller.signal, parseLegacyDeliveryTasks, '推送任务'))
      }
      if (permissions.includes('pipeline.execute')) requests.push(request(client, '/v1/legacy-tasks', controller.signal, parseLegacyTasks, '旧任务'))

      try {
        const results = await Promise.all(requests)
        if (controller.signal.aborted) return
        const failures = results.filter((item) => !item.ok).map((item) => item.label)
        const pick = <T,>(label: string, fallback: T): T => results.find((item) => item.label === label && item.ok)?.value as T ?? fallback
        const pipelineResult = results.find((item) => item.label === '流水线')
        const pipelines = pipelineResult?.ok ? pipelineResult.value as PipelineSummary[] : []
        const detailResults = await Promise.all(pipelines.map(async (pipeline) => {
          try {
            const response = await client(`/v1/pipelines/${pipeline.id}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
            const detail = response.ok ? parsePipelineDetail(response.data) : null
            return detail?.pipeline.id === pipeline.id ? detail.steps.map((step) => pipelineStepMethodDisplay(step, pipeline.name || pipeline.code)) : null
          } catch { return null }
        }))
        if (controller.signal.aborted) return
        if (detailResults.some((item) => item === null)) failures.push('流水线步骤')
        setData((current) => ({
          pipelines: pick('流水线', current.pipelines),
          sources: pick('数据源', current.sources),
          rules: pick('清洗规则', current.rules),
          destinations: pick('推送目标', current.destinations),
          tasks: pick('推送任务', current.tasks),
          legacyTasks: pick('旧任务', current.legacyTasks),
          legacyRules: pick('旧清洗规则', current.legacyRules),
          pipelineMethods: pipelineResult?.ok ? detailResults.flatMap((item) => item ?? []) : current.pipelineMethods,
        }))
        setError(failures.length ? `${Array.from(new Set(failures)).join('、')}加载失败；其余目录仍可使用。` : '')
      } catch {
        if (!controller.signal.aborted) setError('方法目录暂时不可用，请稍后重试。')
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    }
    void load()
    return () => controller.abort()
  }, [client, permissions, refreshVersion, reloadVersion])

  const methods = useMemo(() => [
    ...buildConfiguredMethodDisplays(data.sources, data.rules, data.destinations, data.tasks),
    ...buildLegacyMethodDisplays(data.legacyTasks, data.legacyRules),
    ...data.pipelineMethods,
    ...builtinMethods,
  ], [data])
  const coreMethods = useMemo(() => buildCoreMethods({ sources: data.sources, transformRules: data.rules, destinations: data.destinations, deliveryTasks: data.tasks, legacyTasks: data.legacyTasks, legacyRules: data.legacyRules }), [data])
  const categories = useMemo(() => Array.from(new Set(methods.map((method) => method.category))).sort((left, right) => left.localeCompare(right, 'zh-CN')), [methods])
  const filtered = methods.filter((method) => includesQuery([method.name, method.code, method.description, method.owner], applied.query)
    && (applied.category === 'all' || method.category === applied.category)
    && (applied.status === 'all' || method.enabled === (applied.status === 'enabled')))

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setApplied({ query: query.trim(), category, status })
  }

  function resetFilters() {
    setQuery(''); setCategory('all'); setStatus('all'); setApplied({ query: '', category: 'all', status: 'all' })
  }

  async function confirmToggle() {
    const pending = pendingToggle
    if (!pending?.method.toggle || toggleInFlightRef.current || !canToggleTarget(pending.method.toggle, permissions)) return
    toggleInFlightRef.current = true; setToggling(true); setNotice('')
    try {
      const response = await updateTargetEnabled(client, pending.method.toggle, pending.enabled)
      if (!response.ok || !validToggleResponse(pending.method.toggle, pending.enabled, response.data)) { setNotice(response.error?.message || '状态更新响应不完整，请刷新确认状态。'); return }
      setNotice(`“${pending.method.name}”已${pending.enabled ? '启用' : '停用'}。`)
      setReloadVersion((value) => value + 1)
    } catch { setNotice('状态更新未完成，请刷新后重试。') } finally {
      toggleInFlightRef.current = false; setToggling(false); setPendingToggle(null)
    }
  }

  return <PageCanvas>
    <PageHeader eyebrow="METHOD REGISTRY" title="方法目录" description="统一查看配置方法、流水线步骤和系统内置能力；状态变更按资源权限独立控制。" actions={<button className={styles.primary} type="button" onClick={() => setComposerOpen(true)}><Settings2 aria-hidden="true" />流水线编排</button>} />
    {notice ? <p className={styles.notice} role="status" aria-live="polite">{notice}</p> : null}
    <MetricStrip items={[
      { key: 'configured', label: '已配置方法', value: methods.filter((item) => item.kind === 'configured').length },
      { key: 'builtin', label: '内置方法', value: methods.filter((item) => item.kind === 'builtin').length },
      { key: 'enabled', label: '启用方法', value: methods.filter((item) => item.enabled).length },
      { key: 'types', label: '方法类型', value: new Set(methods.map((item) => item.methodType)).size },
    ]} />
    <Section title="核心能力" description="聚合状态用于快速核对链路；具体启停请在下方目录中操作。"><div className={styles.coreList}>{coreMethods.filter((method) => ['youzan_fetch', 'qimai_process', 'mall_push'].includes(method.key)).map((method) => <article className={styles.coreRow} key={method.key}><span className={styles.coreIcon}><Wrench aria-hidden="true" /></span><div><strong>{method.title}</strong><span>{method.category}</span></div><p>{method.status}</p><StatusTag tone={method.enabled ? 'success' : 'neutral'}>{method.enabled ? '已开启' : '已关闭'}</StatusTag></article>)}</div></Section>
    <FilterToolbar summary={loading && data === emptyData ? '正在加载…' : `${filtered.length} / ${methods.length} 条`}><form className={styles.filters} onSubmit={applyFilters}><label>名称 / 编码 / 负责人<input type="search" value={query} onChange={(event) => setQuery(event.currentTarget.value)} placeholder="搜索方法" /></label><label>分类<select value={category} onChange={(event) => setCategory(event.currentTarget.value)}><option value="all">全部</option>{categories.map((item) => <option key={item} value={item}>{item}</option>)}</select></label><label>状态<select value={status} onChange={(event) => setStatus(event.currentTarget.value)}><option value="all">全部</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label><div className={styles.filterActions}><button type="button" onClick={resetFilters}>重置</button><button className={styles.primary} type="submit">查询</button></div></form></FilterToolbar>
    {error ? <FeedbackState kind="error" title="方法目录查询提示" description={error} /> : null}
    <Section title="全部方法" description="内置方法只读；配置方法仅在具备对应管理权限时可变更状态。" flush>{loading && methods.length === builtinMethods.length ? <FeedbackState kind="loading" title="正在加载方法目录" /> : filtered.length === 0 ? <FeedbackState kind="empty" title="暂无匹配的方法" /> : <MethodTable methods={filtered} permissions={permissions} busy={toggling} onToggle={(method, enabled) => setPendingToggle({ method, enabled })} />}</Section>
    <Drawer open={composerOpen} size="wide" title="流水线方法高级配置" description={canManagePipeline ? '创建、编辑、预览、生成与发布均连接真实接口。' : '当前为只读模式，可查看阶段、步骤和 JSON 预览。'} closeDisabled={composerBusy} closeOnBackdrop={!composerBusy} onClose={() => { if (!composerBusy) setComposerOpen(false) }}><PipelineComposerPanel pipelines={data.pipelines} client={client} canManage={canManagePipeline} onBusyChange={setComposerBusy} onRefresh={() => setReloadVersion((value) => value + 1)} /></Drawer>
    <Dialog open={Boolean(pendingToggle)} role="alertdialog" title={`确认${pendingToggle?.enabled ? '启用' : '停用'}方法`} description="此操作会更新真实配置资源。" closeDisabled={toggling} closeOnBackdrop={!toggling} onClose={() => { if (!toggling) setPendingToggle(null) }} footer={<><button type="button" disabled={toggling} onClick={() => setPendingToggle(null)}>取消</button><button className={pendingToggle?.enabled ? styles.primary : styles.danger} type="button" disabled={toggling} onClick={() => void confirmToggle()}>{toggling ? '更新中…' : '确认更新'}</button></>}><p className={styles.dialogCopy}><Braces aria-hidden="true" />将把“{pendingToggle?.method.name}”设置为{pendingToggle?.enabled ? '启用' : '停用'}，其他关联资源不会自动变更。</p></Dialog>
  </PageCanvas>
}

function MethodTable({ methods, permissions, busy, onToggle }: { methods: MethodDisplay[]; permissions: readonly string[]; busy: boolean; onToggle: (method: MethodDisplay, enabled: boolean) => void }) {
  return <DataTable containerClassName={styles.table} minWidth={1020} scrollLabel="方法目录列表"><thead><tr><th scope="col">方法名称</th><th scope="col">编码</th><th scope="col">分类</th><th scope="col">来源</th><th scope="col">类型</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead><tbody>{methods.map((method) => { const writable = Boolean(method.toggle && canToggleTarget(method.toggle, permissions)); return <tr key={method.key}><td><span className={styles.identity}><Wrench aria-hidden="true" /><span><strong>{method.name}</strong><small>{method.description}</small></span></span></td><td><code>{method.code}</code></td><td>{method.category}</td><td>{method.owner}</td><td>{method.kind === 'builtin' ? '内置' : method.methodType}</td><td><StatusTag tone={method.enabled ? 'success' : 'neutral'}>{method.enabled ? '启用' : '停用'}</StatusTag></td><td>{writable ? <button type="button" disabled={busy} onClick={() => onToggle(method, !method.enabled)}>{method.enabled ? '停用' : '启用'}</button> : <span className={styles.readOnly}>只读</span>}</td></tr> })}</tbody></DataTable>
}

async function request<T>(client: ConfigurationClient, path: string, signal: AbortSignal, parser: (payload: unknown) => T | null, label: string): Promise<{ label: string; ok: boolean; value: T | null }> {
  try {
    const response = await client(path, { method: 'GET', signal, showResult: false, silentLoading: true })
    const value = response.ok ? parser(response.data) : null
    return { label, ok: value !== null, value }
  } catch { return { label, ok: false, value: null } }
}

function includesQuery(values: Array<string | number>, query: string): boolean {
  const normalized = query.trim().toLocaleLowerCase('zh-CN')
  return !normalized || values.some((value) => String(value ?? '').toLocaleLowerCase('zh-CN').includes(normalized))
}

function validToggleResponse(target: ToggleTarget, enabled: boolean, payload: unknown): boolean {
  if (target.type === 'source') { const value = parseSourceDetail(payload); return value?.id === target.id && value.enabled === enabled }
  if (target.type === 'transform_rule') { const value = parseTransformRuleDetail(payload); return value?.id === target.id && value.enabled === enabled }
  if (target.type === 'destination') { const value = parseDestinationDetail(payload); return value?.id === target.id && value.enabled === enabled }
  const value = parseDeliveryTaskDetail(payload)
  return value?.id === target.id && value.enabled === enabled
}
