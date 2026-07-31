import { type FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { CloudRain, Download, MapPin, RefreshCcw, Thermometer, Wind } from 'lucide-react'
import './MallWeatherPage.css'
import { MallWeatherChart } from './MallWeatherChart'
import { MallWeatherExportPanel } from './MallWeatherExportPanel'
import { MallWeatherExportProfilePanel } from './MallWeatherExportProfilePanel'
import { MallWeatherForecastPanel, type MallWeatherForecastDataSnapshot } from './MallWeatherForecastPanel'
import { MallDetailsFields, MallWeatherMallEditor } from './MallWeatherMallEditor'
import { mallWeatherCapacityPlanPath, parseMallWeatherCapacityPlan, type MallWeatherCapacityPlan, type MallWeatherCapacityPlanInput } from './mallWeatherCapacityPlan'
import { mallImportRequestWithinLimit, parseMallImportCSV, parseMallImportResult, type MallImportResult, type MallImportRow } from './mallImport'
import {
  createMallWeatherDatasetCsv,
  downloadMallWeatherBytes,
  mallWeatherCsvFileName,
  type MallWeatherCsvZipData,
} from './mallWeatherCsv'
import { mallWeatherDataNavigationItems, navigateMallWeatherSection } from './mallWeatherNavigation'
import {
  mallWeatherCreateKey,
  mallWeatherCreateRequest,
  clearMallWeatherPendingCreate,
  clearMallWeatherPendingRefresh,
  clearMallWeatherPendingSheetPush,
  loadMallWeatherPendingCreate,
  loadMallWeatherPendingRefresh,
  loadMallWeatherPendingSheetPush,
  loadAllMallWeatherAlerts,
  mallWeatherFreshnessLabel,
  mallWeatherMetric,
  mallWeatherGeocodeCandidatesPath,
  mallWeatherCandidateConfirmationRequest,
  mallWeatherCoordinateAdjustmentRequest,
  mallWeatherGeocodeConfirmPath,
  mallWeatherGeocodeRunTerminal,
  mallWeatherShouldPollGeocode,
  mallWeatherMallReady,
  mallWeatherOverviewHasBusinessData,
  mallWeatherOverviewReadiness,
  mallWeatherSheetPushKey,
  mallWeatherSheetPushRequest,
  mallWeatherSheetPushRequestMatchesOption,
  mallWeatherSheetPushResultMatchesRequest,
  mallWeatherSheetPushRunMatchesResult,
  mergeMallWeatherMalls,
  mallWeatherOverviewPath,
  mallWeatherRefreshKey,
  mallWeatherRefreshDisposition,
  mallWeatherRefreshPath,
  mallWeatherRefreshRequest,
  mallWeatherRefreshResultMessage,
  pollMallWeatherFetchRun,
  saveMallWeatherPendingRefresh,
  saveMallWeatherPendingCreate,
  saveMallWeatherPendingSheetPush,
  mallWeatherSkyconLabel,
  parseMallWeatherMallList,
  parseMallWeatherMall,
  parseMallWeatherCreateResult,
  parseMallWeatherGeocodeCandidates,
  parseMallWeatherOverview,
  parseMallWeatherSheetPushDryRun,
  parseMallWeatherSheetPushOptions,
  parseMallWeatherSheetPushResult,
  pollMallWeatherSheetPushRun,
  submitMallWeatherGeocodeConfirmation,
  submitMallWeatherGeocodeTrigger,
  type MallWeatherCreateInput,
  type MallWeatherAlert,
  type MallWeatherPendingCreate,
  type MallWeatherPendingSheetPush,
  type MallWeatherGeocodeConfirmRequest,
  type MallWeatherGeocodeCandidates,
  type MallWeatherMall,
  type MallWeatherOverview,
  type MallWeatherOverviewReadiness,
  type MallWeatherPendingRefresh,
  type MallWeatherRefreshRequest,
  type MallWeatherFetchRun,
  type MallWeatherSheetPushDryRun,
  type MallWeatherSheetPushOption,
  type MallWeatherSheetPushRun,
} from './mallWeather'

type MallWeatherApiResult = {
  ok: boolean
  status: number
  data: unknown
}

type MallWeatherApiClient = (
  path: string,
  options?: { method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'; body?: unknown; headers?: Record<string, string>; showResult?: boolean; silentLoading?: boolean; signal?: AbortSignal },
) => Promise<MallWeatherApiResult>

type MallWeatherFileClient = (
  path: string,
  fileName: string,
  signal: AbortSignal,
) => Promise<MallWeatherApiResult>

type LoadState = 'idle' | 'loading' | 'success' | 'error'
type OverviewLoadState = LoadState | 'waiting'
type WeatherOverviewReloadResult = 'ready' | 'waiting' | 'failed' | 'aborted'
type OverviewWaitingReason = Exclude<MallWeatherOverviewReadiness, 'ready'> | ''
type OverviewStableState = { mallID: number; state: 'success' | 'waiting'; reason: OverviewWaitingReason }
type AlertDataSnapshot = {
  mallID: number
  items: MallWeatherAlert[]
  loading: boolean
  ready: boolean
  error: string
}

type MallModulePageProps = {
  actorID: string | null
  client: MallWeatherApiClient
  downloadFile: MallWeatherFileClient
}

export function MallWeatherPage(props: MallModulePageProps) {
  return <MallModulePage {...props} view="weather" />
}

export function StoreInfoPage(props: MallModulePageProps) {
  return <MallModulePage {...props} view="stores" />
}

function MallModulePage({
  actorID,
  client,
  downloadFile,
  view,
}: MallModulePageProps & { view: 'weather' | 'stores' }) {
  const [malls, setMalls] = useState<MallWeatherMall[]>([])
  const [nextAfterID, setNextAfterID] = useState(0)
  const [mallState, setMallState] = useState<LoadState>('idle')
  const [mallError, setMallError] = useState('')
  const [selectedMallID, setSelectedMallID] = useState(0)
  const [overview, setOverview] = useState<MallWeatherOverview | null>(null)
  const [overviewMallID, setOverviewMallID] = useState(0)
  const [overviewState, setOverviewState] = useState<OverviewLoadState>('idle')
  const [overviewError, setOverviewError] = useState('')
  const [overviewRetryCount, setOverviewRetryCount] = useState(0)
  const [overviewWaitingReason, setOverviewWaitingReason] = useState<OverviewWaitingReason>('')
  const [weatherReloadVersion, setWeatherReloadVersion] = useState(0)
  const [forecastSnapshot, setForecastSnapshot] = useState<MallWeatherForecastDataSnapshot | null>(null)
  const [alertSnapshot, setAlertSnapshot] = useState<AlertDataSnapshot>({ mallID: 0, items: [], loading: false, ready: false, error: '' })
  const [query, setQuery] = useState('')
  const [city, setCity] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [showImport, setShowImport] = useState(false)
  const mallRequestSequence = useRef(0)
  const overviewRequestSequence = useRef(0)
  const overviewController = useRef<AbortController | null>(null)
  const overviewSnapshotRef = useRef<MallWeatherOverview | null>(null)
  const overviewSnapshotMallIDRef = useRef(0)
  const overviewRetryMallIDRef = useRef(0)
  const overviewLastStableRef = useRef<OverviewStableState | null>(null)
  const selectedMallIDRef = useRef(0)
  const alertRequestSequence = useRef(0)
  const alertController = useRef<AbortController | null>(null)

  const updateOverviewState = useCallback((state: OverviewLoadState) => {
    setOverviewState(state)
  }, [])

  const updateOverviewWaitingReason = useCallback((reason: OverviewWaitingReason) => {
    setOverviewWaitingReason(reason)
  }, [])

  const updateOverviewStableState = useCallback((
    mallID: number,
    state: OverviewStableState['state'],
    reason: OverviewWaitingReason,
  ) => {
    overviewLastStableRef.current = { mallID, state, reason }
    setOverviewState(state)
    setOverviewWaitingReason(reason)
  }, [])

  useEffect(() => {
    selectedMallIDRef.current = selectedMallID
  }, [selectedMallID])

  const loadMalls = useCallback(async (afterID = 0) => {
    const sequence = ++mallRequestSequence.current
    setMallState('loading')
    setMallError('')
    const search = new URLSearchParams({ limit: '50' })
    if (afterID > 0) search.set('afterId', String(afterID))
    const response = await client(`/v1/malls?${search.toString()}`, { method: 'GET', showResult: false, silentLoading: true })
    if (sequence !== mallRequestSequence.current) return
    if (!response.ok) {
      setMallState('error')
      setMallError(weatherRequestError(response.status, '商场列表加载失败', '当前账号缺少 mall.read 权限'))
      return
    }
    const parsed = parseMallWeatherMallList(response.data)
    if (!parsed) {
      setMallState('error')
      setMallError('商场列表响应格式不正确，请联系管理员')
      return
    }
    if (parsed.items.length === 50 && parsed.nextAfterId <= afterID) {
      setMallState('error')
      setMallError('商场列表游标无效，请联系管理员')
      return
    }
    let nextItems = parsed.items
    const selectedID = selectedMallIDRef.current
    if (afterID === 0 && selectedID > 0 && !nextItems.some((mall) => mall.id === selectedID)) {
      const selectedResponse = await client(`/v1/malls/${selectedID}`, { method: 'GET', showResult: false, silentLoading: true })
      if (sequence !== mallRequestSequence.current) return
      const selected = selectedResponse.ok ? parseMallWeatherMall(selectedResponse.data) : null
      if (selected) nextItems = mergeMallWeatherMalls(nextItems, [selected])
    }
    setMalls((current) => {
      if (afterID > 0) return mergeMallWeatherMalls(current, nextItems)
      const liveSelected = current.find((mall) => mall.id === selectedMallIDRef.current)
      const base = liveSelected && !nextItems.some((mall) => mall.id === liveSelected.id) ? [...nextItems, liveSelected] : nextItems
      const baseByID = new Map(base.map((mall) => [mall.id, mall]))
      const fresherCurrent = current.filter((mall) => {
        const incoming = baseByID.get(mall.id)
        return Boolean(incoming && mall.version > incoming.version)
      })
      return mergeMallWeatherMalls(base, fresherCurrent)
    })
    setNextAfterID(parsed.items.length === 50 ? parsed.nextAfterId : 0)
    setMallState('success')
    if (afterID === 0) setSelectedMallID((current) => current !== selectedID ? current : nextItems.some((mall) => mall.id === current) ? current : nextItems[0]?.id || 0)
  }, [client])

  const loadMall = useCallback(async (mallID: number) => {
    const response = await client(`/v1/malls/${mallID}`, { method: 'GET', showResult: false, silentLoading: true })
    if (!response.ok) return null
    const parsed = parseMallWeatherMall(response.data)
    if (!parsed) return null
    setMalls((current) => mergeMallWeatherMalls(current, [parsed]))
    return parsed
  }, [client])

  const loadOverview = useCallback(async (
    mallID: number,
    externalSignal?: AbortSignal,
  ): Promise<WeatherOverviewReloadResult> => {
    if (!mallID || externalSignal?.aborted || selectedMallIDRef.current !== mallID) return 'aborted'
    if (overviewRetryMallIDRef.current !== mallID) {
      overviewRetryMallIDRef.current = mallID
      setOverviewRetryCount(0)
    }
    const previousStable = overviewLastStableRef.current?.mallID === mallID
      ? overviewLastStableRef.current
      : null
    overviewController.current?.abort()
    const controller = new AbortController()
    overviewController.current = controller
    const abortFromExternal = () => controller.abort()
    externalSignal?.addEventListener('abort', abortFromExternal, { once: true })
    const sequence = ++overviewRequestSequence.current
    const restoreAfterAbort = () => {
      if (sequence !== overviewRequestSequence.current || selectedMallIDRef.current !== mallID) return
      if (previousStable) {
        updateOverviewStableState(mallID, previousStable.state, previousStable.reason)
        return
      }
      const previous = overviewSnapshotMallIDRef.current === mallID ? overviewSnapshotRef.current : null
      if (!previous) {
        updateOverviewStableState(mallID, 'waiting', 'waiting-empty')
        return
      }
      const previousReadiness = mallWeatherOverviewReadiness(previous)
      updateOverviewStableState(
        mallID,
        previousReadiness === 'ready' ? 'success' : 'waiting',
        previousReadiness === 'ready' ? '' : previousReadiness,
      )
    }
    try {
      updateOverviewState('loading')
      setOverviewError('')
      let response: MallWeatherApiResult
      try {
        response = await client(mallWeatherOverviewPath(mallID), {
          method: 'GET', showResult: false, silentLoading: true, signal: controller.signal,
        })
      } catch {
        if (controller.signal.aborted || sequence !== overviewRequestSequence.current) {
          restoreAfterAbort()
          return 'aborted'
        }
        updateOverviewState('error')
        updateOverviewWaitingReason('')
        setOverviewError('天气概览加载异常，请检查网络后重试')
        return 'failed'
      }
      if (controller.signal.aborted || sequence !== overviewRequestSequence.current) {
        restoreAfterAbort()
        return 'aborted'
      }
      if (!response.ok) {
        if (response.status === 404) {
          const previous = overviewSnapshotMallIDRef.current === mallID ? overviewSnapshotRef.current : null
          if (!previous || !mallWeatherOverviewHasBusinessData(previous)) {
            overviewSnapshotRef.current = null
            overviewSnapshotMallIDRef.current = 0
            setOverview(null)
            setOverviewMallID(0)
          }
          updateOverviewStableState(mallID, 'waiting', previous && mallWeatherOverviewHasBusinessData(previous)
            ? 'waiting-hourly-temperature'
            : 'waiting-empty')
          return 'waiting'
        }
        updateOverviewState('error')
        updateOverviewWaitingReason('')
        setOverviewError(weatherRequestError(response.status, '天气概览加载失败', '当前账号缺少 weather.read 权限'))
        return 'failed'
      }
      const parsed = parseMallWeatherOverview(response.data)
      if (!parsed) {
        updateOverviewState('error')
        updateOverviewWaitingReason('')
        setOverviewError('天气概览响应格式不正确，请联系管理员')
        return 'failed'
      }
      const readiness = mallWeatherOverviewReadiness(parsed)
      if (readiness !== 'ready') {
        const previous = overviewSnapshotMallIDRef.current === mallID ? overviewSnapshotRef.current : null
        const preservePrevious = Boolean(previous && (
          mallWeatherOverviewReadiness(previous) === 'ready' ||
          (readiness === 'waiting-empty' && mallWeatherOverviewHasBusinessData(previous))
        ))
        if (!preservePrevious) {
          overviewSnapshotRef.current = parsed
          overviewSnapshotMallIDRef.current = mallID
          setOverview(parsed)
          setOverviewMallID(mallID)
        }
        updateOverviewStableState(mallID, 'waiting', preservePrevious && previous && mallWeatherOverviewHasBusinessData(previous)
          ? 'waiting-hourly-temperature'
          : readiness)
        return 'waiting'
      }
      overviewSnapshotRef.current = parsed
      overviewSnapshotMallIDRef.current = mallID
      setOverview(parsed)
      setOverviewMallID(mallID)
      updateOverviewStableState(mallID, 'success', '')
      setOverviewRetryCount(0)
      setForecastSnapshot(null)
      setWeatherReloadVersion((current) => current + 1)
      return 'ready'
    } finally {
      externalSignal?.removeEventListener('abort', abortFromExternal)
      if (overviewController.current === controller) overviewController.current = null
    }
  }, [client, updateOverviewStableState, updateOverviewState, updateOverviewWaitingReason])

  useEffect(() => {
    void loadMalls()
  }, [loadMalls])

  useEffect(() => {
    const mall = malls.find((item) => item.id === selectedMallID)
    if (view === 'weather' && mall && mallWeatherMallReady(mall)) {
      void loadOverview(selectedMallID)
      return
    }
    overviewRequestSequence.current++
    overviewController.current?.abort()
    overviewSnapshotRef.current = null
    overviewSnapshotMallIDRef.current = 0
    overviewRetryMallIDRef.current = 0
    overviewLastStableRef.current = null
    setOverview(null)
    setOverviewMallID(0)
    updateOverviewState('idle')
    updateOverviewWaitingReason('')
    setOverviewError('')
    setOverviewRetryCount(0)
  }, [loadOverview, malls, selectedMallID, updateOverviewState, updateOverviewWaitingReason, view])

  useEffect(() => () => {
    overviewController.current?.abort()
    overviewRequestSequence.current++
  }, [])

  useEffect(() => {
    const mall = malls.find((item) => item.id === selectedMallID)
    if (view !== 'weather' || !mall || !mallWeatherMallReady(mall) || overviewState !== 'waiting' || overviewRetryCount >= 30) return
    const timer = window.setTimeout(() => {
      setOverviewRetryCount((current) => current + 1)
      void loadOverview(mall.id)
    }, 2_000)
    return () => window.clearTimeout(timer)
  }, [loadOverview, malls, overviewRetryCount, overviewState, selectedMallID, view])

  const cities = useMemo(() => Array.from(new Set(malls.map((mall) => mall.city).filter(Boolean))).sort(), [malls])
  useEffect(() => {
    if (city && !cities.includes(city)) setCity('')
  }, [cities, city])
  const visibleMalls = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    return malls.filter((mall) => (!city || mall.city === city) && (!normalized || `${mall.nameCn} ${mall.mallCode} ${mall.city} ${mall.address}`.toLowerCase().includes(normalized)))
  }, [city, malls, query])
  const selectedMall = malls.find((mall) => mall.id === selectedMallID)
  const selectedMallMatchesFilter = Boolean(selectedMall && visibleMalls.some((mall) => mall.id === selectedMall.id))
  const selectedOverview = selectedMallID === overviewMallID ? overview : null
  const selectedMallReady = Boolean(selectedMall && mallWeatherMallReady(selectedMall))
  const handleMallCreated = useCallback((mall: MallWeatherMall) => {
    setMalls((current) => mergeMallWeatherMalls(current, [mall]))
    selectedMallIDRef.current = mall.id
    setSelectedMallID(mall.id)
    setShowCreate(false)
    void loadMall(mall.id)
  }, [loadMall])

  const handleMallUpdated = useCallback((mall: MallWeatherMall) => {
    setMalls((current) => mergeMallWeatherMalls(current, [mall]))
    setSelectedMallID(mall.id)
  }, [])

  const handleMallDeleted = useCallback((mallID: number) => {
    const remaining = malls.filter((mall) => mall.id !== mallID)
    const nextID = remaining[0]?.id ?? 0
    setMalls(remaining)
    selectedMallIDRef.current = nextID
    setSelectedMallID(nextID)
  }, [malls])

  const reloadWeatherData = useCallback(async (mallID: number, signal: AbortSignal) => {
    const result = await loadOverview(mallID, signal)
    if (result === 'waiting' && !signal.aborted) {
      setForecastSnapshot(null)
      setWeatherReloadVersion((current) => current + 1)
    }
    return result
  }, [loadOverview])

  const loadAlerts = useCallback(async (mallID: number, timeZone: string) => {
    if (!mallID || selectedMallIDRef.current !== mallID) return
    alertController.current?.abort()
    const controller = new AbortController()
    alertController.current = controller
    const sequence = ++alertRequestSequence.current
    setAlertSnapshot({ mallID, items: [], loading: true, ready: false, error: '' })
    let timedOut = false
    const timeout = window.setTimeout(() => {
      timedOut = true
      controller.abort()
    }, 15_000)
    try {
      const result = await loadAllMallWeatherAlerts(
        (path) => client(path, {
          method: 'GET', showResult: false, silentLoading: true, signal: controller.signal,
        }),
        mallID,
        timeZone,
      )
      if (controller.signal.aborted || sequence !== alertRequestSequence.current || selectedMallIDRef.current !== mallID) return
      setAlertSnapshot({ mallID, items: result.items, loading: false, ready: true, error: '' })
    } catch (error) {
      if (sequence !== alertRequestSequence.current || selectedMallIDRef.current !== mallID || (controller.signal.aborted && !timedOut)) return
      setAlertSnapshot({
        mallID,
        items: [],
        loading: false,
        ready: false,
        error: timedOut ? '气象预警查询超时，请检查网络后重试' : error instanceof Error ? error.message : '气象预警查询失败',
      })
    } finally {
      window.clearTimeout(timeout)
      if (alertController.current === controller) alertController.current = null
    }
  }, [client])

  useEffect(() => {
    setForecastSnapshot(null)
  }, [selectedMallID, weatherReloadVersion])

  useEffect(() => {
    if (view !== 'weather' || !selectedMall || !selectedMallReady) {
      alertRequestSequence.current++
      alertController.current?.abort()
      alertController.current = null
      setAlertSnapshot({ mallID: 0, items: [], loading: false, ready: false, error: '' })
      return
    }
    void loadAlerts(selectedMall.id, selectedMall.timeZone)
  }, [loadAlerts, selectedMall, selectedMallReady, view, weatherReloadVersion])

  useEffect(() => () => {
    alertRequestSequence.current++
    alertController.current?.abort()
  }, [])

  const handleForecastDatasetsChange = useCallback((snapshot: MallWeatherForecastDataSnapshot) => {
    if (selectedMallIDRef.current === snapshot.mallID) setForecastSnapshot(snapshot)
  }, [])

  const selectedAlerts = alertSnapshot.mallID === selectedMallID
    ? alertSnapshot
    : { mallID: selectedMallID, items: [], loading: true, ready: false, error: '' }
  const selectedForecast = forecastSnapshot?.mallID === selectedMallID ? forecastSnapshot : null
  const csvZipData = useMemo<MallWeatherCsvZipData>(() => ({
    realtime: selectedOverview?.realtime ? [selectedOverview.realtime] : [],
    minutely: selectedForecast?.minutely ?? [],
    hourly: selectedForecast?.hourly ?? [],
    daily: selectedForecast?.daily ?? [],
    alerts: selectedAlerts.items,
    life_indices: selectedForecast?.lifeIndices ?? [],
  }), [selectedAlerts.items, selectedForecast, selectedOverview])
  const csvZipReady = Boolean(overviewState === 'success' && selectedOverview && selectedForecast?.ready && selectedAlerts.ready)
  const csvZipLoading = overviewState === 'idle' || overviewState === 'loading' || overviewState === 'waiting' ||
    Boolean(selectedForecast?.loading) || selectedAlerts.loading
  const csvZipStatus = overviewState === 'loading'
    ? '正在重新加载当前商场实况'
    : overviewState === 'error'
      ? overviewError || '当前商场实况加载失败'
      : overviewState === 'waiting'
        ? '正在等待当前商场完整实况数据'
        : !selectedOverview
          ? '正在等待当前商场实况数据'
          : !selectedForecast || selectedForecast.loading
      ? '正在加载完整预报与生活指数'
      : selectedForecast.error
        ? selectedForecast.error
        : selectedAlerts.loading
          ? '正在加载全部气象预警'
          : selectedAlerts.error || (csvZipReady ? '六类天气数据已就绪' : '天气数据尚未准备完成')

  if (view === 'stores') {
    return (
      <div className="view-stack mall-weather-page store-info-page">
        <section className="workbench-panel store-info-toolbar" aria-label="店铺筛选与选择">
          <div className="store-info-toolbar-fields">
            <label>
              <span>筛选店铺</span>
              <input name="storeInfoQuery" type="search" autoComplete="off" value={query} onChange={(event) => setQuery(event.currentTarget.value)} placeholder="名称、编码或地址" />
              <small>{query.trim() || city ? `找到 ${visibleMalls.length} 个店铺` : `共 ${malls.length} 个店铺`}</small>
            </label>
            <label>
              <span>城市</span>
              <select name="storeInfoCity" value={city} onChange={(event) => setCity(event.currentTarget.value)}>
                <option value="">全部城市</option>
                {cities.map((item) => <option value={item} key={item}>{item}</option>)}
              </select>
            </label>
            <label>
              <span>选择店铺</span>
              <select
                name="storeInfoMallID"
                value={selectedMall ? selectedMallID : ''}
                onChange={(event) => {
                  const mallID = Number(event.currentTarget.value)
                  if (!Number.isSafeInteger(mallID) || mallID <= 0 || !visibleMalls.some((mall) => mall.id === mallID)) return
                  selectedMallIDRef.current = mallID
                  setSelectedMallID(mallID)
                  setShowCreate(false)
                }}
                disabled={!selectedMall && visibleMalls.length === 0}
              >
                {!selectedMall && visibleMalls.length === 0 && <option value="">没有匹配的店铺</option>}
                {selectedMall && !selectedMallMatchesFilter && <option value={selectedMall.id}>{selectedMall.nameCn}</option>}
                {visibleMalls.map((mall) => <option value={mall.id} key={mall.id}>{mall.nameCn}</option>)}
              </select>
              {selectedMall && !selectedMallMatchesFilter && <small>当前店铺不在筛选结果中，可从匹配结果切换</small>}
            </label>
          </div>
          <div className="store-info-toolbar-actions">
            <button type="button" onClick={() => void loadMalls()} disabled={mallState === 'loading'}>
              <RefreshCcw aria-hidden="true" />刷新店铺
            </button>
            {nextAfterID > 0 && <button type="button" onClick={() => void loadMalls(nextAfterID)} disabled={mallState === 'loading'}>加载更多</button>}
            <button className="primary" type="button" onClick={() => setShowCreate((current) => !current)} aria-expanded={showCreate} aria-controls="mall-weather-create-panel">
              {showCreate ? '返回店铺资料' : '新增店铺'}
            </button>
            <button type="button" onClick={() => setShowImport((current) => !current)}>批量导入 CSV</button>
          </div>
        </section>

        {mallState === 'error' && <RequestError message={mallError} onRetry={() => void loadMalls()} />}
        {mallState === 'loading' && malls.length === 0 && <LoadingState label="正在加载店铺" />}
        {mallState === 'success' && malls.length === 0 && <EmptyState title="还没有店铺" detail="点击“新增店铺”开始维护基础资料。" />}

        {showCreate && (actorID
          ? <MallCreatePanel actorID={actorID} client={client} onCreated={handleMallCreated} onCancel={() => setShowCreate(false)} />
          : <RequestError message="无法识别当前登录账号，请退出后重新登录再新增店铺。" onRetry={() => window.location.reload()} />)}
        {showImport && <MallImportPanel client={client} onImported={() => void loadMalls()} />}

        {!showCreate && selectedMall && (actorID
          ? <div className="store-info-detail-layout">
            <aside className="workbench-panel store-info-summary" aria-label={`${selectedMall.nameCn}资料摘要`}>
              <div className="store-info-summary-heading">
                <span>当前店铺</span>
                <strong>{selectedMall.nameCn}</strong>
                <small>{selectedMall.mallCode}</small>
              </div>
              <dl>
                <div><dt>城市</dt><dd>{selectedMall.city || '待完善'}</dd></div>
                <div><dt>行政区</dt><dd>{selectedMall.district || '待完善'}</dd></div>
                <div><dt>地址</dt><dd>{selectedMall.address || '待完善'}</dd></div>
                <div><dt>坐标状态</dt><dd>{selectedMall.geocodeStatus === 'confirmed' ? '已确认' : '待确认'}</dd></div>
                <div><dt>天气服务</dt><dd>{selectedMall.weatherEnabled ? '已启用' : '未启用'}</dd></div>
              </dl>
            </aside>
            <section className="view-stack store-info-maintenance" aria-label={`${selectedMall.nameCn}店铺资料维护`}>
              <div className="store-info-maintenance-heading">
                <div><strong>店铺资料维护</strong><span>基础资料、地址解析和服务坐标在此统一维护</span></div>
                <span>版本 {selectedMall.version}</span>
              </div>
              <MallWeatherMallEditor
                mall={selectedMall}
                client={client}
                onMallUpdated={handleMallUpdated}
                onMallDeleted={handleMallDeleted}
                key={`store-editor-${selectedMall.id}`}
              />
              {selectedMallReady
                ? <MallCoordinateAdjustmentPanel mall={selectedMall} client={client} onMallUpdated={handleMallUpdated} key={`store-coordinate-${selectedMall.id}:${selectedMall.version}`} />
                : <MallOnboardingPanel mall={selectedMall} client={client} onMallUpdated={handleMallUpdated} onReloadMall={loadMall} key={`store-onboarding-${selectedMall.id}`} />}
            </section>
          </div>
          : <RequestError message="无法识别当前登录账号，请退出后重新登录再维护店铺。" onRetry={() => window.location.reload()} />)}
      </div>
    )
  }

  return (
    <div className="view-stack mall-weather-page mall-weather-single-page">
      <section className="workbench-panel mall-weather-selector" aria-label="选择天气商场">
        <div className="mall-weather-selector-fields">
          <label>
            <span>筛选商场</span>
            <input
              name="mallWeatherQuery"
              type="search"
              autoComplete="off"
              value={query}
              onChange={(event) => setQuery(event.currentTarget.value)}
              placeholder="名称、编码、城市或地址"
            />
            <small>{query.trim() ? `找到 ${visibleMalls.length} 个商场` : `共 ${malls.length} 个商场`}</small>
          </label>
          <label>
            <span>选择商场</span>
            <select
              name="mallWeatherMallID"
              value={selectedMall ? selectedMallID : ''}
              onChange={(event) => {
                const mallID = Number(event.currentTarget.value)
                if (!Number.isSafeInteger(mallID) || mallID <= 0 || !visibleMalls.some((mall) => mall.id === mallID)) return
                selectedMallIDRef.current = mallID
                setSelectedMallID(mallID)
              }}
              disabled={!selectedMall && visibleMalls.length === 0}
            >
              {!selectedMall && visibleMalls.length === 0 && <option value="">没有匹配的商场</option>}
              {selectedMall && !selectedMallMatchesFilter && <option value={selectedMall.id}>{selectedMall.nameCn}</option>}
              {visibleMalls.map((mall) => <option value={mall.id} key={mall.id}>{mall.nameCn}</option>)}
            </select>
            <small>{selectedMall && !selectedMallMatchesFilter
              ? `当前商场不在筛选结果中 · ${selectedMall.mallCode}`
              : selectedMall
                ? `${selectedMall.mallCode} · ${selectedMall.city || '城市待完善'}`
                : '选择商场后查看全部天气数据'}</small>
          </label>
        </div>
        <div className="mall-weather-selector-actions">
          <button type="button" onClick={() => void loadMalls()} disabled={mallState === 'loading'}>
            <RefreshCcw aria-hidden="true" />刷新商场
          </button>
          {nextAfterID > 0 && <button type="button" onClick={() => void loadMalls(nextAfterID)} disabled={mallState === 'loading'}>加载更多</button>}
        </div>
      </section>

      <MallWeatherCapacityPlanPanel client={client} />
      <MallWeatherExportProfilePanel client={client} />

      {mallState === 'error' && <RequestError message={mallError} onRetry={() => void loadMalls()} />}
      {mallState === 'loading' && malls.length === 0 && <LoadingState label="正在加载商场" />}
      {mallState === 'success' && malls.length === 0 && <EmptyState title="还没有可用商场" detail="请先到“基础信息 → 店铺信息”新增并维护店铺。" />}
      {selectedMall && !selectedMallReady && <section className="workbench-panel mall-weather-unavailable">
        <div><strong>{selectedMall.nameCn}尚未完成天气接入</strong><span>请到“基础信息 → 店铺信息”完成地址、坐标和天气启用设置。</span></div>
        <button type="button" onClick={() => { window.location.hash = 'store_info' }}>前往店铺信息</button>
      </section>}

      {selectedMallReady && selectedMall && <>
        <div className="mall-weather-actions-row">
          {actorID
            ? <ManualRefreshPanel
              actorID={actorID}
              mall={selectedMall}
              client={client}
              onWeatherUpdated={(signal) => reloadWeatherData(selectedMall.id, signal)}
              key={`refresh-${actorID}:${selectedMall.id}`}
            />
            : <RequestError message="无法识别当前登录账号，请退出后重新登录再提交天气刷新。" onRetry={() => window.location.reload()} />}
          {actorID && <MallWeatherExportPanel
            actorID={actorID}
            mallID={selectedMall.id}
            mallCode={selectedMall.mallCode}
            mallName={selectedMall.nameCn}
            csvData={csvZipData}
            csvReady={csvZipReady}
            csvLoading={csvZipLoading}
            csvStatus={csvZipStatus}
            client={client}
            downloadFile={downloadFile}
            compact
            key={`weather-export-${actorID}:${selectedMall.id}`}
          />}
        </div>
        <MallWeatherDataNavigation />
        <div id="mall-weather-overview" tabIndex={-1}>
          {!selectedOverview && overviewState !== 'error' && overviewState !== 'waiting' &&
            <LoadingState label={`正在加载${selectedMall.nameCn}天气`} />}
          {selectedOverview && <WeatherRealtime mall={selectedMall} overview={selectedOverview} />}
          {overviewState === 'error' &&
            <RequestError message={overviewError} onRetry={() => { setOverviewRetryCount(0); void loadOverview(selectedMall.id) }} />}
          {overviewState === 'waiting' && overviewRetryCount < 30 &&
            <LoadingState label={overviewWaitingReason === 'waiting-empty'
              ? `首次天气采集中，正在等待实况与未来逐小时数据（${overviewRetryCount + 1}/30）`
              : `实况已加载，未来逐小时温度正在同步（${overviewRetryCount + 1}/30）`} />}
          {overviewState === 'waiting' && overviewRetryCount >= 30 &&
            <RequestError
              message={overviewWaitingReason === 'waiting-empty'
                ? '首次采集长时间未生成数据，请确认 MALL_WEATHER_ENABLED=true 且 weather 队列消费进程正在运行。'
                : '实况已加载，但未来逐小时温度长时间不可用。请确认天气业务事务已提交，并检查最近采集记录。'}
              onRetry={() => { setOverviewRetryCount(0); void loadOverview(selectedMall.id) }}
            />}
        </div>
        {selectedOverview && <WeatherOverviewDetails
          mall={selectedMall}
          overview={selectedOverview}
          alerts={selectedAlerts}
          refreshing={overviewState === 'loading'}
          onRefresh={() => void loadOverview(selectedMall.id)}
          onAlertsRetry={() => void loadAlerts(selectedMall.id, selectedMall.timeZone)}
        />}
        <MallWeatherForecastPanel
          mallID={selectedMall.id}
          mallCode={selectedMall.mallCode}
          mallName={selectedMall.nameCn}
          timeZone={selectedMall.timeZone}
          client={client}
          onDatasetsChange={handleForecastDatasetsChange}
          key={`forecast-${selectedMall.id}:${selectedMall.timeZone}:${weatherReloadVersion}`}
        />
        {actorID && <section id="mall-weather-push" tabIndex={-1}>
          <MallWeatherSheetPushPanel actorID={actorID} mall={selectedMall} client={client} key={`push-${actorID}:${selectedMall.id}`} />
        </section>}
      </>}
    </div>
  )
}

