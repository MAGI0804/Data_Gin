export type MallWeatherExportProfileColumn = {
  field: string
  title: string
  width?: number
  format?: string
}

export type MallWeatherExportProfileConditionalFormat = {
  field: string
  operator: string
  value?: number
  secondValue?: number
  backgroundColor?: string
  fontColor?: string
}

export type MallWeatherExportProfileDataset = {
  kind: string
  sheetName: string
  columns: MallWeatherExportProfileColumn[]
  latest?: boolean
  asOf?: string
  splitBy?: string
  freezeHeader: boolean
  autoFilter: boolean
  maxRows?: number
  conditionalFormats: MallWeatherExportProfileConditionalFormat[]
}

export type MallWeatherExportProfileFilters = {
  mallIds: number[]
  cities: string[]
  mallStatuses: string[]
  qualityStatuses: string[]
  start?: string
  end?: string
}

export type MallWeatherExportProfile = {
  id: number
  code: string
  name: string
  version: number
  enabled: boolean
  timeZone: string
  unitSystem: 'metric' | 'imperial'
  dateFormat: string
  dateTimeFormat: string
  fileNameTemplate: string
  filters: MallWeatherExportProfileFilters
  datasets: MallWeatherExportProfileDataset[]
  createdBy: number
  updatedBy: number
  createdAt: string
  updatedAt: string
}

export type MallWeatherExportProfilePage = {
  items: MallWeatherExportProfile[]
  pageSize: number
  nextCursor: string
}

export type MallWeatherExportProfileForm = {
  code: string
  name: string
  enabled: boolean
  timeZone: string
  unitSystem: 'metric' | 'imperial'
  dateFormat: string
  dateTimeFormat: string
  fileNameTemplate: string
  mallIds: string
  cities: string
  mallStatuses: string
  qualityStatuses: string
  start: string
  end: string
  datasets: MallWeatherExportProfileDataset[]
}

const datasetKinds = new Set(['malls', 'realtime', 'minutely', 'hourly', 'daily', 'alerts', 'life_indices', 'fetch_runs'])
const splitByValues = new Set(['', 'city', 'mall', 'date', 'data_type'])
const columnFormats = new Set(['general', 'text', 'integer', 'decimal', 'percent', 'date', 'datetime'])
const conditionalOperators = new Set(['equal', 'not_equal', 'less_than', 'less_than_or_equal', 'greater_than', 'greater_than_or_equal', 'between', 'not_between'])
const profileCodePattern = /^[a-z][a-z0-9_-]{2,99}$/

export const mallWeatherExportProfileDatasetKinds = [...datasetKinds]

export function mallWeatherExportProfileReadOnly(code: string) {
  return code === 'mall_weather_excel_fixed' || code.startsWith('mall_weather_excel_fixed_')
}

export function mallWeatherExportProfileListPath(enabled: '' | 'true' | 'false', cursor = '') {
  const query = new URLSearchParams({ pageSize: '50' })
  if (enabled) query.set('enabled', enabled)
  if (cursor) query.set('cursor', cursor)
  return `/v1/weather-export-profiles?${query.toString()}`
}

export function parseMallWeatherExportProfilePage(payload: unknown): MallWeatherExportProfilePage | null {
  const data = envelopeData(payload)
  if (!data || !Array.isArray(data.items) || !isRecord(data.pagination) || !positiveSafeInteger(data.pagination.pageSize) || data.pagination.pageSize > 100) return null
  const items: MallWeatherExportProfile[] = []
  for (const item of data.items) {
    const profile = parseProfile(item)
    if (!profile) return null
    items.push(profile)
  }
  const nextCursor = data.pagination.nextCursor
  if (nextCursor !== undefined && (!trimmedString(nextCursor, 512) || /[\r\n]/.test(nextCursor))) return null
  return { items, pageSize: data.pagination.pageSize, nextCursor: typeof nextCursor === 'string' ? nextCursor : '' }
}

export function parseMallWeatherExportProfile(payload: unknown): MallWeatherExportProfile | null {
  const data = envelopeData(payload)
  return data ? parseProfile(data) : null
}

export function emptyMallWeatherExportProfileForm(): MallWeatherExportProfileForm {
  return {
    code: '', name: '', enabled: true, timeZone: 'Asia/Shanghai', unitSystem: 'metric',
    dateFormat: '2006-01-02', dateTimeFormat: '2006-01-02 15:04:05', fileNameTemplate: 'weather_export.xlsx',
    mallIds: '', cities: '', mallStatuses: 'active', qualityStatuses: '', start: '', end: '',
    datasets: [newDataset('hourly', '逐小时天气')],
  }
}

