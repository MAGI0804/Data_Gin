import { FormEvent, ReactNode, useCallback, useEffect, useMemo, useState } from 'react'
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  BookOpen,
  CheckCircle2,
  Database,
  FileJson,
  Inbox,
  ListChecks,
  LogOut,
  RefreshCcw,
  ScrollText,
  Send,
  Wrench,
} from 'lucide-react'
import './App.css'

type ApiResult = {
  ok: boolean
  status: number
  data: unknown
}

type ApiClientOptions = {
  method?: 'GET' | 'POST' | 'PUT'
  body?: unknown
  showResult?: boolean
  silentLoading?: boolean
}

type ApiClient = (path: string, options?: ApiClientOptions) => Promise<ApiResult>
type NavKey = 'push_status' | 'methods' | 'receive' | 'pull' | 'process' | 'push' | 'logs'
type MethodKind = 'configured' | 'builtin'
type MethodType = 'request' | 'bojun_signed_request' | 'extract' | 'mapping' | 'validate' | 'db_query' | 'db_write' | 'template' | 'delivery' | 'log' | 'utility'
type JsonRecord = Record<string, unknown>

type PipelineDefinition = {
  id: number
  name: string
  code: string
  enabled: boolean
}

type MethodStep = {
  id: number
  pipeline_id: number
  stage_id: number
  code: string
  name: string
  method_type: MethodType
  order_index: number
  enabled: boolean
  timeout_seconds: number
  generated_config_json: string
}

type MethodParam = {
  location: string
  name: string
  value_source: string
  value: string
  value_type: string
  required: boolean
  secret: boolean
  description: string
}

type MethodOutput = {
  name: string
  source_path: string
  value_type: string
  required: boolean
  description: string
}

type MethodStepDetail = {
  step: MethodStep
  params: MethodParam[]
  outputs: MethodOutput[]
}

type PipelineDetail = {
  pipeline: PipelineDefinition
  steps: MethodStepDetail[]
}

type MethodDisplay = {
  key: string
  kind: MethodKind
  name: string
  code: string
  method_type: MethodType
  category: string
  owner: string
  description: string
  enabled: boolean
  params?: MethodParam[]
  outputs?: MethodOutput[]
  toggle?: ToggleTarget
}

type ToggleTarget =
  | { type: 'source'; id: number }
  | { type: 'transform_rule'; id: number }
  | { type: 'destination'; id: number }
  | { type: 'delivery_task'; id: number }

type CoreMethod = {
  key: string
  title: string
  category: string
  description: string
  enabled: boolean
  status: string
  refs: ToggleTarget[]
}

type PipelineRun = {
  id: number
  trace_id: string
  run_type: string
  trigger_type: string
  status: string
  total_count: number
  success_count: number
  failed_count: number
  source_id: number
  destination_id: number
  error_message: string
  started_at: string | null
  finished_at: string | null
}

type StepRun = {
  id: number
  run_id: number
  step_code: string
  method_type: string
  status: string
  input_json: string
  output_json: string
  error_message: string
}

type SourceDefinition = {
  id: number
  name: string
  code: string
  source_type: string
  enabled: boolean
  auth_type: string
  config_json: string
  schema_json: string
  dedupe_keys: string
  source_query_key: string
}

type RawData = {
  id: number
  data_source_id: number
  external_id: string
  data_type: string
  raw_content: unknown
  rawContent?: unknown
  metadata: unknown
  status: string
  remark: string
  source: string
  created_at: number
  updated_at: number
}

type ProcessedData = {
  id: number
  raw_data_id: number
  data_type: string
  data_fields: string
  quality_score: number
  created_at: number
}

type TransformRule = {
  id: number
  source_id: number
  name: string
  rule_type: string
  order_index: number
  config_json: string
  enabled: boolean
}

type DestinationDefinition = {
  id: number
  name: string
  code: string
  destination_type: string
  config_json: string
  enabled: boolean
}

type DeliveryTask = {
  id: number
  name: string
  source_id: number
  clean_table: string
  destination_id: number
  trigger_type: string
  cron_expr: string
  filter_json: string
  payload_template: string
  enabled: boolean
}

type LegacyTask = {
  code: string
  name: string
  category: string
  source_code: string
  source_name: string
  cron_expr: string
  input_table: string
  output_table: string
  target_system: string
  description: string
}

type LegacyTransformRule = {
  code: string
  name: string
  source_code: string
  source_name: string
  rule_type: string
  trigger_mode: string
  input_table: string
  output_table: string
  description: string
}

type DeliveryLog = {
  id: number
  trace_id: string
  destination_id: number
  clean_record_id: number
  status: string
  request_json: string
  response_json: string
  error_message: string
  delivered_at: string | null
}

const tokenStorageKey = 'warehouse-token'

const navItems: Array<{ key: NavKey; label: string; icon: ReactNode }> = [
  { key: 'push_status', label: '推送情况', icon: <Activity aria-hidden="true" /> },
  { key: 'methods', label: '方法', icon: <Wrench aria-hidden="true" /> },
  { key: 'receive', label: '数据接收', icon: <Inbox aria-hidden="true" /> },
  { key: 'pull', label: '数据拉取', icon: <ArrowDownToLine aria-hidden="true" /> },
  { key: 'process', label: '数据处理', icon: <ListChecks aria-hidden="true" /> },
  { key: 'push', label: '数据推送', icon: <ArrowUpFromLine aria-hidden="true" /> },
  { key: 'logs', label: '日志', icon: <ScrollText aria-hidden="true" /> },
]

