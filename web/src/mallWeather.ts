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

export type MallWeatherRefreshKind = 'V26_FULL' | 'V3_LIFE_INDEX'

export type MallWeatherRefreshRequest = {
  kinds: MallWeatherRefreshKind[]
  force: false
  reason: string
}

export type MallWeatherPendingRefresh = {
  key: string
  body: MallWeatherRefreshRequest
}

export type MallWeatherRefreshResult = {
  jobId: number
  mallId: number
  force: boolean
  reason: string
  requestedBy: number
  kinds: Array<{
    kind: MallWeatherRefreshKind
    status: 'QUEUED' | 'SKIPPED_FRESH'
    outboxJobId?: number
  }>
}

export type MallWeatherRefreshDisposition =
  | { kind: 'accepted'; result: MallWeatherRefreshResult }
  | { kind: 'uncertain' }
  | { kind: 'rejected' }

type JsonRecord = Record<string, unknown>
type RefreshStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

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

  const items: MallWeatherMall[] = []
  for (const item of data.items) {
    if (!isRecord(item) || !Number.isSafeInteger(item.id) || Number(item.id) <= 0 || typeof item.nameCn !== 'string') return null
    items.push({
      id: Number(item.id),
      mallCode: typeof item.mallCode === 'string' ? item.mallCode : '',
      nameCn: item.nameCn,
      city: typeof item.city === 'string' ? item.city : '',
      address: typeof item.address === 'string' ? item.address : '',
      geocodeStatus: typeof item.geocodeStatus === 'string' ? item.geocodeStatus : '',
      weatherEnabled: item.weatherEnabled === true,
      status: typeof item.status === 'string' ? item.status : '',
    })
  }
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

export function parseMallWeatherRefreshResult(payload: unknown): MallWeatherRefreshResult | null {
  const data = envelopeData(payload)
  if (!data || !Number.isSafeInteger(data.jobId) || Number(data.jobId) <= 0 || !Number.isSafeInteger(data.mallId) || Number(data.mallId) <= 0 ||
    typeof data.force !== 'boolean' || typeof data.reason !== 'string' || !Number.isSafeInteger(data.requestedBy) || Number(data.requestedBy) <= 0 ||
    !Array.isArray(data.kinds) || data.kinds.length === 0) return null
  const kinds: MallWeatherRefreshResult['kinds'] = []
  const seenKinds = new Set<MallWeatherRefreshKind>()
  for (const item of data.kinds) {
    if (!isRecord(item) || (item.kind !== 'V26_FULL' && item.kind !== 'V3_LIFE_INDEX') ||
      (item.status !== 'QUEUED' && item.status !== 'SKIPPED_FRESH')) return null
    if (seenKinds.has(item.kind)) return null
    seenKinds.add(item.kind)
    const outboxJobId = numberValue(item, 'outboxJobId')
    if (item.status === 'QUEUED' && (outboxJobId === undefined || !Number.isSafeInteger(outboxJobId) || outboxJobId <= 0)) return null
    if (item.status === 'SKIPPED_FRESH' && outboxJobId !== undefined) return null
    kinds.push({ kind: item.kind, status: item.status, ...(outboxJobId === undefined ? {} : { outboxJobId }) })
  }
  return { jobId: Number(data.jobId), mallId: Number(data.mallId), force: data.force, reason: data.reason, requestedBy: Number(data.requestedBy), kinds }
}

export function mallWeatherRefreshDisposition(
  response: { ok: boolean; status: number; data: unknown },
  actorID: string,
  mallID: number,
  request: MallWeatherRefreshRequest,
): MallWeatherRefreshDisposition {
  if (!response.ok) {
    const uncertain = response.status === 0 || response.status === 408 || response.status === 409 || response.status >= 500 ||
      (response.status >= 200 && response.status < 300)
    return { kind: uncertain ? 'uncertain' : 'rejected' }
  }
  if (response.status !== 202) return { kind: 'uncertain' }
  const result = parseMallWeatherRefreshResult(response.data)
  if (!result || result.mallId !== mallID || String(result.requestedBy) !== actorID || result.force !== request.force ||
    result.reason !== request.reason || result.kinds.length !== request.kinds.length) return { kind: 'uncertain' }
  const returnedKinds = new Set(result.kinds.map((item) => item.kind))
  if (request.kinds.some((kind) => !returnedKinds.has(kind))) return { kind: 'uncertain' }
  return { kind: 'accepted', result }
}

