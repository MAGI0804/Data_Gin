import { type Dispatch, type ReactNode, type SetStateAction, useCallback, useEffect, useRef, useState } from 'react'
import { RefreshCcw } from 'lucide-react'
import {
  mallWeatherFreshnessLabel,
  mallWeatherForecastQueryWindows,
  mallWeatherMetric,
  mallWeatherSkyconLabel,
  loadAllMallWeatherPages,
  parseMallWeatherDailyPage,
  parseMallWeatherHourlyPage,
  parseMallWeatherLifeIndexPage,
  type MallWeatherDaily,
  type MallWeatherForecastWindows,
  type MallWeatherHourly,
  type MallWeatherLifeIndex,
  type MallWeatherMeta,
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
    setHourly((state) => ({ ...state, loading: true, error: '' }))
    setDaily((state) => ({ ...state, loading: true, error: '' }))
    setLife((state) => ({ ...state, loading: true, error: '' }))
    let windows: MallWeatherForecastWindows
    try {
      windows = mallWeatherForecastQueryWindows(new Date(), timeZone)
    } catch {
      const message = '商场时区无效，无法构造完整预报窗口'
      setHourly((state) => ({ ...state, loading: false, error: message }))
      setDaily((state) => ({ ...state, loading: false, error: message }))
      setLife((state) => ({ ...state, loading: false, error: message }))
      return
    }
    const asOf = new Date()
    const request = async (path: string) => {
      const controller = new AbortController()
      activeControllers.current.add(controller)
      const timeout = window.setTimeout(() => controller.abort(), 15_000)
      try {
        return await client(path, { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal })
      } finally {
        window.clearTimeout(timeout)
        activeControllers.current.delete(controller)
      }
    }
    const isCurrent = () => !disposed.current && sequence === requestSequence.current
    void Promise.all([
      settleDataset(loadAllMallWeatherPages(request, mallID, 'hourly', windows.hourly, timeZone, asOf, parseMallWeatherHourlyPage), isCurrent, setHourly),
      settleDataset(loadAllMallWeatherPages(request, mallID, 'daily', windows.daily, timeZone, asOf, parseMallWeatherDailyPage), isCurrent, setDaily),
      settleDataset(loadAllMallWeatherPages(request, mallID, 'life-indices', windows.daily, timeZone, asOf, parseMallWeatherLifeIndexPage), isCurrent, setLife),
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

  const loading = hourly.loading || daily.loading || life.loading
  return (
    <section className="workbench-panel mall-weather-forecast-panel" aria-busy={loading}>
      <div className="mall-weather-section-title">
        <div><strong>完整预报与生活指数</strong><span>未来 360 小时 · 15 天 · 自动读取全部游标页</span></div>
        <button type="button" onClick={load} disabled={loading}><RefreshCcw aria-hidden="true" />{loading ? '加载中' : '重新查询'}</button>
      </div>
      <ForecastDataset title="360 小时逐小时预报" state={hourly} empty="未来 360 小时窗口没有小时预报">
        <table className="data-table"><caption className="mall-weather-table-caption">未来 360 小时逐小时天气明细</caption><thead><tr><th scope="col">时间</th><th scope="col">天气</th><th scope="col">温度</th><th scope="col">降水</th><th scope="col">风速</th><th scope="col">AQI</th><th scope="col">质量</th></tr></thead><tbody>
          {hourly.items.map((item, index) => <tr key={`${item.forecastTimeLocal}-${index}`}><td>{item.forecastTimeLocal}</td><td>{mallWeatherSkyconLabel(item.skycon)}</td><td>{mallWeatherMetric(item.temperatureC, '°C')}</td><td>{mallWeatherMetric(item.precipitationMmH, ' mm/h')}</td><td>{mallWeatherMetric(item.windSpeedKph, ' km/h')}</td><td>{mallWeatherMetric(item.aqiChn, '', 0)}</td><td>{qualityLabel(item.qualityStatus, item.qualityWarnings.length)}</td></tr>)}
        </tbody></table>
      </ForecastDataset>
      <ForecastDataset title="15 日逐日预报" state={daily} empty="当前 15 天窗口没有逐日预报">
        <table className="data-table"><caption className="mall-weather-table-caption">未来 15 个本地日逐日天气明细</caption><thead><tr><th scope="col">日期</th><th scope="col">白天 / 夜间</th><th scope="col">最低 / 最高</th><th scope="col">降水概率</th><th scope="col">最大风速</th><th scope="col">日出 / 日落</th><th scope="col">质量</th></tr></thead><tbody>
          {daily.items.map((item, index) => <tr key={`${item.forecastDateLocal}-${index}`}><td>{item.forecastDateLocal}</td><td>{mallWeatherSkyconLabel(item.daySkycon)} / {mallWeatherSkyconLabel(item.nightSkycon)}</td><td>{mallWeatherMetric(item.temperatureMinC, '°C')} / {mallWeatherMetric(item.temperatureMaxC, '°C')}</td><td>{mallWeatherMetric(item.precipitationProbabilityPct, '%', 0)}</td><td>{mallWeatherMetric(item.windMaxSpeedKph, ' km/h')}</td><td>{item.sunriseLocalTime || '—'} / {item.sunsetLocalTime || '—'}</td><td>{qualityLabel(item.qualityStatus, item.qualityWarnings.length)}</td></tr>)}
        </tbody></table>
      </ForecastDataset>
      <ForecastDataset title="15 日生活指数" state={life} empty="当前 15 天窗口没有生活指数">
        <table className="data-table mall-weather-life-table"><caption className="mall-weather-table-caption">未来 15 个本地日全部生活指数明细</caption><thead><tr><th scope="col">日期</th><th scope="col">指数</th><th scope="col">等级</th><th scope="col">建议</th><th scope="col">来源</th><th scope="col">质量</th></tr></thead><tbody>
          {life.items.map((item, index) => <tr key={`${item.forecastDateLocal}-${item.sourceApi}-${item.indexType}-${index}`}><td>{item.forecastDateLocal}</td><td>{item.indexName || item.indexCode}{item.isUnknownType ? '（未知类型）' : ''}</td><td>{item.level ?? '—'}</td><td>{item.shortDescription || item.detail || '—'}</td><td>{item.sourceApi}</td><td>{qualityLabel(item.qualityStatus, item.qualityWarnings.length)}</td></tr>)}
        </tbody></table>
      </ForecastDataset>
    </section>
  )
}

function ForecastDataset<T>({ title, state, empty, children }: { title: string; state: QueryState<T>; empty: string; children: ReactNode }) {
  return (
    <details className="mall-weather-forecast-dataset" open>
      <summary>{title}（{state.items.length} 条）{state.meta ? ` · ${mallWeatherFreshnessLabel(state.meta.freshnessStatus)}` : ''}</summary>
      {state.loading && state.items.length === 0 && <p role="status">正在加载全部分页…</p>}
      {state.error && <p className="mall-weather-action-message error" role="alert">{state.error}</p>}
      {!state.loading && !state.error && state.items.length === 0 && <p role="status">{empty}</p>}
      {state.items.length > 0 && <div className="data-table-wrap">{children}</div>}
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
