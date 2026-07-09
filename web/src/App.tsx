import { FormEvent, ReactNode, useCallback, useEffect, useMemo, useState } from 'react'
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  BookOpen,
  CheckCircle2,
  Database,
  Download,
  FileJson,
  Inbox,
  ListChecks,
  LogOut,
  RefreshCcw,
  ScrollText,
  Send,
  Upload,
  Wrench,
} from 'lucide-react'
import './App.css'

const defaultApiBaseURL = import.meta.env.VITE_API_BASE_URL ?? 'https://shop-test.youlankids.com'

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
type NavKey = 'push_status' | 'methods' | 'receive' | 'pull' | 'process' | 'push' | 'excel' | 'logs'
type MethodKind = 'configured' | 'builtin'
type MethodType = 'request' | 'bojun_signed_request' | 'extract' | 'mapping' | 'validate' | 'db_query' | 'db_write' | 'template' | 'delivery' | 'shanghai_mall_push' | 'log' | 'utility'
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

type ExcelMatchJobLog = {
  id: number
  job_id: number
  level: string
  message: string
  detail_json: string
  created_at: number
}

type ExcelDialogMode = 'export' | 'import' | 'clear' | 'query'
type ExcelUploadSlot = 'export' | 'import' | 'clear'

type ExcelUploadSession = {
  uploadId: string
  fileName: string
  totalChunks: number
  uploadedChunks: number
  complete: boolean
  expiresAt: string
}

type ExcelUploadRef = {
  uploadId: string
  fileName: string
  size: number
  lastModified: number
  totalChunks: number
}

type ExcelExportSchemeConfig = {
  sheetName: string
  filterColumn: string
  filterValue: string
  matchExcelColumn: string
  dbMatchField: string
  dbValueField: string
  outputColumnName: string
  batchSize: string
}

type ExcelImportSchemeConfig = {
  sheetName: string
  tableName: string
  dbMatchField: string
  matchExcelColumn: string
  dbWriteField: string
  writeExcelColumn: string
  batchSize: string
}

type ExcelMatchSchemeConfig = {
  operation?: string
  sheetName?: string
  filters?: Array<{ column?: string; op?: string; value?: string }>
  matchExcelColumn?: string
  dbTemplate?: string
  dbMatchField?: string
  dbValueField?: string
  tableName?: string
  dbWriteField?: string
  writeExcelColumn?: string
  outputColumnName?: string
  batchSize?: number
  dryRun?: boolean
  confirmWrite?: boolean
}

type ExcelMatchScheme = {
  id: number
  name: string
  operation: string
  config: ExcelMatchSchemeConfig
  config_json: string
  created_at: number
  updated_at: number
}

type ExcelMatchPreviewStats = {
  TotalRows?: number
  ProcessedRows?: number
  FilteredRows?: number
  MatchedRows?: number
  UnmatchedRows?: number
}

type ExcelMatchPreviewSample = {
  rowNumber: number
  matchKey: string
  matchedValue: string
  status: string
  reason: string
  values: Record<string, string>
}

type ExcelMatchPreviewResult = {
  stats: ExcelMatchPreviewStats
  scanLimit: number
  sampleLimit: number
  truncated: boolean
  samples: ExcelMatchPreviewSample[]
}

const bojunMatchFieldOptions = [
  { value: 'docno', label: '订单号 docno' },
  { value: 'otherdocno', label: '外部单号 otherdocno' },
  { value: 'o2o_so_docno', label: '线上订单号 o2o_so_docno' },
  { value: 'related_normal_docno', label: '关联原单 related_normal_docno' },
  { value: 'matched_docno', label: '匹配单号 matched_docno' },
]

const bojunValueFieldOptions = [
  { value: 'docno', label: '订单号 docno' },
  { value: 'otherdocno', label: '外部单号 otherdocno' },
  { value: 'tot_amt_actual', label: '实付金额 tot_amt_actual' },
  { value: 'tot_amt_list', label: '吊牌金额 tot_amt_list' },
  { value: 'tot_qty', label: '数量 tot_qty' },
  { value: 'c_store_code', label: '门店编码 c_store_code' },
  { value: 'c_store_name', label: '门店名称 c_store_name' },
  { value: 'order_type_name', label: '单据类型 order_type_name' },
  { value: 'order_type_code', label: '单据类型编码 order_type_code' },
  { value: 'retailbilltype', label: '零售单类型 retailbilltype' },
  { value: 'billdate', label: '单据日期 billdate' },
  { value: 'vipno', label: '会员号 vipno' },
  { value: 'related_normal_docno', label: '关联原单 related_normal_docno' },
  { value: 'o2o_so_docno', label: '线上订单号 o2o_so_docno' },
  { value: 'matched_docno', label: '匹配单号 matched_docno' },
]

const excelChunkSize = 4 * 1024 * 1024

const defaultExcelExportScheme: ExcelExportSchemeConfig = {
  sheetName: 'Sheet1',
  filterColumn: '店铺',
  filterValue: '幼岚-有赞',
  matchExcelColumn: '原始线上订单号',
  dbMatchField: 'matched_docno',
  dbValueField: 'c_store_name',
  outputColumnName: '线下店名称',
  batchSize: '1000',
}

const defaultExcelImportScheme: ExcelImportSchemeConfig = {
  sheetName: 'Sheet1',
  tableName: 'bojun_retail_orders',
  dbMatchField: 'docno',
  matchExcelColumn: '外部订单编号',
  dbWriteField: 'matched_docno',
  writeExcelColumn: '订单号',
  batchSize: '1000',
}

type BojunOrderBackfillSample = {
  docno: string
  otherdocno: string
  c_store_code: string
  c_store_name: string
  order_type_code: string
  order_type_name: string
  billdate: number
  tot_qty: number
  tot_amt_actual: number
  status: string
  reason: string
}

type BojunOrderBackfillResult = {
  start_time: string
  end_time: string
  page_size: number
  max_pages: number
  fetch_pages: number
  total_count: number
  preview_count: number
  writable_count: number
  existing_count: number
  saved_count: number
  retail_count: number
  skipped_count: number
  failed_count: number
  samples: BojunOrderBackfillSample[]
  failed_samples: BojunOrderBackfillSample[]
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
  run_id: number
  source_code: string
  destination_code: string
  destination_name: string
  destination_id: number
  clean_record_id: number
  business_key: string
  request_body: string
  response_body: string
  http_status: number
  success: boolean
  error_message: string
  retry_count: number
  sent_at: string | null
}

type LogDetail =
  | { type: 'run'; title: string; value: unknown }
  | { type: 'delivery'; title: string; value: unknown }

type LogSelection = {
  type: 'run' | 'delivery'
  id: number
}

type DeliveryStore = {
  key: string
  name: string
  aliases: string[]
}

type DeliveryLogWithStore = {
  log: DeliveryLog
  store: DeliveryStore
}

const tokenStorageKey = 'warehouse-token'