function MallWeatherDataNavigation() {
  function navigateTo(targetID: string) {
    const reduceMotion = typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    navigateMallWeatherSection(document, targetID, reduceMotion)
  }

  return (
    <nav className="mall-weather-data-nav" aria-label="天气数据快速入口">
      <strong>天气数据</strong>
      {mallWeatherDataNavigationItems.map((item) => (
        <button type="button" aria-controls={item.targetID} onClick={() => navigateTo(item.targetID)} key={item.targetID}>
          {item.label}
        </button>
      ))}
    </nav>
  )
}

const emptyMallCreateInput: MallWeatherCreateInput = {
  mallCode: '', nameCn: '', province: '', city: '', district: '', address: '',
}

const defaultMallWeatherCapacityInput: MallWeatherCapacityPlanInput = {
  mallCount: '', providerQps: '', hourlySteps: '360', dailySteps: '15', lifeIndexDays: '15', alertsPerMall: '0', feishuBatchRows: '200',
}

function MallWeatherCapacityPlanPanel({ client }: { client: MallWeatherApiClient }) {
  const [form, setForm] = useState(defaultMallWeatherCapacityInput)
  const [plan, setPlan] = useState<MallWeatherCapacityPlan | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const controllerRef = useRef<AbortController | null>(null)

  useEffect(() => () => controllerRef.current?.abort(), [])

  function change(field: keyof MallWeatherCapacityPlanInput, value: string) {
    setForm((current) => ({ ...current, [field]: value }))
    setError('')
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    let path: string
    try {
      path = mallWeatherCapacityPlanPath(form)
    } catch {
      setError('请输入有效的目标商场数、供应商 QPS 和各数据集范围。')
      return
    }
    controllerRef.current?.abort()
    const controller = new AbortController()
    controllerRef.current = controller
    setSubmitting(true)
    setError('')
    const response = await client(path, { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal })
    if (controller.signal.aborted) return
    setSubmitting(false)
    if (!response.ok) {
      setError(weatherRequestError(response.status, '容量规划计算失败', '当前账号缺少 weather.config.manage 权限'))
      return
    }
    const parsed = parseMallWeatherCapacityPlan(response.data)
    if (!parsed) {
      setError('容量规划响应格式不正确，请联系管理员。')
      return
    }
    setPlan(parsed)
  }

  return <section className="workbench-panel mall-weather-capacity-panel" aria-busy={submitting}>
    <div className="mall-weather-section-title"><div><strong>天气容量规划</strong><span>按规划目标测算供应商调用、数据库写入和飞书批次；不会修改任何配置。</span></div></div>
    <form className="mall-weather-capacity-form" onSubmit={submit}>
      <label><span>目标商场数 *</span><input name="capacityMallCount" inputMode="numeric" type="number" min="1" max="100000" value={form.mallCount} onChange={(event) => change('mallCount', event.currentTarget.value)} required disabled={submitting} /></label>
      <label><span>供应商 QPS *</span><input name="capacityProviderQps" inputMode="decimal" type="number" min="0" max="10000" step="any" value={form.providerQps} onChange={(event) => change('providerQps', event.currentTarget.value)} required disabled={submitting} /></label>
      <label><span>逐小时预报步数</span><input name="capacityHourlySteps" inputMode="numeric" type="number" min="1" max="360" value={form.hourlySteps} onChange={(event) => change('hourlySteps', event.currentTarget.value)} required disabled={submitting} /></label>
      <label><span>逐日预报天数</span><input name="capacityDailySteps" inputMode="numeric" type="number" min="1" max="15" value={form.dailySteps} onChange={(event) => change('dailySteps', event.currentTarget.value)} required disabled={submitting} /></label>
      <label><span>生活指数天数</span><input name="capacityLifeIndexDays" inputMode="numeric" type="number" min="1" max="15" value={form.lifeIndexDays} onChange={(event) => change('lifeIndexDays', event.currentTarget.value)} required disabled={submitting} /></label>
      <label><span>每商场预警数</span><input name="capacityAlertsPerMall" inputMode="numeric" type="number" min="0" max="256" value={form.alertsPerMall} onChange={(event) => change('alertsPerMall', event.currentTarget.value)} required disabled={submitting} /></label>
      <label><span>飞书每批行数</span><input name="capacityFeishuBatchRows" inputMode="numeric" type="number" min="1" max="500" value={form.feishuBatchRows} onChange={(event) => change('feishuBatchRows', event.currentTarget.value)} required disabled={submitting} /></label>
      <div className="mall-weather-form-actions"><button className="primary" type="submit" disabled={submitting}>{submitting ? '计算中' : '计算容量'}</button></div>
    </form>
    {plan && <>
      <div className="mall-weather-meta" aria-live="polite">
        <MetaItem label="每日供应商请求" value={String(plan.providerRequests)} />
        <MetaItem label="预计耗时" value={`${plan.providerDrainSeconds.toFixed(1)} 秒`} />
        <MetaItem label="一小时最低 QPS" value={plan.minimumQpsForOneHourDrain.toFixed(2)} />
        <MetaItem label="数据库总行数" value={String(plan.totalDatabaseRows)} />
        <MetaItem label="数据库批次" value={String(plan.totalDatabaseBatches)} />
        <MetaItem label="飞书批次" value={String(plan.totalFeishuBatches)} />
        <MetaItem label="飞书每批行数" value={String(plan.feishuBatchRows)} />
        <MetaItem label="规划商场数" value={String(plan.mallCount)} />
      </div>
      <div className="data-table-wrap"><table className="data-table"><caption>各天气数据集容量明细</caption><thead><tr><th>数据集</th><th>行数</th><th>数据库批次</th><th>飞书批次</th></tr></thead><tbody>{plan.datasets.map((dataset) => <tr key={dataset.kind}><td>{dataset.kind}</td><td>{dataset.rows}</td><td>{dataset.databaseBatches}</td><td>{dataset.feishuBatches}</td></tr>)}</tbody></table></div>
    </>}
    {error && <p className="mall-weather-action-message error" role="alert">{error}</p>}
  </section>
}