const builtinMethods: MethodDisplay[] = [
  {
    key: 'builtin-md32',
    kind: 'builtin',
    name: 'MD32 验证',
    code: 'md32_verify',
    method_type: 'utility',
    category: '前置验证方法',
    owner: '系统内置',
    description: '对传入字段生成或校验 32 位 MD5 摘要。',
    enabled: true,
  },
  {
    key: 'builtin-uniqstr',
    kind: 'builtin',
    name: '生成唯一 uniqstr',
    code: 'generate_uniqstr',
    method_type: 'utility',
    category: '内置工具方法',
    owner: '系统内置',
    description: '生成可用于幂等键、批次号或外部请求追踪的唯一字符串。',
    enabled: true,
  },
  {
    key: 'builtin-http-push',
    kind: 'builtin',
    name: 'HTTP 推送方法',
    code: 'http_delivery',
    method_type: 'delivery',
    category: '推送方法',
    owner: '系统内置',
    description: '根据推送目标配置发送 HTTP 请求并记录响应。',
    enabled: true,
  },
  {
    key: 'builtin-api-pull',
    kind: 'builtin',
    name: 'API 拉取方法',
    code: 'api_poll',
    method_type: 'request',
    category: '数据拉取方法',
    owner: '系统内置',
    description: '按数据源配置请求第三方 API 并落原始数据。',
    enabled: true,
  },
  {
    key: 'builtin-bojun-signed-request',
    kind: 'builtin',
    name: '伯俊签名请求',
    code: 'bojun_signed_request',
    method_type: 'bojun_signed_request',
    category: '数据拉取方法',
    owner: '系统内置',
    description: 'Go 系统方法 pkg/bojun.SendSignedRequest，入参 method 和 body，凭据从 BOJUN_* 环境变量读取，出参为接口 JSON。',
    enabled: true,
  },
]

function App() {
  const [authenticated, setAuthenticated] = useState(() => Boolean(sessionStorage.getItem(tokenStorageKey)))
  const [token, setToken] = useState(() => sessionStorage.getItem(tokenStorageKey) ?? '')
  const [activeNav, setActiveNav] = useState<NavKey>('push_status')
  const [loading, setLoading] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [result, setResult] = useState<ApiResult | null>(null)
  const [runs, setRuns] = useState<PipelineRun[]>([])
  const [stepRuns, setStepRuns] = useState<StepRun[]>([])
  const [methods, setMethods] = useState<MethodDisplay[]>(builtinMethods)
  const [sources, setSources] = useState<SourceDefinition[]>([])
  const [rawData, setRawData] = useState<RawData[]>([])
  const [processedData, setProcessedData] = useState<ProcessedData[]>([])
  const [transformRules, setTransformRules] = useState<TransformRule[]>([])
  const [destinations, setDestinations] = useState<DestinationDefinition[]>([])
  const [deliveryTasks, setDeliveryTasks] = useState<DeliveryTask[]>([])
  const [deliveryLogs, setDeliveryLogs] = useState<DeliveryLog[]>([])
  const [legacyTasks, setLegacyTasks] = useState<LegacyTask[]>([])
  const [legacyRules, setLegacyRules] = useState<LegacyTransformRule[]>([])

  const client = useCallback<ApiClient>(
    async (path, options = {}) => {
      const method = options.method ?? 'POST'
      if (!options.silentLoading) setLoading(true)
      try {
        const response = await fetch(`/api${path}`, {
          method,
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { token } : {}),
          },
          body: method === 'GET' || options.body === undefined ? undefined : JSON.stringify(options.body),
        })
        const data = await response.json().catch(() => ({}))
        const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
        if (options.showResult !== false) setResult(nextResult)
        return nextResult
      } catch (error) {
        const nextResult = { ok: false, status: 0, data: error instanceof Error ? error.message : String(error) }
        if (options.showResult !== false) setResult(nextResult)
        return nextResult
      } finally {
        if (!options.silentLoading) setLoading(false)
      }
    },
    [token],
  )

  const loadConfiguredMethods = useCallback(async (pipelines: PipelineDefinition[]) => {
    const details = await Promise.all(
      pipelines.map((pipeline) => client(`/v1/pipelines/${pipeline.id}`, { method: 'GET', showResult: false, silentLoading: true })),
    )
    return details.flatMap((detailResult, index) => {
      const detail = readObject<PipelineDetail>(detailResult, 'pipeline')
      if (!detail) return []
      return detail.steps.map((item) => ({
        key: `configured-${item.step.id}`,
        kind: 'configured' as const,
        name: item.step.name,
        code: item.step.code,
        method_type: item.step.method_type,
        category: methodCategory(item.step.method_type),
        owner: detail.pipeline?.name || pipelines[index]?.name || '未命名流水线',
        description: methodDescription(item),
        enabled: item.step.enabled,
        params: item.params,
        outputs: item.outputs,
      }))
    })
  }, [client])

  const refreshAll = useCallback(
    async (showResult = false) => {
      if (!token) return
      setRefreshing(true)
      try {
        const [pipelineResult, runResult, sourceResult, rawResult, processedResult, ruleResult, destinationResult, taskResult, logResult, legacyTaskResult, legacyRuleResult] = await Promise.all([
          client('/v1/pipelines', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/runs?limit=50', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/sources', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/data/raw?limit=50', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/data/processed?limit=50', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/transform-rules', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/destinations', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/delivery-tasks', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/delivery-logs?limit=50', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/legacy-tasks', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/legacy-transform-rules', { method: 'GET', showResult: false, silentLoading: true }),
        ])

        if (runResult.ok) setRuns(readList<PipelineRun>(runResult, 'runs'))
        const nextSources = sourceResult.ok ? readList<SourceDefinition>(sourceResult, 'sources') : []
        const nextRules = ruleResult.ok ? readList<TransformRule>(ruleResult, 'rules') : []
        const nextDestinations = destinationResult.ok ? readList<DestinationDefinition>(destinationResult, 'destinations') : []
        const nextTasks = taskResult.ok ? readList<DeliveryTask>(taskResult, 'tasks') : []
        const nextLegacyTasks = legacyTaskResult.ok ? readList<LegacyTask>(legacyTaskResult, 'tasks') : []
        const nextLegacyRules = legacyRuleResult.ok ? readList<LegacyTransformRule>(legacyRuleResult, 'rules') : []
        if (sourceResult.ok) setSources(nextSources)
        if (rawResult.ok) setRawData(readList<RawData>(rawResult, 'data'))
        if (processedResult.ok) setProcessedData(readList<ProcessedData>(processedResult, 'data'))
        if (ruleResult.ok) setTransformRules(nextRules)
        if (destinationResult.ok) setDestinations(nextDestinations)
        if (taskResult.ok) setDeliveryTasks(nextTasks)
        if (logResult.ok) setDeliveryLogs(readList<DeliveryLog>(logResult, 'logs'))
        if (legacyTaskResult.ok) setLegacyTasks(nextLegacyTasks)
        if (legacyRuleResult.ok) setLegacyRules(nextLegacyRules)
        if (pipelineResult.ok) {
          const pipelines = readList<PipelineDefinition>(pipelineResult, 'pipelines')
          const configuredMethods = await loadConfiguredMethods(pipelines)
          setMethods([...buildConfiguredMethodDisplays(nextSources, nextRules, nextDestinations, nextTasks), ...buildLegacyMethodDisplays(nextLegacyTasks, nextLegacyRules), ...configuredMethods, ...builtinMethods])
        }
        if (showResult) setResult({ ok: true, status: 200, data: { refreshed_at: new Date().toISOString() } })
      } finally {
        setRefreshing(false)
      }
    },
    [client, loadConfiguredMethods, token],
  )

  useEffect(() => {
    if (authenticated) void refreshAll(false)
  }, [authenticated, refreshAll])

  function handleLogin(nextToken: string) {
    sessionStorage.setItem(tokenStorageKey, nextToken)
    setToken(nextToken)
    setAuthenticated(true)
  }

  function handleLogout() {
    sessionStorage.removeItem(tokenStorageKey)
    setToken('')
    setAuthenticated(false)
    setResult(null)
  }

  async function loadStepRuns(runId: number) {
    const response = await client(`/v1/pipeline-runs/${runId}/steps`, { method: 'GET' })
    if (response.ok) setStepRuns(readList<StepRun>(response, 'step_runs'))
  }

  async function toggleTarget(target: ToggleTarget, enabled: boolean) {
    const response = await updateTargetEnabled(client, target, enabled, { sources, transformRules, destinations, deliveryTasks })
    if (response.ok) await refreshAll(false)
  }

  const receivedData = useMemo(() => rawData.filter((item) => !isPulledOrigin(rawDataOrigin(item))), [rawData])
  const pulledData = useMemo(() => rawData.filter((item) => isPulledOrigin(rawDataOrigin(item))), [rawData])
  const coreMethods = useMemo(
    () => buildCoreMethods({ sources, transformRules, destinations, deliveryTasks, legacyTasks, legacyRules }),
    [deliveryTasks, destinations, legacyRules, legacyTasks, sources, transformRules],
  )

  if (!authenticated) return <LoginScreen onLogin={handleLogin} />

  return (
    <main className="ops-shell">
      <aside className="ops-sidebar" aria-label="数据仓库导航">
        <div className="brand">
          <Database aria-hidden="true" />
          <div>
            <h1>数据仓库</h1>
            <span>运行与方法总览</span>
          </div>
        </div>
        <nav className="module-nav">
          {navItems.map((item) => (
            <button className={item.key === activeNav ? 'nav-item active' : 'nav-item'} key={item.key} type="button" onClick={() => setActiveNav(item.key)}>
              {item.icon}
              {item.label}
            </button>
          ))}
        </nav>
        <div className="sidebar-actions">
          <button type="button" onClick={() => refreshAll(true)} disabled={refreshing}>
            <RefreshCcw aria-hidden="true" />
            刷新
          </button>
          <button type="button" onClick={handleLogout}>
            <LogOut aria-hidden="true" />
            退出
          </button>
        </div>
      </aside>

      <section className="ops-workspace">
        <ModuleHeader activeNav={activeNav} loading={loading || refreshing} />
        {activeNav === 'push_status' && <PushStatusView runs={runs} deliveryLogs={deliveryLogs} onLoadSteps={loadStepRuns} />}
        {activeNav === 'methods' && <MethodsView methods={methods} coreMethods={coreMethods} onToggle={toggleTarget} />}
        {activeNav === 'receive' && <ReceiveView records={receivedData} coreMethod={coreMethods.find((item) => item.key === 'interface_ingest')} />}
        {activeNav === 'pull' && <PullView sources={sources} records={pulledData} coreMethods={coreMethods.filter((item) => item.key === 'youzan_fetch' || item.key === 'bojun_order_fetch')} onToggle={toggleTarget} />}
        {activeNav === 'process' && <ProcessView rules={transformRules} records={processedData} coreMethod={coreMethods.find((item) => item.key === 'qimai_process')} onToggle={toggleTarget} />}
        {activeNav === 'push' && <PushConfigView destinations={destinations} tasks={deliveryTasks} coreMethod={coreMethods.find((item) => item.key === 'mall_push')} onToggle={toggleTarget} />}
        {activeNav === 'logs' && <LogsView runs={runs} stepRuns={stepRuns} deliveryLogs={deliveryLogs} onLoadSteps={loadStepRuns} />}
      </section>

      <ResultPanel result={result} />
    </main>
  )
}

