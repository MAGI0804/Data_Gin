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
  FileSpreadsheet,
  Inbox,
  ListChecks,
  LogOut,
  RefreshCcw,
  Search,
  ScrollText,
  Send,
  Settings2,
  Upload,
	Users,
  Wrench,
  X,
} from 'lucide-react'
import './App.css'
import { apiURL as buildApiURL } from './apiURL'
import { clearStoredToken, loadStoredSessionUser, loadStoredToken, saveStoredSessionUser, saveStoredToken, saveStoredTokenExpiry, storedTokenExpiresAt, tokenActorID, type StoredSessionUser } from './authStorage'
import { createApiClient, type ApiRequestOptions, type ClientResponse, type HTTPMethod } from './api/client'
import { verifySessionResponses, type SessionUser } from './api/auth'
import { parseDataStatisticsSummary, parseHealthSummary, parseMallWeatherMetricsSummary, redactMonitoringJSON, type DataStatisticsSummary, type HealthSummary, type MallWeatherMetricsSummary } from './monitoring'
import { MallWeatherPage, StoreInfoPage } from './MallWeatherPage'
import { AccessManagementPage } from './AccessManagementPage'
import { pipelineListPath } from './pipelineRun'
import { Brand } from './components/Brand'
import { ReportCenter } from './reportCenter/ReportCenter'
import type { ReportCenterSection } from './reportCenter/types'
import { AppShell, Dialog, WorkspaceHeader } from './ui'
import { RunOverviewPage } from './monitoringPages/RunOverviewPage/RunOverviewPage'
import { PipelineRunsPage } from './monitoringPages/PipelineRunsPage/PipelineRunsPage'
import { StepRunsPage } from './monitoringPages/StepRunsPage/StepRunsPage'
import { DeliveryLogsPage } from './monitoringPages/DeliveryLogsPage/DeliveryLogsPage'
import { parseDeliveryLog, parsePipelineRun } from './monitoringPages/contracts'
import type { DeliveryLog, PipelineRun } from './monitoringPages/types'
import { SourcesPage } from './configurationPages/SourcesPage/SourcesPage'
import type { SourceDefinition } from './configurationPages/types'
import { RulesPage } from './configurationPages/RulesPage/RulesPage'
import type { TransformRule } from './configurationPages/ruleContracts'
import { DestinationsPage } from './configurationPages/DestinationsPage/DestinationsPage'
import { DeliveryTasksPage } from './configurationPages/DeliveryTasksPage/DeliveryTasksPage'
import { PushPolicyPage } from './configurationPages/PushPolicyPage/PushPolicyPage'
import { MethodsPage } from './configurationPages/MethodsPage/MethodsPage'
import { ExcelMatchPage } from './excelPages/ExcelMatchPage'
import type { DestinationDefinition } from './configurationPages/types'
import { parseMallWeatherExportContentStatus, submitMallWeatherExportContentDownload } from './mallWeatherExport'
import { parseMonitoringPage } from './monitoringRecords'
import { ProcessedRecordsPage } from './dataPages/ProcessedRecordsPage'
import { RawRecordsPage } from './dataPages/RawRecordsPage'

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
type NavKey = 'overview' | 'runs' | 'delivery_logs' | 'step_runs' | 'store_info' | 'mall_weather' | 'access_management' | 'sources' | 'receive' | 'pull_records' | 'backfill' | 'youzan_distribution' | 'rules' | 'processed' | 'methods' | 'destinations' | 'tasks' | 'push_policy' | 'excel_jobs' | 'excel_schemes' | 'excel_write' | 'report_catalog' | 'report_configuration' | 'report_query' | 'report_exports'
type NavItem = { key: NavKey; label: string; description: string; icon: ReactNode }
type NavGroup = { label: string; items: NavItem[] }