export function mallWeatherExportProfileForm(profile: MallWeatherExportProfile): MallWeatherExportProfileForm {
  return {
    code: profile.code, name: profile.name, enabled: profile.enabled, timeZone: profile.timeZone, unitSystem: profile.unitSystem,
    dateFormat: profile.dateFormat, dateTimeFormat: profile.dateTimeFormat, fileNameTemplate: profile.fileNameTemplate,
    mallIds: profile.filters.mallIds.join(','), cities: profile.filters.cities.join(','), mallStatuses: profile.filters.mallStatuses.join(','),
    qualityStatuses: profile.filters.qualityStatuses.join(','), start: profile.filters.start ?? '', end: profile.filters.end ?? '',
    datasets: profile.datasets.map(copyDataset),
  }
}

export function newMallWeatherExportProfileDataset(kind = 'hourly', sheetName = '逐小时天气'): MallWeatherExportProfileDataset {
  return newDataset(kind, sheetName)
}

export function mallWeatherExportProfileSaveRequest(form: MallWeatherExportProfileForm, expectedVersion?: number) {
  const code = form.code.trim().toLowerCase()
  const name = form.name.trim()
  const timeZone = form.timeZone.trim()
  const fileNameTemplate = form.fileNameTemplate.trim()
  if (!profileCodePattern.test(code) || !name || Array.from(name).length > 255 || !timeZone || !validFileName(fileNameTemplate) ||
    !['metric', 'imperial'].includes(form.unitSystem) || !validFormat(form.dateFormat) || !validFormat(form.dateTimeFormat)) throw new Error('invalid export profile')
  const filters: MallWeatherExportProfileFilters = {
    mallIds: numericList(form.mallIds, 1_000),
    cities: stringList(form.cities, 100),
    mallStatuses: allowedStringList(form.mallStatuses, new Set(['draft', 'active', 'disabled']), 3),
    qualityStatuses: allowedStringList(form.qualityStatuses, new Set(['valid', 'warning']), 2),
  }
  if (form.start.trim()) filters.start = rfc3339(form.start)
  if (form.end.trim()) filters.end = rfc3339(form.end)
  if (filters.start && filters.end && Date.parse(filters.start) >= Date.parse(filters.end)) throw new Error('invalid export profile')
  if (form.datasets.length < 1 || form.datasets.length > 8) throw new Error('invalid export profile')
  const datasets = form.datasets.map(parseDataset)
  const kinds = new Set(datasets.map((dataset) => dataset.kind))
  const sheets = new Set(datasets.map((dataset) => dataset.sheetName.toLowerCase()))
  if (kinds.size !== datasets.length || sheets.size !== datasets.length) throw new Error('invalid export profile')
  return {
    code, name, ...(expectedVersion ? { expectedVersion } : {}), enabled: form.enabled, timeZone,
    unitSystem: form.unitSystem, dateFormat: form.dateFormat.trim(), dateTimeFormat: form.dateTimeFormat.trim(), fileNameTemplate,
    filters, datasets,
  }
}

function parseProfile(value: unknown): MallWeatherExportProfile | null {
  if (!isRecord(value) || !positiveSafeInteger(value.id) || !trimmedString(value.code, 100) || !profileCodePattern.test(value.code) ||
    !trimmedString(value.name, 255) || !positiveSafeInteger(value.version) || typeof value.enabled !== 'boolean' ||
    !trimmedString(value.timeZone, 128) || (value.unitSystem !== 'metric' && value.unitSystem !== 'imperial') ||
    !validFormat(value.dateFormat) || !validFormat(value.dateTimeFormat) || !validFileName(value.fileNameTemplate) || !isRecord(value.filters) ||
    !Array.isArray(value.datasets) || value.datasets.length < 1 || value.datasets.length > 8 || !positiveSafeInteger(value.createdBy) ||
    !positiveSafeInteger(value.updatedBy) || !rfc3339OrNull(value.createdAt) || !rfc3339OrNull(value.updatedAt)) return null
  const filters = parseFilters(value.filters)
  if (!filters) return null
  const datasets: MallWeatherExportProfileDataset[] = []
  for (const dataset of value.datasets) {
    try { datasets.push(parseDataset(dataset)) } catch { return null }
  }
  if (new Set(datasets.map((dataset) => dataset.kind)).size !== datasets.length || new Set(datasets.map((dataset) => dataset.sheetName.toLowerCase())).size !== datasets.length) return null
  const unitSystem: 'metric' | 'imperial' = value.unitSystem === 'metric' ? 'metric' : 'imperial'
  const dateFormat = typeof value.dateFormat === 'string' ? value.dateFormat : ''
  const dateTimeFormat = typeof value.dateTimeFormat === 'string' ? value.dateTimeFormat : ''
  const fileNameTemplate = typeof value.fileNameTemplate === 'string' ? value.fileNameTemplate : ''
  return {
    id: value.id, code: value.code, name: value.name, version: value.version, enabled: value.enabled, timeZone: value.timeZone,
    unitSystem, dateFormat, dateTimeFormat, fileNameTemplate,
    filters, datasets, createdBy: value.createdBy, updatedBy: value.updatedBy, createdAt: value.createdAt, updatedAt: value.updatedAt,
  }
}