function LoginScreen({ onLogin }: { onLogin: (token: string) => void }) {
  const [error, setError] = useState('')

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const response = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: formValue(form, 'username'), password: formValue(form, 'password') }),
    })
    const data: unknown = await response.json().catch(() => ({}))
    const token = readToken(data)
    if (!response.ok || !token) {
      setError(readMessage(data) || '登录失败')
      return
    }
    onLogin(token)
  }

  return (
    <main className="login-shell">
      <section className="login-panel">
        <div className="login-title">
          <Database aria-hidden="true" />
          <div>
            <h1>数据仓库</h1>
            <p>登录后查看运行情况和现有方法</p>
          </div>
        </div>
        <form className="login-form" onSubmit={submit}>
          <Field label="用户名" name="username" defaultValue="admin" />
          <Field label="密码" name="password" defaultValue="123456" type="password" />
          {error && <div className="login-error">{error}</div>}
          <button className="primary" type="submit">登录</button>
        </form>
      </section>
    </main>
  )
}

function ModuleHeader({ activeNav, loading }: { activeNav: NavKey; loading: boolean }) {
  const titles: Record<NavKey, { title: string; subtitle: string }> = {
    push_status: { title: '推送情况', subtitle: '查看最近推送运行、成功失败数量和推送日志状态。' },
    methods: { title: '方法', subtitle: '只展示当前已配置方法和系统内置方法；方法如何使用由后续模块决定。' },
    receive: { title: '数据接收', subtitle: '查看对方通过接口发送过来的原始数据情况。' },
    pull: { title: '数据拉取', subtitle: '查看 API 数据源配置和通过 API 拉取落库的数据。' },
    process: { title: '数据处理', subtitle: '查看清洗规则和已处理数据。' },
    push: { title: '数据推送', subtitle: '查看配置好的推送目标和推送任务。' },
    logs: { title: '日志', subtitle: '查看流水线运行日志、步骤日志和推送日志。' },
  }
  return (
    <header className="workspace-header">
      <div>
        <p className="eyebrow">warehouse overview</p>
        <h2>{titles[activeNav].title}</h2>
        <span>{titles[activeNav].subtitle}</span>
      </div>
      <StatusPill label={loading ? '加载中' : '已就绪'} />
    </header>
  )
}

