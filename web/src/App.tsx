import { FormEvent, ReactNode, useCallback, useEffect, useState } from 'react'
import {
  Activity,
  Database,
  Download,
  FileJson,
  LockKeyhole,
  LogOut,
  Play,
  Plus,
  RefreshCcw,
  Send,
  Settings,
  ShieldCheck,
  Upload,
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
type RefreshAction = (showResult?: boolean) => Promise<void>
type NavKey = 'sources' | 'transform' | 'delivery' | 'excel' | 'runs'
type SourceView = 'list' | 'legacy' | 'create' | 'debug'

const authStorageKey = 'warehouse-auth'
const tokenStorageKey = 'warehouse-token'

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
  created_at: number
}

type DestinationDefinition = {
  id: number
  name: string
  code: string
  destination_type: string
  config_json: string
  enabled: boolean
  created_at: number
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
  created_at: number
}

type DeliveryLog = {
  id: number
  trace_id: string
  run_id: number
  business_key: string
  http_status: number
  success: boolean
  error_message: string
  sent_at: string | null
}

type PipelineRun = {
  id: number
  trace_id: string
  run_type: string
  trigger_type: string
  source_id: number
  destination_id: number
  status: string
  total_count: number
  success_count: number
  failed_count: number
  started_at: string | null
  finished_at: string | null
}

type ExcelMatchJob = {
  id: number
  source_file_name: string
  config_json: string
  status: string
  total_rows: number
  processed_rows: number
  filtered_rows: number
  matched_rows: number
  unmatched_rows: number
  error_message: string
  started_at: string | null
  finished_at: string | null
  expires_at: string | null
  created_at: number
}

type LegacyTask = {
  code: string
  name: string
  category: 'fetch' | 'delivery' | 'process'
  source_code: string
  source_name: string
  task_type: string
  queue: string
  cron_expr: string
  input_table: string
  output_table: string
  target_system: string
  handler: string
  description: string
  editable: boolean
  default_payload: Record<string, unknown>
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
  handler: string
  description: string
  editable: boolean
  config: Record<string, unknown>
  steps: string[]
}

type Column<T> = {
  label: string
  render: (row: T) => ReactNode
}

const defaultMappingConfig = JSON.stringify(
  {
    table_name: 'clean_orders',
    business_key_field: 'order_no',
    fields: [
      { name: 'order_no', source_path: '$.params.orderNo', type: 'string', required: true },
      { name: 'actual_amount', source_path: '$.data.amount', type: 'decimal', transform: 'divide:100' },
    ],
  },
  null,
  2,
)

const sourceConfigTemplates = [
  {
    code: 'youzan_order_api',
    sourceCode: 'youzan_order_api',
    name: '有赞订单 API 拉取',
    description: '先请求有赞 token_url 获取 access_token，再把 token 注入订单接口 query 参数，body 使用最近 5 分钟支付成功时间窗口。',
    sourceType: 'api_poll',
    sourceQueryKey: 'data_source',
    dedupeKeys: ['order_info.tid'],
    schema: { target_table: 'youzan_order_data', primary_key: 'tid' },
    config: {
      description: '有赞订单拉取配置。动态值来自 cfg.youzan.*，不在页面保存真实密钥。',
      url: { source: 'config', path: 'cfg.youzan.orders_url' },
      method: 'POST',
      timeout_seconds: 30,
      auth: {
        type: 'request_token',
        request: {
          url: { source: 'config', path: 'cfg.youzan.token_url' },
          method: 'POST',
          headers: [{ name: 'Content-Type', value: 'application/json' }],
          body_json: {
            authorize_type: 'silent',
            client_id: { source: 'config', path: 'cfg.youzan.client_id' },
            client_secret: { source: 'config', path: 'cfg.youzan.client_secret' },
            grant_id: { source: 'config', path: 'cfg.youzan.grant_id' },
            refresh: false,
          },
        },
        token_path: 'data.access_token',
        inject: { in: 'query', name: 'access_token' },
      },
      headers: [{ name: 'Content-Type', value: 'application/json' }],
      body_json: {
        page_size: 100,
        page_no: 1,
        start_success: { source: 'time', format: '2006-01-02 15:04:05', offset_seconds: -300 },
        end_success: { source: 'time', format: '2006-01-02 15:04:05' },
      },
      records_path: 'data.full_order_info_list',
      parameter_docs: [
        { location: 'url', name: 'url', value: 'cfg.youzan.orders_url', source: '配置文件或环境变量 YOUZAN_ORDERS_URL' },
        { location: 'query', name: 'access_token', value: 'auth.data.access_token', source: 'token 前置请求返回' },
        { location: 'body', name: 'start_success/end_success', value: '当前时间窗口', source: '运行时动态计算' },
      ],
    },
  },
  {
    code: 'youzan_refund_api',
    sourceCode: 'youzan_refund_api',
    name: '有赞退款 API 拉取',
    description: '先获取有赞 access_token，再请求退款接口，body 使用最近 5 分钟 Unix 时间窗口。',
    sourceType: 'api_poll',
    sourceQueryKey: 'data_source',
    dedupeKeys: ['refund_id'],
    schema: { target_table: 'youzan_return_data', primary_key: 'refund_id' },
    config: {
      description: '有赞退款拉取配置。node_kdt_id 和接口地址从 cfg.youzan.* 获取。',
      url: { source: 'config', path: 'cfg.youzan.refund_url' },
      method: 'POST',
      timeout_seconds: 30,
      auth: {
        type: 'request_token',
        request: {
          url: { source: 'config', path: 'cfg.youzan.token_url' },
          method: 'POST',
          headers: [{ name: 'Content-Type', value: 'application/json' }],
          body_json: {
            authorize_type: 'silent',
            client_id: { source: 'config', path: 'cfg.youzan.client_id' },
            client_secret: { source: 'config', path: 'cfg.youzan.client_secret' },
            grant_id: { source: 'config', path: 'cfg.youzan.grant_id' },
            refresh: false,
          },
        },
        token_path: 'data.access_token',
        inject: { in: 'query', name: 'access_token' },
      },
      headers: [{ name: 'Content-Type', value: 'application/json' }],
      body_json: {
        create_time_start: { source: 'time', unix: true, offset_seconds: -300 },
        create_time_end: { source: 'time', unix: true },
        page_no: 1,
        page_size: 100,
      },
      records_path: 'data.refunds',
      parameter_docs: [
        { location: 'url', name: 'url', value: 'cfg.youzan.refund_url', source: '配置文件或环境变量 YOUZAN_REFUND_URL' },
        { location: 'query', name: 'access_token', value: 'auth.data.access_token', source: 'token 前置请求返回' },
        { location: 'body', name: 'create_time_start/create_time_end', value: 'Unix 时间戳', source: '运行时动态计算' },
      ],
    },
  },
  {
    code: 'qimai_order_enrich_api',
    sourceCode: 'qimai_order_enrich_api',
    name: '企迈订单详情补数 API',
    description: '企迈订单详情涉及签名 token，当前旧逻辑由 Trigger/qimai_order_trigger.go 执行；此模板用于显式记录 URL、凭证来源、body 字段和落表。',
    sourceType: 'api_poll',
    sourceQueryKey: 'source',
    dedupeKeys: ['params.orderNo'],
    schema: { target_table: 'qimai_order_data', primary_key: 'orderNo' },
    config: {
      description: '企迈订单详情补数配置说明。实际签名逻辑仍由旧 Trigger 处理，避免页面保存 open_key 明文。',
      url: { source: 'config', path: 'cfg.qimai.order_detail_url' },
      method: 'POST',
      timeout_seconds: 30,
      headers: [{ name: 'Content-Type', value: 'application/json' }],
      body_json: {
        openId: { source: 'config', path: 'cfg.qimai.open_id' },
        grantCode: { source: 'config', path: 'cfg.qimai.grant_code' },
        nonce: { source: 'config', path: 'cfg.qimai.nonce' },
        timestamp: { source: 'time', unix: true },
        token: { source: 'static', value: '由 openId/grantCode/timestamp/nonce/openKey 生成，旧 Trigger 中执行' },
        params: {
          bizType: 7,
          orderNo: { source: 'static', value: '从 raw_data.raw_content.params.orderNo 获取' },
        },
      },
      records_path: 'data',
      parameter_docs: [
        { location: 'body', name: 'openId/grantCode/nonce', value: 'cfg.qimai.*', source: '配置文件或 token_data' },
        { location: 'body', name: 'token', value: 'HMAC-SHA1 签名', source: 'Trigger/qimai_order_trigger.go' },
        { location: 'body', name: 'orderNo', value: '$.params.orderNo', source: '接收到的原始数据' },
      ],
    },
  },
  {
    code: 'generic_api',
    sourceCode: 'generic_api_source',
    name: '通用 API 拉取',
    description: '适合普通 HTTP API。可配置 URL、query 参数、header、body、响应列表路径和动态参数来源。',
    sourceType: 'api_poll',
    sourceQueryKey: 'source',
    dedupeKeys: ['id'],
    schema: { target_table: 'clean_records', primary_key: 'id' },
    config: {
      description: '通用 API 拉取配置模板。',
      url: 'https://example.com/api/orders',
      method: 'POST',
      timeout_seconds: 30,
      query_params: [{ name: 'shop', value: { source: 'env', name: 'SHOP_CODE', fallback: '' } }],
      headers: [
        { name: 'Content-Type', value: 'application/json' },
        { name: 'Authorization', value: { source: 'env', name: 'API_AUTHORIZATION', fallback: '' } },
      ],
      body_json: {
        start_time: { source: 'time', format: '2006-01-02 15:04:05', offset_seconds: -300 },
        end_time: { source: 'time', format: '2006-01-02 15:04:05' },
      },
      records_path: 'data.items',
    },
  },
] as const

