export type MallWeatherMall = {
  id: number
  mallCode: string
  nameCn: string
  province: string
  city: string
  district: string
  address: string
  longitude?: number
  latitude?: number
  coordinateSystem: string
  geocodeStatus: string
  weatherEnabled: boolean
  detailProfile: string
  coverageRadiusM: number
  status: string
  version: number
}

export type MallWeatherMallList = {
  items: MallWeatherMall[]
  nextAfterId: number
}

export type MallWeatherCreateInput = {
  mallCode: string
  nameCn: string
  province: string
  city: string
  district: string
  address: string
}

export type MallWeatherCreateRequest = {
  mallCode: string
  nameCn: string
  province: string
  city: string
  district?: string
  address: string
  weather: { detailProfile: 'full'; coverageRadiusM: 1000 }
}

export type MallWeatherCreateResult = {
  id: number
  mallCode: string
  status: string
  geocodeStatus: string
  weatherStatus: string
  version: number
}

export type MallWeatherPendingCreate = {
  key: string
  body: MallWeatherCreateRequest
}

export type MallWeatherGeocodeCandidate = {
  id: number
  candidateNo: number
  formattedAddress: string
  province: string
  city: string
  district: string
  longitude: number
  latitude: number
  coordinateSystem: string
  level: string
  confidenceScore: number
  selected: boolean
}

export type MallWeatherGeocodeCandidates = {
  mallId: number
  mallVersion: number
  runId: number
  runStatus: string
  items: MallWeatherGeocodeCandidate[]
}

export type MallWeatherGeocodeConfirmRequest = {
  candidateId?: number
  manualCoordinate?: {
    longitude: number
    latitude: number
    coordinateSystem: 'GCJ02'
    reason: string
  }
  expectedMallVersion: number
  weatherEnabled: true
}

export type MallWeatherSheetPushOption = {
  destinationId: number
  name: string
  code: string
  profileId: number
  profileCode: string
  profileVersion: number
}

export type MallWeatherSheetPushRequest = {
  destinationId: number
  profileId: number
  expectedProfileVersion: number
  filters: { mallIds: number[] }
}

export type MallWeatherSheetPushDryRun = {
  destinationId: number
  destinationCode: string
  profileId: number
  profileCode: string
  profileVersion: number
  writeMode: string
  totalEstimatedRows: number
  totalEstimatedCells: number
  canExecute: boolean
  warnings: string[]
  datasets: MallWeatherSheetPushDatasetDryRun[]
}

export type MallWeatherSheetPushDatasetDryRun = {
  datasetKind: string
  estimatedRows: number
  estimatedCells: number
  canExecute: boolean
  warnings: string[]
}

export type MallWeatherSheetPushResult = {
  runId: number
  traceId: string
  status: string
  destinationId: number
  profileId: number
  profileVersion: number
  estimatedRows: number
}

