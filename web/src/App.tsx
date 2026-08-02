import { FormEvent, ReactNode, type RefObject, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import {
  Activity,
  AlertTriangle,
  ArrowDownToLine,
  ArrowUpFromLine,
  BookOpen,
  Building2,
  CalendarDays,
  CheckCircle2,
  ChevronDown,
  CloudSun,
  Database,
  Download,
  FileJson,
  Inbox,
  ListChecks,
  LogOut,
  Menu,
  RefreshCcw,
  Search,
  ScrollText,
  Send,
  ShieldCheck,
  Upload,
  Wrench,
  X,
} from 'lucide-react'
import './App.css'
import './DataWorkspacePages.css'
import { apiURL as buildApiURL } from './apiURL'
import { clearStoredToken, loadStoredSessionUser, loadStoredToken, saveStoredSessionUser, saveStoredToken, saveStoredTokenExpiry, storedTokenExpiresAt, tokenActorID, type StoredSessionUser } from './authStorage'
import { createApiClient, type ApiRequestOptions, type ClientResponse, type HTTPMethod } from './api/client'
import { verifySessionResponses, type SessionUser } from './api/auth'
import { parseDataStatisticsSummary, parseHealthSummary, parseMallWeatherMetricsSummary, redactMonitoringJSON, type DataStatisticsSummary, type HealthSummary, type MallWeatherMetricsSummary } from './monitoring'
import { MallWeatherPage, StoreInfoPage } from './MallWeatherPage'
import { DataAuthorizationPage } from './DataAuthorizationPage'
import { PipelineRunPanel } from './PipelineRunPanel'
import { PipelineComposerPanel } from './PipelineComposerPanel'
import { pipelineListPath } from './pipelineRun'
import { Brand } from './components/Brand'
import { parseMallWeatherExportContentStatus, submitMallWeatherExportContentDownload } from './mallWeatherExport'
import { buildRawRecordsRequest, buildWarehouseRawRecordsQuery, parseRawRecordsPage, type RawRecordOrigin, type RawRecordsPage } from './rawRecords'
import { buildDeliveryLogListQuery, buildDeliveryTaskListQuery, buildDestinationListQuery, buildExcelMatchJobListQuery, buildRunListQuery, buildSourceListQuery, buildTransformRuleListQuery, normalizeMonitoringPageNumber, parseMonitoringPage, type MonitoringPage, type MonitoringPagination } from './monitoringRecords'
import { validateOrderPushSkipPolicy } from './orderPushPolicy'
import { runSingleFlight } from './singleFlight'
import { parseSourceFetchSummary } from './sourceOperations'
import { buildCleanRecordsQuery, buildProcessedRecordsQuery, parseProcessedRecordsPage, type ProcessedRecordsPage } from './processedRecords'
import {
  buildExcelExportConfig,
  cloneExcelMatchSteps,
  excelMatchSchemePath,
  excelFieldSelectOptions,
  excelModelSelectOptions,
  migrateExcelMatchSteps,
  selectExcelMatchStepModel,
  type ExcelMatchFilterConfig,
  type ExcelMatchModel,
  type ExcelMatchModelField,
  type ExcelMatchStepConfig,
} from './excelMatchConfig'

const defaultApiBaseURL = import.meta.env.VITE_API_BASE_URL ?? ''

type ApiResult = ClientResponse

type ApiClientOptions = Omit<ApiRequestOptions, 'method'> & {
  method?: HTTPMethod
  showResult?: boolean
  silentLoading?: boolean
}

type ApiClient = (path: string, options?: ApiClientOptions) => Promise<ApiResult>
type MonitoringSnapshot = { statistics: DataStatisticsSummary | null; weather: MallWeatherMetricsSummary | null; health: HealthSummary | null }
type FileDownloadClient = (path: string, fileName: string, signal: AbortSignal) => Promise<ApiResult>
type NavKey = 'overview' | 'runs' | 'delivery_logs' | 'step_runs' | 'store_info' | 'mall_weather' | 'data_authorizations' | 'sources' | 'receive' | 'pull_records' | 'backfill' | 'youzan_distribution' | 'rules' | 'processed' | 'methods' | 'destinations' | 'tasks' | 'push_policy' | 'excel_jobs' | 'excel_schemes' | 'excel_write'
type NavItem = { key: NavKey; label: string; description: string; icon: ReactNode }
type NavGroup = { label: string; items: NavItem[] }
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
  started_at: string | null
  finished_at: string | null
}

type ExcelMatchJob = {
  id: number
  source_file_name: string
  config_json?: string
  status: string
  total_rows: number
  processed_rows: number
  filtered_rows: number
  matched_rows: number
  unmatched_rows: number
  error_message?: string
  started_at: string | null
  finished_at: string | null
  expires_at: string | null
  result_url?: string
  can_download?: boolean
  download_message?: string
  operation?: string
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

type ExcelExportColumnFormat = {
  column: string
  format: string
}

type ExcelExportSchemeConfig = {
  sheetName: string
  steps: ExcelMatchStepConfig[]
  exportColumnFormats: string
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
  filters?: Array<Partial<ExcelMatchFilterConfig>>
  matchExcelColumn?: string
  dbTemplate?: string
  dbMatchField?: string
  dbValueField?: string
  tableName?: string
  dbWriteField?: string
  writeExcelColumn?: string
  outputColumnName?: string
  steps?: ExcelMatchStepConfig[]
  exportColumnFormats?: ExcelExportColumnFormat[]
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

type PendingSchemeSave = {
  operation: 'export_match' | 'import_update'
  config: unknown
  name: string
  overwriteConfirmed: boolean
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
  stepResults?: Array<{ stepIndex: number; stepName: string; matchKey: string; matchedValue: string; status: string; reason: string }>
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

const excelChunkSize = 4 * 1024 * 1024
const excelJobPollMaxAttempts = 60

const excelMatchFilterOperatorOptions = [
  { value: 'eq', label: '等于' },
  { value: 'neq', label: '不等于' },
  { value: 'contains', label: '包含' },
  { value: 'not_contains', label: '不包含' },
  { value: 'starts_with', label: '开头是' },
  { value: 'ends_with', label: '结尾是' },
  { value: 'empty', label: '为空' },
  { value: 'not_empty', label: '不为空' },
]

const defaultExcelExportScheme: ExcelExportSchemeConfig = {
  sheetName: 'Sheet1',
  steps: [{
    name: '匹配伯俊门店',
    filters: [{ column: '店铺', op: 'eq', value: '幼岚-有赞' }],
    matchMode: 'field',
    tableName: 'bojun_retail_orders',
    matchExcelColumn: '原始线上订单号',
    dbMatchField: 'matched_docno',
    dbValueField: 'c_store_name',
    outputColumnName: '线下店名称',
    specExcelColumn: '',
    priceExcelColumn: '',
    qtyExcelColumn: '',
  }],
  exportColumnFormats: '',
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

const defaultOrderPushSkipConfig: OrderPushSkipConfig = {
  targets: [],
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
  pipeline_id: number
  step_id: number
  step_code: string
  method_type: string
  status: string
  input_json: string
  output_json: string
  generated_config_json: string
  error_message: string
  started_at: string | null
  finished_at: string | null
}

type SourceDefinition = {
  id: number
  name: string
  code: string
  source_type: string
  enabled: boolean
  auth_type: string
  config_json: string
	 has_secret?: boolean
  schema_json: string
  dedupe_keys: string
  source_query_key: string
}

type SourceDraft = {
  id: number | null
  name: string
  code: string
  sourceType: string
  enabled: boolean
  authType: string
  configJSON: string
  schemaJSON: string
  dedupeKeys: string
  sourceQueryKey: string
  hasSecret: boolean
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

type WarehouseRawRecord = {
  id: number
  sourceID: number
  sourceCode: string
  status: 'received' | 'queued' | 'cleaning' | 'cleaned' | 'failed'
  traceID: string
  receivedAt: string
  createdAt: number
}

type ProcessedData = {
  id: number
  raw_data_id: number
  data_type: string
  data_fields: string
  quality_score: number
  created_at: number
}

type CleanRecord = {
  id: number
  raw_record_id: number
  source_id: number
  table_name: string
  business_key: string
  quality_score: number
  status: string
  created_at: number
}

type TransformRule = {
  id: number
  source_id: number
  name: string
  rule_type: string
  order_index: number
  config_json: string
	 has_secret?: boolean
  enabled: boolean
}

type RuleDraft = {
  id: number | null
  sourceID: string
  name: string
  ruleType: string
  orderIndex: string
  configJSON: string
  enabled: boolean
  hasSecret: boolean
}

type DestinationDefinition = {
  id: number
  name: string
  code: string
  destination_type: string
  config_json: string
  has_secret?: boolean
  enabled: boolean
}

type DestinationDraft = {
  id: number | null
  name: string
  code: string
  destinationType: string
  configJSON: string
  enabled: boolean
  hasSecret: boolean
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

type DeliveryTaskDraft = {
  id: number | null
  name: string
  sourceID: string
  cleanTable: string
  destinationID: string
  triggerType: string
  cronExpr: string
  filterJSON: string
  payloadTemplate: string
  enabled: boolean
}

type OrderPushSkipConfig = {
  targets: OrderPushSkipTargetConfig[]
}

type OrderPushSkipTargetConfig = {
  target_code: string
  target_name: string
  cycle: number
  skip: number
}

type OrderPushTargetOption = {
  code: string
  name: string
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

type LegacyTaskRunResult = {
  id: string
  queue: string
  type: string
}

type YouzanDistributionBackfillSample = {
  tid: string
  status: string
  reason: string
  success_time: string
  payment: string
  fans_nickname: string
}

type YouzanDistributionTimeFilter = 'created' | 'success'

type YouzanDistributionBackfillPayload = {
  time_filter: YouzanDistributionTimeFilter
  start_time: string
  end_time: string
}

type YouzanDistributionBackfillResult = {
  time_filter: YouzanDistributionTimeFilter
  start_time: string
  end_time: string
  page_size: number
  fetch_pages: number
  total_count: number
  preview_count: number
  writable_count: number
  saved_count: number
  existing_count: number
  failed_count: number
  samples: YouzanDistributionBackfillSample[]
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
  response_summary: string
  http_status: number
  success: boolean
  error_message: string
  retry_count: number
  sent_at: string | null
}

const navGroups: NavGroup[] = [
  {
    label: '基础信息',
    items: [
      { key: 'store_info', label: '店铺信息', description: '店铺资料与坐标维护', icon: <Building2 aria-hidden="true" /> },
    ],
  },
  {
    label: '运行监控',
    items: [
      { key: 'overview', label: '运行总览', description: '运行与推送健康度', icon: <Activity aria-hidden="true" /> },
      { key: 'runs', label: '流水线运行', description: '按状态与 Trace 查询', icon: <ListChecks aria-hidden="true" /> },
      { key: 'delivery_logs', label: '推送日志', description: '按门店与业务键查询', icon: <Send aria-hidden="true" /> },
      { key: 'step_runs', label: '步骤运行', description: '选择运行查看步骤', icon: <BookOpen aria-hidden="true" /> },
    ],
  },
  {
    label: '数据接入',
    items: [
      { key: 'sources', label: '数据源', description: '接入配置与启用状态', icon: <Database aria-hidden="true" /> },
      { key: 'receive', label: '接口接收', description: '外部推送入库记录', icon: <Inbox aria-hidden="true" /> },
      { key: 'pull_records', label: '拉取记录', description: '主动拉取原始数据', icon: <ArrowDownToLine aria-hidden="true" /> },
      { key: 'backfill', label: '伯俊补拉', description: '预览并确认补写订单', icon: <Download aria-hidden="true" /> },
      { key: 'youzan_distribution', label: '有赞分销', description: '每日拉取与手动补拉', icon: <Download aria-hidden="true" /> },
    ],
  },
  {
    label: '数据服务',
    items: [
      { key: 'mall_weather', label: '商场天气', description: '实况、趋势与预警', icon: <CloudSun aria-hidden="true" /> },
    ],
  },
  {
    label: '数据授权',
    items: [
      { key: 'data_authorizations', label: '授权管理', description: '开户、授权与访问审计', icon: <ShieldCheck aria-hidden="true" /> },
    ],
  },
  {
    label: '数据处理',
    items: [
      { key: 'rules', label: '清洗规则', description: '规则类型与执行顺序', icon: <ListChecks aria-hidden="true" /> },
      { key: 'processed', label: '处理结果', description: '质量与业务数据查询', icon: <CheckCircle2 aria-hidden="true" /> },
      { key: 'methods', label: '方法目录', description: '配置与系统方法', icon: <Wrench aria-hidden="true" /> },
    ],
  },
  {
    label: '数据交付',
    items: [
      { key: 'destinations', label: '推送目标', description: '目标接口配置', icon: <Send aria-hidden="true" /> },
      { key: 'tasks', label: '推送任务', description: '任务与目标关系', icon: <ArrowUpFromLine aria-hidden="true" /> },
      { key: 'push_policy', label: '推送策略', description: '订单少推送设置', icon: <ListChecks aria-hidden="true" /> },
    ],
  },
  {
    label: '数据工具',
    items: [
      { key: 'excel_jobs', label: 'Excel 任务', description: '状态与结果下载', icon: <ScrollText aria-hidden="true" /> },
      { key: 'excel_schemes', label: 'Excel 匹配', description: '自定义多步骤方案', icon: <Upload aria-hidden="true" /> },
      { key: 'excel_write', label: 'Excel 写入', description: '导入与退回未匹配', icon: <Database aria-hidden="true" /> },
    ],
  },
]

const navItems = navGroups.flatMap((group) => group.items)

function navGroupFor(key: NavKey) {
  return navGroups.find((group) => group.items.some((item) => item.key === key))
}

function navFromHash(): NavKey {
  const value = window.location.hash.replace(/^#\/?/, '') as NavKey
  return navItems.some((item) => item.key === value) ? value : 'overview'
}

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
  const [token, setToken] = useState(() => loadStoredToken(window.localStorage))
  const tokenRef = useRef(token)
  const actorID = useMemo(() => tokenActorID(token), [token])
  const [sessionState, setSessionState] = useState<'checking' | 'authenticated' | 'anonymous'>(() => token ? 'checking' : 'anonymous')
  const authenticatedSessionRef = useRef(sessionState === 'authenticated')
  const [sessionUser, setSessionUser] = useState<SessionUser | null>(() => {
    const user = loadStoredSessionUser(window.localStorage)
    return user ? { ...user, email: '', consoleManaged: true } : null
  })
  const [sessionExpiresAt, setSessionExpiresAt] = useState(() => storedTokenExpiresAt(window.localStorage))
  const [sessionValidationError, setSessionValidationError] = useState('')
  const [sessionValidationAttempt, setSessionValidationAttempt] = useState(0)
  const [activeNav, setActiveNav] = useState<NavKey>(navFromHash)
  const [expandedNavGroup, setExpandedNavGroup] = useState(() => navGroupFor(navFromHash())?.label ?? navGroups[0].label)
  const [navQuery, setNavQuery] = useState('')
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const mobileNavTriggerRef = useRef<HTMLButtonElement>(null)
  const mobileNavRef = useRef<HTMLElement>(null)
  const [loading, setLoading] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [workspaceRefreshVersion, setWorkspaceRefreshVersion] = useState(0)
  const [workspaceError, setWorkspaceError] = useState('')
  const [result, setResult] = useState<ApiResult | null>(null)
  const [runs, setRuns] = useState<PipelineRun[]>([])
  const [stepRunFocusID, setStepRunFocusID] = useState<number | null>(null)
  const workspaceRequestRef = useRef<AbortController | null>(null)
  const [methods, setMethods] = useState<MethodDisplay[]>(builtinMethods)
  const [pipelines, setPipelines] = useState<PipelineDefinition[]>([])
  const [sources, setSources] = useState<SourceDefinition[]>([])
  const [transformRules, setTransformRules] = useState<TransformRule[]>([])
  const [destinations, setDestinations] = useState<DestinationDefinition[]>([])
  const [deliveryTasks, setDeliveryTasks] = useState<DeliveryTask[]>([])
  const [deliveryLogs, setDeliveryLogs] = useState<DeliveryLog[]>([])
  const [overviewTotals, setOverviewTotals] = useState({ runs: null as number | null, deliveryLogs: null as number | null })
  const [monitoring, setMonitoring] = useState<MonitoringSnapshot>({ statistics: null, weather: null, health: null })
  const [monitoringStale, setMonitoringStale] = useState(false)
  const [orderPushSkipConfig, setOrderPushSkipConfig] = useState<OrderPushSkipConfig>(defaultOrderPushSkipConfig)
  const [orderPushTargets, setOrderPushTargets] = useState<OrderPushTargetOption[]>([])
  const [legacyTasks, setLegacyTasks] = useState<LegacyTask[]>([])
  const [legacyRules, setLegacyRules] = useState<LegacyTransformRule[]>([])

  const clearSession = useCallback(() => {
    clearStoredToken(window.localStorage)
    tokenRef.current = ''
    setToken('')
    setSessionUser(null)
    setSessionExpiresAt(null)
    setSessionValidationError('')
    setSessionState('anonymous')
    setResult(null)
  }, [])

  const updateSessionToken = useCallback((nextToken: string) => {
    const expiresAt = saveStoredToken(nextToken, window.localStorage)
    tokenRef.current = nextToken
    setToken(nextToken)
    setSessionExpiresAt(expiresAt)
  }, [])

  const apiClient = useMemo(
    () => createApiClient({
      baseURL: defaultApiBaseURL,
      getToken: () => tokenRef.current,
      onTokenRefreshed: updateSessionToken,
      onUnauthorized: clearSession,
    }),
    [clearSession, updateSessionToken],
  )

  const client = useCallback<ApiClient>(
    async (path, options = {}) => {
      if (!options.silentLoading) setLoading(true)
      try {
        if (!options.method) {
          const nextResult: ApiResult = {
            ok: false,
            status: 0,
            data: { message: '请求方法未指定。' },
            error: { kind: 'client', message: '请求方法未指定。' },
          }
          if (options.showResult !== false) setResult(nextResult)
          return nextResult
        }
        const requestOptions: ApiRequestOptions = {
          method: options.method,
          body: options.body,
          headers: options.headers,
          signal: options.signal,
          retry: options.retry,
          timeoutMs: options.timeoutMs,
          acceptBareJSONSuccess: options.acceptBareJSONSuccess,
        }
        const nextResult = await apiClient.request(path, requestOptions)
        if (options.showResult !== false) setResult(nextResult)
        return nextResult
      } finally {
        if (!options.silentLoading) setLoading(false)
      }
    },
    [apiClient],
  )

  const downloadFile = useCallback<FileDownloadClient>(
    async (path, fileName, signal) => {
      const validFileName = /^mall_weather_export_[0-9a-f-]{36}\.xlsx$/i.test(fileName)
      if (!validFileName) return { ok: false, status: 422, data: 'invalid download file name' }
      if (signal.aborted) return { ok: false, status: 0, data: 'download request aborted' }
      try {
        const readiness = await client(`${path}/status`, {
          method: 'GET', signal, showResult: false, silentLoading: true,
        })
        if (!readiness.ok) return readiness
        const contentStatus = parseMallWeatherExportContentStatus(readiness.data)
        if (!contentStatus || contentStatus.fileName !== fileName) {
          return { ok: false, status: 502, data: 'invalid XLSX content status' }
        }
        submitMallWeatherExportContentDownload(
          document,
          apiURL(path),
          token,
          fileName,
          (callback, delayMilliseconds) => window.setTimeout(callback, delayMilliseconds),
        )
        return { ok: true, status: readiness.status, data: contentStatus }
      } catch (error) {
        return { ok: false, status: 0, data: error instanceof Error ? error.message : String(error) }
      }
    },
    [client, token],
  )

  const loadConfiguredMethods = useCallback(async (pipelines: PipelineDefinition[], signal: AbortSignal) => {
    const details = await Promise.all(
      pipelines.map((pipeline) => client(`/v1/pipelines/${pipeline.id}`, { method: 'GET', signal, showResult: false, silentLoading: true })),
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

  const refreshWorkspace = useCallback(
    async (showResult = false) => {
      if (!token) return
      workspaceRequestRef.current?.abort()
      const controller = new AbortController()
      workspaceRequestRef.current = controller
      setRefreshing(true)
      setWorkspaceError('')
      try {
        const get = (path: string) => client(path, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
        if (activeNav === 'overview') {
          const startTime = monitoringDayStartTime()
          const [runResult, logResult] = await Promise.all([
            get(`/v1/runs?page=1&page_size=100&start_time=${encodeURIComponent(startTime)}`),
            get(`/v1/delivery-logs?page=1&page_size=100&start_time=${encodeURIComponent(startTime)}`),
          ])
          if (!controller.signal.aborted) {
            const runPage = runResult.ok ? parseMonitoringPage<unknown>(runResult.data, 'runs') : null
            const parsedRuns = runPage?.list.map(parsePipelineRun) ?? []
            if (runPage && parsedRuns.every((run): run is PipelineRun => run !== null)) {
              setRuns(parsedRuns)
              setOverviewTotals((current) => ({ ...current, runs: runPage.pagination.total }))
            } else if (runResult.ok) {
              setRuns(readList<PipelineRun>(runResult, 'runs'))
              setOverviewTotals((current) => ({ ...current, runs: null }))
            }
            const logPage = logResult.ok ? parseMonitoringPage<unknown>(logResult.data, 'logs') : null
            const parsedLogs = logPage?.list.map(parseDeliveryLog) ?? []
            if (logPage && parsedLogs.every((log): log is DeliveryLog => log !== null)) {
              setDeliveryLogs(parsedLogs)
              setOverviewTotals((current) => ({ ...current, deliveryLogs: logPage.pagination.total }))
            } else if (logResult.ok) {
              setDeliveryLogs(readList<DeliveryLog>(logResult, 'logs'))
              setOverviewTotals((current) => ({ ...current, deliveryLogs: null }))
            }
          }
        } else if (activeNav === 'step_runs') {
          const runResult = await get('/v1/runs?limit=50')
          if (!controller.signal.aborted && runResult.ok) setRuns(readList<PipelineRun>(runResult, 'runs'))
        } else if (activeNav === 'runs') {
          const pipelineResult = await get(pipelineListPath())
          if (!controller.signal.aborted) {
            if (pipelineResult.ok) setPipelines(readList<PipelineDefinition>(pipelineResult, 'pipelines'))
            if (!pipelineResult.ok) setWorkspaceError('可执行流水线加载失败，已保留上一次成功数据。')
          }
        } else if (activeNav === 'rules') {
          const sourceResult = await get('/v1/sources')
          if (!controller.signal.aborted) {
            if (sourceResult.ok) setSources(readList<SourceDefinition>(sourceResult, 'sources'))
            if (!sourceResult.ok) setWorkspaceError('规则来源加载失败，已保留上一次成功数据。')
          }
        } else if (activeNav === 'tasks') {
          const [sourceResult, destinationResult] = await Promise.all([get('/v1/sources'), get('/v1/destinations')])
          if (!controller.signal.aborted) {
            if (sourceResult.ok) setSources(readList<SourceDefinition>(sourceResult, 'sources'))
            if (destinationResult.ok) setDestinations(readList<DestinationDefinition>(destinationResult, 'destinations'))
            if (!sourceResult.ok || !destinationResult.ok) setWorkspaceError('推送任务关联配置加载不完整，已保留上一次成功数据。')
          }
        } else if (activeNav === 'push_policy') {
          const [sourceResult, ruleResult, destinationResult, taskResult, orderPushSkipResult] = await Promise.all([get('/v1/sources'), get('/v1/transform-rules'), get('/v1/destinations'), get('/v1/delivery-tasks'), get('/v1/order-push-skip-config')])
          if (!controller.signal.aborted) {
            if (sourceResult.ok) setSources(readList<SourceDefinition>(sourceResult, 'sources'))
            if (ruleResult.ok) setTransformRules(readList<TransformRule>(ruleResult, 'rules'))
            if (destinationResult.ok) setDestinations(readList<DestinationDefinition>(destinationResult, 'destinations'))
            if (taskResult.ok) setDeliveryTasks(readList<DeliveryTask>(taskResult, 'tasks'))
            if (orderPushSkipResult.ok) {
              setOrderPushSkipConfig(normalizeOrderPushSkipConfig(readObject<OrderPushSkipConfig>(orderPushSkipResult, 'config')))
              setOrderPushTargets(readList<OrderPushTargetOption>(orderPushSkipResult, 'targets'))
            }
            if (!sourceResult.ok || !ruleResult.ok || !destinationResult.ok || !taskResult.ok || !orderPushSkipResult.ok) setWorkspaceError('推送策略配置加载不完整，已保留上一次成功数据。')
          }
        } else if (activeNav === 'youzan_distribution') {
          const legacyTaskResult = await get('/v1/legacy-tasks')
          if (!controller.signal.aborted && legacyTaskResult.ok) setLegacyTasks(readList<LegacyTask>(legacyTaskResult, 'tasks'))
        } else if (activeNav === 'methods') {
          const [pipelineResult, sourceResult, ruleResult, destinationResult, taskResult, legacyTaskResult, legacyRuleResult] = await Promise.all([
            get('/v1/pipelines'), get('/v1/sources'), get('/v1/transform-rules'), get('/v1/destinations'), get('/v1/delivery-tasks'), get('/v1/legacy-tasks'), get('/v1/legacy-transform-rules'),
          ])
          if (!controller.signal.aborted) {
            const nextSources = sourceResult.ok ? readList<SourceDefinition>(sourceResult, 'sources') : []
            const nextRules = ruleResult.ok ? readList<TransformRule>(ruleResult, 'rules') : []
            const nextDestinations = destinationResult.ok ? readList<DestinationDefinition>(destinationResult, 'destinations') : []
            const nextTasks = taskResult.ok ? readList<DeliveryTask>(taskResult, 'tasks') : []
            const nextLegacyTasks = legacyTaskResult.ok ? readList<LegacyTask>(legacyTaskResult, 'tasks') : []
            const nextLegacyRules = legacyRuleResult.ok ? readList<LegacyTransformRule>(legacyRuleResult, 'rules') : []
            if (sourceResult.ok) setSources(nextSources)
            if (ruleResult.ok) setTransformRules(nextRules)
            if (destinationResult.ok) setDestinations(nextDestinations)
            if (taskResult.ok) setDeliveryTasks(nextTasks)
            if (legacyTaskResult.ok) setLegacyTasks(nextLegacyTasks)
            if (legacyRuleResult.ok) setLegacyRules(nextLegacyRules)
            const nextPipelines = pipelineResult.ok ? readList<PipelineDefinition>(pipelineResult, 'pipelines') : []
            if (pipelineResult.ok) setPipelines(nextPipelines)
            const configuredMethods = pipelineResult.ok ? await loadConfiguredMethods(nextPipelines, controller.signal) : []
            if (!controller.signal.aborted) {
              if (!pipelineResult.ok || !sourceResult.ok || !ruleResult.ok || !destinationResult.ok || !taskResult.ok || !legacyTaskResult.ok || !legacyRuleResult.ok) setWorkspaceError('方法目录加载不完整，已保留上一次成功数据。')
              setMethods([...buildConfiguredMethodDisplays(nextSources, nextRules, nextDestinations, nextTasks), ...buildLegacyMethodDisplays(nextLegacyTasks, nextLegacyRules), ...configuredMethods, ...builtinMethods])
            }
          }
        }
        if (!controller.signal.aborted && showResult) setResult({ ok: true, status: 200, data: { refreshed_at: new Date().toISOString() } })
      } finally {
        if (workspaceRequestRef.current === controller) {
          workspaceRequestRef.current = null
          setRefreshing(false)
          if (!controller.signal.aborted) setWorkspaceRefreshVersion((version) => version + 1)
        }
      }
    },
    [activeNav, client, loadConfiguredMethods, token],
  )

  useEffect(() => {
    if (sessionState === 'authenticated') void refreshWorkspace(false)
    return () => workspaceRequestRef.current?.abort()
  }, [refreshWorkspace, sessionState])

  useEffect(() => {
    if (!token) return
    if (sessionExpiresAt === null || sessionExpiresAt <= Date.now()) {
      clearSession()
      return
    }

    let timer = 0
    const scheduleRefresh = () => {
      const remaining = sessionExpiresAt - Date.now()
      if (remaining <= 0) {
        clearSession()
        return
      }
      const refreshDelay = Math.max(0, remaining - 60_000)
      timer = window.setTimeout(() => {
        void apiClient.refresh().then((refreshed) => {
          if (!refreshed) clearSession()
        })
      }, Math.min(refreshDelay, 2_147_000_000))
    }
    scheduleRefresh()
    return () => window.clearTimeout(timer)
  }, [apiClient, clearSession, sessionExpiresAt, token])

  useEffect(() => {
    authenticatedSessionRef.current = sessionState === 'authenticated'
  }, [sessionState])

  useEffect(() => {
    if (!token) return
    let current = true
    const controller = new AbortController()
    const preserveAuthenticatedSession = authenticatedSessionRef.current
    if (!preserveAuthenticatedSession) setSessionState('checking')
    void Promise.all([
      apiClient.request('/auth/me', { method: 'GET', signal: controller.signal }),
      apiClient.request('/auth/token/info', { method: 'GET', signal: controller.signal }),
    ]).then(([profileResponse, tokenInfoResponse]) => {
      if (!current) return
      const verification = verifySessionResponses(profileResponse, tokenInfoResponse)
      if (verification.kind !== 'valid') {
        if (verification.kind === 'unauthorized' || verification.kind === 'invalid') {
          clearSession()
          return
        }
        if (preserveAuthenticatedSession) {
          setSessionValidationError('会话校验暂不可用，当前数据可能已过期。')
        } else {
          clearSession()
        }
        return
      }
      const { user, tokenInfo } = verification
      const storedUser: StoredSessionUser = { id: user.id, account: user.account, nickname: user.nickname }
      saveStoredSessionUser(storedUser, window.localStorage)
      saveStoredTokenExpiry(tokenInfo.expireTime * 1000, window.localStorage)
      setSessionUser(user)
      setSessionExpiresAt(tokenInfo.expireTime * 1000)
      setSessionValidationError('')
      setSessionState('authenticated')
    })
    return () => {
      current = false
      controller.abort()
    }
  }, [apiClient, clearSession, sessionValidationAttempt, token])

  useEffect(() => {
    if (!mobileNavOpen) return
    const previousOverflow = document.body.style.overflow
    const navigation = mobileNavRef.current
    document.body.style.overflow = 'hidden'
    const focusableSelector = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    const closeNavigation = () => setMobileNavOpen(false)
    const handleKeydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        closeNavigation()
        return
      }
      if (event.key !== 'Tab') return
      const items = Array.from(navigation?.querySelectorAll<HTMLElement>(focusableSelector) ?? []).filter((item) => !item.hasAttribute('hidden'))
      if (items.length === 0) return
      const first = items[0]
      const last = items[items.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    window.addEventListener('keydown', handleKeydown)
    const firstFocusable = navigation?.querySelector<HTMLElement>(focusableSelector)
    const mobileNavTrigger = mobileNavTriggerRef.current
    firstFocusable?.focus()
    return () => {
      document.body.style.overflow = previousOverflow
      window.removeEventListener('keydown', handleKeydown)
      if (window.matchMedia('(max-width: 840px)').matches) {
        mobileNavTrigger?.focus()
      } else {
        const desktopNavigationTarget = navigation?.querySelector<HTMLElement>('.nav-item.active')
          ?? navigation?.querySelector<HTMLElement>(focusableSelector)
        desktopNavigationTarget?.focus()
      }
    }
  }, [mobileNavOpen])

  useLayoutEffect(() => {
    const mobileViewport = window.matchMedia('(max-width: 840px)')
    const syncMobileNavigationAccessibility = () => {
      const navigation = mobileNavRef.current
      if (!navigation) return
      const shouldHideNavigation = mobileViewport.matches && !mobileNavOpen
      if (shouldHideNavigation && navigation.contains(document.activeElement)) mobileNavTriggerRef.current?.focus()
      navigation.toggleAttribute('inert', shouldHideNavigation)
      if (shouldHideNavigation) navigation.setAttribute('aria-hidden', 'true')
      else navigation.removeAttribute('aria-hidden')
      if (!mobileViewport.matches) setMobileNavOpen(false)
    }

    syncMobileNavigationAccessibility()
    mobileViewport.addEventListener('change', syncMobileNavigationAccessibility)
    return () => mobileViewport.removeEventListener('change', syncMobileNavigationAccessibility)
  }, [mobileNavOpen])

  useEffect(() => {
    const handleHashChange = () => {
      const nextNav = navFromHash()
      setActiveNav(nextNav)
      setExpandedNavGroup(navGroupFor(nextNav)?.label ?? navGroups[0].label)
    }
    window.addEventListener('hashchange', handleHashChange)
    return () => window.removeEventListener('hashchange', handleHashChange)
  }, [])

  useEffect(() => {
    if (sessionState !== 'authenticated' || activeNav !== 'overview') return
    const controller = new AbortController()
    void Promise.all([
      client('/v1/data/statistics', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }),
      client('/v1/mall-weather/metrics', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }),
      client('/health', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true, acceptBareJSONSuccess: true }),
    ]).then(([statisticsResponse, weatherResponse, healthResponse]) => {
      if (controller.signal.aborted) return
      const nextStatistics = statisticsResponse.ok ? parseDataStatisticsSummary(statisticsResponse.data) : null
      const nextWeather = weatherResponse.ok ? parseMallWeatherMetricsSummary(weatherResponse.data) : null
      const nextHealth = healthResponse.ok ? parseHealthSummary(healthResponse.data) : null
      setMonitoring((current) => ({
        statistics: nextStatistics ?? current.statistics,
        weather: nextWeather ?? current.weather,
        health: nextHealth ?? current.health,
      }))
      setMonitoringStale(!nextStatistics || !nextWeather || !nextHealth)
    })
    return () => controller.abort()
  }, [activeNav, client, sessionState])

  function navigate(key: NavKey) {
    window.location.hash = key
    setActiveNav(key)
    setMobileNavOpen(false)
  }

  function openMobileNavigation() {
    setMobileNavOpen(true)
  }

  function handleLogin(nextToken: string) {
    updateSessionToken(nextToken)
    setSessionState('checking')
  }

  function handleLogout() {
    clearSession()
  }

  function openStepRuns(runID: number) {
    setStepRunFocusID(runID)
    navigate('step_runs')
  }

  async function toggleTarget(target: ToggleTarget, enabled: boolean) {
    const response = await updateTargetEnabled(client, target, enabled, { sources, transformRules, destinations, deliveryTasks })
    if (response.ok) await refreshWorkspace(false)
  }

  async function retryDeliveryLog(logId: number) {
    const response = await client(`/v1/delivery-logs/${logId}/retry`, { method: 'POST' })
    if (response.ok) await refreshWorkspace(false)
  }

  async function fetchSource(sourceID: number) {
    const response = await client(`/v1/sources/${sourceID}/fetch`, { method: 'POST' })
    if (response.ok) await refreshWorkspace(false)
    return response
  }

  async function testSource(sourceID: number) {
    return client(`/v1/sources/${sourceID}/test`, { method: 'POST' })
  }

  async function saveOrderPushSkipConfig(config: OrderPushSkipConfig) {
    const response = await client('/v1/order-push-skip-config', {
      method: 'PUT',
      body: config,
    })
    if (response.ok) {
      setOrderPushSkipConfig(normalizeOrderPushSkipConfig(readObject<OrderPushSkipConfig>(response, 'config')) ?? config)
      await refreshWorkspace(false)
    }
    return response
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
      await refreshWorkspace(false)
      return readObject<BojunOrderBackfillResult>(response, 'result')
    }
    return null
  }

  async function previewYouzanDistributionBackfill(payload: YouzanDistributionBackfillPayload) {
    const response = await client('/v1/youzan-distribution-order-backfill/preview', {
      method: 'POST',
      body: payload,
    })
    return response.ok ? readObject<YouzanDistributionBackfillResult>(response, 'result') : null
  }

  async function confirmYouzanDistributionBackfill(payload: YouzanDistributionBackfillPayload) {
    const response = await client('/v1/youzan-distribution-order-backfill/confirm', {
      method: 'POST',
      body: payload,
    })
    return response.ok ? readObject<YouzanDistributionBackfillResult>(response, 'result') : null
  }

  async function runLegacyTask(code: string, payload: YouzanDistributionBackfillPayload) {
    const response = await client(`/v1/legacy-tasks/${encodeURIComponent(code)}/run`, {
      method: 'POST',
      body: payload,
    })
    if (response.ok) await refreshWorkspace(false)
    return response
  }

  const coreMethods = useMemo(
    () => buildCoreMethods({ sources, transformRules, destinations, deliveryTasks, legacyTasks, legacyRules }),
    [deliveryTasks, destinations, legacyRules, legacyTasks, sources, transformRules],
  )

  if (sessionState !== 'authenticated') return <LoginScreen onLogin={handleLogin} checking={sessionState === 'checking'} />

  return (
    <main className={activeNav === 'mall_weather' || activeNav === 'store_info' ? 'ops-shell mall-weather-shell' : 'ops-shell'}>
      {mobileNavOpen && <button className="mobile-nav-backdrop" type="button" aria-label="关闭导航抽屉" onClick={() => setMobileNavOpen(false)} />}
      <aside ref={mobileNavRef} className={mobileNavOpen ? 'ops-sidebar mobile-open' : 'ops-sidebar'} aria-label="主导航">
        <Brand />
        <button
          className="mobile-nav-toggle"
          type="button"
          aria-expanded={mobileNavOpen}
          aria-controls="primary-navigation"
          onClick={() => setMobileNavOpen((open) => !open)}
        >
          <X aria-hidden="true" />
          关闭菜单
        </button>
        <label className="nav-search">
          <span>查找页面</span>
          <div>
            <Search aria-hidden="true" />
            <input
              name="moduleNavigationSearch"
              value={navQuery}
              onChange={(event) => setNavQuery(event.currentTarget.value)}
              placeholder="输入页面名称或用途"
            />
          </div>
        </label>
        <nav className="module-nav" id="primary-navigation">
          {navGroups.map((group) => {
            const query = navQuery.trim().toLowerCase()
            const items = group.items.filter((item) => !query || `${item.label} ${item.description}`.toLowerCase().includes(query))
            if (items.length === 0) return null
            const expanded = Boolean(query) || window.matchMedia('(min-width: 841px)').matches || expandedNavGroup === group.label
            const panelID = `nav-group-${group.items[0].key}`
            return (
              <section className="nav-group" key={group.label}>
                <h2>
                  <button
                    className="nav-group-toggle"
                    type="button"
                    aria-expanded={expanded}
                    aria-controls={panelID}
                    onClick={() => setExpandedNavGroup((current) => current === group.label ? '' : group.label)}
                  >
                    <span>{group.label}</span>
                    <ChevronDown aria-hidden="true" />
                  </button>
                </h2>
                {expanded && (
                  <div className="nav-group-items" id={panelID}>
                    {items.map((item) => (
                      <button className={item.key === activeNav ? 'nav-item active' : 'nav-item'} key={item.key} type="button" onClick={() => navigate(item.key)}>
                        {item.icon}
                        <span><strong>{item.label}</strong><small>{item.description}</small></span>
                      </button>
                    ))}
                  </div>
                )}
              </section>
            )
          })}
        </nav>
        <div className="sidebar-actions">
          <button type="button" onClick={() => void refreshWorkspace(true)} disabled={refreshing}>
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
        <ModuleHeader activeNav={activeNav} loading={loading || refreshing} sessionUser={sessionUser} onOpenNavigation={openMobileNavigation} onRefresh={() => void refreshWorkspace(true)} refreshing={refreshing} mobileNavTriggerRef={mobileNavTriggerRef} />
        {sessionValidationError && <div className="result-banner error" role="status" aria-live="polite">{sessionValidationError} <button type="button" onClick={() => setSessionValidationAttempt((attempt) => attempt + 1)}>重试校验</button></div>}
        {workspaceError && <div className="result-banner error" role="alert">{workspaceError} <button type="button" onClick={() => void refreshWorkspace(false)} disabled={refreshing}>重试</button></div>}
        {activeNav === 'overview' && <PushStatusView runs={runs} deliveryLogs={deliveryLogs} monitoring={monitoring} stale={monitoringStale} overviewTotals={overviewTotals} onLoadSteps={openStepRuns} />}
        {activeNav === 'runs' && <RunsQueryPage client={client} pipelines={pipelines} onLoadSteps={openStepRuns} onPipelineRunCompleted={() => void refreshWorkspace(false)} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'delivery_logs' && <DeliveryLogsQueryPage client={client} onRetryLog={retryDeliveryLog} />}
        {activeNav === 'step_runs' && <StepRunsQueryPage client={client} focusRunID={stepRunFocusID} />}
        {activeNav === 'store_info' && <StoreInfoPage actorID={actorID} client={client} downloadFile={downloadFile} />}
        {activeNav === 'mall_weather' && <MallWeatherPage actorID={actorID} client={client} downloadFile={downloadFile} />}
        {activeNav === 'data_authorizations' && <DataAuthorizationPage client={client} />}
        {activeNav === 'sources' && <SourcesQueryPage client={client} onFetchSource={fetchSource} onTestSource={testSource} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'methods' && <MethodsView methods={methods} pipelines={pipelines} client={client} coreMethods={coreMethods} onToggle={toggleTarget} onPipelineRunCompleted={() => void refreshWorkspace(false)} />}
        {activeNav === 'receive' && <RawRecordsQueryPage title="接口接收记录" origin="receive" client={client} onFetchSource={fetchSource} />}
        {activeNav === 'pull_records' && <RawRecordsQueryPage title="数据拉取记录" origin="pull" client={client} onFetchSource={fetchSource} />}
        {activeNav === 'backfill' && <BojunBackfillPage loading={loading || refreshing} onPreview={previewBojunOrderBackfill} onConfirm={confirmBojunOrderBackfill} />}
        {activeNav === 'youzan_distribution' && <YouzanDistributionPage task={legacyTasks.find((item) => item.code === 'youzan_distribution_order_fetch')} loading={loading || refreshing} onPreview={previewYouzanDistributionBackfill} onConfirm={confirmYouzanDistributionBackfill} onRun={runLegacyTask} />}
        {activeNav === 'rules' && <RulesQueryPage client={client} rules={transformRules} sources={sources} onRulesChange={setTransformRules} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'processed' && <ProcessedQueryPage client={client} />}
        {activeNav === 'destinations' && <DestinationsQueryPage client={client} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'tasks' && <DeliveryTasksQueryPage client={client} sources={sources} destinations={destinations} onRefresh={() => refreshWorkspace(false)} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'push_policy' && <PushPolicyPage coreMethod={coreMethods.find((item) => item.key === 'mall_push')} config={orderPushSkipConfig} targets={orderPushTargets} onSave={saveOrderPushSkipConfig} onToggle={toggleTarget} />}
        {(activeNav === 'excel_jobs' || activeNav === 'excel_schemes' || activeNav === 'excel_write') && <ExcelMatchView section={activeNav === 'excel_jobs' ? 'jobs' : activeNav === 'excel_schemes' ? 'schemes' : 'write'} client={client} token={token} loading={loading} setLoading={setLoading} setResult={setResult} onNavigateToJobs={() => navigate('excel_jobs')} />}
      </section>

      <ResultPanel result={result} onClose={() => setResult(null)} />
    </main>
  )
}

function LoginScreen({ onLogin, checking }: { onLogin: (token: string) => void; checking: boolean }) {
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    const form = new FormData(event.currentTarget)
    try {
      const response = await fetch(apiURL('/auth/login'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: formValue(form, 'username'), password: formValue(form, 'password') }),
      })
      const data: unknown = await response.json().catch(() => ({}))
      const token = readToken(data)
      if (!response.ok || !token) {
        setError(loginFailureMessage(response.status))
        return
      }
      onLogin(token)
    } catch {
      setError('无法连接登录服务，请检查后端服务或代理配置。')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="login-shell">
      <section className="login-panel">
        <div className="login-title">
          <Brand size="large" />
        </div>
        <form className="login-form" onSubmit={submit}>
          <h1>管理员登录</h1>
          <Field label="管理员账号" name="username" required autoComplete="username" />
          <Field label="管理员密码" name="password" type="password" required autoComplete="current-password" />
          {error && <div className="login-error" role="alert" aria-live="polite">{error}</div>}
          <button className="primary" type="submit" disabled={submitting || checking}>{submitting || checking ? '正在验证会话…' : '管理员登录'}</button>
        </form>
      </section>
    </main>
  )
}

function ModuleHeader({ activeNav, loading, sessionUser, onOpenNavigation, onRefresh, refreshing, mobileNavTriggerRef }: { activeNav: NavKey; loading: boolean; sessionUser: SessionUser | null; onOpenNavigation: () => void; onRefresh: () => void; refreshing: boolean; mobileNavTriggerRef: RefObject<HTMLButtonElement> }) {
  const titles: Record<NavKey, { title: string; subtitle: string }> = {
    overview: { title: '运行总览', subtitle: '只看当前运行与交付健康度，快速定位失败。' },
    runs: { title: '流水线运行', subtitle: '按状态、运行类型和 Trace ID 查询执行记录。' },
    delivery_logs: { title: '推送日志', subtitle: '按成功状态、门店和业务键查询外部交付结果。' },
    step_runs: { title: '步骤运行', subtitle: '选择一次流水线运行并查看每个步骤的输入输出。' },
    store_info: { title: '店铺信息', subtitle: '统一维护店铺资料、地址与天气服务坐标。' },
    mall_weather: { title: '商场天气', subtitle: '查看商场中心点实况、未来降水、小时趋势和气象预警。' },
    data_authorizations: { title: '数据授权', subtitle: '由管理员开通开放接口账号，并管理权限有效期、凭证与审计。' },
    sources: { title: '数据源', subtitle: '查询数据接入配置、类型和启用状态。' },
    receive: { title: '接口接收', subtitle: '查询外部系统主动推送进来的原始数据。' },
    pull_records: { title: '拉取记录', subtitle: '查询系统主动从外部接口拉取的数据。' },
    backfill: { title: '伯俊补拉', subtitle: '先预览、再确认写入指定时间范围的伯俊订单。' },
    youzan_distribution: { title: '有赞分销订单', subtitle: '查看每日自动任务，并按时间范围提交异步补拉。' },
    rules: { title: '清洗规则', subtitle: '查询规则类型、来源、顺序和启用状态。' },
    processed: { title: '处理结果', subtitle: '按业务键、类型和质量状态查询处理后数据。' },
    methods: { title: '方法目录', subtitle: '查询已配置方法和系统内置能力。' },
    destinations: { title: '推送目标', subtitle: '查询目标系统和接口配置。' },
    tasks: { title: '推送任务', subtitle: '查询交付任务、触发方式和目标关系。' },
    push_policy: { title: '推送策略', subtitle: '配置各具体推送目标的订单跳过周期。' },
    excel_jobs: { title: 'Excel 任务', subtitle: '查询任务状态、进度、日志和下载结果。' },
    excel_schemes: { title: 'Excel 多步骤匹配', subtitle: '配置数据库表、字段和顺序匹配步骤。' },
    excel_write: { title: 'Excel 写入', subtitle: '执行导入更新与退回未匹配操作。' },
  }
  return (
    <header className="workspace-header">
      <div>
        <button ref={mobileNavTriggerRef} className="workspace-menu-button" type="button" aria-label="打开主导航" onClick={onOpenNavigation}>
          <Menu aria-hidden="true" />
        </button>
        <h2>{titles[activeNav].title}</h2>
        <span>{titles[activeNav].subtitle}</span>
      </div>
      <div className="workspace-session">
        {activeNav !== 'store_info' && <span className="workspace-date"><CalendarDays aria-hidden="true" />{new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'Asia/Shanghai' }).format(new Date())}</span>}
        {sessionUser && <span className="workspace-user">{sessionUser.nickname || sessionUser.account}</span>}
        <button className="workspace-refresh" type="button" onClick={onRefresh} disabled={refreshing}><RefreshCcw aria-hidden="true" />{refreshing ? '刷新中' : '刷新数据'}</button>
        <span className={loading ? 'workspace-health is-loading' : 'workspace-health'}><i aria-hidden="true" />{loading ? '数据加载中' : '系统正常'}</span>
      </div>
    </header>
  )
}

function PushStatusView({ runs, deliveryLogs, monitoring, stale, overviewTotals, onLoadSteps }: { runs: PipelineRun[]; deliveryLogs: DeliveryLog[]; monitoring: MonitoringSnapshot; stale: boolean; overviewTotals: { runs: number | null; deliveryLogs: number | null }; onLoadSteps: (runId: number) => void }) {
  const failedLogs = deliveryLogs.filter((log) => !log.success)
  const runningRuns = runs.filter((run) => run.status === 'running')
  const loadedRunTotal = sum(runs, 'success_count') + sum(runs, 'failed_count')
  const successRate = loadedRunTotal > 0 ? sum(runs, 'success_count') / loadedRunTotal : null
  const delivered = deliveryLogs.filter((log) => log.success).length
  const deliveryRate = deliveryLogs.length > 0 ? delivered / deliveryLogs.length : null
  const healthTotal = delivered + failedLogs.length
  return (
    <div className="view-stack">
      <section className="overview-grid">
        <Metric label="今日运行" value={overviewTotals.runs ?? runs.length} />
        <Metric label="已加载运行成功率" value={successRate === null ? '-' : `${(successRate * 100).toFixed(1)}%`} />
        <Metric label="待处理运行" value={runningRuns.length} />
        <Metric label="失败交付记录" value={overviewTotals.deliveryLogs === null ? failedLogs.length : `${failedLogs.length} / ${overviewTotals.deliveryLogs}`} />
      </section>
      {stale && <p className="backfill-note" role="status">部分统计暂时不可用，已保留最近一次成功数据。</p>}
      <section className="overview-workspace">
        <Panel title="最近流水线运行" icon={<Activity />} meta={`今日已加载 ${runs.length} 条`}>
          <OverviewRunTable runs={runs} onLoadSteps={onLoadSteps} />
        </Panel>
        <aside className="overview-monitoring" aria-label="交付健康度与最近异常">
          <section className="overview-monitoring-section">
            <h3>交付健康度</h3>
            <progress className="overview-health-progress" value={healthTotal ? delivered : 0} max={Math.max(healthTotal, 1)} aria-label="已加载交付成功率" />
            <div className="overview-health-legend"><span className="success">推送成功 {delivered}</span><span className="danger">推送失败 {failedLogs.length}</span></div>
            <small>{deliveryRate === null ? '今日暂无已加载交付记录。' : `已加载交付成功率 ${(deliveryRate * 100).toFixed(1)}%`}</small>
          </section>
          <section className="overview-monitoring-section">
            <h3>最近异常</h3>
            {failedLogs.length === 0 && monitoring.weather?.firingAlerts === 0 ? <EmptyState text="暂无已加载异常。" /> : <div className="overview-anomaly-list">
              {failedLogs.slice(0, 4).map((log) => <article className="overview-anomaly" key={log.id}><strong>{log.destination_name || log.destination_code || `目标 #${log.destination_id}`} 推送失败</strong><span>{formatDate(log.sent_at)} / HTTP {log.http_status || '-'}</span></article>)}
              {(monitoring.weather?.firingAlerts ?? 0) > 0 && <article className="overview-anomaly"><strong>天气服务告警</strong><span>当前触发 {monitoring.weather?.firingAlerts} 条告警</span></article>}
            </div>}
          </section>
          <section className="overview-monitoring-section overview-service-summary"><span>服务状态</span><strong>{monitoring.health?.healthy ? '系统正常' : '状态未知'}</strong><small>接收 {monitoring.statistics?.totalCount ?? '-'} / 已处理 {monitoring.statistics?.processedCount ?? '-'} / 处理失败 {monitoring.statistics?.errorCount ?? '-'}</small></section>
        </aside>
      </section>
    </div>
  )
}

function OverviewRunTable({ runs, onLoadSteps }: { runs: PipelineRun[]; onLoadSteps: (runId: number) => void }) {
  if (runs.length === 0) return <EmptyState text="今日暂无运行记录。" />
  return <div className="data-table-wrap"><table className="data-table overview-run-table"><thead><tr><th>运行任务</th><th>来源</th><th>状态</th><th>处理进度</th><th>开始时间</th><th>操作</th></tr></thead><tbody>{runs.slice(0, 12).map((run) => {
    const completed = run.success_count + run.failed_count
    const progress = run.total_count > 0 ? Math.min(100, Math.round(completed / run.total_count * 100)) : run.status === 'success' ? 100 : 0
    return <tr key={run.id}><td><strong>#{run.id} {run.run_type}</strong><small>{run.trace_id || '-'}</small></td><td>{run.trigger_type || '-'}</td><td><StatusPill label={run.status} /></td><td><div className="overview-run-progress"><span>{completed} / {run.total_count} ({progress}%)</span><progress value={progress} max="100" aria-label={`运行 #${run.id} 处理进度`} /></div></td><td>{formatDate(run.started_at)}</td><td><button type="button" onClick={() => onLoadSteps(run.id)}>步骤</button></td></tr>
  })}</tbody></table></div>
}

function MethodsView({ methods, pipelines, client, coreMethods, onToggle, onPipelineRunCompleted }: { methods: MethodDisplay[]; pipelines: PipelineDefinition[]; client: ApiClient; coreMethods: CoreMethod[]; onToggle: (target: ToggleTarget, enabled: boolean) => void; onPipelineRunCompleted: () => void }) {
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('all')
  const [status, setStatus] = useState('all')
  const filtered = methods.filter((method) => includesQuery([method.name, method.code, method.description, method.owner], query)
    && (category === 'all' || method.category === category)
    && (status === 'all' || (status === 'enabled' ? method.enabled : !method.enabled)))
  const featuredCoreMethods = coreMethods.filter((method) => ['youzan_fetch', 'qimai_process', 'mall_push'].includes(method.key))
  return (
    <div className="view-stack design-data-page design-methods-page">
      <section className="overview-grid design-method-summary">
        <Metric label="已配置方法" value={methods.filter((item) => item.kind === 'configured').length} />
        <Metric label="内置方法" value={methods.filter((item) => item.kind === 'builtin').length} />
        <Metric label="启用方法" value={methods.filter((item) => item.enabled).length} />
        <Metric label="方法类型" value={new Set(methods.map((item) => item.method_type)).size} />
      </section>
      <section className="design-core-methods">
        <div className="design-section-heading"><div><h3>当前已有核心方法</h3><span>{featuredCoreMethods.length} 项核心能力</span></div></div>
        <CoreMethodList methods={featuredCoreMethods} onToggle={onToggle} />
      </section>
      <section className="query-bar design-filter-bar design-method-filter" aria-label="查询条件">
        <div className="query-fields">
          <label>名称 / 编码 / 负责人<span className="design-search-control trailing"><input name="method_query" type="search" value={query} onChange={(event) => setQuery(event.currentTarget.value)} placeholder="搜索方法名称、编码或负责人" /><Search aria-hidden="true" /></span></label>
          <SelectFilter label="分类" value={category} onChange={setCategory} options={uniqueOptions(methods.map((method) => method.category))} />
          <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
        </div>
        <div className="design-filter-count"><strong>{filtered.length}</strong><span>/ {methods.length} 条</span></div>
      </section>
      <section className="design-table-section design-method-catalog">
        <div className="design-section-heading"><div><h3>方法目录</h3><span>展示当前可用方法与配置来源</span></div></div>
        <MethodCatalogTable methods={filtered} onToggle={onToggle} />
      </section>
      <details className="design-method-advanced"><summary>流水线方法高级配置</summary><PipelineComposerPanel pipelines={pipelines} client={client} onRefresh={onPipelineRunCompleted} /></details>
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

function useConfigurationListPage<T>(client: ApiClient, path: string, key: string, query: string, reloadVersion: number) {
  const [recordsPage, setRecordsPage] = useState<MonitoringPage<T> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const requestRef = useRef<AbortController | null>(null)

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    void client(`${path}?${query}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }).then((response) => {
      if (controller.signal.aborted) return
      const parsed = response.ok ? parseMonitoringPage<T>(response.data, key) : null
      if (parsed) {
        setRecordsPage(parsed)
        return
      }
      const legacyItems = readDataField(response.data, key)
      if (response.ok && Array.isArray(legacyItems)) {
        const pageSize = 20
        setRecordsPage({ list: legacyItems.slice(0, pageSize) as T[], pagination: { page: 1, pageSize, total: legacyItems.length, totalPages: legacyItems.length ? 1 : 0 } })
        setError('当前服务暂不支持配置列表分页或筛选，已显示未筛选的兼容数据。')
        return
      }
      setError(response.error?.message || '配置列表暂时不可用，请稍后重试。')
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false)
    })
    return () => controller.abort()
  }, [client, key, path, query, reloadVersion])

  return { recordsPage, loading, error }
}

function RunsQueryPage({ client, pipelines, onLoadSteps, onPipelineRunCompleted, refreshVersion }: { client: ApiClient; pipelines: PipelineDefinition[]; onLoadSteps: (runId: number) => void; onPipelineRunCompleted: () => void; refreshVersion: number }) {
  const [traceID, setTraceID] = useState('')
  const [status, setStatus] = useState('all')
  const [runType, setRunType] = useState('all')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [applied, setApplied] = useState({ traceID: '', status: '', runType: '', startTime: '', endTime: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<MonitoringPage<PipelineRun> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const requestRef = useRef<AbortController | null>(null)

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    const query = buildRunListQuery({ page, pageSize: 20, ...applied })
    void client(`/v1/runs?${query}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }).then((response) => {
      if (controller.signal.aborted) return
      const parsed = response.ok ? parseMonitoringPage<unknown>(response.data, 'runs') : null
      const runs = parsed?.list.map(parsePipelineRun) ?? []
      if (parsed && runs.every((run): run is PipelineRun => run !== null)) {
        setRecordsPage({ ...parsed, list: runs })
        return
      }
      setError(response.error?.message || '运行记录查询暂时不可用，请稍后重试。')
    }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [applied, client, page, refreshVersion])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setApplied({ traceID, status: status === 'all' ? '' : status, runType: runType === 'all' ? '' : runType, startTime, endTime })
  }

  const runs = recordsPage?.list ?? []
  const pagination = recordsPage?.pagination
  return (
    <div className="view-stack">
      <PipelineRunPanel pipelines={pipelines} client={client} onRunCompleted={onPipelineRunCompleted} />
      <form className="query-bar" onSubmit={submit}>
        <div className="query-fields">
          <Field label="Trace ID" name="run_trace_id" value={traceID} onChange={setTraceID} />
          <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'running', label: '运行中' }, { value: 'success', label: '成功' }, { value: 'failed', label: '失败' }, { value: 'partial_success', label: '部分成功' }]} />
          <SelectFilter label="运行类型" value={runType} onChange={setRunType} options={[{ value: 'fetch', label: '拉取' }, { value: 'ingest', label: '接收' }, { value: 'transform', label: '清洗' }, { value: 'delivery', label: '推送' }]} />
          <Field label="开始时间" name="run_start_time" type="datetime-local" value={startTime} onChange={setStartTime} />
          <Field label="结束时间" name="run_end_time" type="datetime-local" value={endTime} onChange={setEndTime} />
        </div>
        <button type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
      {error && <div className="result-banner error" role="alert">{error} 已保留最近一次成功数据。</div>}
      <Panel title="运行记录" icon={<Activity />} meta={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}>
        <RunTable runs={runs} onLoadSteps={onLoadSteps} />
        <MonitoringPaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} />
      </Panel>
    </div>
  )
}

