import { FormEvent, ReactNode, type RefObject, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  BookOpen,
  Building2,
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
import { apiURL as buildApiURL } from './apiURL'
import { clearStoredToken, loadStoredSessionUser, loadStoredToken, saveStoredSessionUser, saveStoredToken, saveStoredTokenExpiry, storedTokenExpiresAt, tokenActorID, type StoredSessionUser } from './authStorage'
import { createApiClient, type ApiRequestOptions, type ClientResponse, type HTTPMethod } from './api/client'
import { readSessionUser, readTokenInfo, type SessionUser } from './api/auth'
import { parseDataStatisticsSummary, parseHealthSummary, parseMallWeatherMetricsSummary, redactMonitoringJSON, type DataStatisticsSummary, type HealthSummary, type MallWeatherMetricsSummary } from './monitoring'
import { MallWeatherPage, StoreInfoPage } from './MallWeatherPage'
import { DataAuthorizationPage } from './DataAuthorizationPage'
import { PipelineRunPanel } from './PipelineRunPanel'
import { PipelineComposerPanel } from './PipelineComposerPanel'
import { Brand } from './components/Brand'
import { parseMallWeatherExportContentStatus, submitMallWeatherExportContentDownload } from './mallWeatherExport'
import { buildRawRecordsRequest, parseRawRecordsPage, type RawRecordOrigin, type RawRecordsPage } from './rawRecords'
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
  result_url?: string
  can_download?: boolean
  download_message?: string
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

type DeliveryStore = {
  key: string
  name: string
  aliases: string[]
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

const deliveryStores: DeliveryStore[] = [
  { key: 'shangsheng', name: '上生新所', aliases: ['ABCN001A001', 'shangsheng', '上生新所', '上升新所'] },
  { key: 'jialicheng', name: '嘉里城', aliases: ['ABCN001A004', 'jialicheng', 'kerry', '嘉里城'] },
  { key: 'panlong', name: '蟠龙', aliases: ['ABCN001A005', 'panlong', '蟠龙'] },
  { key: 'xintiandi', name: '新天地', aliases: ['ABCN001A003', 'xintiandi', '新天地'] },
  { key: 'qiantan', name: '前滩', aliases: ['ABCN001P012', 'qiantan', '前滩'] },
  { key: 'hangzhou_henglong', name: '杭州恒隆', aliases: ['ABCN002A001', 'hangzhou_henglong', 'henglong', '杭州恒隆'] },
]

const otherDeliveryStore: DeliveryStore = { key: 'other', name: '其他目标', aliases: [] }

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
  const [sessionUser, setSessionUser] = useState<SessionUser | null>(() => {
    const user = loadStoredSessionUser(window.localStorage)
    return user ? { ...user, email: '', consoleManaged: true } : null
  })
  const [sessionExpiresAt, setSessionExpiresAt] = useState(() => storedTokenExpiresAt(window.localStorage))
  const [activeNav, setActiveNav] = useState<NavKey>(navFromHash)
  const [expandedNavGroup, setExpandedNavGroup] = useState(() => navGroupFor(navFromHash())?.label ?? navGroups[0].label)
  const [navQuery, setNavQuery] = useState('')
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const mobileNavTriggerRef = useRef<HTMLButtonElement>(null)
  const mobileNavRef = useRef<HTMLElement>(null)
  const [loading, setLoading] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [result, setResult] = useState<ApiResult | null>(null)
  const [runs, setRuns] = useState<PipelineRun[]>([])
  const [stepRuns, setStepRuns] = useState<StepRun[]>([])
  const [selectedStepRunID, setSelectedStepRunID] = useState<number | null>(null)
  const stepRequestRef = useRef<AbortController | null>(null)
  const workspaceRequestRef = useRef<AbortController | null>(null)
  const [methods, setMethods] = useState<MethodDisplay[]>(builtinMethods)
  const [pipelines, setPipelines] = useState<PipelineDefinition[]>([])
  const [sources, setSources] = useState<SourceDefinition[]>([])
  const [transformRules, setTransformRules] = useState<TransformRule[]>([])
  const [destinations, setDestinations] = useState<DestinationDefinition[]>([])
  const [deliveryTasks, setDeliveryTasks] = useState<DeliveryTask[]>([])
  const [deliveryLogs, setDeliveryLogs] = useState<DeliveryLog[]>([])
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
      try {
        const get = (path: string) => client(path, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
        if (activeNav === 'overview') {
          const [runResult, logResult] = await Promise.all([get('/v1/runs?limit=50'), get('/v1/delivery-logs?limit=50')])
          if (!controller.signal.aborted) {
            if (runResult.ok) setRuns(readList<PipelineRun>(runResult, 'runs'))
            if (logResult.ok) setDeliveryLogs(readList<DeliveryLog>(logResult, 'logs'))
          }
        } else if (activeNav === 'runs' || activeNav === 'step_runs') {
          const runResult = await get('/v1/runs?limit=50')
          if (!controller.signal.aborted && runResult.ok) setRuns(readList<PipelineRun>(runResult, 'runs'))
        } else if (activeNav === 'delivery_logs') {
          const logResult = await get('/v1/delivery-logs?limit=50')
          if (!controller.signal.aborted && logResult.ok) setDeliveryLogs(readList<DeliveryLog>(logResult, 'logs'))
        } else if (activeNav === 'sources') {
          const sourceResult = await get('/v1/sources')
          if (!controller.signal.aborted && sourceResult.ok) setSources(readList<SourceDefinition>(sourceResult, 'sources'))
        } else if (activeNav === 'rules') {
          const [sourceResult, ruleResult] = await Promise.all([get('/v1/sources'), get('/v1/transform-rules')])
          if (!controller.signal.aborted) {
            if (sourceResult.ok) setSources(readList<SourceDefinition>(sourceResult, 'sources'))
            if (ruleResult.ok) setTransformRules(readList<TransformRule>(ruleResult, 'rules'))
          }
        } else if (activeNav === 'destinations') {
          const destinationResult = await get('/v1/destinations')
          if (!controller.signal.aborted && destinationResult.ok) setDestinations(readList<DestinationDefinition>(destinationResult, 'destinations'))
        } else if (activeNav === 'tasks') {
          const [sourceResult, destinationResult, taskResult] = await Promise.all([get('/v1/sources'), get('/v1/destinations'), get('/v1/delivery-tasks')])
          if (!controller.signal.aborted) {
            if (sourceResult.ok) setSources(readList<SourceDefinition>(sourceResult, 'sources'))
            if (destinationResult.ok) setDestinations(readList<DestinationDefinition>(destinationResult, 'destinations'))
            if (taskResult.ok) setDeliveryTasks(readList<DeliveryTask>(taskResult, 'tasks'))
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
            if (!controller.signal.aborted) setMethods([...buildConfiguredMethodDisplays(nextSources, nextRules, nextDestinations, nextTasks), ...buildLegacyMethodDisplays(nextLegacyTasks, nextLegacyRules), ...configuredMethods, ...builtinMethods])
          }
        }
        if (!controller.signal.aborted && showResult) setResult({ ok: true, status: 200, data: { refreshed_at: new Date().toISOString() } })
      } finally {
        if (workspaceRequestRef.current === controller) {
          workspaceRequestRef.current = null
          setRefreshing(false)
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
    if (!token) return
    let current = true
    const controller = new AbortController()
    setSessionState('checking')
    void Promise.all([
      apiClient.request('/auth/me', { method: 'GET', signal: controller.signal }),
      apiClient.request('/auth/token/info', { method: 'GET', signal: controller.signal }),
    ]).then(([profileResponse, tokenInfoResponse]) => {
      if (!current) return
      const user = profileResponse.ok ? readSessionUser(profileResponse.data) : null
      const tokenInfo = tokenInfoResponse.ok ? readTokenInfo(tokenInfoResponse.data) : null
      if (!user || !tokenInfo || tokenInfo.userID !== user.id) {
        if (profileResponse.error?.kind === 'unauthorized' || tokenInfoResponse.error?.kind === 'unauthorized') clearSession()
        else setSessionState('anonymous')
        return
      }
      const storedUser: StoredSessionUser = { id: user.id, account: user.account, nickname: user.nickname }
      saveStoredSessionUser(storedUser, window.localStorage)
      saveStoredTokenExpiry(tokenInfo.expireTime * 1000, window.localStorage)
      setSessionUser(user)
      setSessionExpiresAt(tokenInfo.expireTime * 1000)
      setSessionState('authenticated')
    })
    return () => {
      current = false
      controller.abort()
    }
  }, [apiClient, clearSession, token])

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

  useEffect(() => () => stepRequestRef.current?.abort(), [])

  useEffect(() => {
    if (sessionState !== 'authenticated' || activeNav !== 'overview') return
    const controller = new AbortController()
    void Promise.all([
      client('/v1/data/statistics', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }),
      client('/v1/mall-weather/metrics', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }),
      client('/health', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }),
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

  async function loadStepRuns(runId: number) {
    stepRequestRef.current?.abort()
    const controller = new AbortController()
    stepRequestRef.current = controller
    setSelectedStepRunID(runId)
    const response = await client(`/v1/pipeline-runs/${runId}/steps`, { method: 'GET', signal: controller.signal })
    if (!controller.signal.aborted && response.ok) setStepRuns(readList<StepRun>(response, 'step_runs'))
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
            const expanded = Boolean(query) || expandedNavGroup === group.label
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
        <ModuleHeader activeNav={activeNav} loading={loading || refreshing} sessionUser={sessionUser} onOpenNavigation={openMobileNavigation} mobileNavTriggerRef={mobileNavTriggerRef} />
        {activeNav === 'overview' && <PushStatusView runs={runs} deliveryLogs={deliveryLogs} monitoring={monitoring} stale={monitoringStale} onLoadSteps={loadStepRuns} />}
        {activeNav === 'runs' && <RunsQueryPage runs={runs} onLoadSteps={loadStepRuns} />}
        {activeNav === 'delivery_logs' && <DeliveryLogsQueryPage logs={deliveryLogs} onRetryLog={retryDeliveryLog} />}
        {activeNav === 'step_runs' && <StepRunsQueryPage runs={runs} stepRuns={stepRuns} selectedRunID={selectedStepRunID} onLoadSteps={loadStepRuns} />}
        {activeNav === 'store_info' && <StoreInfoPage actorID={actorID} client={client} downloadFile={downloadFile} />}
        {activeNav === 'mall_weather' && <MallWeatherPage actorID={actorID} client={client} downloadFile={downloadFile} />}
        {activeNav === 'data_authorizations' && <DataAuthorizationPage client={client} />}
        {activeNav === 'sources' && <SourcesQueryPage client={client} sources={sources} onFetchSource={fetchSource} onTestSource={testSource} onRefresh={() => refreshWorkspace(false)} />}
        {activeNav === 'methods' && <MethodsView methods={methods} pipelines={pipelines} client={client} coreMethods={coreMethods} onToggle={toggleTarget} onPipelineRunCompleted={() => void refreshWorkspace(false)} />}
        {activeNav === 'receive' && <RawRecordsQueryPage title="接口接收记录" origin="receive" client={client} />}
        {activeNav === 'pull_records' && <RawRecordsQueryPage title="数据拉取记录" origin="pull" client={client} />}
        {activeNav === 'backfill' && <BojunBackfillPage loading={loading || refreshing} onPreview={previewBojunOrderBackfill} onConfirm={confirmBojunOrderBackfill} />}
        {activeNav === 'youzan_distribution' && <YouzanDistributionPage task={legacyTasks.find((item) => item.code === 'youzan_distribution_order_fetch')} loading={loading || refreshing} onPreview={previewYouzanDistributionBackfill} onConfirm={confirmYouzanDistributionBackfill} onRun={runLegacyTask} />}
        {activeNav === 'rules' && <RulesQueryPage client={client} rules={transformRules} sources={sources} onRulesChange={setTransformRules} />}
        {activeNav === 'processed' && <ProcessedQueryPage client={client} />}
        {activeNav === 'destinations' && <DestinationsQueryPage client={client} destinations={destinations} onRefresh={() => refreshWorkspace(false)} />}
        {activeNav === 'tasks' && <DeliveryTasksQueryPage client={client} tasks={deliveryTasks} sources={sources} destinations={destinations} onRefresh={() => refreshWorkspace(false)} />}
        {activeNav === 'push_policy' && <PushPolicyPage coreMethod={coreMethods.find((item) => item.key === 'mall_push')} config={orderPushSkipConfig} targets={orderPushTargets} onSave={saveOrderPushSkipConfig} onToggle={toggleTarget} />}
        {(activeNav === 'excel_jobs' || activeNav === 'excel_schemes' || activeNav === 'excel_write') && <ExcelMatchView section={activeNav === 'excel_jobs' ? 'jobs' : activeNav === 'excel_schemes' ? 'schemes' : 'write'} token={token} loading={loading} setLoading={setLoading} setResult={setResult} onNavigateToJobs={() => navigate('excel_jobs')} />}
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

function ModuleHeader({ activeNav, loading, sessionUser, onOpenNavigation, mobileNavTriggerRef }: { activeNav: NavKey; loading: boolean; sessionUser: SessionUser | null; onOpenNavigation: () => void; mobileNavTriggerRef: RefObject<HTMLButtonElement> }) {
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
        {sessionUser && <span>{sessionUser.nickname || sessionUser.account}</span>}
        <StatusPill label={loading ? '加载中' : '已就绪'} />
      </div>
    </header>
  )
}

function PushStatusView({ runs, deliveryLogs, monitoring, stale, onLoadSteps }: { runs: PipelineRun[]; deliveryLogs: DeliveryLog[]; monitoring: MonitoringSnapshot; stale: boolean; onLoadSteps: (runId: number) => void }) {
  const deliveryRuns = runs.filter((run) => run.run_type === 'delivery')
  const failedLogs = deliveryLogs.filter((log) => !log.success)
  return (
    <div className="view-stack">
      <section className="overview-grid">
        <Metric label="接收总量" value={monitoring.statistics?.totalCount ?? '-'} />
        <Metric label="已处理" value={monitoring.statistics?.processedCount ?? '-'} />
        <Metric label="处理失败" value={monitoring.statistics?.errorCount ?? '-'} />
        <Metric label="交付失败" value={sum(deliveryRuns, 'failed_count') + failedLogs.length} />
      </section>
      <section className="overview-grid compact" aria-live="polite">
        <Metric label="天气告警" value={monitoring.weather?.firingAlerts ?? '-'} />
        <Metric label="天气拉取失败" value={monitoring.weather?.failedFetches ?? '-'} />
        <Metric label="服务健康" value={monitoring.health?.healthy ? '正常' : '未知'} />
        <Metric label="运行记录" value={deliveryLogs.length} />
      </section>
      {stale && <p className="backfill-note" role="status">部分监控数据暂时不可用，正在保留最近一次成功结果。</p>}
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

function MethodsView({ methods, pipelines, client, coreMethods, onToggle, onPipelineRunCompleted }: { methods: MethodDisplay[]; pipelines: PipelineDefinition[]; client: ApiClient; coreMethods: CoreMethod[]; onToggle: (target: ToggleTarget, enabled: boolean) => void; onPipelineRunCompleted: () => void }) {
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('all')
  const [status, setStatus] = useState('all')
  const filtered = methods.filter((method) => includesQuery([method.name, method.code, method.description, method.owner], query)
    && (category === 'all' || method.category === category)
    && (status === 'all' || (status === 'enabled' ? method.enabled : !method.enabled)))
  const groups = groupBy(filtered, (method) => method.category)
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
      <PipelineRunPanel pipelines={pipelines} client={client} onRunCompleted={onPipelineRunCompleted} />
      <PipelineComposerPanel pipelines={pipelines} client={client} onRefresh={onPipelineRunCompleted} />
      <QueryBar count={filtered.length} total={methods.length}>
        <Field label="名称 / 编码 / 负责人" name="method_query" value={query} onChange={setQuery} />
        <SelectFilter label="分类" value={category} onChange={setCategory} options={uniqueOptions(methods.map((method) => method.category))} />
        <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
      </QueryBar>
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

function RunsQueryPage({ runs, onLoadSteps }: { runs: PipelineRun[]; onLoadSteps: (runId: number) => void }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [runType, setRunType] = useState('all')
  const filtered = useMemo(() => runs.filter((run) => {
    const matchesQuery = includesQuery([run.id, run.trace_id, run.trigger_type, run.error_message], query)
    return matchesQuery && (status === 'all' || run.status === status) && (runType === 'all' || run.run_type === runType)
  }), [query, runType, runs, status])
  return (
    <div className="view-stack">
      <QueryBar count={filtered.length} total={runs.length}>
        <Field label="ID / Trace / 错误" name="run_query" value={query} onChange={setQuery} />
        <SelectFilter label="状态" value={status} onChange={setStatus} options={uniqueOptions(runs.map((run) => run.status))} />
        <SelectFilter label="运行类型" value={runType} onChange={setRunType} options={uniqueOptions(runs.map((run) => run.run_type))} />
      </QueryBar>
      <Panel title="运行记录" icon={<Activity />} meta={`查询命中 ${filtered.length} 条`}><RunTable runs={filtered} onLoadSteps={onLoadSteps} /></Panel>
    </div>
  )
}

function DeliveryLogsQueryPage({ logs, onRetryLog }: { logs: DeliveryLog[]; onRetryLog: (logId: number) => Promise<void> }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [retryingLogID, setRetryingLogID] = useState<number | null>(null)
  const [pendingRetryLog, setPendingRetryLog] = useState<DeliveryLog | null>(null)
  const filtered = useMemo(() => logs.filter((log) => {
    const matchesQuery = includesQuery([log.id, log.trace_id, log.business_key, log.source_code, log.destination_code, log.destination_name, log.error_message], query)
    const matchesStatus = status === 'all' || (status === 'success' ? log.success : !log.success)
    return matchesQuery && matchesStatus
  }), [logs, query, status])

  async function retryPendingLog() {
    if (!pendingRetryLog || retryingLogID !== null) return
    setRetryingLogID(pendingRetryLog.id)
    try {
      await onRetryLog(pendingRetryLog.id)
    } finally {
      setRetryingLogID(null)
      setPendingRetryLog(null)
    }
  }

  return (
    <div className="view-stack">
      <QueryBar count={filtered.length} total={logs.length}>
        <Field label="业务键 / Trace / 来源 / 目标" name="delivery_query" value={query} onChange={setQuery} />
        <SelectFilter label="交付状态" value={status} onChange={setStatus} options={[{ value: 'success', label: '成功' }, { value: 'failed', label: '失败' }]} />
      </QueryBar>
      <Panel title="推送日志" icon={<Send />} meta={`当前仅显示后端返回的最近 ${logs.length} 条`}><DeliveryLogList logs={filtered} retryingLogID={retryingLogID} onRetryLog={(log) => {
        if (!log.success && retryingLogID === null) setPendingRetryLog(log)
      }} /></Panel>
      {pendingRetryLog && <Modal title="确认重试推送日志" closeDisabled={retryingLogID !== null} onClose={() => { if (retryingLogID === null) setPendingRetryLog(null) }} footer={<><button type="button" disabled={retryingLogID !== null} onClick={() => setPendingRetryLog(null)}>取消</button><button className="primary" type="button" disabled={retryingLogID !== null} onClick={() => void retryPendingLog()}>{retryingLogID === pendingRetryLog.id ? '重试中…' : '确认重试'}</button></>}><p>确认重试失败日志 #{pendingRetryLog.id}？这会再次向原推送目标发起交付请求。</p></Modal>}
    </div>
  )
}

function StepRunsQueryPage({ runs, stepRuns, selectedRunID, onLoadSteps }: { runs: PipelineRun[]; stepRuns: StepRun[]; selectedRunID: number | null; onLoadSteps: (runId: number) => void }) {
  const [runQuery, setRunQuery] = useState('')
  const [stepQuery, setStepQuery] = useState('')
  const visibleRuns = runs.filter((run) => includesQuery([run.id, run.trace_id, run.run_type], runQuery))
  const visibleSteps = stepRuns.filter((step) => includesQuery([step.id, step.run_id, step.step_code, step.method_type, step.status, step.error_message], stepQuery))
  return (
    <div className="view-stack">
      <QueryBar count={visibleRuns.length} total={runs.length}>
        <Field label="先查询运行" name="step_run_query" value={runQuery} onChange={setRunQuery} />
      </QueryBar>
      <Panel title="选择运行" icon={<Activity />} meta="点击步骤加载该次运行"><RunTable runs={visibleRuns} onLoadSteps={onLoadSteps} /></Panel>
      <QueryBar count={visibleSteps.length} total={stepRuns.length}>
        <Field label="步骤编码 / 类型 / 状态" name="step_query" value={stepQuery} onChange={setStepQuery} />
      </QueryBar>
      <Panel title="步骤明细" icon={<BookOpen />} meta={selectedRunID ? `运行 #${selectedRunID} / ${visibleSteps.length} 条` : '请先选择运行'}><StepRunList stepRuns={visibleSteps} /></Panel>
    </div>
  )
}

function SourcesQueryPage({ client, sources, onFetchSource, onTestSource, onRefresh }: { client: ApiClient; sources: SourceDefinition[]; onFetchSource: (sourceID: number) => Promise<ApiResult>; onTestSource: (sourceID: number) => Promise<ApiResult>; onRefresh: () => Promise<void> }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [sourceType, setSourceType] = useState('all')
  const [draft, setDraft] = useState<SourceDraft | null>(null)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')
  const filtered = sources.filter((source) => includesQuery([source.id, source.name, source.code, source.auth_type], query)
    && (status === 'all' || (status === 'enabled' ? source.enabled : !source.enabled))
    && (sourceType === 'all' || source.source_type === sourceType))

  async function openDetail(id: number) {
    setMessage('')
    const response = await client(`/v1/sources/${id}`, { method: 'GET', showResult: false, silentLoading: true })
    const source = response.ok ? readObject<SourceDefinition>(response, 'source') : null
    if (!source) { setMessage(response.error?.message || '数据源详情暂时不可用。'); return }
    setDraft(sourceDraftFrom(source))
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft || saving || draft.hasSecret) return
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
    await onRefresh()
  }
  return (
    <div className="view-stack">
      {message && <div className="result-banner" role="status">{message}</div>}
      <QueryBar count={filtered.length} total={sources.length}>
        <Field label="名称 / 编码 / 鉴权" name="source_query" value={query} onChange={setQuery} />
        <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
        <SelectFilter label="类型" value={sourceType} onChange={setSourceType} options={uniqueOptions(sources.map((source) => source.source_type))} />
      </QueryBar>
      <div className="record-actions"><button type="button" className="primary" onClick={() => setDraft({ id: null, name: '', code: '', sourceType: 'api_poll', enabled: true, authType: 'none', configJSON: '{\n  "url": "",\n  "method": "GET",\n  "records_path": "data"\n}', schemaJSON: '{}', dedupeKeys: '[]', sourceQueryKey: '', hasSecret: false })}>新增数据源</button></div>
      <Panel title="数据源配置" icon={<Database />} meta={`查询命中 ${filtered.length} 条`}><SourceList sources={filtered} onDetail={(source) => { void openDetail(source.id) }} onFetchSource={onFetchSource} onTestSource={onTestSource} /></Panel>
      {draft && <Modal title={draft.id ? '数据源详情与编辑' : '新增数据源'} onClose={() => { if (!saving) setDraft(null) }}>
        {draft.hasSecret && <div className="result-banner error" role="alert">该配置含已隐藏凭据，当前仅可查看、测试和拉取；完整更新会覆盖真实凭据。</div>}
        <form className="excel-upload-form" onSubmit={save}>
          <Field label="数据源名称" name="source_name" value={draft.name} required onChange={(name) => setDraft({ ...draft, name })} />
          <Field label="数据源编码" name="source_code" value={draft.code} required onChange={(code) => setDraft({ ...draft, code })} />
          <label>数据源类型<select value={draft.sourceType} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, sourceType: event.currentTarget.value })}><option value="api_poll">API 轮询</option><option value="database">数据库</option><option value="webhook">Webhook</option></select></label>
          <Field label="鉴权类型" name="source_auth_type" value={draft.authType} onChange={(authType) => setDraft({ ...draft, authType })} />
          <label className="checkbox-label"><input type="checkbox" checked={draft.enabled} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, enabled: event.currentTarget.checked })} />启用数据源</label>
          <Field label="来源查询键" name="source_query_key" value={draft.sourceQueryKey} onChange={(sourceQueryKey) => setDraft({ ...draft, sourceQueryKey })} />
          <label>连接配置 JSON<textarea rows={10} value={draft.configJSON} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, configJSON: event.currentTarget.value })} /></label>
          <label>Schema JSON<textarea rows={5} value={draft.schemaJSON} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, schemaJSON: event.currentTarget.value })} /></label>
          <label>去重键 JSON 数组<textarea rows={4} value={draft.dedupeKeys} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, dedupeKeys: event.currentTarget.value })} /></label>
          <p className="query-contract-note">API 测试会发起真实连通性请求；Webhook 不支持主动拉取。Schema 与去重键目前由服务端保存，未参与拉取校验。</p>
          <div className="excel-form-actions"><button className="primary" type="submit" disabled={draft.hasSecret || saving}>{saving ? '保存中…' : '保存数据源'}</button></div>
        </form>
      </Modal>}
    </div>
  )
}