function MallImportPanel({ client, onImported }: { client: MallWeatherApiClient; onImported: () => void }) {
  const [rows, setRows] = useState<MallImportRow[]>([])
  const [result, setResult] = useState<MallImportResult | null>(null)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [canRetryOriginal, setCanRetryOriginal] = useState(false)
  const requestKeyRef = useRef(mallWeatherCreateKey())
  const fileInputRef = useRef<HTMLInputElement>(null)
  const fileSequenceRef = useRef(0)
  const items = rows.flatMap((row) => row.item ? [row.item] : [])
  const invalidRows = rows.filter((row) => row.error)
  const validForSubmit = items.length > 0 && invalidRows.length === 0

  async function submit() {
    if (!validForSubmit) {
      setError('请先修正 CSV 中的错误行。')
      return
    }
    if (!mallImportRequestWithinLimit(items)) {
      setError('转换后的导入请求超过 1 MiB 服务端限制，请减少导入行数或缩短字段内容。')
      return
    }
    setSubmitting(true)
    setError('')
    setCanRetryOriginal(false)
    const response = await client('/v1/malls/import', {
      method: 'POST', body: { items }, headers: { 'Idempotency-Key': requestKeyRef.current }, showResult: false, silentLoading: true,
    })
    setSubmitting(false)
    if (!response.ok) {
      setConfirming(false)
      setCanRetryOriginal(response.status === 0 || response.status === 409 || response.status >= 500)
      setError(importSubmissionError(response.status))
      return
    }
    const parsed = parseMallImportResult(response.data, items.length)
    if (!parsed) {
      setConfirming(false)
      setCanRetryOriginal(true)
      setError('导入响应格式不正确；已保留原请求，请使用“重试原请求”确认结果。')
      return
    }
    setResult(parsed)
    setConfirming(false)
    onImported()
  }

  function chooseFile(file: File) {
    const sequence = ++fileSequenceRef.current
    void file.text().then((text) => {
      if (sequence !== fileSequenceRef.current) return
      try {
        const parsed = parseMallImportCSV(text)
        requestKeyRef.current = mallWeatherCreateKey()
        setRows(parsed)
        setResult(null)
        setError('')
        setConfirming(false)
        setCanRetryOriginal(false)
      } catch (cause) {
        setError(`${cause instanceof Error ? cause.message : 'CSV 解析失败'}；当前已解析内容和重试键未改变。`)
      }
    }).catch(() => {
      if (sequence === fileSequenceRef.current) setError('读取 CSV 文件失败；当前已解析内容和重试键未改变。')
    })
  }

  function abandon() {
    fileSequenceRef.current++
    requestKeyRef.current = mallWeatherCreateKey()
    if (fileInputRef.current) fileInputRef.current.value = ''
    setRows([])
    setResult(null)
    setError('')
    setConfirming(false)
    setCanRetryOriginal(false)
  }

  return <section className="workbench-panel mall-import-panel" aria-busy={submitting}>
    <div className="mall-weather-section-title"><div><strong>批量导入店铺</strong><span>仅支持 UTF-8 CSV，表头固定为 mallCode,nameCn,province,city,district,address；每次 1–200 行。</span></div></div>
    <label>CSV 文件<input ref={fileInputRef} type="file" accept=".csv,text/csv" disabled={submitting} onChange={(event) => {
      const file = event.currentTarget.files?.[0]
      if (file) chooseFile(file)
    }} /></label>
    {rows.length > 0 && <div className="mall-import-summary" role="status" aria-live="polite"><MetaItem label="已解析" value={`${rows.length} 行`} /><MetaItem label="可提交" value={`${items.length} 行`} /><MetaItem label="待修正" value={`${invalidRows.length} 行`} /></div>}
    {invalidRows.length > 0 && <ul className="mall-import-errors" role="alert">{invalidRows.map((row) => <li key={row.row}>CSV 第 {row.row} 行：{row.error}</li>)}</ul>}
    {validForSubmit && !result && <div className="mall-import-actions">
      {!confirming
        ? <button className="primary" type="button" disabled={submitting} onClick={() => { setError(''); setConfirming(true) }}>提交导入</button>
        : <div className="mall-import-confirm" role="status"><span>将逐行创建 {items.length} 个店铺。成功店铺仍需确认坐标后才能启用天气。</span><button type="button" disabled={submitting} onClick={() => setConfirming(false)}>返回检查</button><button className="primary" type="button" disabled={submitting} onClick={() => void submit()}>{submitting ? '导入中…' : '确认并提交'}</button></div>}
      {canRetryOriginal && <button type="button" disabled={submitting} onClick={() => void submit()}>重试原请求</button>}
      <button type="button" disabled={submitting} onClick={abandon}>放弃本次导入</button>
    </div>}
    {result && <MallImportResultPanel result={result} rows={rows} onAbandon={abandon} />}
    {error && <p className="mall-weather-action-message error" role="alert">{error}</p>}
  </section>
}

