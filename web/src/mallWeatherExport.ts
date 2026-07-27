export const mallWeatherExportPollIntervalMilliseconds = 2_000
export const mallWeatherExportMaximumPollAttempts = 150

export type MallWeatherExportDatasetKind =
  | 'malls'
  | 'realtime'
  | 'minutely'
  | 'hourly'
  | 'daily'
  | 'alerts'
  | 'life_indices'
  | 'fetch_runs'

export type MallWeatherExportDataset = {
  kind: MallWeatherExportDatasetKind
  sheetName: string
  latest?: boolean
}

export type MallWeatherExportProfile = {
  id: number
  code: string
  name: string
  version: number
  enabled: boolean
  timeZone: string
  datasets: MallWeatherExportDataset[]
}

export type MallWeatherExportProfilePage = {
  items: MallWeatherExportProfile[]
  pagination: { pageSize: number; nextCursor: string }
}

export type MallWeatherExportFilters = {
  mallIds: number[]
  start: string
  end: string
}

export type MallWeatherExportCreateRequest = {
  profileId: number
  expectedProfileVersion: number
  filters: MallWeatherExportFilters
}

export type MallWeatherExportPendingCreate = {
  key: string
  body: MallWeatherExportCreateRequest
}

export type MallWeatherExportSession = {
  pending: MallWeatherExportPendingCreate | null
  jobId: string
}

export type MallWeatherExportCreateResult = {
  jobId: string
  status: 'PENDING'
  profileId: number
  profileVersion: number
  estimatedRows: number
  createdBy: number
  createdAt: string
}

export type MallWeatherExportJobStatus = 'PENDING' | 'RUNNING' | 'SUCCEEDED' | 'FAILED' | 'CANCELLED' | 'EXPIRED'

export type MallWeatherExportJob = {
  jobId: string
  profileId: number
  profileVersion: number
  status: MallWeatherExportJobStatus
  totalRows: number
  processedRows: number
  currentSheet: string
  cancelRequested: boolean
  fileSizeBytes: number
  errorMessageSafe: string
  expiresAt?: string
}

export type MallWeatherExportDownload = {
  url: string
  expiresAt: string
}

export type MallWeatherExportDownloadReadiness = 'ready' | 'not-ready' | 'expired'

export const mallWeatherExportDownloadRequestTimeoutMilliseconds = 15_000

export class MallWeatherExportDownloadTimeoutError extends Error {
  constructor() {
    super('mall weather export download request timed out')
    this.name = 'MallWeatherExportDownloadTimeoutError'
  }
}

export type MallWeatherExportProfileSaveRequest = {
  code: string
  name: string
  expectedVersion?: number
  enabled: true
  timeZone: string
  unitSystem: 'metric'
  dateFormat: string
  dateTimeFormat: string
  fileNameTemplate: string
  filters: Record<string, never>
  datasets: Array<{
    kind: Exclude<MallWeatherExportDatasetKind, 'fetch_runs'>
    sheetName: string
    latest?: true
    freezeHeader: true
    autoFilter: true
  }>
}

type JsonRecord = Record<string, unknown>
type ExportStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

const exportProfileCodePattern = /^[a-z][a-z0-9_-]{2,99}$/
const exportJobIDPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
const exportKeyPattern = /^[A-Za-z0-9][A-Za-z0-9._:-]{7,254}$/
const exportDatasetKinds = new Set<MallWeatherExportDatasetKind>([
  'malls', 'realtime', 'minutely', 'hourly', 'daily', 'alerts', 'life_indices', 'fetch_runs',
])
const exportJobStatuses = new Set<MallWeatherExportJobStatus>([
  'PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'CANCELLED', 'EXPIRED',
])
const completeExportDatasetKinds: MallWeatherExportDatasetKind[] = [
  'malls', 'realtime', 'minutely', 'hourly', 'daily', 'alerts', 'life_indices',
]

