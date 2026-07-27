import { type FormEvent, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AlertTriangle, CloudRain, MapPin, RefreshCcw, Thermometer, Wind } from 'lucide-react'
import './MallWeatherPage.css'
import { MallWeatherForecastPanel } from './MallWeatherForecastPanel'
import {
  mallWeatherChartSegments,
  mallWeatherCreateKey,
  mallWeatherCreateRequest,
  clearMallWeatherPendingCreate,
  clearMallWeatherPendingRefresh,
  clearMallWeatherPendingSheetPush,
  loadMallWeatherPendingCreate,
  loadMallWeatherPendingRefresh,
  loadMallWeatherPendingSheetPush,
  mallWeatherFreshnessLabel,
  mallWeatherMetric,
  mallWeatherGeocodeCandidatesPath,
  mallWeatherGeocodeConfirmPath,
  mallWeatherGeocodeRunTerminal,
  mallWeatherGeocodeTriggerPath,
  mallWeatherMallReady,
  mallWeatherSheetPushKey,
  mallWeatherSheetPushRequest,
  mallWeatherSheetPushRequestMatchesOption,
  mallWeatherSheetPushResultMatchesRequest,
  mergeMallWeatherMalls,
  mallWeatherOverviewPath,
  mallWeatherRefreshKey,
  mallWeatherRefreshDisposition,
  mallWeatherRefreshPath,
  mallWeatherRefreshRequest,
  mallWeatherRefreshResultMessage,
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
  type MallWeatherCreateInput,
  type MallWeatherPendingCreate,
  type MallWeatherPendingSheetPush,
  type MallWeatherGeocodeCandidates,
  type MallWeatherMall,
  type MallWeatherOverview,
  type MallWeatherPendingRefresh,
  type MallWeatherRefreshRequest,
  type MallWeatherSheetPushDryRun,
  type MallWeatherSheetPushOption,
} from './mallWeather'

type MallWeatherApiResult = {
  ok: boolean
  status: number
  data: unknown
}

type MallWeatherApiClient = (
  path: string,
  options?: { method?: 'GET' | 'POST'; body?: unknown; headers?: Record<string, string>; showResult?: boolean; silentLoading?: boolean; signal?: AbortSignal },
) => Promise<MallWeatherApiResult>

type LoadState = 'idle' | 'loading' | 'success' | 'error'

