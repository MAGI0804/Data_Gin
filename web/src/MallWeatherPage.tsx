import { type FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AlertTriangle, Clock3, CloudRain, CloudSun, Database, Download, MapPin, RefreshCcw, Thermometer } from 'lucide-react'
import storeStyles from './MallStoreWorkspace.module.css'
import { MallWeatherChart, type MallWeatherChartSeries } from './MallWeatherChart'
import { MallWeatherAdvancedTools } from './MallWeatherAdvancedTools'
import { MallWeatherExportPanel } from './MallWeatherExportPanel'
import { MallWeatherForecastPanel, type MallWeatherForecastDataSnapshot } from './MallWeatherForecastPanel'
import { MallWeatherManualRefresh } from './MallWeatherManualRefresh'
import { MallWeatherMallEditor } from './MallWeatherMallEditor'
import { MallWeatherSheetPushPanel } from './MallWeatherSheetPushPanel'
import { MallCreatePanel, MallImportPanel } from './MallStoreOperations'
import { Dialog, PageCanvas, PageHeader } from './ui'
import { runSingleFlight } from './singleFlight'
import {
  createMallWeatherChartCsv,
  createMallWeatherDatasetCsv,
  downloadMallWeatherBytes,
  mallWeatherChartCsvFileName,
  mallWeatherCsvFileName,
  type MallWeatherCsvZipData,
} from './mallWeatherCsv'
import { mallWeatherDataNavigationItems, navigateMallWeatherSection } from './mallWeatherNavigation'
import {
  loadAllMallWeatherAlerts,
  mallWeatherFreshnessLabel,
  mallWeatherMetric,
  mallWeatherGeocodeCandidatesPath,
  mallWeatherCandidateConfirmationRequest,
  mallWeatherCoordinateAdjustmentRequest,
  mallWeatherGeocodeConfirmPath,
  mallWeatherGeocodePollDelayMilliseconds,
  mallWeatherGeocodePollMaxAttempts,
  mallWeatherGeocodeRunTerminal,
  mallWeatherShouldPollGeocode,
  mallWeatherMallReady,
  mallWeatherOverviewHasBusinessData,
  mallWeatherOverviewReadiness,
  mergeMallWeatherMalls,
  mallWeatherOverviewPath,
  mallWeatherRealtimePath,
  mallWeatherSkyconLabel,
  parseMallWeatherMallList,
  parseMallWeatherMall,
  parseMallWeatherGeocodeCandidates,
  parseMallWeatherOverview,
  parseMallWeatherRealtimePage,
  submitMallWeatherGeocodeConfirmation,
  submitMallWeatherGeocodeTrigger,
  type MallWeatherAlert,
  type MallWeatherGeocodeConfirmRequest,
  type MallWeatherGeocodeCandidates,
  type MallWeatherMall,
  type MallWeatherOverview,
  type MallWeatherOverviewReadiness,
} from './mallWeather'
import styles from './MallWeatherPage.module.css'

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
  const mallController = useRef<AbortController | null>(null)
  const mallDetailRequestSequence = useRef(0)
  const mallDetailController = useRef<AbortController | null>(null)
  const overviewRequestSequence = useRef(0)
  const overviewController = useRef<AbortController | null>(null)
  const overviewSnapshotRef = useRef<MallWeatherOverview | null>(null)
  const overviewSnapshotMallIDRef = useRef(0)
  const overviewRetryMallIDRef = useRef(0)
  const realtimeFallbackAttemptedMallIDRef = useRef(0)
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
    mallController.current?.abort()
    const controller = new AbortController()
    mallController.current = controller
    setMallState('loading')
    setMallError('')
    const search = new URLSearchParams({ limit: '50' })
    if (afterID > 0) search.set('afterId', String(afterID))
    try {
      const response = await client(`/v1/malls?${search.toString()}`, { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal })
      if (controller.signal.aborted || sequence !== mallRequestSequence.current) return
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
        const selectedResponse = await client(`/v1/malls/${selectedID}`, { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal })
        if (controller.signal.aborted || sequence !== mallRequestSequence.current) return
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
    } catch {
      if (controller.signal.aborted || sequence !== mallRequestSequence.current) return
      setMallState('error')
      setMallError('商场列表加载异常，请检查网络后重试')
    } finally {
      if (mallController.current === controller) mallController.current = null
    }
  }, [client])

  const loadMall = useCallback(async (mallID: number) => {
    const sequence = ++mallDetailRequestSequence.current
    mallDetailController.current?.abort()
    const controller = new AbortController()
    mallDetailController.current = controller
    try {
      const response = await client(`/v1/malls/${mallID}`, { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal })
      if (controller.signal.aborted || sequence !== mallDetailRequestSequence.current || !response.ok) return null
      const parsed = parseMallWeatherMall(response.data)
      if (!parsed) return null
      setMalls((current) => mergeMallWeatherMalls(current, [parsed]))
      return parsed
    } catch {
      return null
    } finally {
      if (mallDetailController.current === controller) mallDetailController.current = null
    }
  }, [client])

  const loadOverview = useCallback(async (
    mallID: number,
    externalSignal?: AbortSignal,
    forceRealtimeFallback = false,
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
      let parsed = parseMallWeatherOverview(response.data)
      if (!parsed) {
        updateOverviewState('error')
        updateOverviewWaitingReason('')
        setOverviewError('天气概览响应格式不正确，请联系管理员')
        return 'failed'
      }
      if (parsed.realtime === null && (forceRealtimeFallback || realtimeFallbackAttemptedMallIDRef.current !== mallID)) {
        realtimeFallbackAttemptedMallIDRef.current = mallID
        const fallbackEnd = new Date()
        const fallbackStart = new Date(fallbackEnd.getTime() - 60 * 60 * 1000)
        const fallbackTimeZone = parsed.meta.timeZone.trim() || 'Asia/Shanghai'
        try {
          const fallbackResponse = await client(mallWeatherRealtimePath(mallID, fallbackStart, fallbackEnd, fallbackTimeZone), {
            method: 'GET', showResult: false, silentLoading: true, signal: controller.signal,
          })
          if (controller.signal.aborted || sequence !== overviewRequestSequence.current) {
            restoreAfterAbort()
            return 'aborted'
          }
          if (fallbackResponse.ok) {
            const fallback = parseMallWeatherRealtimePage(fallbackResponse.data)
            const latestRealtime = fallback && fallback.items.length > 0
              ? fallback.items[fallback.items.length - 1]
              : undefined
            if (fallback && latestRealtime) parsed = { ...parsed, realtime: latestRealtime, meta: fallback.meta }
          }
        } catch {
          if (controller.signal.aborted || sequence !== overviewRequestSequence.current) {
            restoreAfterAbort()
            return 'aborted'
          }
        }
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
    realtimeFallbackAttemptedMallIDRef.current = 0
    overviewLastStableRef.current = null
    setOverview(null)
    setOverviewMallID(0)
    updateOverviewState('idle')
    updateOverviewWaitingReason('')
    setOverviewError('')
    setOverviewRetryCount(0)
  }, [loadOverview, malls, selectedMallID, updateOverviewState, updateOverviewWaitingReason, view])

  useEffect(() => () => {
    mallRequestSequence.current++
    mallController.current?.abort()
    mallDetailRequestSequence.current++
    mallDetailController.current?.abort()
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
    const result = await loadOverview(mallID, signal, true)
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
      <PageCanvas className={`${storeStyles.page} ${styles.weatherPage}`}>
        <PageHeader
          eyebrow="STORE DIRECTORY"
          title="店铺信息"
          description="筛选并选择店铺，在同一工作画布中维护基础资料、地址解析和天气服务坐标。"
        />
        <section className={storeStyles.toolbar} aria-label="店铺筛选与选择">
          <div className={storeStyles.toolbarFields}>
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
          <div className={storeStyles.toolbarActions}>
            <button type="button" onClick={() => void loadMalls()} disabled={mallState === 'loading'}>
              <RefreshCcw aria-hidden="true" />刷新店铺
            </button>
            {nextAfterID > 0 && <button type="button" onClick={() => void loadMalls(nextAfterID)} disabled={mallState === 'loading'}>加载更多</button>}
            <button className={styles['primary']} type="button" onClick={() => setShowCreate((current) => !current)} aria-expanded={showCreate} aria-controls="mall-weather-create-panel">
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
          ? <div className={storeStyles.detailLayout}>
            <aside className={storeStyles.summary} aria-label={`${selectedMall.nameCn}资料摘要`}>
              <div className={storeStyles.summaryHeading}>
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
            <section className={storeStyles.maintenance} aria-label={`${selectedMall.nameCn}店铺资料维护`}>
              <div className={storeStyles.maintenanceHeading}>
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
      </PageCanvas>
    )
  }

  return (
    <PageCanvas className={`${styles.page} ${styles.weatherPage}`}>
      <PageHeader
        className={styles.pageHeader}
        eyebrow="WEATHER OPERATIONS"
        title="商场天气"
        description="选择商场后查看实况、预报、预警与生活指数，并执行刷新、导出和推送操作。"
      />
      <section className={styles.selector} aria-label="选择天气商场">
        <strong className={styles.selectorTitle}>筛选商场</strong>
        <div className={styles.selectorFields}>
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
          <label className={styles.queryField}>
            <span className={styles['sr-only']}>搜索商场</span>
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
        </div>
        <div className={styles.selectorActions}>
          <button type="button" onClick={() => void loadMalls()} disabled={mallState === 'loading'}>
            <RefreshCcw aria-hidden="true" />刷新商场
          </button>
          {nextAfterID > 0 && <button type="button" onClick={() => void loadMalls(nextAfterID)} disabled={mallState === 'loading'}>加载更多</button>}
        </div>
      </section>

      {mallState === 'error' && <RequestError message={mallError} onRetry={() => void loadMalls()} />}
      {mallState === 'loading' && malls.length === 0 && <LoadingState label="正在加载商场" />}
      {mallState === 'success' && malls.length === 0 && <EmptyState title="还没有可用商场" detail="请先到“基础信息 → 店铺信息”新增并维护店铺。" />}
      {selectedMall && !selectedMallReady && <section className={styles.unavailable}>
        <div><strong>{selectedMall.nameCn}尚未完成天气接入</strong><span>请到“基础信息 → 店铺信息”完成地址、坐标和天气启用设置。</span></div>
        <button type="button" onClick={() => { window.location.hash = 'store_info' }}>前往店铺信息</button>
      </section>}

      {selectedMallReady && selectedMall && <>
        <div className={styles.toolbar}>
          <div className={styles['mall-weather-actions-row']}>
            {actorID
              ? <MallWeatherManualRefresh
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
        </div>
        <div className={styles.dashboard} id="mall-weather-overview" tabIndex={-1}>
          {!selectedOverview && overviewState !== 'error' && overviewState !== 'waiting' && <LoadingState label={`正在加载${selectedMall.nameCn}天气`} />}
          {selectedOverview && <>
            <WeatherRealtime mall={selectedMall} overview={selectedOverview} />
            <WeatherOverviewDetails
              mall={selectedMall}
              overview={selectedOverview}
              alerts={selectedAlerts}
              refreshing={overviewState === 'loading'}
              onRefresh={() => void loadOverview(selectedMall.id)}
              onAlertsRetry={() => void loadAlerts(selectedMall.id, selectedMall.timeZone)}
            />
          </>}
          {overviewState === 'error' && <RequestError message={overviewError} onRetry={() => { setOverviewRetryCount(0); void loadOverview(selectedMall.id) }} />}
          {overviewState === 'waiting' && overviewRetryCount < 30 && <LoadingState label={overviewWaitingReason === 'waiting-empty'
            ? `首次天气采集中，正在等待实况与未来逐小时数据（${overviewRetryCount + 1}/30）`
            : `实况已加载，未来逐小时温度正在同步（${overviewRetryCount + 1}/30）`} />}
          {overviewState === 'waiting' && overviewRetryCount >= 30 && <RequestError
            message={overviewWaitingReason === 'waiting-empty'
              ? '首次采集长时间未生成数据，请确认 MALL_WEATHER_ENABLED=true 且 weather 队列消费进程正在运行。'
              : '实况已加载，但未来逐小时温度长时间不可用。请确认天气业务事务已提交，并检查最近采集记录。'}
            onRetry={() => { setOverviewRetryCount(0); void loadOverview(selectedMall.id) }}
          />}
        </div>
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
        <MallWeatherAdvancedTools client={client} />
      </>}
    </PageCanvas>
  )
}

function MallWeatherDataNavigation() {
  function navigateTo(targetID: string) {
    const reduceMotion = typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    navigateMallWeatherSection(document, targetID, reduceMotion)
  }

  return (
    <nav className={styles['mall-weather-data-nav']} aria-label="天气数据快速入口">
      <strong><Database aria-hidden="true" />天气数据</strong>
      {mallWeatherDataNavigationItems.filter((item) => ['mall-weather-overview', 'mall-weather-minutely', 'mall-weather-hourly', 'mall-weather-alerts'].includes(item.targetID)).map((item) => (
        <button type="button" aria-controls={item.targetID} onClick={() => navigateTo(item.targetID)} key={item.targetID}>
          {item.targetID === 'mall-weather-overview' ? '实况' : item.targetID === 'mall-weather-minutely' ? '降水' : item.targetID === 'mall-weather-hourly' ? '小时' : '预警'}
        </button>
      ))}
    </nav>
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
  const [coordinateConfirming, setCoordinateConfirming] = useState(false)
  const [candidatePollAttempts, setCandidatePollAttempts] = useState(0)
  const [candidatePollFailures, setCandidatePollFailures] = useState(0)
  const [candidatePollVisibilityRevision, setCandidatePollVisibilityRevision] = useState(0)
  const candidateRequestSequence = useRef(0)
  const candidateAbort = useRef<AbortController | null>(null)
  const coordinateConfirmationInFlight = useRef({ current: false })
  const cancelCandidateRequests = useCallback(() => {
    candidateRequestSequence.current++
    candidateAbort.current?.abort()
  }, [])

  const loadCandidates = useCallback(async (polling = false) => {
    const sequence = ++candidateRequestSequence.current
    candidateAbort.current?.abort()
    const controller = new AbortController()
    candidateAbort.current = controller
    if (polling) setCandidatePollAttempts((current) => current + 1)
    else setCandidatePollFailures(0)
    setCandidateState('loading')
    setError('')
    try {
      const response = await client(mallWeatherGeocodeCandidatesPath(mall.id), { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal })
      if (controller.signal.aborted || sequence !== candidateRequestSequence.current) return false
      if (!response.ok) {
        if (polling) setCandidatePollFailures((current) => current + 1)
        setCandidateState('error')
        setError(weatherActionError(response.status, '坐标候选加载失败', '当前账号缺少 mall.read 权限'))
        return false
      }
      const parsed = parseMallWeatherGeocodeCandidates(response.data)
      if (!parsed) {
        if (polling) setCandidatePollFailures((current) => current + 1)
        setCandidateState('error')
        setError('坐标候选响应格式不正确，请联系管理员')
        return false
      }
      if (polling) setCandidatePollFailures(0)
      setCandidates(parsed)
      const defaultCandidate = parsed.items.find((candidate) => candidate.selected) || parsed.items[0]
      setSelectedCandidateID(defaultCandidate?.id || 0)
      setLongitude(defaultCandidate ? String(defaultCandidate.longitude) : '')
      setLatitude(defaultCandidate ? String(defaultCandidate.latitude) : '')
      if (mallWeatherGeocodeRunTerminal(parsed.runStatus)) await onReloadMall(mall.id)
      setCandidateState('success')
      return true
    } catch {
      if (controller.signal.aborted || sequence !== candidateRequestSequence.current) return false
      if (polling) setCandidatePollFailures((current) => current + 1)
      setCandidateState('error')
      setError('坐标候选加载异常，请检查网络后重试。')
      return false
    } finally {
      if (candidateAbort.current === controller) candidateAbort.current = null
    }
  }, [client, mall.id, onReloadMall])

  useEffect(() => {
    void loadCandidates()
    return cancelCandidateRequests
  }, [cancelCandidateRequests, loadCandidates, mall.version])

  useEffect(() => {
    const handleVisibilityChange = () => setCandidatePollVisibilityRevision((current) => current + 1)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [])

  useEffect(() => {
    setCandidatePollAttempts(0)
    setCandidatePollFailures(0)
  }, [mall.id, mall.version])

  useEffect(() => {
    if (!mallWeatherShouldPollGeocode(mall.geocodeStatus, candidates, candidateState === 'loading')) return
    if (candidatePollAttempts >= mallWeatherGeocodePollMaxAttempts) {
      setError(`坐标候选自动刷新已在 ${mallWeatherGeocodePollMaxAttempts} 次后停止；请检查地址或任务状态后手动刷新。`)
      return
    }
    const delay = mallWeatherGeocodePollDelayMilliseconds(candidatePollFailures, document.visibilityState !== 'hidden')
    const timer = window.setTimeout(() => void loadCandidates(true), delay)
    return () => window.clearTimeout(timer)
  }, [candidatePollAttempts, candidatePollFailures, candidatePollVisibilityRevision, candidateState, candidates, loadCandidates, mall.geocodeStatus])

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
    setCandidatePollAttempts(0)
    setCandidatePollFailures(0)
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

  function requestCoordinateConfirmation() {
    const expectedMallVersion = candidates?.mallVersion || 0
    const selectedCandidate = candidates?.items.find((candidate) => candidate.id === selectedCandidateID)
    if (!selectedCandidate) {
      setError('请先选择一个高德解析候选，再确认或修改坐标。')
      return
    }
    try {
      mallWeatherCandidateConfirmationRequest(selectedCandidate, longitude, latitude, reason, expectedMallVersion)
    } catch {
      setError('请填写有效的高德 GCJ-02 经纬度和 500 字以内的单行修改原因。')
      return
    }
    setError('')
    setCoordinateConfirming(true)
  }

  return (
    <section className={[styles['workbench-panel'], styles['mall-weather-onboarding-panel']].join(' ')}>
      <div className={styles['mall-weather-section-title']}>
        <div><strong>{mall.nameCn} · 接入天气</strong><span>{mall.mallCode} · {mallLifecycleLabel(mall)}</span></div>
        <button type="button" onClick={() => void onReloadMall(mall.id)} disabled={submitting}>刷新状态</button>
      </div>
      <ol className={styles['mall-weather-steps']} aria-label="商场天气接入步骤">
        <li className={styles['done']}><strong>1. 商场已创建</strong><span>{mall.province} {mall.city} {mall.district} {mall.address}</span></li>
        <li className={mall.geocodeStatus.toLowerCase() === 'confirmed' ? styles.done : styles.current} aria-current={mall.geocodeStatus.toLowerCase() === 'confirmed' ? undefined : 'step'}><strong>2. 确认坐标</strong><span>先读取高德 GCJ-02 候选，再确认或修改</span></li>
        <li className={mall.geocodeStatus.toLowerCase() === 'confirmed' && !mall.weatherEnabled ? styles.current : undefined} aria-current={mall.geocodeStatus.toLowerCase() === 'confirmed' && !mall.weatherEnabled ? 'step' : undefined}><strong>3. 启用天气</strong><span>确认坐标时同步启用并创建首次采集任务</span></li>
      </ol>

      <div className={styles['mall-weather-geocode-actions']}>
        <div><strong>高德地址解析</strong><span>任务状态：{candidates?.runStatus || mall.geocodeStatus || '等待处理'} · 输出坐标系 GCJ-02</span></div>
        <div className={styles['mall-weather-form-actions']}>
          <button type="button" onClick={() => { setCandidatePollAttempts(0); setCandidatePollFailures(0); void loadCandidates() }} disabled={candidateState === 'loading' || submitting}>{candidateState === 'loading' ? '加载中' : '刷新候选'}</button>
          <button type="button" onClick={() => void triggerGeocode()} disabled={submitting || candidateState === 'loading'}>{submitting ? '处理中' : '重新解析地址'}</button>
        </div>
      </div>
      {candidates && candidates.items.length > 0 && <div className={styles['mall-weather-candidates']}>
        {candidates.items.map((candidate) => <article key={candidate.id}>
          <div><strong>候选 {candidate.candidateNo}</strong><span>置信度 {candidate.confidenceScore.toFixed(0)}% · {candidate.level || '层级未知'}</span></div>
          <p>{candidate.formattedAddress}</p>
          <small>{candidate.longitude.toFixed(6)}, {candidate.latitude.toFixed(6)} · 高德 {candidate.coordinateSystem}</small>
          <button className={candidate.id === selectedCandidateID ? styles.primary : undefined} type="button" onClick={() => selectCandidate(candidate.id)} disabled={submitting || candidateState === 'loading'}>{candidate.id === selectedCandidateID ? '已选择，可在下方修改' : '选择此高德坐标'}</button>
        </article>)}
      </div>}
      {candidateState === 'success' && candidates?.items.length === 0 && <p className={styles['mall-weather-action-message']}>高德暂未返回候选。请确认地址完整、AMAP_WEB_SERVICE_KEY 已配置且天气 Worker 已启用，然后重新解析地址。</p>}

      {selectedCandidateID > 0 && <form className={styles['mall-weather-manual-coordinate']} onSubmit={(event) => { event.preventDefault(); requestCoordinateConfirmation() }}>
        <div className={styles['mall-weather-section-title']}><div><strong>确认或修改高德坐标</strong><span>默认带入所选高德候选；如位置有偏差，可修改后再确认</span></div><span>GCJ-02</span></div>
        <label><span>高德经度</span><input name="longitude" inputMode="decimal" value={longitude} onChange={(event) => setLongitude(event.currentTarget.value)} required disabled={submitting || candidateState === 'loading'} /></label>
        <label><span>高德纬度</span><input name="latitude" inputMode="decimal" value={latitude} onChange={(event) => setLatitude(event.currentTarget.value)} required disabled={submitting || candidateState === 'loading'} /></label>
        <label className={styles['mall-weather-form-wide']}><span>修改原因</span><input name="reason" value={reason} onChange={(event) => setReason(event.currentTarget.value)} maxLength={500} disabled={submitting || candidateState === 'loading'} /></label>
        <button className={[styles['primary'], styles['mall-weather-form-wide']].join(' ')} type="submit" disabled={submitting || candidateState === 'loading'}>{submitting ? '确认中' : '确认高德坐标并启用天气'}</button>
      </form>}
      {coordinateConfirming && <Dialog open role="alertdialog" title="确认高德坐标并启用天气" closeDisabled={submitting} onClose={() => setCoordinateConfirming(false)}><div className={styles.dialogBody}>
        <p>将使用当前 GCJ-02 坐标启用天气服务，并创建首次采集任务。</p>
        <div className={styles['mall-weather-form-actions']}>
          <button className={styles['primary']} type="button" disabled={submitting} onClick={() => { setCoordinateConfirming(false); void runSingleFlight(coordinateConfirmationInFlight.current, confirmCoordinate) }}>{submitting ? '确认中' : '确认启用天气'}</button>
          <button type="button" disabled={submitting} onClick={() => setCoordinateConfirming(false)}>返回修改</button>
        </div>
      </div></Dialog>}
      {error && <p className={[styles['mall-weather-action-message'], styles['error']].join(' ')} role="alert">{error}</p>}
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
    <section className={[styles['workbench-panel'], styles['mall-weather-onboarding-panel']].join(' ')}>
      <div className={styles['mall-weather-section-title']}>
        <div><strong>高德商场坐标</strong><span>{Number(mall.longitude).toFixed(6)}, {Number(mall.latitude).toFixed(6)} · GCJ-02</span></div>
        <button type="button" onClick={() => { setEditing((current) => !current); setError('') }} disabled={submitting}>{editing ? '取消修改' : '修改坐标'}</button>
      </div>
      {editing && <form className={styles['mall-weather-manual-coordinate']} onSubmit={submit}>
        <label><span>高德经度</span><input name="longitude" inputMode="decimal" value={longitude} onChange={(event) => setLongitude(event.currentTarget.value)} required disabled={submitting} /></label>
        <label><span>高德纬度</span><input name="latitude" inputMode="decimal" value={latitude} onChange={(event) => setLatitude(event.currentTarget.value)} required disabled={submitting} /></label>
        <label className={styles['mall-weather-form-wide']}><span>修改原因</span><input name="reason" value={reason} onChange={(event) => setReason(event.currentTarget.value)} maxLength={500} required disabled={submitting} /></label>
        <button className={[styles['primary'], styles['mall-weather-form-wide']].join(' ')} type="submit" disabled={submitting}>{submitting ? '保存中' : '保存高德坐标'}</button>
      </form>}
      {error && <p className={[styles['mall-weather-action-message'], styles['error']].join(' ')} role="alert">{error}</p>}
    </section>
  )
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
    <article className={[styles['workbench-panel'], styles['mall-weather-realtime']].join(' ')} aria-label="当前实况天气">
      <header className={styles['mall-weather-realtime-heading']}>
        <div><strong>当前实况</strong><span>{realtime?.snapshotAtLocal || '暂无快照时间'}</span></div>
        <button type="button" onClick={downloadCsv} disabled={!realtime} aria-label={`下载数据：${mall.nameCn}当前实况 CSV`}><Download aria-hidden="true" />下载数据</button>
      </header>
      {downloadError && <p className={[styles['mall-weather-action-message'], styles['error']].join(' ')} role="alert">{downloadError}</p>}
      {realtime ? (
        <>
          <div className={styles['mall-weather-realtime-body']}>
            <div className={styles['mall-weather-sky-summary']}>
              <CloudSun aria-hidden="true" />
              <div className={styles['mall-weather-temperature']}><strong>{mallWeatherMetric(realtime.temperatureC, '°C')}</strong><span>{mallWeatherSkyconLabel(realtime.skycon).replace(/（.*）/, '')}</span></div>
            </div>
            <dl className={styles['mall-weather-current-metrics']}>
              <div><dt>体感</dt><dd>{mallWeatherMetric(realtime.apparentTemperatureC, '°C')}</dd></div>
              <div><dt>湿度</dt><dd>{mallWeatherMetric(realtime.humidityPct, '%', 0)}</dd></div>
              <div><dt>风速</dt><dd>{mallWeatherMetric(realtime.windSpeedKph, ' km/h')}</dd></div>
              <div><dt>能见度</dt><dd>{mallWeatherMetric(realtime.visibilityKm, ' km')}</dd></div>
              <div><dt>本地降水</dt><dd>{mallWeatherMetric(realtime.localPrecipitationMmH, ' mm/h')}</dd></div>
              <div><dt>质量</dt><dd className={realtime.qualityWarnings.length === 0 ? styles.success : styles.warning}>{realtime.qualityWarnings.length === 0 ? '正常' : `${realtime.qualityWarnings.length} 项告警`}</dd></div>
            </dl>
          </div>
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
  const [chartDownloadError, setChartDownloadError] = useState('')
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

  function downloadChartCsv(chartID: string, unit: string, series: MallWeatherChartSeries[]) {
    setChartDownloadError('')
    try {
      downloadMallWeatherBytes(
        createMallWeatherChartCsv(series.map((item) => ({ ...item, unit })), { mallCode: mall.mallCode, mallName: mall.nameCn }),
        mallWeatherChartCsvFileName(chartID, mall.mallCode),
      )
    } catch {
      setChartDownloadError('曲线数据 CSV 文件生成失败，请重新加载后重试。')
    }
  }

  const primaryAlert = alerts.items[0]

  return (
    <div className={styles['mall-weather-overview-details']}>
      <section className={[styles['workbench-panel'], styles['mall-weather-air-quality']].join(' ')} aria-label="空气质量">
        <strong className={styles['mall-weather-card-title']}>空气质量</strong>
        <div className={styles['mall-weather-aqi']}><strong>{mallWeatherMetric(realtime?.aqiChn, '', 0)}</strong><span>AQI</span></div>
        <b className={styles['mall-weather-aqi-grade']}>{realtime?.aqiDescriptionChn || '暂无评级'}</b>
        <dl className={styles['mall-weather-air-metrics']}>
          <div><dt>PM2.5</dt><dd>{mallWeatherMetric(realtime?.pm25UgM3, '')}</dd></div>
          <div><dt>PM10</dt><dd>{mallWeatherMetric(realtime?.pm10UgM3, '')}</dd></div>
          <div><dt>O₃</dt><dd>{mallWeatherMetric(realtime?.o3UgM3, '')}</dd></div>
        </dl>
      </section>

      <section className={[styles['workbench-panel'], styles['mall-weather-design-alert']].join(' ')} id="mall-weather-alerts" tabIndex={-1} aria-busy={alerts.loading}>
        <header className={styles['mall-weather-card-heading']}>
          <strong className={styles['mall-weather-card-title']}>气象预警</strong>
          <button type="button" onClick={downloadAlertsCsv} disabled={alerts.loading || Boolean(alerts.error)} aria-label={`下载数据：${mall.nameCn}气象预警 CSV`}><Download aria-hidden="true" />下载数据</button>
        </header>
        {alerts.loading && <LoadingState label="正在加载全部气象预警" />}
        {alerts.error && <RequestError message={alerts.error} onRetry={onAlertsRetry} />}
        {alerts.ready && (primaryAlert
          ? <article>
            <AlertTriangle aria-hidden="true" />
            <strong>{primaryAlert.title}</strong>
            <span>{primaryAlert.source || '预警发布机构'}</span>
            <time>{primaryAlert.publishedAtLocal || '发布时间未知'}</time>
            {primaryAlert.description && <p>{primaryAlert.description}</p>}
          </article>
          : <EmptyState title="当前无有效预警" detail="当前查询窗口没有返回有效气象预警。" />)}
        {alertDownloadError && <p className={[styles['mall-weather-action-message'], styles['error']].join(' ')} role="alert">{alertDownloadError}</p>}
      </section>

      <section className={[styles['content-grid'], styles['two'], styles['mall-weather-design-charts']].join(' ')}>
        <MallWeatherChart
          title="未来 120 分钟降水"
          detail="1 km 级"
          unit="mm/h"
          icon={<CloudRain aria-hidden="true" />}
          series={[{ id: 'precipitationMmH', name: '降水强度', color: '#0F9F78', data: overview.minutely.map((item) => ({ time: item.forecastMinuteLocal, value: item.precipitationMmH })) }]}
          floorZero
          onDownload={(series) => downloadChartCsv('overview_minutely_precipitation', 'mm/h', series)}
        />
        <MallWeatherChart
          title="未来 24 小时温度"
          detail="9～13 km 预报网格"
          unit="°C"
          icon={<Thermometer aria-hidden="true" />}
          series={[
            { id: 'temperatureC', name: '温度', color: '#B45309', data: overview.hourly.map((item) => ({ time: item.forecastTimeLocal, value: item.temperatureC })) },
            { id: 'apparentTemperatureC', name: '体感温度', color: '#B91C1C', dash: '6 4', data: overview.hourly.map((item) => ({ time: item.forecastTimeLocal, value: item.apparentTemperatureC })) },
          ]}
          onDownload={(series) => downloadChartCsv('overview_hourly_temperature', '°C', series)}
        />
      </section>
      {chartDownloadError && <p className={[styles['mall-weather-action-message'], styles['error'], styles['mall-weather-chart-download-error']].join(' ')} role="alert">{chartDownloadError}</p>}

      <section className={[styles['workbench-panel'], styles['mall-weather-summary']].join(' ')}>
        <div className={styles['mall-weather-meta']} aria-label="天气数据口径">
          <div className={styles['mall-weather-source-item']}><CloudSun aria-hidden="true" /><span>供应商<strong>{`${meta.provider || '彩云天气'} ${meta.apiVersion}`.trim()}</strong></span></div>
          <div className={styles['mall-weather-source-item']}><Clock3 aria-hidden="true" /><span>新鲜度<strong>{mallWeatherFreshnessLabel(meta.freshnessStatus)}</strong></span></div>
          <div className={styles['mall-weather-source-item']}><MapPin aria-hidden="true" /><span>坐标<strong>{meta.longitude === 0 && meta.latitude === 0 ? '坐标未提供' : `${meta.longitude.toFixed(4)}, ${meta.latitude.toFixed(4)} ${meta.coordinateSystem}`}</strong></span></div>
        </div>
        <details className={styles['mall-weather-resolution']}><summary>查看天气数据口径</summary><p>代表点：{representativePoint} · {coverageRadius}。实况与未来两小时降水为商场中心点 1 km 级数据；常规小时预报为商场中心点所在 9～13 km 预报网格，预警按行政区域发布。</p><button type="button" onClick={onRefresh} disabled={refreshing}><RefreshCcw aria-hidden="true" />{refreshing ? '加载中' : '重新加载'}</button></details>
      </section>
    </div>
  )
}

function RequestError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <div className={[styles['mall-weather-request-state'], styles['error']].join(' ')} role="alert"><strong>加载失败</strong><span>{message}</span><button type="button" onClick={onRetry}>重试</button></div>
}

function LoadingState({ label }: { label: string }) {
  return <div className={styles['mall-weather-request-state']} role="status" aria-busy="true"><RefreshCcw aria-hidden="true" /><span>{label}</span></div>
}

function EmptyState({ title, detail }: { title: string; detail: string }) {
  return <div className={styles['empty-state']} role="status"><strong>{title}</strong><span>{detail}</span></div>
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

function mallLifecycleLabel(mall: MallWeatherMall) {
  if (mall.geocodeStatus.toLowerCase() === 'failed') return '坐标解析失败'
  if (mall.geocodeStatus.toLowerCase() !== 'confirmed') return '等待确认坐标'
  if (!mall.weatherEnabled) return '天气未启用'
  if (mall.status.toLowerCase() !== 'active') return '商场未启用'
  return '可查询'
}
