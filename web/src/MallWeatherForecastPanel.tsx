import { type Dispatch, type ReactNode, type SetStateAction, useCallback, useEffect, useRef, useState } from 'react'
import { CloudRain, Droplets, RefreshCcw, Sparkles, Thermometer, Wind } from 'lucide-react'
import { MallWeatherChart, type MallWeatherChartSeries } from './MallWeatherChart'
import { createMallWeatherChartCsv, downloadMallWeatherBytes, mallWeatherChartCsvFileName } from './mallWeatherCsv'
import {
  mallWeatherDailyForecastDays,
  mallWeatherFreshnessLabel,
  mallWeatherHourlyForecastHours,
  mallWeatherMinutelyForecastMinutes,
  loadMallWeatherForecastDatasets,
  type MallWeatherDaily,
  type MallWeatherHourly,
  type MallWeatherLifeIndex,
  type MallWeatherMeta,
  type MallWeatherMinutely,
} from './mallWeather'
import styles from './MallWeatherForecastPanel.module.css'
import { weatherChartPalette } from './weatherChartPalette'

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

export type MallWeatherForecastDataSnapshot = {
  mallID: number
  minutely: MallWeatherMinutely[]
  hourly: MallWeatherHourly[]
  daily: MallWeatherDaily[]
  lifeIndices: MallWeatherLifeIndex[]
  loading: boolean
  ready: boolean
  error: string
}

type MallWeatherForecastPanelProps = {
  mallID: number
  mallCode: string
  mallName: string
  timeZone: string
  client: QueryClient
  onDatasetsChange: (snapshot: MallWeatherForecastDataSnapshot) => void
}

const emptyState = <T,>(): QueryState<T> => ({ loading: true, error: '', items: [], meta: null })