function App() {
  const [authenticated, setAuthenticated] = useState(() => Boolean(sessionStorage.getItem(tokenStorageKey)))
  const [active, setActive] = useState<NavKey>('sources')
  const [token, setToken] = useState(() => sessionStorage.getItem(tokenStorageKey) ?? '')
  const [result, setResult] = useState<ApiResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [sources, setSources] = useState<SourceDefinition[]>([])
  const [rules, setRules] = useState<TransformRule[]>([])
  const [destinations, setDestinations] = useState<DestinationDefinition[]>([])
  const [tasks, setTasks] = useState<DeliveryTask[]>([])
  const [logs, setLogs] = useState<DeliveryLog[]>([])
  const [runs, setRuns] = useState<PipelineRun[]>([])
  const [legacyTasks, setLegacyTasks] = useState<LegacyTask[]>([])
  const [legacyTransformRules, setLegacyTransformRules] = useState<LegacyTransformRule[]>([])

  const client = useCallback<ApiClient>(
    async (path, options = {}) => {
      const method = options.method ?? 'POST'
      if (!options.silentLoading) {
        setLoading(true)
      }

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
        if (options.showResult !== false) {
          setResult(nextResult)
        }
        return nextResult
      } catch (error) {
        const nextResult = {
          ok: false,
          status: 0,
          data: error instanceof Error ? error.message : String(error),
        }
        if (options.showResult !== false) {
          setResult(nextResult)
        }
        return nextResult
      } finally {
        if (!options.silentLoading) {
          setLoading(false)
        }
      }
    },
    [token],
  )

  const refreshLists = useCallback<RefreshAction>(
    async (showResult = false) => {
      if (!token) {
        clearLists(setSources, setRules, setDestinations, setTasks, setLogs, setRuns, setLegacyTasks, setLegacyTransformRules)
        if (showResult) {
          setResult({ ok: false, status: 0, data: '登录状态已失效，请重新登录。' })
        }
        return
      }

      setRefreshing(true)
      try {
        const [
          sourceResult,
          ruleResult,
          destinationResult,
          taskResult,
          logResult,
          runResult,
          legacyTaskResult,
          legacyTransformRuleResult,
        ] = await Promise.all([
          client('/v1/sources', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/transform-rules', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/destinations', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/delivery-tasks', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/delivery-logs?limit=50', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/runs?limit=50', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/legacy-tasks', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/legacy-transform-rules', { method: 'GET', showResult: false, silentLoading: true }),
        ])

        if (sourceResult.ok) setSources(readList<SourceDefinition>(sourceResult, 'sources'))
        if (ruleResult.ok) setRules(readList<TransformRule>(ruleResult, 'rules'))
        if (destinationResult.ok) setDestinations(readList<DestinationDefinition>(destinationResult, 'destinations'))
        if (taskResult.ok) setTasks(readList<DeliveryTask>(taskResult, 'tasks'))
        if (logResult.ok) setLogs(readList<DeliveryLog>(logResult, 'logs'))
        if (runResult.ok) setRuns(readList<PipelineRun>(runResult, 'runs'))
        if (legacyTaskResult.ok) setLegacyTasks(readList<LegacyTask>(legacyTaskResult, 'tasks'))
        if (legacyTransformRuleResult.ok) setLegacyTransformRules(readList<LegacyTransformRule>(legacyTransformRuleResult, 'rules'))

        if (showResult) {
          const results = [
            sourceResult,
            ruleResult,
            destinationResult,
            taskResult,
            logResult,
            runResult,
            legacyTaskResult,
            legacyTransformRuleResult,
          ]
          const failed = results.find((item) => !item.ok)
          setResult({
            ok: !failed,
            status: failed?.status ?? 200,
            data: {
              sources: sourceResult.data,
              rules: ruleResult.data,
              destinations: destinationResult.data,
              tasks: taskResult.data,
              logs: logResult.data,
              runs: runResult.data,
              legacy_tasks: legacyTaskResult.data,
              legacy_transform_rules: legacyTransformRuleResult.data,
            },
          })
        }
      } finally {
        setRefreshing(false)
      }
    },
    [client, token],
  )

  useEffect(() => {
    if (!authenticated) return
    if (!token) {
      clearLists(setSources, setRules, setDestinations, setTasks, setLogs, setRuns, setLegacyTasks, setLegacyTransformRules)
      return
    }
    void refreshLists(false)
  }, [authenticated, refreshLists, token])

  function handleLoginSuccess(nextToken: string) {
    sessionStorage.setItem(authStorageKey, 'ok')
    sessionStorage.setItem(tokenStorageKey, nextToken)
    setToken(nextToken)
    setAuthenticated(true)
  }

  function handleLogout() {
    sessionStorage.removeItem(authStorageKey)
    sessionStorage.removeItem(tokenStorageKey)
    setToken('')
    setAuthenticated(false)
    setResult(null)
    clearLists(setSources, setRules, setDestinations, setTasks, setLogs, setRuns, setLegacyTasks, setLegacyTransformRules)
  }

  if (!authenticated) {
    return <LoginScreen onLogin={handleLoginSuccess} />
  }

  return (
    <main className="app-shell">
      <aside className="sidebar" aria-label="主导航">
        <div className="brand">
          <Database aria-hidden="true" />
          <div>
            <h1>数据仓库</h1>
            <span>流水线管理台</span>
          </div>
        </div>
        <nav className="nav-list">
          <NavButton active={active === 'sources'} icon={<Database />} label="数据源" onClick={() => setActive('sources')} />
          <NavButton active={active === 'transform'} icon={<FileJson />} label="清洗规则" onClick={() => setActive('transform')} />
          <NavButton active={active === 'delivery'} icon={<Send />} label="推送任务" onClick={() => setActive('delivery')} />
          <NavButton active={active === 'excel'} icon={<Upload />} label="Excel 匹配" onClick={() => setActive('excel')} />
          <NavButton active={active === 'runs'} icon={<Activity />} label="运行记录" onClick={() => setActive('runs')} />
        </nav>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">develop 分支</p>
            <h2>{sectionTitle(active)}</h2>
          </div>
          <div className="connection-panel" aria-label="接口设置">
            <Settings aria-hidden="true" />
            <span className="connection-status">已连接后端认证</span>
            <button type="button" onClick={() => refreshLists(true)} disabled={loading || refreshing}>
              <RefreshCcw aria-hidden="true" />
              刷新
            </button>
            <button type="button" onClick={handleLogout}>
              <LogOut aria-hidden="true" />
              退出
            </button>
          </div>
        </header>

        <div className="content-grid">
          <section className="panel">
            {active === 'sources' && (
              <SourcesPanel
                client={client}
                loading={loading}
                refreshing={refreshing}
                sources={sources}
                legacyTasks={legacyTasks.filter((task) => task.category === 'fetch' || task.category === 'process')}
                onRefresh={refreshLists}
              />
            )}
            {active === 'transform' && (
              <TransformPanel
                client={client}
                loading={loading}
                refreshing={refreshing}
                rules={rules}
                legacyRules={legacyTransformRules}
                onRefresh={refreshLists}
              />
            )}
            {active === 'delivery' && (
              <DeliveryPanel
                client={client}
                loading={loading}
                refreshing={refreshing}
                destinations={destinations}
                tasks={tasks}
                logs={logs}
                legacyTasks={legacyTasks.filter((task) => task.category === 'delivery')}
                onRefresh={refreshLists}
              />
            )}
            {active === 'excel' && (
              <ExcelMatchPanel token={token} loading={loading} setLoading={setLoading} setResult={setResult} />
            )}
            {active === 'runs' && <RunsPanel refreshing={refreshing} runs={runs} onRefresh={refreshLists} />}
          </section>

          <aside className="result-panel" aria-live="polite">
            <div className="panel-heading">
              <ShieldCheck aria-hidden="true" />
              <h3>接口返回</h3>
            </div>
            {result ? (
              <pre className={result.ok ? 'result success' : 'result error'}>{JSON.stringify(result, null, 2)}</pre>
            ) : (
              <div className="empty-state">还没有发送请求。登录后会自动关联后端接口。</div>
            )}
          </aside>
        </div>
      </section>
    </main>
  )
}