function RawRecordsQueryPage({ title, origin, client }: { title: string; origin: RawRecordOrigin; client: ApiClient }) {
  const [source, setSource] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [appliedQuery, setAppliedQuery] = useState({ source: '', startTime: '', endTime: '' })
  const [page, setPage] = useState(1)
  const [recordsPage, setRecordsPage] = useState<RawRecordsPage<RawData> | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
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
    setAppliedQuery({ source, startTime: backendDateTime(startTime), endTime: backendDateTime(endTime) })
  }

  const records = recordsPage?.list ?? []
  const total = recordsPage?.total ?? 0
  const totalPages = recordsPage?.totalPages ?? 0
  return (
    <div className="view-stack">
      <form className="query-bar" onSubmit={submit}>
        <div className="query-fields">
          <Field label="来源" name="raw_source" value={source} onChange={setSource} />
          <Field label="开始时间" name="raw_start_time" type="datetime-local" value={startTime} onChange={setStartTime} />
          <Field label="结束时间" name="raw_end_time" type="datetime-local" value={endTime} onChange={setEndTime} />
        </div>
        <button type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
      <p className="query-contract-note">已按真实后端能力提供来源、时间范围和分页筛选；类型、状态及业务键筛选需后端增加对应索引后再开放。</p>
      {error && <div className="result-banner error" role="alert">{error} 已保留最近一次成功数据。</div>}
      <Panel title={title} icon={<Inbox />} meta={loading && !recordsPage ? '正在加载…' : `共 ${total} 条`}>
        <RawDataList records={records} />
        <div className="record-actions raw-record-pagination" role="status" aria-live="polite">
          <span>第 {recordsPage?.page ?? page} / {Math.max(totalPages, 1)} 页</span>
          <button type="button" onClick={() => setPage((current) => Math.max(1, current - 1))} disabled={loading || page <= 1}>上一页</button>
          <button type="button" onClick={() => setPage((current) => current + 1)} disabled={loading || totalPages === 0 || page >= totalPages}>下一页</button>
        </div>
      </Panel>
    </div>
  )
}