export type MallWeatherPendingSheetPush = {
  key: string
  body: MallWeatherSheetPushRequest
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
  forecastTimeUtc: string
  forecastTimeLocal: string
  issuedAtUtc: string
  issuedAtLocal: string
  fetchedAtUtc: string
  fetchedAtLocal: string
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

export type MallWeatherDaily = {
  forecastDateLocal: string
  issuedAtUtc: string
  issuedAtLocal: string
  fetchedAtUtc: string
  fetchedAtLocal: string
  temperatureMaxC?: number
  temperatureMinC?: number
  temperatureAvgC?: number
  precipitationProbabilityPct?: number
  windMaxSpeedKph?: number
  daySkycon: string
  nightSkycon: string
  sunriseLocalTime: string
  sunsetLocalTime: string
  qualityStatus: string
  qualityWarnings: MallWeatherWarning[]
}

export type MallWeatherLifeIndex = {
  sourceApi: string
  forecastDateLocal: string
  indexType: number
  indexCode: string
  indexName: string
  level?: number
  shortDescription: string
  detail: string
  isUnknownType: boolean
  issuedAtUtc: string
  issuedAtLocal: string
  fetchedAtUtc: string
  fetchedAtLocal: string
  qualityStatus: string
  qualityWarnings: MallWeatherWarning[]
}

export type MallWeatherPageResult<T> = {
  items: T[]
  meta: MallWeatherMeta
  pagination: { pageSize: number; nextCursor: string }
}

export type MallWeatherSeries = 'hourly' | 'daily' | 'life-indices'

export type MallWeatherQueryWindow = {
  start: Date
  end: Date
}

export type MallWeatherForecastWindows = {
  hourly: MallWeatherQueryWindow
  daily: MallWeatherQueryWindow
}

export type MallWeatherPageParser<T> = (payload: unknown) => MallWeatherPageResult<T> | null

export type MallWeatherPageRequester = (path: string) => Promise<{
  ok: boolean
  status: number
  data: unknown
}>

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
type OnboardingStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

function isRecord(value: unknown): value is JsonRecord {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function positiveSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

function validCoordinateValue(value: unknown, minimum: number, maximum: number): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= minimum && value <= maximum
}

function positiveMallID(mallID: number) {
  if (!Number.isSafeInteger(mallID) || mallID <= 0) throw new Error('invalid mall id')
  return mallID
}

function mallWeatherOperationKey(scope: string, seed?: string, invalidMessage = 'invalid operation key') {
  const value = seed || globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`
  const key = `${scope}:${value}`
  if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{7,254}$/.test(key)) throw new Error(invalidMessage)
  return key
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

function mallWeatherMall(value: unknown): MallWeatherMall | null {
  if (!isRecord(value) || !positiveSafeInteger(value.id) || !positiveSafeInteger(value.version) ||
    typeof value.mallCode !== 'string' || typeof value.nameCn !== 'string' || typeof value.province !== 'string' ||
    typeof value.city !== 'string' || typeof value.address !== 'string' || typeof value.geocodeStatus !== 'string' ||
    typeof value.weatherEnabled !== 'boolean' || typeof value.weatherProvider !== 'string' ||
    typeof value.detailProfile !== 'string' || typeof value.status !== 'string' ||
    !value.mallCode.trim() || !value.nameCn.trim() || !value.province.trim() || !value.city.trim() || !value.address.trim() ||
    !value.geocodeStatus.trim() || !value.weatherProvider.trim() || !['full', 'standard', 'economy'].includes(value.detailProfile) || !value.status.trim()) return null
  const hasLongitude = value.longitude !== undefined
  const hasLatitude = value.latitude !== undefined
  if (hasLongitude !== hasLatitude || hasLongitude && (!validCoordinateValue(value.longitude, -180, 180) ||
    !validCoordinateValue(value.latitude, -90, 90) || typeof value.coordinateSystem !== 'string' ||
    value.coordinateSystem.trim().toUpperCase() !== 'GCJ02')) return null
  const coverageRadiusM = numberValue(value, 'coverageRadiusM')
  if (coverageRadiusM === undefined || !Number.isSafeInteger(coverageRadiusM) || coverageRadiusM < 100 || coverageRadiusM > 10000) return null
  return {
    id: value.id,
    mallCode: value.mallCode,
    nameCn: value.nameCn,
    province: textValue(value, 'province'),
    city: textValue(value, 'city'),
    district: textValue(value, 'district'),
    address: textValue(value, 'address'),
    ...(typeof value.longitude === 'number' ? { longitude: value.longitude } : {}),
    ...(typeof value.latitude === 'number' ? { latitude: value.latitude } : {}),
    coordinateSystem: hasLongitude ? 'GCJ02' : '',
    geocodeStatus: textValue(value, 'geocodeStatus'),
    weatherEnabled: value.weatherEnabled === true,
    detailProfile: textValue(value, 'detailProfile'),
    coverageRadiusM,
    status: textValue(value, 'status'),
    version: value.version,
  }
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

function strictMallWeatherMeta(value: unknown): MallWeatherMeta | null {
  if (!isRecord(value)) return null
  const requiredText = ['provider', 'apiVersion', 'representativePoint', 'coordinateSystem', 'samplingMode', 'spatialResolution', 'timeZone', 'unit', 'freshnessStatus']
  if (requiredText.some((key) => typeof value[key] !== 'string' || !String(value[key]).trim())) return null
  const longitude = numberValue(value, 'longitude')
  const latitude = numberValue(value, 'latitude')
  const coverageRadiusM = numberValue(value, 'coverageRadiusM')
  const dataAgeSeconds = numberValue(value, 'dataAgeSeconds')
  if (longitude === undefined || longitude < -180 || longitude > 180 || latitude === undefined || latitude < -90 || latitude > 90 ||
    coverageRadiusM === undefined || !Number.isSafeInteger(coverageRadiusM) || coverageRadiusM < 0 ||
    (value.dataAgeSeconds !== undefined && (dataAgeSeconds === undefined || !Number.isSafeInteger(dataAgeSeconds) || dataAgeSeconds < 0))) return null
  return mallWeatherMeta(value)
}

function strictWarningValues(value: unknown): MallWeatherWarning[] | null {
  if (!Array.isArray(value)) return null
  const warnings: MallWeatherWarning[] = []
  for (const warning of value) {
    if (!isRecord(warning) || typeof warning.code !== 'string' || !warning.code.trim() || typeof warning.path !== 'string') return null
    warnings.push({ code: warning.code, path: warning.path })
  }
  return warnings
}

function isRFC3339(value: unknown): value is string {
  return typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value) && Number.isFinite(Date.parse(value))
}

function isISODate(value: unknown): value is string {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(`${value}T00:00:00.000Z`)
  return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value
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

function mallWeatherHourly(record: JsonRecord, qualityWarnings = warningValues(record.qualityWarnings)): MallWeatherHourly {
  return {
    forecastTimeUtc: textValue(record, 'forecastTimeUtc'),
    forecastTimeLocal: textValue(record, 'forecastTimeLocal'),
    issuedAtUtc: textValue(record, 'issuedAtUtc'),
    issuedAtLocal: textValue(record, 'issuedAtLocal'),
    fetchedAtUtc: textValue(record, 'fetchedAtUtc'),
    fetchedAtLocal: textValue(record, 'fetchedAtLocal'),
    temperatureC: numberValue(record, 'temperatureC'),
    precipitationMmH: numberValue(record, 'precipitationMmH'),
    precipitationProbabilityPct: numberValue(record, 'precipitationProbabilityPct'),
    windSpeedKph: numberValue(record, 'windSpeedKph'),
    skycon: textValue(record, 'skycon'),
    pm25UgM3: numberValue(record, 'pm25UgM3'),
    aqiChn: numberValue(record, 'aqiChn'),
    qualityStatus: textValue(record, 'qualityStatus'),
    qualityWarnings,
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
    const mall = mallWeatherMall(item)
    if (!mall) return null
    items.push(mall)
  }
  const nextAfterId = data.nextAfterId === undefined ? 0 : numberValue(data, 'nextAfterId')
  if (nextAfterId === undefined || !Number.isSafeInteger(nextAfterId) || nextAfterId < 0) return null
  return { items, nextAfterId }
}

export function parseMallWeatherMall(payload: unknown): MallWeatherMall | null {
  const data = envelopeData(payload)
  return data ? mallWeatherMall(data) : null
}

export function parseMallWeatherCreateResult(payload: unknown): MallWeatherCreateResult | null {
  const data = envelopeData(payload)
  if (!data || !positiveSafeInteger(data.id) || !positiveSafeInteger(data.version) || typeof data.mallCode !== 'string') return null
  return {
    id: Number(data.id),
    mallCode: data.mallCode,
    status: textValue(data, 'status'),
    geocodeStatus: textValue(data, 'geocodeStatus'),
    weatherStatus: textValue(data, 'weatherStatus'),
    version: Number(data.version),
  }
}

export function parseMallWeatherGeocodeCandidates(payload: unknown): MallWeatherGeocodeCandidates | null {
  const data = envelopeData(payload)
  if (!data || !positiveSafeInteger(data.mallId) || !positiveSafeInteger(data.mallVersion) || !Array.isArray(data.items)) return null
  const items: MallWeatherGeocodeCandidate[] = []
  for (const item of data.items) {
    if (!isRecord(item) || !positiveSafeInteger(item.id) || !Number.isSafeInteger(item.candidateNo) || Number(item.candidateNo) < 1 ||
      typeof item.formattedAddress !== 'string' || !validCoordinateValue(item.longitude, -180, 180) ||
      !validCoordinateValue(item.latitude, -90, 90) || typeof item.coordinateSystem !== 'string' ||
      item.coordinateSystem.trim().toUpperCase() !== 'GCJ02' || typeof item.confidenceScore !== 'number' ||
      !Number.isFinite(item.confidenceScore) || item.confidenceScore < 0 || item.confidenceScore > 100) return null
    items.push({
      id: Number(item.id),
      candidateNo: Number(item.candidateNo),
      formattedAddress: item.formattedAddress,
      province: textValue(item, 'province'),
      city: textValue(item, 'city'),
      district: textValue(item, 'district'),
      longitude: Number(item.longitude),
      latitude: Number(item.latitude),
      coordinateSystem: 'GCJ02',
      level: textValue(item, 'level'),
      confidenceScore: item.confidenceScore,
      selected: item.selected === true,
    })
  }
  const runId = data.runId === undefined ? 0 : numberValue(data, 'runId')
  if (runId === undefined || !Number.isSafeInteger(runId) || runId < 0) return null
  return {
    mallId: Number(data.mallId),
    mallVersion: Number(data.mallVersion),
    runId,
    runStatus: textValue(data, 'runStatus'),
    items,
  }
}

export function mallWeatherCreateRequest(input: MallWeatherCreateInput): MallWeatherCreateRequest {
  const mallCode = input.mallCode.trim().toUpperCase()
  const nameCn = input.nameCn.trim()
  const province = input.province.trim()
  const city = input.city.trim()
  const district = input.district.trim()
  const address = input.address.trim()
  if (!/^[A-Z0-9][A-Z0-9_-]{1,63}$/.test(mallCode) || !nameCn || !province || !city || !address ||
    nameCn.length > 255 || province.length > 128 || city.length > 128 || district.length > 128 || address.length > 1000) {
    throw new Error('invalid mall create request')
  }
  return {
    mallCode,
    nameCn,
    province,
    city,
    ...(district ? { district } : {}),
    address,
    weather: { detailProfile: 'full', coverageRadiusM: 1000 },
  }
}

export function mallWeatherCreateKey(seed?: string) {
  return mallWeatherOperationKey('mall-create', seed)
}

export function mallWeatherMallReady(mall: MallWeatherMall) {
  return mall.status.toLowerCase() === 'active' && mall.geocodeStatus.toLowerCase() === 'confirmed' && mall.weatherEnabled &&
    mall.longitude !== undefined && mall.latitude !== undefined && mall.coordinateSystem.trim().toUpperCase() === 'GCJ02'
}

export function mergeMallWeatherMalls(current: MallWeatherMall[], incoming: MallWeatherMall[]) {
  const byID = new Map(current.map((mall) => [mall.id, mall]))
  for (const mall of incoming) {
    const existing = byID.get(mall.id)
    if (!existing || mall.version >= existing.version) byID.set(mall.id, mall)
  }
  return Array.from(byID.values())
}

export function loadMallWeatherPendingCreate(actorID: string, storage: OnboardingStorage): MallWeatherPendingCreate | null {
  const storageKey = mallWeatherPendingCreateStorageKey(actorID)
  const raw = storage.getItem(storageKey)
  if (!raw) return null
  try {
    const snapshot: unknown = JSON.parse(raw)
    if (!isRecord(snapshot) || typeof snapshot.key !== 'string' || !isRecord(snapshot.body) || !validMallWeatherCreateKey(snapshot.key)) return null
    const body = mallWeatherCreateRequest({
      mallCode: textValue(snapshot.body, 'mallCode'),
      nameCn: textValue(snapshot.body, 'nameCn'),
      province: textValue(snapshot.body, 'province'),
      city: textValue(snapshot.body, 'city'),
      district: textValue(snapshot.body, 'district'),
      address: textValue(snapshot.body, 'address'),
    })
    return { key: snapshot.key, body }
  } catch {
    return null
  }
}

export function saveMallWeatherPendingCreate(actorID: string, pending: MallWeatherPendingCreate, storage: OnboardingStorage) {
  if (!validMallWeatherCreateKey(pending.key)) throw new Error('invalid create key')
  const body = mallWeatherCreateRequest({
    mallCode: pending.body.mallCode,
    nameCn: pending.body.nameCn,
    province: pending.body.province,
    city: pending.body.city,
    district: pending.body.district || '',
    address: pending.body.address,
  })
  storage.setItem(mallWeatherPendingCreateStorageKey(actorID), JSON.stringify({ key: pending.key, body }))
}

export function clearMallWeatherPendingCreate(actorID: string, storage: OnboardingStorage) {
  storage.removeItem(mallWeatherPendingCreateStorageKey(actorID))
}

export function mallWeatherGeocodeCandidatesPath(mallID: number) {
  return `/v1/malls/${positiveMallID(mallID)}/geocode-candidates`
}

export function mallWeatherGeocodeTriggerPath(mallID: number) {
  return `/v1/malls/${positiveMallID(mallID)}/geocode`
}

export function mallWeatherGeocodeConfirmPath(mallID: number) {
  return `/v1/malls/${positiveMallID(mallID)}/geocode-confirm`
}

export function mallWeatherGeocodeRunTerminal(status: string) {
  return ['SUCCEEDED', 'FAILED', 'STALE', 'NO_CANDIDATES', 'AUTO_CONFIRMED', 'REVIEW_REQUIRED'].includes(status.toUpperCase())
}

export function mallWeatherCandidateConfirmationRequest(
  candidate: MallWeatherGeocodeCandidate,
  longitudeInput: string,
  latitudeInput: string,
  reasonInput: string,
  expectedMallVersion: number,
): MallWeatherGeocodeConfirmRequest {
  if (!positiveSafeInteger(candidate.id) || candidate.coordinateSystem.trim().toUpperCase() !== 'GCJ02' ||
    !positiveSafeInteger(expectedMallVersion)) throw new Error('invalid geocode candidate')
  const coordinate = mallWeatherCoordinateInput(longitudeInput, latitudeInput, reasonInput)
  if (coordinate.longitude === candidate.longitude && coordinate.latitude === candidate.latitude) {
    return { candidateId: candidate.id, expectedMallVersion, weatherEnabled: true }
  }
  return { manualCoordinate: coordinate, expectedMallVersion, weatherEnabled: true }
}

export function mallWeatherCoordinateAdjustmentRequest(
  mall: MallWeatherMall,
  longitudeInput: string,
  latitudeInput: string,
  reasonInput: string,
): MallWeatherGeocodeConfirmRequest {
  if (!positiveSafeInteger(mall.version) || mall.longitude === undefined || mall.latitude === undefined ||
    mall.coordinateSystem.trim().toUpperCase() !== 'GCJ02') throw new Error('invalid mall coordinate')
  return {
    manualCoordinate: mallWeatherCoordinateInput(longitudeInput, latitudeInput, reasonInput),
    expectedMallVersion: mall.version,
    weatherEnabled: true,
  }
}

function mallWeatherCoordinateInput(longitudeInput: string, latitudeInput: string, reasonInput: string) {
  const longitude = Number(longitudeInput.trim())
  const latitude = Number(latitudeInput.trim())
  const reason = reasonInput.trim()
  if (!longitudeInput.trim() || !latitudeInput.trim() || !validCoordinateValue(longitude, -180, 180) ||
    !validCoordinateValue(latitude, -90, 90) || !reason || reason.length > 500 || reason.includes('\n') || reason.includes('\r')) {
    throw new Error('invalid coordinate adjustment')
  }
  return { longitude, latitude, coordinateSystem: 'GCJ02' as const, reason }
}

export function parseMallWeatherSheetPushOptions(payload: unknown): MallWeatherSheetPushOption[] | null {
  const data = envelopeData(payload)
  if (!data || !Array.isArray(data.items)) return null
  const items: MallWeatherSheetPushOption[] = []
  for (const item of data.items) {
    if (!isRecord(item) || !positiveSafeInteger(item.destinationId) || !positiveSafeInteger(item.profileId) ||
      !positiveSafeInteger(item.profileVersion) || typeof item.name !== 'string' || !item.name.trim() ||
      typeof item.code !== 'string' || !item.code.trim() || typeof item.profileCode !== 'string' || !item.profileCode.trim()) return null
    items.push({
      destinationId: item.destinationId,
      name: item.name,
      code: item.code,
      profileId: item.profileId,
      profileCode: item.profileCode,
      profileVersion: item.profileVersion,
    })
  }
  return items
}

export function mallWeatherSheetPushRequest(option: MallWeatherSheetPushOption, mallID: number): MallWeatherSheetPushRequest {
  if (!positiveSafeInteger(option.destinationId) || !positiveSafeInteger(option.profileId) || !positiveSafeInteger(option.profileVersion)) {
    throw new Error('invalid weather sheet push option')
  }
  return {
    destinationId: option.destinationId,
    profileId: option.profileId,
    expectedProfileVersion: option.profileVersion,
    filters: { mallIds: [positiveMallID(mallID)] },
  }
}

export function mallWeatherSheetPushRequestMatchesOption(request: MallWeatherSheetPushRequest, option: MallWeatherSheetPushOption, mallID: number) {
  return request.destinationId === option.destinationId && request.profileId === option.profileId &&
    request.expectedProfileVersion === option.profileVersion && request.filters.mallIds.length === 1 &&
    request.filters.mallIds[0] === mallID
}

export function mallWeatherSheetPushResultMatchesRequest(result: MallWeatherSheetPushResult, request: MallWeatherSheetPushRequest) {
  return result.destinationId === request.destinationId && result.profileId === request.profileId &&
    result.profileVersion === request.expectedProfileVersion
}

export function parseMallWeatherSheetPushDryRun(payload: unknown): MallWeatherSheetPushDryRun | null {
  const data = envelopeData(payload)
  if (!data || !positiveSafeInteger(data.destinationId) || !positiveSafeInteger(data.profileId) || !positiveSafeInteger(data.profileVersion) ||
    typeof data.destinationCode !== 'string' || typeof data.profileCode !== 'string' || typeof data.writeMode !== 'string' ||
    !Number.isSafeInteger(data.totalEstimatedRows) || Number(data.totalEstimatedRows) < 0 ||
    !Number.isSafeInteger(data.totalEstimatedCells) || Number(data.totalEstimatedCells) < 0 || typeof data.canExecute !== 'boolean' ||
    !Array.isArray(data.warnings) || !data.warnings.every((warning) => typeof warning === 'string') || !Array.isArray(data.datasets)) return null
  const datasets: MallWeatherSheetPushDatasetDryRun[] = []
  for (const dataset of data.datasets) {
    if (!isRecord(dataset) || typeof dataset.datasetKind !== 'string' || !dataset.datasetKind.trim() ||
      !Number.isSafeInteger(dataset.estimatedRows) || Number(dataset.estimatedRows) < 0 ||
      !Number.isSafeInteger(dataset.estimatedCells) || Number(dataset.estimatedCells) < 0 || typeof dataset.canExecute !== 'boolean' ||
      !Array.isArray(dataset.warnings) || !dataset.warnings.every((warning) => typeof warning === 'string')) return null
    datasets.push({
      datasetKind: dataset.datasetKind,
      estimatedRows: Number(dataset.estimatedRows),
      estimatedCells: Number(dataset.estimatedCells),
      canExecute: dataset.canExecute,
      warnings: dataset.warnings.map((warning) => String(warning)),
    })
  }
  return {
    destinationId: data.destinationId,
    destinationCode: data.destinationCode,
    profileId: data.profileId,
    profileCode: data.profileCode,
    profileVersion: data.profileVersion,
    writeMode: data.writeMode,
    totalEstimatedRows: Number(data.totalEstimatedRows),
    totalEstimatedCells: Number(data.totalEstimatedCells),
    canExecute: data.canExecute,
    warnings: data.warnings.map((warning) => String(warning)),
    datasets,
  }
}

export function parseMallWeatherSheetPushResult(payload: unknown): MallWeatherSheetPushResult | null {
  const data = envelopeData(payload)
  if (!data || !positiveSafeInteger(data.runId) || !positiveSafeInteger(data.destinationId) || !positiveSafeInteger(data.profileId) ||
    !positiveSafeInteger(data.profileVersion) || typeof data.traceId !== 'string' || !data.traceId.trim() || typeof data.status !== 'string' ||
    !Number.isSafeInteger(data.estimatedRows) || Number(data.estimatedRows) < 0) return null
  return {
    runId: data.runId,
    traceId: data.traceId,
    status: data.status,
    destinationId: data.destinationId,
    profileId: data.profileId,
    profileVersion: data.profileVersion,
    estimatedRows: Number(data.estimatedRows),
  }
}

export function mallWeatherSheetPushKey(seed?: string) {
  return mallWeatherOperationKey('weather-sheet-push', seed)
}

export function loadMallWeatherPendingSheetPush(actorID: string, mallID: number, storage: OnboardingStorage): MallWeatherPendingSheetPush | null {
  const storageKey = mallWeatherPendingSheetPushStorageKey(actorID, mallID)
  const raw = storage.getItem(storageKey)
  if (!raw) return null
  try {
    const snapshot: unknown = JSON.parse(raw)
    if (!isRecord(snapshot) || typeof snapshot.key !== 'string' || !validMallWeatherSheetPushKey(snapshot.key) || !isRecord(snapshot.body) ||
      !positiveSafeInteger(snapshot.body.destinationId) || !positiveSafeInteger(snapshot.body.profileId) ||
      !positiveSafeInteger(snapshot.body.expectedProfileVersion) || !isRecord(snapshot.body.filters) ||
      !Array.isArray(snapshot.body.filters.mallIds) || snapshot.body.filters.mallIds.length !== 1 || snapshot.body.filters.mallIds[0] !== mallID) return null
    return {
      key: snapshot.key,
      body: {
        destinationId: snapshot.body.destinationId,
        profileId: snapshot.body.profileId,
        expectedProfileVersion: snapshot.body.expectedProfileVersion,
        filters: { mallIds: [mallID] },
      },
    }
  } catch {
    return null
  }
}

export function saveMallWeatherPendingSheetPush(actorID: string, mallID: number, pending: MallWeatherPendingSheetPush, storage: OnboardingStorage) {
  if (!validMallWeatherSheetPushKey(pending.key) || !positiveSafeInteger(pending.body.destinationId) ||
    !positiveSafeInteger(pending.body.profileId) || !positiveSafeInteger(pending.body.expectedProfileVersion) ||
    pending.body.filters.mallIds.length !== 1 || pending.body.filters.mallIds[0] !== mallID) {
    throw new Error('invalid weather sheet push')
  }
  storage.setItem(mallWeatherPendingSheetPushStorageKey(actorID, mallID), JSON.stringify(pending))
}

export function clearMallWeatherPendingSheetPush(actorID: string, mallID: number, storage: OnboardingStorage) {
  storage.removeItem(mallWeatherPendingSheetPushStorageKey(actorID, mallID))
}

export function parseMallWeatherOverview(payload: unknown): MallWeatherOverview | null {
  const data = envelopeData(payload)
  if (!data || !isRecord(data.meta) || !Array.isArray(data.minutely) || !Array.isArray(data.hourly) || !Array.isArray(data.alerts)) return null

  return {
    realtime: isRecord(data.realtime) ? mallWeatherRealtime(data.realtime) : null,
    minutely: data.minutely.filter(isRecord).map(mallWeatherMinutely),
    hourly: data.hourly.filter(isRecord).map((item) => mallWeatherHourly(item)),
    alerts: data.alerts.filter(isRecord).map(mallWeatherAlert),
    meta: mallWeatherMeta(data.meta),
  }
}

function mallWeatherPagination(value: unknown): MallWeatherPageResult<never>['pagination'] | null {
  if (!isRecord(value) || !Number.isSafeInteger(value.pageSize) || Number(value.pageSize) < 1 || Number(value.pageSize) > 200 ||
    (value.nextCursor !== undefined && typeof value.nextCursor !== 'string')) return null
  return { pageSize: Number(value.pageSize), nextCursor: typeof value.nextCursor === 'string' ? value.nextCursor : '' }
}

function mallWeatherPageData(payload: unknown): { data: JsonRecord; pagination: MallWeatherPageResult<never>['pagination'] } | null {
  const data = envelopeData(payload)
  if (!data || !Array.isArray(data.items) || !isRecord(data.meta)) return null
  const pagination = mallWeatherPagination(data.pagination)
  return pagination ? { data, pagination } : null
}

export function parseMallWeatherHourlyPage(payload: unknown): MallWeatherPageResult<MallWeatherHourly> | null {
  const page = mallWeatherPageData(payload)
  if (!page) return null
  const meta = strictMallWeatherMeta(page.data.meta)
  if (!meta) return null
  const items: MallWeatherHourly[] = []
  for (const item of page.data.items as unknown[]) {
    if (!isRecord(item) || !isRFC3339(item.forecastTimeUtc) || !isRFC3339(item.forecastTimeLocal) || !isRFC3339(item.issuedAtUtc) ||
      !isRFC3339(item.issuedAtLocal) || !isRFC3339(item.fetchedAtUtc) || !isRFC3339(item.fetchedAtLocal) ||
      typeof item.qualityStatus !== 'string' || !item.qualityStatus.trim()) return null
    const warnings = strictWarningValues(item.qualityWarnings)
    if (!warnings) return null
    items.push(mallWeatherHourly(item, warnings))
  }
  return { items, meta, pagination: page.pagination }
}

export function parseMallWeatherDailyPage(payload: unknown): MallWeatherPageResult<MallWeatherDaily> | null {
  const page = mallWeatherPageData(payload)
  if (!page) return null
  const meta = strictMallWeatherMeta(page.data.meta)
  if (!meta) return null
  const items: MallWeatherDaily[] = []
  for (const item of page.data.items as unknown[]) {
    if (!isRecord(item) || !isISODate(item.forecastDateLocal) || !isRFC3339(item.issuedAtUtc) || !isRFC3339(item.issuedAtLocal) ||
      !isRFC3339(item.fetchedAtUtc) || !isRFC3339(item.fetchedAtLocal) || typeof item.qualityStatus !== 'string' || !item.qualityStatus.trim()) return null
    const warnings = strictWarningValues(item.qualityWarnings)
    if (!warnings) return null
    items.push({
      forecastDateLocal: item.forecastDateLocal,
      issuedAtUtc: item.issuedAtUtc,
      issuedAtLocal: item.issuedAtLocal,
      fetchedAtUtc: item.fetchedAtUtc,
      fetchedAtLocal: item.fetchedAtLocal,
      temperatureMaxC: numberValue(item, 'temperatureMaxC'),
      temperatureMinC: numberValue(item, 'temperatureMinC'),
      temperatureAvgC: numberValue(item, 'temperatureAvgC'),
      precipitationProbabilityPct: numberValue(item, 'precipitationProbabilityPct'),
      windMaxSpeedKph: numberValue(item, 'windMaxSpeedKph'),
      daySkycon: textValue(item, 'daySkycon'),
      nightSkycon: textValue(item, 'nightSkycon'),
      sunriseLocalTime: textValue(item, 'sunriseLocalTime'),
      sunsetLocalTime: textValue(item, 'sunsetLocalTime'),
      qualityStatus: item.qualityStatus,
      qualityWarnings: warnings,
    })
  }
  return { items, meta, pagination: page.pagination }
}

export function parseMallWeatherLifeIndexPage(payload: unknown): MallWeatherPageResult<MallWeatherLifeIndex> | null {
  const page = mallWeatherPageData(payload)
  if (!page) return null
  const meta = strictMallWeatherMeta(page.data.meta)
  if (!meta) return null
  const items: MallWeatherLifeIndex[] = []
  for (const item of page.data.items as unknown[]) {
    if (!isRecord(item) || typeof item.sourceApi !== 'string' || !item.sourceApi.trim() || !isISODate(item.forecastDateLocal) ||
      !Number.isSafeInteger(item.indexType) || Number(item.indexType) < 0 || typeof item.indexCode !== 'string' || !item.indexCode.trim() ||
      !isRFC3339(item.issuedAtUtc) || !isRFC3339(item.issuedAtLocal) || !isRFC3339(item.fetchedAtUtc) || !isRFC3339(item.fetchedAtLocal) ||
      typeof item.qualityStatus !== 'string' || !item.qualityStatus.trim() || typeof item.isUnknownType !== 'boolean') return null
    const level = numberValue(item, 'level')
    if (level !== undefined && !Number.isSafeInteger(level)) return null
    const warnings = strictWarningValues(item.qualityWarnings)
    if (!warnings) return null
    items.push({
      sourceApi: item.sourceApi,
      forecastDateLocal: item.forecastDateLocal,
      indexType: Number(item.indexType),
      indexCode: item.indexCode,
      indexName: textValue(item, 'indexName'),
      level,
      shortDescription: textValue(item, 'shortDescription'),
      detail: textValue(item, 'detail'),
      isUnknownType: item.isUnknownType,
      issuedAtUtc: item.issuedAtUtc,
      issuedAtLocal: item.issuedAtLocal,
      fetchedAtUtc: item.fetchedAtUtc,
      fetchedAtLocal: item.fetchedAtLocal,
      qualityStatus: item.qualityStatus,
      qualityWarnings: warnings,
    })
  }
  return { items, meta, pagination: page.pagination }
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

export function mallWeatherOverviewPath(mallID: number, timeZone = '') {
  if (!Number.isSafeInteger(mallID) || mallID <= 0) throw new Error('invalid mall id')
  const query = new URLSearchParams()
  if (timeZone.trim()) query.set('timeZone', timeZone)
  const suffix = query.toString()
  return `/v1/malls/${mallID}/weather/overview${suffix ? `?${suffix}` : ''}`
}

export function mallWeatherSeriesPath(mallID: number, series: MallWeatherSeries, start: Date, end: Date, cursor = '', timeZone = 'Asia/Shanghai', asOf = new Date()) {
  if (!Number.isSafeInteger(mallID) || mallID <= 0) throw new Error('invalid mall id')
  if (!['hourly', 'daily', 'life-indices'].includes(series)) throw new Error('invalid weather series')
  if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime()) || start >= end || end.getTime() - start.getTime() > 31 * 24 * 60 * 60 * 1000) {
    throw new Error('invalid weather range')
  }
  if (!Number.isFinite(asOf.getTime())) throw new Error('invalid weather snapshot time')
  if (!timeZone.trim()) throw new Error('invalid weather time zone')
  const query = new URLSearchParams({
    start: start.toISOString(),
    end: end.toISOString(),
    timeZone,
    latest: 'true',
    asOf: asOf.toISOString(),
    pageSize: '200',
  })
  if (cursor) query.set('cursor', cursor)
  return `/v1/malls/${mallID}/weather/${series}?${query.toString()}`
}

export function mallWeatherForecastQueryWindows(now = new Date(), timeZone = 'Asia/Shanghai'): MallWeatherForecastWindows {
  if (!Number.isFinite(now.getTime())) throw new Error('invalid weather query time')
  const hourMilliseconds = 60 * 60 * 1000
  const hourlyStart = new Date(Math.floor(now.getTime() / hourMilliseconds) * hourMilliseconds)
  const localDate = datePartsInTimeZone(now, timeZone)
  const dailyStart = localMidnight(localDate.year, localDate.month, localDate.day, timeZone)
  const normalizedEndDate = new Date(Date.UTC(localDate.year, localDate.month - 1, localDate.day + 15))
  const dailyEnd = localMidnight(normalizedEndDate.getUTCFullYear(), normalizedEndDate.getUTCMonth() + 1, normalizedEndDate.getUTCDate(), timeZone)
  return {
    hourly: { start: hourlyStart, end: new Date(hourlyStart.getTime() + 360 * hourMilliseconds) },
    daily: { start: dailyStart, end: dailyEnd },
  }
}

export async function loadAllMallWeatherPages<T>(
  request: MallWeatherPageRequester,
  mallID: number,
  series: MallWeatherSeries,
  window: MallWeatherQueryWindow,
  timeZone: string,
  asOf: Date,
  parser: MallWeatherPageParser<T>,
): Promise<{ items: T[]; meta: MallWeatherMeta | null }> {
  const items: T[] = []
  let cursor = ''
  let meta: MallWeatherMeta | null = null
  const seenCursors = new Set<string>()
  const seenLogicalKeys = new Set<string>()
  for (let pageNumber = 0; pageNumber < 10; pageNumber++) {
    const response = await request(mallWeatherSeriesPath(mallID, series, window.start, window.end, cursor, timeZone, asOf))
    if (!response.ok) throw new Error(mallWeatherQueryError(response.status))
    const page = parser(response.data)
    if (!page) throw new Error('响应格式不正确，请联系管理员')
    for (const item of page.items) {
      const logicalKey = mallWeatherLogicalKey(series, item)
      if (!logicalKey || seenLogicalKeys.has(logicalKey)) throw new Error('分页数据重复或缺少业务键，请联系管理员')
      seenLogicalKeys.add(logicalKey)
    }
    items.push(...page.items)
    meta = page.meta
    const nextCursor = page.pagination.nextCursor
    if (!nextCursor) return { items, meta }
    if (seenCursors.has(nextCursor)) throw new Error('分页游标重复，请联系管理员')
    seenCursors.add(nextCursor)
    cursor = nextCursor
  }
  throw new Error('分页数量超过安全上限，请联系管理员')
}

function mallWeatherLogicalKey(series: MallWeatherSeries, value: unknown) {
  if (!isRecord(value)) return ''
  if (series === 'hourly') return typeof value.forecastTimeUtc === 'string' ? value.forecastTimeUtc : ''
  if (series === 'daily') return typeof value.forecastDateLocal === 'string' ? value.forecastDateLocal : ''
  return typeof value.forecastDateLocal === 'string' && typeof value.sourceApi === 'string' && Number.isSafeInteger(value.indexType)
    ? `${value.forecastDateLocal}\u0000${value.sourceApi}\u0000${value.indexType}`
    : ''
}

function mallWeatherQueryError(status: number) {
  if (status === 0) return '无法连接服务，请检查网络后重试'
  if (status === 403) return '当前账号缺少 weather.read 权限'
  if (status === 404) return '商场或天气数据不存在'
  if (status === 422) return '查询窗口、时区或商场坐标无效'
  return `完整天气查询失败（HTTP ${status}）`
}

function datePartsInTimeZone(value: Date, timeZone: string) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(value)
  const part = (type: Intl.DateTimeFormatPartTypes) => Number(parts.find((item) => item.type === type)?.value)
  const year = part('year')
  const month = part('month')
  const day = part('day')
  if (!Number.isSafeInteger(year) || !Number.isSafeInteger(month) || !Number.isSafeInteger(day)) throw new Error('invalid weather time zone')
  return { year, month, day }
}

function localMidnight(year: number, month: number, day: number, timeZone: string) {
  const targetTimestamp = Date.UTC(year, month - 1, day)
  let candidateTimestamp = targetTimestamp
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  })
  for (let attempt = 0; attempt < 3; attempt++) {
    const parts = formatter.formatToParts(new Date(candidateTimestamp))
    const part = (type: Intl.DateTimeFormatPartTypes) => Number(parts.find((item) => item.type === type)?.value)
    const representedTimestamp = Date.UTC(part('year'), part('month') - 1, part('day'), part('hour'), part('minute'), part('second'))
    const nextTimestamp = targetTimestamp - (representedTimestamp - candidateTimestamp)
    if (nextTimestamp === candidateTimestamp) return new Date(candidateTimestamp)
    candidateTimestamp = nextTimestamp
  }
  return new Date(candidateTimestamp)
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
  return mallWeatherOperationKey('weather-refresh', seed, 'invalid refresh key')
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

function mallWeatherPendingCreateStorageKey(actorID: string) {
  const numericActorID = Number(actorID)
  if (!/^[1-9]\d*$/.test(actorID) || !Number.isSafeInteger(numericActorID)) throw new Error('invalid actor id')
  return `mall-weather-pending-create:${actorID}`
}

function validMallWeatherCreateKey(key: string) {
  return key.startsWith('mall-create:') && /^[A-Za-z0-9][A-Za-z0-9._:-]{7,254}$/.test(key)
}

function mallWeatherPendingSheetPushStorageKey(actorID: string, mallID: number) {
  const numericActorID = Number(actorID)
  if (!/^[1-9]\d*$/.test(actorID) || !Number.isSafeInteger(numericActorID)) throw new Error('invalid actor id')
  if (!Number.isSafeInteger(mallID) || mallID <= 0) throw new Error('invalid mall id')
  return `mall-weather-pending-sheet-push:${actorID}:${mallID}`
}

function validMallWeatherSheetPushKey(key: string) {
  return key.startsWith('weather-sheet-push:') && /^[A-Za-z0-9][A-Za-z0-9._:-]{7,254}$/.test(key)
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
