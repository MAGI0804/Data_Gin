import { FormEvent, ReactNode, type RefObject, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import {
  Activity,
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
import { AppShell, WorkspaceHeader } from './ui'
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
import { BojunBackfillPage } from './backfillPages/BojunBackfillPage'
import { YouzanDistributionPage } from './backfillPages/YouzanDistributionPage'
import type { LegacyTask } from './backfillPages/youzanDistributionSupport'

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
      navigation={shellNavigation}
      navigationClassName="ops-sidebar"
      navigationRef={mobileNavRef}
      navigationOpen={mobileNavOpen}
      onDismissNavigation={() => setMobileNavOpen(false)}
      flushWorkspace={Boolean(reportSection) || ['access_management', 'sources', 'receive', 'pull_records', 'backfill', 'youzan_distribution', 'rules', 'processed', 'methods', 'destinations', 'tasks', 'push_policy', 'overview', 'runs', 'delivery_logs', 'step_runs', 'store_info', 'mall_weather'].includes(activeNav)}
      workspaceClassName="ops-workspace"
      header={<ModuleHeader compact={Boolean(reportSection) || ['access_management', 'sources', 'receive', 'pull_records', 'backfill', 'youzan_distribution', 'rules', 'processed', 'methods', 'destinations', 'tasks', 'push_policy', 'overview', 'runs', 'delivery_logs', 'step_runs', 'store_info', 'mall_weather'].includes(activeNav)} activeNav={activeNav} loading={loading || refreshing} sessionUser={sessionUser} onOpenNavigation={openMobileNavigation} onRefresh={() => void refreshWorkspace(true)} onLogout={handleLogout} refreshing={refreshing} mobileNavTriggerRef={mobileNavTriggerRef} />}
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
        {activeNav === 'backfill' && <BojunBackfillPage client={client} loading={loading || refreshing} onCompletedRefresh={() => refreshWorkspace(false)} />}
        {activeNav === 'youzan_distribution' && <YouzanDistributionPage client={client} task={legacyTasks.find((item) => item.code === 'youzan_distribution_order_fetch')} loading={loading || refreshing} onCompletedRefresh={() => refreshWorkspace(false)} />}
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

function monitoringDayStartTime() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}T00:00`
}

export default App