function RulesQueryPage({ client, rules, sources, onRulesChange }: { client: ApiClient; rules: TransformRule[]; sources: SourceDefinition[]; onRulesChange: (rules: TransformRule[]) => void }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [ruleType, setRuleType] = useState('all')
  const [draft, setDraft] = useState<RuleDraft | null>(null)
  const [rawContent, setRawContent] = useState('{}')
  const [testResult, setTestResult] = useState<unknown>(null)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [operationError, setOperationError] = useState('')
  const filtered = rules.filter((rule) => includesQuery([rule.id, rule.name, rule.source_id, rule.rule_type], query)
    && (status === 'all' || (status === 'enabled' ? rule.enabled : !rule.enabled))
    && (ruleType === 'all' || rule.rule_type === ruleType))

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
    if (!draft || saving || draft.hasSecret) return
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
  }

  async function runTest(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft || draft.ruleType !== 'mapping' || draft.hasSecret || testing) return
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

  const testable = draft?.ruleType === 'mapping' && !draft.hasSecret
  return (
    <div className="view-stack">
      {operationError && <div className="result-banner error" role="alert">{operationError}</div>}
      <QueryBar count={filtered.length} total={rules.length}>
        <Field label="名称 / 来源 ID / 类型" name="rule_query" value={query} onChange={setQuery} />
        <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
        <SelectFilter label="规则类型" value={ruleType} onChange={setRuleType} options={uniqueOptions(rules.map((rule) => rule.rule_type))} />
      </QueryBar>
      <div className="record-actions"><button type="button" className="primary" onClick={openCreate}>新增规则</button></div>
      <Panel title="清洗规则" icon={<ListChecks />} meta={`查询命中 ${filtered.length} 条`}><TransformRuleList rules={filtered} onDetail={(rule) => { void openDetail(rule.id) }} /></Panel>
      {draft && (
        <Modal title={draft.id ? '清洗规则详情与编辑' : '新增清洗规则'} onClose={() => { if (!saving && !testing) setDraft(null) }}>
          {draft.hasSecret && <div className="result-banner error" role="alert">该规则包含已隐藏的密钥。当前后端仅支持完整更新，不能安全回写脱敏配置；请使用专用保留密钥编辑接口。</div>}
          <form className="excel-upload-form" onSubmit={saveDraft}>
            <label>来源
              <select value={draft.sourceID} disabled={draft.hasSecret || saving} required onChange={(event) => setDraft({ ...draft, sourceID: event.currentTarget.value })}>
                <option value="">选择数据源</option>
                {sources.map((source) => <option value={source.id} key={source.id}>#{source.id} {source.name}</option>)}
              </select>
            </label>
            <Field label="规则名称" name="rule_name" value={draft.name} required onChange={(name) => setDraft({ ...draft, name })} />
            <label>规则类型
              <select value={draft.ruleType} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, ruleType: event.currentTarget.value })}>
                {['mapping', 'http_enrich', 'db_enrich', 'script', 'validator'].map((type) => <option key={type} value={type}>{type}</option>)}
              </select>
            </label>
            <Field label="执行顺序" name="rule_order" type="number" value={draft.orderIndex} required onChange={(orderIndex) => setDraft({ ...draft, orderIndex })} />
            <label className="checkbox-label"><input type="checkbox" checked={draft.enabled} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, enabled: event.currentTarget.checked })} />启用规则</label>
            <label>规则配置 JSON<textarea value={draft.configJSON} disabled={draft.hasSecret || saving} rows={12} onChange={(event) => setDraft({ ...draft, configJSON: event.currentTarget.value })} /></label>
            <div className="excel-form-actions"><button className="primary" type="submit" disabled={draft.hasSecret || saving}>{saving ? '保存中…' : '保存规则'}</button></div>
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
  const [view, setView] = useState<'legacy' | 'clean'>('legacy')
  return (
    <div className="view-stack">
      <div className="tab-actions" role="tablist" aria-label="处理结果数据视图">
        <button type="button" role="tab" aria-selected={view === 'legacy'} className={view === 'legacy' ? 'active' : ''} onClick={() => setView('legacy')}>旧处理结果</button>
        <button type="button" role="tab" aria-selected={view === 'clean'} className={view === 'clean' ? 'active' : ''} onClick={() => setView('clean')}>清洗记录</button>
      </div>
      {view === 'legacy' ? <LegacyProcessedQueryPanel client={client} /> : <CleanRecordsQueryPanel client={client} />}
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

function CleanRecordsQueryPanel({ client }: { client: ApiClient }) {
  const [sourceID, setSourceID] = useState('')
  const [tableName, setTableName] = useState('')
  const [businessKey, setBusinessKey] = useState('')
  const [status, setStatus] = useState('')
  const [minQuality, setMinQuality] = useState('')
  const [maxQuality, setMaxQuality] = useState('')
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
    <div className="view-stack">
      <form className="query-bar" onSubmit={submit}>
        <div className="query-fields">
          <Field label="来源 ID" name="clean_source_id" type="number" value={sourceID} onChange={setSourceID} />
          <Field label="逻辑表名" name="clean_table_name" value={tableName} onChange={setTableName} />
          <Field label="业务键" name="clean_business_key" value={businessKey} onChange={setBusinessKey} />
          <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'ready', label: '待推送' }, { value: 'invalid', label: '无效' }, { value: 'delivered', label: '已交付' }]} />
          <Field label="最低质量分" name="clean_min_quality" type="number" value={minQuality} onChange={setMinQuality} />
          <Field label="最高质量分" name="clean_max_quality" type="number" value={maxQuality} onChange={setMaxQuality} />
          <Field label="开始时间" name="clean_from" type="datetime-local" value={createdFrom} onChange={setCreatedFrom} />
          <Field label="结束时间" name="clean_to" type="datetime-local" value={createdTo} onChange={setCreatedTo} />
        </div>
        <button type="submit" disabled={loading}>{loading ? '查询中…' : '查询'}</button>
      </form>
      {error && <div className="result-banner error" role="alert">{error} 已保留最近一次成功数据。</div>}
      <Panel title="清洗记录" icon={<CheckCircle2 />} meta={loading && !recordsPage ? '正在加载…' : `共 ${recordsPage?.total ?? 0} 条 / 平均质量 ${recordsPage?.averageQuality.toFixed(1) ?? '-'}`}>
        <CleanRecordList records={records} />
        <div className="record-actions raw-record-pagination" role="status" aria-live="polite">
          <span>第 {recordsPage?.page ?? page} / {Math.max(totalPages, 1)} 页</span>
          <button type="button" onClick={() => setPage((current) => Math.max(1, current - 1))} disabled={loading || page <= 1}>上一页</button>
          <button type="button" onClick={() => setPage((current) => current + 1)} disabled={loading || totalPages === 0 || page >= totalPages}>下一页</button>
        </div>
      </Panel>
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
  const [showBackfill, setShowBackfill] = useState(false)
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

  function openBackfill() {
    invalidateBackfillPreview()
    setShowBackfill(true)
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
              <button className="primary" type="button" onClick={openBackfill} disabled={loading}>发起补拉</button>
              <button type="button" onClick={openManualRun} disabled={loading}>运行计划任务</button>
            </div>
          </>
        )}
      </Panel>

      {confirmed && (
        <Panel title="最近写入结果" icon={<CheckCircle2 />} meta={`${youzanDistributionTimeFilterLabel(confirmed.time_filter)} / ${confirmed.start_time} ~ ${confirmed.end_time}`}>
          <YouzanDistributionBackfillResultView title="写入结果" result={confirmed} />
        </Panel>
      )}

      {showBackfill && (
        <Modal title={confirmingBackfill ? '确认写入有赞分销订单' : '补拉有赞分销订单'} focusKey={confirmingBackfill ? 'confirm' : 'form'} closeDisabled={loading || writingBackfill} onClose={() => { if (!loading && !writingBackfill) setShowBackfill(false) }}>
          {confirmingBackfill && preview ? <div className="view-stack"><p>确认写入 {preview.writable_count} 条有赞分销订单？系统会按 tid 判重，已有订单不会覆盖。</p><div className="excel-form-actions"><button type="button" disabled={loading || writingBackfill} onClick={() => setConfirmingBackfill(false)}>返回预览</button><button className="primary" type="button" disabled={loading || writingBackfill} onClick={() => void confirmBackfill()}>{writingBackfill ? '写入中…' : '确认写入'}</button></div></div> : <><form className="youzan-backfill-form" onSubmit={submit}>
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
          <p className="backfill-note">当前按{youzanDistributionTimeFilterLabel(timeFilter)}筛选。预览会真实拉取、解密并判重，但不写数据库；确认后重新拉取相同筛选方式和时间范围并写入，已有 tid 不覆盖。</p>
          {preview && <YouzanDistributionBackfillResultView title="预览结果" result={preview} />}
          {confirmed && <YouzanDistributionBackfillResultView title="写入结果" result={confirmed} />}</>}
        </Modal>
      )}

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