function PushStatusView({ runs, deliveryLogs, onLoadSteps }: { runs: PipelineRun[]; deliveryLogs: DeliveryLog[]; onLoadSteps: (runId: number) => void }) {
  const deliveryRuns = runs.filter((run) => run.run_type === 'delivery')
  const failedLogs = deliveryLogs.filter((log) => log.status !== 'success')
  return (
    <div className="view-stack">
      <section className="overview-grid">
        <Metric label="推送运行" value={deliveryRuns.length} />
        <Metric label="成功记录" value={sum(deliveryRuns, 'success_count')} />
        <Metric label="失败记录" value={sum(deliveryRuns, 'failed_count') + failedLogs.length} />
        <Metric label="最近日志" value={deliveryLogs.length} />
      </section>
      <section className="content-grid two">
        <Panel title="最近推送运行" icon={<Activity />} meta="delivery runs">
          <RunTable runs={deliveryRuns.length ? deliveryRuns : runs.slice(0, 12)} onLoadSteps={onLoadSteps} />
        </Panel>
        <Panel title="最近推送日志" icon={<Send />} meta="delivery logs">
          <DeliveryLogList logs={deliveryLogs} />
        </Panel>
      </section>
    </div>
  )
}

function MethodsView({ methods, coreMethods, onToggle }: { methods: MethodDisplay[]; coreMethods: CoreMethod[]; onToggle: (target: ToggleTarget, enabled: boolean) => void }) {
  const groups = groupBy(methods, (method) => method.category)
  return (
    <div className="view-stack">
      <section className="overview-grid">
        <Metric label="已配置方法" value={methods.filter((item) => item.kind === 'configured').length} />
        <Metric label="内置方法" value={methods.filter((item) => item.kind === 'builtin').length} />
        <Metric label="启用方法" value={methods.filter((item) => item.enabled).length} />
        <Metric label="方法类型" value={new Set(methods.map((item) => item.method_type)).size} />
      </section>
      <Panel title="当前已有核心方法" icon={<Wrench />} meta="可开启的真实配置会显示操作按钮">
        <CoreMethodList methods={coreMethods} onToggle={onToggle} />
      </Panel>
      <section className="method-groups">
        {Object.entries(groups).map(([category, items]) => (
          <Panel title={category} icon={<Wrench />} meta={`${items.length} 个方法`} key={category}>
            <div className="method-list">
              {items.map((method) => (
                <article className="method-row" key={method.key}>
                  <div>
                    <strong>{method.name}</strong>
                    <span>{method.description}</span>
                  </div>
                  <div className="method-meta">
                    <StatusPill label={method.kind === 'builtin' ? '内置' : '已配置'} />
                    <code>{method.code}</code>
                    <small>{method.owner}</small>
                    {method.toggle && <ToggleButton enabled={method.enabled} target={method.toggle} onToggle={onToggle} />}
                  </div>
                </article>
              ))}
            </div>
          </Panel>
        ))}
      </section>
    </div>
  )
}

function ReceiveView({ records, coreMethod }: { records: RawData[]; coreMethod?: CoreMethod }) {
  return (
    <div className="view-stack">
      {coreMethod && <Panel title="接口数据接收方法" icon={<Inbox />} meta="现有接收入口"><CoreMethodList methods={[coreMethod]} /></Panel>}
      <section className="overview-grid">
        <Metric label="接收记录" value={records.length} />
        <Metric label="已排队" value={records.filter((item) => item.status === 'queued').length} />
        <Metric label="已清洗" value={records.filter((item) => item.status === 'cleaned').length} />
        <Metric label="失败" value={records.filter((item) => item.status === 'failed').length} />
      </section>
      <Panel title="接口接收数据" icon={<Inbox />} meta="raw ingest">
        <RawDataList records={records} />
      </Panel>
    </div>
  )
}

function PullView({ sources, records, coreMethods, onToggle }: { sources: SourceDefinition[]; records: RawData[]; coreMethods: CoreMethod[]; onToggle: (target: ToggleTarget, enabled: boolean) => void }) {
  return (
    <div className="view-stack">
      {coreMethods.length > 0 && <Panel title="数据拉取方法" icon={<ArrowDownToLine />} meta="现有拉取能力"><CoreMethodList methods={coreMethods} onToggle={onToggle} /></Panel>}
      <section className="overview-grid">
        <Metric label="拉取数据源" value={sources.length} />
        <Metric label="启用数据源" value={sources.filter((item) => item.enabled).length} />
        <Metric label="拉取记录" value={records.length} />
        <Metric label="API 类型" value={sources.filter((item) => item.source_type.includes('api')).length} />
      </section>
      <section className="content-grid two">
        <Panel title="API 数据源" icon={<Database />} meta="sources">
          <SourceList sources={sources} />
        </Panel>
        <Panel title="API 拉取数据" icon={<ArrowDownToLine />} meta="fetched raw data">
          <RawDataList records={records} />
        </Panel>
      </section>
    </div>
  )
}

function ProcessView({ rules, records, coreMethod, onToggle }: { rules: TransformRule[]; records: ProcessedData[]; coreMethod?: CoreMethod; onToggle: (target: ToggleTarget, enabled: boolean) => void }) {
  return (
    <div className="view-stack">
      {coreMethod && <Panel title="企迈标签数据处理方法" icon={<ListChecks />} meta="现有处理能力"><CoreMethodList methods={[coreMethod]} onToggle={onToggle} /></Panel>}
      <section className="overview-grid">
        <Metric label="处理规则" value={rules.length} />
        <Metric label="启用规则" value={rules.filter((item) => item.enabled).length} />
        <Metric label="处理数据" value={records.length} />
        <Metric label="平均质量" value={average(records.map((item) => item.quality_score)).toFixed(1)} />
      </section>
      <section className="content-grid two">
        <Panel title="数据处理规则" icon={<ListChecks />} meta="transform rules">
          <TransformRuleList rules={rules} />
        </Panel>
        <Panel title="已处理数据" icon={<CheckCircle2 />} meta="processed records">
          <ProcessedDataList records={records} />
        </Panel>
      </section>
    </div>
  )
}