function DeliveryLogsQueryPage({ client, onRetryLog }: { client: ApiClient; onRetryLog: (logId: number) => Promise<void> }) {
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

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setApplied({ destination, source, success: status === 'success' ? 'true' : status === 'failed' ? 'false' : '', businessKey, startTime, endTime })
  }

  async function retryPendingLog() {
    if (!pendingRetryLog || retryingLogID !== null) return
    setRetryingLogID(pendingRetryLog.id)
    try {
      await onRetryLog(pendingRetryLog.id)
    } finally {
      setRetryingLogID(null)
      setPendingRetryLog(null)
      setReloadVersion((version) => version + 1)
    }
  }

  const logs = recordsPage?.list ?? []
  const pagination = recordsPage?.pagination

  return (
    <div className="view-stack">
      <form className="query-bar" onSubmit={submit}>
        <div className="query-fields">
        <Field label="推送目标编码" name="delivery_destination" value={destination} onChange={setDestination} />
        <Field label="来源编码" name="delivery_source" value={source} onChange={setSource} />
        <SelectFilter label="交付状态" value={status} onChange={setStatus} options={[{ value: 'success', label: '成功' }, { value: 'failed', label: '失败' }]} />
        <Field label="业务键" name="delivery_business_key" value={businessKey} onChange={setBusinessKey} />
        <Field label="开始时间" name="delivery_start_time" type="datetime-local" value={startTime} onChange={setStartTime} />
        <Field label="结束时间" name="delivery_end_time" type="datetime-local" value={endTime} onChange={setEndTime} />
        </div>
        <button type="submit" disabled={loading || retryingLogID !== null}>{loading ? '查询中…' : '查询'}</button>
      </form>
      {error && <div className="result-banner error" role="alert">{error} 已保留最近一次成功数据。</div>}
      <Panel title="推送日志" icon={<Send />} meta={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}><DeliveryLogList logs={logs} retryingLogID={retryingLogID} onRetryLog={(log) => {
        if (!log.success && retryingLogID === null) setPendingRetryLog(log)
      }} /><MonitoringPaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading || retryingLogID !== null} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} /></Panel>
      {pendingRetryLog && <Modal title="确认重试推送日志" closeDisabled={retryingLogID !== null} onClose={() => { if (retryingLogID === null) setPendingRetryLog(null) }} footer={<><button type="button" disabled={retryingLogID !== null} onClick={() => setPendingRetryLog(null)}>取消</button><button className="primary" type="button" disabled={retryingLogID !== null} onClick={() => void retryPendingLog()}>{retryingLogID === pendingRetryLog.id ? '重试中…' : '确认重试'}</button></>}><p>确认重试失败日志 #{pendingRetryLog.id}？这会再次向原推送目标发起交付请求。</p></Modal>}
    </div>
  )
}