function isRecord(value: unknown): value is JsonRecord {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function envelopeData(payload: unknown): JsonRecord | null {
  if (!isRecord(payload) || payload.code !== 0 || !isRecord(payload.data)) return null
  return payload.data
}

function positiveInteger(value: unknown): value is number {
  return Number.isSafeInteger(value) && Number(value) > 0
}

function nonNegativeInteger(value: unknown): value is number {
  return Number.isSafeInteger(value) && Number(value) >= 0
}

function nonEmptyString(value: unknown, maximumLength = 1_000): value is string {
  return typeof value === 'string' && value.trim() === value && value.length > 0 && Array.from(value).length <= maximumLength
}

function isRFC3339(value: unknown): value is string {
  if (typeof value !== 'string' || value.length < 20 || value.length > 64) return false
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) && /(?:Z|[+-]\d{2}:\d{2})$/.test(value)
}

function parseDataset(value: unknown): MallWeatherExportDataset | null {
  if (!isRecord(value) || typeof value.kind !== 'string' || !exportDatasetKinds.has(value.kind as MallWeatherExportDatasetKind) ||
    !nonEmptyString(value.sheetName, 31)) return null
  if (value.latest !== undefined && typeof value.latest !== 'boolean') return null
  return {
    kind: value.kind as MallWeatherExportDatasetKind,
    sheetName: value.sheetName,
    ...(typeof value.latest === 'boolean' ? { latest: value.latest } : {}),
  }
}

export function parseMallWeatherExportProfile(payload: unknown): MallWeatherExportProfile | null {
  const data = envelopeData(payload)
  return data ? parseProfile(data) : null
}

function parseProfile(value: unknown): MallWeatherExportProfile | null {
  if (!isRecord(value) || !positiveInteger(value.id) || !positiveInteger(value.version) || typeof value.enabled !== 'boolean' ||
    typeof value.code !== 'string' || !exportProfileCodePattern.test(value.code) || !nonEmptyString(value.name, 255) ||
    !nonEmptyString(value.timeZone, 128) || !Array.isArray(value.datasets) || value.datasets.length < 1 || value.datasets.length > 8) return null
  const datasets: MallWeatherExportDataset[] = []
  const kinds = new Set<MallWeatherExportDatasetKind>()
  for (const rawDataset of value.datasets) {
    const dataset = parseDataset(rawDataset)
    if (!dataset || kinds.has(dataset.kind)) return null
    kinds.add(dataset.kind)
    datasets.push(dataset)
  }
  return {
    id: value.id,
    code: value.code,
    name: value.name,
    version: value.version,
    enabled: value.enabled,
    timeZone: value.timeZone,
    datasets,
  }
}

export function parseMallWeatherExportProfilePage(payload: unknown): MallWeatherExportProfilePage | null {
  const data = envelopeData(payload)
  if (!data || !Array.isArray(data.items) || !isRecord(data.pagination) ||
    !positiveInteger(data.pagination.pageSize) || Number(data.pagination.pageSize) > 100 ||
    (data.pagination.nextCursor !== undefined && typeof data.pagination.nextCursor !== 'string')) return null
  const items: MallWeatherExportProfile[] = []
  const ids = new Set<number>()
  for (const value of data.items) {
    const profile = parseProfile(value)
    if (!profile || ids.has(profile.id)) return null
    ids.add(profile.id)
    items.push(profile)
  }
  return {
    items,
    pagination: {
      pageSize: Number(data.pagination.pageSize),
      nextCursor: typeof data.pagination.nextCursor === 'string' ? data.pagination.nextCursor : '',
    },
  }
}

export function mallWeatherExportProfilesPath(cursor = '', enabled = true) {
  const query = new URLSearchParams({ enabled: String(enabled), pageSize: '100' })
  if (cursor.trim()) query.set('cursor', cursor.trim())
  return `/v1/weather-export-profiles?${query.toString()}`
}

export function selectMallWeatherExportProfile(profiles: MallWeatherExportProfile[]): MallWeatherExportProfile | null {
  const enabledProfiles = profiles.filter((profile) => profile.enabled)
  if (enabledProfiles.length === 0) return null
  return enabledProfiles.sort((left, right) => {
    const leftKinds = new Set(left.datasets.map((dataset) => dataset.kind))
    const rightKinds = new Set(right.datasets.map((dataset) => dataset.kind))
    const leftForecast = Number(leftKinds.has('minutely') && leftKinds.has('hourly'))
    const rightForecast = Number(rightKinds.has('minutely') && rightKinds.has('hourly'))
    return rightForecast - leftForecast || rightKinds.size - leftKinds.size || left.code.localeCompare(right.code) || left.id - right.id
  })[0] ?? null
}