export function MallWeatherForecastPanel({
  mallID,
  mallCode,
  mallName,
  timeZone,
  client,
  onDatasetsChange,
}: MallWeatherForecastPanelProps) {
  const [minutely, setMinutely] = useState<QueryState<MallWeatherMinutely>>(emptyState)
  const [hourly, setHourly] = useState<QueryState<MallWeatherHourly>>(emptyState)
  const [daily, setDaily] = useState<QueryState<MallWeatherDaily>>(emptyState)
  const [life, setLife] = useState<QueryState<MallWeatherLifeIndex>>(emptyState)
  const [datasetMallID, setDatasetMallID] = useState(mallID)
  const [csvError, setCsvError] = useState('')
  const requestSequence = useRef(0)
  const disposed = useRef(false)
  const activeControllers = useRef(new Set<AbortController>())
  const datasetMallIDRef = useRef(mallID)
  const onDatasetsChangeRef = useRef(onDatasetsChange)

  useEffect(() => {
    onDatasetsChangeRef.current = onDatasetsChange
  }, [onDatasetsChange])

  const abortActiveRequests = useCallback(() => {
    for (const controller of activeControllers.current) controller.abort()
    activeControllers.current.clear()
  }, [])

  const load = useCallback(() => {
    abortActiveRequests()
    const sequence = ++requestSequence.current
    const mallChanged = datasetMallIDRef.current !== mallID
    datasetMallIDRef.current = mallID
    setDatasetMallID(mallID)
    setCsvError('')
    setMinutely((state) => beginDatasetLoad(state, mallChanged))
    setHourly((state) => beginDatasetLoad(state, mallChanged))
    setDaily((state) => beginDatasetLoad(state, mallChanged))
    setLife((state) => beginDatasetLoad(state, mallChanged))
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

  const currentMallData = datasetMallID === mallID
  const loading = !currentMallData || minutely.loading || hourly.loading || daily.loading || life.loading
  const datasetError = currentMallData
    ? [
        minutely.error ? `分钟预报：${minutely.error}` : '',
        hourly.error ? `小时预报：${hourly.error}` : '',
        daily.error ? `逐日预报：${daily.error}` : '',
        life.error ? `生活指数：${life.error}` : '',
      ].filter(Boolean).join('；')
    : ''

  useEffect(() => {
    const snapshot: MallWeatherForecastDataSnapshot = {
      mallID,
      minutely: currentMallData ? minutely.items : [],
      hourly: currentMallData ? hourly.items : [],
      daily: currentMallData ? daily.items : [],
      lifeIndices: currentMallData ? life.items : [],
      loading,
      ready: currentMallData && !loading && !datasetError,
      error: datasetError,
    }
    onDatasetsChangeRef.current(snapshot)
  }, [currentMallData, daily.items, datasetError, hourly.items, life.items, loading, mallID, minutely.items])

  const downloadChartCsv = useCallback((chartID: string, unit: string, series: MallWeatherChartSeries[]) => {
    setCsvError('')
    try {
      downloadMallWeatherBytes(
        createMallWeatherChartCsv(series.map((item) => ({ ...item, unit })), { mallCode, mallName }),
        mallWeatherChartCsvFileName(chartID, mallCode),
      )
    } catch {
      setCsvError('CSV 文件生成失败，请重新查询数据后重试。')
    }
  }, [mallCode, mallName])

  const minutelyDownloadDisabled = !currentMallData || minutely.loading || Boolean(minutely.error)
  const hourlyDownloadDisabled = !currentMallData || hourly.loading || Boolean(hourly.error)
  const dailyDownloadDisabled = !currentMallData || daily.loading || Boolean(daily.error)
  const lifeDownloadDisabled = !currentMallData || life.loading || Boolean(life.error)
  const lifeSeries = buildLifeIndexSeries(life.items)

  return (
    <section className={[styles['workbench-panel'], styles['mall-weather-forecast-panel']].join(' ')} aria-busy={loading}>
      <div className={styles['mall-weather-section-title']}>
        <div>
          <strong>完整预报与生活指数</strong>
          <span>
            中心点未来 {mallWeatherMinutelyForecastMinutes} 分钟 · 最多 {mallWeatherHourlyForecastHours} 小时 · {mallWeatherDailyForecastDays} 天 · 自动读取全部游标页
          </span>
        </div>
        <button type="button" onClick={load} disabled={loading}><RefreshCcw aria-hidden="true" />{loading ? '加载中' : '重新查询'}</button>
      </div>
      {csvError && <p className={[styles['mall-weather-action-message'], styles['error']].join(' ')} role="alert">{csvError}</p>}
      <ForecastDataset
        id="mall-weather-minutely"
        title={`中心点未来 ${mallWeatherMinutelyForecastMinutes} 分钟降水（约 1 km 分辨率）`}
        state={minutely}
        empty={`未来 ${mallWeatherMinutelyForecastMinutes} 分钟窗口没有分钟级降水预报`}
      >
        <div className={styles['mall-weather-forecast-charts']}>
          <MallWeatherChart title="分钟级降水强度" detail={`未来 ${mallWeatherMinutelyForecastMinutes} 分钟`} unit="mm/h" icon={<CloudRain aria-hidden="true" />} floorZero
            series={[chartSeries('降水强度', weatherChartPalette.precipitation, minutely.items, 'forecastMinuteLocal', 'precipitationMmH')]}
            onDownload={(series) => downloadChartCsv('minutely_precipitation', 'mm/h', series)} downloadDisabled={minutelyDownloadDisabled} />
          <MallWeatherChart title="分钟级降水概率" detail={`未来 ${mallWeatherMinutelyForecastMinutes} 分钟`} unit="%" icon={<Droplets aria-hidden="true" />} floorZero
            series={[chartSeries('降水概率', weatherChartPalette.probability, minutely.items, 'forecastMinuteLocal', 'probabilityPct')]}
            onDownload={(series) => downloadChartCsv('minutely_probability', '%', series)} downloadDisabled={minutelyDownloadDisabled} />
        </div>
      </ForecastDataset>
      <ForecastDataset
        id="mall-weather-hourly"
        title={`未来逐小时预报（目标 ${mallWeatherHourlyForecastHours} 小时）`}
        state={hourly}
        empty={`未来 ${mallWeatherHourlyForecastHours} 小时窗口没有小时预报`}
        notice={!hourly.loading && !hourly.error && hourly.items.length > 0 && hourly.items.length < mallWeatherHourlyForecastHours
          ? `当前服务端可用 ${hourly.items.length} / ${mallWeatherHourlyForecastHours} 条连续逐小时数据，已展示全部可用内容。`
          : ''}
      >
        <div className={styles['mall-weather-forecast-charts']}>
          <MallWeatherChart title="温度趋势" detail="逐小时预报" unit="°C" icon={<Thermometer aria-hidden="true" />}
            series={[
              chartSeries('温度', weatherChartPalette.temperature, hourly.items, 'forecastTimeLocal', 'temperatureC'),
              chartSeries('体感温度', weatherChartPalette.apparentTemperature, hourly.items, 'forecastTimeLocal', 'apparentTemperatureC', '6 4'),
            ]} onDownload={(series) => downloadChartCsv('hourly_temperature', '°C', series)} downloadDisabled={hourlyDownloadDisabled} />
          <MallWeatherChart title="降水趋势" detail="逐小时预报" unit="mm/h" icon={<CloudRain aria-hidden="true" />} floorZero
            series={[chartSeries('降水强度', weatherChartPalette.precipitation, hourly.items, 'forecastTimeLocal', 'precipitationMmH')]}
            onDownload={(series) => downloadChartCsv('hourly_precipitation', 'mm/h', series)} downloadDisabled={hourlyDownloadDisabled} />
          <MallWeatherChart title="湿度与降水概率" detail="逐小时预报" unit="%" icon={<Droplets aria-hidden="true" />} floorZero
            series={[
              chartSeries('相对湿度', weatherChartPalette.probability, hourly.items, 'forecastTimeLocal', 'humidityPct'),
              chartSeries('降水概率', weatherChartPalette.probabilityDark, hourly.items, 'forecastTimeLocal', 'precipitationProbabilityPct', '6 4'),
            ]} onDownload={(series) => downloadChartCsv('hourly_humidity_probability', '%', series)} downloadDisabled={hourlyDownloadDisabled} />
          <MallWeatherChart title="风速趋势" detail="逐小时预报" unit="km/h" icon={<Wind aria-hidden="true" />} floorZero
            series={[chartSeries('风速', weatherChartPalette.wind, hourly.items, 'forecastTimeLocal', 'windSpeedKph')]}
            onDownload={(series) => downloadChartCsv('hourly_wind_speed', 'km/h', series)} downloadDisabled={hourlyDownloadDisabled} />
        </div>
      </ForecastDataset>
      <ForecastDataset
        id="mall-weather-daily"
        title={`${mallWeatherDailyForecastDays} 日逐日预报`}
        state={daily}
        empty={`当前 ${mallWeatherDailyForecastDays} 天窗口没有逐日预报`}
      >
        <div className={styles['mall-weather-forecast-charts']}>
          <MallWeatherChart title="每日温度区间" detail={`未来 ${mallWeatherDailyForecastDays} 日`} unit="°C" icon={<Thermometer aria-hidden="true" />}
            series={dailyRangeSeries(daily.items, 'temperatureMinC', 'temperatureAvgC', 'temperatureMaxC', [weatherChartPalette.probability, weatherChartPalette.temperature, weatherChartPalette.apparentTemperature])}
            onDownload={(series) => downloadChartCsv('daily_temperature', '°C', series)} downloadDisabled={dailyDownloadDisabled} />
          <MallWeatherChart title="每日降水区间" detail={`未来 ${mallWeatherDailyForecastDays} 日`} unit="mm/h" icon={<CloudRain aria-hidden="true" />} floorZero
            series={dailyRangeSeries(daily.items, 'precipitationMinMmH', 'precipitationAvgMmH', 'precipitationMaxMmH', [weatherChartPalette.precipitationLight, weatherChartPalette.precipitationAverage, weatherChartPalette.precipitationDark])}
            onDownload={(series) => downloadChartCsv('daily_precipitation', 'mm/h', series)} downloadDisabled={dailyDownloadDisabled} />
          <MallWeatherChart title="每日湿度区间" detail={`未来 ${mallWeatherDailyForecastDays} 日`} unit="%" icon={<Droplets aria-hidden="true" />} floorZero
            series={dailyRangeSeries(daily.items, 'humidityMinPct', 'humidityAvgPct', 'humidityMaxPct', [weatherChartPalette.probability, weatherChartPalette.humidity, weatherChartPalette.humidityDark])}
            onDownload={(series) => downloadChartCsv('daily_humidity', '%', series)} downloadDisabled={dailyDownloadDisabled} />
          <MallWeatherChart title="每日风速区间" detail={`未来 ${mallWeatherDailyForecastDays} 日`} unit="km/h" icon={<Wind aria-hidden="true" />} floorZero
            series={dailyRangeSeries(daily.items, 'windMinSpeedKph', 'windAvgSpeedKph', 'windMaxSpeedKph', [weatherChartPalette.wind, weatherChartPalette.windDark, weatherChartPalette.windDeep])}
            onDownload={(series) => downloadChartCsv('daily_wind_speed', 'km/h', series)} downloadDisabled={dailyDownloadDisabled} />
        </div>
      </ForecastDataset>
      <ForecastDataset
        id="mall-weather-life-indices"
        title={`${mallWeatherDailyForecastDays} 日生活指数`}
        state={life}
        empty={`当前 ${mallWeatherDailyForecastDays} 天窗口没有生活指数`}
      >
        <div className={[styles['mall-weather-forecast-charts'], styles['single']].join(' ')}>
          <MallWeatherChart title="生活指数等级趋势" detail="按日期对比全部指数" unit="级" icon={<Sparkles aria-hidden="true" />} floorZero
            series={lifeSeries} onDownload={(series) => downloadChartCsv('life_indices', '级', series)} downloadDisabled={lifeDownloadDisabled} />
        </div>
      </ForecastDataset>
    </section>
  )
}

function ForecastDataset<T>({ id, title, state, empty, notice = '', children }: {
  id?: string
  title: string
  state: QueryState<T>
  empty: string
  notice?: string
  children: ReactNode
}) {
  return (
    <section id={id} className={styles['mall-weather-forecast-dataset']} tabIndex={-1}>
      <header><strong>{title}</strong><span>{state.items.length} 条{state.meta ? ` · ${mallWeatherFreshnessLabel(state.meta.freshnessStatus)}` : ''}</span></header>
      {state.loading && state.items.length === 0 && <p role="status">正在加载全部分页…</p>}
      {state.error && <p className={[styles['mall-weather-action-message'], styles['error']].join(' ')} role="alert">{state.error}</p>}
      {!state.loading && !state.error && state.items.length === 0 && <p role="status">{empty}</p>}
      {notice && <p className={styles['mall-weather-action-message']} role="status">{notice}</p>}
      {state.items.length > 0 && children}
    </section>
  )
}

function beginDatasetLoad<T>(state: QueryState<T>, clearData: boolean): QueryState<T> {
  return {
    loading: true,
    error: '',
    items: clearData ? [] : state.items,
    meta: clearData ? null : state.meta,
  }
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

function chartSeries<T>(
  name: string,
  color: string,
  items: T[],
  timeKey: keyof T,
  valueKey: keyof T,
  dash?: string,
): MallWeatherChartSeries {
  return {
    id: String(valueKey),
    name,
    color,
    dash,
    data: items.map((item) => ({
      time: String(item[timeKey] ?? ''),
      value: typeof item[valueKey] === 'number' ? item[valueKey] as number : undefined,
    })),
  }
}

function dailyRangeSeries(
  items: MallWeatherDaily[],
  minimumKey: keyof MallWeatherDaily,
  averageKey: keyof MallWeatherDaily,
  maximumKey: keyof MallWeatherDaily,
  colors: [string, string, string],
): MallWeatherChartSeries[] {
  return [
    chartSeries('最低', colors[0], items, 'forecastDateLocal', minimumKey, '6 4'),
    chartSeries('平均', colors[1], items, 'forecastDateLocal', averageKey),
    chartSeries('最高', colors[2], items, 'forecastDateLocal', maximumKey, '2 3'),
  ]
}

function buildLifeIndexSeries(items: MallWeatherLifeIndex[]): MallWeatherChartSeries[] {
  const dates = [...new Set(items.map((item) => item.forecastDateLocal))].sort()
  const groups = new Map<string, MallWeatherLifeIndex[]>()
  for (const item of items) {
    const key = item.indexCode || String(item.indexType)
    groups.set(key, [...(groups.get(key) ?? []), item])
  }
  const colors = weatherChartPalette.life
  const dashPatterns = [undefined, '6 4', '2 3', '8 3 2 3']
  return [...groups.entries()].map(([code, group], index) => {
    const byDate = new Map(group.map((item) => [item.forecastDateLocal, item]))
    return {
      id: code,
      name: group[0]?.indexName || code,
      color: colors[index % colors.length],
      dash: dashPatterns[Math.floor(index / colors.length) % dashPatterns.length],
      data: dates.map((date) => ({ time: date, value: byDate.get(date)?.level })),
    }
  })
}