function StepRunsQueryPage({ client, focusRunID }: { client: ApiClient; focusRunID: number | null }) {
  const [runQuery, setRunQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [runType, setRunType] = useState('all')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [applied, setApplied] = useState({ traceID: '', status: '', runType: '', startTime: '', endTime: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<MonitoringPage<PipelineRun> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selectedRunID, setSelectedRunID] = useState<number | null>(null)
  const [stepRuns, setStepRuns] = useState<StepRun[]>([])
  const [stepLoading, setStepLoading] = useState(false)
  const [stepError, setStepError] = useState('')
  const [stepQuery, setStepQuery] = useState('')
  const [selectedStepID, setSelectedStepID] = useState<number | null>(null)
  const requestRef = useRef<AbortController | null>(null)
  const stepRequestRef = useRef<AbortController | null>(null)

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    const query = buildRunListQuery({ page, pageSize: 20, ...applied })
    void client(`/v1/runs?${query}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }).then((response) => {
      if (controller.signal.aborted) return
      const parsed = response.ok ? parseMonitoringPage<unknown>(response.data, 'runs') : null
      const runs = parsed?.list.map(parsePipelineRun) ?? []
      if (parsed && runs.every((run): run is PipelineRun => run !== null)) {
        setRecordsPage({ ...parsed, list: runs })
        return
      }
      setError(response.error?.message || '步骤运行查询暂时不可用，请稍后重试。')
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false)
    })
    return () => controller.abort()
  }, [applied, client, page])

  useEffect(() => () => stepRequestRef.current?.abort(), [])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setApplied({ traceID: runQuery, status: status === 'all' ? '' : status, runType: runType === 'all' ? '' : runType, startTime, endTime })
  }

  const selectRun = useCallback(async (runID: number) => {
    stepRequestRef.current?.abort()
    const controller = new AbortController()
    stepRequestRef.current = controller
    setSelectedRunID(runID)
    setStepRuns([])
    setSelectedStepID(null)
    setStepError('')
    setStepLoading(true)
    const response = await client(`/v1/pipeline-runs/${runID}/steps`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
    if (controller.signal.aborted) return
    if (!response.ok) {
      setStepError(response.error?.message || '步骤详情暂时不可用，请稍后重试。')
      setStepLoading(false)
      return
    }
    const rawStepRuns = readList<unknown>(response, 'step_runs')
    const parsedStepRuns = rawStepRuns.map(parseStepRun)
    if (!parsedStepRuns.every((step): step is StepRun => step !== null)) {
      setStepError('步骤详情返回格式无效，已拒绝展示。')
      setStepLoading(false)
      return
    }
    setStepRuns(parsedStepRuns)
    setStepLoading(false)
  }, [client])

  useEffect(() => {
    if (!focusRunID || selectedRunID === focusRunID || !recordsPage?.list.some((run) => run.id === focusRunID)) return
    void selectRun(focusRunID)
  }, [focusRunID, recordsPage, selectRun, selectedRunID])

  const runs = recordsPage?.list ?? []
  const pagination = recordsPage?.pagination
  const visibleRuns = runs.filter((run) => includesQuery([run.id, run.trace_id, run.run_type], runQuery))
  const visibleSteps = stepRuns.filter((step) => includesQuery([step.id, step.run_id, step.step_code, step.method_type, step.status, step.error_message], stepQuery))
  const selectedStep = visibleSteps.find((step) => step.id === selectedStepID) ?? visibleSteps[0] ?? null

  useEffect(() => {
    setSelectedStepID((current) => visibleSteps.some((step) => step.id === current) ? current : visibleSteps[0]?.id ?? null)
  }, [visibleSteps])

  return (
    <div className="view-stack">
      <form className="query-bar" onSubmit={submit}>
        <div className="query-fields">
          <Field label="运行 / Trace ID" name="step_run_query" value={runQuery} onChange={setRunQuery} />
          <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'running', label: '运行中' }, { value: 'success', label: '成功' }, { value: 'failed', label: '失败' }, { value: 'partial_success', label: '部分成功' }]} />
          <SelectFilter label="运行类型" value={runType} onChange={setRunType} options={[{ value: 'fetch', label: '拉取' }, { value: 'ingest', label: '接收' }, { value: 'transform', label: '清洗' }, { value: 'delivery', label: '推送' }]} />
          <Field label="开始时间" name="step_run_start_time" type="datetime-local" value={startTime} onChange={setStartTime} />
          <Field label="结束时间" name="step_run_end_time" type="datetime-local" value={endTime} onChange={setEndTime} />
        </div>
        <button type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
      {error && <div className="result-banner error" role="alert">{error}{recordsPage ? ' 已保留最近一次成功数据。' : ''}</div>}
      <div className="step-runs-layout">
        <section className="step-runs-column" aria-label="流水线运行">
          <div className="step-runs-column-heading"><strong>选择运行</strong><span>{loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}</span></div>
          {visibleRuns.length === 0 ? <EmptyState text={loading ? '正在加载运行记录…' : '暂无匹配运行。'} /> : <div className="step-runs-list" role="list">{visibleRuns.map((run) => {
            const selected = run.id === selectedRunID
            return <button className={`step-runs-run ${selected ? 'is-selected' : ''}`} type="button" key={run.id} aria-pressed={selected} onClick={() => void selectRun(run.id)}>
              <span><strong>#{run.id}</strong><small>{run.trace_id || '-'} · {formatDate(run.started_at)}</small></span><StatusPill label={pipelineRunStatusLabel(run.status)} />
            </button>
          })}</div>}
          <MonitoringPaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} />
        </section>
        <section className="step-runs-workspace" aria-label="运行步骤与详情">
          <div className="step-runs-column-heading"><strong>步骤时间线</strong><span>{selectedRunID ? `运行 #${selectedRunID}` : '请选择运行'}</span></div>
          <Field label="编码 / 类型 / 状态" name="step_query" value={stepQuery} onChange={setStepQuery} />
          {stepError && <div className="result-banner error" role="alert">{stepError}</div>}
          {visibleSteps.length === 0 ? <EmptyState text={stepLoading ? '正在加载步骤详情…' : selectedRunID ? '当前运行没有匹配步骤。' : '从左侧选择运行后加载步骤。'} /> : <div className="step-runs-list step-runs-timeline" role="list">{visibleSteps.map((step) => {
            const selected = step.id === selectedStep?.id
            return <button className={`step-runs-step ${selected ? 'is-selected' : ''}`} type="button" key={step.id} aria-pressed={selected} onClick={() => setSelectedStepID(step.id)}>
              <span><strong>{step.step_code || `步骤 #${step.id}`}</strong><small>{step.method_type || '-'} · {formatDate(step.started_at)} · {runDurationLabel(step.started_at, step.finished_at)}</small></span><StatusPill label={stepRunStatusLabel(step.status)} />
            </button>
          })}</div>}
          <aside className="step-runs-detail" aria-live="polite" aria-label="已选步骤详情">
            {selectedStep ? <>
              <div className="step-runs-column-heading"><div><strong>{selectedStep.step_code || `步骤 #${selectedStep.id}`}</strong><span>{selectedStep.method_type || '未声明方法类型'} · 运行 #{selectedStep.run_id}</span></div><StatusPill label={stepRunStatusLabel(selectedStep.status)} /></div>
              {selectedStep.error_message && <p className="step-runs-error" role="alert">该步骤执行失败，错误详情已受保护。</p>}
              <div className="step-runs-json-grid">
                <CopyableRedactedJSON label="输入（脱敏）" value={parseJsonText(selectedStep.input_json)} />
                <CopyableRedactedJSON label="输出（脱敏）" value={parseJsonText(selectedStep.output_json)} />
              </div>
            </> : <EmptyState text="选择步骤后查看安全详情。" />}
          </aside>
        </section>
      </div>
    </div>
  )
}

function SourcesQueryPage({ client, onFetchSource, onTestSource, refreshVersion }: { client: ApiClient; onFetchSource: (sourceID: number) => Promise<ApiResult>; onTestSource: (sourceID: number) => Promise<ApiResult>; refreshVersion: number }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [sourceType, setSourceType] = useState('')
  const [applied, setApplied] = useState({ keyword: '', enabled: '' as '' | 'true' | 'false', sourceType: '' })
  const [page, setPage] = useState(1)
  const [reloadVersion, setReloadVersion] = useState(0)
  const [draft, setDraft] = useState<SourceDraft | null>(null)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')
  const listQuery = useMemo(() => buildSourceListQuery({ page, pageSize: 20, ...applied }), [applied, page])
  const { recordsPage, loading, error } = useConfigurationListPage<SourceDefinition>(client, '/v1/sources', 'sources', listQuery, reloadVersion + refreshVersion)
  const listedSources = recordsPage?.list ?? []
  const pagination = recordsPage?.pagination

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

  async function openDetail(id: number) {
    setMessage('')
    const response = await client(`/v1/sources/${id}`, { method: 'GET', showResult: false, silentLoading: true })
    const source = response.ok ? readObject<SourceDefinition>(response, 'source') : null
    if (!source) { setMessage(response.error?.message || '数据源详情暂时不可用。'); return }
    setDraft(sourceDraftFrom(source))
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft || saving) return
    if (!draft.name.trim() || !draft.code.trim()) { setMessage('请填写数据源名称和编码。'); return }
    try {
      const config = JSON.parse(draft.configJSON) as unknown
      const schema = JSON.parse(draft.schemaJSON) as unknown
      const dedupe = JSON.parse(draft.dedupeKeys) as unknown
      if (!config || Array.isArray(config) || typeof config !== 'object' || !schema || Array.isArray(schema) || typeof schema !== 'object' || !Array.isArray(dedupe)) throw new Error('shape')
    } catch { setMessage('配置和 Schema 必须为 JSON 对象，去重键必须为 JSON 数组。'); return }
    setSaving(true)
    const response = await client(draft.id ? `/v1/sources/${draft.id}` : '/v1/sources', {
      method: draft.id ? 'PUT' : 'POST', showResult: false, silentLoading: true,
      body: { name: draft.name.trim(), code: draft.code.trim(), source_type: draft.sourceType, enabled: draft.enabled, auth_type: draft.authType.trim() || 'none', config_json: draft.configJSON, schema_json: draft.schemaJSON, dedupe_keys: draft.dedupeKeys, source_query_key: draft.sourceQueryKey.trim() },
    })
    setSaving(false)
    if (!response.ok) { setMessage(response.error?.message || '数据源保存未完成。'); return }
    setDraft(null)
    setMessage('数据源已保存。')
    setReloadVersion((version) => version + 1)
  }
  return (
    <div className="view-stack">
      {message && <div className="result-banner" role="status">{message}</div>}
      <form className="query-bar source-query-bar" onSubmit={submitQuery}><div className="query-fields">
        <Field label="名称 / 编码 / 鉴权" name="source_query" value={query} onChange={setQuery} />
        <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
        <SelectFilter label="类型" value={sourceType || 'all'} onChange={(value) => setSourceType(value === 'all' ? '' : value)} options={[{ value: 'api_poll', label: 'API' }, { value: 'webhook', label: 'Webhook' }, { value: 'database', label: '数据库' }, { value: 'file', label: '文件' }]} />
      </div><div className="query-bar-actions"><span>查询命中 <strong>{pagination?.total ?? 0}</strong> 条</span><button type="button" onClick={resetQuery} disabled={loading}>重置筛选</button><button className="primary" type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button></div></form>
      {error && <div className="result-banner error" role="alert">{error}{recordsPage ? ' 已保留最近一次成功数据。' : ''}</div>}
      <div className="record-actions"><button type="button" className="primary" onClick={() => setDraft({ id: null, name: '', code: '', sourceType: 'api_poll', enabled: true, authType: 'none', configJSON: '{\n  "url": "",\n  "method": "GET",\n  "records_path": "data"\n}', schemaJSON: '{}', dedupeKeys: '[]', sourceQueryKey: '', hasSecret: false })}>新增数据源</button></div>
      <Panel title="数据源配置" icon={<Database />} meta={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}><SourceList sources={listedSources} onDetail={(source) => { void openDetail(source.id) }} onFetchSource={onFetchSource} onTestSource={onTestSource} /><MonitoringPaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} /></Panel>
      {draft && <Modal title={draft.id ? '数据源详情与编辑' : '新增数据源'} onClose={() => { if (!saving) setDraft(null) }}>
        {draft.hasSecret && <div className="result-banner" role="status">配置中的敏感值已隐藏。保留“[已隐藏]”会保留原值；改为新值即可轮换，且不会回显旧值。</div>}
        <form className="excel-upload-form" onSubmit={save}>
          <Field label="数据源名称" name="source_name" value={draft.name} required onChange={(name) => setDraft({ ...draft, name })} />
          <Field label="数据源编码" name="source_code" value={draft.code} required onChange={(code) => setDraft({ ...draft, code })} />
          <label>数据源类型<select value={draft.sourceType} disabled={saving} onChange={(event) => setDraft({ ...draft, sourceType: event.currentTarget.value })}><option value="api_poll">API 轮询</option><option value="database">数据库</option><option value="webhook">Webhook</option></select></label>
          <Field label="鉴权类型" name="source_auth_type" value={draft.authType} onChange={(authType) => setDraft({ ...draft, authType })} />
          <label className="checkbox-label"><input type="checkbox" checked={draft.enabled} disabled={saving} onChange={(event) => setDraft({ ...draft, enabled: event.currentTarget.checked })} />启用数据源</label>
          <Field label="来源查询键" name="source_query_key" value={draft.sourceQueryKey} onChange={(sourceQueryKey) => setDraft({ ...draft, sourceQueryKey })} />
          <label>连接配置 JSON<textarea rows={10} value={draft.configJSON} disabled={saving} onChange={(event) => setDraft({ ...draft, configJSON: event.currentTarget.value })} /></label>
          <label>Schema JSON<textarea rows={5} value={draft.schemaJSON} disabled={saving} onChange={(event) => setDraft({ ...draft, schemaJSON: event.currentTarget.value })} /></label>
          <label>去重键 JSON 数组<textarea rows={4} value={draft.dedupeKeys} disabled={saving} onChange={(event) => setDraft({ ...draft, dedupeKeys: event.currentTarget.value })} /></label>
          <p className="query-contract-note">API 测试会发起真实连通性请求；Webhook 不支持主动拉取。Schema 与去重键目前由服务端保存，未参与拉取校验。</p>
          <div className="excel-form-actions"><button className="primary" type="submit" disabled={saving}>{saving ? '保存中…' : '保存数据源'}</button></div>
        </form>
      </Modal>}
    </div>
  )
}

function RawRecordsQueryPage({ title, origin, client, onFetchSource }: { title: string; origin: RawRecordOrigin; client: ApiClient; onFetchSource: (sourceID: number) => Promise<ApiResult> }) {
  const [source, setSource] = useState('')
  const [dataType, setDataType] = useState('')
  const [status, setStatus] = useState('')
  const [businessKey, setBusinessKey] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [appliedQuery, setAppliedQuery] = useState({ source: '', dataType: '', status: '', businessKey: '', startTime: '', endTime: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<RawRecordsPage<RawData> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [pendingSourceFetchID, setPendingSourceFetchID] = useState<number | null>(null)
  const [fetchingSourceID, setFetchingSourceID] = useState<number | null>(null)
  const [sourceFetchMessage, setSourceFetchMessage] = useState('')
  const requestRef = useRef<AbortController | null>(null)

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    const body = buildRawRecordsRequest({ page, pageSize: 20, origin, ...appliedQuery })
    void client('/v1/data/raw/list', {
      method: 'POST', body, signal: controller.signal, showResult: false, silentLoading: true,
    }).then((response) => {
      if (controller.signal.aborted) return
      const nextPage = response.ok ? parseRawRecordsPage<RawData>(response.data) : null
      if (nextPage) {
        setRecordsPage(nextPage)
        return
      }
      setError(response.error?.message || '记录查询暂时不可用，请稍后重试。')
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false)
    })
    return () => controller.abort()
  }, [appliedQuery, client, origin, page])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setAppliedQuery({
      source,
      dataType,
      status,
      businessKey,
      startTime: backendDateTime(startTime),
      endTime: backendDateTime(endTime),
    })
  }

  function resetQuery() {
    setSource('')
    setDataType('')
    setStatus('')
    setBusinessKey('')
    setStartTime('')
    setEndTime('')
    setPage(1)
    setAppliedQuery({ source: '', dataType: '', status: '', businessKey: '', startTime: '', endTime: '' })
  }

  async function fetchSource() {
    const sourceID = pendingSourceFetchID
    if (!sourceID || fetchingSourceID !== null) return
    setFetchingSourceID(sourceID)
    setSourceFetchMessage('')
    const response = await onFetchSource(sourceID)
    const summary = response.ok ? parseSourceFetchSummary(response.data) : null
    setSourceFetchMessage(summary
      ? `数据源 #${sourceID} 拉取完成：成功 ${summary.successCount}/${summary.totalCount}，失败 ${summary.failedCount}。`
      : response.error?.message || '数据源拉取未完成，请稍后重试。')
    setFetchingSourceID(null)
    setPendingSourceFetchID(null)
  }

  const records = recordsPage?.list ?? []
  const total = recordsPage?.total ?? 0
  const totalPages = recordsPage?.totalPages ?? 0
  return (
    <div className="view-stack">
      {origin === 'pull' && <section className="raw-record-summary" aria-label="拉取记录摘要">
        <Metric label="当前结果" value={total} />
        <Metric label="当前页" value={records.length} />
        <Metric label="总页数" value={Math.max(totalPages, 1)} />
      </section>}
      <form className="query-bar raw-record-query-bar" onSubmit={submit}>
        <div className="query-fields">
          <Field label="ID / 外部编号 / 内容" name="raw_business_key" value={businessKey} onChange={setBusinessKey} />
          <SelectFilter label="状态" value={status || 'all'} onChange={(next) => setStatus(next === 'all' ? '' : next)} options={[
            { value: 'pending', label: '待处理' }, { value: 'processing', label: '处理中' }, { value: 'processed', label: '已处理' }, { value: 'error', label: '异常' },
          ]} />
          <Field label="来源" name="raw_source" value={source} onChange={setSource} />
          {origin === 'pull' && <Field label="开始时间" name="raw_start_time" type="datetime-local" value={startTime} onChange={setStartTime} />}
          {origin === 'pull' && <Field label="结束时间" name="raw_end_time" type="datetime-local" value={endTime} onChange={setEndTime} />}
        </div>
        <div className="query-bar-actions"><span>查询命中 <strong>{total}</strong> 条</span><button type="button" onClick={resetQuery} disabled={loading}>重置</button><button className="primary" type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button></div>
        <details className="raw-record-advanced-filter"><summary>更多筛选</summary><Field label="数据类型" name="raw_data_type" value={dataType} onChange={setDataType} />{origin === 'receive' && <><Field label="开始时间" name="raw_start_time" type="datetime-local" value={startTime} onChange={setStartTime} /><Field label="结束时间" name="raw_end_time" type="datetime-local" value={endTime} onChange={setEndTime} /></>}</details>
      </form>
      <p className="query-contract-note">来源、类型、状态、外部业务键与时间范围均由服务端分页筛选；业务键对应原始记录的外部 ID。</p>
      {error && <div className="result-banner error" role="alert">{error} 已保留最近一次成功数据。</div>}
      <Panel title={`${title}（含脱敏内容）`} icon={<Inbox />} meta={loading && !recordsPage ? '正在加载…' : `共 ${total} 条`}>
        {sourceFetchMessage && <div className="result-banner" role="status" aria-live="polite">{sourceFetchMessage}</div>}
        <RawDataList origin={origin} records={records} onRequestSourceFetch={setPendingSourceFetchID} />
        <div className="record-actions raw-record-pagination" role="status" aria-live="polite">
          <span>第 {recordsPage?.page ?? page} / {Math.max(totalPages, 1)} 页</span>
          <button type="button" onClick={() => setPage((current) => Math.max(1, current - 1))} disabled={loading || page <= 1}>上一页</button>
          <button type="button" onClick={() => setPage((current) => current + 1)} disabled={loading || totalPages === 0 || page >= totalPages}>下一页</button>
        </div>
      </Panel>
      <WarehouseRawRecordsPanel client={client} origin={origin} />
      {pendingSourceFetchID !== null && <Modal title="确认拉取数据源" closeDisabled={fetchingSourceID !== null} onClose={() => { if (fetchingSourceID === null) setPendingSourceFetchID(null) }} footer={<><button type="button" disabled={fetchingSourceID !== null} onClick={() => setPendingSourceFetchID(null)}>取消</button><button className="primary" type="button" disabled={fetchingSourceID !== null} onClick={() => void fetchSource()}>{fetchingSourceID === pendingSourceFetchID ? '拉取中…' : '确认拉取'}</button></>}><p>确认立即拉取数据源 #{pendingSourceFetchID}？该操作会向已配置的来源发起真实请求。</p></Modal>}
    </div>
  )
}

function WarehouseRawRecordsPanel({ client, origin }: { client: ApiClient; origin: RawRecordOrigin }) {
  const [source, setSource] = useState('')
  const [status, setStatus] = useState('')
  const [traceID, setTraceID] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [appliedQuery, setAppliedQuery] = useState({ source: '', status: '', traceID: '', startTime: '', endTime: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<RawRecordsPage<WarehouseRawRecord> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [pendingRetransform, setPendingRetransform] = useState<WarehouseRawRecord | null>(null)
  const [retransformingID, setRetransformingID] = useState<number | null>(null)
  const [reloadVersion, setReloadVersion] = useState(0)
  const requestRef = useRef<AbortController | null>(null)

  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setLoading(true)
    setError('')
    const query = buildWarehouseRawRecordsQuery({ page, pageSize: 20, origin, ...appliedQuery })
    void client(`/v1/raw-records?${query}`, {
      method: 'GET', signal: controller.signal, showResult: false, silentLoading: true,
    }).then((response) => {
      if (controller.signal.aborted) return
      const parsed = response.ok ? parseRawRecordsPage<unknown>(response.data) : null
      const records = parsed?.list.map(parseWarehouseRawRecord) ?? []
      if (parsed && records.every((record): record is WarehouseRawRecord => record !== null)) {
        setRecordsPage({ ...parsed, list: records })
        return
      }
      setError(response.error?.message || '可重新处理记录查询暂时不可用，请稍后重试。')
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false)
    })
    return () => controller.abort()
  }, [appliedQuery, client, origin, page, reloadVersion])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setAppliedQuery({ source, status, traceID, startTime: backendDateTime(startTime), endTime: backendDateTime(endTime) })
  }

  async function retransform() {
    const record = pendingRetransform
    if (!record || retransformingID !== null) return
    setRetransformingID(record.id)
    setError('')
    const response = await client(`/v1/raw-records/${record.id}/retransform`, {
      method: 'POST', retry: false, showResult: false, silentLoading: true,
    })
    setRetransformingID(null)
    if (!response.ok) {
      setError(response.error?.message || '重新处理未完成，请稍后重试。')
      return
    }
    const result = parseRetransformResult(response.data)
    if (!result) {
      setError('重新处理已提交，但未收到可验证的结果摘要。')
      return
    }
    setPendingRetransform(null)
    setMessage(`重新处理完成：追踪 ${result.traceID || '-'}，清洗记录 #${result.cleanRecordID}。`)
    setReloadVersion((version) => version + 1)
  }

  const records = recordsPage?.list ?? []
  const total = recordsPage?.total ?? 0
  const totalPages = recordsPage?.totalPages ?? 0
  return (
    <>
      <Panel title={`可重新处理${origin === 'pull' ? '拉取' : '接收'}记录（仅元数据）`} icon={<RefreshCcw />} meta={loading && !recordsPage ? '正在加载…' : `共 ${total} 条`}>
        <p className="query-contract-note">此列表查询新仓库中 origin={origin} 的脱敏记录。历史列表仍只读；只有本列表中的 ID 可安全重新处理。</p>
        <form className="query-bar" onSubmit={submit}>
          <div className="query-fields">
            <Field label="来源" name="warehouse_raw_source" value={source} onChange={setSource} />
            <SelectFilter label="状态" value={status || 'all'} onChange={(next) => setStatus(next === 'all' ? '' : next)} options={[
              { value: 'received', label: '已接收' }, { value: 'queued', label: '排队中' }, { value: 'cleaning', label: '处理中' }, { value: 'cleaned', label: '已清洗' }, { value: 'failed', label: '失败' },
            ]} />
            <Field label="追踪 ID" name="warehouse_raw_trace_id" value={traceID} onChange={setTraceID} />
            <Field label="开始时间" name="warehouse_raw_start_time" type="datetime-local" value={startTime} onChange={setStartTime} />
            <Field label="结束时间" name="warehouse_raw_end_time" type="datetime-local" value={endTime} onChange={setEndTime} />
          </div>
          <button type="submit" disabled={loading || retransformingID !== null}>{loading ? '查询中…' : '查询'}</button>
        </form>
        {message && <div className="result-banner" role="status" aria-live="polite">{message}</div>}
        {error && <div className="result-banner error" role="alert">{error} 已保留最近一次成功数据。</div>}
        {records.length === 0 ? <EmptyState text="暂无可重新处理的原始记录。" /> : (
          <div className="data-table-wrap" role="region" aria-label="可重新处理原始记录列表" tabIndex={0}>
            <table className="data-table">
              <thead><tr><th scope="col">记录 ID</th><th scope="col">来源</th><th scope="col">追踪 ID</th><th scope="col">接收时间</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
              <tbody>{records.map((record) => (
                <tr key={record.id}>
                  <td>#{record.id}</td>
                  <td><strong>{record.sourceCode || '未命名来源'}</strong><small>来源 #{record.sourceID || '-'}</small></td>
                  <td>{record.traceID || '-'}</td>
                  <td>{record.receivedAt || (record.createdAt ? formatUnixTime(record.createdAt) : '-')}</td>
                  <td><StatusPill label={rawRecordStatusLabel(record.status)} /></td>
                  <td><button type="button" disabled={retransformingID !== null || record.status === 'cleaning'} onClick={() => setPendingRetransform(record)}>{retransformingID === record.id ? '处理中…' : '重新处理'}</button></td>
                </tr>
              ))}</tbody>
            </table>
          </div>
        )}
        <div className="record-actions raw-record-pagination" role="status" aria-live="polite">
          <span>第 {recordsPage?.page ?? page} / {Math.max(totalPages, 1)} 页</span>
          <button type="button" onClick={() => setPage((current) => Math.max(1, current - 1))} disabled={loading || retransformingID !== null || page <= 1}>上一页</button>
          <button type="button" onClick={() => setPage((current) => current + 1)} disabled={loading || retransformingID !== null || totalPages === 0 || page >= totalPages}>下一页</button>
        </div>
      </Panel>
      {pendingRetransform && <Modal title="确认重新处理原始记录" closeDisabled={retransformingID !== null} onClose={() => { if (retransformingID === null) setPendingRetransform(null) }} footer={<><button type="button" disabled={retransformingID !== null} onClick={() => setPendingRetransform(null)}>取消</button><button className="primary" type="button" disabled={retransformingID !== null} onClick={() => void retransform()}>{retransformingID === pendingRetransform.id ? '处理中…' : '确认重新处理'}</button></>}><p>确认重新处理仓库原始记录 #{pendingRetransform.id}？系统会创建新的清洗记录；原始内容不会在管理端展示。</p></Modal>}
    </>
  )
}

function RulesQueryPage({ client, rules, sources, onRulesChange, refreshVersion }: { client: ApiClient; rules: TransformRule[]; sources: SourceDefinition[]; onRulesChange: (rules: TransformRule[]) => void; refreshVersion: number }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [ruleType, setRuleType] = useState('all')
  const [applied, setApplied] = useState({ keyword: '', enabled: '' as '' | 'true' | 'false', ruleType: '' })
  const [page, setPage] = useState(1)
  const [reloadVersion, setReloadVersion] = useState(0)
  const [draft, setDraft] = useState<RuleDraft | null>(null)
  const [rawContent, setRawContent] = useState('{}')
  const [testResult, setTestResult] = useState<unknown>(null)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [operationError, setOperationError] = useState('')
  const listQuery = useMemo(() => buildTransformRuleListQuery({ page, pageSize: 20, ...applied }), [applied, page])
  const { recordsPage, loading, error } = useConfigurationListPage<TransformRule>(client, '/v1/transform-rules', 'rules', listQuery, reloadVersion + refreshVersion)
  const listedRules = recordsPage?.list ?? []
  const pagination = recordsPage?.pagination

  function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    applyQuery(query, status, ruleType)
  }

  function applyQuery(nextQuery: string, nextStatus: string, nextRuleType: string) {
    setPage(1)
    setApplied({ keyword: nextQuery, enabled: nextStatus === 'enabled' ? 'true' : nextStatus === 'disabled' ? 'false' : '', ruleType: nextRuleType === 'all' ? '' : nextRuleType })
    setReloadVersion((version) => version + 1)
  }

  function openCreate() {
    setOperationError('')
    setDraft({ id: null, sourceID: sources[0]?.id ? String(sources[0].id) : '', name: '', ruleType: 'mapping', orderIndex: '0', configJSON: '{\n  "table_name": "",\n  "business_key_field": "",\n  "fields": []\n}', enabled: true, hasSecret: false })
  }

  async function openDetail(ruleID: number) {
    setOperationError('')
    const response = await client(`/v1/transform-rules/${ruleID}`, { method: 'GET', showResult: false, silentLoading: true })
    const rule = response.ok ? readObject<TransformRule>(response, 'rule') : null
    if (!rule) {
      setOperationError(response.error?.message || '规则详情暂时不可用，请稍后重试。')
      return
    }
    setDraft(ruleDraftFrom(rule))
  }

  async function saveDraft(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft || saving) return
    const sourceID = Number(draft.sourceID)
    const orderIndex = Number(draft.orderIndex)
    if (!Number.isInteger(sourceID) || sourceID <= 0 || !draft.name.trim() || !Number.isInteger(orderIndex)) {
      setOperationError('请填写来源、名称和有效的顺序号。')
      return
    }
    try { JSON.parse(draft.configJSON) } catch { setOperationError('配置必须是有效 JSON。'); return }
    setSaving(true)
    setOperationError('')
    const response = await client(draft.id ? `/v1/transform-rules/${draft.id}` : '/v1/transform-rules', {
      method: draft.id ? 'PUT' : 'POST', showResult: false, silentLoading: true,
      body: { source_id: sourceID, name: draft.name.trim(), rule_type: draft.ruleType, order_index: orderIndex, config_json: draft.configJSON, enabled: draft.enabled },
    })
    const saved = response.ok ? readObject<TransformRule>(response, 'rule') : null
    if (!saved) {
      setOperationError(response.error?.message || '规则保存未完成，请稍后重试。')
      setSaving(false)
      return
    }
    onRulesChange(draft.id ? rules.map((rule) => rule.id === saved.id ? saved : rule) : [...rules, saved].sort((left, right) => left.source_id - right.source_id || left.order_index - right.order_index || right.id - left.id))
    setSaving(false)
    setDraft(null)
    setReloadVersion((version) => version + 1)
  }

  async function runTest(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft || draft.ruleType !== 'mapping' || testing) return
    let parsedContent: Record<string, unknown>
    try {
      parsedContent = JSON.parse(rawContent) as Record<string, unknown>
      JSON.parse(draft.configJSON)
    } catch {
      setOperationError('测试内容和规则配置都必须是有效 JSON。')
      return
    }
    if (!parsedContent || Array.isArray(parsedContent) || typeof parsedContent !== 'object') {
      setOperationError('测试原始内容必须是 JSON 对象。')
      return
    }
    setTesting(true)
    setOperationError('')
    const response = await client('/v1/transform-rules/test', { method: 'POST', body: { raw_content: parsedContent, config_json: draft.configJSON }, showResult: false, silentLoading: true })
    const cleanContent = response.ok ? readObject<unknown>(response, 'clean_content') : null
    if (cleanContent === null) setOperationError(response.error?.message || '规则测试未完成，请稍后重试。')
    else setTestResult(redactMonitoringJSON(cleanContent))
    setTesting(false)
  }

  const testable = draft?.ruleType === 'mapping'
  return (
    <div className="view-stack design-data-page design-rules-page">
      {operationError && <div className="result-banner error" role="alert">{operationError}</div>}
      <form className="query-bar design-filter-bar design-rules-filter" onSubmit={submitQuery}>
        <div className="query-fields">
          <label>名称 / 来源 ID / 配置<span className="design-search-control"><Search aria-hidden="true" /><input name="rule_query" type="search" value={query} onChange={(event) => setQuery(event.currentTarget.value)} placeholder="搜索规则" /></span></label>
          <SelectFilter label="状态" value={status} onChange={(nextStatus) => { setStatus(nextStatus); applyQuery(query, nextStatus, ruleType) }} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
          <SelectFilter label="规则类型" value={ruleType} onChange={(nextRuleType) => { setRuleType(nextRuleType); applyQuery(query, status, nextRuleType) }} options={[{ value: 'mapping', label: 'mapping' }, { value: 'validator', label: 'validator' }, { value: 'http_enrich', label: 'http_enrich' }, { value: 'db_enrich', label: 'db_enrich' }, { value: 'script', label: 'script' }]} />
        </div>
        <div className="design-filter-count"><strong>{listedRules.length}</strong><span>/ {pagination?.total ?? 0} 条</span></div>
        <button className="sr-only" type="submit" disabled={loading}>{loading ? '查询中…' : '查询规则'}</button>
      </form>
      {error && <div className="result-banner error" role="alert">{error}{recordsPage ? ' 已保留最近一次成功数据。' : ''}</div>}
      <section className="design-table-section">
        <div className="design-section-heading"><div><h3>清洗规则</h3><span>{loading && !recordsPage ? '正在加载…' : `查询命中 ${pagination?.total ?? 0} 条`}</span></div><button type="button" className="primary" onClick={openCreate}>新增规则</button></div>
        <TransformRuleList rules={listedRules} sources={sources} onDetail={(rule) => { void openDetail(rule.id) }} />
        <MonitoringPaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} />
      </section>
      {draft && (
        <Modal title={draft.id ? '清洗规则详情与编辑' : '新增清洗规则'} onClose={() => { if (!saving && !testing) setDraft(null) }}>
          {draft.hasSecret && <div className="result-banner" role="status">配置中的敏感值已隐藏。保留“[已隐藏]”会保留原值；改为新值即可轮换，且不会回显旧值。</div>}
          <form className="excel-upload-form" onSubmit={saveDraft}>
            <label>来源
              <select value={draft.sourceID} disabled={saving} required onChange={(event) => setDraft({ ...draft, sourceID: event.currentTarget.value })}>
                <option value="">选择数据源</option>
                {sources.map((source) => <option value={source.id} key={source.id}>#{source.id} {source.name}</option>)}
              </select>
            </label>
            <Field label="规则名称" name="rule_name" value={draft.name} required onChange={(name) => setDraft({ ...draft, name })} />
            <label>规则类型
              <select value={draft.ruleType} disabled={saving} onChange={(event) => setDraft({ ...draft, ruleType: event.currentTarget.value })}>
                {['mapping', 'http_enrich', 'db_enrich', 'script', 'validator'].map((type) => <option key={type} value={type}>{type}</option>)}
              </select>
            </label>
            <Field label="执行顺序" name="rule_order" type="number" value={draft.orderIndex} required onChange={(orderIndex) => setDraft({ ...draft, orderIndex })} />
            <label className="checkbox-label"><input type="checkbox" checked={draft.enabled} disabled={saving} onChange={(event) => setDraft({ ...draft, enabled: event.currentTarget.checked })} />启用规则</label>
            <label>规则配置 JSON<textarea value={draft.configJSON} disabled={saving} rows={12} onChange={(event) => setDraft({ ...draft, configJSON: event.currentTarget.value })} /></label>
            <div className="excel-form-actions"><button className="primary" type="submit" disabled={saving}>{saving ? '保存中…' : '保存规则'}</button></div>
          </form>
          <hr />
          <form className="excel-upload-form" onSubmit={runTest}>
            <h4>Mapping 规则测试</h4>
            <p className="query-contract-note">测试只使用当前草稿，不会保存配置；仅 mapping 类型有真实后端执行能力。</p>
            <label>测试原始内容 JSON<textarea value={rawContent} disabled={!testable || testing} rows={8} onChange={(event) => setRawContent(event.currentTarget.value)} /></label>
            <div className="excel-form-actions"><button type="submit" disabled={!testable || testing}>{testing ? '测试中…' : '执行测试'}</button></div>
            {testResult !== null && <ReadonlyJSON value={testResult} />}
          </form>
        </Modal>
      )}
    </div>
  )
}