export function MallWeatherPage({ actorID, client }: { actorID: string | null; client: MallWeatherApiClient }) {
  const [malls, setMalls] = useState<MallWeatherMall[]>([])
  const [nextAfterID, setNextAfterID] = useState(0)
  const [mallState, setMallState] = useState<LoadState>('idle')
  const [mallError, setMallError] = useState('')
  const [selectedMallID, setSelectedMallID] = useState(0)
  const [overview, setOverview] = useState<MallWeatherOverview | null>(null)
  const [overviewMallID, setOverviewMallID] = useState(0)
  const [overviewState, setOverviewState] = useState<LoadState>('idle')
  const [overviewError, setOverviewError] = useState('')
  const [query, setQuery] = useState('')
  const [city, setCity] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const mallRequestSequence = useRef(0)
  const overviewRequestSequence = useRef(0)
  const selectedMallIDRef = useRef(0)

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

  const loadOverview = useCallback(async (mallID: number) => {
    if (!mallID) return
    const sequence = ++overviewRequestSequence.current
    setOverviewState('loading')
    setOverviewError('')
    const response = await client(mallWeatherOverviewPath(mallID), { method: 'GET', showResult: false, silentLoading: true })
    if (sequence !== overviewRequestSequence.current) return
    if (!response.ok) {
      setOverview(null)
      setOverviewMallID(0)
      setOverviewState('error')
      setOverviewError(weatherRequestError(response.status, '天气概览加载失败', '当前账号缺少 weather.read 权限'))
      return
    }
    const parsed = parseMallWeatherOverview(response.data)
    if (!parsed) {
      setOverview(null)
      setOverviewMallID(0)
      setOverviewState('error')
      setOverviewError('天气概览响应格式不正确，请联系管理员')
      return
    }
    setOverview(parsed)
    setOverviewMallID(mallID)
    setOverviewState('success')
  }, [client])

  useEffect(() => {
    void loadMalls()
  }, [loadMalls])

  useEffect(() => {
    const mall = malls.find((item) => item.id === selectedMallID)
    if (mall && mallWeatherMallReady(mall)) {
      void loadOverview(selectedMallID)
      return
    }
    overviewRequestSequence.current++
    setOverview(null)
    setOverviewMallID(0)
    setOverviewState('idle')
    setOverviewError('')
  }, [loadOverview, malls, selectedMallID])

  const cities = useMemo(() => Array.from(new Set(malls.map((mall) => mall.city).filter(Boolean))).sort(), [malls])
  useEffect(() => {
    if (city && !cities.includes(city)) setCity('')
  }, [cities, city])
  const visibleMalls = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    return malls.filter((mall) => (!city || mall.city === city) && (!normalized || `${mall.nameCn} ${mall.mallCode} ${mall.city} ${mall.address}`.toLowerCase().includes(normalized)))
  }, [city, malls, query])
  const selectedMall = malls.find((mall) => mall.id === selectedMallID)
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
    setMalls((current) => current.map((item) => item.id === mall.id ? mall : item))
    setSelectedMallID(mall.id)
  }, [])

  return (
    <div className="view-stack mall-weather-page">
      <section className="mall-weather-toolbar" aria-label="商场天气筛选">
        <label>
          <span>搜索商场</span>
          <input name="mallWeatherQuery" type="search" autoComplete="off" value={query} onChange={(event) => setQuery(event.currentTarget.value)} placeholder="名称、编码或地址" />
        </label>
        <label>
          <span>城市</span>
          <select name="mallWeatherCity" value={city} onChange={(event) => setCity(event.currentTarget.value)}>
            <option value="">全部城市</option>
            {cities.map((item) => <option value={item} key={item}>{item}</option>)}
          </select>
        </label>
        <button type="button" onClick={() => void loadMalls()} disabled={mallState === 'loading'}>
          <RefreshCcw aria-hidden="true" />刷新列表
        </button>
        <button className="primary" type="button" onClick={() => setShowCreate((current) => !current)} aria-expanded={showCreate} aria-controls="mall-weather-create-panel">
          {showCreate ? '关闭新增' : '新增商场'}
        </button>
      </section>

      <div className="mall-weather-layout">
        <aside className="workbench-panel mall-weather-malls" aria-label="商场接入列表">
          <div className="mall-weather-section-title">
            <div><strong>商场</strong><span>全部接入状态</span></div>
            <span>{visibleMalls.length} / {malls.length}</span>
          </div>
          {mallState === 'error' && <RequestError message={mallError} onRetry={() => void loadMalls()} />}
          {mallState === 'loading' && malls.length === 0 && <LoadingState label="正在加载商场" />}
          {mallState === 'success' && malls.length === 0 && <EmptyState title="还没有商场" detail="点击“新增商场”开始接入天气。" />}
          {malls.length > 0 && visibleMalls.length === 0 && <EmptyState title="没有匹配结果" detail="请调整名称或城市筛选。" />}
          <div className="mall-weather-mall-list">
            {visibleMalls.map((mall) => (
              <button
                type="button"
                className={mall.id === selectedMallID ? 'mall-weather-mall active' : 'mall-weather-mall'}
                aria-pressed={mall.id === selectedMallID}
                key={mall.id}
                onClick={() => { selectedMallIDRef.current = mall.id; setSelectedMallID(mall.id); setShowCreate(false) }}
              >
                <strong>{mall.nameCn}</strong>
                <span>{mall.mallCode} · {mall.city || '城市未填写'} · {mallWeatherMallReady(mall) ? '可查询' : mallLifecycleLabel(mall)}</span>
                <small>{mall.address || '地址未填写'}</small>
              </button>
            ))}
          </div>
          {nextAfterID > 0 && (
            <button type="button" onClick={() => void loadMalls(nextAfterID)} disabled={mallState === 'loading'}>加载更多商场</button>
          )}
        </aside>

        <section className="mall-weather-content">
          {showCreate && (actorID
            ? <MallCreatePanel actorID={actorID} client={client} onCreated={handleMallCreated} onCancel={() => setShowCreate(false)} />
            : <RequestError message="无法识别当前登录账号，请退出后重新登录再新增商场。" onRetry={() => window.location.reload()} />)}
          {!showCreate && !selectedMall && mallState !== 'loading' && <EmptyState title="请选择或新增商场" detail="选择左侧商场查看接入进度，或新增一个商场。" />}
          {!showCreate && selectedMall && !selectedMallReady && <MallOnboardingPanel
            key={`onboarding-${selectedMall.id}`}
            mall={selectedMall}
            client={client}
            onMallUpdated={handleMallUpdated}
            onReloadMall={loadMall}
          />}
          {!showCreate && selectedMallReady && selectedMall && (actorID
            ? <>
              <ManualRefreshPanel actorID={actorID} mall={selectedMall} client={client} key={`refresh-${actorID}:${selectedMall.id}`} />
              <MallWeatherSheetPushPanel actorID={actorID} mall={selectedMall} client={client} key={`push-${actorID}:${selectedMall.id}`} />
            </>
            : <RequestError message="无法识别当前登录账号，请退出后重新登录再提交天气刷新。" onRetry={() => window.location.reload()} />)}
          {!showCreate && selectedMallReady && selectedMall && overviewState === 'loading' && !selectedOverview && <LoadingState label={`正在加载${selectedMall.nameCn}天气`} />}
          {!showCreate && selectedMallReady && selectedMall && overviewState === 'error' && <RequestError message={overviewError} onRetry={() => void loadOverview(selectedMall.id)} />}
          {!showCreate && selectedMallReady && selectedMall && selectedOverview && <WeatherOverview mall={selectedMall} overview={selectedOverview} refreshing={overviewState === 'loading'} onRefresh={() => void loadOverview(selectedMall.id)} />}
          {!showCreate && selectedMallReady && selectedMall && <MallWeatherForecastPanel
            mallID={selectedMall.id}
            timeZone={selectedOverview?.meta.timeZone || 'Asia/Shanghai'}
            client={client}
            key={`forecast-${selectedMall.id}:${selectedOverview?.meta.timeZone || 'Asia/Shanghai'}`}
          />}
        </section>
      </div>
    </div>
  )
}

