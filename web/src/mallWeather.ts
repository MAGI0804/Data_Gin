export type MallWeatherMall = {
  id: number
  mallCode: string
  nameCn: string
  city: string
  address: string
  geocodeStatus: string
  weatherEnabled: boolean
  status: string
}

export type MallWeatherMallList = {
  items: MallWeatherMall[]
  nextAfterId: number
}

export type MallWeatherWarning = {
  code: string
  path: string
}

export type MallWeatherMeta = {
  provider: string
  apiVersion: string
  representativePoint: string
  longitude: number
  latitude: number
  coordinateSystem: string
  samplingMode: string
  coverageRadiusM: number
  spatialResolution: string
  timeZone: string
  unit: string
  freshnessStatus: string
  dataAgeSeconds?: number
}

export type MallWeatherRealtime = {
  snapshotAtLocal: string
  providerServerTimeLocal: string
  fetchedAtLocal: string
  temperatureC?: number
  apparentTemperatureC?: number
  humidityPct?: number
  windSpeedKph?: number
  localPrecipitationMmH?: number
  visibilityKm?: number
  skycon?: string
  pm25UgM3?: number
  pm10UgM3?: number
  o3UgM3?: number
  so2UgM3?: number
  no2UgM3?: number
  coMgM3?: number
  aqiChn?: number
  aqiDescriptionChn?: string
  qualityStatus: string
  qualityWarnings: MallWeatherWarning[]
}

export type MallWeatherMinutely = {
  forecastMinuteLocal: string
  minuteOffset: number
  precipitationMmH?: number
  probabilityPct?: number
  description?: string
  forecastKeypoint?: string
  qualityStatus: string
  qualityWarnings: MallWeatherWarning[]
}

export type MallWeatherHourly = {
  forecastTimeLocal: string
  temperatureC?: number
  precipitationMmH?: number
  precipitationProbabilityPct?: number
  windSpeedKph?: number
  skycon?: string
  pm25UgM3?: number
  aqiChn?: number
  qualityStatus: string
  qualityWarnings: MallWeatherWarning[]
}

export type MallWeatherAlert = {
  alertId: string
  status: string
  title: string
  description?: string
  alertTypeName?: string
  alertLevelName?: string
  publishedAtLocal?: string
  source?: string
  qualityStatus: string
  qualityWarnings: MallWeatherWarning[]
}

export type MallWeatherOverview = {
  realtime: MallWeatherRealtime | null
  minutely: MallWeatherMinutely[]
  hourly: MallWeatherHourly[]
  alerts: MallWeatherAlert[]
  meta: MallWeatherMeta
}

type JsonRecord = Record<string, unknown>