export function selectMallWeatherCompleteExportProfile(profiles: MallWeatherExportProfile[]): MallWeatherExportProfile | null {
  return profiles
    .filter((profile) => profile.enabled && profile.code === 'mall_weather_full' && profileHasCompleteWeatherData(profile))
    .sort((left, right) => right.version - left.version || left.id - right.id)[0] ?? null
}

function profileHasCompleteWeatherData(profile: MallWeatherExportProfile) {
  const kinds = new Set(profile.datasets.map((dataset) => dataset.kind))
  return completeExportDatasetKinds.every((kind) => kinds.has(kind))
}

export function mallWeatherDefaultExportProfileRequest(expectedVersion?: number): MallWeatherExportProfileSaveRequest {
  const dataset = (kind: Exclude<MallWeatherExportDatasetKind, 'fetch_runs'>, sheetName: string, latest = true) => ({
    kind,
    sheetName,
    ...(latest ? { latest: true as const } : {}),
    freezeHeader: true as const,
    autoFilter: true as const,
  })
  return {
    code: 'mall_weather_full',
    name: '商场天气完整导出',
    ...(expectedVersion ? { expectedVersion } : {}),
    enabled: true,
    timeZone: 'Asia/Shanghai',
    unitSystem: 'metric',
    dateFormat: '2006-01-02',
    dateTimeFormat: '2006-01-02 15:04:05',
    fileNameTemplate: '商场天气_{{date:20060102_150405}}.xlsx',
    filters: {},
    datasets: [
      dataset('malls', '商场', false),
      dataset('realtime', '实时天气'),
      dataset('minutely', '约1公里分钟降水'),
      dataset('hourly', '小时预报'),
      dataset('daily', '每日预报'),
      dataset('alerts', '气象预警'),
      dataset('life_indices', '生活指数'),
    ],
  }
}

export function mallWeatherExportCreateRequest(
  profile: MallWeatherExportProfile,
  mallID: number,
  now = new Date(),
): MallWeatherExportCreateRequest {
  if (!positiveInteger(mallID) || !positiveInteger(profile.id) || !positiveInteger(profile.version) || !Number.isFinite(now.getTime())) {
    throw new Error('invalid mall weather export request')
  }
  const hour = 60 * 60 * 1_000
  const start = new Date(Math.floor(now.getTime() / hour) * hour - 24 * hour)
  const end = new Date(start.getTime() + 17 * 24 * hour)
  return {
    profileId: profile.id,
    expectedProfileVersion: profile.version,
    filters: { mallIds: [mallID], start: start.toISOString(), end: end.toISOString() },
  }
}

export function mallWeatherExportKey(seed?: string) {
  const suffix = (seed ?? globalThis.crypto?.randomUUID?.() ?? '').trim()
  const key = `weather-export:${suffix}`
  if (!exportKeyPattern.test(key)) throw new Error('invalid mall weather export key')
  return key
}

export function mallWeatherExportRequestMatches(
  request: MallWeatherExportCreateRequest,
  profile: MallWeatherExportProfile,
  mallID: number,
) {
  return request.profileId === profile.id && request.expectedProfileVersion === profile.version &&
    request.filters.mallIds.length === 1 && request.filters.mallIds[0] === mallID
}

export function loadMallWeatherExportSession(
  actorID: string,
  mallID: number,
  storage: ExportStorage,
): MallWeatherExportSession | null {
  const storageKey = mallWeatherExportSessionStorageKey(actorID, mallID)
  const raw = storage.getItem(storageKey)
  if (!raw) return null
  try {
    const snapshot: unknown = JSON.parse(raw)
    if (!isRecord(snapshot)) return null
    const pending = snapshot.pending === null || snapshot.pending === undefined
      ? null
      : parseStoredPendingCreate(snapshot.pending, mallID)
    const jobId = snapshot.jobId === undefined ? '' : snapshot.jobId
    if (pending === null && snapshot.pending !== null && snapshot.pending !== undefined ||
      typeof jobId !== 'string' || jobId !== '' && !exportJobIDPattern.test(jobId) ||
      !pending && !jobId) return null
    return { pending, jobId }
  } catch {
    return null
  }
}