function LoginScreen({ onLogin }: { onLogin: (token: string) => void }) {
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function submitLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const username = String(form.get('username') ?? '')
    const password = String(form.get('password') ?? '')

    setLoading(true)
    try {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })
      const data = await response.json().catch(() => ({}))
      const nextToken = readToken(data)
      if (response.ok && isSuccessPayload(data) && nextToken) {
        setError('')
        onLogin(nextToken)
        return
      }

      setError(readMessage(data) || '账号或密码不正确')
    } catch (loginError) {
      setError(loginError instanceof Error ? loginError.message : '无法连接后端登录接口')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="login-shell">
      <section className="login-panel" aria-label="登录">
        <div className="login-title">
          <LockKeyhole aria-hidden="true" />
          <div>
            <h1>数据仓库管理台</h1>
            <p>请输入管理员账号后继续操作。</p>
          </div>
        </div>
        <form className="login-form" onSubmit={submitLogin}>
          <label>
            账号
            <input name="username" autoComplete="username" defaultValue="admin" autoFocus />
          </label>
          <label>
            密码
            <input name="password" type="password" autoComplete="current-password" defaultValue="youlan123" />
          </label>
          {error && <div className="login-error">{error}</div>}
          <button className="primary" type="submit" disabled={loading}>
            <ShieldCheck aria-hidden="true" />
            登录
          </button>
        </form>
      </section>
    </main>
  )
}

function SourcesPanel({
  client,
  loading,
  refreshing,
  sources,
  legacyTasks,
  onRefresh,
}: {
  client: ApiClient
  loading: boolean
  refreshing: boolean
  sources: SourceDefinition[]
  legacyTasks: LegacyTask[]
  onRefresh: RefreshAction
}) {
  const [sourceID, setSourceID] = useState('1')
  const [sourceView, setSourceView] = useState<SourceView>('list')
  const [selectedTemplateCode, setSelectedTemplateCode] = useState<string>(sourceConfigTemplates[0].code)
  const [selectedSource, setSelectedSource] = useState<SourceDefinition | null>(null)
  const selectedTemplate =
    sourceConfigTemplates.find((template) => template.code === selectedTemplateCode) ?? sourceConfigTemplates[0]

  async function createSource(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const response = await client('/v1/sources', {
      body: {
        name: form.get('name'),
        code: form.get('code'),
        source_type: form.get('source_type'),
        auth_type: form.get('auth_type'),
        source_query_key: form.get('source_query_key'),
        config_json: form.get('config_json'),
        schema_json: form.get('schema_json'),
        dedupe_keys: form.get('dedupe_keys'),
        enabled: true,
      },
    })
    if (response.ok) await onRefresh(false)
  }

  async function runSourceAction(path: string) {
    const response = await client(path)
    if (response.ok) await onRefresh(false)
  }

  async function updateSource(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedSource) return

    const form = new FormData(event.currentTarget)
    const response = await client(`/v1/sources/${selectedSource.id}`, {
      method: 'PUT',
      body: {
        name: form.get('name'),
        code: form.get('code'),
        source_type: form.get('source_type'),
        auth_type: form.get('auth_type'),
        source_query_key: form.get('source_query_key'),
        config_json: form.get('config_json'),
        schema_json: form.get('schema_json'),
        dedupe_keys: form.get('dedupe_keys'),
        enabled: form.get('enabled') === 'on',
      },
    })
    if (response.ok) {
      setSelectedSource(null)
      await onRefresh(false)
    }
  }

  return (
    <>
      <SummaryGrid
        items={[
          { label: '已配置数据源', value: sources.length },
          { label: '已启用数据源', value: sources.filter((source) => source.enabled).length },
          { label: '迁移拉取/补数规则', value: legacyTasks.length },
        ]}
      />
      <SectionTabs
        active={sourceView}
        onChange={setSourceView}
        tabs={[
          { key: 'list', label: '已配置数据源', count: sources.length },
          { key: 'legacy', label: '迁移拉取/补数规则', count: legacyTasks.length },
          { key: 'create', label: '新增数据源' },
          { key: 'debug', label: '调试执行' },
        ]}
      />
      {sourceView === 'list' && (
        <>
      <ListHeader title="已配置数据源" count={sources.length} refreshing={refreshing} onRefresh={onRefresh} />
      <DataTable
        rows={sources}
        emptyText="暂无数据源。登录后刷新，或创建一个新的数据源。"
        columns={[
          { label: 'ID', render: (source) => source.id },
          { label: '名称', render: (source) => source.name },
          { label: '编码', render: (source) => source.code },
          { label: '类型', render: (source) => sourceTypeLabel(source.source_type) },
          { label: '状态', render: (source) => <StatusBadge active={source.enabled} /> },
          { label: '来源参数', render: (source) => source.source_query_key || '-' },
          { label: '创建时间', render: (source) => formatUnixTime(source.created_at) },
          {
            label: '操作',
            render: (source) => (
              <button className="table-action" type="button" onClick={() => setSelectedSource(source)}>
                详情/编辑
              </button>
            ),
          },
        ]}
      />

      {selectedSource && (
        <DetailPanel title={`数据源详情：${selectedSource.name}`} onClose={() => setSelectedSource(null)}>
          <form className="form-grid compact" key={selectedSource.id} onSubmit={updateSource}>
            <Field label="名称" name="name" defaultValue={selectedSource.name} />
            <Field label="编码" name="code" defaultValue={selectedSource.code} />
            <SourceTypeSelect defaultValue={selectedSource.source_type} />
            <AuthTypeSelect defaultValue={selectedSource.auth_type || 'none'} />
            <Field label="来源参数名" name="source_query_key" defaultValue={selectedSource.source_query_key || ''} />
            <label className="checkbox-label">
              <input name="enabled" type="checkbox" defaultChecked={selectedSource.enabled} />
              启用
            </label>
            <JsonField label="配置 JSON" name="config_json" value={selectedSource.config_json || '{}'} rows={7} />
            <JsonField label="结构定义 JSON" name="schema_json" value={selectedSource.schema_json || '{}'} rows={5} />
            <JsonField label="去重字段 JSON" name="dedupe_keys" value={selectedSource.dedupe_keys || '[]'} rows={4} />
            <button className="primary" disabled={loading}>
              保存数据源
            </button>
          </form>
        </DetailPanel>
      )}
        </>
      )}

      {sourceView === 'legacy' && (
        <>
      <ListHeader title="已迁移拉取/补数规则" count={legacyTasks.length} refreshing={refreshing} onRefresh={onRefresh} />
      <LegacyTaskTable
        client={client}
        loading={loading}
        tasks={legacyTasks}
        emptyText="暂无迁移的拉取或补数规则。"
        onRefresh={onRefresh}
      />
        </>
      )}

      {sourceView === 'create' && (
        <>
      <div className="panel-heading form-heading">
        <Plus aria-hidden="true" />
        <h3>新增数据源</h3>
      </div>
      <div className="template-note">
        <h3>{selectedTemplate.name}</h3>
        <p>{selectedTemplate.description}</p>
        <div className="template-points">
          <MetaItem label="数据源类型" value={sourceTypeLabel(selectedTemplate.sourceType)} />
          <MetaItem label="来源参数" value={selectedTemplate.sourceQueryKey} />
          <MetaItem label="目标表" value={selectedTemplate.schema.target_table} />
          <MetaItem label="去重字段" value={selectedTemplate.dedupeKeys.join(', ')} />
        </div>
      </div>
      <form className="form-grid" key={selectedTemplate.code} onSubmit={createSource}>
        <label>
          配置模板
          <select value={selectedTemplateCode} onChange={(event) => setSelectedTemplateCode(event.target.value)}>
            {sourceConfigTemplates.map((template) => (
              <option key={template.code} value={template.code}>
                {template.name}
              </option>
            ))}
          </select>
        </label>
        <Field label="名称" name="name" defaultValue={selectedTemplate.name} />
        <Field label="编码" name="code" defaultValue={selectedTemplate.sourceCode} />
        <SourceTypeSelect defaultValue={selectedTemplate.sourceType} />
        <AuthTypeSelect defaultValue="none" />
        <Field label="来源参数名" name="source_query_key" defaultValue={selectedTemplate.sourceQueryKey} />
        <JsonField label="配置 JSON" name="config_json" value={jsonText(selectedTemplate.config)} rows={16} />
        <JsonField label="结构定义 JSON" name="schema_json" value={jsonText(selectedTemplate.schema)} rows={5} />
        <JsonField label="去重字段 JSON" name="dedupe_keys" value={jsonText(selectedTemplate.dedupeKeys)} rows={4} />
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          创建
        </button>
      </form>
        </>
      )}

      {sourceView === 'debug' && (
      <div className="inline-actions">
        <label>
          数据源 ID
          <input value={sourceID} onChange={(event) => setSourceID(event.target.value)} />
        </label>
        <button type="button" onClick={() => runSourceAction(`/v1/sources/${sourceID}/test`)} disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          测试
        </button>
        <button type="button" onClick={() => runSourceAction(`/v1/sources/${sourceID}/fetch`)} disabled={loading}>
          <Play aria-hidden="true" />
          拉取
        </button>
      </div>
      )}
    </>
  )
}

