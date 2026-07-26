import { type FormEvent, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AlertTriangle, CloudRain, MapPin, RefreshCcw, Thermometer, Wind } from 'lucide-react'
import './MallWeatherPage.css'
import { MallWeatherForecastPanel } from './MallWeatherForecastPanel'
import {
  mallWeatherChartSegments,
  clearMallWeatherPendingRefresh,
  loadMallWeatherPendingRefresh,
  mallWeatherFreshnessLabel,
  mallWeatherMetric,
  mallWeatherOverviewPath,
  mallWeatherRefreshKey,
  mallWeatherRefreshDisposition,
  mallWeatherRefreshPath,
  mallWeatherRefreshRequest,
  mallWeatherRefreshResultMessage,
  saveMallWeatherPendingRefresh,
  mallWeatherSkyconLabel,
  parseMallWeatherMallList,
  parseMallWeatherOverview,
  type MallWeatherMall,
  type MallWeatherOverview,
  type MallWeatherPendingRefresh,
  type MallWeatherRefreshRequest,
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
  const mallRequestSequence = useRef(0)
  const overviewRequestSequence = useRef(0)

  const loadMalls = useCallback(async (afterID = 0) => {
    const sequence = ++mallRequestSequence.current
    setMallState('loading')
    setMallError('')
    const search = new URLSearchParams({
      limit: '50',
      status: 'active',
      geocodeStatus: 'confirmed',
      weatherEnabled: 'true',
    })
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
    setMalls((current) => afterID > 0 ? mergeMalls(current, parsed.items) : parsed.items)
    setNextAfterID(parsed.items.length === 50 ? parsed.nextAfterId : 0)
    setMallState('success')
    if (afterID === 0) setSelectedMallID((current) => parsed.items.some((mall) => mall.id === current) ? current : parsed.items[0]?.id || 0)
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
    if (selectedMallID) void loadOverview(selectedMallID)
  }, [loadOverview, selectedMallID])

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

  return (
    <div className="view-stack mall-weather-page">
      <section className="mall-weather-toolbar" aria-label="商场天气筛选">
        <label>
          <span>搜索商场</span>
          <input type="search" value={query} onChange={(event) => setQuery(event.currentTarget.value)} placeholder="名称、编码或地址" />
        </label>
        <label>
          <span>城市</span>
          <select value={city} onChange={(event) => setCity(event.currentTarget.value)}>
            <option value="">全部城市</option>
            {cities.map((item) => <option value={item} key={item}>{item}</option>)}
          </select>
        </label>
        <button type="button" onClick={() => void loadMalls()} disabled={mallState === 'loading'}>
          <RefreshCcw aria-hidden="true" />刷新列表
        </button>
      </section>

      <div className="mall-weather-layout">
        <aside className="workbench-panel mall-weather-malls" aria-label="已启用天气的商场">
          <div className="mall-weather-section-title">
            <div><strong>商场</strong><span>已启用且坐标已确认</span></div>
            <span>{visibleMalls.length} / {malls.length}</span>
          </div>
          {mallState === 'error' && <RequestError message={mallError} onRetry={() => void loadMalls()} />}
          {mallState === 'loading' && malls.length === 0 && <LoadingState label="正在加载商场" />}
          {mallState === 'success' && malls.length === 0 && <EmptyState title="暂无可查询商场" detail="请先启用天气并确认商场坐标。" />}
          {malls.length > 0 && visibleMalls.length === 0 && <EmptyState title="没有匹配结果" detail="请调整名称或城市筛选。" />}
          <div className="mall-weather-mall-list">
            {visibleMalls.map((mall) => (
              <button
                type="button"
                className={mall.id === selectedMallID ? 'mall-weather-mall active' : 'mall-weather-mall'}
                aria-pressed={mall.id === selectedMallID}
                key={mall.id}
                onClick={() => setSelectedMallID(mall.id)}
              >
                <strong>{mall.nameCn}</strong>
                <span>{mall.mallCode} · {mall.city || '城市未填写'}</span>
                <small>{mall.address || '地址未填写'}</small>
              </button>
            ))}
          </div>
          {nextAfterID > 0 && (
            <button type="button" onClick={() => void loadMalls(nextAfterID)} disabled={mallState === 'loading'}>加载更多商场</button>
          )}
        </aside>

        <section className="mall-weather-content">
          {!selectedMall && mallState !== 'loading' && <EmptyState title="请选择商场" detail="选择左侧商场后查看天气概览。" />}
          {selectedMall && (actorID
            ? <ManualRefreshPanel actorID={actorID} mall={selectedMall} client={client} key={`${actorID}:${selectedMall.id}`} />
            : <RequestError message="无法识别当前登录账号，请退出后重新登录再提交天气刷新。" onRetry={() => window.location.reload()} />)}
          {selectedMall && overviewState === 'loading' && !selectedOverview && <LoadingState label={`正在加载${selectedMall.nameCn}天气`} />}
          {selectedMall && overviewState === 'error' && <RequestError message={overviewError} onRetry={() => void loadOverview(selectedMall.id)} />}
          {selectedMall && selectedOverview && <WeatherOverview mall={selectedMall} overview={selectedOverview} refreshing={overviewState === 'loading'} onRefresh={() => void loadOverview(selectedMall.id)} />}
          {selectedMall && <MallWeatherForecastPanel
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

function mergeMalls(current: MallWeatherMall[], incoming: MallWeatherMall[]) {
  const byID = new Map(current.map((mall) => [mall.id, mall]))
  incoming.forEach((mall) => byID.set(mall.id, mall))
  return Array.from(byID.values())
}