export function saveMallWeatherExportSession(
  actorID: string,
  mallID: number,
  session: MallWeatherExportSession,
  storage: ExportStorage,
) {
  const storageKey = mallWeatherExportSessionStorageKey(actorID, mallID)
  if (!session.pending && !session.jobId) {
    storage.removeItem(storageKey)
    return
  }
  const pending = session.pending
  if (pending && (!exportKeyPattern.test(pending.key) ||
    !storedExportRequestIsValid(pending.body, mallID)) ||
    session.jobId && !exportJobIDPattern.test(session.jobId)) {
    throw new Error('invalid mall weather export session')
  }
  storage.setItem(storageKey, JSON.stringify(session))
}

export function clearMallWeatherExportSession(actorID: string, mallID: number, storage: ExportStorage) {
  storage.removeItem(mallWeatherExportSessionStorageKey(actorID, mallID))
}

function parseStoredPendingCreate(value: unknown, mallID: number): MallWeatherExportPendingCreate | null {
  if (!isRecord(value) || typeof value.key !== 'string' || !exportKeyPattern.test(value.key) ||
    !isRecord(value.body)) return null
  const body = value.body
  if (!positiveInteger(body.profileId) || !positiveInteger(body.expectedProfileVersion) ||
    !isRecord(body.filters) || !Array.isArray(body.filters.mallIds) ||
    body.filters.mallIds.length !== 1 || body.filters.mallIds[0] !== mallID ||
    !isRFC3339(body.filters.start) || !isRFC3339(body.filters.end)) return null
  const request: MallWeatherExportCreateRequest = {
    profileId: body.profileId,
    expectedProfileVersion: body.expectedProfileVersion,
    filters: {
      mallIds: [mallID],
      start: body.filters.start,
      end: body.filters.end,
    },
  }
  return storedExportRequestIsValid(request, mallID) ? { key: value.key, body: request } : null
}

function storedExportRequestIsValid(request: MallWeatherExportCreateRequest, mallID: number) {
  if (!positiveInteger(request.profileId) || !positiveInteger(request.expectedProfileVersion) ||
    request.filters.mallIds.length !== 1 || request.filters.mallIds[0] !== mallID ||
    !isRFC3339(request.filters.start) || !isRFC3339(request.filters.end)) return false
  const start = Date.parse(request.filters.start)
  const end = Date.parse(request.filters.end)
  return end > start && end - start <= 366 * 24 * 60 * 60 * 1_000
}

function mallWeatherExportSessionStorageKey(actorID: string, mallID: number) {
  const numericActorID = Number(actorID)
  if (!/^[1-9]\d*$/.test(actorID) || !Number.isSafeInteger(numericActorID)) throw new Error('invalid actor id')
  if (!positiveInteger(mallID)) throw new Error('invalid mall id')
  return `mall-weather-export:${actorID}:${mallID}`
}

export function parseMallWeatherExportCreateResult(payload: unknown): MallWeatherExportCreateResult | null {
  const data = envelopeData(payload)
  if (!data || typeof data.jobId !== 'string' || !exportJobIDPattern.test(data.jobId) || data.status !== 'PENDING' ||
    !positiveInteger(data.profileId) || !positiveInteger(data.profileVersion) || !nonNegativeInteger(data.estimatedRows) ||
    !positiveInteger(data.createdBy) || !isRFC3339(data.createdAt)) return null
  return {
    jobId: data.jobId,
    status: 'PENDING',
    profileId: data.profileId,
    profileVersion: data.profileVersion,
    estimatedRows: data.estimatedRows,
    createdBy: data.createdBy,
    createdAt: data.createdAt,
  }
}