function TransformPanel({
  client,
  loading,
  refreshing,
  rules,
  legacyRules,
  onRefresh,
}: {
  client: ApiClient
  loading: boolean
  refreshing: boolean
  rules: TransformRule[]
  legacyRules: LegacyTransformRule[]
  onRefresh: RefreshAction
}) {
  const [rawRecordID, setRawRecordID] = useState('1')
  const [selectedRule, setSelectedRule] = useState<TransformRule | null>(null)
  const [selectedLegacyRule, setSelectedLegacyRule] = useState<LegacyTransformRule | null>(null)

  async function createRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const response = await client('/v1/transform-rules', {
      body: {
        source_id: Number(form.get('source_id')),
        name: form.get('name'),
        rule_type: form.get('rule_type') || 'mapping',
        order_index: Number(form.get('order_index')),
        config_json: form.get('config_json'),
        enabled: true,
      },
    })
    if (response.ok) await onRefresh(false)
  }

  function testRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    client('/v1/transform-rules/test', {
      body: {
        raw_content: JSON.parse(String(form.get('raw_content'))),
        config_json: form.get('config_json'),
      },
    })
  }

  async function retransformRecord() {
    const response = await client(`/v1/raw-records/${rawRecordID}/retransform`)
    if (response.ok) await onRefresh(false)
  }

  async function updateRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedRule) return

    const form = new FormData(event.currentTarget)
    const response = await client(`/v1/transform-rules/${selectedRule.id}`, {
      method: 'PUT',
      body: {
        source_id: Number(form.get('source_id')),
        name: form.get('name'),
        rule_type: form.get('rule_type'),
        order_index: Number(form.get('order_index')),
        config_json: form.get('config_json'),
        enabled: form.get('enabled') === 'on',
      },
    })
    if (response.ok) {
      setSelectedRule(null)
      await onRefresh(false)
    }
  }

  return (
    <>
      <SummaryGrid
        items={[
          { label: '数据库清洗规则', value: rules.length },
          { label: '已启用规则', value: rules.filter((rule) => rule.enabled).length },
          { label: '旧清洗规则', value: legacyRules.length },
        ]}
      />
      <ListHeader title="已有清洗规则" count={rules.length} refreshing={refreshing} onRefresh={onRefresh} />
      <DataTable
        rows={rules}
        emptyText="暂无清洗规则。登录后刷新，或新增映射规则。"
        columns={[
          { label: 'ID', render: (rule) => rule.id },
          { label: '数据源 ID', render: (rule) => rule.source_id },
          { label: '名称', render: (rule) => rule.name },
          { label: '类型', render: (rule) => transformRuleTypeLabel(rule.rule_type) },
          { label: '排序', render: (rule) => rule.order_index },
          { label: '状态', render: (rule) => <StatusBadge active={rule.enabled} /> },
          { label: '创建时间', render: (rule) => formatUnixTime(rule.created_at) },
          {
            label: '操作',
            render: (rule) => (
              <button className="table-action" type="button" onClick={() => setSelectedRule(rule)}>
                详情/编辑
              </button>
            ),
          },
        ]}
      />

      {selectedRule && (
        <DetailPanel title={`清洗规则详情：${selectedRule.name}`} onClose={() => setSelectedRule(null)}>
          <form className="form-grid compact" key={selectedRule.id} onSubmit={updateRule}>
            <Field label="数据源 ID" name="source_id" defaultValue={String(selectedRule.source_id)} />
            <Field label="规则名称" name="name" defaultValue={selectedRule.name} />
            <TransformRuleTypeSelect defaultValue={selectedRule.rule_type} />
            <Field label="排序" name="order_index" defaultValue={String(selectedRule.order_index)} />
            <label className="checkbox-label">
              <input name="enabled" type="checkbox" defaultChecked={selectedRule.enabled} />
              启用
            </label>
            <JsonField label="配置 JSON" name="config_json" value={selectedRule.config_json || '{}'} rows={12} />
            <button className="primary" disabled={loading}>
              保存清洗规则
            </button>
          </form>
        </DetailPanel>
      )}

      <ListHeader title="已迁移旧清洗规则" count={legacyRules.length} refreshing={refreshing} onRefresh={onRefresh} />
      <DataTable
        rows={legacyRules.map((rule) => ({ ...rule, id: rule.code }))}
        emptyText="暂无迁移的旧清洗规则。"
        columns={[
          { label: '名称', render: (rule) => rule.name },
          { label: '来源', render: (rule) => `${rule.source_name || '-'} / ${rule.source_code || '-'}` },
          { label: '类型', render: (rule) => transformRuleTypeLabel(rule.rule_type) },
          { label: '触发', render: (rule) => legacyTriggerModeLabel(rule.trigger_mode) },
          { label: '输入', render: (rule) => rule.input_table || '-' },
          { label: '输出', render: (rule) => rule.output_table || '-' },
          { label: '处理文件', render: (rule) => <span className="muted-text">{rule.handler}</span> },
          {
            label: '操作',
            render: (rule) => (
              <button className="table-action" type="button" onClick={() => setSelectedLegacyRule(rule)}>
                详情/编辑
              </button>
            ),
          },
        ]}
      />

      {selectedLegacyRule && (
        <DetailPanel title={`旧清洗规则详情：${selectedLegacyRule.name}`} onClose={() => setSelectedLegacyRule(null)}>
          <div className="detail-grid">
            <MetaItem label="来源" value={`${selectedLegacyRule.source_name} / ${selectedLegacyRule.source_code}`} />
            <MetaItem label="触发方式" value={legacyTriggerModeLabel(selectedLegacyRule.trigger_mode)} />
            <MetaItem label="输入" value={selectedLegacyRule.input_table} />
            <MetaItem label="输出" value={selectedLegacyRule.output_table} />
            <MetaItem label="处理文件" value={selectedLegacyRule.handler} />
            <MetaItem label="说明" value={selectedLegacyRule.description} />
          </div>
          <div className="step-list">
            {selectedLegacyRule.steps.map((step) => (
              <span key={step}>{step}</span>
            ))}
          </div>
          <JsonField label="规则配置 JSON" name="legacy_rule_config" value={jsonText(selectedLegacyRule.config)} rows={10} />
        </DetailPanel>
      )}

      <div className="panel-heading form-heading">
        <FileJson aria-hidden="true" />
        <h3>新增清洗规则</h3>
      </div>
      <form className="form-grid" onSubmit={createRule}>
        <Field label="数据源 ID" name="source_id" defaultValue="1" />
        <Field label="规则名称" name="name" defaultValue="订单字段映射" />
        <TransformRuleTypeSelect defaultValue="mapping" />
        <Field label="排序" name="order_index" defaultValue="1" />
        <label className="wide">
          映射配置
          <textarea name="config_json" defaultValue={defaultMappingConfig} rows={10} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          保存规则
        </button>
      </form>

      <form className="form-grid separated" onSubmit={testRule}>
        <label className="wide">
          原始样例 JSON
          <textarea
            name="raw_content"
            rows={5}
            defaultValue={'{"params":{"orderNo":"ORDER-1"},"data":{"amount":"12345"}}'}
          />
        </label>
        <label className="wide">
          配置 JSON
          <textarea name="config_json" rows={8} defaultValue={defaultMappingConfig} />
        </label>
        <button disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          测试规则
        </button>
      </form>

      <div className="inline-actions">
        <label>
          原始记录 ID
          <input value={rawRecordID} onChange={(event) => setRawRecordID(event.target.value)} />
        </label>
        <button type="button" onClick={retransformRecord} disabled={loading}>
          <Play aria-hidden="true" />
          重新清洗
        </button>
      </div>
    </>
  )
}