function DestinationsQueryPage({ client, destinations, onRefresh }: { client: ApiClient; destinations: DestinationDefinition[]; onRefresh: () => Promise<void> }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [destinationType, setDestinationType] = useState('all')
  const [draft, setDraft] = useState<DestinationDraft | null>(null)
  const [saving, setSaving] = useState(false)
  const [testingID, setTestingID] = useState<number | null>(null)
  const [pendingTest, setPendingTest] = useState<DestinationDefinition | null>(null)
  const [message, setMessage] = useState('')
  const filtered = destinations.filter((destination) => includesQuery([destination.id, destination.name, destination.code, destination.config_json], query)
    && (status === 'all' || (status === 'enabled' ? destination.enabled : !destination.enabled))
    && (destinationType === 'all' || destination.destination_type === destinationType))

  async function openDetail(id: number) {
    setMessage('')
    const response = await client(`/v1/destinations/${id}`, { method: 'GET', showResult: false, silentLoading: true })
    const destination = response.ok ? readObject<DestinationDefinition>(response, 'destination') : null
    if (!destination) { setMessage(response.error?.message || '推送目标详情暂时不可用。'); return }
    setDraft(destinationDraftFrom(destination))
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!draft || saving || draft.hasSecret) return
    if (!draft.name.trim() || !draft.code.trim()) { setMessage('请填写目标名称和编码。'); return }
    try { JSON.parse(draft.configJSON) } catch { setMessage('配置必须是有效 JSON。'); return }
    setSaving(true)
    const response = await client(draft.id ? `/v1/destinations/${draft.id}` : '/v1/destinations', { method: draft.id ? 'PUT' : 'POST', showResult: false, silentLoading: true, body: { name: draft.name.trim(), code: draft.code.trim(), destination_type: draft.destinationType, config_json: draft.configJSON, enabled: draft.enabled } })
    setSaving(false)
    if (!response.ok) { setMessage(response.error?.message || '推送目标保存未完成。'); return }
    setDraft(null)
    setMessage('推送目标已保存。')
    await onRefresh()
  }

  async function test(destination: DestinationDefinition) {
    if (testingID !== null) return
    setTestingID(destination.id)
    const response = await client(`/v1/destinations/${destination.id}/test`, { method: 'POST', showResult: false, silentLoading: true })
    setTestingID(null)
    setMessage(response.ok ? '连通性测试通过，未推送业务记录。' : response.error?.message || '连通性测试未完成。')
  }
  return (
    <div className="view-stack">
      {message && <div className="result-banner" role="status">{message}</div>}
      <QueryBar count={filtered.length} total={destinations.length}>
        <Field label="名称 / 编码" name="destination_query" value={query} onChange={setQuery} />
        <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
        <SelectFilter label="类型" value={destinationType} onChange={setDestinationType} options={uniqueOptions(destinations.map((destination) => destination.destination_type))} />
      </QueryBar>
      <div className="record-actions"><button type="button" className="primary" onClick={() => setDraft({ id: null, name: '', code: '', destinationType: 'http', configJSON: '{\n  "url": "",\n  "method": "POST"\n}', enabled: true, hasSecret: false })}>新增目标</button></div>
      <Panel title="推送目标" icon={<Send />} meta={`查询命中 ${filtered.length} 条`}><DestinationList destinations={filtered} testingID={testingID} onDetail={(item) => { void openDetail(item.id) }} onTest={setPendingTest} /></Panel>
      {draft && <Modal title={draft.id ? '推送目标详情与编辑' : '新增推送目标'} onClose={() => { if (!saving) setDraft(null) }}>
        {draft.hasSecret && <div className="result-banner error" role="alert">该目标包含已隐藏密钥，当前仅可查看与测试；完整更新会覆盖真实密钥。</div>}
        <form className="excel-upload-form" onSubmit={save}>
          <Field label="目标名称" name="destination_name" value={draft.name} required onChange={(name) => setDraft({ ...draft, name })} />
          <Field label="目标编码" name="destination_code" value={draft.code} required onChange={(code) => setDraft({ ...draft, code })} />
          <label>目标类型<select value={draft.destinationType} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, destinationType: event.currentTarget.value })}><option value="http">http</option><option value="soap">soap</option></select></label>
          <label className="checkbox-label"><input type="checkbox" checked={draft.enabled} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, enabled: event.currentTarget.checked })} />启用目标</label>
          <label>配置 JSON<textarea rows={10} value={draft.configJSON} disabled={draft.hasSecret || saving} onChange={(event) => setDraft({ ...draft, configJSON: event.currentTarget.value })} /></label>
          <div className="excel-form-actions"><button className="primary" type="submit" disabled={draft.hasSecret || saving}>{saving ? '保存中…' : '保存目标'}</button></div>
        </form>
      </Modal>}
      {pendingTest && <Modal title="确认测试推送目标" onClose={() => { if (testingID === null) setPendingTest(null) }} footer={<><button type="button" disabled={testingID !== null} onClick={() => setPendingTest(null)}>取消</button><button className="primary" type="button" disabled={testingID !== null} onClick={() => { const target = pendingTest; setPendingTest(null); void test(target) }}>{testingID === pendingTest.id ? '测试中…' : '确认测试'}</button></>}><p>将向“{pendingTest.name}”配置的目标地址发起真实连通性请求，不会推送业务记录。确认继续？</p></Modal>}
    </div>
  )
}