function PushConfigView({ destinations, tasks, coreMethod, onToggle }: { destinations: DestinationDefinition[]; tasks: DeliveryTask[]; coreMethod?: CoreMethod; onToggle: (target: ToggleTarget, enabled: boolean) => void }) {
  return (
    <div className="view-stack">
      {coreMethod && <Panel title="商场数据推送方法" icon={<Send />} meta="现有推送能力"><CoreMethodList methods={[coreMethod]} onToggle={onToggle} /></Panel>}
      <section className="overview-grid">
        <Metric label="推送目标" value={destinations.length} />
        <Metric label="启用目标" value={destinations.filter((item) => item.enabled).length} />
        <Metric label="推送任务" value={tasks.length} />
        <Metric label="启用任务" value={tasks.filter((item) => item.enabled).length} />
      </section>
      <section className="content-grid two">
        <Panel title="推送目标" icon={<Send />} meta="destinations">
          <DestinationList destinations={destinations} />
        </Panel>
        <Panel title="推送任务" icon={<ArrowUpFromLine />} meta="delivery tasks">
          <DeliveryTaskList tasks={tasks} />
        </Panel>
      </section>
    </div>
  )
}

function CoreMethodList({ methods, onToggle }: { methods: CoreMethod[]; onToggle?: (target: ToggleTarget, enabled: boolean) => void }) {
  return (
    <div className="method-list">
      {methods.map((method) => (
        <article className="method-row" key={method.key}>
          <div>
            <strong>{method.title}</strong>
            <span>{method.description}</span>
          </div>
          <div className="method-meta">
            <StatusPill label={method.enabled ? '已开启' : '已关闭'} />
            <code>{method.category}</code>
            <small>{method.status}</small>
            {onToggle && method.refs.map((target) => <ToggleButton enabled={method.enabled} key={`${target.type}-${target.id}`} target={target} onToggle={onToggle} />)}
          </div>
        </article>
      ))}
    </div>
  )
}

function ToggleButton({ enabled, target, onToggle }: { enabled: boolean; target: ToggleTarget; onToggle: (target: ToggleTarget, enabled: boolean) => void }) {
  return (
    <button type="button" onClick={() => onToggle(target, !enabled)}>
      {enabled ? '停用' : '开启'}
    </button>
  )
}

function LogsView({ runs, stepRuns, deliveryLogs, onLoadSteps }: { runs: PipelineRun[]; stepRuns: StepRun[]; deliveryLogs: DeliveryLog[]; onLoadSteps: (runId: number) => void }) {
  return (
    <div className="view-stack">
      <section className="content-grid two">
        <Panel title="运行日志" icon={<ScrollText />} meta="pipeline runs">
          <RunTable runs={runs} onLoadSteps={onLoadSteps} />
        </Panel>
        <Panel title="推送日志" icon={<Send />} meta="delivery logs">
          <DeliveryLogList logs={deliveryLogs} />
        </Panel>
      </section>
      <Panel title="步骤日志" icon={<BookOpen />} meta="选择运行记录查看步骤">
        <StepRunList stepRuns={stepRuns} />
      </Panel>
    </div>
  )
}