const emptyMallCreateInput: MallWeatherCreateInput = {
  mallCode: '', nameCn: '', province: '', city: '', district: '', address: '',
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
      status: created.status,
      version: created.version,
    })
  }

  return (
    <section className="workbench-panel mall-weather-onboarding-panel" id="mall-weather-create-panel">
      <div className="mall-weather-section-title"><div><strong>新增商场</strong><span>创建后继续确认坐标并启用天气</span></div><span>天气口径：full · 1000 m</span></div>
      <form className="mall-weather-create-form" onSubmit={submit} aria-busy={submitting}>
        <label><span>商场编码 *</span><input name="mallCode" value={form.mallCode} onChange={(event) => change('mallCode', event.currentTarget.value)} placeholder="例如 SH-PD-001" maxLength={64} disabled={submitting} /></label>
        <label><span>商场名称 *</span><input name="nameCn" value={form.nameCn} onChange={(event) => change('nameCn', event.currentTarget.value)} placeholder="中文名称" maxLength={255} disabled={submitting} /></label>
        <label><span>省份 *</span><input name="province" value={form.province} onChange={(event) => change('province', event.currentTarget.value)} placeholder="上海市" maxLength={128} disabled={submitting} /></label>
        <label><span>城市 *</span><input name="city" value={form.city} onChange={(event) => change('city', event.currentTarget.value)} placeholder="上海市" maxLength={128} disabled={submitting} /></label>
        <label><span>区县</span><input name="district" value={form.district} onChange={(event) => change('district', event.currentTarget.value)} placeholder="浦东新区" maxLength={128} disabled={submitting} /></label>
        <label className="mall-weather-form-wide"><span>详细地址 *</span><input name="address" value={form.address} onChange={(event) => change('address', event.currentTarget.value)} placeholder="道路、门牌号及建筑名称" maxLength={1000} disabled={submitting} /></label>
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
  const [longitude, setLongitude] = useState(mall.longitude === undefined ? '' : String(mall.longitude))
  const [latitude, setLatitude] = useState(mall.latitude === undefined ? '' : String(mall.latitude))
  const [reason, setReason] = useState('管理端人工确认商场坐标')
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
    if (sequence !== candidateRequestSequence.current) return
    if (!response.ok) {
      setCandidateState('error')
      setError(weatherActionError(response.status, '坐标候选加载失败', '当前账号缺少 mall.read 权限'))
      return
    }
    const parsed = parseMallWeatherGeocodeCandidates(response.data)
    if (!parsed) {
      setCandidateState('error')
      setError('坐标候选响应格式不正确，请联系管理员')
      return
    }
    setCandidates(parsed)
    setCandidateState('success')
    if (parsed.runStatus === 'AUTO_CONFIRMED') await onReloadMall(mall.id)
  }, [client, mall.id, onReloadMall])

  useEffect(() => {
    void loadCandidates()
    return cancelCandidateRequests
  }, [cancelCandidateRequests, loadCandidates, mall.version])

  useEffect(() => {
    if (!candidates || candidates.items.length > 0 || mallWeatherGeocodeRunTerminal(candidates.runStatus)) return
    const timer = window.setTimeout(() => void loadCandidates(), 5000)
    return () => window.clearTimeout(timer)
  }, [candidates, loadCandidates])

  async function triggerGeocode() {
    setSubmitting(true)
    setError('')
    const expectedMallVersion = candidates?.mallVersion || mall.version
    const response = await client(mallWeatherGeocodeTriggerPath(mall.id), {
      method: 'POST', body: { expectedMallVersion }, showResult: false, silentLoading: true,
    })
    setSubmitting(false)
    if (!response.ok) {
      setError(weatherActionError(response.status, '坐标解析任务提交失败', '当前账号缺少 mall.write 权限'))
      return
    }
    await onReloadMall(mall.id)
    await loadCandidates()
  }

  async function confirmCoordinate(candidateID?: number) {
    const expectedMallVersion = candidates?.mallVersion || mall.version
    let body: unknown
    if (candidateID) {
      body = { candidateId: candidateID, expectedMallVersion, weatherEnabled: true }
    } else {
      const nextLongitude = Number(longitude)
      const nextLatitude = Number(latitude)
      const nextReason = reason.trim()
      if (!longitude.trim() || !latitude.trim() || !Number.isFinite(nextLongitude) || nextLongitude < -180 || nextLongitude > 180 || !Number.isFinite(nextLatitude) || nextLatitude < -90 || nextLatitude > 90 || !nextReason || nextReason.length > 500) {
        setError('请填写有效的经纬度和 500 字以内的确认原因。')
        return
      }
      body = { manualCoordinate: { longitude: nextLongitude, latitude: nextLatitude, coordinateSystem: 'GCJ02', reason: nextReason }, expectedMallVersion, weatherEnabled: true }
    }
    setSubmitting(true)
    setError('')
    const response = await client(mallWeatherGeocodeConfirmPath(mall.id), { method: 'POST', body, showResult: false, silentLoading: true })
    setSubmitting(false)
    if (!response.ok) {
      setError(weatherActionError(response.status, '坐标确认失败', '当前账号缺少 mall.geocode.confirm 权限'))
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
        <div><strong>{mall.nameCn} · 接入天气</strong><span>{mall.mallCode} · {mallLifecycleLabel(mall)}</span></div>
        <button type="button" onClick={() => void onReloadMall(mall.id)} disabled={submitting}>刷新状态</button>
      </div>
      <ol className="mall-weather-steps" aria-label="商场天气接入步骤">
        <li className="done"><strong>1. 商场已创建</strong><span>{mall.province} {mall.city} {mall.district} {mall.address}</span></li>
        <li className={mall.geocodeStatus.toLowerCase() === 'confirmed' ? 'done' : 'current'} aria-current={mall.geocodeStatus.toLowerCase() === 'confirmed' ? undefined : 'step'}><strong>2. 确认坐标</strong><span>候选解析或人工录入 GCJ02 坐标</span></li>
        <li className={mall.geocodeStatus.toLowerCase() === 'confirmed' && !mall.weatherEnabled ? 'current' : ''} aria-current={mall.geocodeStatus.toLowerCase() === 'confirmed' && !mall.weatherEnabled ? 'step' : undefined}><strong>3. 启用天气</strong><span>确认坐标时同步启用并创建首次采集任务</span></li>
      </ol>

      <div className="mall-weather-geocode-actions">
        <div><strong>自动解析候选</strong><span>任务状态：{candidates?.runStatus || mall.geocodeStatus || '等待处理'}</span></div>
        <div className="mall-weather-form-actions">
          <button type="button" onClick={() => void loadCandidates()} disabled={candidateState === 'loading' || submitting}>{candidateState === 'loading' ? '加载中' : '刷新候选'}</button>
          <button type="button" onClick={() => void triggerGeocode()} disabled={submitting || candidateState === 'loading'}>{submitting ? '处理中' : '重新解析地址'}</button>
        </div>
      </div>
      {candidates && candidates.items.length > 0 && <div className="mall-weather-candidates">
        {candidates.items.map((candidate) => <article key={candidate.id}>
          <div><strong>候选 {candidate.candidateNo}</strong><span>置信度 {(candidate.confidenceScore * 100).toFixed(0)}% · {candidate.level || '层级未知'}</span></div>
          <p>{candidate.formattedAddress}</p>
          <small>{candidate.longitude.toFixed(6)}, {candidate.latitude.toFixed(6)} {candidate.coordinateSystem}</small>
          <button className="primary" type="button" onClick={() => void confirmCoordinate(candidate.id)} disabled={submitting || candidateState === 'loading'}>选用并启用天气</button>
        </article>)}
      </div>}
      {candidateState === 'success' && candidates?.items.length === 0 && <p className="mall-weather-action-message">暂未产生候选；后台任务会继续处理，也可直接使用下方人工坐标。</p>}

      <form className="mall-weather-manual-coordinate" onSubmit={(event) => { event.preventDefault(); void confirmCoordinate() }}>
        <div className="mall-weather-section-title"><div><strong>人工确认坐标</strong><span>自动解析不可用时的高可用兜底，坐标系固定为 GCJ02</span></div></div>
        <label><span>经度</span><input name="longitude" inputMode="decimal" value={longitude} onChange={(event) => setLongitude(event.currentTarget.value)} placeholder="121.473701" required disabled={submitting || candidateState === 'loading'} /></label>
        <label><span>纬度</span><input name="latitude" inputMode="decimal" value={latitude} onChange={(event) => setLatitude(event.currentTarget.value)} placeholder="31.230416" required disabled={submitting || candidateState === 'loading'} /></label>
        <label className="mall-weather-form-wide"><span>确认原因</span><input name="reason" value={reason} onChange={(event) => setReason(event.currentTarget.value)} maxLength={500} disabled={submitting || candidateState === 'loading'} /></label>
        <button className="primary mall-weather-form-wide" type="submit" disabled={submitting || candidateState === 'loading'}>{submitting ? '确认中' : '确认坐标并启用天气'}</button>
      </form>
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
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const optionRequestSequence = useRef(0)

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
    setMessage(`推送任务 #${result.runId} 已创建，预计 ${result.estimatedRows} 行，状态 ${result.status}。`)
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
        <label><span>已有推送目标</span><select name="weatherSheetPushTarget" value={selectedDestinationID} onChange={(event) => changeOption(event.currentTarget.value)} disabled={checking || submitting}>
          {options.map((option) => <option value={option.destinationId} key={option.destinationId}>{option.name} · {option.code} · {option.profileCode} v{option.profileVersion}</option>)}
        </select></label>
        <button type="button" onClick={() => void checkBinding()} disabled={checking || submitting}>{checking ? '验证中' : '验证绑定'}</button>
        <button className="primary" type="button" onClick={() => void submitPush()} disabled={checking || submitting || !dryRun?.canExecute || Boolean(pending && !pendingMatchesOption)}>{submitting ? '提交中' : pending ? '重试原请求' : '绑定并发起推送'}</button>
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
      {message && <p className="mall-weather-action-message" role="status">{message}</p>}
      {error && optionState !== 'error' && <p className="mall-weather-action-message error" role="alert">{error}</p>}
    </section>
  )
}