function MallImportResultPanel({ result, rows, onAbandon }: { result: MallImportResult; rows: MallImportRow[]; onAbandon: () => void }) {
  return <div className="mall-import-result" aria-live="polite">
    <div className="mall-import-summary"><MetaItem label="已创建" value={`${result.created} 行`} /><MetaItem label="幂等重放" value={`${result.replayed} 行`} /><MetaItem label="失败" value={`${result.failed} 行`} /></div>
    <ul className="mall-import-result-rows">
      {result.rows.map((row) => <li key={row.row} data-status={row.status}>
        <strong>{row.status === 'CREATED' ? '已创建' : row.status === 'REPLAYED' ? '已确认创建' : '未创建'}</strong>
        <span>CSV 第 {rows[row.row - 1]?.row ?? row.row + 1} 行</span>
        {row.mallCode && <span>{row.mallCode}</span>}
        {row.reviewStatus && <span>后续：{row.reviewStatus === 'PENDING_GEOCODE' ? '等待确认坐标' : row.reviewStatus}</span>}
        {row.errorCode && <span>{mallImportErrorCodeLabel(row.errorCode)}</span>}
      </li>)}
    </ul>
    <button type="button" onClick={onAbandon}>开始下一次导入</button>
  </div>
}

function importSubmissionError(status: number) {
  if (status === 0) return '导入结果暂不确定；已保留原请求，请点击“重试原请求”确认。'
  if (status === 409) return '导入请求正在处理或发生冲突；已保留原请求，请点击“重试原请求”确认。'
  if (status === 403) return '当前账号缺少 mall.write 权限。'
  if (status === 413) return '导入文件或请求内容超过服务端限制。'
  if (status === 422) return '导入内容校验失败，请检查 CSV 后重新选择有效文件。'
  return weatherActionError(status, '批量导入失败', '当前账号缺少 mall.write 权限')
}

function mallImportErrorCodeLabel(code: 'INVALID_INPUT' | 'CONFLICT' | 'UNAVAILABLE') {
  if (code === 'INVALID_INPUT') return '字段校验未通过'
  if (code === 'CONFLICT') return '商场编码或幂等状态冲突'
  return '服务暂不可用，可稍后重新提交本文件'
}

function pendingCreateInput(pending: MallWeatherPendingCreate | null): MallWeatherCreateInput {
  if (!pending) return emptyMallCreateInput
  return {
    mallCode: pending.body.mallCode,
    nameCn: pending.body.nameCn,
    province: pending.body.province,
    city: pending.body.city,
    district: pending.body.district || '',
    address: pending.body.address,
  }
}