function RunTable({ runs, onLoadSteps }: { runs: PipelineRun[]; onLoadSteps: (runId: number) => void }) {
  if (runs.length === 0) return <EmptyState text="暂无运行记录。" />
  return (
    <div className="data-table-wrap">
      <table className="data-table">
        <thead><tr><th>ID</th><th>类型</th><th>状态</th><th>成功/总数</th><th>开始时间</th><th>明细</th></tr></thead>
        <tbody>
          {runs.slice(0, 20).map((run) => (
            <tr key={run.id}>
              <td>{run.id}</td>
              <td>{run.run_type}</td>
              <td>{run.status}</td>
              <td>{run.success_count}/{run.total_count}</td>
              <td>{formatDate(run.started_at)}</td>
              <td><button type="button" onClick={() => onLoadSteps(run.id)}>查看</button></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function DeliveryLogList({ logs }: { logs: DeliveryLog[] }) {
  if (logs.length === 0) return <EmptyState text="暂无推送日志。" />
  return (
    <div className="record-list">
      {logs.slice(0, 20).map((log) => (
        <article className="record-row" key={log.id}>
          <div>
            <strong>#{log.id} / {log.status}</strong>
            <span>{log.error_message || `trace: ${log.trace_id || '-'}`}</span>
          </div>
          <small>{formatDate(log.delivered_at)}</small>
        </article>
      ))}
    </div>
  )
}

function RawDataList({ records }: { records: RawData[] }) {
  if (records.length === 0) return <EmptyState text="暂无原始数据。" />
  return (
    <div className="record-list">
      {records.slice(0, 30).map((record) => (
        <article className="record-row" key={record.id}>
          <div>
            <strong>#{record.id} / {record.data_type || 'raw'}</strong>
            <span>来源 {record.data_source_id || '-'} / 状态 {record.status || '-'}</span>
          </div>
          <small>{rawDataOrigin(record)}</small>
        </article>
      ))}
    </div>
  )
}

function SourceList({ sources }: { sources: SourceDefinition[] }) {
  if (sources.length === 0) return <EmptyState text="暂无数据源配置。" />
  return (
    <div className="record-list">
      {sources.map((source) => (
        <article className="record-row" key={source.id}>
          <div>
            <strong>{source.name}</strong>
            <span>{source.code} / {source.source_type} / {source.auth_type || 'none'}</span>
          </div>
          <StatusPill label={source.enabled ? '启用' : '停用'} />
        </article>
      ))}
    </div>
  )
}

function TransformRuleList({ rules }: { rules: TransformRule[] }) {
  if (rules.length === 0) return <EmptyState text="暂无处理规则。" />
  return (
    <div className="record-list">
      {rules.map((rule) => (
        <article className="record-row" key={rule.id}>
          <div>
            <strong>{rule.name}</strong>
            <span>{rule.rule_type} / source #{rule.source_id} / 顺序 {rule.order_index}</span>
          </div>
          <StatusPill label={rule.enabled ? '启用' : '停用'} />
        </article>
      ))}
    </div>
  )
}

function ProcessedDataList({ records }: { records: ProcessedData[] }) {
  if (records.length === 0) return <EmptyState text="暂无处理后数据。" />
  return (
    <div className="record-list">
      {records.slice(0, 30).map((record) => (
        <article className="record-row" key={record.id}>
          <div>
            <strong>#{record.id} / {record.data_type || 'processed'}</strong>
            <span>raw #{record.raw_data_id} / 质量 {record.quality_score}</span>
          </div>
          <small>{record.created_at || '-'}</small>
        </article>
      ))}
    </div>
  )
}

function DestinationList({ destinations }: { destinations: DestinationDefinition[] }) {
  if (destinations.length === 0) return <EmptyState text="暂无推送目标。" />
  return (
    <div className="record-list">
      {destinations.map((destination) => (
        <article className="record-row" key={destination.id}>
          <div>
            <strong>{destination.name}</strong>
            <span>{destination.code} / {destination.destination_type}</span>
          </div>
          <StatusPill label={destination.enabled ? '启用' : '停用'} />
        </article>
      ))}
    </div>
  )
}

function DeliveryTaskList({ tasks }: { tasks: DeliveryTask[] }) {
  if (tasks.length === 0) return <EmptyState text="暂无推送任务。" />
  return (
    <div className="record-list">
      {tasks.map((task) => (
        <article className="record-row" key={task.id}>
          <div>
            <strong>{task.name}</strong>
            <span>{`${task.clean_table} -> destination #${task.destination_id} / ${task.trigger_type}`}</span>
          </div>
          <StatusPill label={task.enabled ? '启用' : '停用'} />
        </article>
      ))}
    </div>
  )
}

function StepRunList({ stepRuns }: { stepRuns: StepRun[] }) {
  if (stepRuns.length === 0) return <EmptyState text="选择运行日志中的“查看”后显示步骤日志。" />
  return (
    <div className="step-run-list">
      {stepRuns.map((run) => (
        <details key={run.id}>
          <summary>{run.step_code} / {run.method_type} / {run.status}</summary>
          <ReadonlyJSON value={{ input: parseJsonText(run.input_json), output: parseJsonText(run.output_json), error: run.error_message }} />
        </details>
      ))}
    </div>
  )
}

function ResultPanel({ result }: { result: ApiResult | null }) {
  return (
    <aside className="result-panel">
      <PanelTitle icon={<FileJson />} title="接口结果" meta={result ? String(result.status) : '等待操作'} />
      <ReadonlyJSON value={result?.data ?? {}} />
    </aside>
  )
}

function Panel({ title, icon, meta, children }: { title: string; icon: ReactNode; meta: string; children: ReactNode }) {
  return (
    <section className="workbench-panel">
      <PanelTitle icon={icon} title={title} meta={meta} />
      {children}
    </section>
  )
}

function PanelTitle({ icon, title, meta }: { icon: ReactNode; title: string; meta: string }) {
  return (
    <div className="panel-title">
      {icon}
      <div>
        <h3>{title}</h3>
        <span>{meta}</span>
      </div>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: ReactNode }) {
  return <div className="metric"><span>{label}</span><strong>{value}</strong></div>
}

function StatusPill({ label }: { label: string }) {
  return <span className="status-pill">{label}</span>
}

function EmptyState({ text }: { text: string }) {
  return <div className="empty-state">{text}</div>
}

function Field({ label, name, defaultValue, type = 'text' }: { label: string; name: string; defaultValue: string; type?: string }) {
  return (
    <label>
      {label}
      <input name={name} defaultValue={defaultValue} type={type} />
    </label>
  )
}

function ReadonlyJSON({ value }: { value: unknown }) {
  return <pre className="json-preview" aria-label="只读 JSON">{jsonText(value)}</pre>
}

function methodCategory(methodType: MethodType) {
  const categories: Record<MethodType, string> = {
    request: '数据拉取方法',
    bojun_signed_request: '数据拉取方法',
    extract: '数据拉取方法',
    mapping: '数据处理方法',
    validate: '前置验证方法',
    db_query: '数据处理方法',
    db_write: '数据处理方法',
    template: '数据推送方法',
    delivery: '推送方法',
    log: '日志方法',
    utility: '内置工具方法',
  }
  return categories[methodType] ?? '其它方法'
}

function buildConfiguredMethodDisplays(sources: SourceDefinition[], rules: TransformRule[], destinations: DestinationDefinition[], tasks: DeliveryTask[]): MethodDisplay[] {
  return [
    ...sources.map((source) => ({
      key: `source-${source.id}`,
      kind: 'configured' as const,
      name: source.name,
      code: source.code,
      method_type: 'request' as const,
      category: source.code.includes('youzan') || source.name.includes('有赞') ? '有赞数据拉取' : '数据拉取方法',
      owner: '数据源配置',
      description: `${source.source_type} 数据源，接收键 ${source.source_query_key || '-'}。`,
      enabled: source.enabled,
      toggle: { type: 'source' as const, id: source.id },
    })),
    ...rules.map((rule) => ({
      key: `rule-${rule.id}`,
      kind: 'configured' as const,
      name: rule.name,
      code: `transform_rule_${rule.id}`,
      method_type: rule.rule_type === 'validator' ? 'validate' as const : 'mapping' as const,
      category: rule.name.includes('企迈') || rule.config_json.includes('qimai') ? '企迈数据处理' : '数据处理方法',
      owner: '清洗规则配置',
      description: `${rule.rule_type} 规则，source #${rule.source_id}，顺序 ${rule.order_index}。`,
      enabled: rule.enabled,
      toggle: { type: 'transform_rule' as const, id: rule.id },
    })),
    ...destinations.map((destination) => ({
      key: `destination-${destination.id}`,
      kind: 'configured' as const,
      name: destination.name,
      code: destination.code,
      method_type: 'delivery' as const,
      category: isMallText(`${destination.name} ${destination.code}`) ? '商场数据推送' : '推送方法',
      owner: '推送目标配置',
      description: `${destination.destination_type} 推送目标。`,
      enabled: destination.enabled,
      toggle: { type: 'destination' as const, id: destination.id },
    })),
    ...tasks.map((task) => ({
      key: `delivery-task-${task.id}`,
      kind: 'configured' as const,
      name: task.name,
      code: `delivery_task_${task.id}`,
      method_type: 'delivery' as const,
      category: isMallText(`${task.name} ${task.clean_table}`) ? '商场数据推送' : '推送方法',
      owner: '推送任务配置',
      description: `${task.clean_table} -> destination #${task.destination_id}，触发方式 ${task.trigger_type}。`,
      enabled: task.enabled,
      toggle: { type: 'delivery_task' as const, id: task.id },
    })),
  ]
}

function buildLegacyMethodDisplays(tasks: LegacyTask[], rules: LegacyTransformRule[]): MethodDisplay[] {
  return [
    ...tasks.map((task) => ({
      key: `legacy-task-${task.code}`,
      kind: 'builtin' as const,
      name: task.name,
      code: task.code,
      method_type: task.category === 'delivery' ? 'delivery' as const : task.category === 'process' ? 'mapping' as const : 'request' as const,
      category: legacyCategory(task),
      owner: '旧任务注册表',
      description: task.description,
      enabled: true,
    })),
    ...rules.map((rule) => ({
      key: `legacy-rule-${rule.code}`,
      kind: 'builtin' as const,
      name: rule.name,
      code: rule.code,
      method_type: rule.rule_type === 'http_enrich' ? 'request' as const : 'mapping' as const,
      category: rule.source_code === 'qimai_order' ? '企迈数据处理' : '数据处理方法',
      owner: '旧清洗规则',
      description: rule.description,
      enabled: true,
    })),
  ]
}

function buildCoreMethods({ sources, transformRules, destinations, deliveryTasks, legacyTasks, legacyRules }: {
  sources: SourceDefinition[]
  transformRules: TransformRule[]
  destinations: DestinationDefinition[]
  deliveryTasks: DeliveryTask[]
  legacyTasks: LegacyTask[]
  legacyRules: LegacyTransformRule[]
}): CoreMethod[] {
  const youzanSources = sources.filter((source) => isYouzanText(`${source.name} ${source.code} ${source.source_type}`))
  const youzanLegacy = legacyTasks.filter((task) => task.category === 'fetch' && isYouzanText(`${task.name} ${task.code} ${task.source_code}`))
  const qimaiRules = transformRules.filter((rule) => isQimaiText(`${rule.name} ${rule.config_json}`))
  const qimaiLegacy = [...legacyTasks.filter((task) => task.code === 'qimai_order_enrich'), ...legacyRules.filter((rule) => rule.code === 'qimai_order_http_enrich')]
  const mallDestinations = destinations.filter((destination) => isMallText(`${destination.name} ${destination.code} ${destination.config_json}`))
  const mallTasks = deliveryTasks.filter((task) => isMallText(`${task.name} ${task.clean_table} ${task.payload_template}`) || mallDestinations.some((destination) => destination.id === task.destination_id))
  const mallLegacy = legacyTasks.filter((task) => task.category === 'delivery' && isMallText(`${task.name} ${task.target_system} ${task.output_table}`))

  return [
    {
      key: 'interface_ingest',
      title: '接口数据接收',
      category: '数据接收方法',
      description: '对方通过 `/api/v1/data/ingest/raw` 等接口发送数据，系统保存原始数据并进入后续处理。',
      enabled: true,
      status: '接口入口已存在，无单独配置开关。',
      refs: [],
    },
    {
      key: 'youzan_fetch',
      title: '有赞数据拉取',
      category: '数据拉取方法',
      description: '现有有赞订单/退款拉取能力，包含旧任务和数据源配置。',
      enabled: youzanSources.length > 0 ? youzanSources.some((source) => source.enabled) : youzanLegacy.length > 0,
      status: youzanSources.length > 0 ? `${youzanSources.length} 个数据源配置` : `${youzanLegacy.length} 个旧任务注册`,
      refs: youzanSources.map((source) => ({ type: 'source' as const, id: source.id })),
    },
    {
      key: 'bojun_order_fetch',
      title: '伯俊订单拉取',
      category: '数据拉取方法',
      description: '每分钟调用伯俊 `/retail/retail.query` 拉取订单，逐条写入原始表并标记来源 `bojun_order`。',
      enabled: true,
      status: '系统定时任务：每分钟执行，运行参数来自 BOJUN_ORDER_* 环境变量。',
      refs: [],
    },
    {
      key: 'qimai_process',
      title: '企迈标签数据处理',
      category: '数据处理方法',
      description: '接收到带 qimai/remark=qimai_order 标签的原始数据后，请求企迈订单详情并写入 `qimai_order_data`。',
      enabled: qimaiRules.length > 0 ? qimaiRules.some((rule) => rule.enabled) : qimaiLegacy.length > 0,
      status: qimaiRules.length > 0 ? `${qimaiRules.length} 条清洗规则配置` : `${qimaiLegacy.length} 条旧处理规则`,
      refs: qimaiRules.map((rule) => ({ type: 'transform_rule' as const, id: rule.id })),
    },
    {
      key: 'mall_push',
      title: '商场数据推送',
      category: '数据推送方法',
      description: '现有推送到商场/杭州恒隆/西岸的销售数据推送能力；如果已关闭，可在这里开启对应推送任务。',
      enabled: [...mallDestinations, ...mallTasks].length > 0 ? mallDestinations.some((destination) => destination.enabled) && mallTasks.some((task) => task.enabled) : mallLegacy.length > 0,
      status: mallTasks.length > 0 ? `${mallTasks.length} 个推送任务，${mallDestinations.length} 个推送目标` : `${mallLegacy.length} 个旧推送任务`,
      refs: [
        ...mallDestinations.map((destination) => ({ type: 'destination' as const, id: destination.id })),
        ...mallTasks.map((task) => ({ type: 'delivery_task' as const, id: task.id })),
      ],
    },
  ]
}

async function updateTargetEnabled(client: ApiClient, target: ToggleTarget, enabled: boolean, data: {
  sources: SourceDefinition[]
  transformRules: TransformRule[]
  destinations: DestinationDefinition[]
  deliveryTasks: DeliveryTask[]
}) {
  if (target.type === 'source') {
    const source = data.sources.find((item) => item.id === target.id)
    if (!source) return { ok: false, status: 404, data: 'source not found' }
    return client(`/v1/sources/${target.id}`, {
      method: 'PUT',
      body: {
        name: source.name,
        code: source.code,
        source_type: source.source_type,
        enabled,
        auth_type: source.auth_type,
        config_json: source.config_json || '{}',
        schema_json: source.schema_json || '{}',
        dedupe_keys: source.dedupe_keys || '[]',
        source_query_key: source.source_query_key,
      },
    })
  }
  if (target.type === 'transform_rule') {
    const rule = data.transformRules.find((item) => item.id === target.id)
    if (!rule) return { ok: false, status: 404, data: 'transform rule not found' }
    return client(`/v1/transform-rules/${target.id}`, {
      method: 'PUT',
      body: {
        source_id: rule.source_id,
        name: rule.name,
        rule_type: rule.rule_type,
        order_index: rule.order_index,
        config_json: rule.config_json || '{}',
        enabled,
      },
    })
  }
  if (target.type === 'destination') {
    const destination = data.destinations.find((item) => item.id === target.id)
    if (!destination) return { ok: false, status: 404, data: 'destination not found' }
    return client(`/v1/destinations/${target.id}`, {
      method: 'PUT',
      body: {
        name: destination.name,
        code: destination.code,
        destination_type: destination.destination_type,
        config_json: destination.config_json || '{}',
        enabled,
      },
    })
  }
  const task = data.deliveryTasks.find((item) => item.id === target.id)
  if (!task) return { ok: false, status: 404, data: 'delivery task not found' }
  return client(`/v1/delivery-tasks/${target.id}`, {
    method: 'PUT',
    body: {
      name: task.name,
      source_id: task.source_id,
      clean_table: task.clean_table,
      destination_id: task.destination_id,
      trigger_type: task.trigger_type,
      cron_expr: task.cron_expr,
      filter_json: task.filter_json || '{}',
      payload_template: task.payload_template,
      enabled,
    },
  })
}

function legacyCategory(task: LegacyTask) {
  if (task.category === 'fetch' && isYouzanText(`${task.name} ${task.code}`)) return '有赞数据拉取'
  if (task.category === 'process' && isQimaiText(`${task.name} ${task.code}`)) return '企迈数据处理'
  if (task.category === 'delivery' && isMallText(`${task.name} ${task.target_system}`)) return '商场数据推送'
  return methodCategory(task.category === 'delivery' ? 'delivery' : task.category === 'process' ? 'mapping' : 'request')
}

function isYouzanText(value: string) {
  return /youzan|有赞/i.test(value)
}

function isQimaiText(value: string) {
  return /qimai|企迈/i.test(value)
}

function isMallText(value: string) {
  return /商场|商城|mall|henglong|恒隆|西岸|xian|plaza/i.test(value)
}

function methodDescription(detail: MethodStepDetail) {
  const inputs = detail.params?.length ?? 0
  const outputs = detail.outputs?.length ?? 0
  return `${methodTypeLabel(detail.step.method_type)}，入参 ${inputs} 个，出参 ${outputs} 个。`
}

function methodTypeLabel(type: MethodType) {
  const labels: Record<MethodType, string> = {
    request: 'Request 请求',
    bojun_signed_request: '伯俊签名请求',
    extract: 'Extract 提取',
    mapping: 'Mapping 清洗',
    validate: 'Validate 校验',
    db_query: 'DB Query 查询',
    db_write: 'DB Write 写入',
    template: 'Template 模板',
    delivery: 'Delivery 推送',
    log: 'Log 记录',
    utility: 'Utility 工具',
  }
  return labels[type] ?? type
}

function rawDataOrigin(record: RawData) {
  const metadata = parseMaybeJson(record.metadata)
  if (record.source) return record.source
  if (record.remark) return record.remark
  if (metadata && typeof metadata === 'object' && typeof (metadata as JsonRecord).source === 'string') return String((metadata as JsonRecord).source)
  if (metadata && typeof metadata === 'object' && typeof (metadata as JsonRecord).remark === 'string') return String((metadata as JsonRecord).remark)
  if (metadata && typeof metadata === 'object' && (metadata as JsonRecord).format === 'fetch') return 'fetch'
  if (metadata && typeof metadata === 'object' && typeof (metadata as JsonRecord).format === 'string') return String((metadata as JsonRecord).format)
  return 'ingest'
}

function isPulledOrigin(origin: string) {
  return origin === 'fetch' || origin === 'bojun_order' || origin === 'youzan_order' || origin === 'youzan_refund'
}

function readList<T>(result: ApiResult, key: string): T[] {
  const value = readDataField(result.data, key)
  return Array.isArray(value) ? (value as T[]) : []
}

function readObject<T>(result: ApiResult, key: string): T | null {
  const value = readDataField(result.data, key)
  return value && typeof value === 'object' ? (value as T) : null
}

function readDataField(data: unknown, key: string) {
  if (!data || typeof data !== 'object') return undefined
  const envelope = data as { data?: Record<string, unknown> }
  return envelope.data?.[key]
}

function readToken(data: unknown) {
  const value = readDataField(data, 'token')
  return typeof value === 'string' ? value : ''
}

function readMessage(data: unknown) {
  if (!data || typeof data !== 'object') return ''
  const envelope = data as { msg?: unknown }
  return typeof envelope.msg === 'string' ? envelope.msg : ''
}

function isSuccessPayload(data: unknown) {
  if (!data || typeof data !== 'object') return false
  const envelope = data as { code?: unknown }
  return envelope.code === 0 || envelope.code === 200
}

function formValue(form: FormData, key: string) {
  const value = form.get(key)
  return typeof value === 'string' ? value : ''
}

function jsonText(value: unknown) {
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  return JSON.stringify(value ?? {}, null, 2)
}

function parseJsonText(value: string) {
  if (!value) return {}
  try {
    return JSON.parse(value) as unknown
  } catch {
    return value
  }
}

function parseMaybeJson(value: unknown) {
  if (!value) return null
  if (typeof value === 'object') return value
  if (typeof value !== 'string') return null
  try {
    return JSON.parse(value) as unknown
  } catch {
    return null
  }
}

function formatDate(value: string | null) {
  if (!value) return '-'
  return value
}

function groupBy<T>(items: T[], keyFn: (item: T) => string) {
  return items.reduce<Record<string, T[]>>((groups, item) => {
    const key = keyFn(item)
    groups[key] = groups[key] ?? []
    groups[key].push(item)
    return groups
  }, {})
}

function sum(items: PipelineRun[], key: 'success_count' | 'failed_count') {
  return items.reduce((total, item) => total + (Number(item[key]) || 0), 0)
}

function average(values: number[]) {
  const filtered = values.filter((value) => Number.isFinite(value))
  if (filtered.length === 0) return 0
  return filtered.reduce((total, value) => total + value, 0) / filtered.length
}

export default App