function DeliveryTasksQueryPage({ client, tasks, sources, destinations, onRefresh }: { client: ApiClient; tasks: DeliveryTask[]; sources: SourceDefinition[]; destinations: DestinationDefinition[]; onRefresh: () => Promise<void> }) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [destinationID, setDestinationID] = useState('all')
  const [draft, setDraft] = useState<DeliveryTaskDraft | null>(null)
  const filtered = tasks.filter((task) => includesQuery([task.id, task.name, task.clean_table, task.trigger_type], query)
    && (status === 'all' || (status === 'enabled' ? task.enabled : !task.enabled))
    && (destinationID === 'all' || String(task.destination_id) === destinationID))
  const destinationOptions = destinations.map((destination) => ({ value: String(destination.id), label: destination.name || destination.code }))
  const [runningID, setRunningID] = useState<number | null>(null)
  const [pendingRun, setPendingRun] = useState<DeliveryTask | null>(null)
  const [loadingDetailID, setLoadingDetailID] = useState<number | null>(null)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')

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
  }

  async function run(task: DeliveryTask) {
    if (runningID !== null) return
    setRunningID(task.id)
    const response = await client(`/v1/delivery-tasks/${task.id}/run`, { method: 'POST', showResult: false, silentLoading: true })
    const result = response.ok ? readObject<{ total_count: number; success_count: number; failed_count: number; skipped_count: number }>(response, 'result') : null
    setRunningID(null)
    setMessage(result ? `执行完成：总计 ${result.total_count}，成功 ${result.success_count}，失败 ${result.failed_count}，跳过 ${result.skipped_count}。` : response.error?.message || '推送任务未完成。')
    if (response.ok) await onRefresh()
  }
  return (
    <div className="view-stack">
      {message && <div className="result-banner" role="status">{message}</div>}
      <QueryBar count={filtered.length} total={tasks.length}>
        <Field label="名称 / 表 / 触发方式" name="task_query" value={query} onChange={setQuery} />
        <SelectFilter label="状态" value={status} onChange={setStatus} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '停用' }]} />
        <SelectFilter label="推送目标" value={destinationID} onChange={setDestinationID} options={destinationOptions} />
      </QueryBar>
      <div className="record-actions"><button type="button" className="primary" onClick={openCreate}>新增推送任务</button></div>
      <Panel title="推送任务" icon={<ArrowUpFromLine />} meta={`查询命中 ${filtered.length} 条`}><DeliveryTaskList tasks={filtered} runningID={runningID} loadingDetailID={loadingDetailID} destinations={destinations} onDetail={(task) => { void openDetail(task.id) }} onRun={setPendingRun} /></Panel>
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
  onSave: (config: OrderPushSkipConfig) => void
  onToggle: (target: ToggleTarget, enabled: boolean) => void
}) {
  return (
    <div className="view-stack">
      {coreMethod && <Panel title="商场推送方法" icon={<Send />} meta="当前推送能力"><CoreMethodList methods={[coreMethod]} onToggle={onToggle} /></Panel>}
      <Panel title="订单少推送配置" icon={<ListChecks />} meta="按具体目标独立配置"><OrderPushSkipConfigForm config={config} targets={targets} onSave={onSave} /></Panel>
    </div>
  )
}

