import { type Dispatch, type ReactNode, type SetStateAction, useCallback, useEffect, useRef, useState } from 'react'
import { RefreshCcw, Thermometer } from 'lucide-react'
import { MallWeatherChart } from './MallWeatherChart'
import {
  mallWeatherDailyForecastDays,
  mallWeatherFreshnessLabel,
  mallWeatherHourlyForecastHours,
  mallWeatherMetric,
  mallWeatherMinutelyForecastMinutes,
  mallWeatherSkyconLabel,
  loadMallWeatherForecastDatasets,
  type MallWeatherDaily,
  type MallWeatherHourly,
  type MallWeatherLifeIndex,
  type MallWeatherMeta,
  type MallWeatherMinutely,
} from './mallWeather'

type QueryClient = (
  path: string,
  options: { method: 'GET'; showResult: false; silentLoading: true; signal: AbortSignal },
) => Promise<{ ok: boolean; status: number; data: unknown }>

type QueryState<T> = {
  loading: boolean
  error: string
  items: T[]
  meta: MallWeatherMeta | null
}

const emptyState = <T,>(): QueryState<T> => ({ loading: false, error: '', items: [], meta: null })

export function MallWeatherForecastPanel({ mallID, timeZone, client }: { mallID: number; timeZone: string; client: QueryClient }) {
  const [minutely, setMinutely] = useState<QueryState<MallWeatherMinutely>>(emptyState)
  const [hourly, setHourly] = useState<QueryState<MallWeatherHourly>>(emptyState)
  const [daily, setDaily] = useState<QueryState<MallWeatherDaily>>(emptyState)
  const [life, setLife] = useState<QueryState<MallWeatherLifeIndex>>(emptyState)
  const requestSequence = useRef(0)
  const disposed = useRef(false)
  const activeControllers = useRef(new Set<AbortController>())

  const abortActiveRequests = useCallback(() => {
    for (const controller of activeControllers.current) controller.abort()
    activeControllers.current.clear()
  }, [])

  const load = useCallback(() => {
    abortActiveRequests()
    const sequence = ++requestSequence.current
    setMinutely((state) => ({ ...state, loading: true, error: '' }))
    setHourly((state) => ({ ...state, loading: true, error: '' }))
    setDaily((state) => ({ ...state, loading: true, error: '' }))
    setLife((state) => ({ ...state, loading: true, error: '' }))
    async function request(path: string) {
      const controller = new AbortController()
      activeControllers.current.add(controller)
      const timeout = window.setTimeout(() => controller.abort(), 15_000)
      try {
        return await client(path, {
          method: 'GET',
          showResult: false,
          silentLoading: true,
          signal: controller.signal,
        })
      } finally {
        window.clearTimeout(timeout)
        activeControllers.current.delete(controller)
      }
    }
    let datasets: ReturnType<typeof loadMallWeatherForecastDatasets>
    try {
      const requestedAt = new Date()
      datasets = loadMallWeatherForecastDatasets(request, mallID, timeZone, requestedAt)
    } catch {
      const message = '商场时区无效，无法构造完整预报窗口'
      setMinutely((state) => ({ ...state, loading: false, error: message }))
      setHourly((state) => ({ ...state, loading: false, error: message }))
      setDaily((state) => ({ ...state, loading: false, error: message }))
      setLife((state) => ({ ...state, loading: false, error: message }))
      return
    }
    const isCurrent = () => !disposed.current && sequence === requestSequence.current
    void Promise.all([
      settleDataset(datasets.minutely, isCurrent, setMinutely),
      settleDataset(datasets.hourly, isCurrent, setHourly),
      settleDataset(datasets.daily, isCurrent, setDaily),
      settleDataset(datasets.life, isCurrent, setLife),
    ])
  }, [abortActiveRequests, client, mallID, timeZone])

  useEffect(() => {
    disposed.current = false
    load()
    return () => {
      disposed.current = true
      abortActiveRequests()
    }
  }, [abortActiveRequests, load])

  const loading = minutely.loading || hourly.loading || daily.loading || life.loading
  return (
    <section className="workbench-panel mall-weather-forecast-panel" aria-busy={loading}>
      <div className="mall-weather-section-title">
        <div>
          <strong>完整预报与生活指数</strong>
          <span>
            中心点未来 {mallWeatherMinutelyForecastMinutes} 分钟 · {mallWeatherHourlyForecastHours} 小时 · {mallWeatherDailyForecastDays} 天 · 自动读取全部游标页
          </span>
        </div>
        <button type="button" onClick={load} disabled={loading}><RefreshCcw aria-hidden="true" />{loading ? '加载中' : '重新查询'}</button>
      </div>
      <ForecastDataset id="mall-weather-minutely" title={`中心点未来 ${mallWeatherMinutelyForecastMinutes} 分钟降水（约 1 km 分辨率）`} state={minutely} empty={`未来 ${mallWeatherMinutelyForecastMinutes} 分钟窗口没有分钟级降水预报`}>
        <table className="data-table"><caption className="mall-weather-table-caption">商场中心点未来 {mallWeatherMinutelyForecastMinutes} 分钟约 1 km 分辨率降水明细</caption><thead><tr><th scope="col">时间</th><th scope="col">分钟偏移</th><th scope="col">降水强度</th><th scope="col">概率</th><th scope="col">描述 / 关键点</th><th scope="col">数据源</th><th scope="col">质量</th></tr></thead><tbody>
          {minutely.items.map((item, index) => <tr key={`${item.forecastMinuteUtc}-${index}`}><td>{item.forecastMinuteLocal}</td><td>+{item.minuteOffset} 分钟</td><td>{mallWeatherMetric(item.precipitationMmH, ' mm/h')}</td><td>{mallWeatherMetric(item.probabilityPct, '%', 0)}</td><td>{[item.description, item.forecastKeypoint].filter(Boolean).join(' / ') || '—'}</td><td>{item.datasource || '—'}</td><td>{qualityLabel(item.qualityStatus, item.qualityWarnings.length)}</td></tr>)}
        </tbody></table>
      </ForecastDataset>
      {hourly.items.length > 0 && (
        <MallWeatherChart
          title={`未来 ${mallWeatherHourlyForecastHours} 小时温度趋势`}
          detail="9～13 km 预报网格 · 自动读取全部游标页"
          unit="°C"
          icon={<Thermometer aria-hidden="true" />}
          series={hourly.items.map((item) => ({ time: item.forecastTimeLocal, value: item.temperatureC }))}
          showDetails={false}
        />
      )}
      <ForecastDataset id="mall-weather-hourly" title={`${mallWeatherHourlyForecastHours} 小时逐小时预报`} state={hourly} empty={`未来 ${mallWeatherHourlyForecastHours} 小时窗口没有小时预报`}>
        <table className="data-table"><caption className="mall-weather-table-caption">未来 {mallWeatherHourlyForecastHours} 小时逐小时天气明细（常规变量为 9～13 km 预报网格）</caption><thead><tr><th scope="col">时间</th><th scope="col">天气</th><th scope="col">温度 / 体感</th><th scope="col">湿度 / 云量</th><th scope="col">气压 / 辐射</th><th scope="col">降水 / 概率</th><th scope="col">风速 / 风向</th><th scope="col">能见度</th><th scope="col">PM2.5 / 中美 AQI</th><th scope="col">描述</th><th scope="col">质量</th></tr></thead><tbody>
          {hourly.items.map((item, index) => <tr key={`${item.forecastTimeLocal}-${index}`}><td>{item.forecastTimeLocal}</td><td>{mallWeatherSkyconLabel(item.skycon)}</td><td>{mallWeatherMetric(item.temperatureC, '°C')} / {mallWeatherMetric(item.apparentTemperatureC, '°C')}</td><td>{mallWeatherMetric(item.humidityPct, '%', 0)} / {ratioPercent(item.cloudrateRatio)}</td><td>{mallWeatherMetric(item.pressurePa, ' Pa', 0)} / {mallWeatherMetric(item.dswrfWM2, ' W/m²')}</td><td>{mallWeatherMetric(item.precipitationMmH, ' mm/h')} / {mallWeatherMetric(item.precipitationProbabilityPct, '%', 0)}</td><td>{mallWeatherMetric(item.windSpeedKph, ' km/h')} / {mallWeatherMetric(item.windDirectionDeg, '°', 0)}</td><td>{mallWeatherMetric(item.visibilityKm, ' km')}</td><td>{mallWeatherMetric(item.pm25UgM3, ' μg/m³')} / {mallWeatherMetric(item.aqiChn, '', 0)} / {mallWeatherMetric(item.aqiUsa, '', 0)}</td><td>{item.hourlyDescription || item.forecastKeypoint || '—'}</td><td>{qualityLabel(item.qualityStatus, item.qualityWarnings.length)}</td></tr>)}
        </tbody></table>
      </ForecastDataset>
      <ForecastDataset id="mall-weather-daily" title={`${mallWeatherDailyForecastDays} 日逐日预报`} state={daily} empty={`当前 ${mallWeatherDailyForecastDays} 天窗口没有逐日预报`}>
        <table className="data-table"><caption className="mall-weather-table-caption">未来 {mallWeatherDailyForecastDays} 个本地日逐日全部综合天气字段（9～13 km 预报网格）</caption><thead><tr><th scope="col">日期</th><th scope="col">全天 / 白天 / 夜间天气</th><th scope="col">全天温度 最低 / 最高 / 平均</th><th scope="col">白天温度 最低 / 最高 / 平均</th><th scope="col">夜间温度 最低 / 最高 / 平均</th><th scope="col">全天降水 最低 / 最高 / 平均 / 概率</th><th scope="col">白天降水 最低 / 最高 / 平均 / 概率</th><th scope="col">夜间降水 最低 / 最高 / 平均 / 概率</th><th scope="col">全天风 最低 / 最高 / 平均</th><th scope="col">白天风 最低 / 最高 / 平均</th><th scope="col">夜间风 最低 / 最高 / 平均</th><th scope="col">湿度 最低 / 最高 / 平均</th><th scope="col">云量 最低 / 最高 / 平均</th><th scope="col">气压 最低 / 最高 / 平均</th><th scope="col">能见度 最低 / 最高 / 平均</th><th scope="col">辐射 最低 / 最高 / 平均</th><th scope="col">PM2.5 最低 / 最高 / 平均</th><th scope="col">中国 AQI 最低 / 最高 / 平均</th><th scope="col">美国 AQI 最低 / 最高 / 平均</th><th scope="col">日出 / 日落</th><th scope="col">质量</th></tr></thead><tbody>
          {daily.items.map((item, index) => <tr key={`${item.forecastDateLocal}-${index}`}><td>{item.forecastDateLocal}</td><td>{mallWeatherSkyconLabel(item.skycon)} / {mallWeatherSkyconLabel(item.daySkycon)} / {mallWeatherSkyconLabel(item.nightSkycon)}</td><td>{rangeMetric(item.temperatureMinC, item.temperatureMaxC, item.temperatureAvgC, '°C')}</td><td>{rangeMetric(item.dayTemperatureMinC, item.dayTemperatureMaxC, item.dayTemperatureAvgC, '°C')}</td><td>{rangeMetric(item.nightTemperatureMinC, item.nightTemperatureMaxC, item.nightTemperatureAvgC, '°C')}</td><td>{precipitationMetric(item.precipitationMinMmH, item.precipitationMaxMmH, item.precipitationAvgMmH, item.precipitationProbabilityPct)}</td><td>{precipitationMetric(item.dayPrecipitationMinMmH, item.dayPrecipitationMaxMmH, item.dayPrecipitationAvgMmH, item.dayPrecipitationProbabilityPct)}</td><td>{precipitationMetric(item.nightPrecipitationMinMmH, item.nightPrecipitationMaxMmH, item.nightPrecipitationAvgMmH, item.nightPrecipitationProbabilityPct)}</td><td>{windRangeMetric(item.windMinSpeedKph, item.windMinDirectionDeg, item.windMaxSpeedKph, item.windMaxDirectionDeg, item.windAvgSpeedKph, item.windAvgDirectionDeg)}</td><td>{windRangeMetric(item.dayWindMinSpeedKph, item.dayWindMinDirectionDeg, item.dayWindMaxSpeedKph, item.dayWindMaxDirectionDeg, item.dayWindAvgSpeedKph, item.dayWindAvgDirectionDeg)}</td><td>{windRangeMetric(item.nightWindMinSpeedKph, item.nightWindMinDirectionDeg, item.nightWindMaxSpeedKph, item.nightWindMaxDirectionDeg, item.nightWindAvgSpeedKph, item.nightWindAvgDirectionDeg)}</td><td>{rangeMetric(item.humidityMinPct, item.humidityMaxPct, item.humidityAvgPct, '%', 0)}</td><td>{ratioRangeMetric(item.cloudrateMinRatio, item.cloudrateMaxRatio, item.cloudrateAvgRatio)}</td><td>{rangeMetric(item.pressureMinPa, item.pressureMaxPa, item.pressureAvgPa, ' Pa', 0)}</td><td>{rangeMetric(item.visibilityMinKm, item.visibilityMaxKm, item.visibilityAvgKm, ' km')}</td><td>{rangeMetric(item.dswrfMinWM2, item.dswrfMaxWM2, item.dswrfAvgWM2, ' W/m²')}</td><td>{rangeMetric(item.pm25MinUgM3, item.pm25MaxUgM3, item.pm25AvgUgM3, ' μg/m³')}</td><td>{rangeMetric(item.aqiMinChn, item.aqiMaxChn, item.aqiAvgChn, '', 0)}</td><td>{rangeMetric(item.aqiMinUsa, item.aqiMaxUsa, item.aqiAvgUsa, '', 0)}</td><td>{item.sunriseLocalTime || '—'} / {item.sunsetLocalTime || '—'}</td><td>{qualityLabel(item.qualityStatus, item.qualityWarnings.length)}</td></tr>)}
        </tbody></table>
      </ForecastDataset>
      <ForecastDataset id="mall-weather-life-indices" title={`${mallWeatherDailyForecastDays} 日生活指数`} state={life} empty={`当前 ${mallWeatherDailyForecastDays} 天窗口没有生活指数`}>
        <table className="data-table mall-weather-life-table"><caption className="mall-weather-table-caption">未来 {mallWeatherDailyForecastDays} 个本地日全部生活指数明细</caption><thead><tr><th scope="col">日期</th><th scope="col">指数</th><th scope="col">等级</th><th scope="col">建议</th><th scope="col">来源</th><th scope="col">质量</th></tr></thead><tbody>
          {life.items.map((item, index) => <tr key={`${item.forecastDateLocal}-${item.sourceApi}-${item.indexType}-${index}`}><td>{item.forecastDateLocal}</td><td>{item.indexName || item.indexCode}{item.isUnknownType ? '（未知类型）' : ''}</td><td>{item.level ?? '—'}</td><td>{item.shortDescription || item.detail || '—'}</td><td>{item.sourceApi}</td><td>{qualityLabel(item.qualityStatus, item.qualityWarnings.length)}</td></tr>)}
        </tbody></table>
      </ForecastDataset>
    </section>
  )
}