function ManualRefreshPanel({ actorID, mall, client }: { actorID: string; mall: MallWeatherMall; client: MallWeatherApiClient }) {
  const [pending, setPending] = useState<MallWeatherPendingRefresh | null>(() => loadMallWeatherPendingRefresh(actorID, mall.id, window.sessionStorage))
  const [profile, setProfile] = useState<'weather' | 'life' | 'all'>(() => refreshProfile(pending?.body.kinds))
  const [reason, setReason] = useState(() => pending?.body.reason || '管理端手工刷新')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const reasonHelpID = `mall-weather-refresh-reason-help-${actorID}-${mall.id}`

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    let request = pending
    if (!request) {
      const kinds = profile === 'weather' ? ['V26_FULL'] as const : profile === 'life' ? ['V3_LIFE_INDEX'] as const : ['V26_FULL', 'V3_LIFE_INDEX'] as const
      let body: MallWeatherRefreshRequest
      try {
        body = mallWeatherRefreshRequest([...kinds], reason)
      } catch {
        setError('请填写单行刷新原因，最多 500 个字符')
        setMessage('')
        return
      }
      request = { key: mallWeatherRefreshKey(), body }
      saveMallWeatherPendingRefresh(actorID, mall.id, request, window.sessionStorage)
      setPending(request)
    }
    setSubmitting(true)
    setError('')
    setMessage('')
    const response = await client(mallWeatherRefreshPath(mall.id), {
      method: 'POST',
      body: request.body,
      headers: { 'Idempotency-Key': request.key },
      showResult: false,
      silentLoading: true,
    })
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
    setMessage(mallWeatherRefreshResultMessage(disposition.result))
  }

  function changeProfile(value: string) {
    if (value !== 'weather' && value !== 'life' && value !== 'all') return
    setProfile(value)
  }

  function changeReason(value: string) {
    setReason(value)
  }

  return (
    <section className="workbench-panel mall-weather-refresh-panel">
      <div className="mall-weather-section-title"><div><strong>手工刷新</strong><span>提交异步采集任务，不阻塞等待供应商</span></div><RefreshCcw aria-hidden="true" /></div>
      <form className="mall-weather-refresh-form" onSubmit={submit} aria-busy={submitting}>
        <label><span>采集范围</span><select value={profile} onChange={(event) => changeProfile(event.currentTarget.value)} disabled={submitting || Boolean(pending)}>
          <option value="all">全量天气 + 生活指数</option>
          <option value="weather">全量天气</option>
          <option value="life">生活指数</option>
        </select></label>
        <label><span>刷新原因</span><input value={reason} onChange={(event) => changeReason(event.currentTarget.value)} disabled={submitting || Boolean(pending)} aria-describedby={reasonHelpID} />
          <small id={reasonHelpID}>必填单行文本，最多 500 个字符</small>
        </label>
        <button className="primary" type="submit" disabled={submitting}>{submitting ? '提交中' : pending ? '重试原请求' : '提交刷新'}</button>
      </form>
      {message && <p className="mall-weather-action-message" role="status">{message}</p>}
      {error && <p className="mall-weather-action-message error" role="alert">{error}</p>}
    </section>
  )
}