function OrderPushSkipConfigForm({ config, targets, onSave }: { config: OrderPushSkipConfig; targets: OrderPushTargetOption[]; onSave: (config: OrderPushSkipConfig) => void }) {
  const [draft, setDraft] = useState(() => targets.map((target) => orderPushTargetConfig(config, target.code)))
  const [error, setError] = useState('')
  const enabledCount = draft.filter((target) => target.cycle > 0 && target.skip > 0).length

  useEffect(() => setDraft(targets.map((target) => orderPushTargetConfig(config, target.code))), [config, targets])

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (draft.some((item) => !Number.isInteger(item.cycle) || !Number.isInteger(item.skip) || item.cycle < 0 || item.skip < 0 || (item.cycle === 0 && item.skip !== 0) || (item.cycle > 0 && item.skip >= item.cycle))) {
      setError('循环和少推数量必须是非负整数；启用少推时，少推单数必须小于循环总单数。')
      return
    }
    setError('')
    onSave({ targets: draft })
  }

  return (
    <form className="push-skip-form" onSubmit={submit}>
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
      <button className="primary" type="submit">保存配置</button>
    </form>
  )
}

function ExcelMatchView({
  section,
  token,
  loading,
  setLoading,
  setResult,
  onNavigateToJobs,
}: {
  section: 'jobs' | 'schemes' | 'write'
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
  const [jobQuery, setJobQuery] = useState('')
  const [jobStatus, setJobStatus] = useState('all')
  const [jobOperation, setJobOperation] = useState('all')
  const focusedJobID = job?.id
  const filteredJobHistory = useMemo(() => jobHistory.filter((item) => includesQuery([item.id, item.source_file_name, item.error_message], jobQuery)
    && (jobStatus === 'all' || item.status === jobStatus)
    && (jobOperation === 'all' || excelJobOperation(item) === jobOperation)), [jobHistory, jobOperation, jobQuery, jobStatus])
  const pendingSchemeNameConflict = pendingSchemeSave
    ? (pendingSchemeSave.operation === 'export_match' ? exportSchemes : importSchemes)
      .find((scheme) => scheme.name === pendingSchemeSave.name.trim()) ?? null
    : null

  const applyJobResult = useCallback((result: ApiResult, options: { track?: boolean } = {}) => {
    const nextJob = readObject<ExcelMatchJob>(result, 'job')
    if (nextJob) {
      setJob(nextJob)
      setJobID(String(nextJob.id))
      setJobHistory((current) => upsertExcelJobHistory(current, nextJob))
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

  const loadExcelModels = useCallback(async () => {
    setExcelModelsLoading(true)
    setExcelModelsError('')
    try {
      const response = await fetch(apiURL('/v1/excel-match-jobs/models'), {
        method: 'GET',
        headers: token ? { token } : undefined,
      })
      const data = await response.json().catch(() => ({}))
      if (!response.ok || !isSuccessPayload(data)) {
        throw new Error(readMessage(data) || '查询模型与字段目录失败')
      }
      const value = readDataField(data, 'models')
      setExcelModels(Array.isArray(value) ? (value as ExcelMatchModel[]) : [])
    } catch (error) {
      setExcelModelsError(error instanceof Error ? error.message : String(error))
    } finally {
      setExcelModelsLoading(false)
    }
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

  const loadJobHistory = useCallback(async () => {
    try {
      const response = await fetch(apiURL('/v1/excel-match-jobs?limit=30'), {
        method: 'GET',
        headers: token ? { token } : undefined,
      })
      const data = await response.json().catch(() => ({}))
      if (!response.ok || !isSuccessPayload(data)) {
        throw new Error(readMessage(data) || '查询 Excel 任务历史失败')
      }
      const value = readDataField(data, 'jobs')
      setJobHistory(Array.isArray(value) ? (value as ExcelMatchJob[]) : [])
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    }
  }, [setResult, token])

  useEffect(() => {
    if (!token) return
    if (section === 'jobs') void loadJobHistory()
    if (section === 'schemes' || section === 'write') void loadSchemes()
  }, [loadJobHistory, loadSchemes, section, token])

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
      const response = await fetch(apiURL(`/v1/excel-match-jobs/${id}`), {
        method: 'GET',
        headers: token ? { token } : undefined,
        signal: options.signal,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      if (!options.silent) setResult(nextResult)
      if (nextResult.ok) {
        applyJobResult(nextResult, { track: options.track })
        if (!options.silent) await loadJobHistory()
        return readObject<ExcelMatchJob>(nextResult, 'job')
      }
      if (options.silent) {
        setAutoRefreshText(`自动刷新失败：${readMessage(data) || response.status}`)
      }
    } catch (error) {
      if (options.signal?.aborted) return null
      if (!options.silent) {
        setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
      } else {
        setAutoRefreshText(`自动刷新失败：${error instanceof Error ? error.message : String(error)}`)
      }
    } finally {
      if (!options.silent) setLoading(false)
    }
    return null
  }, [applyJobResult, loadJobHistory, setLoading, setResult, token])

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
      const response = await fetch(apiURL('/v1/excel-match-jobs/schemes'), {
        method: 'POST',
        headers: token ? { token, 'Content-Type': 'application/json' } : { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), operation, config }),
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (nextResult.ok) {
        const savedScheme = readObjectFromData<ExcelMatchScheme>(data, 'scheme')
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
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
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
      const response = await fetch(apiURL(excelMatchSchemePath(scheme.id)), {
        method: 'DELETE',
        headers: token ? { token } : undefined,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      setResult(nextResult)
      if (!nextResult.ok) return

      if (scheme.operation === 'export_match' && selectedExportSchemeID === String(scheme.id)) {
        applyExportScheme('')
      }
      if (scheme.operation === 'import_update' && selectedImportSchemeID === String(scheme.id)) {
        applyImportScheme('')
      }
      await loadSchemes()
      setPendingSchemeDelete(null)
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
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
      const response = await fetch(apiURL('/v1/excel-match-jobs'), {
        method: 'POST',
        headers: token ? { token } : undefined,
        body: payload,
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      if (nextResult.ok) {
        showCreatedJob(nextResult)
      } else {
        setResult(nextResult)
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

  async function createExcelWriteJob(slot: 'import' | 'clear', file: File, config: unknown) {
    setLoading(true)
    try {
      const uploadId = await ensureExcelUpload(slot, file)
      const response = await fetch(apiURL('/v1/excel-match-jobs'), {
        method: 'POST',
        headers: token ? { token } : undefined,
        body: buildConfigPayload(uploadId, config),
      })
      const data = await response.json().catch(() => ({}))
      const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
      if (nextResult.ok) showCreatedJob(nextResult)
      else setResult(nextResult)
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
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
    onNavigateToJobs()
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

  return (
    <div className="view-stack">
      {section === 'jobs' && <>
        <section className="overview-grid">
          <Metric label="历史任务" value={jobHistory.length} />
          <Metric label="当前任务" value={job?.id ?? '-'} />
          <Metric label="任务状态" value={job ? excelJobStatusLabel(job.status) : '-'} />
          <Metric label="已处理行" value={job ? `${job.processed_rows}/${job.total_rows}` : '-'} />
          <Metric label="自动跟踪" value={trackingJobID ? `#${trackingJobID}` : '-'} />
        </section>
        <QueryBar count={filteredJobHistory.length} total={jobHistory.length}>
          <Field label="任务 ID / 文件名 / 错误" name="excel_job_query" value={jobQuery} onChange={setJobQuery} />
          <SelectFilter label="状态" value={jobStatus} onChange={setJobStatus} options={uniqueOptions(jobHistory.map((item) => item.status))} />
          <SelectFilter label="操作" value={jobOperation} onChange={setJobOperation} options={Array.from(new Set(jobHistory.map(excelJobOperation).filter(Boolean))).map((value) => ({ value, label: excelJobOperationLabel(value) }))} />
        </QueryBar>
        <Panel title="Excel 任务" icon={<ListChecks />} meta="最多读取最近 30 条，可查询、查看和下载">
          <ExcelJobHistoryTable
            jobs={filteredJobHistory}
            loading={loading}
            downloadingJobID={downloadingJobID}
            onDownload={downloadJob}
            onView={(id) => void refreshJobByID(id)}
          />
        </Panel>
        <Panel title="按任务 ID 定位" icon={<Download />} meta="直接查询历史任务并下载结果">
          <button type="button" onClick={() => openExcelDialog('query')}>打开任务定位</button>
        </Panel>

        {job && (
          <div
            ref={jobExecutionRef}
            className="excel-job-focus-target"
            tabIndex={-1}
            role="region"
            aria-label={`Excel 任务 ${job.id} 执行详情`}
          >
            <Panel title={`Excel 任务 #${job.id}`} icon={<FileJson />} meta={job.source_file_name || 'job detail'}>
              {autoRefreshText && <p className="excel-mode-note">{autoRefreshText}</p>}
              <div className="excel-job-detail">
                <Metric label="源文件" value={job.source_file_name || '-'} />
                <Metric label="任务类型" value={excelJobOperationLabel(excelJobOperation(job))} />
                <Metric label="筛选/命中" value={job.filtered_rows || '-'} />
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
              {job.error_message && <div className="login-error">{job.error_message}</div>}
              <section className="content-grid two">
                <ReadonlyJSON value={job.config_json || '{}'} />
                <ExcelJobLogList logs={jobLogs} />
              </section>
            </Panel>
          </div>
        )}
      </>}

      {section === 'schemes' && <>
        <section className="overview-grid">
          <Metric label="导出方案" value={exportSchemes.length} />
          <Metric label="当前步骤" value={exportSteps.length} />
          <Metric label="最大步骤" value="20" />
          <Metric label="筛选规则" value="可选" />
        </section>
        <Panel title="自定义匹配流程" icon={<Upload />} meta="每一步均可指定任意数据表与字段，后一步可使用前一步追加的列">
          <div className="excel-action-grid compact-actions">
          <button type="button" className="excel-action-card" onClick={() => openExcelDialog('export')}>
            <Upload aria-hidden="true" />
              <span>新建或编辑匹配方案</span>
              <small>配置步骤顺序、预览匹配并创建导出任务</small>
          </button>
          </div>
        </Panel>
        <Panel title="已保存导出方案" icon={<ListChecks />} meta={`${exportSchemes.length} 个方案`}>
          <ExcelSchemeList schemes={exportSchemes} deletingSchemeID={deletingSchemeID} onDelete={setPendingSchemeDelete} onOpen={(id) => { applyExportScheme(String(id)); openExcelDialog('export') }} />
        </Panel>
      </>}

      {section === 'write' && <>
        <section className="overview-grid">
          <Metric label="导入方案" value={importSchemes.length} />
          <Metric label="默认模式" value="只预检" />
          <Metric label="写入保护" value="不覆盖" />
          <Metric label="清空保护" value="需确认" />
        </section>
        <Panel title="数据库回写" icon={<Database />} meta="匹配导入与退回未匹配分开执行">
          <div className="excel-action-grid compact-actions">
          <button type="button" className="excel-action-card" onClick={() => openExcelDialog('import')}>
            <Database aria-hidden="true" />
            <span>匹配导入</span>
            <small>默认预检，确认后写入空的 matched_docno</small>
          </button>
          <button type="button" className="excel-action-card" onClick={() => openExcelDialog('clear')}>
            <RefreshCcw aria-hidden="true" />
            <span>退回未匹配</span>
            <small>按 Excel 匹配范围清空 matched_docno</small>
          </button>
        </div>
      </Panel>
        <Panel title="已保存导入方案" icon={<ListChecks />} meta={`${importSchemes.length} 个方案`}>
          <ExcelSchemeList schemes={importSchemes} deletingSchemeID={deletingSchemeID} onDelete={setPendingSchemeDelete} onOpen={(id) => { applyImportScheme(String(id)); openExcelDialog('import') }} />
        </Panel>
      </>}

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
      <table className="data-table">
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
  onView,
  onDownload,
}: {
  jobs: ExcelMatchJob[]
  loading: boolean
  downloadingJobID: number | null
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
            <tr key={item.id}>
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

function DeliveryLogList({ logs, onSelectLog, onRetryLog, retryingLogID }: { logs: DeliveryLog[]; onSelectLog?: (log: DeliveryLog) => void; onRetryLog?: (log: DeliveryLog) => void | Promise<void>; retryingLogID?: number | null }) {
  const [storeFilter, setStoreFilter] = useState('all')
  const matchedLogs = useMemo(
	    () => logs.map((log) => ({ log, store: matchDeliveryStore(log) ?? otherDeliveryStore })),
    [logs],
  )
  const visibleLogs = storeFilter === 'all' ? matchedLogs : matchedLogs.filter((item) => item.store.key === storeFilter)
  const groupedLogs = useMemo(() => {
    return [...deliveryStores, otherDeliveryStore]
      .map((store) => ({
        store,
        logs: visibleLogs.filter((item) => item.store.key === store.key).map((item) => item.log),
      }))
      .filter((group) => group.logs.length > 0)
  }, [visibleLogs])

  if (matchedLogs.length === 0) return <EmptyState text="暂无推送日志。" />
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
			<option value={otherDeliveryStore.key}>{otherDeliveryStore.name}</option>
          </select>
        </label>
        <span>共 {matchedLogs.length} 条，当前显示 {visibleLogs.length} 条</span>
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
                        {!log.success && onRetryLog && <button type="button" disabled={retryingLogID !== null} onClick={() => void onRetryLog(log)}>{retryingLogID === log.id ? '重试中…' : '重试'}</button>}
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

function matchDeliveryStore(log: DeliveryLog) {
  const text = [
    log.destination_code,
    log.destination_name,
    log.source_code,
    log.business_key,
    log.response_summary,
    log.error_message,
  ].join(' ').toLowerCase()

  return deliveryStores.find((store) => store.aliases.some((alias) => text.includes(alias.toLowerCase()))) ?? null
}

function deliveryLogPreview(log: DeliveryLog) {
  const response = compactText(log.response_summary)
  if (response) return response
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
    <div className="record-list">
      {sources.map((source) => (
        <article className="record-row" key={source.id}>
          <div>
            <strong>{source.name}</strong>
            <span>{source.code} / {source.source_type} / {source.auth_type || 'none'}</span>
	            {source.has_secret && <small>配置包含已隐藏的密钥；编辑时仅可重新填写。</small>}
          </div>
          <div className="record-actions">
            <StatusPill label={source.enabled ? '启用' : '停用'} />
            <button type="button" disabled={testingID !== null || fetchingID !== null} onClick={() => onDetail(source)}>详情</button>
            <button type="button" disabled={testingID === source.id || fetchingID === source.id || !source.enabled} onClick={() => { void test(source.id) }}>{testingID === source.id ? '测试中…' : '测试连接'}</button>
            <button type="button" disabled={testingID === source.id || fetchingID === source.id || !source.enabled || source.source_type === 'webhook'} onClick={() => { void fetch(source.id) }}>{fetchingID === source.id ? '拉取中…' : '手动拉取'}</button>
          </div>
          {messageByID[source.id] && <small className="source-operation-message" role="status" aria-live="polite">{messageByID[source.id]}</small>}
        </article>
      ))}
    </div>
  )
}

function TransformRuleList({ rules, onDetail }: { rules: TransformRule[]; onDetail: (rule: TransformRule) => void }) {
  if (rules.length === 0) return <EmptyState text="暂无处理规则。" />
  return (
    <div className="record-list">
      {rules.map((rule) => (
        <article className="record-row" key={rule.id}>
          <div>
            <strong>{rule.name}</strong>
            <span>{rule.rule_type} / source #{rule.source_id} / 顺序 {rule.order_index}</span>
            {rule.has_secret && <small>配置包含已隐藏的密钥；当前仅可查看。</small>}
          </div>
          <div className="record-actions"><StatusPill label={rule.enabled ? '启用' : '停用'} /><button type="button" onClick={() => onDetail(rule)}>详情</button></div>
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

function CleanRecordList({ records }: { records: CleanRecord[] }) {
  if (records.length === 0) return <EmptyState text="暂无清洗记录。" />
  return (
    <div className="record-list">
      {records.map((record) => (
        <article className="record-row" key={record.id}>
          <div>
            <strong>{record.business_key || `#${record.id}`}</strong>
            <span>来源 #{record.source_id} / 表 {record.table_name || '-'} / 原始记录 #{record.raw_record_id}</span>
          </div>
          <div className="record-actions"><span>质量 {record.quality_score}</span><StatusPill label={cleanRecordStatusLabel(record.status)} /><small>{formatUnixTime(record.created_at)}</small></div>
        </article>
      ))}
    </div>
  )
}

function DestinationList({ destinations, testingID, onDetail, onTest }: { destinations: DestinationDefinition[]; testingID: number | null; onDetail: (destination: DestinationDefinition) => void; onTest: (destination: DestinationDefinition) => void }) {
  if (destinations.length === 0) return <EmptyState text="暂无推送目标。" />
  return (
    <div className="record-list">
      {destinations.map((destination) => (
        <article className="record-row" key={destination.id}>
          <div>
            <strong>{destination.name}</strong>
            <span>{destination.code} / {destination.destination_type}</span>
            {destination.has_secret && <small>配置包含已隐藏密钥；当前仅可查看和测试。</small>}
          </div>
          <div className="record-actions"><StatusPill label={destination.enabled ? '启用' : '停用'} /><button type="button" onClick={() => onDetail(destination)}>详情</button><button type="button" disabled={testingID !== null} onClick={() => onTest(destination)}>{testingID === destination.id ? '测试中…' : '测试连接'}</button></div>
        </article>
      ))}
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
    <div className="record-list">
      {tasks.map((task) => (
        <article className="record-row" key={task.id}>
          <div>
            <strong>{task.name}</strong>
            <span>{`${task.clean_table} -> ${deliveryTaskDestinationLabel(task, destinations)} / ${deliveryTaskTriggerLabel(task.trigger_type)}`}</span>
          </div>
          <div className="record-actions">
            <StatusPill label={task.enabled ? '启用' : '停用'} />
            <button type="button" disabled={loadingDetailID !== null || runningID !== null} onClick={() => onDetail(task)}>{loadingDetailID === task.id ? '加载中…' : '详情'}</button>
            <button type="button" disabled={!task.enabled || runningID !== null || loadingDetailID !== null} onClick={() => onRun(task)}>{runningID === task.id ? '推送中…' : '手动运行'}</button>
          </div>
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
          <ReadonlyJSON value={redactMonitoringJSON({ input: parseJsonText(run.input_json), output: parseJsonText(run.output_json), error: run.error_message })} />
        </details>
      ))}
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

function QueryBar({ count, total, children }: { count: number; total: number; children: ReactNode }) {
  return (
    <section className="query-bar" aria-label="查询条件">
      <div className="query-fields">{children}</div>
      <div className="query-count"><strong>{count}</strong><span>/ {total} 条</span></div>
    </section>
  )
}

function SelectFilter({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: Array<{ value: string; label: string }> }) {
  return (
    <label>
      {label}
      <select value={value} onChange={(event) => onChange(event.currentTarget.value)}>
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
  return <span className="status-pill">{label}</span>
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
    if (source.has_secret) return protectedConfigUpdateFailure()
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
    if (rule.has_secret) return protectedConfigUpdateFailure()
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
    if (destination.has_secret) return protectedConfigUpdateFailure()
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

function protectedConfigUpdateFailure(): ApiResult {
  const message = '该配置含有已隐藏的密钥，不能通过完整更新覆盖；请使用保留密钥的专用编辑操作。'
  return { ok: false, status: 422, data: { message }, error: { kind: 'client', message } }
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

function loginFailureMessage(status: number) {
  if (status === 401) return '账号或密码不正确，请重试。'
  if (status === 429) return '登录尝试过于频繁，请稍后再试。'
  if (status >= 500) return '登录服务暂时不可用，请稍后再试。'
  return '登录请求未完成，请检查账号和密码后重试。'
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
  const config = parseMaybeJson(job.config_json)
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

function upsertExcelJobHistory(jobs: ExcelMatchJob[], nextJob: ExcelMatchJob) {
  const index = jobs.findIndex((item) => item.id === nextJob.id)
  if (index === -1) return [nextJob, ...jobs].slice(0, 30)
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

function groupBy<T>(items: T[], keyFn: (item: T) => string) {
  return items.reduce<Record<string, T[]>>((groups, item) => {
    const key = keyFn(item)
    groups[key] = groups[key] ?? []
    groups[key].push(item)
    return groups
  }, {})
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