function DeliveryPanel({
  client,
  loading,
  refreshing,
  destinations,
  tasks,
  logs,
  legacyTasks,
  onRefresh,
}: {
  client: ApiClient
  loading: boolean
  refreshing: boolean
  destinations: DestinationDefinition[]
  tasks: DeliveryTask[]
  logs: DeliveryLog[]
  legacyTasks: LegacyTask[]
  onRefresh: RefreshAction
}) {
  const [destinationID, setDestinationID] = useState('1')
  const [taskID, setTaskID] = useState('1')
  const [selectedDestination, setSelectedDestination] = useState<DestinationDefinition | null>(null)
  const [selectedTask, setSelectedTask] = useState<DeliveryTask | null>(null)

  async function createDestination(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const response = await client('/v1/destinations', {
      body: {
        name: form.get('name'),
        code: form.get('code'),
        destination_type: form.get('destination_type'),
        config_json: form.get('config_json'),
        enabled: true,
      },
    })
    if (response.ok) await onRefresh(false)
  }

  async function createTask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const response = await client('/v1/delivery-tasks', {
      body: {
        name: form.get('name'),
        source_id: Number(form.get('source_id')),
        clean_table: form.get('clean_table'),
        destination_id: Number(form.get('destination_id')),
        trigger_type: form.get('trigger_type'),
        cron_expr: form.get('cron_expr'),
        payload_template: form.get('payload_template'),
        enabled: true,
      },
    })
    if (response.ok) await onRefresh(false)
  }

  async function runDeliveryAction(path: string) {
    const response = await client(path)
    if (response.ok) await onRefresh(false)
  }

  async function updateDestination(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedDestination) return

    const form = new FormData(event.currentTarget)
    const response = await client(`/v1/destinations/${selectedDestination.id}`, {
      method: 'PUT',
      body: {
        name: form.get('name'),
        code: form.get('code'),
        destination_type: form.get('destination_type'),
        config_json: form.get('config_json'),
        enabled: form.get('enabled') === 'on',
      },
    })
    if (response.ok) {
      setSelectedDestination(null)
      await onRefresh(false)
    }
  }

  async function updateTask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedTask) return

    const form = new FormData(event.currentTarget)
    const response = await client(`/v1/delivery-tasks/${selectedTask.id}`, {
      method: 'PUT',
      body: {
        name: form.get('name'),
        source_id: Number(form.get('source_id')),
        clean_table: form.get('clean_table'),
        destination_id: Number(form.get('destination_id')),
        trigger_type: form.get('trigger_type'),
        cron_expr: form.get('cron_expr'),
        filter_json: form.get('filter_json'),
        payload_template: form.get('payload_template'),
        enabled: form.get('enabled') === 'on',
      },
    })
    if (response.ok) {
      setSelectedTask(null)
      await onRefresh(false)
    }
  }

  return (
    <>
      <SummaryGrid
        items={[
          { label: '推送目标', value: destinations.length },
          { label: '数据库推送任务', value: tasks.length },
          { label: '迁移自动推送任务', value: legacyTasks.length },
          { label: '近期推送日志', value: logs.length },
        ]}
      />
      <div className="list-stack">
        <ListHeader title="已配置推送目标" count={destinations.length} refreshing={refreshing} onRefresh={onRefresh} />
        <DataTable
          rows={destinations}
          emptyText="暂无推送目标。"
          columns={[
            { label: 'ID', render: (destination) => destination.id },
            { label: '名称', render: (destination) => destination.name },
            { label: '编码', render: (destination) => destination.code },
            { label: '类型', render: (destination) => destinationTypeLabel(destination.destination_type) },
            { label: '状态', render: (destination) => <StatusBadge active={destination.enabled} /> },
            { label: '创建时间', render: (destination) => formatUnixTime(destination.created_at) },
            {
              label: '操作',
              render: (destination) => (
                <button className="table-action" type="button" onClick={() => setSelectedDestination(destination)}>
                  详情/编辑
                </button>
              ),
            },
          ]}
        />

        {selectedDestination && (
          <DetailPanel title={`推送目标详情：${selectedDestination.name}`} onClose={() => setSelectedDestination(null)}>
            <form className="form-grid compact" key={selectedDestination.id} onSubmit={updateDestination}>
              <Field label="目标名称" name="name" defaultValue={selectedDestination.name} />
              <Field label="目标编码" name="code" defaultValue={selectedDestination.code} />
              <DestinationTypeSelect defaultValue={selectedDestination.destination_type} />
              <label className="checkbox-label">
                <input name="enabled" type="checkbox" defaultChecked={selectedDestination.enabled} />
                启用
              </label>
              <JsonField label="配置 JSON" name="config_json" value={selectedDestination.config_json || '{}'} rows={8} />
              <button className="primary" disabled={loading}>
                保存推送目标
              </button>
            </form>
          </DetailPanel>
        )}

        <ListHeader title="已配置推送任务" count={tasks.length} refreshing={refreshing} onRefresh={onRefresh} />
        <DataTable
          rows={tasks}
          emptyText="暂无推送任务。"
          columns={[
            { label: 'ID', render: (task) => task.id },
            { label: '任务名称', render: (task) => task.name },
            { label: '数据源', render: (task) => task.source_id },
            { label: '清洗表', render: (task) => task.clean_table },
            { label: '目标', render: (task) => task.destination_id },
            { label: '触发', render: (task) => triggerTypeLabel(task.trigger_type) },
            { label: '调度表达式', render: (task) => task.cron_expr || '-' },
            { label: '状态', render: (task) => <StatusBadge active={task.enabled} /> },
            {
              label: '操作',
              render: (task) => (
                <button className="table-action" type="button" onClick={() => setSelectedTask(task)}>
                  详情/编辑
                </button>
              ),
            },
          ]}
        />

        {selectedTask && (
          <DetailPanel title={`推送任务详情：${selectedTask.name}`} onClose={() => setSelectedTask(null)}>
            <form className="form-grid compact" key={selectedTask.id} onSubmit={updateTask}>
              <Field label="任务名称" name="name" defaultValue={selectedTask.name} />
              <Field label="数据源 ID" name="source_id" defaultValue={String(selectedTask.source_id)} />
              <Field label="清洗表" name="clean_table" defaultValue={selectedTask.clean_table} />
              <Field label="目标 ID" name="destination_id" defaultValue={String(selectedTask.destination_id)} />
              <TriggerTypeSelect defaultValue={selectedTask.trigger_type} />
              <Field label="调度表达式" name="cron_expr" defaultValue={selectedTask.cron_expr || ''} />
              <label className="checkbox-label">
                <input name="enabled" type="checkbox" defaultChecked={selectedTask.enabled} />
                启用
              </label>
              <JsonField label="过滤条件 JSON" name="filter_json" value={selectedTask.filter_json || '{}'} rows={5} />
              <label className="wide">
                报文模板
                <textarea name="payload_template" defaultValue={selectedTask.payload_template || ''} rows={8} />
              </label>
              <button className="primary" disabled={loading}>
                保存推送任务
              </button>
            </form>
          </DetailPanel>
        )}

        <ListHeader title="已迁移自动推送任务" count={legacyTasks.length} refreshing={refreshing} onRefresh={onRefresh} />
        <LegacyTaskTable
          client={client}
          loading={loading}
          tasks={legacyTasks}
          emptyText="暂无迁移的自动推送任务。"
          onRefresh={onRefresh}
        />

        <ListHeader title="近期推送日志" count={logs.length} refreshing={refreshing} onRefresh={onRefresh} />
        <DataTable
          rows={logs}
          emptyText="暂无推送日志。执行推送任务后会显示最近 50 条。"
          columns={[
            { label: 'ID', render: (log) => log.id },
            { label: '追踪号', render: (log) => shortText(log.trace_id) },
            { label: '运行ID', render: (log) => log.run_id || '-' },
            { label: '业务键', render: (log) => log.business_key || '-' },
            { label: 'HTTP 状态', render: (log) => log.http_status || '-' },
            { label: '结果', render: (log) => <StatusBadge active={log.success} activeText="成功" inactiveText="失败" /> },
            { label: '发送时间', render: (log) => log.sent_at || '-' },
          ]}
        />
      </div>

      <div className="panel-heading form-heading">
        <Send aria-hidden="true" />
        <h3>新增推送配置</h3>
      </div>
      <form className="form-grid" onSubmit={createDestination}>
        <Field label="目标名称" name="name" defaultValue="HTTP 推送目标" />
        <Field label="目标编码" name="code" defaultValue="http_sink" />
        <DestinationTypeSelect defaultValue="http" />
        <label className="wide">
          配置 JSON
          <textarea name="config_json" rows={6} defaultValue={'{"url":"http://localhost:9000/orders","method":"POST"}'} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          创建目标
        </button>
      </form>

      <form className="form-grid separated" onSubmit={createTask}>
        <Field label="任务名称" name="name" defaultValue="推送清洗订单" />
        <Field label="数据源 ID" name="source_id" defaultValue="1" />
        <Field label="清洗表" name="clean_table" defaultValue="clean_orders" />
        <Field label="目标 ID" name="destination_id" defaultValue="1" />
        <TriggerTypeSelect defaultValue="manual" />
        <Field label="调度表达式" name="cron_expr" defaultValue="@every 5m" />
        <label className="wide">
          报文模板
          <textarea name="payload_template" rows={5} defaultValue={'{"order_no":"{{order_no}}","amount":"{{actual_amount}}"}'} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          创建任务
        </button>
      </form>

      <div className="inline-actions">
        <label>
          目标 ID
          <input value={destinationID} onChange={(event) => setDestinationID(event.target.value)} />
        </label>
        <button type="button" onClick={() => runDeliveryAction(`/v1/destinations/${destinationID}/test`)} disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          测试
        </button>
        <label>
          任务 ID
          <input value={taskID} onChange={(event) => setTaskID(event.target.value)} />
        </label>
        <button type="button" onClick={() => runDeliveryAction(`/v1/delivery-tasks/${taskID}/run`)} disabled={loading}>
          <Play aria-hidden="true" />
          执行
        </button>
      </div>
    </>
  )
}