function parseFilters(value: Record<string, unknown>): MallWeatherExportProfileFilters | null {
  const mallIds = optionalArray(value.mallIds, positiveSafeInteger)
  const cities = optionalArray(value.cities, (item): item is string => trimmedString(item, 128))
  const mallStatuses = optionalArray(value.mallStatuses, (item): item is string => item === 'draft' || item === 'active' || item === 'disabled')
  const qualityStatuses = optionalArray(value.qualityStatuses, (item): item is string => item === 'valid' || item === 'warning')
  if (!mallIds || !cities || !mallStatuses || !qualityStatuses || mallIds.length > 1_000 || cities.length > 100 || mallStatuses.length > 3 || qualityStatuses.length > 2 ||
    new Set(mallIds).size !== mallIds.length || new Set(cities).size !== cities.length || new Set(mallStatuses).size !== mallStatuses.length || new Set(qualityStatuses).size !== qualityStatuses.length) return null
  const start = typeof value.start === 'string' && rfc3339OrNull(value.start) ? value.start : undefined
  const end = typeof value.end === 'string' && rfc3339OrNull(value.end) ? value.end : undefined
  if ((value.start !== undefined && !start) || (value.end !== undefined && !end) || (start && end && Date.parse(start) >= Date.parse(end))) return null
  return { mallIds, cities, mallStatuses, qualityStatuses, ...(start ? { start } : {}), ...(end ? { end } : {}) }
}

function parseDataset(value: unknown): MallWeatherExportProfileDataset {
  if (!isRecord(value) || !trimmedString(value.kind, 64) || !datasetKinds.has(value.kind) || !validSheetName(value.sheetName) ||
    typeof value.freezeHeader !== 'boolean' || typeof value.autoFilter !== 'boolean' ||
    (value.latest !== undefined && typeof value.latest !== 'boolean') || (value.asOf !== undefined && !rfc3339OrNull(value.asOf)) ||
    (value.splitBy !== undefined && (!trimmedString(value.splitBy, 32) || !splitByValues.has(value.splitBy))) ||
    (value.maxRows !== undefined && (!positiveSafeInteger(value.maxRows) || value.maxRows > 1_048_575))) throw new Error('invalid dataset')
  if (value.latest === true && value.asOf) throw new Error('invalid dataset')
  const columnsInput = optionalArray(value.columns, isRecord)
  const formatsInput = optionalArray(value.conditionalFormats, isRecord)
  if (!columnsInput || !formatsInput || columnsInput.length > 128 || formatsInput.length > 64) throw new Error('invalid dataset')
  const columns = columnsInput.map(parseColumn)
  if (new Set(columns.map((column) => column.field)).size !== columns.length) throw new Error('invalid dataset')
  const conditionalFormats = formatsInput.map((rule) => parseConditionalFormat(rule, columns))
  return {
    kind: value.kind, sheetName: value.sheetName, columns, freezeHeader: value.freezeHeader, autoFilter: value.autoFilter,
    conditionalFormats, ...(typeof value.latest === 'boolean' ? { latest: value.latest } : {}),
    ...(typeof value.asOf === 'string' ? { asOf: value.asOf } : {}), ...(typeof value.splitBy === 'string' ? { splitBy: value.splitBy } : {}),
    ...(typeof value.maxRows === 'number' ? { maxRows: value.maxRows } : {}),
  }
}

function parseColumn(value: unknown): MallWeatherExportProfileColumn {
  if (!isRecord(value) || !trimmedString(value.field, 128) || !trimmedString(value.title, 128) ||
    (value.width !== undefined && (!finiteNumber(value.width) || value.width < 0 || value.width > 255)) ||
    (value.format !== undefined && (!trimmedString(value.format, 32) || !columnFormats.has(value.format)))) throw new Error('invalid column')
  return { field: value.field, title: value.title, ...(typeof value.width === 'number' ? { width: value.width } : {}), ...(typeof value.format === 'string' ? { format: value.format } : {}) }
}