export function parseMallWeatherExportJob(payload: unknown): MallWeatherExportJob | null {
  const data = envelopeData(payload)
  if (!data || typeof data.jobId !== 'string' || !exportJobIDPattern.test(data.jobId) || !positiveInteger(data.profileId) ||
    !positiveInteger(data.profileVersion) || typeof data.status !== 'string' || !exportJobStatuses.has(data.status as MallWeatherExportJobStatus) ||
    !nonNegativeInteger(data.totalRows) || !nonNegativeInteger(data.processedRows) || Number(data.processedRows) > Number(data.totalRows) ||
    typeof data.cancelRequested !== 'boolean' || !nonNegativeInteger(data.fileSizeBytes) ||
    (data.currentSheet !== undefined && (typeof data.currentSheet !== 'string' || Array.from(data.currentSheet).length > 255)) ||
    (data.errorMessageSafe !== undefined && (typeof data.errorMessageSafe !== 'string' || Array.from(data.errorMessageSafe).length > 2_000)) ||
    (data.expiresAt !== undefined && !isRFC3339(data.expiresAt))) return null
  return {
    jobId: data.jobId,
    profileId: data.profileId,
    profileVersion: data.profileVersion,
    status: data.status as MallWeatherExportJobStatus,
    totalRows: data.totalRows,
    processedRows: data.processedRows,
    currentSheet: typeof data.currentSheet === 'string' ? data.currentSheet : '',
    cancelRequested: data.cancelRequested,
    fileSizeBytes: data.fileSizeBytes,
    errorMessageSafe: typeof data.errorMessageSafe === 'string' ? data.errorMessageSafe : '',
    ...(typeof data.expiresAt === 'string' ? { expiresAt: data.expiresAt } : {}),
  }
}

export function mallWeatherExportJobTerminal(status: MallWeatherExportJobStatus) {
  return status === 'SUCCEEDED' || status === 'FAILED' || status === 'CANCELLED' || status === 'EXPIRED'
}

export function mallWeatherExportDownloadReadiness(
  job: Pick<MallWeatherExportJob, 'status' | 'expiresAt'>,
  now = new Date(),
): MallWeatherExportDownloadReadiness {
  if (job.status === 'EXPIRED') return 'expired'
  if (job.status !== 'SUCCEEDED') return 'not-ready'
  const expiresAt = job.expiresAt ? Date.parse(job.expiresAt) : Number.NaN
  if (!Number.isFinite(now.getTime()) || !Number.isFinite(expiresAt) || expiresAt - now.getTime() < 60_000) return 'expired'
  return 'ready'
}

export function mallWeatherExportProgress(job: Pick<MallWeatherExportJob, 'processedRows' | 'totalRows'>) {
  if (job.totalRows <= 0) return job.processedRows > 0 ? 100 : 0
  return Math.min(100, Math.max(0, Math.round(job.processedRows * 100 / job.totalRows)))
}

export function parseMallWeatherExportDownload(payload: unknown): MallWeatherExportDownload | null {
  const data = envelopeData(payload)
  if (!data || typeof data.url !== 'string' || data.url.length > 8_192 || !isRFC3339(data.expiresAt)) return null
  try {
    const url = new URL(data.url)
    if (url.protocol !== 'https:' || !url.hostname || url.username || url.password ||
      url.hostname.toLowerCase().endsWith('-internal.aliyuncs.com')) return null
  } catch {
    return null
  }
  return { url: data.url, expiresAt: data.expiresAt }
}

export async function waitForMallWeatherExportDownload<T>(
  request: Promise<T>,
  controller: AbortController,
  timeoutMilliseconds = mallWeatherExportDownloadRequestTimeoutMilliseconds,
): Promise<T> {
  if (!Number.isSafeInteger(timeoutMilliseconds) || timeoutMilliseconds < 1 || controller.signal.aborted) {
    throw new Error('invalid mall weather export download timeout')
  }
  let timer = 0
  try {
    return await new Promise<T>((resolve, reject) => {
      timer = globalThis.setTimeout(() => {
        controller.abort()
        reject(new MallWeatherExportDownloadTimeoutError())
      }, timeoutMilliseconds)
      request.then(resolve, reject)
    })
  } finally {
    globalThis.clearTimeout(timer)
  }
}

export function parseMallWeatherExportSafeErrorMessage(payload: unknown): string | null {
  if (!isRecord(payload) || typeof payload.code !== 'number' || payload.code === 0 ||
    !nonEmptyString(payload.msg, 200)) return null
  return payload.msg
}

export function mallWeatherExportJobPath(jobID: string) {
  if (!exportJobIDPattern.test(jobID)) throw new Error('invalid mall weather export job id')
  return `/v1/weather-exports/${jobID}`
}

export function mallWeatherExportDownloadPath(jobID: string) {
  return `${mallWeatherExportJobPath(jobID)}/download`
}