function ExcelMatchPanel({
  token,
  loading,
  setLoading,
  setResult,
}: {
  token: string
  loading: boolean
  setLoading: (value: boolean) => void
  setResult: (value: ApiResult | null) => void
}) {
  const [jobID, setJobID] = useState('')
  const [job, setJob] = useState<ExcelMatchJob | null>(null)
  const [selectedFileName, setSelectedFileName] = useState('')

  async function createExcelMatchJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const file = form.get('file')
    if (!(file instanceof File) || file.size === 0) {
      setResult({ ok: false, status: 0, data: '请选择 .xlsx 文件' })
      return
    }

    const config = {
      filters: [
        {
          column: String(form.get('filterColumn') ?? '').trim(),
          op: 'eq',
          value: String(form.get('filterValue') ?? '').trim(),
        },
      ],
      matchExcelColumn: String(form.get('matchExcelColumn') ?? '').trim(),
      dbTemplate: 'bojun_retail_order',
      dbMatchField: 'docno',
      dbValueField: String(form.get('dbValueField') ?? '').trim(),
      outputColumnName: String(form.get('outputColumnName') ?? '').trim(),
      batchSize: Number(form.get('batchSize') || 1000),
    }

    const payload = new FormData()
    payload.append('file', file)
    payload.append('config', JSON.stringify(config))

    setLoading(true)
    try {
      const response = await fetch('/api/v1/excel-match-jobs', {
        method: 'POST',
        headers: token ? { token } : undefined,
        body: payload,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (nextResult.ok) {
        const nextJob = readObject<ExcelMatchJob>(nextResult, 'job')
        if (nextJob) {
          setJob(nextJob)
          setJobID(String(nextJob.id))
        }
      }
    } catch (error) {
      setResult({
        ok: false,
        status: 0,
        data: error instanceof Error ? error.message : String(error),
      })
    } finally {
      setLoading(false)
    }
  }

  async function refreshJob() {
    const id = Number(jobID)
    if (!id) {
      setResult({ ok: false, status: 0, data: '请输入任务 ID' })
      return
    }

    setLoading(true)
    try {
      const response = await fetch(`/api/v1/excel-match-jobs/${id}`, {
        method: 'GET',
        headers: token ? { token } : undefined,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (nextResult.ok) {
        const nextJob = readObject<ExcelMatchJob>(nextResult, 'job')
        if (nextJob) setJob(nextJob)
      }
    } catch (error) {
      setResult({
        ok: false,
        status: 0,
        data: error instanceof Error ? error.message : String(error),
      })
    } finally {
      setLoading(false)
    }
  }

  async function downloadJob() {
    const id = Number(jobID || job?.id)
    if (!id) {
      setResult({ ok: false, status: 0, data: '请输入任务 ID' })
      return
    }

    setLoading(true)
    try {
      const response = await fetch(`/api/v1/excel-match-jobs/${id}/download`, {
        method: 'GET',
        headers: token ? { token } : undefined,
      })
      if (!response.ok) {
        const data = await response.json().catch(() => ({}))
        setResult({ ok: false, status: response.status, data })
        return
      }

      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = readDownloadFilename(response.headers.get('Content-Disposition')) ?? `excel_match_job_${id}.xlsx`
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
      setResult({ ok: true, status: response.status, data: `任务 ${id} 结果文件已开始下载` })
    } catch (error) {
      setResult({
        ok: false,
        status: 0,
        data: error instanceof Error ? error.message : String(error),
      })
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <SummaryGrid
        items={[
          { label: '当前任务', value: job?.id ?? '-' },
          { label: '任务状态', value: job ? excelJobStatusLabel(job.status) : '-' },
          { label: '已处理行', value: job ? `${job.processed_rows}/${job.total_rows}` : '-' },
          { label: '匹配结果', value: job ? `${job.matched_rows} 匹配 / ${job.unmatched_rows} 未匹配` : '-' },
        ]}
      />

      <div className="template-note">
        <h3>百万行 Excel 匹配导出</h3>
        <p>文件只上传到系统临时目录，后端成功处理后删除源文件。结果必须通过鉴权下载接口获取，默认 24 小时后清理。</p>
        <div className="template-points">
          <MetaItem label="前置筛选" value="例如 店铺名称 = 杭州恒隆" />
          <MetaItem label="数据库匹配" value="bojun_retail_orders.docno" />
          <MetaItem label="导出策略" value="保留全量原行，追加新列" />
          <MetaItem label="查询方式" value="筛选命中行批量 IN 查询" />
        </div>
      </div>

      <form className="form-grid excel-upload-form" onSubmit={createExcelMatchJob}>
        <label className="wide file-input-label">
          Excel 文件
          <input
            name="file"
            type="file"
            accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            onChange={(event) => setSelectedFileName(event.currentTarget.files?.[0]?.name ?? '')}
          />
          <span>{selectedFileName || '请选择需要匹配的 .xlsx 文件'}</span>
        </label>
        <Field label="筛选列名" name="filterColumn" defaultValue="店铺名称" />
        <Field label="筛选值" name="filterValue" defaultValue="杭州恒隆" />
        <Field label="Excel 订单号列" name="matchExcelColumn" defaultValue="订单号" />
        <label>
          伯俊写回字段
          <select name="dbValueField" defaultValue="tot_amt_actual">
            <option value="tot_amt_actual">实付金额 tot_amt_actual</option>
            <option value="tot_amt_list">吊牌金额 tot_amt_list</option>
            <option value="tot_qty">数量 tot_qty</option>
            <option value="c_store_code">门店编码 c_store_code</option>
            <option value="c_store_name">门店名称 c_store_name</option>
            <option value="order_type_name">单据类型 order_type_name</option>
            <option value="retailbilltype">零售单类型 retailbilltype</option>
            <option value="related_normal_docno">关联原单 related_normal_docno</option>
          </select>
        </label>
        <Field label="追加列名" name="outputColumnName" defaultValue="伯俊匹配金额" />
        <Field label="批量查询大小" name="batchSize" defaultValue="1000" />
        <button className="primary" disabled={loading}>
          <Upload aria-hidden="true" />
          创建匹配任务
        </button>
      </form>

      <div className="inline-actions">
        <label>
          任务 ID
          <input value={jobID} onChange={(event) => setJobID(event.target.value)} />
        </label>
        <button type="button" onClick={refreshJob} disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          查询状态
        </button>
        <button type="button" onClick={downloadJob} disabled={loading || job?.status !== 'success'}>
          <Download aria-hidden="true" />
          下载结果
        </button>
      </div>

      {job && (
        <DetailPanel title={`Excel 匹配任务：${job.id}`} onClose={() => setJob(null)}>
          <div className="detail-grid">
            <MetaItem label="源文件" value={job.source_file_name || '-'} />
            <MetaItem label="状态" value={excelJobStatusLabel(job.status)} />
            <MetaItem label="总行数" value={job.total_rows || '-'} />
            <MetaItem label="已处理" value={job.processed_rows || '-'} />
            <MetaItem label="筛选命中" value={job.filtered_rows || '-'} />
            <MetaItem label="匹配成功" value={job.matched_rows || '-'} />
            <MetaItem label="未匹配" value={job.unmatched_rows || '-'} />
            <MetaItem label="结果过期" value={job.expires_at || '-'} />
            <MetaItem label="开始时间" value={job.started_at || '-'} />
            <MetaItem label="结束时间" value={job.finished_at || '-'} />
            <MetaItem label="错误信息" value={job.error_message || '-'} />
          </div>
          <JsonField label="任务配置 JSON" name="excel_match_config" value={job.config_json || '{}'} rows={10} />
        </DetailPanel>
      )}
    </>
  )
}

function LegacyTaskTable({
  client,
  loading,
  tasks,
  emptyText,
  onRefresh,
}: {
  client: ApiClient
  loading: boolean
  tasks: LegacyTask[]
  emptyText: string
  onRefresh: RefreshAction
}) {
  const [selectedTask, setSelectedTask] = useState<LegacyTask | null>(null)
  const [payloadText, setPayloadText] = useState('{}')

  function openLegacyTask(task: LegacyTask) {
    setSelectedTask(task)
    setPayloadText(jsonText(task.default_payload ?? {}))
  }

  async function runLegacyTask(task: LegacyTask, payload = task.default_payload) {
    const response = await client(`/v1/legacy-tasks/${task.code}/run`, {
      body: payload ?? {},
    })
    if (response.ok) await onRefresh(false)
  }

  return (
    <>
      <DataTable
        rows={tasks.map((task) => ({ ...task, id: task.code }))}
        emptyText={emptyText}
        columns={[
          { label: '名称', render: (task) => task.name },
          { label: '来源', render: (task) => `${task.source_name || '-'} / ${task.source_code || '-'}` },
          { label: '分类', render: (task) => legacyCategoryLabel(task.category) },
          { label: '队列', render: (task) => task.queue || '-' },
          { label: '调度', render: (task) => task.cron_expr || '手动' },
          { label: '输入', render: (task) => task.input_table || '-' },
          { label: '输出', render: (task) => task.output_table || '-' },
          { label: '目标', render: (task) => task.target_system || '-' },
          { label: '处理文件', render: (task) => <span className="muted-text">{task.handler || '-'}</span> },
          {
            label: '操作',
            render: (task) => (
              <div className="row-actions">
                <button className="table-action" type="button" onClick={() => openLegacyTask(task)}>
                  详情/编辑
                </button>
                <button
                  className="table-action"
                  type="button"
                  disabled={loading || !canRunLegacyTask(task)}
                  title={canRunLegacyTask(task) ? '立即投递任务' : '该任务需要 raw_data_id 后才能手动执行'}
                  onClick={() => runLegacyTask(task)}
                >
                  <Play aria-hidden="true" />
                  执行
                </button>
              </div>
            ),
          },
        ]}
      />

      {selectedTask && (
        <DetailPanel title={`迁移任务详情：${selectedTask.name}`} onClose={() => setSelectedTask(null)}>
          <div className="detail-grid">
            <MetaItem label="来源" value={`${selectedTask.source_name} / ${selectedTask.source_code}`} />
            <MetaItem label="任务类型" value={selectedTask.task_type} />
            <MetaItem label="队列" value={selectedTask.queue} />
            <MetaItem label="调度" value={selectedTask.cron_expr || '手动'} />
            <MetaItem label="输入" value={selectedTask.input_table} />
            <MetaItem label="输出" value={selectedTask.output_table} />
            <MetaItem label="目标系统" value={selectedTask.target_system || '-'} />
            <MetaItem label="处理文件" value={selectedTask.handler || '-'} />
            <MetaItem label="说明" value={selectedTask.description || '-'} />
          </div>
          <label className="wide">
            运行参数 JSON
            <textarea value={payloadText} onChange={(event) => setPayloadText(event.target.value)} rows={8} />
          </label>
          <div className="inline-actions compact-actions">
            <button
              className="primary"
              type="button"
              disabled={loading}
              onClick={() => runLegacyTask(selectedTask, parseJSONPayload(payloadText))}
            >
              <Play aria-hidden="true" />
              使用当前参数执行
            </button>
          </div>
        </DetailPanel>
      )}
    </>
  )
}

function RunsPanel({
  refreshing,
  runs,
  onRefresh,
}: {
  refreshing: boolean
  runs: PipelineRun[]
  onRefresh: RefreshAction
}) {
  return (
    <>
      <SummaryGrid
        items={[
          { label: '近期运行', value: runs.length },
          { label: '成功', value: runs.filter((run) => run.status === 'success').length },
          { label: '失败', value: runs.filter((run) => run.status === 'failed').length },
          { label: '运行中', value: runs.filter((run) => run.status === 'running').length },
        ]}
      />
      <ListHeader title="近期运行记录" count={runs.length} refreshing={refreshing} onRefresh={onRefresh} />
      <DataTable
        rows={runs}
        emptyText="暂无运行记录。拉取、清洗或推送后会显示最近 50 条。"
        columns={[
          { label: 'ID', render: (run) => run.id },
          { label: '追踪号', render: (run) => shortText(run.trace_id) },
          { label: '类型', render: (run) => runTypeLabel(run.run_type) },
          { label: '触发', render: (run) => triggerTypeLabel(run.trigger_type) },
          { label: '数据源', render: (run) => run.source_id || '-' },
          { label: '目标', render: (run) => run.destination_id || '-' },
          { label: '状态', render: (run) => runStatusLabel(run.status) },
          { label: '数量', render: (run) => `${run.success_count}/${run.total_count}` },
          { label: '开始时间', render: (run) => run.started_at || '-' },
          { label: '结束时间', render: (run) => run.finished_at || '-' },
        ]}
      />
    </>
  )
}

function ListHeader({
  title,
  count,
  refreshing,
  onRefresh,
}: {
  title: string
  count: number
  refreshing: boolean
  onRefresh: RefreshAction
}) {
  return (
    <div className="list-header">
      <div>
        <h3>{title}</h3>
        <span>{count} 条记录</span>
      </div>
      <button type="button" onClick={() => onRefresh(true)} disabled={refreshing}>
        <RefreshCcw aria-hidden="true" />
        刷新
      </button>
    </div>
  )
}

function SectionTabs<T extends string>({
  tabs,
  active,
  onChange,
}: {
  tabs: Array<{ key: T; label: string; count?: number }>
  active: T
  onChange: (key: T) => void
}) {
  return (
    <div className="section-tabs">
      {tabs.map((tab) => (
        <button
          className={`section-tab ${active === tab.key ? 'active' : ''}`}
          key={tab.key}
          type="button"
          onClick={() => onChange(tab.key)}
        >
          <span>{tab.label}</span>
          {typeof tab.count === 'number' && <strong>{tab.count}</strong>}
        </button>
      ))}
    </div>
  )
}

function SummaryGrid({ items }: { items: Array<{ label: string; value: ReactNode }> }) {
  return (
    <div className="summary-grid">
      {items.map((item) => (
        <div className="summary-tile" key={item.label}>
          <span>{item.label}</span>
          <strong>{item.value}</strong>
        </div>
      ))}
    </div>
  )
}

function SourceTypeSelect({ defaultValue }: { defaultValue: string }) {
  return (
    <label>
      数据源类型
      <select name="source_type" defaultValue={defaultValue}>
        <option value="webhook">Webhook 接收</option>
        <option value="api_poll">接口轮询拉取</option>
        <option value="database">数据库读取</option>
      </select>
    </label>
  )
}

function AuthTypeSelect({ defaultValue }: { defaultValue: string }) {
  return (
    <label>
      认证方式
      <select name="auth_type" defaultValue={defaultValue}>
        <option value="none">无需认证</option>
        <option value="token">Token 认证</option>
        <option value="basic">账号密码认证</option>
        <option value="signature">签名认证</option>
      </select>
    </label>
  )
}

function TransformRuleTypeSelect({ defaultValue }: { defaultValue: string }) {
  return (
    <label>
      清洗规则类型
      <select name="rule_type" defaultValue={defaultValue}>
        <option value="mapping">字段映射</option>
        <option value="http_enrich">接口补数</option>
        <option value="db_enrich">数据库补数</option>
        <option value="script">脚本转换</option>
        <option value="validator">数据校验</option>
      </select>
    </label>
  )
}

function DestinationTypeSelect({ defaultValue }: { defaultValue: string }) {
  return (
    <label>
      推送目标类型
      <select name="destination_type" defaultValue={defaultValue}>
        <option value="http">HTTP 接口</option>
        <option value="soap">SOAP 接口</option>
      </select>
    </label>
  )
}

function TriggerTypeSelect({ defaultValue }: { defaultValue: string }) {
  return (
    <label>
      触发方式
      <select name="trigger_type" defaultValue={defaultValue}>
        <option value="manual">手动执行</option>
        <option value="schedule">定时调度</option>
        <option value="event">事件触发</option>
      </select>
    </label>
  )
}

function DetailPanel({
  title,
  children,
  onClose,
}: {
  title: string
  children: ReactNode
  onClose: () => void
}) {
  return (
    <section className="detail-panel">
      <div className="detail-header">
        <h3>{title}</h3>
        <button type="button" onClick={onClose}>
          关闭
        </button>
      </div>
      {children}
    </section>
  )
}

function MetaItem({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="meta-item">
      <span>{label}</span>
      <strong>{value || '-'}</strong>
    </div>
  )
}

function JsonField({ label, name, value, rows }: { label: string; name: string; value: string; rows: number }) {
  return (
    <label className="wide">
      {label}
      <textarea name={name} defaultValue={formatJSONText(value)} rows={rows} />
    </label>
  )
}

function DataTable<T extends { id: number | string }>({
  rows,
  columns,
  emptyText,
}: {
  rows: T[]
  columns: Column<T>[]
  emptyText: string
}) {
  if (rows.length === 0) {
    return <div className="empty-state">{emptyText}</div>
  }

  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column.label}>{column.label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.id}>
              {columns.map((column) => (
                <td key={column.label}>{column.render(row)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function StatusBadge({
  active,
  activeText = '启用',
  inactiveText = '停用',
}: {
  active: boolean
  activeText?: string
  inactiveText?: string
}) {
  return <span className={active ? 'status-badge active' : 'status-badge inactive'}>{active ? activeText : inactiveText}</span>
}

function Field({ label, name, defaultValue }: { label: string; name: string; defaultValue: string }) {
  return (
    <label>
      {label}
      <input name={name} defaultValue={defaultValue} />
    </label>
  )
}

function NavButton({
  active,
  icon,
  label,
  onClick,
}: {
  active: boolean
  icon: ReactNode
  label: string
  onClick: () => void
}) {
  return (
    <button className={active ? 'nav-button active' : 'nav-button'} onClick={onClick} type="button">
      {icon}
      <span>{label}</span>
    </button>
  )
}

function readList<T>(result: ApiResult, key: string): T[] {
  const envelope = result.data as { data?: Record<string, unknown> }
  const value = envelope.data?.[key]
  return Array.isArray(value) ? (value as T[]) : []
}

function readObject<T>(result: ApiResult, key: string): T | null {
  const envelope = result.data as { data?: Record<string, unknown> }
  const value = envelope.data?.[key]
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as T) : null
}

function readToken(data: unknown) {
  const envelope = data as { data?: Record<string, unknown> }
  const token = envelope.data?.token
  return typeof token === 'string' ? token : ''
}

function readMessage(data: unknown) {
  const envelope = data as { msg?: unknown }
  return typeof envelope.msg === 'string' ? envelope.msg : ''
}

function isSuccessPayload(data: unknown) {
  const envelope = data as { code?: unknown }
  return envelope.code === 0 || envelope.code === 200
}

function clearLists(
  setSources: (value: SourceDefinition[]) => void,
  setRules: (value: TransformRule[]) => void,
  setDestinations: (value: DestinationDefinition[]) => void,
  setTasks: (value: DeliveryTask[]) => void,
  setLogs: (value: DeliveryLog[]) => void,
  setRuns: (value: PipelineRun[]) => void,
  setLegacyTasks: (value: LegacyTask[]) => void,
  setLegacyTransformRules: (value: LegacyTransformRule[]) => void,
) {
  setSources([])
  setRules([])
  setDestinations([])
  setTasks([])
  setLogs([])
  setRuns([])
  setLegacyTasks([])
  setLegacyTransformRules([])
}

function formatUnixTime(value: number) {
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString('zh-CN', { hour12: false })
}

function shortText(value: string) {
  if (!value) return '-'
  return value.length > 14 ? `${value.slice(0, 14)}...` : value
}

function sourceTypeLabel(value: string) {
  return labelFromMap(
    value,
    {
      webhook: 'Webhook 接收',
      api_poll: '接口轮询拉取',
      api: '接口轮询拉取',
      database: '数据库读取',
    },
    '未知数据源类型',
  )
}

function destinationTypeLabel(value: string) {
  return labelFromMap(
    value,
    {
      http: 'HTTP 接口',
      soap: 'SOAP 接口',
    },
    '未知推送目标',
  )
}

function transformRuleTypeLabel(value: string) {
  return labelFromMap(
    value,
    {
      mapping: '字段映射',
      http_enrich: '接口补数',
      db_enrich: '数据库补数',
      script: '脚本转换',
      validator: '数据校验',
    },
    '未知清洗类型',
  )
}

function triggerTypeLabel(value: string) {
  return labelFromMap(
    value,
    {
      manual: '手动执行',
      schedule: '定时调度',
      event: '事件触发',
      api: '接口触发',
    },
    '未知触发方式',
  )
}

function legacyTriggerModeLabel(value: string) {
  return labelFromMap(
    value,
    {
      'data:process': '数据处理任务触发',
      'youzan:sync': '有赞订单拉取任务触发',
      'youzan:return': '有赞退款拉取任务触发',
    },
    '旧任务触发',
  )
}

function runTypeLabel(value: string) {
  return labelFromMap(
    value,
    {
      fetch: '数据拉取',
      ingest: '数据接收',
      transform: '数据清洗',
      delivery: '数据推送',
    },
    '未知运行类型',
  )
}

function runStatusLabel(value: string) {
  return labelFromMap(
    value,
    {
      running: '运行中',
      success: '成功',
      failed: '失败',
      partial_success: '部分成功',
    },
    '未知状态',
  )
}

function excelJobStatusLabel(value: string) {
  return labelFromMap(
    value,
    {
      pending: '等待处理',
      running: '处理中',
      success: '成功',
      failed: '失败',
      expired: '已过期',
    },
    '未知状态',
  )
}

function labelFromMap(value: string, labels: Record<string, string>, fallback: string) {
  if (!value) return '-'
  return labels[value] ? `${labels[value]}（${value}）` : `${fallback}（${value}）`
}

function jsonText(value: unknown) {
  return JSON.stringify(value ?? {}, null, 2)
}

function formatJSONText(value: string) {
  if (!value) return '{}'
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

function parseJSONPayload(value: string) {
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? (parsed as Record<string, unknown>) : {}
  } catch {
    return {}
  }
}

function canRunLegacyTask(task: LegacyTask) {
  if (task.code !== 'qimai_order_enrich') return true
  const rawDataID = task.default_payload?.raw_data_id
  return typeof rawDataID === 'number' && rawDataID > 0
}

function legacyCategoryLabel(category: LegacyTask['category']) {
  switch (category) {
    case 'fetch':
      return '数据拉取'
    case 'delivery':
      return '数据推送'
    case 'process':
      return '补数处理'
  }
}

function readDownloadFilename(contentDisposition: string | null) {
  if (!contentDisposition) return null
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) return decodeURIComponent(utf8Match[1])
  const asciiMatch = contentDisposition.match(/filename="?([^";]+)"?/i)
  return asciiMatch?.[1] ?? null
}

function sectionTitle(key: NavKey) {
  switch (key) {
    case 'sources':
      return '数据源'
    case 'transform':
      return '清洗流水线'
    case 'delivery':
      return '推送流水线'
    case 'excel':
      return 'Excel 匹配导出'
    case 'runs':
      return '运行记录'
  }
}

export default App