function MallCreatePanel({ actorID, client, onCreated, onCancel }: {
  actorID: string
  client: MallWeatherApiClient
  onCreated: (mall: MallWeatherMall) => void
  onCancel: () => void
}) {
  const restored = useMemo(() => loadMallWeatherPendingCreate(actorID, window.sessionStorage), [actorID])
  const [form, setForm] = useState<MallWeatherCreateInput>(() => pendingCreateInput(restored))
  const [pending, setPending] = useState<MallWeatherPendingCreate | null>(restored)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  function change(field: keyof MallWeatherCreateInput, value: string) {
    setForm((current) => ({ ...current, [field]: value }))
    setPending(null)
    clearMallWeatherPendingCreate(actorID, window.sessionStorage)
    setError('')
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    let request = pending
    if (!request) {
      try {
        request = { key: mallWeatherCreateKey(), body: mallWeatherCreateRequest(form) }
      } catch {
        setError('请完整填写商场编码、名称、省市和地址；编码仅支持字母、数字、下划线和连字符。')
        return
      }
      setPending(request)
      saveMallWeatherPendingCreate(actorID, request, window.sessionStorage)
    }
    setSubmitting(true)
    setError('')
    const response = await client('/v1/malls', {
      method: 'POST', body: request.body, headers: { 'Idempotency-Key': request.key }, showResult: false, silentLoading: true,
    })
    setSubmitting(false)
    if (!response.ok) {
      if (response.status === 0 || response.status === 409) {
        setError(response.status === 0 ? '创建结果暂不确定，已保留原请求；请点击“重试原请求”确认。' : '创建请求正在处理或发生冲突，请先重试原请求；仍失败时再修改表单。')
      } else {
        setPending(null)
        clearMallWeatherPendingCreate(actorID, window.sessionStorage)
        setError(weatherActionError(response.status, '商场创建失败', '当前账号缺少 mall.write 权限'))
      }
      return
    }
    const created = parseMallWeatherCreateResult(response.data)
    if (!created) {
      setError('商场已提交，但响应格式不正确；请刷新列表确认结果。')
      return
    }
    setPending(null)
    clearMallWeatherPendingCreate(actorID, window.sessionStorage)
    onCreated({
      id: created.id,
      mallCode: created.mallCode,
      nameCn: request.body.nameCn,
      province: request.body.province,
      city: request.body.city,
      district: request.body.district || '',
      address: request.body.address,
      coordinateSystem: '',
      geocodeStatus: created.geocodeStatus,
      weatherEnabled: false,
      detailProfile: request.body.weather.detailProfile,
      coverageRadiusM: request.body.weather.coverageRadiusM,
      timeZone: 'Asia/Shanghai',
      status: created.status,
      version: created.version,
    })
  }

  return (
    <section className="workbench-panel mall-weather-onboarding-panel" id="mall-weather-create-panel">
      <div className="mall-weather-section-title"><div><strong>新增商场</strong><span>创建后继续确认坐标并启用天气</span></div><span>天气口径：full · 1000 m</span></div>
      <form className="mall-weather-create-form" onSubmit={submit} aria-busy={submitting}>
        <MallDetailsFields form={form} onChange={change} disabled={submitting} />
        <div className="mall-weather-form-actions mall-weather-form-wide">
          <button className="primary" type="submit" disabled={submitting}>{submitting ? '提交中' : pending ? '重试原请求' : '创建并继续'}</button>
          <button type="button" onClick={onCancel} disabled={submitting}>取消</button>
        </div>
      </form>
      {error && <p className="mall-weather-action-message error" role="alert">{error}</p>}
    </section>
  )
}

function MallOnboardingPanel({ mall, client, onMallUpdated, onReloadMall }: {
  mall: MallWeatherMall
  client: MallWeatherApiClient
  onMallUpdated: (mall: MallWeatherMall) => void
  onReloadMall: (mallID: number) => Promise<MallWeatherMall | null>
}) {
  const [candidates, setCandidates] = useState<MallWeatherGeocodeCandidates | null>(null)
  const [candidateState, setCandidateState] = useState<LoadState>('idle')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [selectedCandidateID, setSelectedCandidateID] = useState(0)
  const [longitude, setLongitude] = useState('')
  const [latitude, setLatitude] = useState('')
  const [reason, setReason] = useState('基于高德候选人工调整商场坐标')
  const candidateRequestSequence = useRef(0)
  const candidateAbort = useRef<AbortController | null>(null)
  const cancelCandidateRequests = useCallback(() => {
    candidateRequestSequence.current++
    candidateAbort.current?.abort()
  }, [])

  const loadCandidates = useCallback(async () => {
    const sequence = ++candidateRequestSequence.current
    candidateAbort.current?.abort()
    const controller = new AbortController()
    candidateAbort.current = controller
    setCandidateState('loading')
    setError('')
    const response = await client(mallWeatherGeocodeCandidatesPath(mall.id), { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal })
    if (sequence !== candidateRequestSequence.current) return false
    if (!response.ok) {
      setCandidateState('error')
      setError(weatherActionError(response.status, '坐标候选加载失败', '当前账号缺少 mall.read 权限'))
      return false
    }
    const parsed = parseMallWeatherGeocodeCandidates(response.data)
    if (!parsed) {
      setCandidateState('error')
      setError('坐标候选响应格式不正确，请联系管理员')
      return false
    }
    setCandidates(parsed)
    const defaultCandidate = parsed.items.find((candidate) => candidate.selected) || parsed.items[0]
    setSelectedCandidateID(defaultCandidate?.id || 0)
    setLongitude(defaultCandidate ? String(defaultCandidate.longitude) : '')
    setLatitude(defaultCandidate ? String(defaultCandidate.latitude) : '')
    if (mallWeatherGeocodeRunTerminal(parsed.runStatus)) await onReloadMall(mall.id)
    setCandidateState('success')
    return true
  }, [client, mall.id, onReloadMall])

  useEffect(() => {
    void loadCandidates()
    return cancelCandidateRequests
  }, [cancelCandidateRequests, loadCandidates, mall.version])

  useEffect(() => {
    if (!mallWeatherShouldPollGeocode(mall.geocodeStatus, candidates, candidateState === 'loading')) return
    const timer = window.setTimeout(() => void loadCandidates(), 5000)
    return () => window.clearTimeout(timer)
  }, [candidateState, candidates, loadCandidates, mall.geocodeStatus])

  async function triggerGeocode() {
    setSubmitting(true)
    setError('')
    const outcome = await submitMallWeatherGeocodeTrigger(
      client,
      mall.id,
      () => onReloadMall(mall.id),
      loadCandidates,
    )
    if (outcome.kind === 'latest_mall_unavailable') {
      setSubmitting(false)
      setError('无法获取商场最新状态，请检查网络后重试。')
      return
    }
    if (outcome.kind === 'conflict') {
      setSubmitting(false)
      setError(outcome.refreshed
        ? '商场状态已更新，请再次点击“重新解析地址”。'
        : '商场版本已变化，但最新状态刷新失败，请检查网络后重试。')
      return
    }
    if (outcome.kind === 'rejected') {
      setSubmitting(false)
      setError(weatherActionError(outcome.response.status, '坐标解析任务提交失败', '当前账号缺少 mall.write 权限'))
      return
    }
    const candidatesLoaded = await loadCandidates()
    const latestMall = await onReloadMall(mall.id)
    setSubmitting(false)
    if (!latestMall || !candidatesLoaded) setError('坐标解析任务已提交，但最新状态刷新失败；页面会继续尝试刷新。')
  }

  function selectCandidate(candidateID: number) {
    const candidate = candidates?.items.find((item) => item.id === candidateID)
    if (!candidate) return
    setSelectedCandidateID(candidate.id)
    setLongitude(String(candidate.longitude))
    setLatitude(String(candidate.latitude))
    setReason('基于高德候选人工调整商场坐标')
    setError('')
  }

  async function confirmCoordinate() {
    const expectedMallVersion = candidates?.mallVersion || 0
    const selectedCandidate = candidates?.items.find((candidate) => candidate.id === selectedCandidateID)
    if (!selectedCandidate) {
      setError('请先选择一个高德解析候选，再确认或修改坐标。')
      return
    }
    let body: MallWeatherGeocodeConfirmRequest
    try {
      body = mallWeatherCandidateConfirmationRequest(selectedCandidate, longitude, latitude, reason, expectedMallVersion)
    } catch {
      setError('请填写有效的高德 GCJ-02 经纬度和 500 字以内的单行修改原因。')
      return
    }
    setSubmitting(true)
    setError('')
    const outcome = await submitMallWeatherGeocodeConfirmation(
      client,
      mall.id,
      mall.version,
      expectedMallVersion,
      body,
      () => onReloadMall(mall.id),
      loadCandidates,
    )
    if (outcome.kind === 'stale' || outcome.kind === 'conflict') {
      setSelectedCandidateID(0)
      setSubmitting(false)
      setError(outcome.refreshed
        ? '商场或高德候选已更新，请重新选择坐标后再确认。'
        : '商场或高德候选已变化，但最新状态刷新失败，请检查网络后重试。')
      return
    }
    if (outcome.kind === 'rejected') {
      setSubmitting(false)
      setError(weatherActionError(outcome.response.status, '坐标确认失败', '当前账号缺少 mall.geocode.confirm 权限'))
      return
    }
    setSubmitting(false)
    const updated = parseMallWeatherMall(outcome.response.data)
    if (!updated) {
      setError('坐标已提交，但响应格式不正确；请刷新商场列表确认。')
      return
    }
    onMallUpdated(updated)
  }

  return (
    <section className="workbench-panel mall-weather-onboarding-panel">
      <div className="mall-weather-section-title">
        <div><strong>{mall.nameCn} · 接入天气</strong><span>{mall.mallCode} · {mallLifecycleLabel(mall)}</span></div>
        <button type="button" onClick={() => void onReloadMall(mall.id)} disabled={submitting}>刷新状态</button>
      </div>
      <ol className="mall-weather-steps" aria-label="商场天气接入步骤">
        <li className="done"><strong>1. 商场已创建</strong><span>{mall.province} {mall.city} {mall.district} {mall.address}</span></li>
        <li className={mall.geocodeStatus.toLowerCase() === 'confirmed' ? 'done' : 'current'} aria-current={mall.geocodeStatus.toLowerCase() === 'confirmed' ? undefined : 'step'}><strong>2. 确认坐标</strong><span>先读取高德 GCJ-02 候选，再确认或修改</span></li>
        <li className={mall.geocodeStatus.toLowerCase() === 'confirmed' && !mall.weatherEnabled ? 'current' : ''} aria-current={mall.geocodeStatus.toLowerCase() === 'confirmed' && !mall.weatherEnabled ? 'step' : undefined}><strong>3. 启用天气</strong><span>确认坐标时同步启用并创建首次采集任务</span></li>
      </ol>

      <div className="mall-weather-geocode-actions">
        <div><strong>高德地址解析</strong><span>任务状态：{candidates?.runStatus || mall.geocodeStatus || '等待处理'} · 输出坐标系 GCJ-02</span></div>
        <div className="mall-weather-form-actions">
          <button type="button" onClick={() => void loadCandidates()} disabled={candidateState === 'loading' || submitting}>{candidateState === 'loading' ? '加载中' : '刷新候选'}</button>
          <button type="button" onClick={() => void triggerGeocode()} disabled={submitting || candidateState === 'loading'}>{submitting ? '处理中' : '重新解析地址'}</button>
        </div>
      </div>
      {candidates && candidates.items.length > 0 && <div className="mall-weather-candidates">
        {candidates.items.map((candidate) => <article key={candidate.id}>
          <div><strong>候选 {candidate.candidateNo}</strong><span>置信度 {candidate.confidenceScore.toFixed(0)}% · {candidate.level || '层级未知'}</span></div>
          <p>{candidate.formattedAddress}</p>
          <small>{candidate.longitude.toFixed(6)}, {candidate.latitude.toFixed(6)} · 高德 {candidate.coordinateSystem}</small>
          <button className={candidate.id === selectedCandidateID ? 'primary' : ''} type="button" onClick={() => selectCandidate(candidate.id)} disabled={submitting || candidateState === 'loading'}>{candidate.id === selectedCandidateID ? '已选择，可在下方修改' : '选择此高德坐标'}</button>
        </article>)}
      </div>}
      {candidateState === 'success' && candidates?.items.length === 0 && <p className="mall-weather-action-message">高德暂未返回候选。请确认地址完整、AMAP_WEB_SERVICE_KEY 已配置且天气 Worker 已启用，然后重新解析地址。</p>}

      {selectedCandidateID > 0 && <form className="mall-weather-manual-coordinate" onSubmit={(event) => { event.preventDefault(); void confirmCoordinate() }}>
        <div className="mall-weather-section-title"><div><strong>确认或修改高德坐标</strong><span>默认带入所选高德候选；如位置有偏差，可修改后再确认</span></div><span>GCJ-02</span></div>
        <label><span>高德经度</span><input name="longitude" inputMode="decimal" value={longitude} onChange={(event) => setLongitude(event.currentTarget.value)} required disabled={submitting || candidateState === 'loading'} /></label>
        <label><span>高德纬度</span><input name="latitude" inputMode="decimal" value={latitude} onChange={(event) => setLatitude(event.currentTarget.value)} required disabled={submitting || candidateState === 'loading'} /></label>
        <label className="mall-weather-form-wide"><span>修改原因</span><input name="reason" value={reason} onChange={(event) => setReason(event.currentTarget.value)} maxLength={500} disabled={submitting || candidateState === 'loading'} /></label>
        <button className="primary mall-weather-form-wide" type="submit" disabled={submitting || candidateState === 'loading'}>{submitting ? '确认中' : '确认高德坐标并启用天气'}</button>
      </form>}
      {error && <p className="mall-weather-action-message error" role="alert">{error}</p>}
    </section>
  )
}