function ForecastDataset<T>({ id, title, state, empty, children }: { id?: string; title: string; state: QueryState<T>; empty: string; children: ReactNode }) {
  return (
    <details id={id} className="mall-weather-forecast-dataset" tabIndex={-1} open>
      <summary>{title}（{state.items.length} 条）{state.meta ? ` · ${mallWeatherFreshnessLabel(state.meta.freshnessStatus)}` : ''}</summary>
      {state.loading && state.items.length === 0 && <p role="status">正在加载全部分页…</p>}
      {state.error && <p className="mall-weather-action-message error" role="alert">{state.error}</p>}
      {!state.loading && !state.error && state.items.length === 0 && <p role="status">{empty}</p>}
      {state.items.length > 0 && (
        <div className="data-table-wrap" role="region" aria-label={`${title}数据表，可横向滚动`} tabIndex={0}>
          {children}
        </div>
      )}
    </details>
  )
}

async function settleDataset<T>(
  request: Promise<{ items: T[]; meta: MallWeatherMeta | null }>,
  isCurrent: () => boolean,
  update: Dispatch<SetStateAction<QueryState<T>>>,
) {
  try {
    const result = await request
    if (isCurrent()) update({ loading: false, error: '', items: result.items, meta: result.meta })
  } catch (error) {
    if (isCurrent()) update((current) => ({ ...current, loading: false, error: error instanceof Error ? error.message : '查询失败' }))
  }
}