export function mallWeatherRefreshResultMessage(result: MallWeatherRefreshResult) {
  const queued = result.kinds.filter((item) => item.status === 'QUEUED').length
  const skipped = result.kinds.filter((item) => item.status === 'SKIPPED_FRESH').length
  if (queued > 0 && skipped > 0) return `${queued} 个采集任务已入队，${skipped} 项数据仍新鲜并已跳过。`
  if (queued > 0) return `${queued} 个采集任务已进入异步队列；稍后重新加载天气即可查看结果。`
  return `${skipped} 项数据仍新鲜，本次未重复入队。`
}

export function mallWeatherOverviewPath(mallID: number, timeZone = 'Asia/Shanghai') {
  if (!Number.isSafeInteger(mallID) || mallID <= 0) throw new Error('invalid mall id')
  const query = new URLSearchParams({ timeZone })
  return `/v1/malls/${mallID}/weather/overview?${query.toString()}`
}

export function mallWeatherRefreshPath(mallID: number) {
  if (!Number.isSafeInteger(mallID) || mallID <= 0) throw new Error('invalid mall id')
  return `/v1/malls/${mallID}/weather-refresh`
}

export function mallWeatherRefreshRequest(kinds: MallWeatherRefreshKind[], reason: string): MallWeatherRefreshRequest {
  const normalizedReason = reason.trim()
  const normalizedKinds = Array.from(new Set(kinds)).sort() as MallWeatherRefreshKind[]
  if (normalizedKinds.length === 0 || normalizedKinds.some((kind) => kind !== 'V26_FULL' && kind !== 'V3_LIFE_INDEX')) {
    throw new Error('invalid refresh kinds')
  }
  if (!normalizedReason || Array.from(normalizedReason).length > 500 || /[\0\r\n]/.test(normalizedReason)) {
    throw new Error('invalid refresh reason')
  }
  return { kinds: normalizedKinds, force: false, reason: normalizedReason }
}

export function mallWeatherRefreshKey(seed?: string) {
  const value = seed || globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`
  const key = `weather-refresh:${value}`
  if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{7,254}$/.test(key)) throw new Error('invalid refresh key')
  return key
}

export function loadMallWeatherPendingRefresh(actorID: string, mallID: number, storage: RefreshStorage): MallWeatherPendingRefresh | null {
  const storageKey = mallWeatherPendingRefreshStorageKey(actorID, mallID)
  const raw = storage.getItem(storageKey)
  if (!raw) return null
  try {
    const snapshot: unknown = JSON.parse(raw)
    if (!isRecord(snapshot) || typeof snapshot.key !== 'string' || !isRecord(snapshot.body) || snapshot.body.force !== false ||
      !Array.isArray(snapshot.body.kinds) || !snapshot.body.kinds.every((kind) => kind === 'V26_FULL' || kind === 'V3_LIFE_INDEX') ||
      typeof snapshot.body.reason !== 'string' || !validMallWeatherRefreshKey(snapshot.key)) return null
    const body = mallWeatherRefreshRequest(snapshot.body.kinds as MallWeatherRefreshKind[], snapshot.body.reason)
    return { key: snapshot.key, body }
  } catch {
    return null
  }
}

export function saveMallWeatherPendingRefresh(actorID: string, mallID: number, pending: MallWeatherPendingRefresh, storage: RefreshStorage) {
  if (!validMallWeatherRefreshKey(pending.key)) throw new Error('invalid refresh key')
  const body = mallWeatherRefreshRequest(pending.body.kinds, pending.body.reason)
  storage.setItem(mallWeatherPendingRefreshStorageKey(actorID, mallID), JSON.stringify({ key: pending.key, body }))
}

export function clearMallWeatherPendingRefresh(actorID: string, mallID: number, storage: RefreshStorage) {
  storage.removeItem(mallWeatherPendingRefreshStorageKey(actorID, mallID))
}

function mallWeatherPendingRefreshStorageKey(actorID: string, mallID: number) {
  const numericActorID = Number(actorID)
  if (!/^[1-9]\d*$/.test(actorID) || !Number.isSafeInteger(numericActorID)) throw new Error('invalid actor id')
  if (!Number.isSafeInteger(mallID) || mallID <= 0) throw new Error('invalid mall id')
  return `mall-weather-pending-refresh:${actorID}:${mallID}`
}

function validMallWeatherRefreshKey(key: string) {
  return /^[A-Za-z0-9][A-Za-z0-9._:-]{7,254}$/.test(key)
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