function MallCoordinateAdjustmentPanel({ mall, client, onMallUpdated }: {
  mall: MallWeatherMall
  client: MallWeatherApiClient
  onMallUpdated: (mall: MallWeatherMall) => void
}) {
  const [editing, setEditing] = useState(false)
  const [longitude, setLongitude] = useState(String(mall.longitude))
  const [latitude, setLatitude] = useState(String(mall.latitude))
  const [reason, setReason] = useState('人工调整高德商场坐标')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    let body: unknown
    try {
      body = mallWeatherCoordinateAdjustmentRequest(mall, longitude, latitude, reason)
    } catch {
      setError('请填写有效的高德 GCJ-02 经纬度和 500 字以内的单行修改原因。')
      return
    }
    setSubmitting(true)
    setError('')
    const response = await client(mallWeatherGeocodeConfirmPath(mall.id), { method: 'POST', body, showResult: false, silentLoading: true })
    setSubmitting(false)
    if (!response.ok) {
      setError(weatherActionError(response.status, '高德坐标修改失败', '当前账号缺少 mall.geocode.confirm 权限'))
      return
    }
    const updated = parseMallWeatherMall(response.data)
    if (!updated) {
      setError('坐标已提交，但响应格式不正确；请刷新商场列表确认。')
      return
    }
    onMallUpdated(updated)
  }

  return (
    <section className="workbench-panel mall-weather-onboarding-panel">
      <div className="mall-weather-section-title">
        <div><strong>高德商场坐标</strong><span>{Number(mall.longitude).toFixed(6)}, {Number(mall.latitude).toFixed(6)} · GCJ-02</span></div>
        <button type="button" onClick={() => { setEditing((current) => !current); setError('') }} disabled={submitting}>{editing ? '取消修改' : '修改坐标'}</button>
      </div>
      {editing && <form className="mall-weather-manual-coordinate" onSubmit={submit}>
        <label><span>高德经度</span><input name="longitude" inputMode="decimal" value={longitude} onChange={(event) => setLongitude(event.currentTarget.value)} required disabled={submitting} /></label>
        <label><span>高德纬度</span><input name="latitude" inputMode="decimal" value={latitude} onChange={(event) => setLatitude(event.currentTarget.value)} required disabled={submitting} /></label>
        <label className="mall-weather-form-wide"><span>修改原因</span><input name="reason" value={reason} onChange={(event) => setReason(event.currentTarget.value)} maxLength={500} required disabled={submitting} /></label>
        <button className="primary mall-weather-form-wide" type="submit" disabled={submitting}>{submitting ? '保存中' : '保存高德坐标'}</button>
      </form>}
      {error && <p className="mall-weather-action-message error" role="alert">{error}</p>}
    </section>
  )
}

function MallWeatherSheetPushPanel({ actorID, mall, client }: {
  actorID: string
  mall: MallWeatherMall
  client: MallWeatherApiClient
}) {
  const restored = useMemo(() => loadMallWeatherPendingSheetPush(actorID, mall.id, window.sessionStorage), [actorID, mall.id])
  const [options, setOptions] = useState<MallWeatherSheetPushOption[]>([])
  const [selectedDestinationID, setSelectedDestinationID] = useState(restored?.body.destinationId || 0)
  const [optionState, setOptionState] = useState<LoadState>('loading')
  const [dryRun, setDryRun] = useState<MallWeatherSheetPushDryRun | null>(null)
  const [pending, setPending] = useState<MallWeatherPendingSheetPush | null>(restored)
  const [checking, setChecking] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [monitoring, setMonitoring] = useState(false)
  const [run, setRun] = useState<MallWeatherSheetPushRun | null>(null)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const optionRequestSequence = useRef(0)
  const pollController = useRef<AbortController | null>(null)

  const loadOptions = useCallback(async (signal?: AbortSignal) => {
    const sequence = ++optionRequestSequence.current
    setOptionState('loading')
    setError('')
    const response = await client('/v1/weather-sheet-push-options', { method: 'GET', showResult: false, silentLoading: true, signal })
    if (signal?.aborted || sequence !== optionRequestSequence.current) return null
    if (!response.ok) {
      setOptionState('error')
      setError(weatherPushError(response.status, '已有推送目标加载失败'))
      return null
    }
    const parsed = parseMallWeatherSheetPushOptions(response.data)
    if (!parsed) {
      setOptionState('error')
      setError('已有推送目标响应格式不正确，请联系管理员')
      return null
    }
    setOptions(parsed)
    setSelectedDestinationID((current) => parsed.some((option) => option.destinationId === current) ? current : parsed[0]?.destinationId || 0)
    setOptionState('success')
    return parsed
  }, [client])

  useEffect(() => {
    const controller = new AbortController()
    void loadOptions(controller.signal)
    return () => controller.abort()
  }, [loadOptions])

  useEffect(() => () => pollController.current?.abort(), [])

  const selectedOption = options.find((option) => option.destinationId === selectedDestinationID)
  const pendingMatchesOption = Boolean(selectedOption && pending && mallWeatherSheetPushRequestMatchesOption(pending.body, selectedOption, mall.id))

  function changeOption(value: string) {
    const destinationID = Number(value)
    if (!Number.isSafeInteger(destinationID) || destinationID <= 0) return
    setSelectedDestinationID(destinationID)
    setDryRun(null)
    setError(pending ? '已保留结果待确认的原请求；如需改用新目标，请先明确放弃原请求，再重新验证绑定。' : '')
    setMessage('')
  }

  async function checkBinding() {
    if (!selectedOption) {
      setError('请选择一个可用的已有推送目标。')
      return
    }
    const body = mallWeatherSheetPushRequest(selectedOption, mall.id)
    setChecking(true)
    setError('')
    setMessage('')
    const response = await client('/v1/weather-sheet-pushes/dry-run', { method: 'POST', body, showResult: false, silentLoading: true })
    setChecking(false)
    if (!response.ok) {
      setDryRun(null)
      setError(weatherPushError(response.status, '推送绑定验证失败'))
      return
    }
    const parsed = parseMallWeatherSheetPushDryRun(response.data)
    if (!parsed || parsed.destinationId !== selectedOption.destinationId || parsed.profileId !== selectedOption.profileId || parsed.profileVersion !== selectedOption.profileVersion) {
      setDryRun(null)
      setError('推送试算响应与所选目标不一致，请刷新目标后重试。')
      return
    }
    setDryRun(parsed)
    setMessage(parsed.canExecute ? '绑定验证通过，可以发起该商场的天气推送。' : '绑定验证完成，但目标当前不可执行。')
  }

  async function submitPush() {
    if (!selectedOption || !dryRun?.canExecute || dryRun.destinationId !== selectedOption.destinationId) {
      setError('请先完成当前目标的绑定验证。')
      return
    }
    if (pending && !pendingMatchesOption) {
      setError('保留的原请求与当前目标版本不同；请先放弃原请求，再重新验证绑定。')
      return
    }
    let request = pending
    if (!request) {
      request = { key: mallWeatherSheetPushKey(), body: mallWeatherSheetPushRequest(selectedOption, mall.id) }
      setPending(request)
      saveMallWeatherPendingSheetPush(actorID, mall.id, request, window.sessionStorage)
    }
    setSubmitting(true)
    setError('')
    setMessage('')
    const response = await client('/v1/weather-sheet-pushes', {
      method: 'POST', body: request.body, headers: { 'Idempotency-Key': request.key }, showResult: false, silentLoading: true,
    })
    if (!response.ok) {
      if (response.status === 409) {
        setDryRun(null)
        const refreshed = await loadOptions()
        const exactOption = refreshed?.find((option) => mallWeatherSheetPushRequestMatchesOption(request.body, option, mall.id))
        if (exactOption) {
          setSelectedDestinationID(exactOption.destinationId)
          setError('推送请求发生冲突，但原目标版本仍可用。已保留原请求；请重新验证绑定后重试原请求。')
        } else if (refreshed) {
          setError('原推送目标或 Profile 已删除、停用或升版。原请求仍保留；请放弃原请求，再选择最新目标重新验证。')
        } else {
          setError('推送请求发生冲突，且暂时无法确认目标版本。原请求仍保留；请稍后刷新目标。')
        }
      } else if (response.status === 0 || response.status === 408 || response.status >= 500) {
        setError('推送结果暂不确定，已保留原请求；请重新验证绑定后重试原请求确认。')
      } else {
        setPending(null)
        clearMallWeatherPendingSheetPush(actorID, mall.id, window.sessionStorage)
        setError(weatherPushError(response.status, '天气推送发起失败'))
      }
      setSubmitting(false)
      return
    }
    const result = parseMallWeatherSheetPushResult(response.data)
    if (!result) {
      setSubmitting(false)
      setError('推送已提交，但响应格式不正确。原请求已保留；请到运行记录中确认后再决定是否放弃。')
      return
    }
    if (!mallWeatherSheetPushResultMatchesRequest(result, request.body)) {
      setSubmitting(false)
      setError('推送响应与原请求的目标或 Profile 版本不一致。结果暂不确定，原请求已保留；请到运行记录中确认。')
      return
    }
    setSubmitting(false)
    setPending(null)
    clearMallWeatherPendingSheetPush(actorID, mall.id, window.sessionStorage)
    setRun({
      runId: result.runId, traceId: result.traceId, status: result.status === 'RUNNING' ? 'RUNNING' : 'PENDING',
      destinationId: result.destinationId, profileId: result.profileId, profileVersion: result.profileVersion,
      totalCount: result.estimatedRows, successCount: 0, failedCount: 0,
    })
    setMonitoring(true)
    setMessage(`推送任务 #${result.runId} 已创建，正在查询真实执行状态。`)
    pollController.current?.abort()
    const controller = new AbortController()
    pollController.current = controller
    const pollResult = await pollMallWeatherSheetPushRun(client, result.runId, {
      signal: controller.signal,
      isPageVisible: () => document.visibilityState === 'visible',
    })
    if (controller.signal.aborted) return
    setMonitoring(false)
    if (pollResult.kind === 'timed_out') {
      setError('推送任务仍在处理中，60 秒内未到达终止状态。已停止轮询，可稍后刷新页面确认。')
      return
    }
    if (pollResult.kind === 'query_error') {
      setError(weatherPushError(pollResult.status, '推送运行状态查询失败'))
      return
    }
    if (pollResult.kind !== 'terminal' || !mallWeatherSheetPushRunMatchesResult(pollResult.run, result)) {
      setError('推送运行记录与创建结果不一致，已停止轮询；请刷新页面后确认。')
      return
    }
    setRun(pollResult.run)
    if (pollResult.run.status === 'FAILED') {
      setError(`推送任务 #${pollResult.run.runId} 失败：成功 ${pollResult.run.successCount} 行，失败 ${pollResult.run.failedCount} 行。`)
      return
    }
    setMessage(`推送任务 #${pollResult.run.runId} ${pollResult.run.status === 'SUCCESS' ? '已完成' : '部分完成'}：成功 ${pollResult.run.successCount} 行，失败 ${pollResult.run.failedCount} 行。`)
  }

  function abandonPending() {
    setPending(null)
    setDryRun(null)
    clearMallWeatherPendingSheetPush(actorID, mall.id, window.sessionStorage)
    setMessage('已放弃待确认请求。为避免误推，请重新验证绑定后再创建新任务。')
    setError('')
  }

  return (
    <section className="workbench-panel mall-weather-push-panel">
      <div className="mall-weather-section-title"><div><strong>绑定已有推送目标</strong><span>以 {mall.nameCn} 作为 mallIds 过滤条件，先验证再发起推送</span></div><span>飞书天气表</span></div>
      {optionState === 'loading' && <LoadingState label="正在加载已有推送目标" />}
      {optionState === 'error' && <RequestError message={error} onRetry={() => void loadOptions()} />}
      {optionState === 'success' && options.length === 0 && <EmptyState title="没有可用推送目标" detail="请先启用 feishu_sheet 推送目标，并关联已启用的天气导出 Profile。" />}
      {optionState === 'success' && options.length > 0 && <div className="mall-weather-push-controls">
        <label><span>已有推送目标</span><select name="weatherSheetPushTarget" value={selectedDestinationID} onChange={(event) => changeOption(event.currentTarget.value)} disabled={checking || submitting || monitoring}>
          {options.map((option) => <option value={option.destinationId} key={option.destinationId}>{option.name} · {option.code} · {option.profileCode} v{option.profileVersion}</option>)}
        </select></label>
        <button type="button" onClick={() => void checkBinding()} disabled={checking || submitting || monitoring}>{checking ? '验证中' : '验证绑定'}</button>
        <button className="primary" type="button" onClick={() => void submitPush()} disabled={checking || submitting || monitoring || !dryRun?.canExecute || Boolean(pending && !pendingMatchesOption)}>{submitting ? '提交中' : pending ? '重试原请求' : '绑定并发起推送'}</button>
      </div>}
      {pending && <div className="mall-weather-pending-push" role="status">
        <span>存在结果待确认的原请求：商场 #{pending.body.filters.mallIds[0]}，目标 #{pending.body.destinationId}，Profile #{pending.body.profileId} v{pending.body.expectedProfileVersion}。{pendingMatchesOption ? ' 重试前仍需重新验证绑定。' : ' 当前选择与原请求不一致。'}</span>
        <button type="button" onClick={abandonPending} disabled={submitting}>放弃原请求</button>
      </div>}
      {dryRun && <div className="mall-weather-push-summary" aria-live="polite">
        <MetaItem label="写入模式" value={dryRun.writeMode} />
        <MetaItem label="预计行数" value={String(dryRun.totalEstimatedRows)} />
        <MetaItem label="预计单元格" value={String(dryRun.totalEstimatedCells)} />
        <MetaItem label="可执行" value={dryRun.canExecute ? '是' : '否'} />
      </div>}
      {dryRun && dryRun.warnings.length > 0 && <ul className="mall-weather-push-warnings">{dryRun.warnings.map((warning) => <li key={warning}>{warning}</li>)}</ul>}
      {dryRun && dryRun.datasets.length > 0 && <div className="mall-weather-push-datasets">
        {dryRun.datasets.map((dataset, datasetIndex) => <section key={`${dataset.datasetKind}:${datasetIndex}`}>
          <strong>{dataset.datasetKind}</strong>
          <span>预计 {dataset.estimatedRows} 行 / {dataset.estimatedCells} 单元格 · 可执行：{dataset.canExecute ? '是' : '否'}</span>
          {dataset.warnings.length > 0 && <ul className="mall-weather-push-warnings">{dataset.warnings.map((warning, warningIndex) => <li key={`${warningIndex}:${warning}`}>{warning}</li>)}</ul>}
        </section>)}
      </div>}
      {run && <div className="mall-weather-push-summary" aria-live="polite">
        <MetaItem label="运行状态" value={monitoring ? `${run.status}（查询中）` : run.status} />
        <MetaItem label="成功行数" value={String(run.successCount)} />
        <MetaItem label="失败行数" value={String(run.failedCount)} />
        <MetaItem label="总行数" value={String(run.totalCount)} />
      </div>}
      {message && <p className="mall-weather-action-message" role="status">{message}</p>}
      {error && optionState !== 'error' && <p className="mall-weather-action-message error" role="alert">{error}</p>}
    </section>
  )
}