function qualityLabel(status: string, warningCount: number) {
  return `${status || '未知'}${warningCount > 0 ? ` · ${warningCount} 项告警` : ''}`
}

function rangeMetric(minimum: number | undefined, maximum: number | undefined, average: number | undefined, unit: string, digits = 1) {
  return `${mallWeatherMetric(minimum, unit, digits)} / ${mallWeatherMetric(maximum, unit, digits)} / ${mallWeatherMetric(average, unit, digits)}`
}

function ratioPercent(value: number | undefined) {
  return mallWeatherMetric(value === undefined ? undefined : value * 100, '%', 0)
}

function ratioRangeMetric(minimum: number | undefined, maximum: number | undefined, average: number | undefined) {
  return `${ratioPercent(minimum)} / ${ratioPercent(maximum)} / ${ratioPercent(average)}`
}

function precipitationMetric(minimum: number | undefined, maximum: number | undefined, average: number | undefined, probability: number | undefined) {
  return `${rangeMetric(minimum, maximum, average, ' mm/h')} / ${mallWeatherMetric(probability, '%', 0)}`
}

function windRangeMetric(minSpeed: number | undefined, minDirection: number | undefined, maxSpeed: number | undefined, maxDirection: number | undefined, avgSpeed: number | undefined, avgDirection: number | undefined) {
  const wind = (speed: number | undefined, direction: number | undefined) => `${mallWeatherMetric(speed, ' km/h')} @ ${mallWeatherMetric(direction, '°', 0)}`
  return `${wind(minSpeed, minDirection)} / ${wind(maxSpeed, maxDirection)} / ${wind(avgSpeed, avgDirection)}`
}