const navItems: Array<{ key: NavKey; label: string; icon: ReactNode }> = [
  { key: 'push_status', label: '推送情况', icon: <Activity aria-hidden="true" /> },
  { key: 'methods', label: '方法', icon: <Wrench aria-hidden="true" /> },
  { key: 'receive', label: '数据接收', icon: <Inbox aria-hidden="true" /> },
  { key: 'pull', label: '数据拉取', icon: <ArrowDownToLine aria-hidden="true" /> },
  { key: 'process', label: '数据处理', icon: <ListChecks aria-hidden="true" /> },
  { key: 'push', label: '数据推送', icon: <ArrowUpFromLine aria-hidden="true" /> },
  { key: 'excel', label: 'Excel 匹配', icon: <Upload aria-hidden="true" /> },
  { key: 'logs', label: '日志', icon: <ScrollText aria-hidden="true" /> },
]

const deliveryStores: DeliveryStore[] = [
  { key: 'shangsheng', name: '上生新所', aliases: ['ABCN001A001', 'shangsheng', '上生新所', '上升新所'] },
  { key: 'jialicheng', name: '嘉里城', aliases: ['ABCN001A004', 'jialicheng', 'kerry', '嘉里城'] },
  { key: 'panlong', name: '蟠龙', aliases: ['ABCN001A005', 'panlong', '蟠龙'] },
  { key: 'xintiandi', name: '新天地', aliases: ['ABCN001A003', 'xintiandi', '新天地'] },
  { key: 'qiantan', name: '前滩', aliases: ['ABCN001P012', 'qiantan', '前滩'] },
  { key: 'hangzhou_henglong', name: '杭州恒隆', aliases: ['ABCN002A001', 'hangzhou_henglong', 'henglong', '杭州恒隆'] },
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
  ...[
    ['builtin-push-jialicheng', '推送嘉里城', 'push_jialicheng', 'jialicheng'],
    ['builtin-push-panlong', '推送蟠龙', 'push_panlong', 'panlong'],
    ['builtin-push-qiantan', '推送前滩', 'push_qiantan', 'qiantan'],
    ['builtin-push-shangsheng', '推送上生新所', 'push_shangsheng', 'shangsheng'],
    ['builtin-push-xintiandi', '推送新天地', 'push_xintiandi', 'xintiandi'],
  ].map(([key, name, code, target]) => ({
    key,
    kind: 'builtin' as const,
    name,
    code,
    method_type: 'shanghai_mall_push' as const,
    category: '商场推送方法',
    owner: '系统内置',
    description: `Go 系统方法 pkg/shanghaimall.Push，target=${target}，按正常单/换货/退货生成对应商场请求。`,
    enabled: true,
  })),
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
        const response = await fetch(apiURL(path), {
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

  async function retryDeliveryLog(logId: number) {
    const response = await client(`/v1/delivery-logs/${logId}/retry`, { method: 'POST' })
    if (response.ok) await refreshAll(false)
  }

  async function previewBojunOrderBackfill(payload: { start_time: string; end_time: string }) {
    const response = await client('/v1/bojun-order-backfill/preview', {
      method: 'POST',
      body: payload,
    })
    return response.ok ? readObject<BojunOrderBackfillResult>(response, 'result') : null
  }

  async function confirmBojunOrderBackfill(payload: { start_time: string; end_time: string }) {
    const response = await client('/v1/bojun-order-backfill/confirm', {
      method: 'POST',
      body: payload,
    })
    if (response.ok) {
      await refreshAll(false)
      return readObject<BojunOrderBackfillResult>(response, 'result')
    }
    return null
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
          <img className="brand-logo" src="/logo.jpg" alt="数据仓库" />
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
        {activeNav === 'pull' && (
          <PullView
            sources={sources}
            records={pulledData}
            coreMethods={coreMethods.filter((item) => item.key === 'youzan_fetch' || item.key === 'bojun_order_fetch')}
            loading={loading || refreshing}
            onBojunBackfillPreview={previewBojunOrderBackfill}
            onBojunBackfillConfirm={confirmBojunOrderBackfill}
            onToggle={toggleTarget}
          />
        )}
        {activeNav === 'process' && <ProcessView rules={transformRules} records={processedData} coreMethod={coreMethods.find((item) => item.key === 'qimai_process')} onToggle={toggleTarget} />}
        {activeNav === 'push' && <PushConfigView destinations={destinations} tasks={deliveryTasks} coreMethod={coreMethods.find((item) => item.key === 'mall_push')} onToggle={toggleTarget} />}
        {activeNav === 'excel' && <ExcelMatchView token={token} loading={loading} setLoading={setLoading} setResult={setResult} />}
        {activeNav === 'logs' && <LogsView runs={runs} stepRuns={stepRuns} deliveryLogs={deliveryLogs} onLoadSteps={loadStepRuns} onRetryLog={retryDeliveryLog} />}
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
    const response = await fetch(apiURL('/auth/login'), {
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
          <img className="brand-logo large" src="/logo.jpg" alt="数据仓库" />
          <div>
            <h1>数据仓库</h1>
            <p>登录后查看运行情况和现有方法</p>
          </div>
        </div>
        <form className="login-form" onSubmit={submit}>
          <Field label="用户名" name="username" />
          <Field label="密码" name="password" type="password" />
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
    excel: { title: 'Excel 匹配导出', subtitle: '上传大 Excel，按筛选条件匹配伯俊零售单并导出追加列结果。' },
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
  const failedLogs = deliveryLogs.filter((log) => !log.success)
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

function PullView({
  sources,
  records,
  coreMethods,
  loading,
  onBojunBackfillPreview,
  onBojunBackfillConfirm,
  onToggle,
}: {
  sources: SourceDefinition[]
  records: RawData[]
  coreMethods: CoreMethod[]
  loading: boolean
  onBojunBackfillPreview: (payload: { start_time: string; end_time: string }) => Promise<BojunOrderBackfillResult | null>
  onBojunBackfillConfirm: (payload: { start_time: string; end_time: string }) => Promise<BojunOrderBackfillResult | null>
  onToggle: (target: ToggleTarget, enabled: boolean) => void
}) {
  const [backfillPayload, setBackfillPayload] = useState<{ start_time: string; end_time: string } | null>(null)
  const [backfillPreview, setBackfillPreview] = useState<BojunOrderBackfillResult | null>(null)
  const [backfillConfirmed, setBackfillConfirmed] = useState<BojunOrderBackfillResult | null>(null)

  async function previewBojunBackfill(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const payload = {
      start_time: formValue(form, 'start_time'),
      end_time: formValue(form, 'end_time'),
    }
    setBackfillConfirmed(null)
    const result = await onBojunBackfillPreview(payload)
    setBackfillPayload(result ? payload : null)
    setBackfillPreview(result)
  }

  async function confirmBojunBackfill() {
    if (!backfillPayload || !backfillPreview) return
    const ok = window.confirm(`确认写入伯俊补拉数据？预计可写入 ${backfillPreview.writable_count} 条，已存在 ${backfillPreview.existing_count} 条，失败 ${backfillPreview.failed_count} 条。`)
    if (!ok) return
    const result = await onBojunBackfillConfirm(backfillPayload)
    setBackfillConfirmed(result)
  }

  return (
    <div className="view-stack">
      {coreMethods.length > 0 && <Panel title="数据拉取方法" icon={<ArrowDownToLine />} meta="现有拉取能力"><CoreMethodList methods={coreMethods} onToggle={onToggle} /></Panel>}
      <Panel title="伯俊订单补拉" icon={<ArrowDownToLine />} meta="先预览伯俊返回数据，确认后再写入数据库">
        <form className="bojun-backfill-form" onSubmit={previewBojunBackfill}>
          <label>
            开始时间
            <input name="start_time" type="datetime-local" defaultValue={datetimeLocalMinutesAgo(60)} required />
          </label>
          <label>
            结束时间
            <input name="end_time" type="datetime-local" defaultValue={datetimeLocalMinutesAgo(0)} required />
          </label>
          <button className="primary" type="submit" disabled={loading}>
            <ArrowDownToLine aria-hidden="true" />
            预览补拉
          </button>
          <button type="button" disabled={loading || !backfillPreview || backfillPreview.writable_count === 0} onClick={confirmBojunBackfill}>
            <CheckCircle2 aria-hidden="true" />
            确认写入
          </button>
        </form>
        <p className="backfill-note">预览不会写数据库；确认后按伯俊订单号 docno 判重，已存在不覆盖，未存在才写入 raw_data 和 bojun_retail_orders。</p>
        {backfillPreview && <BojunBackfillResultView title="预览结果" result={backfillPreview} />}
        {backfillConfirmed && <BojunBackfillResultView title="写入结果" result={backfillConfirmed} />}
      </Panel>
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

function BojunBackfillResultView({ title, result }: { title: string; result: BojunOrderBackfillResult }) {
  const samples = [...(result.samples ?? []), ...(result.failed_samples ?? [])].slice(0, 12)
  return (
    <section className="backfill-result">
      <div className="backfill-result-title">
        <strong>{title}</strong>
        <span>{result.start_time} ~ {result.end_time} / 拉取 {result.fetch_pages} 页</span>
      </div>
      <div className="overview-grid compact">
        <Metric label="伯俊返回" value={result.total_count} />
        <Metric label="可写入" value={result.writable_count} />
        <Metric label="已存在" value={result.existing_count} />
        <Metric label="已写入" value={result.retail_count} />
        <Metric label="失败" value={result.failed_count} />
      </div>
      {samples.length === 0 ? <EmptyState text="暂无样例数据。" /> : (
        <div className="data-table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>状态</th>
                <th>订单号</th>
                <th>门店</th>
                <th>类型</th>
                <th>数量</th>
                <th>金额</th>
                <th>说明</th>
              </tr>
            </thead>
            <tbody>
              {samples.map((sample, index) => (
                <tr key={`${sample.docno || 'empty'}-${sample.status}-${index}`}>
                  <td>{bojunBackfillStatusLabel(sample.status)}</td>
                  <td>{sample.docno || '-'}</td>
                  <td>{sample.c_store_name || sample.c_store_code || '-'}</td>
                  <td>{sample.order_type_name || sample.order_type_code || '-'}</td>
                  <td>{sample.tot_qty ?? '-'}</td>
                  <td>{sample.tot_amt_actual ?? '-'}</td>
                  <td>{sample.reason || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
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

function ExcelMatchView({
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
  const [jobLogs, setJobLogs] = useState<ExcelMatchJobLog[]>([])
  const [selectedExportFileName, setSelectedExportFileName] = useState('')
  const [selectedImportFileName, setSelectedImportFileName] = useState('')
  const [selectedClearFileName, setSelectedClearFileName] = useState('')
  const [excelDialog, setExcelDialog] = useState<ExcelDialogMode | null>(null)
  const [previewResult, setPreviewResult] = useState<ExcelMatchPreviewResult | null>(null)
  const [uploadRefs, setUploadRefs] = useState<Partial<Record<ExcelUploadSlot, ExcelUploadRef>>>({})
  const [uploadProgress, setUploadProgress] = useState('')
  const [exportSchemes, setExportSchemes] = useState<ExcelMatchScheme[]>([])
  const [importSchemes, setImportSchemes] = useState<ExcelMatchScheme[]>([])
  const [exportDefaults, setExportDefaults] = useState<ExcelExportSchemeConfig>(defaultExcelExportScheme)
  const [importDefaults, setImportDefaults] = useState<ExcelImportSchemeConfig>(defaultExcelImportScheme)
  const [exportFormKey, setExportFormKey] = useState(0)
  const [importFormKey, setImportFormKey] = useState(0)

  function applyJobResult(result: ApiResult) {
    const nextJob = readObject<ExcelMatchJob>(result, 'job')
    if (nextJob) {
      setJob(nextJob)
      setJobID(String(nextJob.id))
    }
    setJobLogs(readList<ExcelMatchJobLog>(result, 'logs'))
  }

  function clearUploadRef(slot: ExcelUploadSlot) {
    setUploadRefs((current) => ({ ...current, [slot]: undefined }))
    setUploadProgress('')
  }

  function buildExportConfig(form: FormData) {
    return {
      operation: 'export_match',
      sheetName: formValue(form, 'sheetName').trim() || 'Sheet1',
      filters: [
        {
          column: formValue(form, 'filterColumn').trim(),
          op: 'eq',
          value: formValue(form, 'filterValue').trim(),
        },
      ],
      matchExcelColumn: formValue(form, 'matchExcelColumn').trim(),
      dbTemplate: 'bojun_retail_order',
      dbMatchField: formValue(form, 'dbMatchField').trim(),
      dbValueField: formValue(form, 'dbValueField').trim(),
      outputColumnName: formValue(form, 'outputColumnName').trim(),
      batchSize: Number(formValue(form, 'batchSize') || 1000),
    }
  }

  function buildImportConfig(form: FormData, confirmWrite: boolean) {
    return {
      operation: 'import_update',
      sheetName: formValue(form, 'sheetName').trim() || 'Sheet1',
      tableName: formValue(form, 'tableName').trim(),
      dbMatchField: formValue(form, 'dbMatchField').trim(),
      matchExcelColumn: formValue(form, 'matchExcelColumn').trim(),
      dbWriteField: formValue(form, 'dbWriteField').trim(),
      writeExcelColumn: formValue(form, 'writeExcelColumn').trim(),
      batchSize: Number(formValue(form, 'batchSize') || 1000),
      dryRun: !confirmWrite,
      confirmWrite,
    }
  }

  function buildConfigPayload(uploadId: string, config: unknown) {
    const payload = new FormData()
    payload.append('uploadId', uploadId)
    payload.append('config', JSON.stringify(config))
    return payload
  }

  const fetchSchemes = useCallback(async (operation: 'export_match' | 'import_update') => {
    const response = await fetch(apiURL(`/v1/excel-match-jobs/schemes?operation=${operation}`), {
      method: 'GET',
      headers: token ? { token } : undefined,
    })
    const data = await response.json().catch(() => ({}))
    if (!response.ok || !isSuccessPayload(data)) {
      throw new Error(readMessage(data) || '查询 Excel 方案失败')
    }
    const value = readDataField(data, 'schemes')
    return Array.isArray(value) ? (value as ExcelMatchScheme[]) : []
  }, [token])

  const loadSchemes = useCallback(async () => {
    try {
      const [nextExportSchemes, nextImportSchemes] = await Promise.all([
        fetchSchemes('export_match'),
        fetchSchemes('import_update'),
      ])
      setExportSchemes(nextExportSchemes)
      setImportSchemes(nextImportSchemes)
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    }
  }, [fetchSchemes, setResult])

  useEffect(() => {
    if (!token) return
    void loadSchemes()
  }, [loadSchemes, token])

  async function ensureExcelUpload(slot: ExcelUploadSlot, file: File) {
    const existing = uploadRefs[slot]
    if (existing && sameExcelFile(file, existing)) {
      return existing.uploadId
    }

    const totalChunks = Math.ceil(file.size / excelChunkSize)
    setUploadProgress(`准备上传 ${file.name}，共 ${totalChunks} 个分片`)

    const createResponse = await fetch(apiURL('/v1/excel-match-jobs/uploads'), {
      method: 'POST',
      headers: token ? { token, 'Content-Type': 'application/json' } : { 'Content-Type': 'application/json' },
      body: JSON.stringify({ fileName: file.name, totalChunks }),
    })
    const createData = await createResponse.json().catch(() => ({}))
    if (!createResponse.ok || !isSuccessPayload(createData)) {
      throw new Error(readMessage(createData) || '创建分片上传会话失败')
    }
    const session = readObjectFromData<ExcelUploadSession>(createData, 'upload')
    if (!session?.uploadId) throw new Error('上传会话返回缺少 uploadId')

    for (let index = 0; index < totalChunks; index++) {
      const start = index * excelChunkSize
      const end = Math.min(file.size, start + excelChunkSize)
      const chunkForm = new FormData()
      chunkForm.append('index', String(index))
      chunkForm.append('totalChunks', String(totalChunks))
      chunkForm.append('chunk', file.slice(start, end), `${file.name}.part${index}`)
      setUploadProgress(`上传分片 ${index + 1}/${totalChunks}`)
      const chunkResponse = await fetch(apiURL(`/v1/excel-match-jobs/uploads/${session.uploadId}/chunks`), {
        method: 'POST',
        headers: token ? { token } : undefined,
        body: chunkForm,
      })
      const chunkData = await chunkResponse.json().catch(() => ({}))
      if (!chunkResponse.ok || !isSuccessPayload(chunkData)) {
        throw new Error(readMessage(chunkData) || `上传分片 ${index + 1} 失败`)
      }
    }

    setUploadProgress('合并 Excel 分片')
    const completeResponse = await fetch(apiURL(`/v1/excel-match-jobs/uploads/${session.uploadId}/complete`), {
      method: 'POST',
      headers: token ? { token, 'Content-Type': 'application/json' } : { 'Content-Type': 'application/json' },
      body: JSON.stringify({ totalChunks }),
    })
    const completeData = await completeResponse.json().catch(() => ({}))
    if (!completeResponse.ok || !isSuccessPayload(completeData)) {
      throw new Error(readMessage(completeData) || '合并 Excel 分片失败')
    }

    const nextRef = {
      uploadId: session.uploadId,
      fileName: file.name,
      size: file.size,
      lastModified: file.lastModified,
      totalChunks,
    }
    setUploadRefs((current) => ({ ...current, [slot]: nextRef }))
    setUploadProgress(`上传完成：${file.name}`)
    return session.uploadId
  }

  async function saveScheme(formElement: HTMLFormElement, operation: 'export_match' | 'import_update') {
    const name = window.prompt('请输入方案名称')
    if (!name?.trim()) return
    const form = new FormData(formElement)
    const config = operation === 'export_match'
      ? buildExportConfig(form)
      : buildImportConfig(form, false)

    setLoading(true)
    try {
      const response = await fetch(apiURL('/v1/excel-match-jobs/schemes'), {
        method: 'POST',
        headers: token ? { token, 'Content-Type': 'application/json' } : { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), operation, config }),
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (nextResult.ok) await loadSchemes()
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    } finally {
      setLoading(false)
    }
  }

  function applyExportScheme(schemeID: string) {
    const scheme = exportSchemes.find((item) => String(item.id) === schemeID)
    if (!scheme) return
    setExportDefaults(exportSchemeDefaults(scheme.config))
    setExportFormKey((value) => value + 1)
    setPreviewResult(null)
    setSelectedExportFileName('')
    clearUploadRef('export')
  }

  function applyImportScheme(schemeID: string) {
    const scheme = importSchemes.find((item) => String(item.id) === schemeID)
    if (!scheme) return
    setImportDefaults(importSchemeDefaults(scheme.config))
    setImportFormKey((value) => value + 1)
    setSelectedImportFileName('')
    clearUploadRef('import')
  }

  async function createExportJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const file = form.get('file')
    if (!(file instanceof File) || file.size === 0) {
      setResult({ ok: false, status: 0, data: '请选择 .xlsx 文件' })
      return
    }

    setLoading(true)
    try {
      const uploadId = await ensureExcelUpload('export', file)
      const payload = buildConfigPayload(uploadId, buildExportConfig(form))
      const response = await fetch(apiURL('/v1/excel-match-jobs'), {
        method: 'POST',
        headers: token ? { token } : undefined,
        body: payload,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (nextResult.ok) {
        applyJobResult(nextResult)
        setExcelDialog(null)
      }
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    } finally {
      setLoading(false)
    }
  }

  async function previewExportJob(formElement: HTMLFormElement) {
    const form = new FormData(formElement)
    const file = form.get('file')
    if (!(file instanceof File) || file.size === 0) {
      setResult({ ok: false, status: 0, data: '请选择 .xlsx 文件' })
      return
    }

    setLoading(true)
    try {
      const uploadId = await ensureExcelUpload('export', file)
      const payload = buildConfigPayload(uploadId, buildExportConfig(form))
      const response = await fetch(apiURL('/v1/excel-match-jobs/preview'), {
        method: 'POST',
        headers: token ? { token } : undefined,
        body: payload,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (nextResult.ok) {
        setPreviewResult(readObject<ExcelMatchPreviewResult>(nextResult, 'preview'))
      }
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    } finally {
      setLoading(false)
    }
  }

  async function createImportJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const file = form.get('file')
    if (!(file instanceof File) || file.size === 0) {
      setResult({ ok: false, status: 0, data: '请选择 .xlsx 文件' })
      return
    }

    const confirmWrite = form.get('confirmWrite') === 'on'
    if (confirmWrite && !window.confirm('确认写入数据库？本次只会填充空的 matched_docno，不会覆盖已有匹配单号。')) {
      return
    }

    setLoading(true)
    try {
      const uploadId = await ensureExcelUpload('import', file)
      const payload = buildConfigPayload(uploadId, buildImportConfig(form, confirmWrite))
      const response = await fetch(apiURL('/v1/excel-match-jobs'), {
        method: 'POST',
        headers: token ? { token } : undefined,
        body: payload,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (nextResult.ok) {
        applyJobResult(nextResult)
        setExcelDialog(null)
      }
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    } finally {
      setLoading(false)
    }
  }

  async function createClearMatchedJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const file = form.get('file')
    if (!(file instanceof File) || file.size === 0) {
      setResult({ ok: false, status: 0, data: '请选择 .xlsx 文件' })
      return
    }

    const confirmWrite = form.get('confirmWrite') === 'on'
    if (confirmWrite && !window.confirm('确认清空命中行的 matched_docno？该操作用于退回未匹配状态。')) {
      return
    }

    setLoading(true)
    try {
      const uploadId = await ensureExcelUpload('clear', file)
      const payload = buildConfigPayload(uploadId, {
        operation: 'clear_matched_docno',
        sheetName: formValue(form, 'sheetName').trim() || 'Sheet1',
        tableName: formValue(form, 'tableName').trim(),
        dbMatchField: formValue(form, 'dbMatchField').trim(),
        matchExcelColumn: formValue(form, 'matchExcelColumn').trim(),
        dbWriteField: 'matched_docno',
        batchSize: Number(formValue(form, 'batchSize') || 1000),
        dryRun: !confirmWrite,
        confirmWrite,
      })
      const response = await fetch(apiURL('/v1/excel-match-jobs'), {
        method: 'POST',
        headers: token ? { token } : undefined,
        body: payload,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (nextResult.ok) {
        applyJobResult(nextResult)
        setExcelDialog(null)
      }
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
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
      const response = await fetch(apiURL(`/v1/excel-match-jobs/${id}`), {
        method: 'GET',
        headers: token ? { token } : undefined,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (nextResult.ok) applyJobResult(nextResult)
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
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
      const response = await fetch(apiURL(`/v1/excel-match-jobs/${id}/download`), {
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
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="view-stack">
      <section className="overview-grid">
        <Metric label="当前任务" value={job?.id ?? '-'} />
        <Metric label="任务状态" value={job ? excelJobStatusLabel(job.status) : '-'} />
        <Metric label="已处理行" value={job ? `${job.processed_rows}/${job.total_rows}` : '-'} />
        <Metric label="匹配结果" value={job ? `${job.matched_rows} 匹配 / ${job.unmatched_rows} 未匹配` : '-'} />
      </section>

      <Panel title="Excel 操作" icon={<Upload />} meta="参数在弹出框内填写，页面只保留状态和结果">
        <div className="excel-action-grid">
          <button type="button" className="excel-action-card" onClick={() => setExcelDialog('export')}>
            <Upload aria-hidden="true" />
            <span>匹配导出</span>
            <small>筛选命中行，查询伯俊字段并追加导出</small>
          </button>
          <button type="button" className="excel-action-card" onClick={() => setExcelDialog('import')}>
            <Database aria-hidden="true" />
            <span>匹配导入</span>
            <small>默认预检，确认后写入空的 matched_docno</small>
          </button>
          <button type="button" className="excel-action-card" onClick={() => setExcelDialog('clear')}>
            <RefreshCcw aria-hidden="true" />
            <span>退回未匹配</span>
            <small>按 Excel 匹配范围清空 matched_docno</small>
          </button>
          <button type="button" className="excel-action-card" onClick={() => setExcelDialog('query')}>
            <Download aria-hidden="true" />
            <span>查询下载</span>
            <small>按任务 ID 查询状态、下载导出结果</small>
          </button>
        </div>
      </Panel>

      {job && (
        <Panel title={`Excel 任务 #${job.id}`} icon={<FileJson />} meta={job.source_file_name || 'job detail'}>
          <div className="excel-job-detail">
            <Metric label="源文件" value={job.source_file_name || '-'} />
            <Metric label="筛选/命中" value={job.filtered_rows || '-'} />
            <Metric label="匹配/更新" value={job.matched_rows || '-'} />
            <Metric label="未匹配" value={job.unmatched_rows || '-'} />
            <Metric label="结果过期" value={formatDate(job.expires_at)} />
            <Metric label="开始时间" value={formatDate(job.started_at)} />
            <Metric label="结束时间" value={formatDate(job.finished_at)} />
          </div>
          {job.error_message && <div className="login-error">{job.error_message}</div>}
          <section className="content-grid two">
            <ReadonlyJSON value={job.config_json || '{}'} />
            <ExcelJobLogList logs={jobLogs} />
          </section>
        </Panel>
      )}

      {excelDialog === 'export' && (
        <Modal title="匹配导出参数" onClose={() => setExcelDialog(null)}>
          <form className="excel-upload-form" onSubmit={createExportJob} key={exportFormKey}>
            <label>
              已保存方案
              <select defaultValue="" onChange={(event) => applyExportScheme(event.currentTarget.value)}>
                <option value="">选择方案</option>
                {exportSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}
              </select>
            </label>
            <button
              type="button"
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form) void saveScheme(form, 'export_match')
              }}
            >
              保存当前方案
            </button>
            <label className="file-input-label">
              Excel 文件
              <input
                name="file"
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                onChange={(event) => {
                  setSelectedExportFileName(event.currentTarget.files?.[0]?.name ?? '')
                  clearUploadRef('export')
                  setPreviewResult(null)
                }}
              />
              <span>{selectedExportFileName || '请选择需要匹配导出的 .xlsx 文件'}</span>
            </label>
            <Field label="Sheet 页名称" name="sheetName" defaultValue={exportDefaults.sheetName} />
            <Field label="筛选列名" name="filterColumn" defaultValue={exportDefaults.filterColumn} />
            <Field label="筛选值" name="filterValue" defaultValue={exportDefaults.filterValue} />
            <Field label="Excel 匹配列名" name="matchExcelColumn" defaultValue={exportDefaults.matchExcelColumn} />
            <label>
              数据库匹配字段
              <select name="dbMatchField" defaultValue={exportDefaults.dbMatchField}>
                {bojunMatchFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
              </select>
            </label>
            <label>
              伯俊取值字段
              <select name="dbValueField" defaultValue={exportDefaults.dbValueField}>
                {bojunValueFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
              </select>
            </label>
            <Field label="追加列名" name="outputColumnName" defaultValue={exportDefaults.outputColumnName} />
            <Field label="批量查询大小" name="batchSize" defaultValue={exportDefaults.batchSize} />
            {uploadProgress && <p className="excel-mode-note">{uploadProgress}</p>}
            <div className="excel-form-actions">
              <button
                type="button"
                onClick={(event) => {
                  const form = event.currentTarget.form
                  if (form) void previewExportJob(form)
                }}
                disabled={loading}
              >
                <FileJson aria-hidden="true" />
                预览匹配
              </button>
              <button className="primary" type="submit" disabled={loading}>
                <Upload aria-hidden="true" />
                创建导出任务
              </button>
            </div>
            {previewResult && <ExcelMatchPreviewPanel preview={previewResult} />}
          </form>
        </Modal>
      )}

      {excelDialog === 'import' && (
        <Modal title="匹配导入参数" onClose={() => setExcelDialog(null)}>
          <form className="excel-upload-form" onSubmit={createImportJob} key={importFormKey}>
            <label>
              已保存方案
              <select defaultValue="" onChange={(event) => applyImportScheme(event.currentTarget.value)}>
                <option value="">选择方案</option>
                {importSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}
              </select>
            </label>
            <button
              type="button"
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form) void saveScheme(form, 'import_update')
              }}
            >
              保存当前方案
            </button>
            <label className="file-input-label">
              Excel 文件
              <input
                name="file"
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                onChange={(event) => {
                  setSelectedImportFileName(event.currentTarget.files?.[0]?.name ?? '')
                  clearUploadRef('import')
                }}
              />
              <span>{selectedImportFileName || '请选择需要导入更新的 .xlsx 文件'}</span>
            </label>
            <Field label="Sheet 页名称" name="sheetName" defaultValue={importDefaults.sheetName} />
            <label>
              匹配表名
              <select name="tableName" defaultValue={importDefaults.tableName}>
                <option value="bojun_retail_orders">伯俊零售单 bojun_retail_orders</option>
              </select>
            </label>
            <label>
              数据库匹配字段
              <select name="dbMatchField" defaultValue={importDefaults.dbMatchField}>
                {bojunMatchFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
              </select>
            </label>
            <Field label="Excel 匹配列名" name="matchExcelColumn" defaultValue={importDefaults.matchExcelColumn} />
            <label>
              写入字段
              <select name="dbWriteField" defaultValue={importDefaults.dbWriteField}>
                <option value="matched_docno">匹配单号 matched_docno</option>
              </select>
            </label>
            <Field label="Excel 写入值列名" name="writeExcelColumn" defaultValue={importDefaults.writeExcelColumn} />
            <Field label="批量更新大小" name="batchSize" defaultValue={importDefaults.batchSize} />
            <label className="checkbox-label">
              <input name="confirmWrite" type="checkbox" />
              确认写入数据库
            </label>
            <p className="excel-mode-note">
              不勾选时只预检匹配数量，不写库；勾选后只写入空的 matched_docno，不覆盖已有匹配单号，不修改伯俊原始字段。
            </p>
            {uploadProgress && <p className="excel-mode-note">{uploadProgress}</p>}
            <div className="excel-form-actions">
              <button className="primary" type="submit" disabled={loading}>
                <Upload aria-hidden="true" />
                创建预检/导入任务
              </button>
            </div>
          </form>
        </Modal>
      )}

      {excelDialog === 'clear' && (
        <Modal title="退回未匹配参数" onClose={() => setExcelDialog(null)}>
          <form className="excel-upload-form" onSubmit={createClearMatchedJob}>
            <label className="file-input-label">
              Excel 文件
              <input
                name="file"
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                onChange={(event) => {
                  setSelectedClearFileName(event.currentTarget.files?.[0]?.name ?? '')
                  clearUploadRef('clear')
                }}
              />
              <span>{selectedClearFileName || '请选择需要退回的 .xlsx 文件'}</span>
            </label>
            <Field label="Sheet 页名称" name="sheetName" defaultValue="Sheet1" />
            <label>
              匹配表名
              <select name="tableName" defaultValue="bojun_retail_orders">
                <option value="bojun_retail_orders">伯俊零售单 bojun_retail_orders</option>
              </select>
            </label>
            <label>
              数据库匹配字段
              <select name="dbMatchField" defaultValue="docno">
                {bojunMatchFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
              </select>
            </label>
            <Field label="Excel 匹配列名" name="matchExcelColumn" defaultValue="外部订单编号" />
            <Field label="批量处理大小" name="batchSize" defaultValue="1000" />
            <label className="checkbox-label">
              <input name="confirmWrite" type="checkbox" />
              确认清空 matched_docno
            </label>
            <p className="excel-mode-note">
              不勾选时只预检会命中的行；勾选后会把命中行的 matched_docno 清空，用于退回未匹配状态。
            </p>
            {uploadProgress && <p className="excel-mode-note">{uploadProgress}</p>}
            <div className="excel-form-actions">
              <button type="submit" disabled={loading}>
                <RefreshCcw aria-hidden="true" />
                创建预检/退回任务
              </button>
            </div>
          </form>
        </Modal>
      )}

      {excelDialog === 'query' && (
        <Modal title="任务查询与下载" onClose={() => setExcelDialog(null)}>
          <div className="excel-job-actions">
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
        </Modal>
      )}
    </div>
  )
}

function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal-panel" role="dialog" aria-modal="true" aria-label={title}>
        <div className="modal-title">
          <h3>{title}</h3>
          <button type="button" onClick={onClose}>关闭</button>
        </div>
        {children}
      </section>
    </div>
  )
}

function ExcelMatchPreviewPanel({ preview }: { preview: ExcelMatchPreviewResult }) {
  return (
    <div className="excel-preview-panel">
      <div className="overview-grid compact">
        <Metric label="扫描行" value={excelPreviewStat(preview.stats, 'TotalRows')} />
        <Metric label="命中筛选" value={excelPreviewStat(preview.stats, 'FilteredRows')} />
        <Metric label="已匹配" value={excelPreviewStat(preview.stats, 'MatchedRows')} />
        <Metric label="未匹配" value={excelPreviewStat(preview.stats, 'UnmatchedRows')} />
        <Metric label="扫描上限" value={preview.truncated ? `${preview.scanLimit}+` : preview.scanLimit} />
      </div>
      <div className="data-table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>行号</th>
              <th>匹配键</th>
              <th>状态</th>
              <th>追加值</th>
              <th>原因</th>
              <th>Excel 行内容</th>
            </tr>
          </thead>
          <tbody>
            {preview.samples.map((sample) => (
              <tr key={`${sample.rowNumber}-${sample.matchKey}-${sample.status}`}>
                <td>{sample.rowNumber}</td>
                <td>{sample.matchKey || '-'}</td>
                <td>{excelPreviewStatusLabel(sample.status)}</td>
                <td>{sample.matchedValue || '-'}</td>
                <td>{sample.reason || '-'}</td>
                <td>{compactText(JSON.stringify(sample.values || {})) || '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {preview.samples.length === 0 && <EmptyState text="预览没有返回样例。" />}
    </div>
  )
}

function ExcelJobLogList({ logs }: { logs: ExcelMatchJobLog[] }) {
  if (logs.length === 0) return <EmptyState text="暂无任务日志。" />
  return (
    <div className="excel-job-log-list">
      {logs.map((log) => (
        <article className="record-row" key={log.id}>
          <div>
            <strong>{log.message}</strong>
            <span>{excelLogLevelLabel(log.level)} / {formatUnixTime(log.created_at)}</span>
            <span>{compactText(log.detail_json || '{}')}</span>
          </div>
        </article>
      ))}
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

function LogsView({ runs, stepRuns, deliveryLogs, onLoadSteps, onRetryLog }: { runs: PipelineRun[]; stepRuns: StepRun[]; deliveryLogs: DeliveryLog[]; onLoadSteps: (runId: number) => void; onRetryLog: (logId: number) => void }) {
  const [selection, setSelection] = useState<LogSelection | null>(() => readLogSelection())

  useEffect(() => {
    const syncRoute = () => setSelection(readLogSelection())
    window.addEventListener('hashchange', syncRoute)
    window.addEventListener('popstate', syncRoute)
    return () => {
      window.removeEventListener('hashchange', syncRoute)
      window.removeEventListener('popstate', syncRoute)
    }
  }, [])

  const detail = useMemo(() => {
    if (!selection) return null
    if (selection.type === 'run') {
      const run = runs.find((item) => item.id === selection.id)
      if (!run) return null
      return {
        type: 'run' as const,
        title: `运行日志 #${run.id}`,
        value: pipelineRunDetail(run),
      }
    }
    const log = deliveryLogs.find((item) => item.id === selection.id)
    if (!log) return null
    return {
      type: 'delivery' as const,
      title: `推送日志 #${log.id}`,
      value: deliveryLogDetail(log),
    }
  }, [deliveryLogs, runs, selection])

  function openLog(nextSelection: LogSelection) {
    pushLogSelection(nextSelection)
    setSelection(nextSelection)
  }

  function closeLog() {
    clearLogSelection()
    setSelection(null)
  }

  if (selection) return <LogDetailPage detail={detail} onBack={closeLog} />

  return (
    <div className="log-board">
      <section className="log-column">
        <Panel title="运行日志" icon={<ScrollText />} meta={`${runs.length} 条运行`}>
          <RunTable runs={runs} onLoadSteps={onLoadSteps} onSelectRun={(run) => openLog({ type: 'run', id: run.id })} />
        </Panel>
      </section>
      <section className="log-column">
        <Panel title="推送日志" icon={<Send />} meta={`${deliveryLogs.length} 条推送`}>
          <DeliveryLogList logs={deliveryLogs} onSelectLog={(log) => openLog({ type: 'delivery', id: log.id })} onRetryLog={onRetryLog} />
        </Panel>
      </section>
      <section className="log-column">
        <Panel title="步骤日志" icon={<BookOpen />} meta="选择运行记录后加载">
          <StepRunList stepRuns={stepRuns} />
        </Panel>
      </section>
    </div>
  )
}

function LogDetailPage({ detail, onBack }: { detail: LogDetail | null; onBack: () => void }) {
  return (
    <div className="view-stack">
      <div className="detail-toolbar">
        <button type="button" onClick={onBack}>返回日志列表</button>
      </div>
      <Panel title={detail?.title ?? '日志详情'} icon={<FileJson />} meta={detail?.type === 'delivery' ? 'request / response / error' : 'run detail'}>
        {detail ? <ReadonlyJSON value={detail.value} /> : <EmptyState text="当前日志不存在或还未加载。" />}
      </Panel>
    </div>
  )
}

function RunTable({ runs, onLoadSteps, onSelectRun }: { runs: PipelineRun[]; onLoadSteps: (runId: number) => void; onSelectRun?: (run: PipelineRun) => void }) {
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
              <td>
                <div className="table-actions">
                  <button type="button" onClick={() => onSelectRun?.(run)}>详情</button>
                  <button type="button" onClick={() => onLoadSteps(run.id)}>步骤</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function DeliveryLogList({ logs, onSelectLog, onRetryLog }: { logs: DeliveryLog[]; onSelectLog?: (log: DeliveryLog) => void; onRetryLog?: (logId: number) => void }) {
  const [storeFilter, setStoreFilter] = useState('all')
  const matchedLogs = useMemo(
    () => logs.map((log) => ({ log, store: matchDeliveryStore(log) })).filter((item): item is DeliveryLogWithStore => Boolean(item.store)),
    [logs],
  )
  const visibleLogs = storeFilter === 'all' ? matchedLogs : matchedLogs.filter((item) => item.store.key === storeFilter)
  const groupedLogs = useMemo(() => {
    return deliveryStores
      .map((store) => ({
        store,
        logs: visibleLogs.filter((item) => item.store.key === store.key).map((item) => item.log),
      }))
      .filter((group) => group.logs.length > 0)
  }, [visibleLogs])

  if (matchedLogs.length === 0) return <EmptyState text="暂无匹配门店的推送日志。" />
  return (
    <div className="store-log-layout">
      <div className="log-filter-bar">
        <label>
          门店
          <select value={storeFilter} onChange={(event) => setStoreFilter(event.target.value)}>
            <option value="all">全部匹配门店</option>
            {deliveryStores.map((store) => (
              <option key={store.key} value={store.key}>{store.name}</option>
            ))}
          </select>
        </label>
        <span>已匹配 {matchedLogs.length} 条，当前显示 {visibleLogs.length} 条</span>
      </div>
      {groupedLogs.length === 0 ? <EmptyState text="当前门店暂无推送日志。" /> : (
        <div className="store-log-groups">
          {groupedLogs.map((group) => {
            const successCount = group.logs.filter((log) => log.success).length
            const failedCount = group.logs.length - successCount
            return (
              <section className="store-log-group" key={group.store.key}>
                <div className="store-log-title">
                  <div>
                    <strong>{group.store.name}</strong>
                    <span>{group.logs.length} 条 / 成功 {successCount} / 失败 {failedCount}</span>
                  </div>
                  <StatusPill label={failedCount > 0 ? '存在失败' : '全部成功'} />
                </div>
                <div className="record-list">
                  {group.logs.slice(0, 8).map((log) => (
                    <article className="record-row" key={log.id}>
                      <div>
                        <strong>#{log.id} / {log.business_key || '-'}</strong>
                        <span>
                          {log.success ? '成功' : '失败'} / 来源 {log.source_code || '-'} / HTTP {log.http_status || '-'}
                          {!log.success && ` / 重试 ${log.retry_count || 0}`}
                        </span>
                        <span>{log.error_message || deliveryLogPreview(log)}</span>
                      </div>
                      <div className="record-actions">
                        <small>{formatDate(log.sent_at)}</small>
                        {!log.success && onRetryLog && <button type="button" onClick={() => onRetryLog(log.id)}>重试</button>}
                        {onSelectLog && <button type="button" onClick={() => onSelectLog(log)}>详情</button>}
                      </div>
                    </article>
                  ))}
                </div>
              </section>
            )
          })}
        </div>
      )}
    </div>
  )
}

function apiURL(path: string) {
  const normalizedPath = path.startsWith('/api') ? path : `/api${path.startsWith('/') ? path : `/${path}`}`
  const base = defaultApiBaseURL.replace(/\/$/, '')
  return `${base}${normalizedPath}`
}

function readLogSelection(): LogSelection | null {
  const match = window.location.hash.match(/^#\/logs\/(run|delivery)\/(\d+)$/)
  if (!match) return null
  return {
    type: match[1] as LogSelection['type'],
    id: Number(match[2]),
  }
}

function pushLogSelection(selection: LogSelection) {
  window.history.pushState(null, '', `#/logs/${selection.type}/${selection.id}`)
}

function clearLogSelection() {
  window.history.pushState(null, '', `${window.location.pathname}${window.location.search}`)
}

function pipelineRunDetail(run: PipelineRun) {
  return {
    id: run.id,
    trace_id: run.trace_id,
    run_type: run.run_type,
    trigger_type: run.trigger_type,
    status: run.status,
    counts: {
      total: run.total_count,
      success: run.success_count,
      failed: run.failed_count,
    },
    refs: {
      source_id: run.source_id,
      destination_id: run.destination_id,
    },
    error_message: run.error_message,
    started_at: run.started_at,
    finished_at: run.finished_at,
  }
}

function deliveryLogDetail(log: DeliveryLog) {
  return {
    id: log.id,
    trace_id: log.trace_id,
    run_id: log.run_id,
    target: {
      destination_id: log.destination_id,
      destination_code: log.destination_code,
      destination_name: log.destination_name,
    },
    source_code: log.source_code,
    clean_record_id: log.clean_record_id,
    business_key: log.business_key,
    http_status: log.http_status,
    success: log.success,
    error_message: log.error_message,
    sent_at: log.sent_at,
    request_body: parseJsonText(log.request_body),
    response_body: parseJsonText(log.response_body),
  }
}

function matchDeliveryStore(log: DeliveryLog) {
  const text = [
    log.destination_code,
    log.destination_name,
    log.source_code,
    log.business_key,
    log.request_body,
    log.response_body,
    log.error_message,
  ].join(' ').toLowerCase()

  return deliveryStores.find((store) => store.aliases.some((alias) => text.includes(alias.toLowerCase()))) ?? null
}

function deliveryLogPreview(log: DeliveryLog) {
  const response = compactText(log.response_body)
  if (response) return response
  const request = compactText(log.request_body)
  if (request) return `请求 ${request}`
  return `trace: ${log.trace_id || '-'}`
}

function compactText(value: string) {
  const text = (value || '').replace(/\s+/g, ' ').trim()
  if (text.length <= 120) return text
  return `${text.slice(0, 120)}...`
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

function Field({ label, name, defaultValue = '', type = 'text' }: { label: string; name: string; defaultValue?: string; type?: string }) {
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
    shanghai_mall_push: '商场推送方法',
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
      description: '每分钟调用伯俊 `/retail/middleretail.query` 拉取订单，也支持按开始/结束时间手动补拉。',
      enabled: true,
      status: '系统定时任务每分钟执行；补拉时 docno 已存在不覆盖，未存在才写入。',
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
    shanghai_mall_push: '上海商场推送',
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

function readObjectFromData<T>(data: unknown, key: string): T | null {
  const value = readDataField(data, key)
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

function sameExcelFile(file: File, ref: ExcelUploadRef) {
  return file.name === ref.fileName && file.size === ref.size && file.lastModified === ref.lastModified
}

function exportSchemeDefaults(config: ExcelMatchSchemeConfig): ExcelExportSchemeConfig {
  const filter = Array.isArray(config.filters) && config.filters.length > 0 ? config.filters[0] : {}
  return {
    sheetName: config.sheetName || defaultExcelExportScheme.sheetName,
    filterColumn: filter.column || defaultExcelExportScheme.filterColumn,
    filterValue: filter.value || defaultExcelExportScheme.filterValue,
    matchExcelColumn: config.matchExcelColumn || defaultExcelExportScheme.matchExcelColumn,
    dbMatchField: config.dbMatchField || defaultExcelExportScheme.dbMatchField,
    dbValueField: config.dbValueField || defaultExcelExportScheme.dbValueField,
    outputColumnName: config.outputColumnName || defaultExcelExportScheme.outputColumnName,
    batchSize: config.batchSize ? String(config.batchSize) : defaultExcelExportScheme.batchSize,
  }
}

function importSchemeDefaults(config: ExcelMatchSchemeConfig): ExcelImportSchemeConfig {
  return {
    sheetName: config.sheetName || defaultExcelImportScheme.sheetName,
    tableName: config.tableName || defaultExcelImportScheme.tableName,
    dbMatchField: config.dbMatchField || defaultExcelImportScheme.dbMatchField,
    matchExcelColumn: config.matchExcelColumn || defaultExcelImportScheme.matchExcelColumn,
    dbWriteField: config.dbWriteField || defaultExcelImportScheme.dbWriteField,
    writeExcelColumn: config.writeExcelColumn || defaultExcelImportScheme.writeExcelColumn,
    batchSize: config.batchSize ? String(config.batchSize) : defaultExcelImportScheme.batchSize,
  }
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

function excelJobStatusLabel(value: string) {
  const labels: Record<string, string> = {
    pending: '等待处理',
    running: '处理中',
    success: '成功',
    failed: '失败',
    expired: '已过期',
  }
  return labels[value] ?? (value || '-')
}

function excelPreviewStat(stats: ExcelMatchPreviewStats, key: keyof ExcelMatchPreviewStats) {
  const value = stats[key]
  return typeof value === 'number' ? value : 0
}

function excelPreviewStatusLabel(value: string) {
  const labels: Record<string, string> = {
    matched: '已匹配',
    unmatched: '未匹配',
    skipped: '已跳过',
  }
  return labels[value] ?? (value || '-')
}

function excelLogLevelLabel(value: string) {
  const labels: Record<string, string> = {
    info: '信息',
    warn: '警告',
    error: '错误',
  }
  return labels[value] ?? (value || '-')
}

function bojunBackfillStatusLabel(value: string) {
  const labels: Record<string, string> = {
    pending: '待写入',
    created: '已写入',
    exists: '已存在',
    invalid: '无效',
    failed: '失败',
    push_failed: '推送失败',
  }
  return labels[value] ?? (value || '-')
}

function formatUnixTime(value: number) {
  if (!value) return '-'
  return formatDate(new Date(value * 1000).toISOString())
}

function readDownloadFilename(contentDisposition: string | null) {
  if (!contentDisposition) return null
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) return decodeURIComponent(utf8Match[1])
  const plainMatch = contentDisposition.match(/filename="?([^";]+)"?/i)
  return plainMatch?.[1] ?? null
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
  const normalized = value.includes('T') ? value : `${value.replace(' ', 'T')}+08:00`
  const date = new Date(normalized)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date).replace(/\//g, '-')
}

function datetimeLocalMinutesAgo(minutes: number) {
  const date = new Date(Date.now() - minutes * 60 * 1000)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
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