function ManualRefreshPanel({ actorID, mall, client, onWeatherUpdated }: {
  actorID: string
  mall: MallWeatherMall
  client: MallWeatherApiClient
  onWeatherUpdated: (signal: AbortSignal) => Promise<WeatherOverviewReloadResult>
}) {
  const [pending, setPending] = useState<MallWeatherPendingRefresh | null>(() => loadMallWeatherPendingRefresh(actorID, mall.id, window.sessionStorage))
  const [reason, setReason] = useState(() => pending?.body.reason || '管理端手工刷新')
  const [submitting, setSubmitting] = useState(false)
  const [monitoring, setMonitoring] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const operationController = useRef<AbortController | null>(null)
  const reasonHelpID = `mall-weather-refresh-reason-help-${actorID}-${mall.id}`

  useEffect(() => () => operationController.current?.abort(), [])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    let request = pending
    if (!request) {
      let body: MallWeatherRefreshRequest
      try {
        body = mallWeatherRefreshRequest(['V26_FULL'], reason)
      } catch {
        setError('请填写单行刷新原因，最多 500 个字符')
        setMessage('')
        return
      }
      request = { key: mallWeatherRefreshKey(), body }
      saveMallWeatherPendingRefresh(actorID, mall.id, request, window.sessionStorage)
      setPending(request)
    }
    operationController.current?.abort()
    const controller = new AbortController()
    operationController.current = controller
    setSubmitting(true)
    setError('')
    setMessage('')
    try {
      const response = await client(mallWeatherRefreshPath(mall.id), {
        method: 'POST',
        body: request.body,
        headers: { 'Idempotency-Key': request.key },
        showResult: false,
        silentLoading: true,
        signal: controller.signal,
      })
      if (controller.signal.aborted) return
      setSubmitting(false)
      const disposition = mallWeatherRefreshDisposition(response, actorID, mall.id, request.body)
      if (disposition.kind === 'rejected') {
        clearMallWeatherPendingRefresh(actorID, mall.id, window.sessionStorage)
        setPending(null)
        setError(weatherRequestError(response.status, '天气刷新任务提交失败', '当前账号缺少 weather.refresh 权限'))
        return
      }
      if (disposition.kind === 'uncertain') {
        setError('刷新响应暂不确定；已保留原请求，请使用“重试原请求”确认。')
        return
      }
      clearMallWeatherPendingRefresh(actorID, mall.id, window.sessionStorage)
      setPending(null)
      const queued = disposition.result.kinds.some((item) => item.status === 'QUEUED')
      if (!queued) {
        const reloaded = await onWeatherUpdated(controller.signal)
        if (controller.signal.aborted) return
        if (reloaded === 'failed') {
          setError('数据仍然新鲜，但页面重新加载失败，请点击“重新加载”重试。')
          return
        }
        if (reloaded === 'waiting') {
          setMessage('数据仍然新鲜，页面正在等待未来逐小时温度同步。')
          return
        }
        if (reloaded === 'aborted') return
        setMessage(mallWeatherRefreshResultMessage(disposition.result))
        return
      }

      setMonitoring(true)
      setMessage('采集任务已入队，正在等待彩云综合天气数据写入…')
      const pollResult = await pollMallWeatherFetchRun(
        client,
        mall.id,
        disposition.result.requestedAt,
        'MANUAL',
        disposition.result.correlationId,
        { signal: controller.signal },
      )
      if (controller.signal.aborted) return
      if (pollResult.kind === 'timed_out') {
        setError('任务已入队，但 60 秒内未发现完成记录。请确认 MALL_WEATHER_ENABLED=true 且 weather 队列消费进程正在运行，然后重试。')
        setMessage('')
        return
      }
      if (pollResult.kind === 'query_error') {
        setError(weatherRequestError(pollResult.status, '天气采集状态查询失败', '当前账号缺少 weather.read 权限'))
        setMessage('')
        return
      }
      if (pollResult.kind !== 'terminal') return
      if (pollResult.run.status !== 'SUCCESS' && pollResult.run.status !== 'PARTIAL_SUCCESS') {
        setError(weatherFetchRunFailureMessage(pollResult.run))
        setMessage('')
        return
      }
      const hourlyRows = pollResult.run.rowCounts.hourly ?? 0
      if (hourlyRows < 1) {
        setError('采集任务已完成，但没有写入未来逐小时数据。请查看任务的解析告警和服务端日志。')
        setMessage('')
        return
      }
      const reloaded = await onWeatherUpdated(controller.signal)
      if (controller.signal.aborted) return
      if (reloaded === 'failed') {
        setError('采集已完成，但天气数据重新加载失败，请点击页面上的“重新加载”重试。')
        setMessage('')
        return
      }
      if (reloaded === 'waiting') {
        setMessage(`采集已完成并写入 ${hourlyRows} 条逐小时数据，页面正在等待温度读模型同步。`)
        return
      }
      if (reloaded === 'aborted') return
      setMessage(`天气已更新并重新加载，共写入 ${hourlyRows} 条未来逐小时数据。`)
    } catch {
      if (controller.signal.aborted) return
      setError('天气采集状态查询异常，请检查网络后重试。')
      setMessage('')
    } finally {
      if (operationController.current === controller) {
        operationController.current = null
        if (!controller.signal.aborted) {
          setSubmitting(false)
          setMonitoring(false)
        }
      }
    }
  }

  function changeReason(value: string) {
    setReason(value)
  }

  return (
    <section className="workbench-panel mall-weather-refresh-panel">
      <div className="mall-weather-section-title"><div><strong>手工刷新</strong><span>提交异步采集任务，不阻塞等待供应商</span></div><RefreshCcw aria-hidden="true" /></div>
      <form className="mall-weather-refresh-form" onSubmit={submit} aria-busy={submitting || monitoring}>
        <label><span>采集范围</span><input name="weatherRefreshScope" value="综合天气（含实况、分钟、小时、逐日、预警、生活指数）" disabled />
          <small>固定提交全部天气数据类型</small>
        </label>
        <label><span>刷新原因</span><input name="weatherRefreshReason" value={reason} onChange={(event) => changeReason(event.currentTarget.value)} disabled={submitting || monitoring || Boolean(pending)} aria-describedby={reasonHelpID} />
          <small id={reasonHelpID}>必填单行文本，最多 500 个字符</small>
        </label>
        <div className="mall-weather-refresh-submit">
          <span>操作</span>
          <button className="primary" type="submit" disabled={submitting || monitoring}>
            {submitting ? '提交中' : monitoring ? '等待采集完成' : pending ? '重试原请求' : '提交刷新'}
          </button>
          <small>提交后异步执行并跟踪结果</small>
        </div>
      </form>
      {message && <p className="mall-weather-action-message" role="status">{message}</p>}
      {error && <p className="mall-weather-action-message error" role="alert">{error}</p>}
    </section>
  )
}

function weatherFetchRunFailureMessage(run: MallWeatherFetchRun) {
  const detail = run.errorMessageSafe || run.errorCode || '未返回安全错误信息'
  return `天气采集失败：${detail}`
}