type PipelineDefinition = {
  id: number
  name: string
  code: string
  enabled: boolean
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
    label: '报表中心',
    items: [
      { key: 'report_catalog', label: '报表目录', description: '报表定义与发布状态', icon: <FileSpreadsheet aria-hidden="true" /> },
      { key: 'report_configuration', label: '报表配置', description: '过程、形参与字段契约', icon: <Settings2 aria-hidden="true" /> },
      { key: 'report_query', label: '报表查询', description: '参数查询与结果预览', icon: <Search aria-hidden="true" /> },
      { key: 'report_exports', label: '导出中心', description: 'Excel 与结果清理状态', icon: <Download aria-hidden="true" /> },
    ],
  },
  {
	label: '账号与权限',
    items: [
	  { key: 'access_management', label: '账号与权限', description: '控制台账号、角色与审计', icon: <Users aria-hidden="true" /> },
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

function App() {
  const [token, setToken] = useState(() => loadStoredToken(window.localStorage))
  const tokenRef = useRef(token)
  const actorID = useMemo(() => tokenActorID(token), [token])
  const [sessionState, setSessionState] = useState<'checking' | 'authenticated' | 'anonymous'>(() => token ? 'checking' : 'anonymous')
  const authenticatedSessionRef = useRef(sessionState === 'authenticated')
  const [sessionUser, setSessionUser] = useState<SessionUser | null>(() => {
    const user = loadStoredSessionUser(window.localStorage)
    return user ? { ...user, phone: '', accountType: 'CONSOLE', status: 'ACTIVE', mallScopeMode: 'SELECTED', roles: [], permissions: [], mallIds: [] } : null
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
  const [pipelines, setPipelines] = useState<PipelineDefinition[]>([])
  const [sources, setSources] = useState<SourceDefinition[]>([])
  const [transformRules, setTransformRules] = useState<TransformRule[]>([])
  const [destinations, setDestinations] = useState<DestinationDefinition[]>([])
  const [deliveryLogs, setDeliveryLogs] = useState<DeliveryLog[]>([])
  const [overviewTotals, setOverviewTotals] = useState({ runs: null as number | null, deliveryLogs: null as number | null })
  const [monitoring, setMonitoring] = useState<MonitoringSnapshot>({ statistics: null, weather: null, health: null })
  const [monitoringStale, setMonitoringStale] = useState(false)
  const [legacyTasks, setLegacyTasks] = useState<LegacyTask[]>([])

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
        } else if (activeNav === 'youzan_distribution') {
          const legacyTaskResult = await get('/v1/legacy-tasks')
          if (!controller.signal.aborted && legacyTaskResult.ok) setLegacyTasks(readList<LegacyTask>(legacyTaskResult, 'tasks'))
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
    [activeNav, client, token],
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

  if (sessionState !== 'authenticated') return <LoginScreen onLogin={handleLogin} checking={sessionState === 'checking'} />

  const visibleNavGroups = navGroups.map((group) => ({
	...group,
	items: group.items.filter((item) => {
	  if (item.key === 'access_management') return Boolean(sessionUser?.permissions.some((permission) => permission.startsWith('system.')))
	  if (item.key === 'report_catalog') return Boolean(sessionUser?.permissions.includes('report.read'))
	  if (item.key === 'report_configuration') return Boolean(sessionUser?.permissions.includes('report.read') && sessionUser.permissions.includes('report.manage'))
	  if (item.key === 'report_query') return Boolean(sessionUser?.permissions.includes('report.read') && sessionUser.permissions.includes('report.execute'))
	  if (item.key === 'report_exports') return Boolean(sessionUser?.permissions.includes('report.read') && sessionUser.permissions.includes('report.export'))
	  const required = navPermission(item.key)
	  return !required || sessionUser?.permissions.includes(required) || sessionUser?.permissions.includes(required.replace(/\.read$/, '.manage'))
	}),
  })).filter((group) => group.items.length > 0)

  const reportSection = reportCenterSection(activeNav)
  const shellNavigation = <>
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
      {visibleNavGroups.map((group) => {
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
  </>

  return (
    <AppShell
      className={activeNav === 'mall_weather' || activeNav === 'store_info' ? 'mall-weather-shell' : undefined}
      navigation={shellNavigation}
      navigationClassName="ops-sidebar"
      navigationRef={mobileNavRef}
      navigationOpen={mobileNavOpen}
      onDismissNavigation={() => setMobileNavOpen(false)}
      flushWorkspace={Boolean(reportSection) || ['access_management', 'sources', 'receive', 'pull_records', 'rules', 'processed', 'methods', 'destinations', 'tasks', 'push_policy', 'overview', 'runs', 'delivery_logs', 'step_runs'].includes(activeNav)}
      workspaceClassName="ops-workspace"
      header={<ModuleHeader compact={Boolean(reportSection) || ['access_management', 'sources', 'receive', 'pull_records', 'rules', 'processed', 'methods', 'destinations', 'tasks', 'push_policy', 'overview', 'runs', 'delivery_logs', 'step_runs'].includes(activeNav)} activeNav={activeNav} loading={loading || refreshing} sessionUser={sessionUser} onOpenNavigation={openMobileNavigation} onRefresh={() => void refreshWorkspace(true)} onLogout={handleLogout} refreshing={refreshing} mobileNavTriggerRef={mobileNavTriggerRef} />}
      notices={<>{sessionValidationError && <div className="result-banner error" role="status" aria-live="polite">{sessionValidationError} <button type="button" onClick={() => setSessionValidationAttempt((attempt) => attempt + 1)}>重试校验</button></div>}{workspaceError && <div className="result-banner error" role="alert">{workspaceError} <button type="button" onClick={() => void refreshWorkspace(false)} disabled={refreshing}>重试</button></div>}</>}
      overlay={<ResultPanel result={result} onClose={() => setResult(null)} />}
    >
        {activeNav === 'overview' && <RunOverviewPage runs={runs} deliveryLogs={deliveryLogs} monitoring={monitoring} stale={monitoringStale} overviewTotals={overviewTotals} onLoadSteps={openStepRuns} />}
        {activeNav === 'runs' && <PipelineRunsPage client={client} pipelines={pipelines} onLoadSteps={openStepRuns} onPipelineRunCompleted={() => void refreshWorkspace(false)} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'delivery_logs' && <DeliveryLogsPage client={client} onRetryLog={retryDeliveryLog} />}
        {activeNav === 'step_runs' && <StepRunsPage client={client} focusRunID={stepRunFocusID} />}
        {activeNav === 'store_info' && <StoreInfoPage actorID={actorID} client={client} downloadFile={downloadFile} />}
        {activeNav === 'mall_weather' && <MallWeatherPage actorID={actorID} client={client} downloadFile={downloadFile} />}
        {reportSection && <ReportCenter client={client} permissions={sessionUser?.permissions ?? []} section={reportSection} onNavigate={(section) => navigate(reportCenterNavKey(section))} />}
        {activeNav === 'access_management' && <AccessManagementPage client={client} permissions={sessionUser?.permissions ?? []} />}
        {activeNav === 'sources' && <SourcesPage client={client} onFetchSource={fetchSource} onTestSource={testSource} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'methods' && <MethodsPage client={client} permissions={sessionUser?.permissions ?? []} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'receive' && <RawRecordsPage title="接口接收记录" origin="receive" client={client} onFetchSource={fetchSource} />}
        {activeNav === 'pull_records' && <RawRecordsPage title="数据拉取记录" origin="pull" client={client} onFetchSource={fetchSource} />}
        {activeNav === 'backfill' && <BojunBackfillPage loading={loading || refreshing} onPreview={previewBojunOrderBackfill} onConfirm={confirmBojunOrderBackfill} />}
        {activeNav === 'youzan_distribution' && <YouzanDistributionPage task={legacyTasks.find((item) => item.code === 'youzan_distribution_order_fetch')} loading={loading || refreshing} onPreview={previewYouzanDistributionBackfill} onConfirm={confirmYouzanDistributionBackfill} onRun={runLegacyTask} />}
        {activeNav === 'rules' && <RulesPage client={client} rules={transformRules} sources={sources} onRulesChange={setTransformRules} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'processed' && <ProcessedRecordsPage client={client} />}
        {activeNav === 'destinations' && <DestinationsPage client={client} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'tasks' && <DeliveryTasksPage client={client} canManage={Boolean(sessionUser?.permissions.includes('delivery.manage'))} sources={sources} destinations={destinations} onRefresh={() => refreshWorkspace(false)} refreshVersion={workspaceRefreshVersion} />}
        {activeNav === 'push_policy' && <PushPolicyPage client={client} canManage={Boolean(sessionUser?.permissions.includes('delivery.manage'))} refreshVersion={workspaceRefreshVersion} />}
        {(activeNav === 'excel_jobs' || activeNav === 'excel_schemes' || activeNav === 'excel_write') && <ExcelMatchPage section={activeNav === 'excel_jobs' ? 'jobs' : activeNav === 'excel_schemes' ? 'schemes' : 'write'} client={client} token={token} loading={loading} refreshVersion={workspaceRefreshVersion} setLoading={setLoading} setResult={setResult} onNavigateToJobs={() => navigate('excel_jobs')} />}
    </AppShell>
  )
}

function LoginScreen({ onLogin, checking }: { onLogin: (token: string) => void; checking: boolean }) {
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [sending, setSending] = useState(false)
  const [mode, setMode] = useState<'password' | 'phone' | 'reset'>('password')
  const [phone, setPhone] = useState('')
  const [countdown, setCountdown] = useState(0)

  useEffect(() => {
    if (countdown <= 0) return
    const timer = window.setTimeout(() => setCountdown((value) => Math.max(0, value - 1)), 1000)
    return () => window.clearTimeout(timer)
  }, [countdown])

  function switchMode(nextMode: 'password' | 'phone' | 'reset') {
    if (submitting || sending) return
    setMode(nextMode)
    setError('')
    setNotice('')
  }

  async function sendCode(purpose: 'LOGIN' | 'PASSWORD_RESET') {
    if (sending || countdown > 0) return
    if (!/^1[3-9]\d{9}$/.test(phone.trim())) {
      setError('请输入正确的中国大陆手机号。')
      return
    }
    setSending(true)
    setError('')
    setNotice('')
    try {
      const response = await fetch(apiURL('/auth/phone-codes'), {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone: phone.trim(), purpose }),
      })
      if (!response.ok) {
        setError(loginFailureMessage(response.status, 'code'))
        return
      }
      setCountdown(60)
      setNotice('若该手机号对应可用账号，验证码将发送至手机。')
    } catch {
      setError('无法连接认证服务，请检查网络后重试。')
    } finally {
      setSending(false)
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    const form = new FormData(event.currentTarget)
    try {
      const path = mode === 'password' ? '/auth/login/password' : mode === 'phone' ? '/auth/login/phone-code' : '/auth/password/reset'
      const body = mode === 'password'
        ? { account: formValue(form, 'account'), password: formValue(form, 'password') }
        : mode === 'phone'
          ? { phone: phone.trim(), code: formValue(form, 'code') }
          : { phone: phone.trim(), code: formValue(form, 'code'), password: formValue(form, 'newPassword') }
      const response = await fetch(apiURL(path), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const data: unknown = await response.json().catch(() => ({}))
      if (mode === 'reset' && response.ok) {
        setMode('password')
        setNotice('密码已重置，请使用新密码登录。')
        setCountdown(0)
        return
      }
      const token = readToken(data)
      if (!response.ok || !token) {
        setError(loginFailureMessage(response.status, mode))
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
          {mode !== 'reset' && <div className="login-tabs" role="tablist" aria-label="登录方式">
            <button type="button" role="tab" aria-selected={mode === 'password'} className={mode === 'password' ? 'active' : ''} onClick={() => switchMode('password')}>密码登录</button>
            <button type="button" role="tab" aria-selected={mode === 'phone'} className={mode === 'phone' ? 'active' : ''} onClick={() => switchMode('phone')}>验证码登录</button>
          </div>}
          {mode === 'password' && <>
            <Field label="账号或手机号" name="account" required autoComplete="username" />
            <Field label="密码" name="password" type="password" required autoComplete="current-password" />
            <button className="login-link" type="button" onClick={() => switchMode('reset')}>忘记密码</button>
          </>}
          {mode !== 'password' && <>
            <label>手机号
              <input name="phone" inputMode="numeric" autoComplete="tel" value={phone} onChange={(event) => setPhone(event.target.value)} required pattern="1[3-9][0-9]{9}" />
            </label>
            <div className="login-code-row">
              <Field label="短信验证码" name="code" required inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" />
              <button type="button" disabled={sending || countdown > 0} onClick={() => void sendCode(mode === 'phone' ? 'LOGIN' : 'PASSWORD_RESET')}>{sending ? '发送中…' : countdown > 0 ? `${countdown} 秒后重发` : '发送验证码'}</button>
            </div>
          </>}
          {mode === 'reset' && <>
            <Field label="新密码" name="newPassword" type="password" required minLength={10} maxLength={72} autoComplete="new-password" />
            <button className="login-link" type="button" onClick={() => switchMode('password')}>返回密码登录</button>
          </>}
          {notice && <div className="login-notice" role="status" aria-live="polite">{notice}</div>}
          {error && <div className="login-error" role="alert" aria-live="polite">{error}</div>}
          <button className="primary" type="submit" disabled={submitting || checking}>{submitting || checking ? '正在处理…' : mode === 'reset' ? '重置密码' : '登录'}</button>
        </form>
      </section>
    </main>
  )
}

function ModuleHeader({ activeNav, compact, loading, sessionUser, onOpenNavigation, onRefresh, onLogout, refreshing, mobileNavTriggerRef }: { activeNav: NavKey; compact: boolean; loading: boolean; sessionUser: SessionUser | null; onOpenNavigation: () => void; onRefresh: () => void; onLogout: () => void; refreshing: boolean; mobileNavTriggerRef: RefObject<HTMLButtonElement> }) {
  const titles: Record<NavKey, { title: string; subtitle: string }> = {
    overview: { title: '运行总览', subtitle: '只看当前运行与交付健康度，快速定位失败。' },
    runs: { title: '流水线运行', subtitle: '按状态、运行类型和 Trace ID 查询执行记录。' },
    delivery_logs: { title: '推送日志', subtitle: '按成功状态、门店和业务键查询外部交付结果。' },
    step_runs: { title: '步骤运行', subtitle: '选择一次流水线运行并查看每个步骤的输入输出。' },
    store_info: { title: '店铺信息', subtitle: '统一维护店铺资料、地址与天气服务坐标。' },
    mall_weather: { title: '商场天气', subtitle: '查看商场中心点实况、未来降水、小时趋势和气象预警。' },
		report_catalog: { title: '报表目录', subtitle: '查看报表定义、发布状态与当前版本。' },
		report_configuration: { title: '报表配置', subtitle: '维护 MySQL 中的过程、形参、字段和权限契约。' },
		report_query: { title: '报表查询', subtitle: '选择已发布报表，提交参数并预览结果。' },
		report_exports: { title: '导出中心', subtitle: '查看 Excel 生成、下载和结果清理状态。' },
	access_management: { title: '账号与权限', subtitle: '管理控制台账号、角色权限矩阵、开放 API 和变更审计。' },
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
    <WorkspaceHeader
      title={compact ? undefined : titles[activeNav].title}
      description={compact ? undefined : titles[activeNav].subtitle}
      context={compact ? activeNav === 'access_management' ? 'ACCESS CONTROL' : ['sources', 'rules', 'destinations'].includes(activeNav) ? 'DATA CONFIGURATION' : ['overview', 'runs', 'delivery_logs', 'step_runs'].includes(activeNav) ? 'OPERATIONS' : 'REPORT CENTER' : undefined}
      menuButtonRef={mobileNavTriggerRef}
      onOpenNavigation={onOpenNavigation}
      actions={<div className="workspace-session">
        {activeNav !== 'store_info' && <span className="workspace-date"><CalendarDays aria-hidden="true" />{new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'Asia/Shanghai' }).format(new Date())}</span>}
        {sessionUser && <span className="workspace-user">{sessionUser.nickname || sessionUser.account}</span>}
        <button className="workspace-refresh" type="button" onClick={onRefresh} disabled={refreshing}><RefreshCcw aria-hidden="true" />{refreshing ? '刷新中' : '刷新数据'}</button>
        <button className="workspace-logout" type="button" onClick={onLogout}><LogOut aria-hidden="true" />退出登录</button>
        <span className={loading ? 'workspace-health is-loading' : 'workspace-health'}><i aria-hidden="true" />{loading ? '数据加载中' : '系统正常'}</span>
      </div>}
    />
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
      {confirmingWrite && preview && <Dialog open title="确认写入伯俊订单" closeDisabled={loading || writing} onClose={() => { if (!loading && !writing) setConfirmingWrite(false) }} footer={<><button type="button" disabled={loading || writing} onClick={() => setConfirmingWrite(false)}>取消</button><button className="primary" type="button" disabled={loading || writing} onClick={() => void confirmWrite()}>{writing ? '写入中…' : '确认写入'}</button></>}><p>确认写入 {preview.writable_count} 条伯俊订单？系统会按 docno 判重，已有订单不会覆盖。</p></Dialog>}
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

      {confirmingBackfill && preview && <Dialog open title="确认写入有赞分销订单" closeDisabled={loading || writingBackfill} onClose={() => { if (!loading && !writingBackfill) setConfirmingBackfill(false) }} footer={<><button type="button" disabled={loading || writingBackfill} onClick={() => setConfirmingBackfill(false)}>返回预览</button><button className="primary" type="button" disabled={loading || writingBackfill} onClick={() => void confirmBackfill()}>{writingBackfill ? '写入中…' : '确认写入'}</button></>}><p>确认写入 {preview.writable_count} 条有赞分销订单？系统会按 tid 判重，已有订单不会覆盖。</p></Dialog>}

      {showManualRun && task && (
        <Dialog open title="运行有赞分销计划任务" onClose={() => { if (!runningManualTask) setShowManualRun(false) }}>
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
              {!manualRunResult && <div className="manual-task-actions">
                <button type="button" onClick={() => setManualRunPayload(null)} disabled={runningManualTask}>返回修改</button>
                <button className="primary" type="button" onClick={() => void confirmManualRun()} disabled={runningManualTask}>{runningManualTask ? '投递中…' : '确认投递任务'}</button>
              </div>}
              {manualRunError && <div className="result-banner error" role="alert">{manualRunError}</div>}
              {manualRunResult && <div className="result-banner" role="status">已投递：任务 ID {manualRunResult.id}，队列 {manualRunResult.queue}，类型 {manualRunResult.type}。</div>}
            </div>
          )}
        </Dialog>
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
function apiURL(path: string) {
  return buildApiURL(path, defaultApiBaseURL)
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

function Metric({ label, value }: { label: string; value: ReactNode }) {
  return <div className="metric"><span>{label}</span><strong>{value}</strong></div>
}

function EmptyState({ text }: { text: string }) {
  return <div className="empty-state">{text}</div>
}

function Field({ label, name, defaultValue = '', type = 'text', value, onChange, required = false, autoComplete, inputMode, pattern, minLength, maxLength }: { label: string; name: string; defaultValue?: string; type?: string; value?: string; onChange?: (value: string) => void; required?: boolean; autoComplete?: string; inputMode?: 'text' | 'numeric' | 'tel' | 'email' | 'decimal' | 'search' | 'url' | 'none'; pattern?: string; minLength?: number; maxLength?: number }) {
  return (
    <label>
      {label}
      <input name={name} defaultValue={value === undefined ? defaultValue : undefined} value={value} type={type} required={required} autoComplete={autoComplete} inputMode={inputMode} pattern={pattern} minLength={minLength} maxLength={maxLength} onChange={onChange ? (event) => onChange(event.currentTarget.value) : undefined} />
    </label>
  )
}

function ReadonlyJSON({ value }: { value: unknown }) {
  return <pre className="json-preview" aria-label="只读 JSON">{jsonText(value)}</pre>
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

function loginFailureMessage(status: number, mode: 'password' | 'phone' | 'reset' | 'code' = 'password') {
  if (status === 401) return mode === 'password' ? '账号或密码不正确，请重试。' : '手机号或验证码无效，请重试。'
  if (status === 429) return '登录尝试过于频繁，请稍后再试。'
  if (status === 503) return mode === 'code' ? '短信服务暂时不可用，密码登录仍可使用。' : '认证服务暂时不可用，请稍后再试。'
  if (status >= 500) return '登录服务暂时不可用，请稍后再试。'
  return '请求未完成，请检查输入后重试。'
}

function navPermission(key: NavKey) {
  const permissions: Partial<Record<NavKey, string>> = {
	store_info: 'mall.read', mall_weather: 'weather.read', sources: 'source.read', receive: 'data.read', pull_records: 'data.read',
	backfill: 'data.manage', youzan_distribution: 'data.manage', rules: 'pipeline.read', processed: 'data.read', methods: 'pipeline.read',
	destinations: 'delivery.read', tasks: 'delivery.read', push_policy: 'delivery.read', runs: 'pipeline.read', step_runs: 'pipeline.read',
		delivery_logs: 'delivery.read', excel_jobs: 'excel.read', excel_schemes: 'excel.read', excel_write: 'excel.manage',
		report_catalog: 'report.read', report_configuration: 'report.manage', report_query: 'report.execute', report_exports: 'report.export',
  }
  return permissions[key]
}

function reportCenterSection(key: NavKey): ReportCenterSection | null {
  if (key === 'report_catalog') return 'catalog'
  if (key === 'report_configuration') return 'configuration'
  if (key === 'report_query') return 'query'
  if (key === 'report_exports') return 'exports'
  return null
}

function reportCenterNavKey(section: ReportCenterSection): NavKey {
  if (section === 'configuration') return 'report_configuration'
  if (section === 'query') return 'report_query'
  if (section === 'exports') return 'report_exports'
  return 'report_catalog'
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

function monitoringDayStartTime() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}T00:00`
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

export default App