function ProcessedQueryPage({ client }: { client: ApiClient }) {
  const [view, setView] = useState<'legacy' | 'clean'>('clean')
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
      if (!parsed) {
        setStatisticsError('处理统计暂时不可用，记录列表仍可继续查询。')
        setStatisticsLoading(false)
        return
      }
      setStatistics(parsed)
      setStatisticsLoading(false)
    }).catch(() => {
      if (controller.signal.aborted) return
      setStatisticsError('处理统计暂时不可用，记录列表仍可继续查询。')
      setStatisticsLoading(false)
    })
    return () => controller.abort()
  }, [client, statisticsRequest])

  return (
    <div className="view-stack design-data-page design-processed-page">
      {statisticsLoading && !statistics && <p className="query-contract-note" role="status">正在加载处理统计…</p>}
      {statisticsError && <div className="result-banner error" role="alert">{statisticsError} {statistics && '当前展示的是上一次成功数据。'} <button type="button" onClick={() => setStatisticsRequest((current) => current + 1)} disabled={statisticsLoading}>重试统计</button></div>}
      {view === 'legacy'
        ? <LegacyProcessedQueryPanel client={client} />
        : <CleanRecordsQueryPanel client={client} statistics={statistics} statisticsLoading={statisticsLoading} />}
      <div className="tab-actions design-view-switch" role="tablist" aria-label="处理结果数据视图">
        <button type="button" role="tab" aria-selected={view === 'legacy'} className={view === 'legacy' ? 'active' : ''} onClick={() => setView('legacy')}>旧处理结果</button>
        <button type="button" role="tab" aria-selected={view === 'clean'} className={view === 'clean' ? 'active' : ''} onClick={() => setView('clean')}>清洗记录</button>
      </div>
    </div>
  )
}

function LegacyProcessedQueryPanel({ client }: { client: ApiClient }) {
  const [dataType, setDataType] = useState('')
  const [minQuality, setMinQuality] = useState('')
  const [maxQuality, setMaxQuality] = useState('')
  const [createdFrom, setCreatedFrom] = useState('')
  const [createdTo, setCreatedTo] = useState('')
  const [appliedQuery, setAppliedQuery] = useState({ dataType: '', minQuality: '', maxQuality: '', createdFrom: '', createdTo: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<ProcessedRecordsPage<ProcessedData> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
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
      if (nextPage) {
        setRecordsPage(nextPage)
        return
      }
      setError(response.error?.message || '处理结果查询暂时不可用，请稍后重试。')
    }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [appliedQuery, client, page])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setAppliedQuery({ dataType, minQuality, maxQuality, createdFrom: unixTimestamp(createdFrom), createdTo: unixTimestamp(createdTo) })
  }

  const records = recordsPage?.list ?? []
  const totalPages = recordsPage?.totalPages ?? 0
  return (
    <div className="view-stack">
      <form className="query-bar" onSubmit={submit}>
        <div className="query-fields">
          <Field label="数据类型" name="processed_type" value={dataType} onChange={setDataType} />
          <Field label="最低质量分" name="processed_min_quality" type="number" value={minQuality} onChange={setMinQuality} />
          <Field label="最高质量分" name="processed_max_quality" type="number" value={maxQuality} onChange={setMaxQuality} />
          <Field label="开始时间" name="processed_from" type="datetime-local" value={createdFrom} onChange={setCreatedFrom} />
          <Field label="结束时间" name="processed_to" type="datetime-local" value={createdTo} onChange={setCreatedTo} />
        </div>
        <button type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
      <p className="query-contract-note">旧处理结果支持类型、质量和时间范围筛选；业务键属于 clean records，待独立查询接口接入后提供。</p>
      {error && <div className="result-banner error" role="alert">{error} 已保留最近一次成功数据。</div>}
      <Panel title="处理结果" icon={<CheckCircle2 />} meta={loading && !recordsPage ? '正在加载…' : `共 ${recordsPage?.total ?? 0} 条 / 平均质量 ${recordsPage?.averageQuality.toFixed(1) ?? '-'}`}>
        <ProcessedDataList records={records} />
        <div className="record-actions raw-record-pagination" role="status" aria-live="polite">
          <span>第 {recordsPage?.page ?? page} / {Math.max(totalPages, 1)} 页</span>
          <button type="button" onClick={() => setPage((current) => Math.max(1, current - 1))} disabled={loading || page <= 1}>上一页</button>
          <button type="button" onClick={() => setPage((current) => current + 1)} disabled={loading || totalPages === 0 || page >= totalPages}>下一页</button>
        </div>
      </Panel>
    </div>
  )
}

function CleanRecordsQueryPanel({ client, statistics, statisticsLoading }: { client: ApiClient; statistics: DataStatisticsSummary | null; statisticsLoading: boolean }) {
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
  const [recordsPage, setRecordsPage] = useState<ProcessedRecordsPage<CleanRecord> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
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
      if (nextPage) {
        setRecordsPage(nextPage)
        return
      }
      setError(response.error?.message || '清洗记录查询暂时不可用，请稍后重试。')
    }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [appliedQuery, client, page])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setAppliedQuery({ sourceID, tableName, businessKey, status, minQuality, maxQuality, createdFrom: unixTimestamp(createdFrom), createdTo: unixTimestamp(createdTo) })
  }

  const records = recordsPage?.list ?? []
  const totalPages = recordsPage?.totalPages ?? 0
  return (
    <div className="view-stack design-clean-records">
      <form className="query-bar design-filter-bar design-processed-filter" onSubmit={submit}>
        <div className="query-fields">
          <label>业务键 / Raw ID / 内容<span className="design-search-control"><Search aria-hidden="true" /><input name="clean_business_key" type="search" value={businessKey} onChange={(event) => setBusinessKey(event.currentTarget.value)} placeholder="输入业务键" /></span></label>
          <label>数据类型<input name="clean_table_name" value={tableName} onChange={(event) => setTableName(event.currentTarget.value)} placeholder="全部" /></label>
          <label>质量<select name="clean_quality_band" value={qualityBand} onChange={(event) => { const next = event.currentTarget.value; setQualityBand(next); setMinQuality(next === 'high' ? '80' : ''); setMaxQuality(next === 'review' ? '79.99' : '') }}><option value="all">全部</option><option value="high">80 分及以上</option><option value="review">待复核（低于 80 分）</option></select></label>
        </div>
        <div className="design-filter-count"><strong>{records.length}</strong><span>/ {recordsPage?.total ?? 0} 条</span></div>
        <button className="sr-only" type="submit" disabled={loading}>{loading ? '查询中…' : '查询处理结果'}</button>
        <details className="design-advanced-filters"><summary>更多筛选</summary><div><Field label="来源 ID" name="clean_source_id" type="number" value={sourceID} onChange={setSourceID} /><SelectFilter label="处理状态" value={status || 'all'} onChange={(next) => setStatus(next === 'all' ? '' : next)} options={[{ value: 'ready', label: '待推送' }, { value: 'invalid', label: '无效' }, { value: 'delivered', label: '已交付' }]} /><Field label="开始时间" name="clean_from" type="datetime-local" value={createdFrom} onChange={setCreatedFrom} /><Field label="结束时间" name="clean_to" type="datetime-local" value={createdTo} onChange={setCreatedTo} /></div></details>
      </form>
      <section className="overview-grid compact design-processed-summary" aria-label="处理统计" aria-busy={statisticsLoading}>
        <Metric label="平均质量" value={statistics?.averageQualityScore === null || statistics?.averageQualityScore === undefined ? '-' : statistics.averageQualityScore.toFixed(1)} />
        <Metric label="已处理" value={statistics?.processedCount ?? '-'} />
        <Metric label="处理失败" value={statistics?.errorCount ?? '-'} />
      </section>
      {error && <div className="result-banner error" role="alert">{error} 已保留最近一次成功数据。</div>}
      <section className="design-table-section">
        <div className="design-section-heading"><div><h3>处理结果</h3><span>{loading && !recordsPage ? '正在加载…' : `查询命中 ${recordsPage?.total ?? 0} 条`}</span></div></div>
        <CleanRecordList records={records} />
        <div className="record-actions raw-record-pagination" role="status" aria-live="polite">
          <span>第 {recordsPage?.page ?? page} / {Math.max(totalPages, 1)} 页</span>
          <button type="button" onClick={() => setPage((current) => Math.max(1, current - 1))} disabled={loading || page <= 1}>上一页</button>
          <button type="button" onClick={() => setPage((current) => current + 1)} disabled={loading || totalPages === 0 || page >= totalPages}>下一页</button>
        </div>
      </section>
    </div>
  )
}

function BojunBackfillPage({ loading, onPreview, onConfirm }: {
  loading: boolean
  onPreview: (payload: { start_time: string; end_time: string }) => Promise<BojunOrderBackfillResult | null>
  onConfirm: (payload: { start_time: string; end_time: string }) => Promise<BojunOrderBackfillResult | null>
}) {
  const previewVersionRef = useRef(0)
  const [payload, setPayload] = useState<{ start_time: string; end_time: string } | null>(null)
  const [preview, setPreview] = useState<BojunOrderBackfillResult | null>(null)
  const [confirmed, setConfirmed] = useState<BojunOrderBackfillResult | null>(null)
  const [confirmingWrite, setConfirmingWrite] = useState(false)
  const [writing, setWriting] = useState(false)
  function invalidatePreview() {
    previewVersionRef.current += 1
    setPayload(null)
    setPreview(null)
    setConfirmed(null)
    setConfirmingWrite(false)
  }
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const nextPayload = { start_time: formValue(form, 'start_time'), end_time: formValue(form, 'end_time') }
    const requestVersion = previewVersionRef.current + 1
    invalidatePreview()
    const result = await onPreview(nextPayload)
    if (previewVersionRef.current !== requestVersion) return
    setPayload(result ? nextPayload : null)
    setPreview(result)
  }
  async function confirmWrite() {
    if (!payload || !preview || writing) return
    setWriting(true)
    try {
      setConfirmed(await onConfirm(payload))
      setConfirmingWrite(false)
    } finally {
      setWriting(false)
    }
  }
  return (
    <div className="view-stack">
      <Panel title="伯俊订单补拉" icon={<Download />} meta="预览不写库，确认后按 docno 判重">
        <form className="bojun-backfill-form" onSubmit={submit}>
          <Field label="开始时间" name="start_time" type="datetime-local" defaultValue={datetimeLocalMinutesAgo(60)} onChange={invalidatePreview} required />
          <Field label="结束时间" name="end_time" type="datetime-local" defaultValue={datetimeLocalMinutesAgo(0)} onChange={invalidatePreview} required />
          <button className="primary" type="submit" disabled={loading}>预览补拉</button>
          <button type="button" disabled={loading || writing || !preview || preview.writable_count === 0} onClick={() => setConfirmingWrite(true)}>确认写入</button>
        </form>
        <aside className="backfill-warning" aria-label="写入提示"><AlertTriangle aria-hidden="true" /><strong>写入前请确认</strong><span>预览会真实请求伯俊接口，但不会写入数据库。</span><span>确认后将重新拉取相同时间范围。</span><span>已有 docno 不覆盖，请核对可写入数量。</span></aside>
        {preview && <BojunBackfillResultView title="预览结果" result={preview} />}
        {confirmed && <BojunBackfillResultView title="写入结果" result={confirmed} />}
      </Panel>
      {confirmingWrite && preview && <Modal title="确认写入伯俊订单" closeDisabled={loading || writing} onClose={() => { if (!loading && !writing) setConfirmingWrite(false) }} footer={<><button type="button" disabled={loading || writing} onClick={() => setConfirmingWrite(false)}>取消</button><button className="primary" type="button" disabled={loading || writing} onClick={() => void confirmWrite()}>{writing ? '写入中…' : '确认写入'}</button></>}><p>确认写入 {preview.writable_count} 条伯俊订单？系统会按 docno 判重，已有订单不会覆盖。</p></Modal>}
    </div>
  )
}

function YouzanDistributionPage({ task, loading, onPreview, onConfirm, onRun }: {
  task?: LegacyTask
  loading: boolean
  onPreview: (payload: YouzanDistributionBackfillPayload) => Promise<YouzanDistributionBackfillResult | null>
  onConfirm: (payload: YouzanDistributionBackfillPayload) => Promise<YouzanDistributionBackfillResult | null>
  onRun: (code: string, payload: YouzanDistributionBackfillPayload) => Promise<ApiResult>
}) {
  const previewVersionRef = useRef(0)
  const [timeFilter, setTimeFilter] = useState<YouzanDistributionTimeFilter>('created')
  const [payload, setPayload] = useState<YouzanDistributionBackfillPayload | null>(null)
  const [preview, setPreview] = useState<YouzanDistributionBackfillResult | null>(null)
  const [confirmed, setConfirmed] = useState<YouzanDistributionBackfillResult | null>(null)
  const [showManualRun, setShowManualRun] = useState(false)
  const [manualRunPayload, setManualRunPayload] = useState<YouzanDistributionBackfillPayload | null>(null)
  const [manualRunResult, setManualRunResult] = useState<LegacyTaskRunResult | null>(null)
  const [manualRunError, setManualRunError] = useState('')
  const [runningManualTask, setRunningManualTask] = useState(false)
  const [confirmingBackfill, setConfirmingBackfill] = useState(false)
  const [writingBackfill, setWritingBackfill] = useState(false)

  function invalidateBackfillPreview() {
    previewVersionRef.current += 1
    setPayload(null)
    setPreview(null)
    setConfirmed(null)
    setConfirmingBackfill(false)
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const nextPayload = {
      time_filter: timeFilter,
      start_time: backendDateTime(formValue(form, 'start_time')),
      end_time: backendDateTime(formValue(form, 'end_time')),
    }
    const requestVersion = previewVersionRef.current + 1
    invalidateBackfillPreview()
    const result = await onPreview(nextPayload)
    if (previewVersionRef.current !== requestVersion) return
    setPayload(result ? nextPayload : null)
    setPreview(result)
  }

  async function confirmBackfill() {
    if (!payload || !preview || writingBackfill) return
    setWritingBackfill(true)
    try {
      setConfirmed(await onConfirm(payload))
      setConfirmingBackfill(false)
    } finally {
      setWritingBackfill(false)
    }
  }

  function changeTimeFilter(value: string) {
    if (value !== 'created' && value !== 'success') return
    setTimeFilter(value)
    invalidateBackfillPreview()
  }

  function openManualRun() {
    setManualRunPayload(null)
    setManualRunResult(null)
    setManualRunError('')
    setShowManualRun(true)
  }

  function prepareManualRun(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    setManualRunPayload({
      time_filter: formValue(form, 'time_filter') === 'success' ? 'success' : 'created',
      start_time: backendDateTime(formValue(form, 'start_time')),
      end_time: backendDateTime(formValue(form, 'end_time')),
    })
  }

  async function confirmManualRun() {
    if (!task || !manualRunPayload || runningManualTask) return
    setRunningManualTask(true)
    setManualRunError('')
    const response = await onRun(task.code, manualRunPayload)
    if (response.ok) {
      const result = readObject<LegacyTaskRunResult>(response, 'result')
      if (result?.id && result.queue && result.type) setManualRunResult(result)
      else setManualRunError('任务已提交，但未收到完整的队列任务信息。请在任务系统中核对执行状态。')
    } else {
      setManualRunError(response.error?.message || '任务投递失败，请稍后重试。')
    }
    setRunningManualTask(false)
  }

  return (
    <div className="view-stack">
      <Panel title="每日自动拉取" icon={<ArrowDownToLine />} meta={task?.cron_expr || '等待后端任务定义'}>
        {!task ? <EmptyState text="后端尚未返回有赞分销任务定义，请确认服务已部署并刷新页面。" /> : (
          <>
            <dl className="task-definition-grid">
              <div><dt>任务名称</dt><dd>{task.name}</dd></div>
              <div><dt>Cron</dt><dd><code>{task.cron_expr}</code></dd></div>
              <div><dt>数据来源</dt><dd>{task.input_table}</dd></div>
              <div><dt>写入表</dt><dd><code>{task.output_table}</code></dd></div>
              <div className="wide"><dt>执行规则</dt><dd>{task.description}</dd></div>
              <div className="wide"><dt>昵称处理</dt><dd>所有非空 fans_nickname 必须先批量解密；解密失败时本页订单不写入。</dd></div>
            </dl>
            <div className="task-page-actions">
              <button type="button" onClick={openManualRun} disabled={loading}>运行计划任务</button>
            </div>
          </>
        )}
      </Panel>

      <section className="youzan-operations-grid">
        <Panel title="时间范围补拉" icon={<Download />} meta="按时间筛选并预览">
          <form className="youzan-backfill-form" onSubmit={submit}>
            <label>
              时间筛选方式
              <select name="time_filter" value={timeFilter} onChange={(event) => changeTimeFilter(event.currentTarget.value)}>
                <option value="created">下单时间</option>
                <option value="success">订单完成时间</option>
              </select>
            </label>
            <Field label={timeFilter === 'created' ? '下单开始时间' : '完成开始时间'} name="start_time" type="datetime-local" defaultValue={previousDayDateTimeLocal(false)} onChange={invalidateBackfillPreview} required />
            <Field label={timeFilter === 'created' ? '下单结束时间' : '完成结束时间'} name="end_time" type="datetime-local" defaultValue={previousDayDateTimeLocal(true)} onChange={invalidateBackfillPreview} required />
            <button className="primary" type="submit" disabled={loading}>{loading ? '预览中' : '预览补拉'}</button>
            <button type="button" disabled={loading || writingBackfill || !preview || preview.writable_count === 0} onClick={() => setConfirmingBackfill(true)}>确认写入</button>
          </form>
          <aside className="backfill-warning compact" aria-label="补拉提示"><AlertTriangle aria-hidden="true" /><span>预览会真实拉取、解密并判重，但不写数据库；已有 tid 不覆盖。</span></aside>
          {preview && <YouzanDistributionBackfillResultView title="预览结果" result={preview} />}
        </Panel>
        <Panel title="补拉结果" icon={<CheckCircle2 />} meta={confirmed ? `${youzanDistributionTimeFilterLabel(confirmed.time_filter)} / ${confirmed.start_time} ~ ${confirmed.end_time}` : '等待本次写入'}>
          {confirmed ? <YouzanDistributionBackfillResultView title="写入结果" result={confirmed} /> : <EmptyState text="当前接口未提供补拉历史列表；完成写入后，这里会保留本次结果。" />}
        </Panel>
      </section>

      {confirmingBackfill && preview && <Modal title="确认写入有赞分销订单" focusKey="confirm" closeDisabled={loading || writingBackfill} onClose={() => { if (!loading && !writingBackfill) setConfirmingBackfill(false) }} footer={<><button type="button" disabled={loading || writingBackfill} onClick={() => setConfirmingBackfill(false)}>返回预览</button><button className="primary" type="button" disabled={loading || writingBackfill} onClick={() => void confirmBackfill()}>{writingBackfill ? '写入中…' : '确认写入'}</button></>}><p>确认写入 {preview.writable_count} 条有赞分销订单？系统会按 tid 判重，已有订单不会覆盖。</p></Modal>}

      {showManualRun && task && (
        <Modal title="运行有赞分销计划任务" onClose={() => { if (!runningManualTask) setShowManualRun(false) }}>
          {!manualRunPayload ? (
            <form className="youzan-backfill-form" onSubmit={prepareManualRun}>
              <p className="backfill-note">此操作会投递异步任务，不直接写入本页。请确认任务编码和时间范围后再继续。</p>
              <label>任务编码<input name="task_code" value={task.code} readOnly /></label>
              <label>
                时间筛选方式
                <select name="time_filter" defaultValue="created">
                  <option value="created">下单时间</option>
                  <option value="success">订单完成时间</option>
                </select>
              </label>
              <Field label="开始时间" name="start_time" type="datetime-local" defaultValue={previousDayDateTimeLocal(false)} required />
              <Field label="结束时间" name="end_time" type="datetime-local" defaultValue={previousDayDateTimeLocal(true)} required />
              <button className="primary" type="submit">继续确认</button>
            </form>
          ) : (
            <div className="view-stack">
              <dl className="task-definition-grid">
                <div><dt>任务编码</dt><dd><code>{task.code}</code></dd></div>
                <div><dt>筛选方式</dt><dd>{youzanDistributionTimeFilterLabel(manualRunPayload.time_filter)}</dd></div>
                <div className="wide"><dt>执行时间范围</dt><dd>{manualRunPayload.start_time} ~ {manualRunPayload.end_time}</dd></div>
              </dl>
              {!manualRunResult && <div className="excel-form-actions">
                <button type="button" onClick={() => setManualRunPayload(null)} disabled={runningManualTask}>返回修改</button>
                <button className="primary" type="button" onClick={() => void confirmManualRun()} disabled={runningManualTask}>{runningManualTask ? '投递中…' : '确认投递任务'}</button>
              </div>}
              {manualRunError && <div className="result-banner error" role="alert">{manualRunError}</div>}
              {manualRunResult && <div className="result-banner" role="status">已投递：任务 ID {manualRunResult.id}，队列 {manualRunResult.queue}，类型 {manualRunResult.type}。</div>}
            </div>
          )}
        </Modal>
      )}
    </div>
  )
}