function refreshProfile(kinds: MallWeatherRefreshRequest['kinds'] | undefined): 'weather' | 'life' | 'all' {
  if (kinds?.length === 1 && kinds[0] === 'V26_FULL') return 'weather'
  if (kinds?.length === 1 && kinds[0] === 'V3_LIFE_INDEX') return 'life'
  return 'all'
}

function WeatherOverview({ mall, overview, refreshing, onRefresh }: { mall: MallWeatherMall; overview: MallWeatherOverview; refreshing: boolean; onRefresh: () => void }) {
  const { realtime, meta } = overview
  const representativePoint = ['MALL_CENTER', 'CENTER', 'center'].includes(meta.representativePoint) ? '商场中心点' : meta.representativePoint || '口径缺失'
  const coverageRadius = meta.coverageRadiusM > 0 ? `业务半径 ${meta.coverageRadiusM} m` : '业务半径口径缺失'

  return (
    <div className="view-stack">
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

      <section className="mall-weather-overview-grid">
        <article className="workbench-panel mall-weather-realtime">
          <div className="mall-weather-section-title"><div><strong>当前实况</strong><span>{realtime?.snapshotAtLocal || '暂无快照时间'}</span></div><Thermometer aria-hidden="true" /></div>
          {realtime ? (
            <>
              <div className="mall-weather-temperature"><strong>{mallWeatherMetric(realtime.temperatureC, '°C')}</strong><span>{mallWeatherSkyconLabel(realtime.skycon)}</span></div>
              <div className="mall-weather-metrics">
                <MetaItem label="体感" value={mallWeatherMetric(realtime.apparentTemperatureC, '°C')} />
                <MetaItem label="湿度" value={mallWeatherMetric(realtime.humidityPct, '%', 0)} />
                <MetaItem label="风速" value={mallWeatherMetric(realtime.windSpeedKph, ' km/h')} />
                <MetaItem label="能见度" value={mallWeatherMetric(realtime.visibilityKm, ' km')} />
                <MetaItem label="本地降水" value={mallWeatherMetric(realtime.localPrecipitationMmH, ' mm/h')} />
                <MetaItem label="质量" value={`${realtime.qualityStatus || '未知'}${realtime.qualityWarnings.length ? ` · ${realtime.qualityWarnings.length} 项告警` : ''}`} />
              </div>
              <p className="mall-weather-caption">供应商时间 {realtime.providerServerTimeLocal || '—'} · 采集时间 {realtime.fetchedAtLocal || '—'}</p>
            </>
          ) : <EmptyState title="暂无实况" detail="最近一次采集尚未产生可用实况。" />}
        </article>

        <article className="workbench-panel">
          <div className="mall-weather-section-title"><div><strong>空气质量</strong><span>{realtime?.aqiDescriptionChn || '中国 AQI 标准'}</span></div><Wind aria-hidden="true" /></div>
          <div className="mall-weather-aqi"><strong>{mallWeatherMetric(realtime?.aqiChn, '', 0)}</strong><span>AQI</span></div>
          <div className="mall-weather-metrics compact">
            <MetaItem label="PM2.5" value={mallWeatherMetric(realtime?.pm25UgM3, ' μg/m³')} />
            <MetaItem label="PM10" value={mallWeatherMetric(realtime?.pm10UgM3, ' μg/m³')} />
            <MetaItem label="O₃" value={mallWeatherMetric(realtime?.o3UgM3, ' μg/m³')} />
            <MetaItem label="NO₂" value={mallWeatherMetric(realtime?.no2UgM3, ' μg/m³')} />
            <MetaItem label="SO₂" value={mallWeatherMetric(realtime?.so2UgM3, ' μg/m³')} />
            <MetaItem label="CO" value={mallWeatherMetric(realtime?.coMgM3, ' mg/m³')} />
          </div>
        </article>
      </section>

      <section className="content-grid two">
        <WeatherChart
          title="未来 120 分钟降水"
          detail="1 km 级"
          unit="mm/h"
          icon={<CloudRain aria-hidden="true" />}
          series={overview.minutely.map((item) => ({ time: item.forecastMinuteLocal, value: item.precipitationMmH }))}
        />
        <WeatherChart
          title="未来 24 小时温度"
          detail="9～13 km 预报网格"
          unit="°C"
          icon={<Thermometer aria-hidden="true" />}
          series={overview.hourly.map((item) => ({ time: item.forecastTimeLocal, value: item.temperatureC }))}
        />
      </section>

      <section className="workbench-panel">
        <div className="mall-weather-section-title"><div><strong>气象预警</strong><span>行政区域口径</span></div><AlertTriangle aria-hidden="true" /></div>
        {overview.alerts.length === 0 ? <EmptyState title="当前无有效预警" detail="最近一次概览没有返回气象预警。" /> : (
          <div className="mall-weather-alerts">
            {overview.alerts.map((alert) => (
              <article key={alert.alertId || alert.title}>
                <div><strong>{alert.title}</strong><span>{[alert.alertTypeName, alert.alertLevelName].filter(Boolean).join(' · ') || alert.status}</span></div>
                {alert.description && <p>{alert.description}</p>}
                <small>{alert.source || '预警发布机构'} · {alert.publishedAtLocal || '发布时间未知'}</small>
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function WeatherChart({ title, detail, unit, icon, series }: { title: string; detail: string; unit: string; icon: ReactNode; series: Array<{ time: string; value?: number }> }) {
  const values = series.map((item) => item.value)
  const segments = mallWeatherChartSegments(values, 640, 140)
  const available = series.filter((item): item is { time: string; value: number } => typeof item.value === 'number' && Number.isFinite(item.value))
  const minimum = available.length ? Math.min(...available.map((item) => item.value)) : undefined
  const maximum = available.length ? Math.max(...available.map((item) => item.value)) : undefined
  const startTime = available[0]?.time || '未知'
  const endTime = available[available.length - 1]?.time || '未知'
  const description = `${startTime} 至 ${endTime}，最低 ${mallWeatherMetric(minimum, unit)}，最高 ${mallWeatherMetric(maximum, unit)}`
  return (
    <article className="workbench-panel mall-weather-chart-panel">
      <div className="mall-weather-section-title"><div><strong>{title}</strong><span>{detail} · {unit}</span></div>{icon}</div>
      {segments.length === 0 ? <EmptyState title="暂无趋势数据" detail="当前时间窗口没有可绘制的数据。" /> : (
        <>
        <svg viewBox="0 0 640 160" role="img" aria-label={`${title}趋势图`} preserveAspectRatio="none">
          <title>{title}</title>
          <desc>{description}</desc>
          <path d="M0 140 H640" />
          <path d="M0 70 H640" />
          {segments.map((points, index) => points.includes(' ')
            ? <polyline points={points} key={`${points}-${index}`} />
            : <ChartPoint point={points} key={`${points}-${index}`} />)}
        </svg>
        <p className="mall-weather-chart-summary">{description}</p>
        <details className="mall-weather-chart-data">
          <summary>查看趋势明细（{series.length} 条）</summary>
          <div className="data-table-wrap">
            <table className="data-table"><thead><tr><th scope="col">时间</th><th scope="col">数值</th></tr></thead><tbody>
              {series.map((item, index) => <tr key={`${item.time}-${index}`}><td>{item.time || '未知'}</td><td>{mallWeatherMetric(item.value, unit)}</td></tr>)}
            </tbody></table>
          </div>
        </details>
        </>
      )}
    </article>
  )
}

function ChartPoint({ point }: { point: string }) {
  const [cx = '0', cy = '0'] = point.split(',')
  return <circle cx={cx} cy={cy} r="4" />
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