function WeatherRealtime({ mall, overview }: { mall: MallWeatherMall; overview: MallWeatherOverview }) {
  const { realtime } = overview
  const [downloadError, setDownloadError] = useState('')

  function downloadCsv() {
    setDownloadError('')
    try {
      downloadMallWeatherBytes(
        createMallWeatherDatasetCsv('realtime', realtime ? [realtime] : [], { mallCode: mall.mallCode, mallName: mall.nameCn }),
        mallWeatherCsvFileName('realtime', mall.mallCode),
      )
    } catch {
      setDownloadError('实况 CSV 文件生成失败，请重新加载后重试。')
    }
  }

  return (
    <article className="workbench-panel mall-weather-realtime" aria-label="当前实况天气">
      <div className="mall-weather-section-title">
        <div><strong>当前实况</strong><span>{mall.nameCn} · {realtime?.snapshotAtLocal || '暂无快照时间'}</span></div>
        <button type="button" onClick={downloadCsv} aria-label={`下载${mall.nameCn}当前实况 CSV`}><Download aria-hidden="true" />下载 CSV</button>
      </div>
      {downloadError && <p className="mall-weather-action-message error" role="alert">{downloadError}</p>}
      {realtime ? (
        <>
          <div className="mall-weather-temperature"><strong>{mallWeatherMetric(realtime.temperatureC, '°C')}</strong><span>{mallWeatherSkyconLabel(realtime.skycon)}</span></div>
          <div className="mall-weather-metrics">
            <MetaItem label="体感" value={mallWeatherMetric(realtime.apparentTemperatureC, '°C')} />
            <MetaItem label="湿度" value={mallWeatherMetric(realtime.humidityPct, '%', 0)} />
            <MetaItem label="风速" value={mallWeatherMetric(realtime.windSpeedKph, ' km/h')} />
            <MetaItem label="风向" value={mallWeatherMetric(realtime.windDirectionDeg, '°', 0)} />
            <MetaItem label="气压" value={mallWeatherMetric(realtime.pressurePa, ' Pa', 0)} />
            <MetaItem label="云量" value={weatherRatioPercent(realtime.cloudrateRatio)} />
            <MetaItem label="短波辐射" value={mallWeatherMetric(realtime.dswrfWM2, ' W/m²')} />
            <MetaItem label="能见度" value={mallWeatherMetric(realtime.visibilityKm, ' km')} />
            <MetaItem label="本地降水" value={mallWeatherMetric(realtime.localPrecipitationMmH, ' mm/h')} />
            <MetaItem label="本地降水状态" value={[realtime.localPrecipitationStatus, realtime.localPrecipitationSource].filter(Boolean).join(' · ') || '暂无'} />
            <MetaItem label="最近降水" value={realtime.nearestPrecipitationDistanceKm === undefined && realtime.nearestPrecipitationMmH === undefined
              ? realtime.nearestPrecipitationStatus || '暂无'
              : `${mallWeatherMetric(realtime.nearestPrecipitationDistanceKm, ' km')} · ${mallWeatherMetric(realtime.nearestPrecipitationMmH, ' mm/h')}`} />
            <MetaItem label="质量" value={`${realtime.qualityStatus || '未知'}${realtime.qualityWarnings.length ? ` · ${realtime.qualityWarnings.length} 项告警` : ''}`} />
            <MetaItem label="舒适度" value={[realtime.comfortIndex, realtime.comfortDescription].filter((value) => value !== undefined && value !== '').join(' · ') || '暂无'} />
            <MetaItem label="紫外线" value={[realtime.ultravioletIndex, realtime.ultravioletDescription].filter((value) => value !== undefined && value !== '').join(' · ') || '暂无'} />
          </div>
          <p className="mall-weather-caption">供应商时间 {realtime.providerServerTimeLocal || '—'} · 采集时间 {realtime.fetchedAtLocal || '—'}</p>
        </>
      ) : <EmptyState title="暂无实况" detail="最近一次采集尚未产生可用实况。" />}
    </article>
  )
}

function WeatherOverviewDetails({ mall, overview, alerts, refreshing, onRefresh, onAlertsRetry }: {
  mall: MallWeatherMall
  overview: MallWeatherOverview
  alerts: AlertDataSnapshot
  refreshing: boolean
  onRefresh: () => void
  onAlertsRetry: () => void
}) {
  const { realtime, meta } = overview
  const [alertDownloadError, setAlertDownloadError] = useState('')
  const representativePoint = ['MALL_CENTER', 'CENTER', 'center'].includes(meta.representativePoint) ? '商场中心点' : meta.representativePoint || '口径缺失'
  const coverageRadius = meta.coverageRadiusM > 0 ? `业务半径 ${meta.coverageRadiusM} m` : '业务半径口径缺失'

  function downloadAlertsCsv() {
    setAlertDownloadError('')
    try {
      downloadMallWeatherBytes(
        createMallWeatherDatasetCsv('alerts', alerts.items, { mallCode: mall.mallCode, mallName: mall.nameCn }),
        mallWeatherCsvFileName('alerts', mall.mallCode),
      )
    } catch {
      setAlertDownloadError('气象预警 CSV 文件生成失败，请重新查询后重试。')
    }
  }

  return (
    <div className="view-stack">
      <section className="workbench-panel" aria-label="空气质量">
        <div className="mall-weather-section-title"><div><strong>空气质量</strong><span>{realtime?.aqiDescriptionChn || '中国 AQI 标准'}</span></div><Wind aria-hidden="true" /></div>
        <div className="mall-weather-aqi"><strong>{mallWeatherMetric(realtime?.aqiChn, '', 0)}</strong><span>AQI</span></div>
        <div className="mall-weather-metrics compact">
          <MetaItem label="PM2.5" value={mallWeatherMetric(realtime?.pm25UgM3, ' μg/m³')} />
          <MetaItem label="PM10" value={mallWeatherMetric(realtime?.pm10UgM3, ' μg/m³')} />
          <MetaItem label="O₃" value={mallWeatherMetric(realtime?.o3UgM3, ' μg/m³')} />
          <MetaItem label="NO₂" value={mallWeatherMetric(realtime?.no2UgM3, ' μg/m³')} />
          <MetaItem label="SO₂" value={mallWeatherMetric(realtime?.so2UgM3, ' μg/m³')} />
          <MetaItem label="CO" value={mallWeatherMetric(realtime?.coMgM3, ' mg/m³')} />
          <MetaItem label="美国 AQI" value={`${mallWeatherMetric(realtime?.aqiUsa, '', 0)}${realtime?.aqiDescriptionUsa ? ` · ${realtime.aqiDescriptionUsa}` : ''}`} />
        </div>
      </section>

      <section className="workbench-panel mall-weather-summary">
        <div className="mall-weather-summary-heading">
          <div>
            <span className="eyebrow">{mall.mallCode} · {mall.city}</span>
            <h3>{mall.nameCn}</h3>
            <p><MapPin aria-hidden="true" />代表点：{representativePoint} · {coverageRadius}</p>
          </div>
          <button type="button" onClick={onRefresh} disabled={refreshing}><RefreshCcw aria-hidden="true" />{refreshing ? '加载中' : '重新加载'}</button>
        </div>
        <div className="mall-weather-meta" aria-label="天气数据口径">
          <MetaItem label="供应商" value={`${meta.provider || '彩云天气'} ${meta.apiVersion}`.trim()} />
          <MetaItem label="新鲜度" value={mallWeatherFreshnessLabel(meta.freshnessStatus)} />
          <MetaItem label="坐标" value={meta.longitude === 0 && meta.latitude === 0 ? '坐标未提供' : `${meta.longitude.toFixed(4)}, ${meta.latitude.toFixed(4)} ${meta.coordinateSystem}`} />
          <MetaItem label="时区 / 单位" value={`${meta.timeZone || 'Asia/Shanghai'} · ${meta.unit || 'metric:v2'}`} />
        </div>
        <p className="mall-weather-resolution">实况与未来两小时降水为商场中心点 1 km 级数据；常规小时预报为商场中心点所在 9～13 km 预报网格。预警按行政区域发布。</p>
      </section>

      <section className="content-grid two">
        <MallWeatherChart
          title="未来 120 分钟降水"
          detail="1 km 级"
          unit="mm/h"
          icon={<CloudRain aria-hidden="true" />}
          series={overview.minutely.map((item) => ({ time: item.forecastMinuteLocal, value: item.precipitationMmH }))}
        />
        <MallWeatherChart
          title="未来 24 小时温度"
          detail="9～13 km 预报网格"
          unit="°C"
          icon={<Thermometer aria-hidden="true" />}
          series={overview.hourly.map((item) => ({ time: item.forecastTimeLocal, value: item.temperatureC }))}
        />
      </section>

      <section className="workbench-panel" id="mall-weather-alerts" tabIndex={-1} aria-busy={alerts.loading}>
        <div className="mall-weather-section-title">
          <div><strong>气象预警</strong><span>行政区域口径 · 自动读取全部游标页</span></div>
          <button type="button" onClick={downloadAlertsCsv} disabled={alerts.loading || Boolean(alerts.error)} aria-label={`下载${mall.nameCn}气象预警 CSV`}>
            <Download aria-hidden="true" />下载 CSV
          </button>
        </div>
        {alertDownloadError && <p className="mall-weather-action-message error" role="alert">{alertDownloadError}</p>}
        {alerts.loading && <LoadingState label="正在加载全部气象预警" />}
        {alerts.error && <RequestError message={alerts.error} onRetry={onAlertsRetry} />}
        {alerts.ready && (alerts.items.length === 0
          ? <EmptyState title="当前无有效预警" detail="当前 31 天查询窗口没有返回有效气象预警。" />
          : <div className="mall-weather-alerts">
            {alerts.items.map((alert) => (
              <article key={alert.alertId || alert.title}>
                <div><strong>{alert.title}</strong><span>{[alert.alertTypeName, alert.alertLevelName].filter(Boolean).join(' · ') || alert.status}</span></div>
                {alert.description && <p>{alert.description}</p>}
                <small>{alert.source || '预警发布机构'} · {alert.publishedAtLocal || '发布时间未知'}</small>
                <small>{[alert.code, alert.location, alert.adcode].filter(Boolean).join(' · ') || '区域信息未提供'}</small>
                <small>首次发现 {alert.firstSeenAtLocal || '—'} · 最近发现 {alert.lastSeenAtLocal || '—'}</small>
              </article>
            ))}
          </div>)}
      </section>
    </div>
  )
}

function weatherRatioPercent(value: number | undefined) {
  return mallWeatherMetric(value === undefined ? undefined : value * 100, '%', 0)
}

function MetaItem({ label, value }: { label: string; value: string }) {
  return <div className="mall-weather-meta-item"><span>{label}</span><strong>{value || '—'}</strong></div>
}

function RequestError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <div className="mall-weather-request-state error" role="alert"><strong>加载失败</strong><span>{message}</span><button type="button" onClick={onRetry}>重试</button></div>
}

function LoadingState({ label }: { label: string }) {
  return <div className="mall-weather-request-state" role="status" aria-busy="true"><RefreshCcw aria-hidden="true" /><span>{label}</span></div>
}

function EmptyState({ title, detail }: { title: string; detail: string }) {
  return <div className="empty-state" role="status"><strong>{title}</strong><span>{detail}</span></div>
}

function weatherRequestError(status: number, fallback: string, forbidden: string) {
  if (status === 0) return '无法连接服务，请检查网络后重试'
  if (status === 403) return forbidden
  if (status === 404) return '商场或天气数据不存在'
  if (status === 422) return '商场坐标尚未确认，暂时无法查询天气'
  return `${fallback}（HTTP ${status}）`
}

function weatherActionError(status: number, fallback: string, forbidden: string) {
  if (status === 0) return '无法连接服务，请检查网络后重试'
  if (status === 403) return forbidden
  if (status === 404) return '商场或坐标候选不存在，请刷新状态后重试'
  if (status === 409) return '商场状态已变化，请刷新后重试'
  if (status === 422) return '提交内容校验失败，请检查输入后重试'
  return `${fallback}（HTTP ${status}）`
}

function weatherPushError(status: number, fallback: string) {
  if (status === 0) return '无法连接服务，请检查网络后重试'
  if (status === 403) return '飞书天气推送未开启，或当前账号缺少 weather.feishu.push 权限'
  if (status === 404) return '所选推送目标或天气导出 Profile 不存在，请刷新目标'
  if (status === 409) return '推送目标或 Profile 已更新，请刷新后重新验证绑定'
  if (status === 422) return '该商场或推送配置未通过校验，请检查坐标、时间范围和目标配置'
  return `${fallback}（HTTP ${status}）`
}

function mallLifecycleLabel(mall: MallWeatherMall) {
  if (mall.geocodeStatus.toLowerCase() === 'failed') return '坐标解析失败'
  if (mall.geocodeStatus.toLowerCase() !== 'confirmed') return '等待确认坐标'
  if (!mall.weatherEnabled) return '天气未启用'
  if (mall.status.toLowerCase() !== 'active') return '商场未启用'
  return '可查询'
}