function YouzanDistributionBackfillResultView({ title, result }: { title: string; result: YouzanDistributionBackfillResult }) {
  return (
    <section className="backfill-result">
      <div className="backfill-result-title">
        <strong>{title}</strong>
        <span>{youzanDistributionTimeFilterLabel(result.time_filter)} / {result.start_time} ~ {result.end_time} / 拉取 {result.fetch_pages} 页</span>
      </div>
      <div className="overview-grid compact">
        <Metric label="有赞返回" value={result.total_count} />
        <Metric label="可写入" value={result.writable_count} />
        <Metric label="已存在" value={result.existing_count} />
        <Metric label="已写入" value={result.saved_count} />
        <Metric label="失败" value={result.failed_count} />
      </div>
      {!result.samples?.length ? <EmptyState text="暂无样例数据。" /> : (
        <div className="data-table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>状态</th>
                <th>订单号</th>
                <th>成功时间</th>
                <th>实付金额</th>
                <th>解密昵称</th>
                <th>说明</th>
              </tr>
            </thead>
            <tbody>
              {result.samples.map((sample, index) => (
                <tr key={`${sample.tid}-${sample.status}-${index}`}>
                  <td>{youzanDistributionBackfillStatusLabel(sample.status)}</td>
                  <td>{sample.tid || '-'}</td>
                  <td>{sample.success_time || '-'}</td>
                  <td>{sample.payment || '-'}</td>
                  <td>{sample.fans_nickname || '-'}</td>
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

function DestinationsQueryPage({ client, refreshVersion }: { client: ApiClient; refreshVersion: number }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [destinationType, setDestinationType] = useState('')
  const [applied, setApplied] = useState({ keyword: '', enabled: '' as '' | 'true' | 'false', destinationType: '' })
  const [page, setPage] = useState(1)
  const [reloadVersion, setReloadVersion] = useState(0)
  const [draft, setDraft] = useState<DestinationDraft | null>(null)
  const [saving, setSaving] = useState(false)
  const [testingID, setTestingID] = useState<number | null>(null)
  const [pendingTest, setPendingTest] = useState<DestinationDefinition | null>(null)
  const [message, setMessage] = useState('')
  const listQuery = useMemo(() => buildDestinationListQuery({ page, pageSize: 20, ...applied }), [applied, page])
  const { recordsPage, loading, error } = useConfigurationListPage<DestinationDefinition>(client, '/v1/destinations', 'destinations', listQuery, reloadVersion + refreshVersion)
  const listedDestinations = recordsPage?.list ?? []
  const pagination = recordsPage?.pagination

  function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setApplied({ keyword: query, enabled: status === 'enabled' ? 'true' : status === 'disabled' ? 'false' : '', destinationType })
    setReloadVersion((version) => version + 1)
  }

  async function openDetail(id: number) {
    setMessage('')
    const response = await client(`/v1/destinations/${id}`, { method: 'GET', showResult: false, silentLoading: true })
    const destination = response.ok ? readObject<DestinationDefinition>(response, 'destination') : null
    if (!destination) { setMessage(response.error?.message || '推送目标详情暂时不可用。'); return }
    setDraft(destinationDraftFrom(destination))
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft || saving) return
    if (!draft.name.trim() || !draft.code.trim()) { setMessage('请填写目标名称和编码。'); return }
    try { JSON.parse(draft.configJSON) } catch { setMessage('配置必须是有效 JSON。'); return }
    setSaving(true)
    const response = await client(draft.id ? `/v1/destinations/${draft.id}` : '/v1/destinations', { method: draft.id ? 'PUT' : 'POST', showResult: false, silentLoading: true, body: { name: draft.name.trim(), code: draft.code.trim(), destination_type: draft.destinationType, config_json: draft.configJSON, enabled: draft.enabled } })
    setSaving(false)
    if (!response.ok) { setMessage(response.error?.message || '推送目标保存未完成。'); return }
    setDraft(null)
    setMessage('推送目标已保存。')
    setReloadVersion((version) => version + 1)
  }

  async function test(destination: DestinationDefinition) {
    if (testingID !== null) return
    setTestingID(destination.id)
    const response = await client(`/v1/destinations/${destination.id}/test`, { method: 'POST', showResult: false, silentLoading: true })
    setTestingID(null)
    setMessage(response.ok ? '连通性测试通过；仅发送了无业务载荷的 HEAD 或 GET 请求。' : response.error?.message || '连通性测试未完成。')
  }
  return (
    <div className="view-stack">
      {message && <div className="result-banner" role="status">{message}</div>}
      <form className="query-bar" onSubmit={submitQuery}><div className="query-fields">
        <Field label="名称 / 编码" name="destination_query" value={query} onChange={setQuery} />
        <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
        <Field label="类型" name="destination_type" value={destinationType} onChange={setDestinationType} />
      </div><button type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button></form>
      {error && <div className="result-banner error" role="alert">{error}{recordsPage ? ' 已保留最近一次成功数据。' : ''}</div>}
      <div className="record-actions"><button type="button" className="primary" onClick={() => setDraft({ id: null, name: '', code: '', destinationType: 'http', configJSON: '{\n  "url": "",\n  "method": "POST"\n}', enabled: true, hasSecret: false })}>新增目标</button></div>
      <Panel title="推送目标" icon={<Send />} meta={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}><DestinationList destinations={listedDestinations} testingID={testingID} onDetail={(item) => { void openDetail(item.id) }} onTest={setPendingTest} /><MonitoringPaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading || testingID !== null} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} /></Panel>
      {draft && <Modal title={draft.id ? '推送目标详情与编辑' : '新增推送目标'} onClose={() => { if (!saving) setDraft(null) }}>
        {draft.hasSecret && <div className="result-banner" role="status">配置中的敏感值已隐藏。保留“[已隐藏]”会保留原值；改为新值即可轮换，且不会回显旧值。</div>}
        <form className="excel-upload-form" onSubmit={save}>
          <Field label="目标名称" name="destination_name" value={draft.name} required onChange={(name) => setDraft({ ...draft, name })} />
          <Field label="目标编码" name="destination_code" value={draft.code} required onChange={(code) => setDraft({ ...draft, code })} />
          <label>目标类型<select value={draft.destinationType} disabled={saving} onChange={(event) => setDraft({ ...draft, destinationType: event.currentTarget.value })}><option value="http">http</option><option value="soap">soap</option></select></label>
          <label className="checkbox-label"><input type="checkbox" checked={draft.enabled} disabled={saving} onChange={(event) => setDraft({ ...draft, enabled: event.currentTarget.checked })} />启用目标</label>
          <label>配置 JSON<textarea rows={10} value={draft.configJSON} disabled={saving} onChange={(event) => setDraft({ ...draft, configJSON: event.currentTarget.value })} /></label>
          <div className="excel-form-actions"><button className="primary" type="submit" disabled={saving}>{saving ? '保存中…' : '保存目标'}</button></div>
        </form>
      </Modal>}
      {pendingTest && <Modal title="确认测试推送目标" onClose={() => { if (testingID === null) setPendingTest(null) }} footer={<><button type="button" disabled={testingID !== null} onClick={() => setPendingTest(null)}>取消</button><button className="primary" type="button" disabled={testingID !== null} onClick={() => { const target = pendingTest; setPendingTest(null); void test(target) }}>{testingID === pendingTest.id ? '测试中…' : '确认测试'}</button></>}><p>将向“{pendingTest.name}”配置的目标地址发起真实连通性请求。系统仅允许无业务载荷的 HEAD 或 GET；请确认目标的 GET 接口无副作用。确认继续？</p></Modal>}
    </div>
  )
}

function DeliveryTasksQueryPage({ client, sources, destinations, onRefresh, refreshVersion }: { client: ApiClient; sources: SourceDefinition[]; destinations: DestinationDefinition[]; onRefresh: () => Promise<void>; refreshVersion: number }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [destinationID, setDestinationID] = useState('all')
  const [applied, setApplied] = useState({ keyword: '', enabled: '' as '' | 'true' | 'false', destinationID: '' })
  const [page, setPage] = useState(1)
  const [reloadVersion, setReloadVersion] = useState(0)
  const [draft, setDraft] = useState<DeliveryTaskDraft | null>(null)
  const destinationOptions = destinations.map((destination) => ({ value: String(destination.id), label: destination.name || destination.code }))
  const [runningID, setRunningID] = useState<number | null>(null)
  const [pendingRun, setPendingRun] = useState<DeliveryTask | null>(null)
  const [loadingDetailID, setLoadingDetailID] = useState<number | null>(null)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')
  const listQuery = useMemo(() => buildDeliveryTaskListQuery({ page, pageSize: 20, ...applied }), [applied, page])
  const { recordsPage, loading, error } = useConfigurationListPage<DeliveryTask>(client, '/v1/delivery-tasks', 'tasks', listQuery, reloadVersion + refreshVersion)
  const listedTasks = recordsPage?.list ?? []
  const pagination = recordsPage?.pagination

  function submitQuery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setApplied({ keyword: query, enabled: status === 'enabled' ? 'true' : status === 'disabled' ? 'false' : '', destinationID: destinationID === 'all' ? '' : destinationID })
    setReloadVersion((version) => version + 1)
  }

  function openCreate() {
    setMessage('')
    setDraft({
      id: null,
      name: '',
      sourceID: sources[0]?.id ? String(sources[0].id) : '',
      cleanTable: '',
      destinationID: destinations[0]?.id ? String(destinations[0].id) : '',
      triggerType: 'manual',
      cronExpr: '',
      filterJSON: '{}',
      payloadTemplate: '',
      enabled: true,
    })
  }

  async function openDetail(id: number) {
    if (loadingDetailID !== null) return
    setMessage('')
    setLoadingDetailID(id)
    const response = await client(`/v1/delivery-tasks/${id}`, { method: 'GET', showResult: false, silentLoading: true })
    setLoadingDetailID(null)
    const task = response.ok ? readObject<DeliveryTask>(response, 'task') : null
    if (!task) {
      setMessage(response.error?.message || '推送任务详情暂时不可用，请稍后重试。')
      return
    }
    setDraft(deliveryTaskDraftFrom(task))
  }

  async function saveDraft(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft || saving) return
    const sourceID = Number(draft.sourceID)
    const nextDestinationID = Number(draft.destinationID)
    if (!draft.name.trim() || !draft.cleanTable.trim() || !Number.isInteger(sourceID) || sourceID <= 0 || !Number.isInteger(nextDestinationID) || nextDestinationID <= 0) {
      setMessage('请填写任务名称、来源、清洗表和推送目标。')
      return
    }
    if (draft.triggerType === 'schedule' && !draft.cronExpr.trim()) {
      setMessage('定时触发任务必须填写 Cron 表达式。')
      return
    }
    try {
      const filter = JSON.parse(draft.filterJSON) as unknown
      if (!filter || Array.isArray(filter) || typeof filter !== 'object') {
        setMessage('筛选条件必须是 JSON 对象。')
        return
      }
    } catch {
      setMessage('筛选条件必须是有效 JSON。')
      return
    }
    setSaving(true)
    setMessage('')
    const response = await client(draft.id ? `/v1/delivery-tasks/${draft.id}` : '/v1/delivery-tasks', {
      method: draft.id ? 'PUT' : 'POST',
      showResult: false,
      silentLoading: true,
      body: {
        name: draft.name.trim(),
        source_id: sourceID,
        clean_table: draft.cleanTable.trim(),
        destination_id: nextDestinationID,
        trigger_type: draft.triggerType,
        cron_expr: draft.cronExpr.trim(),
        filter_json: draft.filterJSON,
        payload_template: draft.payloadTemplate,
        enabled: draft.enabled,
      },
    })
    setSaving(false)
    if (!response.ok || !readObject<DeliveryTask>(response, 'task')) {
      setMessage(response.error?.message || '推送任务保存未完成，请稍后重试。')
      return
    }
    setDraft(null)
    setMessage('推送任务已保存。')
    await onRefresh()
    setReloadVersion((version) => version + 1)
  }

  async function run(task: DeliveryTask) {
    if (runningID !== null) return
    setRunningID(task.id)
    const response = await client(`/v1/delivery-tasks/${task.id}/run`, { method: 'POST', showResult: false, silentLoading: true })
    const result = response.ok ? readObject<{ total_count: number; success_count: number; failed_count: number; skipped_count: number }>(response, 'result') : null
    setRunningID(null)
    setMessage(result ? `执行完成：总计 ${result.total_count}，成功 ${result.success_count}，失败 ${result.failed_count}，跳过 ${result.skipped_count}。` : response.error?.message || '推送任务未完成。')
    if (response.ok) {
      await onRefresh()
      setReloadVersion((version) => version + 1)
    }
  }
  return (
    <div className="view-stack">
      {message && <div className="result-banner" role="status">{message}</div>}
      <form className="query-bar" onSubmit={submitQuery}><div className="query-fields">
        <Field label="名称 / 表 / 触发方式" name="task_query" value={query} onChange={setQuery} />
        <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
        <SelectFilter label="推送目标" value={destinationID} onChange={setDestinationID} options={destinationOptions} />
      </div><button type="submit" disabled={loading || runningID !== null}>{loading ? '查询中…' : '查询'}</button></form>
      {error && <div className="result-banner error" role="alert">{error}{recordsPage ? ' 已保留最近一次成功数据。' : ''}</div>}
      <div className="record-actions"><button type="button" className="primary" onClick={openCreate}>新增推送任务</button></div>
      <Panel title="推送任务" icon={<ArrowUpFromLine />} meta={loading && !recordsPage ? '正在加载…' : `共 ${pagination?.total ?? 0} 条`}><DeliveryTaskList tasks={listedTasks} runningID={runningID} loadingDetailID={loadingDetailID} destinations={destinations} onDetail={(task) => { void openDetail(task.id) }} onRun={setPendingRun} /><MonitoringPaginationControls page={pagination?.page ?? page} totalPages={pagination?.totalPages ?? 0} loading={loading || runningID !== null || loadingDetailID !== null} onPrevious={() => setPage((current) => Math.max(1, current - 1))} onNext={() => setPage((current) => current + 1)} /></Panel>
      {draft && <Modal title={draft.id ? '推送任务详情与编辑' : '新增推送任务'} onClose={() => { if (!saving) setDraft(null) }}>
        <form className="excel-upload-form" onSubmit={saveDraft}>
          <Field label="任务名称" name="delivery_task_name" value={draft.name} required onChange={(name) => setDraft({ ...draft, name })} />
          <label>来源
            <select value={draft.sourceID} required disabled={saving} onChange={(event) => setDraft({ ...draft, sourceID: event.currentTarget.value })}>
              <option value="">选择数据源</option>
              {sources.map((source) => <option value={source.id} key={source.id}>#{source.id} {source.name || source.code}{source.enabled ? '' : '（已停用）'}</option>)}
            </select>
          </label>
          <Field label="清洗结果表" name="delivery_clean_table" value={draft.cleanTable} required onChange={(cleanTable) => setDraft({ ...draft, cleanTable })} />
          <label>推送目标
            <select value={draft.destinationID} required disabled={saving} onChange={(event) => setDraft({ ...draft, destinationID: event.currentTarget.value })}>
              <option value="">选择推送目标</option>
              {destinations.map((destination) => <option value={destination.id} key={destination.id}>#{destination.id} {destination.name || destination.code}{destination.enabled ? '' : '（已停用）'}</option>)}
            </select>
          </label>
          <label>触发方式
            <select value={draft.triggerType} disabled={saving} onChange={(event) => setDraft({ ...draft, triggerType: event.currentTarget.value })}>
              <option value="manual">手动</option>
              <option value="schedule">定时</option>
              <option value="event">事件</option>
            </select>
          </label>
          <Field label="Cron 表达式" name="delivery_task_cron" value={draft.cronExpr} required={draft.triggerType === 'schedule'} onChange={(cronExpr) => setDraft({ ...draft, cronExpr })} />
          <label className="checkbox-label"><input type="checkbox" checked={draft.enabled} disabled={saving} onChange={(event) => setDraft({ ...draft, enabled: event.currentTarget.checked })} />启用任务</label>
          <label>筛选条件 JSON<textarea rows={8} value={draft.filterJSON} disabled={saving} onChange={(event) => setDraft({ ...draft, filterJSON: event.currentTarget.value })} /></label>
          <label>推送载荷模板<textarea rows={8} value={draft.payloadTemplate} disabled={saving} onChange={(event) => setDraft({ ...draft, payloadTemplate: event.currentTarget.value })} /></label>
          <p className="query-contract-note">完整保存会覆盖任务配置；筛选条件仅接受 JSON 对象。手动运行请从列表单独确认执行。</p>
          <div className="excel-form-actions"><button className="primary" type="submit" disabled={saving}>{saving ? '保存中…' : '保存任务'}</button></div>
        </form>
      </Modal>}
      {pendingRun && <Modal title="确认执行推送任务" onClose={() => { if (runningID === null) setPendingRun(null) }} footer={<><button type="button" disabled={runningID !== null} onClick={() => setPendingRun(null)}>取消</button><button className="primary" type="button" disabled={runningID !== null} onClick={() => { const task = pendingRun; setPendingRun(null); void run(task) }}>{runningID === pendingRun.id ? '执行中…' : '确认执行'}</button></>}><p>将执行“{pendingRun.name}”，最多向 {destinations.find((item) => item.id === pendingRun.destination_id)?.name || `目标 #${pendingRun.destination_id}`} 推送 100 条 ready 记录；成功记录将标记为已交付。</p></Modal>}
    </div>
  )
}

function PushPolicyPage({ coreMethod, config, targets, onSave, onToggle }: {
  coreMethod?: CoreMethod
  config: OrderPushSkipConfig
  targets: OrderPushTargetOption[]
  onSave: (config: OrderPushSkipConfig) => Promise<ApiResult>
  onToggle: (target: ToggleTarget, enabled: boolean) => void
}) {
  return (
    <div className="view-stack">
      {coreMethod && <Panel title="商场推送方法" icon={<Send />} meta="当前推送能力"><CoreMethodList methods={[coreMethod]} onToggle={onToggle} /></Panel>}
      <Panel title="订单少推送配置" icon={<ListChecks />} meta="按具体目标独立配置"><OrderPushSkipConfigForm config={config} targets={targets} onSave={onSave} /></Panel>
    </div>
  )
}

function OrderPushSkipConfigForm({ config, targets, onSave }: { config: OrderPushSkipConfig; targets: OrderPushTargetOption[]; onSave: (config: OrderPushSkipConfig) => Promise<ApiResult> }) {
  const [draft, setDraft] = useState(() => targets.map((target) => orderPushTargetConfig(config, target.code)))
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const saveInFlightRef = useRef(false)
  const enabledCount = draft.filter((target) => target.cycle > 0 && target.skip > 0).length

  useEffect(() => setDraft(targets.map((target) => orderPushTargetConfig(config, target.code))), [config, targets])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const validationError = validateOrderPushSkipPolicy(draft)
    if (validationError) {
      setError(validationError)
      return
    }
    setError('')
    const responsePromise = runSingleFlight(saveInFlightRef, () => onSave({ targets: draft }))
    if (!responsePromise) return
    setSaving(true)
    try {
      const response = await responsePromise
      if (!response.ok) setError(response.error?.message || '配置保存未完成，请稍后重试。')
    } catch {
      setError('配置保存未完成，请稍后重试。')
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className="push-skip-form" onSubmit={submit}>
      <fieldset disabled={saving}>
        <div className="push-skip-summary">
          <StatusPill label={enabledCount > 0 ? `已启用 ${enabledCount} 个目标` : '未启用'} />
          <span>只对下方配置的推送目标生效；未配置或填 0 的目标不少推。</span>
        </div>
        {targets.length === 0 ? <EmptyState text="后端未返回可配置推送目标。" /> : <div className="push-skip-list">
          {targets.map((target, index) => {
            const value = draft[index] ?? { target_code: target.code, target_name: target.name, cycle: 0, skip: 0 }
            const ratio = value.cycle > 0 ? `${(((value.cycle - value.skip) / value.cycle) * 100).toFixed(1)}%` : '100.0%'
            return (
              <div className="push-skip-row" key={target.code}>
                <div>
                  <strong>{target.name}</strong>
                  <span>{target.code}</span>
                </div>
                <Field label="循环总单数" name={`cycle_${index}`} value={String(value.cycle)} type="number" onChange={(raw) => setDraft((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, cycle: Number(raw) } : item))} />
                <Field label="少推单数" name={`skip_${index}`} value={String(value.skip)} type="number" onChange={(raw) => setDraft((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, skip: Number(raw) } : item))} />
                <small>预计推送比例：{ratio}</small>
              </div>
            )
          })}
        </div>}
        {error && <div className="result-banner error" role="alert">{error}</div>}
        <button className="primary" type="submit">{saving ? '保存中…' : '保存配置'}</button>
      </fieldset>
    </form>
  )
}

function ExcelMatchView({
  section,
  client,
  token,
  loading,
  setLoading,
  setResult,
  onNavigateToJobs,
}: {
  section: 'jobs' | 'schemes' | 'write'
  client: ApiClient
  token: string
  loading: boolean
  setLoading: (value: boolean) => void
  setResult: (value: ApiResult | null) => void
  onNavigateToJobs: () => void
}) {
  const pendingJobFocusIDRef = useRef<number | null>(null)
  const jobExecutionRef = useRef<HTMLDivElement>(null)
  const [jobID, setJobID] = useState('')
  const [job, setJob] = useState<ExcelMatchJob | null>(null)
  const [jobHistory, setJobHistory] = useState<ExcelMatchJob[]>([])
  const [jobLogs, setJobLogs] = useState<ExcelMatchJobLog[]>([])
  const [trackingJobID, setTrackingJobID] = useState<number | null>(null)
  const [autoRefreshText, setAutoRefreshText] = useState('')
  const [downloadingJobID, setDownloadingJobID] = useState<number | null>(null)
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
  const [exportSteps, setExportSteps] = useState<ExcelMatchStepConfig[]>(cloneExcelMatchSteps(defaultExcelExportScheme.steps))
  const [excelModels, setExcelModels] = useState<ExcelMatchModel[]>([])
  const [excelModelsLoading, setExcelModelsLoading] = useState(false)
  const [excelModelsError, setExcelModelsError] = useState('')
  const [importDefaults, setImportDefaults] = useState<ExcelImportSchemeConfig>(defaultExcelImportScheme)
  const [exportFormKey, setExportFormKey] = useState(0)
  const [importFormKey, setImportFormKey] = useState(0)
  const [selectedExportSchemeID, setSelectedExportSchemeID] = useState('')
  const [selectedImportSchemeID, setSelectedImportSchemeID] = useState('')
  const [pendingSchemeDelete, setPendingSchemeDelete] = useState<ExcelMatchScheme | null>(null)
  const [pendingSchemeSave, setPendingSchemeSave] = useState<PendingSchemeSave | null>(null)
  const [schemeSaveError, setSchemeSaveError] = useState('')
  const schemeSaveInFlightRef = useRef(false)
  const [deletingSchemeID, setDeletingSchemeID] = useState<number | null>(null)
  const [pendingWrite, setPendingWrite] = useState<{ slot: 'import' | 'clear'; file: File; config: unknown; message: string } | null>(null)
  const [writeMode, setWriteMode] = useState<'import' | 'clear'>('import')
  const [latestWriteJob, setLatestWriteJob] = useState<ExcelMatchJob | null>(null)
  const [jobQuery, setJobQuery] = useState('')
  const [jobStatus, setJobStatus] = useState('all')
  const [appliedJobHistoryFilters, setAppliedJobHistoryFilters] = useState({ keyword: '', status: '' })
  const [jobHistoryPage, setJobHistoryPage] = useState(1)
  const [jobHistoryPagination, setJobHistoryPagination] = useState<MonitoringPagination | null>(null)
  const [jobHistoryLoading, setJobHistoryLoading] = useState(false)
  const [jobHistoryError, setJobHistoryError] = useState('')
  const [jobHistoryReloadVersion, setJobHistoryReloadVersion] = useState(0)
  const jobHistoryRequestRef = useRef<AbortController | null>(null)
  const focusedJobID = job?.id
  const selectedJobProgress = job && job.total_rows > 0
    ? Math.min(100, Math.round(job.processed_rows / job.total_rows * 100))
    : 0
  const pendingSchemeNameConflict = pendingSchemeSave
    ? (pendingSchemeSave.operation === 'export_match' ? exportSchemes : importSchemes)
      .find((scheme) => scheme.name === pendingSchemeSave.name.trim()) ?? null
    : null
  const requestErrorMessage = (response: ApiResult, fallback: string) => response.error?.message || fallback

  const applyJobResult = useCallback((result: ApiResult, options: { track?: boolean } = {}) => {
    const nextJob = readObject<ExcelMatchJob>(result, 'job')
    if (nextJob) {
      setJob(nextJob)
      setJobID(String(nextJob.id))
      setJobHistory((current) => replaceExcelJobHistoryItem(current, nextJob))
      if (options.track !== false) {
        setTrackingJobID(isExcelJobActive(nextJob) ? nextJob.id : null)
      }
    }
    setJobLogs(readList<ExcelMatchJobLog>(result, 'logs'))
    return nextJob
  }, [])

  function clearUploadRef(slot: ExcelUploadSlot) {
    setUploadRefs((current) => ({ ...current, [slot]: undefined }))
    setUploadProgress('')
  }

  function resetExcelDialogFiles() {
    setSelectedExportFileName('')
    setSelectedImportFileName('')
    setSelectedClearFileName('')
    setPreviewResult(null)
    setUploadRefs({})
    setUploadProgress('')
  }

  function openExcelDialog(mode: ExcelDialogMode) {
    setPendingWrite(null)
    setPendingSchemeSave(null)
    setSchemeSaveError('')
    resetExcelDialogFiles()
    setExcelDialog(mode)
  }

  function closeExcelDialog() {
    setPendingWrite(null)
    setPendingSchemeSave(null)
    setSchemeSaveError('')
    setExcelDialog(null)
    resetExcelDialogFiles()
  }

  function buildExportConfig(form: FormData) {
    return buildExcelExportConfig({
      sheetName: formValue(form, 'sheetName').trim() || 'Sheet1',
      steps: exportSteps,
      exportColumnFormats: parseExportColumnFormats(formValue(form, 'exportColumnFormats')),
      batchSize: Number(formValue(form, 'batchSize') || 1000),
    })
  }

  function updateExportStep(index: number, key: Exclude<keyof ExcelMatchStepConfig, 'filters'>, value: string) {
    setExportSteps((current) => current.map((step, stepIndex) => stepIndex === index ? { ...step, [key]: value } : step))
  }

  function selectExportStepModel(index: number, tableName: string) {
    setExportSteps((current) => current.map((step, stepIndex) => stepIndex === index
      ? selectExcelMatchStepModel(step, tableName, excelModels)
      : step))
  }

  function updateExportStepFilter(stepIndex: number, filterIndex: number, key: keyof ExcelMatchFilterConfig, value: string) {
    setExportSteps((current) => current.map((step, currentStepIndex) => currentStepIndex === stepIndex
      ? {
          ...step,
          filters: step.filters.map((filter, currentFilterIndex) => currentFilterIndex === filterIndex ? { ...filter, [key]: value } : filter),
        }
      : step))
  }

  function addExportStepFilter(stepIndex: number) {
    setExportSteps((current) => current.map((step, currentStepIndex) => currentStepIndex === stepIndex
      ? { ...step, filters: [...step.filters, { column: '', op: 'eq', value: '' }] }
      : step))
  }

  function removeExportStepFilter(stepIndex: number, filterIndex: number) {
    setExportSteps((current) => current.map((step, currentStepIndex) => currentStepIndex === stepIndex
      ? { ...step, filters: step.filters.filter((_, currentFilterIndex) => currentFilterIndex !== filterIndex) }
      : step))
  }

  function addExportStep() {
    setExportSteps((current) => {
      if (current.length >= 20) return current
      return [...current, {
        name: `步骤 ${current.length + 1}`,
        filters: [],
        matchMode: 'field',
        tableName: '',
        matchExcelColumn: current[current.length - 1]?.outputColumnName ?? '',
        dbMatchField: '',
        dbValueField: '',
        outputColumnName: '',
        specExcelColumn: '',
        priceExcelColumn: '',
        qtyExcelColumn: '',
      }]
    })
  }

  function removeExportStep(index: number) {
    setExportSteps((current) => current.length === 1 ? current : current.filter((_, stepIndex) => stepIndex !== index))
  }

  function moveExportStep(index: number, direction: -1 | 1) {
    setExportSteps((current) => {
      const nextIndex = index + direction
      if (nextIndex < 0 || nextIndex >= current.length) return current
      const next = [...current]
      ;[next[index], next[nextIndex]] = [next[nextIndex], next[index]]
      return next
    })
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
    const response = await client(`/v1/excel-match-jobs/schemes?operation=${operation}`, { method: 'GET', showResult: false, silentLoading: true })
    if (!response.ok) throw new Error(requestErrorMessage(response, '查询 Excel 方案失败'))
    const value = readDataField(response.data, 'schemes')
    return Array.isArray(value) ? (value as ExcelMatchScheme[]) : []
  }, [client])

  const loadExcelModels = useCallback(async () => {
    setExcelModelsLoading(true)
    setExcelModelsError('')
    try {
      const response = await client('/v1/excel-match-jobs/models', { method: 'GET', showResult: false, silentLoading: true })
      if (!response.ok) throw new Error(requestErrorMessage(response, '查询模型与字段目录失败'))
      const value = readDataField(response.data, 'models')
      setExcelModels(Array.isArray(value) ? filterSensitiveExcelModels(value as ExcelMatchModel[]) : [])
    } catch (error) {
      setExcelModelsError(error instanceof Error ? error.message : '查询模型与字段目录失败')
    } finally {
      setExcelModelsLoading(false)
    }
  }, [client])

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

  const loadJobHistory = useCallback(async () => {
    jobHistoryRequestRef.current?.abort()
    const controller = new AbortController()
    jobHistoryRequestRef.current = controller
    setJobHistoryLoading(true)
    setJobHistoryError('')
    const query = buildExcelMatchJobListQuery({ page: jobHistoryPage, pageSize: 20, ...appliedJobHistoryFilters })
    try {
      const response = await client(`/v1/excel-match-jobs?${query}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
      if (controller.signal.aborted) return
      const parsed = response.ok ? parseMonitoringPage<ExcelMatchJob>(response.data, 'jobs') : null
      if (parsed) {
        const nextPage = normalizeMonitoringPageNumber(jobHistoryPage, parsed.pagination.totalPages)
        if (nextPage !== jobHistoryPage) {
          setJobHistoryPage(nextPage)
          return
        }
        setJobHistory(parsed.list)
        setJobHistoryPagination(parsed.pagination)
        return
      }
      const legacyItems = readDataField(response.data, 'jobs')
      if (response.ok && Array.isArray(legacyItems)) {
        const pageSize = 20
        if (jobHistoryPage !== 1) {
          setJobHistoryPage(1)
          return
        }
        setJobHistory(legacyItems.slice(0, pageSize) as ExcelMatchJob[])
        setJobHistoryPagination({ page: 1, pageSize, total: legacyItems.length, totalPages: legacyItems.length ? 1 : 0 })
        setJobHistoryError('当前服务暂不支持 Excel 任务分页或筛选，已显示未筛选的兼容数据。')
        return
      }
      setJobHistoryError(response.error?.message || 'Excel 任务历史暂时不可用，请稍后重试。')
    } finally {
      if (!controller.signal.aborted) setJobHistoryLoading(false)
    }
  }, [appliedJobHistoryFilters, client, jobHistoryPage])

  useEffect(() => {
    if (!token) return
    if (section === 'jobs') void loadJobHistory()
    if (section === 'schemes' || section === 'write') void loadSchemes()
    return () => jobHistoryRequestRef.current?.abort()
  }, [jobHistoryReloadVersion, loadJobHistory, loadSchemes, section, token])

  useEffect(() => {
    if (!token || section !== 'schemes') return
    void loadExcelModels()
  }, [loadExcelModels, section, token])

  useEffect(() => {
    setPendingWrite(null)
    setExcelDialog(null)
    setSelectedExportFileName('')
    setSelectedImportFileName('')
    setSelectedClearFileName('')
    setPreviewResult(null)
    setUploadRefs({})
    setUploadProgress('')
  }, [section])

  useEffect(() => {
    if (section !== 'jobs' || !focusedJobID || pendingJobFocusIDRef.current !== focusedJobID) return

    const animationFrame = window.requestAnimationFrame(() => {
      const target = jobExecutionRef.current
      if (!target || pendingJobFocusIDRef.current !== focusedJobID) return
      pendingJobFocusIDRef.current = null
      const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
      target.scrollIntoView({ block: 'start', behavior: reduceMotion ? 'auto' : 'smooth' })
      target.focus({ preventScroll: true })
    })

    return () => window.cancelAnimationFrame(animationFrame)
  }, [focusedJobID, section])

  const refreshJobByID = useCallback(async (id: number, options: { silent?: boolean; track?: boolean; signal?: AbortSignal } = {}) => {
    if (!options.silent) setLoading(true)
    try {
      const nextResult = await client(`/v1/excel-match-jobs/${id}`, { method: 'GET', signal: options.signal, showResult: false, silentLoading: true })
      if (!options.silent && !nextResult.ok) setResult(nextResult)
      if (nextResult.ok) {
        applyJobResult(nextResult, { track: options.track })
        if (!options.silent) await loadJobHistory()
        return readObject<ExcelMatchJob>(nextResult, 'job')
      }
      if (options.silent) {
        setAutoRefreshText(`自动刷新失败：${requestErrorMessage(nextResult, '请稍后重试。')}`)
      }
    } catch {
      if (options.signal?.aborted) return null
      if (!options.silent) {
        setResult({ ok: false, status: 0, data: { message: '查询 Excel 任务失败，请稍后重试。' } })
      } else {
        setAutoRefreshText('自动刷新失败，请稍后重试。')
      }
    } finally {
      if (!options.silent) setLoading(false)
    }
    return null
  }, [applyJobResult, client, loadJobHistory, setLoading, setResult])

  useEffect(() => {
    if (!token || !trackingJobID) return

    let cancelled = false
    let attempts = 0
    let consecutiveFailures = 0
    let timer: number | null = null
    let inFlight = false
    const pollingState = { resumeWhenVisible: false }
    const controller = new AbortController()
    const isPageVisible = () => document.visibilityState !== 'hidden'

    const clearScheduledRefresh = () => {
      if (timer !== null) {
        window.clearTimeout(timer)
        timer = null
      }
    }

    const stopPolling = (message: string) => {
      clearScheduledRefresh()
      setAutoRefreshText(message)
      setTrackingJobID(null)
    }

    const scheduleRefresh = (delayMilliseconds: number) => {
      clearScheduledRefresh()
      if (cancelled || document.visibilityState === 'hidden') return
      if (inFlight) return
      timer = window.setTimeout(() => {
        timer = null
        void refreshTrackedJob()
      }, delayMilliseconds)
    }

    const refreshTrackedJob = async () => {
      if (cancelled || document.visibilityState === 'hidden') return
      if (inFlight) {
        pollingState.resumeWhenVisible = true
        return
      }
      if (attempts >= excelJobPollMaxAttempts) {
        stopPolling(`自动刷新已在 ${excelJobPollMaxAttempts} 次后停止，请手动查询任务状态。`)
        return
      }

      attempts += 1
      inFlight = true
      let nextDelay: number | null = null
      try {
        const nextJob = await refreshJobByID(trackingJobID, { silent: true, signal: controller.signal })
        if (cancelled || controller.signal.aborted) return
        if (!nextJob) {
          consecutiveFailures += 1
          const delayMilliseconds = Math.min(30_000, 2_000 * 2 ** Math.min(consecutiveFailures, 4))
          if (attempts >= excelJobPollMaxAttempts) {
            stopPolling(`自动刷新已在 ${excelJobPollMaxAttempts} 次后停止，请手动查询任务状态。`)
            return
          }
          setAutoRefreshText(`自动刷新失败，将在 ${Math.ceil(delayMilliseconds / 1000)} 秒后重试。`)
          nextDelay = delayMilliseconds
          return
        }

        consecutiveFailures = 0
        setAutoRefreshText(`自动刷新中：任务 #${nextJob.id}，${excelJobStatusLabel(nextJob.status)}，${new Date().toLocaleTimeString()}`)
        if (!isExcelJobActive(nextJob)) {
          setTrackingJobID(null)
          void loadJobHistory()
          return
        }
        nextDelay = 2_000
      } finally {
        inFlight = false
        if (pollingState.resumeWhenVisible && !cancelled && !controller.signal.aborted && isPageVisible()) {
          pollingState.resumeWhenVisible = false
          scheduleRefresh(0)
        } else if (nextDelay !== null) {
          scheduleRefresh(nextDelay)
        }
      }
    }

    void refreshTrackedJob()
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        clearScheduledRefresh()
        setAutoRefreshText('页面已隐藏，任务自动刷新已暂停。')
        return
      }
      if (inFlight) {
        pollingState.resumeWhenVisible = true
        return
      }
      scheduleRefresh(0)
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      cancelled = true
      clearScheduledRefresh()
      controller.abort()
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [loadJobHistory, refreshJobByID, token, trackingJobID])

  async function ensureExcelUpload(slot: ExcelUploadSlot, file: File) {
    const existing = uploadRefs[slot]
    if (existing && sameExcelFile(file, existing)) {
      return existing.uploadId
    }

    const totalChunks = Math.ceil(file.size / excelChunkSize)
    setUploadProgress(`准备上传 ${file.name}，共 ${totalChunks} 个分片`)

    const createResult = await client('/v1/excel-match-jobs/uploads', {
      method: 'POST',
      body: { fileName: file.name, totalChunks },
      showResult: false,
      silentLoading: true,
      retry: false,
    })
    if (!createResult.ok) throw new Error(requestErrorMessage(createResult, '创建分片上传会话失败'))
    const session = readObject<ExcelUploadSession>(createResult, 'upload')
    if (!session?.uploadId) throw new Error('上传会话返回缺少 uploadId')

    for (let index = 0; index < totalChunks; index++) {
      const start = index * excelChunkSize
      const end = Math.min(file.size, start + excelChunkSize)
      const chunkForm = new FormData()
      chunkForm.append('index', String(index))
      chunkForm.append('totalChunks', String(totalChunks))
      chunkForm.append('chunk', file.slice(start, end), `${file.name}.part${index}`)
      setUploadProgress(`上传分片 ${index + 1}/${totalChunks}`)
      const chunkResult = await client(`/v1/excel-match-jobs/uploads/${encodeURIComponent(session.uploadId)}/chunks`, {
        method: 'POST',
        body: chunkForm,
        showResult: false,
        silentLoading: true,
        retry: false,
        timeoutMs: 120_000,
      })
      if (!chunkResult.ok) throw new Error(requestErrorMessage(chunkResult, `上传分片 ${index + 1} 失败`))
    }

    setUploadProgress('合并 Excel 分片')
    const completeResult = await client(`/v1/excel-match-jobs/uploads/${encodeURIComponent(session.uploadId)}/complete`, {
      method: 'POST',
      body: { totalChunks },
      showResult: false,
      silentLoading: true,
      retry: false,
    })
    if (!completeResult.ok) throw new Error(requestErrorMessage(completeResult, '合并 Excel 分片失败'))

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

  function beginSchemeSave(formElement: HTMLFormElement, operation: 'export_match' | 'import_update', mode: 'current' | 'new') {
    const selectedSchemeID = operation === 'export_match' ? selectedExportSchemeID : selectedImportSchemeID
    const schemes = operation === 'export_match' ? exportSchemes : importSchemes
    const selectedScheme = schemes.find((item) => String(item.id) === selectedSchemeID)
    const form = new FormData(formElement)
    const config = operation === 'export_match'
      ? buildExportConfig(form)
      : buildImportConfig(form, false)

    if (mode === 'current' && selectedScheme?.name) {
      void persistScheme(operation, config, selectedScheme.name)
      return
    }
    setSchemeSaveError('')
    setPendingSchemeSave({ operation, config, name: '', overwriteConfirmed: false })
  }

  async function persistScheme(operation: 'export_match' | 'import_update', config: unknown, name: string) {
    if (schemeSaveInFlightRef.current) return false
    schemeSaveInFlightRef.current = true
    setLoading(true)
    try {
      const nextResult = await client('/v1/excel-match-jobs/schemes', {
        method: 'POST',
        body: { name: name.trim(), operation, config },
        showResult: false,
        silentLoading: true,
        retry: false,
      })
      setResult({ ok: nextResult.ok, status: nextResult.status, data: { message: nextResult.ok ? 'Excel 方案已保存。' : requestErrorMessage(nextResult, '保存 Excel 方案失败。') }, error: nextResult.error })
      if (nextResult.ok) {
        const savedScheme = readObject<ExcelMatchScheme>(nextResult, 'scheme')
        if (savedScheme?.id) {
          if (operation === 'export_match') {
            setSelectedExportSchemeID(String(savedScheme.id))
          } else {
            setSelectedImportSchemeID(String(savedScheme.id))
          }
        }
        await loadSchemes()
      }
      return nextResult.ok
    } catch {
      setResult({ ok: false, status: 0, data: { message: '保存 Excel 方案失败，请稍后重试。' } })
      return false
    } finally {
      schemeSaveInFlightRef.current = false
      setLoading(false)
    }
  }

  async function confirmPendingSchemeSave() {
    if (!pendingSchemeSave || loading) return
    const name = pendingSchemeSave.name.trim()
    if (name.length < 1 || name.length > 100) {
      setSchemeSaveError('方案名称应为 1 至 100 个字符。')
      return
    }
    if (pendingSchemeNameConflict && !pendingSchemeSave.overwriteConfirmed) {
      setSchemeSaveError('存在同类型同名方案；请确认覆盖后再保存。')
      return
    }
    const saved = await persistScheme(pendingSchemeSave.operation, pendingSchemeSave.config, name)
    if (saved) {
      setPendingSchemeSave(null)
      setSchemeSaveError('')
    }
  }

  async function deleteScheme(scheme: ExcelMatchScheme) {
    setDeletingSchemeID(scheme.id)
    try {
      const nextResult = await client(excelMatchSchemePath(scheme.id), {
        method: 'DELETE',
        showResult: false,
        silentLoading: true,
        retry: false,
      })
      setResult({ ok: nextResult.ok, status: nextResult.status, data: { message: nextResult.ok ? 'Excel 方案已删除。' : requestErrorMessage(nextResult, '删除 Excel 方案失败。') }, error: nextResult.error })
      if (!nextResult.ok) return

      if (scheme.operation === 'export_match' && selectedExportSchemeID === String(scheme.id)) {
        applyExportScheme('')
      }
      if (scheme.operation === 'import_update' && selectedImportSchemeID === String(scheme.id)) {
        applyImportScheme('')
      }
      await loadSchemes()
      setPendingSchemeDelete(null)
    } catch {
      setResult({ ok: false, status: 0, data: { message: '删除 Excel 方案失败，请稍后重试。' } })
    } finally {
      setDeletingSchemeID(null)
    }
  }

  function applyExportScheme(schemeID: string) {
    setSelectedExportSchemeID(schemeID)
    if (!schemeID) {
      setExportDefaults(defaultExcelExportScheme)
      setExportSteps(cloneExcelMatchSteps(defaultExcelExportScheme.steps))
      setExportFormKey((value) => value + 1)
      setPreviewResult(null)
      setSelectedExportFileName('')
      clearUploadRef('export')
      return
    }
    const scheme = exportSchemes.find((item) => String(item.id) === schemeID)
    if (!scheme) return
    const defaults = exportSchemeDefaults(scheme.config)
    setExportDefaults(defaults)
    setExportSteps(cloneExcelMatchSteps(defaults.steps))
    setExportFormKey((value) => value + 1)
    setPreviewResult(null)
    setSelectedExportFileName('')
    clearUploadRef('export')
  }

  function applyImportScheme(schemeID: string) {
    setSelectedImportSchemeID(schemeID)
    if (!schemeID) {
      setImportDefaults(defaultExcelImportScheme)
      setImportFormKey((value) => value + 1)
      setSelectedImportFileName('')
      clearUploadRef('import')
      return
    }
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
      const nextResult = await client('/v1/excel-match-jobs', {
        method: 'POST',
        body: payload,
        showResult: false,
        silentLoading: true,
        retry: false,
        timeoutMs: 120_000,
      })
      if (nextResult.ok) {
        showCreatedJob(nextResult)
      } else {
        setResult(nextResult)
      }
    } catch {
      setResult({ ok: false, status: 0, data: { message: '创建 Excel 导出任务失败，请稍后重试。' } })
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
      const nextResult = await client('/v1/excel-match-jobs/preview', {
        method: 'POST',
        body: payload,
        showResult: false,
        silentLoading: true,
        retry: false,
        timeoutMs: 120_000,
      })
      setResult({ ok: nextResult.ok, status: nextResult.status, data: { message: nextResult.ok ? 'Excel 匹配预览已更新。' : requestErrorMessage(nextResult, '预览 Excel 匹配失败。') }, error: nextResult.error })
      if (nextResult.ok) {
        setPreviewResult(readObject<ExcelMatchPreviewResult>(nextResult, 'preview'))
      }
    } catch {
      setResult({ ok: false, status: 0, data: { message: '预览 Excel 匹配失败，请稍后重试。' } })
    } finally {
      setLoading(false)
    }
  }

  async function createExcelWriteJob(slot: 'import' | 'clear', file: File, config: unknown) {
    setLoading(true)
    try {
      const uploadId = await ensureExcelUpload(slot, file)
      const nextResult = await client('/v1/excel-match-jobs', {
        method: 'POST',
        body: buildConfigPayload(uploadId, config),
        showResult: false,
        silentLoading: true,
        retry: false,
        timeoutMs: 120_000,
      })
      if (nextResult.ok) showCreatedWriteJob(nextResult)
      else setResult(nextResult)
    } catch {
      setResult({ ok: false, status: 0, data: { message: '创建 Excel 写入任务失败，请稍后重试。' } })
    } finally {
      setLoading(false)
    }
  }

  async function confirmPendingWrite() {
    if (!pendingWrite || loading) return
    await createExcelWriteJob(pendingWrite.slot, pendingWrite.file, pendingWrite.config)
    setPendingWrite(null)
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
    const writeField = formValue(form, 'dbWriteField').trim()
    const confirmMessage = writeField === 'completed_at'
      ? '确认写入数据库？本次只会填充为空的订单完成时间，不会覆盖已有 completed_at。'
      : '确认写入数据库？本次只会填充空的 matched_docno，不会覆盖已有匹配单号。'
    const config = buildImportConfig(form, confirmWrite)
    if (confirmWrite) {
      setPendingWrite({ slot: 'import', file, config, message: confirmMessage })
      return
    }
    await createExcelWriteJob('import', file, config)
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
    const config = {
      operation: 'clear_matched_docno',
      sheetName: formValue(form, 'sheetName').trim() || 'Sheet1',
      tableName: formValue(form, 'tableName').trim(),
      dbMatchField: formValue(form, 'dbMatchField').trim(),
      matchExcelColumn: formValue(form, 'matchExcelColumn').trim(),
      dbWriteField: 'matched_docno',
      batchSize: Number(formValue(form, 'batchSize') || 1000),
      dryRun: !confirmWrite,
      confirmWrite,
    }
    if (confirmWrite) {
      setPendingWrite({ slot: 'clear', file, config, message: '确认清空命中行的 matched_docno？该操作会将这些订单退回未匹配状态。' })
      return
    }
    await createExcelWriteJob('clear', file, config)
  }

  function showCreatedJob(result: ApiResult) {
    const createdJob = applyJobResult(result)
    if (createdJob) pendingJobFocusIDRef.current = createdJob.id
    setAutoRefreshText('')
    setResult(null)
    closeExcelDialog()
    setJobHistoryReloadVersion((version) => version + 1)
    onNavigateToJobs()
  }

  function showCreatedWriteJob(result: ApiResult) {
    const createdJob = applyJobResult(result)
    if (createdJob) {
      setLatestWriteJob(createdJob)
      pendingJobFocusIDRef.current = createdJob.id
    }
    setAutoRefreshText('')
    setResult(null)
    setJobHistoryReloadVersion((version) => version + 1)
  }

  async function refreshJob() {
    const id = Number(jobID)
    if (!id) {
      setResult({ ok: false, status: 0, data: '请输入任务 ID' })
      return
    }
    await refreshJobByID(id)
  }

  async function downloadJob(targetID?: number) {
    const id = targetID ?? Number(jobID || job?.id)
    if (!id) {
      setResult({ ok: false, status: 0, data: '请输入任务 ID' })
      return
    }
    const targetJob = job?.id === id ? job : jobHistory.find((item) => item.id === id) ?? null
    if (targetJob && !canDownloadExcelJob(targetJob)) {
      setResult({ ok: false, status: 0, data: targetJob.download_message || '结果文件尚未上传到OSS，上传成功后才能下载，请稍后刷新任务状态' })
      return
    }

    setDownloadingJobID(id)
    setResult({ ok: true, status: 0, data: `正在提交任务 ${id} 的下载请求，浏览器会接管文件下载。` })
    try {
      submitExcelDownloadForm(id, token)
      setResult({ ok: true, status: 0, data: `任务 ${id} 下载请求已提交，请查看浏览器下载栏。` })
      await loadJobHistory()
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    } finally {
      setDownloadingJobID(null)
    }
  }

  function renderPendingSchemeSave() {
    if (!pendingSchemeSave) return null
    return (
      <form className="view-stack" onSubmit={(event) => { event.preventDefault(); void confirmPendingSchemeSave() }}>
        <p>将保存当前表单的配置快照；取消后可继续编辑，已填写内容不会丢失。</p>
        <label>
          方案名称
          <input
            value={pendingSchemeSave.name}
            maxLength={100}
            data-autofocus
            onChange={(event) => {
              const name = event.currentTarget.value
              setPendingSchemeSave((current) => current ? { ...current, name, overwriteConfirmed: false } : current)
              setSchemeSaveError('')
            }}
          />
        </label>
        {pendingSchemeNameConflict && (
          <label className="checkbox-label">
            <input
              type="checkbox"
              checked={pendingSchemeSave.overwriteConfirmed}
              onChange={(event) => setPendingSchemeSave((current) => current ? { ...current, overwriteConfirmed: event.currentTarget.checked } : current)}
            />
            覆盖同类型的“{pendingSchemeNameConflict.name}”方案
          </label>
        )}
        {schemeSaveError && <p className="result-banner error" role="alert">{schemeSaveError}</p>}
        <div className="excel-form-actions">
          <button type="button" disabled={loading} onClick={() => { setPendingSchemeSave(null); setSchemeSaveError('') }}>返回编辑</button>
          <button className="primary" type="submit" disabled={loading}>{loading ? '保存中…' : '保存方案'}</button>
        </div>
      </form>
    )
  }

  function renderExportMatchForm() {
    return (
      <form className="excel-upload-form" onSubmit={createExportJob} key={exportFormKey}>
        <label>已保存方案<select value={selectedExportSchemeID} onChange={(event) => applyExportScheme(event.currentTarget.value)}><option value="">选择方案</option>{exportSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}</select></label>
        <button type="button" disabled={!selectedExportSchemeID || loading} onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) beginSchemeSave(form, 'export_match', 'current') }}>保存到当前方案</button>
        <button type="button" disabled={loading} onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) beginSchemeSave(form, 'export_match', 'new') }}>另存为新方案</button>
        <label className="file-input-label">Excel 文件<input name="file" type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={(event) => { setSelectedExportFileName(event.currentTarget.files?.[0]?.name ?? ''); clearUploadRef('export'); setPreviewResult(null) }} /><span>{selectedExportFileName || '请选择需要匹配导出的 .xlsx 文件'}</span></label>
        <Field label="Sheet 页名称" name="sheetName" defaultValue={exportDefaults.sheetName} />
        <div className="excel-step-editor">
          <div className="excel-step-editor-title"><div><strong>匹配步骤</strong><span>每一步从完整 Excel 行集独立筛选；前一步跳过的行仍可进入后续步骤。</span></div><button type="button" onClick={addExportStep} disabled={exportSteps.length >= 20}>添加步骤</button></div>
          {excelModelsLoading && <p className="excel-mode-note">正在加载模型与字段目录…</p>}
          {excelModelsError && <div className="excel-catalog-error" role="alert"><span>模型与字段目录加载失败：{excelModelsError}</span><button type="button" onClick={() => void loadExcelModels()}>重试加载</button></div>}
          {!excelModelsLoading && !excelModelsError && excelModels.length === 0 && <p className="excel-mode-note">当前数据库没有返回可选择的模型；历史配置仍可查看，但新步骤需要先确认数据库连接和模型表。</p>}
          {exportSteps.map((step, index) => (
            <article className="excel-step-card" key={`${exportFormKey}-${index}`}>
              <div className="excel-step-heading"><strong><span className="excel-step-index" aria-hidden="true">{index + 1}</span>步骤 {index + 1}</strong><div className="table-actions"><button type="button" onClick={() => moveExportStep(index, -1)} disabled={index === 0}>上移</button><button type="button" onClick={() => moveExportStep(index, 1)} disabled={index === exportSteps.length - 1}>下移</button><button type="button" onClick={() => removeExportStep(index)} disabled={exportSteps.length === 1}>删除</button></div></div>
              <div className="excel-step-fields">
                <Field label="步骤名称" name={`step_name_${index}`} value={step.name} onChange={(value) => updateExportStep(index, 'name', value)} required />
                <label>匹配模式<select name={`step_mode_${index}`} value={step.matchMode} onChange={(event) => updateExportStep(index, 'matchMode', event.currentTarget.value)}><option value="field">普通字段匹配</option><option value="order_item_sku">订单商品 SKU 匹配</option></select></label>
                <ExcelModelSelector name={`step_table_${index}`} models={excelModels} value={step.tableName} onChange={(value) => selectExportStepModel(index, value)} />
                <Field label={step.matchMode === 'order_item_sku' ? '订单号 Excel 列' : 'Excel 输入列'} name={`step_excel_${index}`} value={step.matchExcelColumn} onChange={(value) => updateExportStep(index, 'matchExcelColumn', value)} required />
                <ExcelModelFieldSelector label={step.matchMode === 'order_item_sku' ? '数据库订单号字段' : '匹配模型字段'} name={`step_match_${index}`} models={excelModels} tableName={step.tableName} value={step.dbMatchField} onChange={(value) => updateExportStep(index, 'dbMatchField', value)} />
                <ExcelModelFieldSelector label={step.matchMode === 'order_item_sku' ? '数据库购物明细字段' : '取值模型字段'} name={`step_value_${index}`} models={excelModels} tableName={step.tableName} value={step.dbValueField} onChange={(value) => updateExportStep(index, 'dbValueField', value)} />
                <Field label={step.matchMode === 'order_item_sku' ? 'SKU 输出列' : '追加输出列'} name={`step_output_${index}`} value={step.outputColumnName} onChange={(value) => updateExportStep(index, 'outputColumnName', value)} required />
                {step.matchMode === 'order_item_sku' && <><Field label="规格编码 Excel 列" name={`step_spec_${index}`} value={step.specExcelColumn} onChange={(value) => updateExportStep(index, 'specExcelColumn', value)} required /><Field label="销售金额 Excel 列（对应 totAmtActual）" name={`step_price_${index}`} value={step.priceExcelColumn} onChange={(value) => updateExportStep(index, 'priceExcelColumn', value)} required /><Field label="销售数量 Excel 列" name={`step_qty_${index}`} value={step.qtyExcelColumn} onChange={(value) => updateExportStep(index, 'qtyExcelColumn', value)} required /></>}
              </div>
              {step.matchMode === 'order_item_sku' && <p className="excel-mode-note">按数据库购物明细字段匹配并输出完整 no：优先用规格编码匹配 mProductName 前缀，并同时校验销售金额和数量；规格编码为 15 位或 16 位时直接跳过。</p>}
              <div className="excel-step-filter-editor"><div className="excel-step-filter-heading"><div><strong>本步骤筛选</strong><span>{step.matchMode === 'order_item_sku' ? '多条条件需要同时满足，订单商品 SKU 模式仅可引用原始 Excel 列。' : '多条条件需要同时满足，可引用原始列或前序步骤追加列。'}</span></div><button type="button" onClick={() => addExportStepFilter(index)}>添加条件</button></div>{step.filters.length === 0 && <p className="excel-step-filter-empty">未设置条件，本步骤处理全部 Excel 行。</p>}{step.filters.map((filter, filterIndex) => { const valueNotRequired = filter.op === 'empty' || filter.op === 'not_empty'; return <div className="excel-step-filter-row" key={`${exportFormKey}-${index}-${filterIndex}`}><Field label={`条件 ${filterIndex + 1} · Excel 列`} name={`step_filter_column_${index}_${filterIndex}`} value={filter.column} onChange={(value) => updateExportStepFilter(index, filterIndex, 'column', value)} required /><label>运算符<select name={`step_filter_op_${index}_${filterIndex}`} value={filter.op} onChange={(event) => updateExportStepFilter(index, filterIndex, 'op', event.currentTarget.value)}>{excelMatchFilterOperatorOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>{valueNotRequired ? <label>筛选值<input value="此运算无需填写" readOnly disabled /></label> : <Field label="筛选值" name={`step_filter_value_${index}_${filterIndex}`} value={filter.value} onChange={(value) => updateExportStepFilter(index, filterIndex, 'value', value)} required />}<button type="button" onClick={() => removeExportStepFilter(index, filterIndex)} aria-label={`删除步骤 ${index + 1} 的条件 ${filterIndex + 1}`}>删除条件</button></div> })}</div>
            </article>
          ))}
        </div>
        <label>导出列内容格式<textarea name="exportColumnFormats" defaultValue={exportDefaults.exportColumnFormats} rows={4} placeholder={'每行一个：列名=格式\n例如：金额=number\n下单时间=date'} /><small>支持格式：text、number、integer、bool、date。列名可填原 Excel 表头或追加列名。</small></label>
        <Field label="批量查询大小" name="batchSize" defaultValue={exportDefaults.batchSize} />
        {previewResult && <ExcelMatchPreviewPanel preview={previewResult} />}
        {uploadProgress && <p className="excel-mode-note" role="status" aria-live="polite">{uploadProgress}</p>}
        <div className="excel-form-actions"><button type="button" onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) void previewExportJob(form) }} disabled={loading}><FileJson aria-hidden="true" />预览匹配</button><button className="primary" type="submit" disabled={loading}><Upload aria-hidden="true" />创建导出任务</button></div>
      </form>
    )
  }

  function renderImportWriteForm() {
    return <form className="excel-upload-form" onSubmit={createImportJob} key={importFormKey}>
      <label>已保存方案<select value={selectedImportSchemeID} onChange={(event) => applyImportScheme(event.currentTarget.value)}><option value="">选择方案</option>{importSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}</select></label>
      <button type="button" disabled={!selectedImportSchemeID || loading} onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) beginSchemeSave(form, 'import_update', 'current') }}>保存到当前方案</button><button type="button" disabled={loading} onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) beginSchemeSave(form, 'import_update', 'new') }}>另存为新方案</button>
      <label className="file-input-label">Excel 文件<input name="file" type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={(event) => { setSelectedImportFileName(event.currentTarget.files?.[0]?.name ?? ''); clearUploadRef('import') }} /><span>{selectedImportFileName || '请选择需要导入更新的 .xlsx 文件'}</span></label>
      <Field label="Sheet 页名称" name="sheetName" defaultValue={importDefaults.sheetName} /><label>匹配表名<select name="tableName" defaultValue={importDefaults.tableName}><option value="bojun_retail_orders">伯俊零售单 bojun_retail_orders</option></select></label><label>数据库匹配字段<select name="dbMatchField" defaultValue={importDefaults.dbMatchField}>{bojunMatchFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></label><Field label="Excel 匹配列名" name="matchExcelColumn" defaultValue={importDefaults.matchExcelColumn} /><label>写入字段<select name="dbWriteField" defaultValue={importDefaults.dbWriteField}><option value="matched_docno">匹配单号 matched_docno</option><option value="completed_at">订单完成时间 completed_at</option></select></label><Field label="Excel 写入值列名" name="writeExcelColumn" defaultValue={importDefaults.writeExcelColumn} /><Field label="批量更新大小" name="batchSize" defaultValue={importDefaults.batchSize} />
      <label className="checkbox-label"><input name="confirmWrite" type="checkbox" />确认写入数据库</label><p className="excel-mode-note">不勾选时只预检匹配数量，不写库；匹配单号只填充空值；订单完成时间只填充为空的 completed_at。</p>{uploadProgress && <p className="excel-mode-note" role="status" aria-live="polite">{uploadProgress}</p>}<div className="excel-form-actions"><button className="primary" type="submit" disabled={loading}><Upload aria-hidden="true" />创建预检/导入任务</button></div>
    </form>
  }

  function renderClearWriteForm() {
    return <form className="excel-upload-form" onSubmit={createClearMatchedJob}>
      <label className="file-input-label">Excel 文件<input name="file" type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={(event) => { setSelectedClearFileName(event.currentTarget.files?.[0]?.name ?? ''); clearUploadRef('clear') }} /><span>{selectedClearFileName || '请选择需要退回的 .xlsx 文件'}</span></label>
      <Field label="Sheet 页名称" name="sheetName" defaultValue="Sheet1" /><label>匹配表名<select name="tableName" defaultValue="bojun_retail_orders"><option value="bojun_retail_orders">伯俊零售单 bojun_retail_orders</option></select></label><label>数据库匹配字段<select name="dbMatchField" defaultValue="docno">{bojunMatchFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></label><Field label="Excel 匹配列名" name="matchExcelColumn" defaultValue="外部订单编号" /><Field label="批量处理大小" name="batchSize" defaultValue="1000" />
      <label className="checkbox-label"><input name="confirmWrite" type="checkbox" />确认清空 matched_docno</label><p className="excel-mode-note">不勾选时只预检会命中的行；勾选后会把命中行的 matched_docno 清空，用于退回未匹配状态。</p>{uploadProgress && <p className="excel-mode-note" role="status" aria-live="polite">{uploadProgress}</p>}<div className="excel-form-actions"><button type="submit" disabled={loading}><RefreshCcw aria-hidden="true" />创建预检/退回任务</button></div>
    </form>
  }

  return (
    <div className="view-stack">
      {section === 'jobs' && <>
        <section className="overview-grid">
          <Metric label="历史任务" value={jobHistoryPagination?.total ?? jobHistory.length} />
          <Metric label="当前任务" value={job?.id ?? '-'} />
          <Metric label="任务状态" value={job ? excelJobStatusLabel(job.status) : '-'} />
          <Metric label="已处理行" value={job ? `${job.processed_rows}/${job.total_rows}` : '-'} />
          <Metric label="自动跟踪" value={trackingJobID ? `#${trackingJobID}` : '-'} />
        </section>
        <form className="query-bar" onSubmit={(event) => {
          event.preventDefault()
          setJobHistoryPage(1)
          setAppliedJobHistoryFilters({ keyword: jobQuery, status: jobStatus === 'all' ? '' : jobStatus })
          setJobHistoryReloadVersion((version) => version + 1)
        }}>
          <div className="query-fields">
            <Field label="文件名" name="excel_job_query" value={jobQuery} onChange={setJobQuery} />
            <SelectFilter label="状态" value={jobStatus} onChange={setJobStatus} options={[{ value: 'pending', label: '等待处理' }, { value: 'running', label: '处理中' }, { value: 'success', label: '成功' }, { value: 'failed', label: '失败' }, { value: 'expired', label: '已过期' }]} />
          </div>
          <button type="submit" disabled={jobHistoryLoading}>{jobHistoryLoading ? '查询中…' : '查询'}</button>
        </form>
        {jobHistoryError && <div className="result-banner error" role="alert">{jobHistoryError}{jobHistoryPagination && !jobHistoryError.includes('兼容数据') ? ' 已保留最近一次成功数据。' : ''} <button type="button" onClick={() => setJobHistoryReloadVersion((version) => version + 1)} disabled={jobHistoryLoading}>重试</button></div>}
        <section className="excel-jobs-workspace">
          <div className="excel-jobs-main">
            <Panel title="Excel 任务" icon={<ListChecks />} meta={jobHistoryLoading && !jobHistoryPagination ? '正在加载…' : `共 ${jobHistoryPagination?.total ?? 0} 条，可查询、查看和下载`}>
              <ExcelJobHistoryTable
                jobs={jobHistory}
                loading={jobHistoryLoading}
                downloadingJobID={downloadingJobID}
                selectedJobID={job?.id ?? null}
                onDownload={downloadJob}
                onView={(id) => void refreshJobByID(id)}
              />
              <MonitoringPaginationControls page={jobHistoryPagination?.page ?? jobHistoryPage} totalPages={jobHistoryPagination?.totalPages ?? 0} loading={jobHistoryLoading} onPrevious={() => setJobHistoryPage((page) => Math.max(1, page - 1))} onNext={() => setJobHistoryPage((page) => page + 1)} />
            </Panel>
            <Panel title="按任务 ID 定位" icon={<Download />} meta="直接查询历史任务并下载结果">
              <button type="button" onClick={() => openExcelDialog('query')}>打开任务定位</button>
            </Panel>
          </div>

          <aside className="excel-job-aside" aria-label="Excel 任务详情">
            {job ? (
              <div
                ref={jobExecutionRef}
                className="excel-job-focus-target"
                tabIndex={-1}
                role="region"
                aria-label={`Excel 任务 ${job.id} 执行详情`}
              >
                <Panel title={`任务 #${job.id}`} icon={<FileJson />} meta={job.source_file_name || '任务详情'}>
                  {autoRefreshText && <p className="excel-mode-note">{autoRefreshText}</p>}
                  <div className="excel-job-progress">
                    <span>处理进度 {job.processed_rows} / {job.total_rows}（{selectedJobProgress}%）</span>
                    <progress value={selectedJobProgress} max="100" aria-label={`Excel 任务 #${job.id} 处理进度`} />
                  </div>
                  <div className="excel-job-detail">
                    <Metric label="任务类型" value={excelJobOperationLabel(excelJobOperation(job))} />
                    <Metric label="匹配/更新" value={job.matched_rows || '-'} />
                    <Metric label="未匹配" value={job.unmatched_rows || '-'} />
                    <Metric label="结果过期" value={formatDate(job.expires_at)} />
                    <Metric label="开始时间" value={formatDate(job.started_at)} />
                    <Metric label="结束时间" value={formatDate(job.finished_at)} />
                  </div>
                  <div className="excel-detail-actions">
                    <button type="button" onClick={() => void refreshJobByID(job.id)} disabled={loading}>
                      <RefreshCcw aria-hidden="true" />
                      刷新状态
                    </button>
                    <button
                      type="button"
                      onClick={() => void downloadJob(job.id)}
                      disabled={loading || downloadingJobID === job.id || !canDownloadExcelJob(job)}
                    >
                      <Download aria-hidden="true" />
                      {downloadingJobID === job.id ? '下载中' : '下载结果'}
                    </button>
                    {!canDownloadExcelJob(job) && <span>{job.download_message || '只有匹配导出成功任务会生成可下载结果文件。'}</span>}
                  </div>
                  {job.status === 'failed' && <div className="login-error" role="alert">任务执行失败，请查看受控服务日志。</div>}
                  <ExcelJobLogList logs={jobLogs} />
                </Panel>
              </div>
            ) : <EmptyState text="选择一个 Excel 任务查看受控进度、日志和下载状态。" />}
          </aside>
        </section>
      </>}

      {section === 'schemes' && <>
        <section className="overview-grid">
          <Metric label="导出方案" value={exportSchemes.length} />
          <Metric label="当前步骤" value={exportSteps.length} />
          <Metric label="最大步骤" value="20" />
          <Metric label="筛选规则" value="可选" />
        </section>
        <section className="excel-config-workspace">
          <Panel title="匹配导出配置" icon={<Upload />} meta="步骤顺序与预览都基于真实导出配置">
            <div className="excel-config-summary">
              <strong>{selectedExportSchemeID ? '正在编辑已保存方案' : '新建匹配方案'}</strong>
              <span>当前包含 {exportSteps.length} 个步骤；可直接调整顺序、筛选和输出字段。</span>
            </div>
            {renderExportMatchForm()}
          </Panel>
          <aside className="excel-config-aside" aria-label="已保存导出方案">
            <Panel title="已保存导出方案" icon={<ListChecks />} meta={`${exportSchemes.length} 个方案`}><ExcelSchemeList schemes={exportSchemes} deletingSchemeID={deletingSchemeID} onDelete={setPendingSchemeDelete} onOpen={(id) => applyExportScheme(String(id))} /></Panel>
          </aside>
        </section>
      </>}

      {section === 'write' && <>
        <section className="overview-grid">
          <Metric label="导入方案" value={importSchemes.length} />
          <Metric label="默认模式" value="只预检" />
          <Metric label="写入保护" value="不覆盖" />
          <Metric label="清空保护" value="需确认" />
        </section>
        <section className="excel-config-workspace">
          <Panel title="数据库回写" icon={<Database />} meta="默认只预检；写入与退回均需二次确认">
            <div className="excel-mode-switch" role="group" aria-label="数据库回写模式"><button type="button" className={writeMode === 'import' ? 'active' : undefined} onClick={() => setWriteMode('import')}><Database aria-hidden="true" />匹配导入</button><button type="button" className={writeMode === 'clear' ? 'active danger' : 'danger'} onClick={() => setWriteMode('clear')}><RefreshCcw aria-hidden="true" />退回未匹配</button></div>
            <p className="excel-mode-note">{writeMode === 'import' ? '先预检，确认后只填充空字段。' : '先预检，确认后清空命中行的 matched_docno。'}</p>
            {writeMode === 'import' ? renderImportWriteForm() : renderClearWriteForm()}
            {latestWriteJob && <section className="excel-write-summary" aria-labelledby="excel-write-summary-title"><div><strong id="excel-write-summary-title">最近预检/写入任务</strong><span>以下数据来自服务端安全任务摘要；不覆盖与错误计数未返回。</span></div><div className="excel-job-detail"><Metric label="任务 ID" value={latestWriteJob.id} /><Metric label="状态" value={excelJobStatusLabel(latestWriteJob.status)} /><Metric label="总行数" value={latestWriteJob.total_rows} /><Metric label="已处理" value={latestWriteJob.processed_rows} /><Metric label="预计命中" value={latestWriteJob.matched_rows} /><Metric label="未匹配" value={latestWriteJob.unmatched_rows} /></div><div className="excel-detail-actions"><span>不覆盖/错误：服务端未返回独立计数。</span><button type="button" onClick={onNavigateToJobs}>查看任务详情</button></div></section>}
          </Panel>
          <aside className="excel-config-aside" aria-label="已保存导入方案"><Panel title="已保存导入方案" icon={<ListChecks />} meta={`${importSchemes.length} 个方案`}><ExcelSchemeList schemes={importSchemes} deletingSchemeID={deletingSchemeID} onDelete={setPendingSchemeDelete} onOpen={(id) => { setWriteMode('import'); applyImportScheme(String(id)) }} /></Panel></aside>
        </section>
      </>}

      {pendingSchemeSave && <Modal title="保存 Excel 匹配方案" focusKey="scheme-save" closeDisabled={loading || schemeSaveInFlightRef.current} onClose={() => { if (!loading && !schemeSaveInFlightRef.current) { setPendingSchemeSave(null); setSchemeSaveError('') } }}>{renderPendingSchemeSave()}</Modal>}

      {pendingWrite && <Modal title={pendingWrite.slot === 'import' ? '确认写入数据库' : '确认退回未匹配'} focusKey="confirm" closeDisabled={loading} onClose={() => { if (!loading) setPendingWrite(null) }}><div className="view-stack"><p>{pendingWrite.message}</p><div className="excel-form-actions"><button type="button" disabled={loading} onClick={() => setPendingWrite(null)}>返回修改</button><button className={pendingWrite.slot === 'clear' ? 'danger' : 'primary'} type="button" disabled={loading} onClick={() => void confirmPendingWrite()}>{loading ? '创建任务中…' : pendingWrite.slot === 'clear' ? '确认退回' : '确认写入'}</button></div></div></Modal>}

      {excelDialog === 'export' && (
        <Modal
          title={pendingSchemeSave ? '保存 Excel 匹配方案' : '匹配导出参数'}
          focusKey={pendingSchemeSave ? 'scheme-save' : 'form'}
          closeDisabled={loading || schemeSaveInFlightRef.current}
          onClose={() => { if (!loading && !schemeSaveInFlightRef.current) closeExcelDialog() }}
          footer={pendingSchemeSave ? undefined : (
            <div className="excel-modal-footer-content">
              {uploadProgress && <p className="excel-mode-note modal-footer-status" role="status" aria-live="polite">{uploadProgress}</p>}
              <div className="excel-form-actions">
                <button
                  type="button"
                  form="excel-export-job-form"
                  onClick={(event) => {
                    const form = event.currentTarget.form
                    if (form?.reportValidity()) void previewExportJob(form)
                  }}
                  disabled={loading}
                >
                  <FileJson aria-hidden="true" />
                  预览匹配
                </button>
                <button className="primary" type="submit" form="excel-export-job-form" disabled={loading}>
                  <Upload aria-hidden="true" />
                  创建导出任务
                </button>
              </div>
            </div>
          )}
        >
          {renderPendingSchemeSave()}
          <form id="excel-export-job-form" className="excel-upload-form" onSubmit={createExportJob} key={exportFormKey} hidden={pendingSchemeSave !== null}>
            <label>
              已保存方案
              <select value={selectedExportSchemeID} onChange={(event) => applyExportScheme(event.currentTarget.value)}>
                <option value="">选择方案</option>
                {exportSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}
              </select>
            </label>
            <button
              type="button"
              disabled={!selectedExportSchemeID || loading}
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form?.reportValidity()) beginSchemeSave(form, 'export_match', 'current')
              }}
            >
              保存到当前方案
            </button>
            <button
              type="button"
              disabled={loading}
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form?.reportValidity()) beginSchemeSave(form, 'export_match', 'new')
              }}
            >
              另存为新方案
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
            <div className="excel-step-editor">
              <div className="excel-step-editor-title">
                <div>
                  <strong>匹配步骤</strong>
                  <span>每一步都从完整 Excel 行集独立筛选；前一步跳过的行仍可进入后续步骤。</span>
                </div>
                <button type="button" onClick={addExportStep} disabled={exportSteps.length >= 20}>添加步骤</button>
              </div>
              {excelModelsLoading && <p className="excel-mode-note">正在加载模型与字段目录…</p>}
              {excelModelsError && (
                <div className="excel-catalog-error" role="alert">
                  <span>模型与字段目录加载失败：{excelModelsError}</span>
                  <button type="button" onClick={() => void loadExcelModels()}>重试加载</button>
                </div>
              )}
              {!excelModelsLoading && !excelModelsError && excelModels.length === 0 && (
                <p className="excel-mode-note">当前数据库没有返回可选择的模型；历史配置仍可查看，但新步骤需要先确认数据库连接和模型表。</p>
              )}
              {exportSteps.map((step, index) => (
                <article className="excel-step-card" key={`${exportFormKey}-${index}`}>
                  <div className="excel-step-heading">
                    <strong>步骤 {index + 1}</strong>
                    <div className="table-actions">
                      <button type="button" onClick={() => moveExportStep(index, -1)} disabled={index === 0}>上移</button>
                      <button type="button" onClick={() => moveExportStep(index, 1)} disabled={index === exportSteps.length - 1}>下移</button>
                      <button type="button" onClick={() => removeExportStep(index)} disabled={exportSteps.length === 1}>删除</button>
                    </div>
                  </div>
                  <div className="excel-step-fields">
                    <Field label="步骤名称" name={`step_name_${index}`} value={step.name} onChange={(value) => updateExportStep(index, 'name', value)} required />
                    <label>
                      匹配模式
                      <select
                        name={`step_mode_${index}`}
                        value={step.matchMode}
                        onChange={(event) => updateExportStep(index, 'matchMode', event.currentTarget.value)}
                      >
                        <option value="field">普通字段匹配</option>
                        <option value="order_item_sku">订单商品 SKU 匹配</option>
                      </select>
                    </label>
                    <ExcelModelSelector
                      name={`step_table_${index}`}
                      models={excelModels}
                      value={step.tableName}
                      onChange={(value) => selectExportStepModel(index, value)}
                    />
                    <Field label={step.matchMode === 'order_item_sku' ? '订单号 Excel 列' : 'Excel 输入列'} name={`step_excel_${index}`} value={step.matchExcelColumn} onChange={(value) => updateExportStep(index, 'matchExcelColumn', value)} required />
                    <ExcelModelFieldSelector
                      label={step.matchMode === 'order_item_sku' ? '数据库订单号字段' : '匹配模型字段'}
                      name={`step_match_${index}`}
                      models={excelModels}
                      tableName={step.tableName}
                      value={step.dbMatchField}
                      onChange={(value) => updateExportStep(index, 'dbMatchField', value)}
                    />
                    <ExcelModelFieldSelector
                      label={step.matchMode === 'order_item_sku' ? '数据库购物明细字段' : '取值模型字段'}
                      name={`step_value_${index}`}
                      models={excelModels}
                      tableName={step.tableName}
                      value={step.dbValueField}
                      onChange={(value) => updateExportStep(index, 'dbValueField', value)}
                    />
                    <Field label={step.matchMode === 'order_item_sku' ? 'SKU 输出列' : '追加输出列'} name={`step_output_${index}`} value={step.outputColumnName} onChange={(value) => updateExportStep(index, 'outputColumnName', value)} required />
                    {step.matchMode === 'order_item_sku' && <>
                      <Field label="规格编码 Excel 列" name={`step_spec_${index}`} value={step.specExcelColumn} onChange={(value) => updateExportStep(index, 'specExcelColumn', value)} required />
                      <Field label="销售金额 Excel 列（对应 totAmtActual）" name={`step_price_${index}`} value={step.priceExcelColumn} onChange={(value) => updateExportStep(index, 'priceExcelColumn', value)} required />
                      <Field label="销售数量 Excel 列" name={`step_qty_${index}`} value={step.qtyExcelColumn} onChange={(value) => updateExportStep(index, 'qtyExcelColumn', value)} required />
                    </>}
                  </div>
                  {step.matchMode === 'order_item_sku' && (
                    <p className="excel-mode-note">
                      按数据库购物明细字段（例如 items_json）匹配并输出完整 no：优先用 Excel 规格编码匹配 mProductName 前缀，并同时校验 totAmtActual（销售金额）和 qty，mProductName 长度不限；若规格编码在订单明细中没有候选，则按销售金额和数量兜底匹配。Excel 规格编码为 15 位或 16 位时直接跳过。每条购物明细按其在 JSON 中的出现次数使用一次；相同明细重复出现时，可按次数重复输出同一 no。
                    </p>
                  )}
                  <div className="excel-step-filter-editor">
                    <div className="excel-step-filter-heading">
                      <div>
                        <strong>本步骤筛选</strong>
                        <span>{step.matchMode === 'order_item_sku'
                          ? '只决定本步骤处理哪些行；多条条件需要同时满足。订单商品 SKU 模式仅可引用原始 Excel 列。'
                          : '只决定本步骤处理哪些行；多条条件需要同时满足。可引用原始列或前序步骤追加列。'}</span>
                      </div>
                      <button type="button" onClick={() => addExportStepFilter(index)}>添加条件</button>
                    </div>
                    {step.filters.length === 0 && <p className="excel-step-filter-empty">未设置条件，本步骤处理全部 Excel 行。</p>}
                    {step.filters.map((filter, filterIndex) => {
                      const valueNotRequired = filter.op === 'empty' || filter.op === 'not_empty'
                      return (
                        <div className="excel-step-filter-row" key={`${exportFormKey}-${index}-${filterIndex}`}>
                          <Field
                            label={`条件 ${filterIndex + 1} · Excel 列`}
                            name={`step_filter_column_${index}_${filterIndex}`}
                            value={filter.column}
                            onChange={(value) => updateExportStepFilter(index, filterIndex, 'column', value)}
                            required
                          />
                          <label>
                            运算符
                            <select
                              name={`step_filter_op_${index}_${filterIndex}`}
                              value={filter.op}
                              onChange={(event) => updateExportStepFilter(index, filterIndex, 'op', event.currentTarget.value)}
                            >
                              {excelMatchFilterOperatorOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                            </select>
                          </label>
                          {valueNotRequired
                            ? <label>筛选值<input value="此运算无需填写" readOnly disabled /></label>
                            : <Field
                                label="筛选值"
                                name={`step_filter_value_${index}_${filterIndex}`}
                                value={filter.value}
                                onChange={(value) => updateExportStepFilter(index, filterIndex, 'value', value)}
                                required
                              />}
                          <button type="button" onClick={() => removeExportStepFilter(index, filterIndex)} aria-label={`删除步骤 ${index + 1} 的条件 ${filterIndex + 1}`}>删除条件</button>
                        </div>
                      )
                    })}
                  </div>
                </article>
              ))}
            </div>
            <label>
              导出列内容格式
              <textarea
                name="exportColumnFormats"
                defaultValue={exportDefaults.exportColumnFormats}
                rows={4}
                placeholder={'每行一个：列名=格式\n例如：金额=number\n下单时间=date'}
              />
              <small>支持格式：text、number、integer、bool、date。列名可填原 Excel 表头或追加列名。</small>
            </label>
            <Field label="批量查询大小" name="batchSize" defaultValue={exportDefaults.batchSize} />
            {previewResult && <ExcelMatchPreviewPanel preview={previewResult} />}
          </form>
        </Modal>
      )}

      {excelDialog === 'import' && (
        <Modal title={pendingSchemeSave ? '保存 Excel 匹配方案' : pendingWrite?.slot === 'import' ? '确认写入数据库' : '匹配导入参数'} focusKey={pendingSchemeSave ? 'scheme-save' : pendingWrite?.slot === 'import' ? 'confirm' : 'form'} closeDisabled={loading || schemeSaveInFlightRef.current} onClose={() => { if (!loading && !schemeSaveInFlightRef.current) { setPendingWrite(null); closeExcelDialog() } }}>
          {pendingWrite?.slot === 'import' && <div className="view-stack"><p>{pendingWrite.message}</p><div className="excel-form-actions"><button type="button" disabled={loading} onClick={() => setPendingWrite(null)}>返回修改</button><button className="primary" type="button" disabled={loading} onClick={() => void confirmPendingWrite()}>{loading ? '创建任务中…' : '确认写入'}</button></div></div>}
          {renderPendingSchemeSave()}
          <form className="excel-upload-form" onSubmit={createImportJob} key={importFormKey} hidden={pendingWrite?.slot === 'import' || pendingSchemeSave !== null}>
            <label>
              已保存方案
              <select value={selectedImportSchemeID} onChange={(event) => applyImportScheme(event.currentTarget.value)}>
                <option value="">选择方案</option>
                {importSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}
              </select>
            </label>
            <button
              type="button"
              disabled={!selectedImportSchemeID || loading}
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form?.reportValidity()) beginSchemeSave(form, 'import_update', 'current')
              }}
            >
              保存到当前方案
            </button>
            <button
              type="button"
              disabled={loading}
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form?.reportValidity()) beginSchemeSave(form, 'import_update', 'new')
              }}
            >
              另存为新方案
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
                <option value="completed_at">订单完成时间 completed_at</option>
              </select>
            </label>
            <Field label="Excel 写入值列名" name="writeExcelColumn" defaultValue={importDefaults.writeExcelColumn} />
            <Field label="批量更新大小" name="batchSize" defaultValue={importDefaults.batchSize} />
            <label className="checkbox-label">
              <input name="confirmWrite" type="checkbox" />
              确认写入数据库
            </label>
            <p className="excel-mode-note">
              不勾选时只预检匹配数量，不写库；匹配单号只填充空值；订单完成时间要求 yyyy-mm-dd hh:mm:ss 格式且只填充为空的 completed_at。
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
        <Modal title={pendingWrite?.slot === 'clear' ? '确认退回未匹配' : '退回未匹配参数'} focusKey={pendingWrite?.slot === 'clear' ? 'confirm' : 'form'} closeDisabled={loading} onClose={() => { if (!loading) { setPendingWrite(null); closeExcelDialog() } }}>
          {pendingWrite?.slot === 'clear' && <div className="view-stack"><p>{pendingWrite.message}</p><div className="excel-form-actions"><button type="button" disabled={loading} onClick={() => setPendingWrite(null)}>返回修改</button><button className="danger" type="button" disabled={loading} onClick={() => void confirmPendingWrite()}>{loading ? '创建任务中…' : '确认退回'}</button></div></div>}
          <form className="excel-upload-form" onSubmit={createClearMatchedJob} hidden={pendingWrite?.slot === 'clear'}>
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
        <Modal title="任务查询与下载" onClose={closeExcelDialog}>
          <div className="excel-job-actions">
            <label>
              任务 ID
              <input value={jobID} onChange={(event) => setJobID(event.target.value)} />
            </label>
            <button type="button" onClick={refreshJob} disabled={loading}>
              <RefreshCcw aria-hidden="true" />
              查询状态
            </button>
            <button type="button" onClick={() => void downloadJob()} disabled={loading || !job || !canDownloadExcelJob(job)}>
              <Download aria-hidden="true" />
              {downloadingJobID === Number(jobID || job?.id) ? '下载中' : '下载结果'}
            </button>
          </div>
        </Modal>
      )}

      {pendingSchemeDelete && (
        <Modal title="删除 Excel 匹配方案" onClose={() => { if (deletingSchemeID === null) setPendingSchemeDelete(null) }}>
          <p>确认删除方案“{pendingSchemeDelete.name}”？删除后不能恢复，已创建的任务不会受影响。</p>
          <div className="excel-form-actions">
            <button type="button" onClick={() => setPendingSchemeDelete(null)} disabled={deletingSchemeID !== null}>取消</button>
            <button type="button" className="danger" onClick={() => void deleteScheme(pendingSchemeDelete)} disabled={deletingSchemeID !== null}>
              {deletingSchemeID === pendingSchemeDelete.id ? '删除中…' : '确认删除'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}

function ExcelSchemeList({ schemes, deletingSchemeID, onDelete, onOpen }: { schemes: ExcelMatchScheme[]; deletingSchemeID: number | null; onDelete: (scheme: ExcelMatchScheme) => void; onOpen: (id: number) => void }) {
  if (schemes.length === 0) return <EmptyState text="暂无已保存方案。" />
  return (
    <div className="data-table-wrap">
      <table className="data-table excel-scheme-table">
        <thead>
          <tr>
            <th>方案名称</th>
            <th>操作类型</th>
            <th>匹配步骤</th>
            <th>更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {schemes.map((scheme) => (
            <tr key={scheme.id}>
              <td>{scheme.name}</td>
              <td>{excelJobOperationLabel(scheme.operation)}</td>
              <td>{scheme.operation === 'export_match' ? (scheme.config.steps?.length || 1) : '-'}</td>
              <td>{formatUnixTime(scheme.updated_at)}</td>
              <td>
                <div className="table-actions">
                  <button type="button" onClick={() => onOpen(scheme.id)} disabled={deletingSchemeID !== null}>打开配置</button>
                  <button type="button" className="danger" onClick={() => onDelete(scheme)} disabled={deletingSchemeID !== null}>
                    {deletingSchemeID === scheme.id ? '删除中…' : '删除'}
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function ExcelJobHistoryTable({
  jobs,
  loading,
  downloadingJobID,
  selectedJobID,
  onView,
  onDownload,
}: {
  jobs: ExcelMatchJob[]
  loading: boolean
  downloadingJobID: number | null
  selectedJobID: number | null
  onView: (id: number) => void
  onDownload: (id: number) => void
}) {
  if (jobs.length === 0) return <EmptyState text="暂无 Excel 任务历史。" />
  return (
    <div className="data-table-wrap">
      <table className="data-table excel-history-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>文件</th>
            <th>类型</th>
            <th>状态</th>
            <th>处理行</th>
            <th>匹配/未匹配</th>
            <th>创建时间</th>
            <th>过期时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {jobs.map((item) => (
            <tr className={item.id === selectedJobID ? 'excel-history-row--selected' : undefined} key={item.id}>
              <td>{item.id}</td>
              <td>{item.source_file_name || '-'}</td>
              <td>{excelJobOperationLabel(excelJobOperation(item))}</td>
              <td>{excelJobStatusLabel(item.status)}</td>
              <td>{item.processed_rows}/{item.total_rows}</td>
              <td>{item.matched_rows}/{item.unmatched_rows}</td>
              <td>{formatUnixTime(item.created_at)}</td>
              <td>{formatDate(item.expires_at)}</td>
              <td>
                <div className="table-actions">
                  <button type="button" onClick={() => onView(item.id)} disabled={loading}>
                    查看
                  </button>
                  <button
                    type="button"
                    onClick={() => onDownload(item.id)}
                    disabled={loading || downloadingJobID === item.id || !canDownloadExcelJob(item)}
                    title={item.download_message || undefined}
                  >
                    {downloadingJobID === item.id ? '下载中' : '下载'}
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Modal({ title, onClose, children, footer, closeDisabled = false, focusKey }: { title: string; onClose: () => void; children: ReactNode; footer?: ReactNode; closeDisabled?: boolean; focusKey?: string }) {
  const panelRef = useRef<HTMLElement>(null)
  const returnFocusRef = useRef<HTMLElement | null>(document.activeElement instanceof HTMLElement ? document.activeElement : null)
  const titleID = useMemo(() => `modal-title-${Math.random().toString(36).slice(2)}`, [])
  const onCloseRef = useRef(onClose)
  const closeDisabledRef = useRef(closeDisabled)
  onCloseRef.current = onClose
  closeDisabledRef.current = closeDisabled

  useEffect(() => {
    const previousOverflow = document.body.style.overflow
    const returnFocus = returnFocusRef.current
    const focusableSelector = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        if (!closeDisabledRef.current) onCloseRef.current()
        return
      }
      if (event.key !== 'Tab') return
      const focusable = Array.from(panelRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? []).filter((element) => !element.closest('[hidden], [aria-hidden="true"]'))
      if (focusable.length === 0) {
        event.preventDefault()
        panelRef.current?.focus()
        return
      }
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', handleKeyDown)
    return () => {
      document.body.style.overflow = previousOverflow
      window.removeEventListener('keydown', handleKeyDown)
      returnFocus?.focus()
    }
  }, [])

  useEffect(() => {
    const focusableSelector = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    const initialFocus = panelRef.current?.querySelector<HTMLElement>('[data-autofocus]')
      ?? Array.from(panelRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? []).find((element) => !element.closest('[hidden], [aria-hidden="true"]'))
    initialFocus?.focus()
  }, [focusKey])

  return (
    <div className="modal-backdrop" role="presentation">
      <section ref={panelRef} className="modal-panel" role="dialog" aria-modal="true" aria-labelledby={titleID} tabIndex={-1}>
        <div className="modal-title">
          <h3 id={titleID}>{title}</h3>
          <button type="button" onClick={onClose} disabled={closeDisabled} aria-label={`关闭${title}`}>关闭</button>
        </div>
        <div className="modal-body">{children}</div>
        {footer && <div className="modal-footer">{footer}</div>}
      </section>
    </div>
  )
}

function ExcelMatchPreviewPanel({ preview }: { preview: ExcelMatchPreviewResult }) {
  return (
    <div className="excel-preview-panel">
      <div className="overview-grid compact">
        <Metric label="扫描行" value={excelPreviewStat(preview.stats, 'TotalRows')} />
        <Metric label="参与步骤行" value={excelPreviewStat(preview.stats, 'FilteredRows')} />
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
              <th>步骤结果</th>
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
                <td>
                  {sample.stepResults?.length ? (
                    <div className="excel-preview-steps">
                      {sample.stepResults.map((step) => (
                        <span key={`${step.stepIndex}-${step.stepName}`}>
                          {step.stepIndex}. {step.stepName || '未命名'}：{step.matchedValue || excelPreviewStatusLabel(step.status)}
                        </span>
                      ))}
                    </div>
                  ) : '-'}
                </td>
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
            <strong>{log.message || '任务日志已记录'}</strong>
            <span>{excelLogLevelLabel(log.level)} / {formatUnixTime(log.created_at)}</span>
          </div>
        </article>
      ))}
    </div>
  )
}

function CoreMethodList({ methods, onToggle }: { methods: CoreMethod[]; onToggle?: (target: ToggleTarget, enabled: boolean) => void }) {
  return (
    <div className="method-list design-core-method-list">
      {methods.map((method) => (
        <article className="method-row" key={method.key}>
          <strong>{method.title}</strong>
          <span>{method.category}</span>
          <small>{method.status}</small>
          <div className="design-core-actions">
            {onToggle && method.refs.length > 0
              ? <details><summary className={method.enabled ? 'design-core-status enabled' : 'design-core-status'}>{method.enabled ? '已开启' : '已关闭'}</summary><div>{method.refs.map((target) => <ToggleButton enabled={method.enabled} key={`${target.type}-${target.id}`} target={target} onToggle={onToggle} />)}</div></details>
              : <span className={method.enabled ? 'design-core-status enabled' : 'design-core-status'}>{method.enabled ? '已开启' : '已关闭'}</span>}
          </div>
          <span className="sr-only">{method.description}</span>
        </article>
      ))}
    </div>
  )
}

function MethodCatalogTable({ methods, onToggle }: { methods: MethodDisplay[]; onToggle: (target: ToggleTarget, enabled: boolean) => void }) {
  if (methods.length === 0) return <EmptyState text="暂无匹配的方法。" />
  return <div className="data-table-wrap" role="region" aria-label="方法目录列表" tabIndex={0}><table className="data-table design-method-table"><thead><tr><th scope="col">方法名称</th><th scope="col">编码</th><th scope="col">分类</th><th scope="col">负责人</th><th scope="col">类型</th><th scope="col">状态</th></tr></thead><tbody>{methods.map((method) => <tr key={method.key}><td><strong>{method.name}</strong><small>{method.description}</small></td><td><code>{method.code}</code></td><td>{method.category}</td><td>{method.owner}</td><td>{method.kind === 'builtin' ? '内置' : '配置'}</td><td>{method.toggle ? <button className={method.enabled ? 'design-method-status enabled' : 'design-method-status'} type="button" onClick={() => onToggle(method.toggle!, !method.enabled)}>{method.enabled ? '启用' : '停用'}</button> : <StatusPill label={method.enabled ? '启用' : '停用'} />}</td></tr>)}</tbody></table></div>
}

function ToggleButton({ enabled, target, onToggle }: { enabled: boolean; target: ToggleTarget; onToggle: (target: ToggleTarget, enabled: boolean) => void }) {
  return (
    <button type="button" onClick={() => onToggle(target, !enabled)}>
      {enabled ? '停用' : '开启'}
    </button>
  )
}

function RunTable({ runs, onLoadSteps, onSelectRun }: { runs: PipelineRun[]; onLoadSteps: (runId: number) => void; onSelectRun?: (run: PipelineRun) => void }) {
  if (runs.length === 0) return <EmptyState text="暂无运行记录。" />
  return (
    <div className="data-table-wrap">
      <table className="data-table">
        <thead><tr><th>ID / Trace ID</th><th>运行类型</th><th>触发方式</th><th>状态</th><th>成功 / 失败 / 总数</th><th>耗时</th><th>开始时间</th><th>明细</th></tr></thead>
        <tbody>
          {runs.slice(0, 20).map((run) => (
            <tr key={run.id}>
              <td><strong>#{run.id}</strong><small>{run.trace_id || '-'}</small></td>
              <td>{run.run_type}</td>
              <td>{run.trigger_type || '-'}</td>
              <td>{run.status}</td>
              <td>{run.success_count} / {run.failed_count} / {run.total_count}</td>
              <td>{runDurationLabel(run.started_at, run.finished_at)}</td>
              <td>{formatDate(run.started_at)}</td>
              <td>
                <div className="table-actions">
                  {onSelectRun && <button type="button" onClick={() => onSelectRun(run)}>详情</button>}
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

function DeliveryLogList({ logs, onRetryLog, retryingLogID }: { logs: DeliveryLog[]; onRetryLog?: (log: DeliveryLog) => void | Promise<void>; retryingLogID?: number | null }) {
  const [selectedLogID, setSelectedLogID] = useState<number | null>(null)

  useEffect(() => {
    setSelectedLogID((current) => logs.some((log) => log.id === current) ? current : logs[0]?.id ?? null)
  }, [logs])

  if (logs.length === 0) return <EmptyState text="暂无推送日志。" />

  const selectedLog = logs.find((log) => log.id === selectedLogID) ?? logs[0]
  return (
    <div className="delivery-log-layout">
      <div className="delivery-log-main">
        <div className="data-table-wrap" role="region" aria-label="推送日志列表" tabIndex={0}>
          <table className="data-table delivery-log-table">
            <thead>
              <tr>
                <th scope="col">状态</th>
                <th scope="col">业务键</th>
                <th scope="col">推送目标</th>
                <th scope="col">来源</th>
                <th scope="col">HTTP</th>
                <th scope="col">推送时间</th>
                <th scope="col">重试</th>
                <th scope="col">操作</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log) => (
                <tr className={log.id === selectedLog.id ? 'delivery-log-row--selected' : undefined} key={log.id}>
                  <td><StatusPill label={log.success ? '成功' : '失败'} /></td>
                  <td>{log.business_key || '-'}</td>
                  <td>{log.destination_name || log.destination_code || `目标 #${log.destination_id}`}</td>
                  <td>{log.source_code || '-'}</td>
                  <td>{log.http_status || '-'}</td>
                  <td>{formatDate(log.sent_at)}</td>
                  <td>{log.retry_count}</td>
                  <td>
                    <div className="table-actions">
                      <button type="button" aria-pressed={log.id === selectedLog.id} onClick={() => setSelectedLogID(log.id)}>查看</button>
                      {!log.success && onRetryLog && <button type="button" disabled={retryingLogID !== null} onClick={() => void onRetryLog(log)}>{retryingLogID === log.id ? '重试中…' : '重试'}</button>}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      <aside className="delivery-log-detail" aria-live="polite" aria-label="已选推送日志详情">
        <div className="delivery-log-detail-heading">
          <div>
            <span>已选日志</span>
            <strong>#{selectedLog.id}</strong>
          </div>
          <StatusPill label={selectedLog.success ? '成功' : '失败'} />
        </div>
        <dl className="delivery-log-detail-fields">
          <div><dt>业务键</dt><dd>{selectedLog.business_key || '-'}</dd></div>
          <div><dt>推送目标</dt><dd>{selectedLog.destination_name || selectedLog.destination_code || `目标 #${selectedLog.destination_id}`}</dd></div>
          <div><dt>来源</dt><dd>{selectedLog.source_code || '-'}</dd></div>
          <div><dt>Trace ID</dt><dd>{selectedLog.trace_id || '-'}</dd></div>
          <div><dt>HTTP 状态</dt><dd>{selectedLog.http_status || '-'}</dd></div>
          <div><dt>推送时间</dt><dd>{formatDate(selectedLog.sent_at)}</dd></div>
          <div><dt>重试次数</dt><dd>{selectedLog.retry_count}</dd></div>
        </dl>
        {!selectedLog.success && <p className="delivery-log-protected-note">该记录存在交付异常。请求与响应内容受保护，不在管理端展示。</p>}
        {!selectedLog.success && onRetryLog && <div className="delivery-log-detail-actions"><button type="button" disabled={retryingLogID !== null} onClick={() => void onRetryLog(selectedLog)}>{retryingLogID === selectedLog.id ? '重试中…' : '重试推送'}</button></div>}
      </aside>
    </div>
  )
}

function orderPushTargetConfig(config: OrderPushSkipConfig, targetCode: string): OrderPushSkipTargetConfig {
  const target = config.targets.find((item) => item.target_code.toLowerCase() === targetCode.toLowerCase())
  return target ?? { target_code: targetCode, target_name: '', cycle: 0, skip: 0 }
}

function normalizeOrderPushSkipConfig(config: OrderPushSkipConfig | null): OrderPushSkipConfig {
  if (!config || !Array.isArray(config.targets)) return defaultOrderPushSkipConfig
  return {
    targets: config.targets.map((target) => ({
      target_code: target.target_code || '',
      target_name: target.target_name || '',
      cycle: Number(target.cycle || 0),
      skip: Number(target.skip || 0),
    })),
  }
}

function apiURL(path: string) {
  return buildApiURL(path, defaultApiBaseURL)
}

function compactText(value: string) {
  const text = (value || '').replace(/\s+/g, ' ').trim()
  if (text.length <= 120) return text
  return `${text.slice(0, 120)}...`
}

function MonitoringPaginationControls({ page, totalPages, loading, onPrevious, onNext }: { page: number; totalPages: number; loading: boolean; onPrevious: () => void; onNext: () => void }) {
  return <div className="record-actions raw-record-pagination" role="status" aria-live="polite">
    <span>第 {page} / {Math.max(totalPages, 1)} 页</span>
    <button type="button" onClick={onPrevious} disabled={loading || page <= 1}>上一页</button>
    <button type="button" onClick={onNext} disabled={loading || totalPages === 0 || page >= totalPages}>下一页</button>
  </div>
}

function publicText(value: unknown, maximumLength: number) {
  if (typeof value !== 'string') return ''
  const text = value.trim()
  return text.length <= maximumLength ? text : ''
}

function publicDateTime(value: unknown) {
  return value === null ? null : publicText(value, 32) || null
}

function publicNonNegativeInteger(value: unknown) {
  const number = Number(value)
  return Number.isSafeInteger(number) && number >= 0 ? number : -1
}

function parsePipelineRun(value: unknown): PipelineRun | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const run = value as Record<string, unknown>
  const id = publicNonNegativeInteger(run.id)
  const sourceID = publicNonNegativeInteger(run.source_id)
  const destinationID = publicNonNegativeInteger(run.destination_id)
  const totalCount = publicNonNegativeInteger(run.total_count)
  const successCount = publicNonNegativeInteger(run.success_count)
  const failedCount = publicNonNegativeInteger(run.failed_count)
  const status = publicText(run.status, 24)
  const runType = publicText(run.run_type, 32)
  if (id <= 0 || sourceID < 0 || destinationID < 0 || totalCount < 0 || successCount < 0 || failedCount < 0
    || !['running', 'success', 'failed', 'partial_success'].includes(status)
    || !['fetch', 'ingest', 'transform', 'delivery'].includes(runType)) return null
  return {
    id,
    trace_id: publicText(run.trace_id, 64),
    run_type: runType,
    trigger_type: publicText(run.trigger_type, 50),
    status,
    total_count: totalCount,
    success_count: successCount,
    failed_count: failedCount,
    source_id: sourceID,
    destination_id: destinationID,
    started_at: publicDateTime(run.started_at),
    finished_at: publicDateTime(run.finished_at),
  }
}

function parseStepRun(value: unknown): StepRun | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const step = value as Record<string, unknown>
  const id = publicNonNegativeInteger(step.id)
  const runID = publicNonNegativeInteger(step.run_id)
  const pipelineID = publicNonNegativeInteger(step.pipeline_id)
  const stepID = publicNonNegativeInteger(step.step_id)
  const status = publicText(step.status, 24)
  if (id <= 0 || runID <= 0 || pipelineID < 0 || stepID < 0 || !['running', 'success', 'failed', 'skipped'].includes(status)) return null
  return {
    id,
    run_id: runID,
    pipeline_id: pipelineID,
    step_id: stepID,
    step_code: publicText(step.step_code, 100),
    method_type: publicText(step.method_type, 50),
    status,
    input_json: publicText(step.input_json, 4096),
    output_json: publicText(step.output_json, 4096),
    generated_config_json: publicText(step.generated_config_json, 4096),
    error_message: publicText(step.error_message, 240),
    started_at: publicDateTime(step.started_at),
    finished_at: publicDateTime(step.finished_at),
  }
}

function pipelineRunStatusLabel(status: string) {
  const labels: Record<string, string> = {
    running: '运行中',
    success: '已完成',
    failed: '失败',
    partial_success: '部分成功',
  }
  return labels[status] ?? '未知'
}

function stepRunStatusLabel(status: string) {
  const labels: Record<string, string> = {
    running: '运行中',
    success: '已完成',
    failed: '失败',
    skipped: '已跳过',
  }
  return labels[status] ?? '未知'
}

function parseDeliveryLog(value: unknown): DeliveryLog | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const log = value as Record<string, unknown>
  const id = publicNonNegativeInteger(log.id)
  const runID = publicNonNegativeInteger(log.run_id)
  const destinationID = publicNonNegativeInteger(log.destination_id)
  const cleanRecordID = publicNonNegativeInteger(log.clean_record_id)
  const retryCount = publicNonNegativeInteger(log.retry_count)
  const httpStatus = publicNonNegativeInteger(log.http_status)
  if (id <= 0 || runID < 0 || destinationID < 0 || cleanRecordID < 0 || retryCount < 0 || httpStatus < 0 || typeof log.success !== 'boolean') return null
  return {
    id,
    trace_id: publicText(log.trace_id, 64),
    run_id: runID,
    source_code: publicText(log.source_code, 100),
    destination_code: publicText(log.destination_code, 100),
    destination_name: publicText(log.destination_name, 100),
    destination_id: destinationID,
    clean_record_id: cleanRecordID,
    business_key: publicText(log.business_key, 255),
    response_summary: publicText(log.response_summary, 240),
    http_status: httpStatus,
    success: log.success,
    error_message: publicText(log.error_message, 240),
    retry_count: retryCount,
    sent_at: publicDateTime(log.sent_at),
  }
}

function parseWarehouseRawRecord(value: unknown): WarehouseRawRecord | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const record = value as Record<string, unknown>
  const id = Number(record.id)
  const sourceID = Number(record.source_id)
  const createdAt = Number(record.created_at)
  const status = publicText(record.status, 16)
  if (!Number.isInteger(id) || id <= 0 || !Number.isInteger(sourceID) || sourceID < 0
    || !Number.isInteger(createdAt) || createdAt < 0
    || !['received', 'queued', 'cleaning', 'cleaned', 'failed'].includes(status)) return null
  return {
    id,
    sourceID,
    sourceCode: publicText(record.source_code, 100),
    status: status as WarehouseRawRecord['status'],
    traceID: publicText(record.trace_id, 64),
    receivedAt: publicText(record.received_at, 32),
    createdAt,
  }
}

function parseRetransformResult(payload: unknown): { traceID: string; cleanRecordID: number } | null {
  if (!payload || typeof payload !== 'object') return null
  const data = (payload as { data?: unknown }).data
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  const result = (data as { result?: unknown }).result
  if (!result || typeof result !== 'object' || Array.isArray(result)) return null
  const cleanRecordID = Number((result as Record<string, unknown>).clean_record_id)
  if (!Number.isInteger(cleanRecordID) || cleanRecordID <= 0) return null
  return { traceID: publicText((result as Record<string, unknown>).trace_id, 64), cleanRecordID }
}

function rawRecordStatusLabel(status: WarehouseRawRecord['status']) {
  return ({ received: '已接收', queued: '排队中', cleaning: '处理中', cleaned: '已清洗', failed: '失败' } as const)[status]
}

function RawDataList({ origin, records, onRequestSourceFetch }: { origin: RawRecordOrigin; records: RawData[]; onRequestSourceFetch: (sourceID: number) => void }) {
  const [selectedID, setSelectedID] = useState<number | null>(null)
  if (records.length === 0) return <EmptyState text="暂无原始数据。" />
  const selected = records.find((record) => record.id === selectedID) ?? records[0]
  return (
    <div className="raw-record-master-detail">
      <div className="data-table-wrap raw-record-table-wrap">
        <table className="data-table raw-record-table">
          <thead><tr><th scope="col">ID / 外部编号</th><th scope="col">数据类型</th><th scope="col">来源</th><th scope="col">{origin === 'pull' ? '拉取时间' : '接收时间'}</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
          <tbody>
            {records.map((record) => {
              const isSelected = selected.id === record.id
              return <tr className={isSelected ? 'is-selected' : ''} key={record.id}>
                <td><button className="table-row-select" type="button" aria-pressed={isSelected} onClick={() => setSelectedID(record.id)}>#{record.id}</button><small>{record.external_id || '-'}</small></td>
                <td>{record.data_type || 'raw'}</td>
                <td>{record.source || `#${record.data_source_id || '-'}`}</td>
                <td>{formatUnixTime(record.created_at)}</td>
                <td><StatusPill label={record.status || '未知'} /></td>
                <td className="table-actions"><button type="button" onClick={() => setSelectedID(record.id)}>查看</button>{origin === 'pull' && record.data_source_id > 0 && <button type="button" onClick={() => onRequestSourceFetch(record.data_source_id)}>重新拉取</button>}</td>
              </tr>
            })}
          </tbody>
        </table>
      </div>
      <section className="raw-record-detail" aria-live="polite" aria-label="原始记录详情">
        <div className="raw-record-detail-title"><div><span>{origin === 'pull' ? '拉取详情' : '原始记录'} #{selected.id}</span><strong>{selected.external_id || selected.data_type || 'raw'}</strong></div><StatusPill label={selected.status || '未知'} /></div>
        <dl className="task-definition-grid raw-record-fields"><div><dt>来源</dt><dd>{selected.source || `数据源 #${selected.data_source_id || '-'}`}</dd></div><div><dt>接入方式</dt><dd>{rawDataOrigin(selected)}</dd></div><div><dt>记录时间</dt><dd>{formatUnixTime(selected.created_at)}</dd></div><div><dt>数据类型</dt><dd>{selected.data_type || '-'}</dd></div></dl>
        <h3>脱敏原始内容与元数据</h3>
        <ReadonlyJSON value={redactMonitoringJSON({ raw_content: selected.raw_content ?? selected.rawContent ?? null, metadata: selected.metadata ?? null })} />
      </section>
    </div>
  )
}

function SourceList({ sources, onDetail, onFetchSource, onTestSource }: { sources: SourceDefinition[]; onDetail: (source: SourceDefinition) => void; onFetchSource: (sourceID: number) => Promise<ApiResult>; onTestSource: (sourceID: number) => Promise<ApiResult> }) {
  const [fetchingID, setFetchingID] = useState<number | null>(null)
  const [testingID, setTestingID] = useState<number | null>(null)
  const [messageByID, setMessageByID] = useState<Record<number, string>>({})
  if (sources.length === 0) return <EmptyState text="暂无数据源配置。" />

  async function fetch(sourceID: number) {
    setFetchingID(sourceID)
    const response = await onFetchSource(sourceID)
    const summary = response.ok ? parseSourceFetchSummary(response.data) : null
    setMessageByID((current) => ({
      ...current,
      [sourceID]: summary
        ? `拉取完成：成功 ${summary.successCount}/${summary.totalCount}，失败 ${summary.failedCount}；追踪 ${summary.traceID}`
        : response.error?.message || '拉取完成，但未收到可验证的结果摘要。',
    }))
    setFetchingID(null)
  }

  async function test(sourceID: number) {
    setTestingID(sourceID)
    const response = await onTestSource(sourceID)
    setMessageByID((current) => ({ ...current, [sourceID]: response.ok ? '连接测试通过。' : response.error?.message || '连接测试未完成，请稍后重试。' }))
    setTestingID(null)
  }

  return (
    <div className="data-table-wrap" role="region" aria-label="数据源列表" tabIndex={0}>
      <table className="data-table">
        <thead><tr><th scope="col">ID</th><th scope="col">数据源名称</th><th scope="col">编码</th><th scope="col">类型</th><th scope="col">鉴权方式</th><th scope="col">接收键</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
        <tbody>{sources.map((source) => (
          <tr key={source.id}>
            <td>#{source.id}</td><td><strong>{source.name}</strong>{source.has_secret && <small>配置已脱敏</small>}</td><td>{source.code}</td><td>{source.source_type}</td><td>{source.auth_type || 'none'}</td><td>{source.source_query_key || '-'}</td><td><StatusPill label={source.enabled ? '启用' : '停用'} /></td>
            <td><div className="table-actions"><button type="button" disabled={testingID !== null || fetchingID !== null} onClick={() => onDetail(source)}>详情</button><button type="button" disabled={testingID === source.id || fetchingID === source.id || !source.enabled} onClick={() => { void test(source.id) }}>{testingID === source.id ? '测试中…' : '测试连接'}</button><button type="button" disabled={testingID === source.id || fetchingID === source.id || !source.enabled || source.source_type === 'webhook'} onClick={() => { void fetch(source.id) }}>{fetchingID === source.id ? '拉取中…' : '手动拉取'}</button></div>{messageByID[source.id] && <small className="source-operation-message" role="status" aria-live="polite">{messageByID[source.id]}</small>}</td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  )
}

function TransformRuleList({ rules, sources, onDetail }: { rules: TransformRule[]; sources: SourceDefinition[]; onDetail: (rule: TransformRule) => void }) {
  if (rules.length === 0) return <EmptyState text="暂无处理规则。" />
  return (
    <div className="data-table-wrap" role="region" aria-label="清洗规则列表" tabIndex={0}>
      <table className="data-table design-rules-table"><thead><tr><th scope="col">规则名称</th><th scope="col">规则类型</th><th scope="col">来源</th><th scope="col">执行顺序</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
        <tbody>{rules.map((rule) => {
          const source = sources.find((item) => item.id === rule.source_id)
          return <tr key={rule.id}><td><strong>{rule.name}</strong>{rule.has_secret && <small>配置已脱敏</small>}</td><td>{rule.rule_type}</td><td>{source ? source.name : `#${rule.source_id}`}</td><td>{rule.order_index}</td><td><StatusPill label={rule.enabled ? '启用' : '停用'} /></td><td><button className="design-table-link" type="button" onClick={() => onDetail(rule)}>查看</button></td></tr>
        })}</tbody>
      </table>
    </div>
  )
}

function ProcessedDataList({ records }: { records: ProcessedData[] }) {
  if (records.length === 0) return <EmptyState text="暂无处理后数据。" />
  return (
    <div className="data-table-wrap" role="region" aria-label="旧处理结果列表" tabIndex={0}>
      <table className="data-table">
        <thead><tr><th scope="col">数据类型</th><th scope="col">Raw ID</th><th scope="col">质量分数</th><th scope="col">处理时间</th><th scope="col">操作</th></tr></thead>
        <tbody>{records.map((record) => (
          <tr key={record.id}>
            <td><strong>{record.data_type || 'processed'}</strong><small>记录 #{record.id}</small></td>
            <td>#{record.raw_data_id}</td>
            <td>{formatQualityScore(record.quality_score)}</td>
            <td>{formatUnixTime(record.created_at)}</td>
            <td><details><summary>查看字段</summary><ReadonlyJSON value={redactMonitoringJSON(parseJsonText(record.data_fields))} /></details></td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  )
}

function CleanRecordList({ records }: { records: CleanRecord[] }) {
  if (records.length === 0) return <EmptyState text="暂无清洗记录。" />
  return (
    <div className="data-table-wrap" role="region" aria-label="清洗记录列表" tabIndex={0}>
      <table className="data-table design-processed-table">
        <thead><tr><th scope="col">业务键</th><th scope="col">数据类型</th><th scope="col">Raw ID</th><th scope="col">质量分数</th><th scope="col">状态</th><th scope="col">处理时间</th><th scope="col">操作</th></tr></thead>
        <tbody>{records.map((record) => (
          <tr key={record.id}>
            <td><strong>{record.business_key || `#${record.id}`}</strong></td>
            <td>{record.table_name || '-'}</td>
            <td>#{record.raw_record_id}<small>来源 #{record.source_id}</small></td>
            <td><div className={record.quality_score >= 80 ? 'design-quality-score' : 'design-quality-score review'}><strong>{Math.round(record.quality_score)}</strong><progress value={Math.max(0, Math.min(100, record.quality_score))} max="100" aria-label={`质量分 ${formatQualityScore(record.quality_score)}`} /></div></td>
            <td><StatusPill label={record.quality_score >= 80 ? '高质量' : '待复核'} /><small>{cleanRecordStatusLabel(record.status)}</small></td>
            <td>{formatUnixTime(record.created_at)}</td>
            <td><details className="design-row-details"><summary className="design-table-link">查看</summary><dl><div><dt>记录 ID</dt><dd>#{record.id}</dd></div><div><dt>业务状态</dt><dd>{cleanRecordStatusLabel(record.status)}</dd></div></dl></details></td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  )
}

function DestinationList({ destinations, testingID, onDetail, onTest }: { destinations: DestinationDefinition[]; testingID: number | null; onDetail: (destination: DestinationDefinition) => void; onTest: (destination: DestinationDefinition) => void }) {
  if (destinations.length === 0) return <EmptyState text="暂无推送目标。" />
  return (
    <div className="data-table-wrap" role="region" aria-label="推送目标列表" tabIndex={0}>
      <table className="data-table"><thead><tr><th scope="col">目标系统</th><th scope="col">目标编码</th><th scope="col">接口类型</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
        <tbody>{destinations.map((destination) => <tr key={destination.id}><td><strong>{destination.name}</strong>{destination.has_secret && <small>配置已脱敏</small>}</td><td>{destination.code}</td><td>{destination.destination_type}</td><td><StatusPill label={destination.enabled ? '启用' : '停用'} /></td><td><div className="table-actions"><button type="button" onClick={() => onDetail(destination)}>详情</button><button type="button" disabled={testingID !== null} onClick={() => onTest(destination)}>{testingID === destination.id ? '测试中…' : '测试连接'}</button></div></td></tr>)}</tbody>
      </table>
    </div>
  )
}

function DeliveryTaskList({ tasks, runningID, loadingDetailID, destinations, onDetail, onRun }: {
  tasks: DeliveryTask[]
  runningID: number | null
  loadingDetailID: number | null
  destinations: DestinationDefinition[]
  onDetail: (task: DeliveryTask) => void
  onRun: (task: DeliveryTask) => void
}) {
  if (tasks.length === 0) return <EmptyState text="暂无推送任务。" />
  return (
    <div className="data-table-wrap" role="region" aria-label="推送任务列表" tabIndex={0}>
      <table className="data-table"><thead><tr><th scope="col">任务名称</th><th scope="col">触发方式</th><th scope="col">清洗表</th><th scope="col">推送目标</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
        <tbody>{tasks.map((task) => <tr key={task.id}><td><strong>{task.name}</strong></td><td>{deliveryTaskTriggerLabel(task.trigger_type)}{task.trigger_type === 'schedule' && task.cron_expr && <small>{task.cron_expr}</small>}</td><td>{task.clean_table}</td><td>{deliveryTaskDestinationLabel(task, destinations)}</td><td><StatusPill label={task.enabled ? '启用' : '停用'} /></td><td><div className="table-actions"><button type="button" disabled={loadingDetailID !== null || runningID !== null} onClick={() => onDetail(task)}>{loadingDetailID === task.id ? '加载中…' : '详情'}</button><button type="button" disabled={!task.enabled || runningID !== null || loadingDetailID !== null} onClick={() => onRun(task)}>{runningID === task.id ? '推送中…' : '手动运行'}</button></div></td></tr>)}</tbody>
      </table>
    </div>
  )
}

function ResultPanel({ result, onClose }: { result: ApiResult | null; onClose: () => void }) {
  if (!result) return null
  return (
    <aside className="result-panel" aria-label="接口结果">
      <div className="result-panel-header">
        <PanelTitle icon={<FileJson />} title="接口结果" meta={String(result.status)} />
        <button type="button" onClick={onClose} aria-label="关闭接口结果"><X aria-hidden="true" /></button>
      </div>
      <ReadonlyJSON value={redactMonitoringJSON(result.data)} />
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

function SelectFilter({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: Array<{ value: string; label: string }> }) {
  return (
    <label>
      {label}
      <select name={`filter-${label}`} value={value} onChange={(event) => onChange(event.currentTarget.value)}>
        <option value="all">全部</option>
        {options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
      </select>
    </label>
  )
}

function Metric({ label, value }: { label: string; value: ReactNode }) {
  return <div className="metric"><span>{label}</span><strong>{value}</strong></div>
}

function StatusPill({ label }: { label: string }) {
  const displayLabel = ({
    success: '已完成',
    running: '运行中',
    failed: '失败',
    partial_success: '部分成功',
    pending: '待处理',
    enabled: '已启用',
    disabled: '已停用',
  } as Record<string, string>)[label.toLowerCase()] ?? label
  const tone = /失败|错误|无效|停用|风险|超时/i.test(displayLabel)
    ? 'danger'
    : /成功|已就绪|启用|完成|已接收|已清洗|已交付|正常/i.test(displayLabel)
      ? 'success'
      : /处理中|运行中|排队|加载|待处理|待推送/i.test(displayLabel)
        ? 'warning'
        : 'neutral'
  return <span className={`status-pill ${tone}`}>{displayLabel}</span>
}

function EmptyState({ text }: { text: string }) {
  return <div className="empty-state">{text}</div>
}

function Field({ label, name, defaultValue = '', type = 'text', value, onChange, required = false, autoComplete }: { label: string; name: string; defaultValue?: string; type?: string; value?: string; onChange?: (value: string) => void; required?: boolean; autoComplete?: string }) {
  return (
    <label>
      {label}
      <input name={name} defaultValue={value === undefined ? defaultValue : undefined} value={value} type={type} required={required} autoComplete={autoComplete} onChange={onChange ? (event) => onChange(event.currentTarget.value) : undefined} />
    </label>
  )
}

function ExcelModelSelector({ name, models, value, onChange }: {
  name: string
  models: ExcelMatchModel[]
  value: string
  onChange: (value: string) => void
}) {
  const selectedModel = models.find((model) => model.tableName === value)
  const options = excelModelSelectOptions(models, value)
  return (
    <label className="excel-catalog-control">
      模型名称
      <select aria-label="模型名称" name={name} value={value} required onChange={(event) => onChange(event.currentTarget.value)}>
        <option value="">选择模型名称</option>
        {options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
      </select>
      {selectedModel
        ? <ExcelCatalogExplanation title={selectedModel.mapping} detail={selectedModel.description} />
        : value
          ? <ExcelCatalogExplanation title={`当前配置 → 数据库表 ${value}`} detail="该表不在当前模型目录中，保留它是为了兼容历史方案；保存前请确认表仍然存在。" />
          : <ExcelCatalogExplanation title="模型名称 → 数据库表" detail="选择模型后，这里会直接解释对应的数据表，无需另行查表。" />}
    </label>
  )
}

function ExcelModelFieldSelector({ label, name, models, tableName, value, onChange }: {
  label: string
  name: string
  models: ExcelMatchModel[]
  tableName: string
  value: string
  onChange: (value: string) => void
}) {
  const selectedModel = models.find((model) => model.tableName === tableName)
  const fields = selectedModel?.fields ?? []
  const selectedField = fields.find((field) => field.columnName === value)
  const options = excelFieldSelectOptions(fields, value)
  return (
    <label className="excel-catalog-control">
      {label}
      <select aria-label={label} name={name} value={value} required disabled={!tableName} onChange={(event) => onChange(event.currentTarget.value)}>
        <option value="">选择模型字段</option>
        {options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
      </select>
      {selectedField
        ? <ExcelModelFieldExplanation field={selectedField} />
        : value
          ? <ExcelCatalogExplanation title={`当前配置字段 → ${tableName}.${value}`} detail="该字段不在当前模型目录中，已作为历史配置保留；保存前请确认字段仍然存在。" />
          : selectedModel
            ? <ExcelCatalogExplanation title={`${selectedModel.modelName}.字段 → ${selectedModel.tableName}.数据库列`} detail={`当前模型提供 ${fields.length} 个字段可选。`} />
            : <ExcelCatalogExplanation title="模型字段 → 数据库列" detail="请先选择模型，再从该模型的字段列表中选择。" />}
    </label>
  )
}

function ExcelModelFieldExplanation({ field }: { field: ExcelMatchModelField }) {
  const typeDetail = field.dataType && !field.description.includes(field.dataType) ? `；数据库类型 ${field.dataType}` : ''
  return (
    <ExcelCatalogExplanation
      title={field.mapping}
      detail={`${field.description}${typeDetail}；${field.nullable ? '允许为空' : '不允许为空'}`}
    />
  )
}

function ExcelCatalogExplanation({ title, detail }: { title: string; detail: string }) {
  return (
    <span className="excel-catalog-explanation">
      <strong>{title}</strong>
      <small>{detail}</small>
    </span>
  )
}

function CopyableRedactedJSON({ label, value }: { label: string; value: unknown }) {
  const [message, setMessage] = useState('')
  const redacted = redactMonitoringJSON(value)

  async function copy() {
    if (!navigator.clipboard?.writeText) {
      setMessage('当前浏览器不支持复制，请手动选择内容。')
      return
    }
    try {
      await navigator.clipboard.writeText(jsonText(redacted))
      setMessage('已复制脱敏内容。')
    } catch {
      setMessage('复制失败，请手动选择内容。')
    }
  }

  return <section>
    <div className="step-runs-json-heading"><h3>{label}</h3><button type="button" onClick={() => void copy()}>复制</button></div>
    {message && <small role="status" aria-live="polite">{message}</small>}
    <ReadonlyJSON value={redacted} />
  </section>
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
  if (metadata && typeof metadata === 'object' && (metadata as JsonRecord).format === 'fetch') return 'fetch'
  if (metadata && typeof metadata === 'object' && typeof (metadata as JsonRecord).format === 'string') return String((metadata as JsonRecord).format)
  if (record.source) return record.source
  if (record.remark) return record.remark
  if (metadata && typeof metadata === 'object' && typeof (metadata as JsonRecord).source === 'string') return String((metadata as JsonRecord).source)
  if (metadata && typeof metadata === 'object' && typeof (metadata as JsonRecord).remark === 'string') return String((metadata as JsonRecord).remark)
  return 'ingest'
}

function cleanRecordStatusLabel(status: string) {
  if (status === 'ready') return '待推送'
  if (status === 'invalid') return '无效'
  if (status === 'delivered') return '已交付'
  return status || '-'
}

function formatQualityScore(value: number) {
  return Number.isFinite(value) ? `${value.toFixed(1)} / 100` : '-'
}

function ruleDraftFrom(rule: TransformRule): RuleDraft {
  return {
    id: rule.id,
    sourceID: String(rule.source_id),
    name: rule.name,
    ruleType: rule.rule_type,
    orderIndex: String(rule.order_index),
    configJSON: rule.config_json || '{}',
    enabled: rule.enabled,
    hasSecret: Boolean(rule.has_secret),
  }
}

function destinationDraftFrom(destination: DestinationDefinition): DestinationDraft {
  return { id: destination.id, name: destination.name, code: destination.code, destinationType: destination.destination_type, configJSON: destination.config_json || '{}', enabled: destination.enabled, hasSecret: Boolean(destination.has_secret) }
}

function sourceDraftFrom(source: SourceDefinition): SourceDraft {
  return { id: source.id, name: source.name, code: source.code, sourceType: source.source_type, enabled: source.enabled, authType: source.auth_type, configJSON: jsonText(source.config_json || '{}'), schemaJSON: jsonText(source.schema_json || '{}'), dedupeKeys: jsonText(source.dedupe_keys || '[]'), sourceQueryKey: source.source_query_key, hasSecret: Boolean(source.has_secret) }
}

function deliveryTaskDraftFrom(task: DeliveryTask): DeliveryTaskDraft {
  return {
    id: task.id,
    name: task.name,
    sourceID: String(task.source_id),
    cleanTable: task.clean_table,
    destinationID: String(task.destination_id),
    triggerType: task.trigger_type,
    cronExpr: task.cron_expr,
    filterJSON: jsonText(task.filter_json || '{}'),
    payloadTemplate: task.payload_template,
    enabled: task.enabled,
  }
}

function deliveryTaskDestinationLabel(task: DeliveryTask, destinations: DestinationDefinition[]) {
  const destination = destinations.find((item) => item.id === task.destination_id)
  return destination ? `${destination.name || destination.code} (#${destination.id})` : `目标 #${task.destination_id}`
}

function deliveryTaskTriggerLabel(value: string) {
  const labels: Record<string, string> = { manual: '手动', schedule: '定时', event: '事件' }
  return labels[value] ?? (value || '-')
}

function readList<T>(result: ApiResult, key: string): T[] {
  const value = readDataField(result.data, key)
  return Array.isArray(value) ? (value as T[]) : []
}

function readObject<T>(result: ApiResult, key: string): T | null {
  const value = readDataField(result.data, key)
  return value && typeof value === 'object' ? (value as T) : null
}

function filterSensitiveExcelModels(models: ExcelMatchModel[]): ExcelMatchModel[] {
  return models.flatMap((model): ExcelMatchModel[] => {
    if (
      !model
      || typeof model.name !== 'string'
      || typeof model.modelName !== 'string'
      || typeof model.tableName !== 'string'
      || typeof model.description !== 'string'
      || typeof model.mapping !== 'string'
      || !Array.isArray(model.fields)
      || isSensitiveExcelCatalogValue(model.name, model.modelName, model.tableName, model.description)
    ) return []

    return [{
      ...model,
      fields: model.fields.filter((field) => field
        && typeof field.name === 'string'
        && typeof field.modelField === 'string'
        && typeof field.columnName === 'string'
        && typeof field.description === 'string'
        && !isSensitiveExcelCatalogValue(field.name, field.modelField, field.columnName, field.description)),
    }]
  })
}

function isSensitiveExcelCatalogValue(...values: string[]) {
  return values.some((value) => /(?:access[_-]?token|refresh[_-]?token|authorization|api[_-]?key|secret|password|credential)/i.test(value))
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

function loginFailureMessage(status: number) {
  if (status === 401) return '账号或密码不正确，请重试。'
  if (status === 429) return '登录尝试过于频繁，请稍后再试。'
  if (status >= 500) return '登录服务暂时不可用，请稍后再试。'
  return '登录请求未完成，请检查账号和密码后重试。'
}

function formValue(form: FormData, key: string) {
  const value = form.get(key)
  return typeof value === 'string' ? value : ''
}

function parseExportColumnFormats(raw: string): ExcelExportColumnFormat[] {
  return raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const separator = line.includes('=') ? '=' : ':'
      const [column = '', format = ''] = line.split(separator)
      return {
        column: column.trim(),
        format: format.trim().toLowerCase(),
      }
    })
    .filter((item) => item.column && item.format)
}

function exportColumnFormatsText(formats: ExcelExportColumnFormat[] | undefined) {
  if (!Array.isArray(formats)) return ''
  return formats
    .filter((item) => item.column && item.format)
    .map((item) => `${item.column}=${item.format}`)
    .join('\n')
}

function sameExcelFile(file: File, ref: ExcelUploadRef) {
  return file.name === ref.fileName && file.size === ref.size && file.lastModified === ref.lastModified
}

function exportSchemeDefaults(config: ExcelMatchSchemeConfig): ExcelExportSchemeConfig {
  const steps = migrateExcelMatchSteps(config, defaultExcelExportScheme.steps[0])
  return {
    sheetName: config.sheetName || defaultExcelExportScheme.sheetName,
    steps,
    exportColumnFormats: exportColumnFormatsText(config.exportColumnFormats) || defaultExcelExportScheme.exportColumnFormats,
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

function excelJobOperation(job: ExcelMatchJob) {
  if (job.operation) return job.operation
  const config = parseMaybeJson(job.config_json ?? '')
  if (config && typeof config === 'object' && typeof (config as JsonRecord).operation === 'string') {
    return String((config as JsonRecord).operation)
  }
  return ''
}

function excelJobOperationLabel(value: string) {
  const labels: Record<string, string> = {
    export_match: '匹配导出',
    import_update: '匹配导入',
    clear_matched_docno: '退回未匹配',
  }
  return labels[value] ?? (value || '-')
}

function canDownloadExcelJob(job: ExcelMatchJob) {
  if (typeof job.can_download === 'boolean') return job.can_download
  return job.status === 'success' && excelJobOperation(job) === 'export_match' && Boolean(job.result_url)
}

function isExcelJobActive(job: ExcelMatchJob | null | undefined) {
  return Boolean(job && !['success', 'failed', 'expired'].includes(job.status))
}

function replaceExcelJobHistoryItem(jobs: ExcelMatchJob[], nextJob: ExcelMatchJob) {
  const index = jobs.findIndex((item) => item.id === nextJob.id)
  if (index === -1) return jobs
  const nextJobs = [...jobs]
  nextJobs[index] = nextJob
  return nextJobs
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

function submitExcelDownloadForm(jobID: number, token: string) {
  if (!token) throw new Error('登录状态已失效，请重新登录后下载')

  const frameName = `excel-download-frame-${jobID}`
  let iframe = document.querySelector<HTMLIFrameElement>(`iframe[name="${frameName}"]`)
  if (!iframe) {
    iframe = document.createElement('iframe')
    iframe.name = frameName
    iframe.style.display = 'none'
    document.body.appendChild(iframe)
  }

  const form = document.createElement('form')
  form.method = 'POST'
  form.action = apiURL(`/v1/excel-match-jobs/${jobID}/download`)
  form.target = frameName
  form.style.display = 'none'

  const tokenInput = document.createElement('input')
  tokenInput.type = 'hidden'
  tokenInput.name = 'token'
  tokenInput.value = token
  form.appendChild(tokenInput)

  document.body.appendChild(form)
  form.submit()
  form.remove()

  window.setTimeout(() => {
    iframe?.remove()
  }, 5 * 60 * 1000)
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

function monitoringDayStartTime() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}T00:00`
}

function runDurationLabel(startedAt: string | null, finishedAt: string | null) {
  if (!startedAt || !finishedAt) return '-'
  const started = Date.parse(startedAt)
  const finished = Date.parse(finishedAt)
  const milliseconds = finished - started
  if (!Number.isFinite(milliseconds) || milliseconds < 0) return '-'
  const seconds = Math.floor(milliseconds / 1000)
  if (seconds < 60) return `${seconds} 秒`
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  return `${minutes} 分 ${remainingSeconds} 秒`
}

function datetimeLocalMinutesAgo(minutes: number) {
  const date = new Date(Date.now() - minutes * 60 * 1000)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function youzanDistributionBackfillStatusLabel(value: string) {
  const labels: Record<string, string> = {
    pending: '待写入',
    created: '已写入',
    exists: '已存在',
    invalid: '无效',
    failed: '失败',
  }
  return labels[value] ?? (value || '-')
}

function youzanDistributionTimeFilterLabel(value: YouzanDistributionTimeFilter) {
  return value === 'success' ? '订单完成时间' : '下单时间'
}

function previousDayDateTimeLocal(endOfDay: boolean) {
  const date = new Date()
  date.setDate(date.getDate() - 1)
  date.setHours(endOfDay ? 23 : 0, endOfDay ? 59 : 0, endOfDay ? 59 : 0, 0)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function backendDateTime(value: string) {
  const normalized = value.trim().replace('T', ' ')
  return /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/.test(normalized) ? `${normalized}:00` : normalized
}

function unixTimestamp(value: string) {
  if (!value) return ''
  const timestamp = Date.parse(value)
  return Number.isFinite(timestamp) ? String(Math.floor(timestamp / 1000)) : ''
}

function includesQuery(values: Array<string | number | null | undefined>, query: string) {
  const normalized = query.trim().toLowerCase()
  if (!normalized) return true
  return values.some((value) => String(value ?? '').toLowerCase().includes(normalized))
}

function uniqueOptions(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort().map((value) => ({ value, label: value }))
}

function sum(items: PipelineRun[], key: 'success_count' | 'failed_count') {
  return items.reduce((total, item) => total + (Number(item[key]) || 0), 0)
}


export default App
