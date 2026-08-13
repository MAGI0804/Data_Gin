import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import styles from './App.module.css'
import { apiURL as buildApiURL } from './apiURL'
import { clearStoredToken, loadStoredSessionUser, loadStoredToken, saveStoredSessionUser, saveStoredToken, saveStoredTokenExpiry, storedTokenExpiresAt, tokenActorID, type StoredSessionUser } from './authStorage'
import { createApiClient, type ApiRequestOptions, type ClientResponse, type HTTPMethod } from './api/client'
import { verifySessionResponses, type SessionUser } from './api/auth'
import { parseDataStatisticsSummary, parseHealthSummary, parseMallWeatherMetricsSummary, type DataStatisticsSummary, type HealthSummary, type MallWeatherMetricsSummary } from './monitoring'
import { MallWeatherPage, StoreInfoPage } from './MallWeatherPage'
import { AccessManagementPage } from './AccessManagementPage'
import { pipelineListPath } from './pipelineRun'
import { ReportCenter } from './reportCenter/ReportCenter'
import { AppShell } from './ui'
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
import { ConsoleHeader } from './appShell/ConsoleHeader'
import { ConsoleNavigation } from './appShell/ConsoleNavigation'
import { consoleNavigationClassName } from './appShell/consoleNavigationStyles'
import { LoginScreen } from './appShell/LoginScreen'
import { ResultDrawer } from './appShell/ResultDrawer'
import { navFromHash, reportCenterNavKey, reportCenterSection, usesCompactWorkspace, type NavKey } from './appShell/navigation'

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

type PipelineDefinition = {
  id: number
  name: string
  code: string
  enabled: boolean
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
        const desktopNavigationTarget = navigation?.querySelector<HTMLElement>('[data-nav-active="true"]')
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
  }, [mobileNavOpen, sessionState])

  useEffect(() => {
    const handleHashChange = () => {
      const nextNav = navFromHash()
      setActiveNav(nextNav)
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

  const reportSection = reportCenterSection(activeNav)
  const compactWorkspace = usesCompactWorkspace(activeNav)
  const shellNavigation = (
    <ConsoleNavigation
      activeNav={activeNav}
      mobileOpen={mobileNavOpen}
      permissions={sessionUser?.permissions ?? []}
      refreshing={refreshing}
      onLogout={handleLogout}
      onNavigate={navigate}
      onRefresh={() => void refreshWorkspace(true)}
      onToggleMobile={() => setMobileNavOpen((open) => !open)}
    />
  )

  return (
    <AppShell
      navigation={shellNavigation}
      navigationClassName={consoleNavigationClassName}
      navigationRef={mobileNavRef}
      navigationOpen={mobileNavOpen}
      onDismissNavigation={() => setMobileNavOpen(false)}
      flushWorkspace={compactWorkspace}
      header={<ConsoleHeader compact={compactWorkspace} activeNav={activeNav} loading={loading || refreshing} sessionUser={sessionUser} onOpenNavigation={openMobileNavigation} onRefresh={() => void refreshWorkspace(true)} onLogout={handleLogout} refreshing={refreshing} mobileNavTriggerRef={mobileNavTriggerRef} />}
      notices={<>{sessionValidationError && <div className={styles.notice} role="status" aria-live="polite">{sessionValidationError} <button type="button" onClick={() => setSessionValidationAttempt((attempt) => attempt + 1)}>重试校验</button></div>}{workspaceError && <div className={styles.notice} role="alert">{workspaceError} <button type="button" onClick={() => void refreshWorkspace(false)} disabled={refreshing}>重试</button></div>}</>}
      overlay={<ResultDrawer result={result} onClose={() => setResult(null)} />}
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

function apiURL(path: string) {
  return buildApiURL(path, defaultApiBaseURL)
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

function monitoringDayStartTime() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}T00:00`
}

export default App