function isRecord(value: unknown): value is JsonRecord {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function envelopeData(payload: unknown): JsonRecord | null {
  if (!isRecord(payload) || (payload.code !== 0 && payload.code !== 200) || !isRecord(payload.data)) return null
  return payload.data
}

function textValue(record: JsonRecord, key: string) {
  return typeof record[key] === 'string' ? record[key] : ''
}

function numberValue(record: JsonRecord, key: string) {
  const value = record[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

function warningValues(value: unknown): MallWeatherWarning[] {
  if (!Array.isArray(value)) return []
  return value.flatMap((warning) => {
    if (!isRecord(warning) || typeof warning.code !== 'string' || typeof warning.path !== 'string') return []
    return [{ code: warning.code, path: warning.path }]
  })
}

function mallWeatherMeta(record: JsonRecord): MallWeatherMeta {
  return {
    provider: textValue(record, 'provider'),
    apiVersion: textValue(record, 'apiVersion'),
    representativePoint: textValue(record, 'representativePoint'),
    longitude: numberValue(record, 'longitude') ?? 0,
    latitude: numberValue(record, 'latitude') ?? 0,
    coordinateSystem: textValue(record, 'coordinateSystem'),
    samplingMode: textValue(record, 'samplingMode'),
    coverageRadiusM: numberValue(record, 'coverageRadiusM') ?? 0,
    spatialResolution: textValue(record, 'spatialResolution'),
    timeZone: textValue(record, 'timeZone'),
    unit: textValue(record, 'unit'),
    freshnessStatus: textValue(record, 'freshnessStatus'),
    dataAgeSeconds: numberValue(record, 'dataAgeSeconds'),
  }
}

function mallWeatherRealtime(record: JsonRecord): MallWeatherRealtime {
  return {
    snapshotAtLocal: textValue(record, 'snapshotAtLocal'),
    providerServerTimeLocal: textValue(record, 'providerServerTimeLocal'),
    fetchedAtLocal: textValue(record, 'fetchedAtLocal'),
    temperatureC: numberValue(record, 'temperatureC'),
    apparentTemperatureC: numberValue(record, 'apparentTemperatureC'),
    humidityPct: numberValue(record, 'humidityPct'),
    windSpeedKph: numberValue(record, 'windSpeedKph'),
    localPrecipitationMmH: numberValue(record, 'localPrecipitationMmH'),
    visibilityKm: numberValue(record, 'visibilityKm'),
    skycon: textValue(record, 'skycon'),
    pm25UgM3: numberValue(record, 'pm25UgM3'),
    pm10UgM3: numberValue(record, 'pm10UgM3'),
    o3UgM3: numberValue(record, 'o3UgM3'),
    so2UgM3: numberValue(record, 'so2UgM3'),
    no2UgM3: numberValue(record, 'no2UgM3'),
    coMgM3: numberValue(record, 'coMgM3'),
    aqiChn: numberValue(record, 'aqiChn'),
    aqiDescriptionChn: textValue(record, 'aqiDescriptionChn'),
    qualityStatus: textValue(record, 'qualityStatus'),
    qualityWarnings: warningValues(record.qualityWarnings),
  }
}

function mallWeatherMinutely(record: JsonRecord): MallWeatherMinutely {
  return {
    forecastMinuteLocal: textValue(record, 'forecastMinuteLocal'),
    minuteOffset: numberValue(record, 'minuteOffset') ?? 0,
    precipitationMmH: numberValue(record, 'precipitationMmH'),
    probabilityPct: numberValue(record, 'probabilityPct'),
    description: textValue(record, 'description'),
    forecastKeypoint: textValue(record, 'forecastKeypoint'),
    qualityStatus: textValue(record, 'qualityStatus'),
    qualityWarnings: warningValues(record.qualityWarnings),
  }
}

function mallWeatherHourly(record: JsonRecord): MallWeatherHourly {
  return {
    forecastTimeLocal: textValue(record, 'forecastTimeLocal'),
    temperatureC: numberValue(record, 'temperatureC'),
    precipitationMmH: numberValue(record, 'precipitationMmH'),
    precipitationProbabilityPct: numberValue(record, 'precipitationProbabilityPct'),
    windSpeedKph: numberValue(record, 'windSpeedKph'),
    skycon: textValue(record, 'skycon'),
    pm25UgM3: numberValue(record, 'pm25UgM3'),
    aqiChn: numberValue(record, 'aqiChn'),
    qualityStatus: textValue(record, 'qualityStatus'),
    qualityWarnings: warningValues(record.qualityWarnings),
  }
}

function mallWeatherAlert(record: JsonRecord): MallWeatherAlert {
  return {
    alertId: textValue(record, 'alertId'),
    status: textValue(record, 'status'),
    title: textValue(record, 'title'),
    description: textValue(record, 'description'),
    alertTypeName: textValue(record, 'alertTypeName'),
    alertLevelName: textValue(record, 'alertLevelName'),
    publishedAtLocal: textValue(record, 'publishedAtLocal'),
    source: textValue(record, 'source'),
    qualityStatus: textValue(record, 'qualityStatus'),
    qualityWarnings: warningValues(record.qualityWarnings),
  }
}

export function parseMallWeatherMallList(payload: unknown): MallWeatherMallList | null {
  const data = envelopeData(payload)
  if (!data || !Array.isArray(data.items)) return null

  const items = data.items.flatMap((item) => {
    if (!isRecord(item) || !Number.isSafeInteger(item.id) || Number(item.id) <= 0 || typeof item.nameCn !== 'string') return []
    return [{
      id: Number(item.id),
      mallCode: typeof item.mallCode === 'string' ? item.mallCode : '',
      nameCn: item.nameCn,
      city: typeof item.city === 'string' ? item.city : '',
      address: typeof item.address === 'string' ? item.address : '',
      geocodeStatus: typeof item.geocodeStatus === 'string' ? item.geocodeStatus : '',
      weatherEnabled: item.weatherEnabled === true,
      status: typeof item.status === 'string' ? item.status : '',
    }]
  })
  const nextAfterId = data.nextAfterId === undefined ? 0 : numberValue(data, 'nextAfterId')
  if (nextAfterId === undefined || !Number.isSafeInteger(nextAfterId) || nextAfterId < 0) return null
  return { items, nextAfterId }
}

export function parseMallWeatherOverview(payload: unknown): MallWeatherOverview | null {
  const data = envelopeData(payload)
  if (!data || !isRecord(data.meta) || !Array.isArray(data.minutely) || !Array.isArray(data.hourly) || !Array.isArray(data.alerts)) return null

  return {
    realtime: isRecord(data.realtime) ? mallWeatherRealtime(data.realtime) : null,
    minutely: data.minutely.filter(isRecord).map(mallWeatherMinutely),
    hourly: data.hourly.filter(isRecord).map(mallWeatherHourly),
    alerts: data.alerts.filter(isRecord).map(mallWeatherAlert),
    meta: mallWeatherMeta(data.meta),
  }
}

export function mallWeatherOverviewPath(mallID: number, timeZone = 'Asia/Shanghai') {
  if (!Number.isSafeInteger(mallID) || mallID <= 0) throw new Error('invalid mall id')
  const query = new URLSearchParams({ timeZone })
  return `/v1/malls/${mallID}/weather/overview?${query.toString()}`
}

export function mallWeatherFreshnessLabel(status: string) {
  const labels: Record<string, string> = {
    FRESH: '数据新鲜',
    WARNING: '即将过期',
    STALE: '数据已过期',
    CRITICAL: '数据严重过期',
    UNAVAILABLE: '暂无数据',
  }
  return labels[status.toUpperCase()] ?? (status || '未知状态')
}

export function mallWeatherSkyconLabel(skycon: string | undefined) {
  if (!skycon) return '暂无天气现象'
  const labels: Record<string, string> = {
    CLEAR_DAY: '晴（白天）',
    CLEAR_NIGHT: '晴（夜间）',
    PARTLY_CLOUDY_DAY: '多云（白天）',
    PARTLY_CLOUDY_NIGHT: '多云（夜间）',
    CLOUDY: '阴',
    LIGHT_HAZE: '轻度雾霾',
    MODERATE_HAZE: '中度雾霾',
    HEAVY_HAZE: '重度雾霾',
    LIGHT_RAIN: '小雨',
    MODERATE_RAIN: '中雨',
    HEAVY_RAIN: '大雨',
    STORM_RAIN: '暴雨',
    FOG: '雾',
  }
  return labels[skycon] ?? skycon
}

export function mallWeatherMetric(value: number | undefined, unit: string, fractionDigits = 1) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '—'
  return `${value.toFixed(fractionDigits)}${unit}`
}

export function mallWeatherChartSegments(values: Array<number | undefined>, width: number, height: number) {
  const finiteValues = values.filter((value): value is number => typeof value === 'number' && Number.isFinite(value))
  if (finiteValues.length === 0 || width <= 0 || height <= 0) return []
  const minimum = Math.min(...finiteValues)
  const maximum = Math.max(...finiteValues)
  const range = maximum - minimum || 1
  const denominator = Math.max(values.length - 1, 1)

  const segments: string[] = []
  let current: string[] = []
  values.forEach((value, index) => {
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      if (current.length > 0) segments.push(current.join(' '))
      current = []
      return
    }
    const x = index / denominator * width
    const y = height - (value - minimum) / range * height
    current.push(`${x.toFixed(1)},${y.toFixed(1)}`)
  })
  if (current.length > 0) segments.push(current.join(' '))
  return segments
}