function parseConditionalFormat(value: unknown, columns: MallWeatherExportProfileColumn[]): MallWeatherExportProfileConditionalFormat {
  if (!isRecord(value) || !trimmedString(value.field, 128) || !trimmedString(value.operator, 64) || !conditionalOperators.has(value.operator) ||
    !finiteNumber(value.value) || (value.secondValue !== undefined && !finiteNumber(value.secondValue)) ||
    (value.backgroundColor !== undefined && !color(value.backgroundColor)) || (value.fontColor !== undefined && !color(value.fontColor)) ||
    (!value.backgroundColor && !value.fontColor) || (columns.length > 0 && !columns.some((column) => column.field === value.field))) throw new Error('invalid conditional format')
  const needsSecond = value.operator === 'between' || value.operator === 'not_between'
  if (needsSecond !== (typeof value.secondValue === 'number')) throw new Error('invalid conditional format')
  return { field: value.field, operator: value.operator, value: value.value, ...(typeof value.secondValue === 'number' ? { secondValue: value.secondValue } : {}), ...(typeof value.backgroundColor === 'string' ? { backgroundColor: value.backgroundColor } : {}), ...(typeof value.fontColor === 'string' ? { fontColor: value.fontColor } : {}) }
}

function newDataset(kind: string, sheetName: string): MallWeatherExportProfileDataset {
  return { kind, sheetName, columns: [], freezeHeader: true, autoFilter: true, conditionalFormats: [] }
}

function copyDataset(dataset: MallWeatherExportProfileDataset): MallWeatherExportProfileDataset {
  return { ...dataset, columns: dataset.columns.map((column) => ({ ...column })), conditionalFormats: dataset.conditionalFormats.map((rule) => ({ ...rule })) }
}

function envelopeData(payload: unknown): Record<string, unknown> | null {
  return isRecord(payload) && payload.code === 0 && isRecord(payload.data) ? payload.data : null
}

function isRecord(value: unknown): value is Record<string, unknown> { return Boolean(value) && typeof value === 'object' && !Array.isArray(value) }
function positiveSafeInteger(value: unknown): value is number { return typeof value === 'number' && Number.isSafeInteger(value) && value > 0 }
function finiteNumber(value: unknown): value is number { return typeof value === 'number' && Number.isFinite(value) }
function trimmedString(value: unknown, limit: number): value is string { return typeof value === 'string' && value === value.trim() && value.length > 0 && Array.from(value).length <= limit }
function rfc3339OrNull(value: unknown): value is string { return typeof value === 'string' && value.length <= 64 && Number.isFinite(Date.parse(value)) && /(?:Z|[+-]\d{2}:\d{2})$/.test(value) }
function rfc3339(value: string) { if (!rfc3339OrNull(value.trim())) throw new Error('invalid export profile'); return value.trim() }
function validFormat(value: unknown) { return trimmedString(value, 64) && !hasControl(value) }
function validFileName(value: unknown) { return trimmedString(value, 255) && !hasControl(value) && !value.includes('/') && !value.includes('\\') && value.toLowerCase().endsWith('.xlsx') }
function validSheetName(value: unknown): value is string { return trimmedString(value, 31) && !hasControl(value) && !['[', ']', ':', '*', '?', '/', '\\'].some((character) => value.includes(character)) && value !== "'" }
function color(value: unknown): value is string { return typeof value === 'string' && /^#[0-9a-f]{6}$/.test(value) }
function hasControl(value: string) { return value.includes('\0') || value.includes('\r') || value.includes('\n') }
function optionalArray<T>(value: unknown, guard: (item: unknown) => item is T): T[] | null { if (value === undefined) return []; return Array.isArray(value) && value.every(guard) ? value : null }
function numericList(value: string, limit: number) { const values = value.split(',').map((item) => item.trim()).filter(Boolean); if (values.length > limit) throw new Error('invalid export profile'); const parsed = values.map(Number); if (!parsed.every((item) => Number.isSafeInteger(item) && item > 0) || new Set(parsed).size !== parsed.length) throw new Error('invalid export profile'); return parsed.sort((left, right) => left - right) }
function stringList(value: string, limit: number) { const values = value.split(',').map((item) => item.trim().toLowerCase()).filter(Boolean); if (values.length > limit || !values.every((item) => Array.from(item).length <= 128) || new Set(values).size !== values.length) throw new Error('invalid export profile'); return values.sort() }
function allowedStringList(value: string, allowed: Set<string>, limit: number) { const values = stringList(value, limit); if (!values.every((item) => allowed.has(item))) throw new Error('invalid export profile'); return values }
