import type { ClientResponse, HTTPMethod } from '../api/client'
import type { ReportCatalogPage, ReportCatalogQuery, ReportDefinitionStatus, ReportExport, ReportParameter, ReportResultPage, ReportRun, ReportRunContract, ReportRunStatus, ReportSummary } from './types'

type JsonRecord = Record<string, unknown>

export type ReportCenterClient = (path: string, options?: {
  method?: HTTPMethod
  body?: unknown
  signal?: AbortSignal
  showResult?: boolean
  silentLoading?: boolean
}) => Promise<ClientResponse>

type ReportAPIResult<T> = { ok: true; data: T } | { ok: false; error: string }

export async function getReportCatalog(client: ReportCenterClient, query: ReportCatalogQuery, signal?: AbortSignal) {
  const search = new URLSearchParams()
  if (query.search?.trim()) search.set('search', query.search.trim())
  if (query.category?.trim()) search.set('category', query.category.trim())
  if (query.afterId && query.afterId > 0) search.set('afterId', String(query.afterId))
  search.set('limit', String(Math.min(100, Math.max(1, query.limit ?? 50))))

  const response = await client(`/v1/reports?${search.toString()}`, {
    method: 'GET',
    signal,
    showResult: false,
    silentLoading: true,
  })
  if (!response.ok) {
    return { ok: false as const, error: response.error?.message ?? '报表目录加载失败，请稍后重试。' }
  }
  return { ok: true as const, page: parseReportCatalogPage(response.data) }
}

export function parseReportCatalogPage(payload: unknown): ReportCatalogPage {
  const data = unwrapData(payload)
  const rawItems = firstArray(data.items, data.reports, data.definitions)
  const items = rawItems.flatMap((item) => {
    const parsed = parseReportSummary(item)
    return parsed ? [parsed] : []
  })
  return {
    items,
    hasMore: data.hasMore === true,
    nextAfterId: positiveInteger(data.nextAfterId) ?? positiveInteger(data.next_after_id) ?? 0,
  }
}

export async function getReportRunContract(client: ReportCenterClient, reportId: number, signal?: AbortSignal): Promise<ReportAPIResult<ReportRunContract>> {
  return requestAndParse(client, `/v1/reports/${reportId}/run-contract`, { method: 'GET', signal }, parseReportRunContract, '报表运行参数加载失败。')
}

export async function createReportRun(client: ReportCenterClient, reportId: number, parameters: Record<string, unknown>): Promise<ReportAPIResult<ReportRun>> {
  return requestAndParse(client, `/v1/reports/${reportId}/runs`, { method: 'POST', body: { parameters } }, parseReportRun, '报表运行创建失败。')
}

export async function getReportRun(client: ReportCenterClient, runId: number, signal?: AbortSignal): Promise<ReportAPIResult<ReportRun>> {
  return requestAndParse(client, `/v1/report-runs/${runId}`, { method: 'GET', signal }, parseReportRun, '报表运行状态加载失败。')
}

export async function cancelReportRun(client: ReportCenterClient, runId: number): Promise<ReportAPIResult<ReportRun>> {
  return requestAndParse(client, `/v1/report-runs/${runId}/cancel`, { method: 'POST' }, parseReportRun, '报表运行取消失败。')
}

export async function getReportResults(client: ReportCenterClient, runId: number, cursor = '', limit = 100, signal?: AbortSignal): Promise<ReportAPIResult<ReportResultPage>> {
  const query = new URLSearchParams({ limit: String(limit) })
  if (cursor) query.set('cursor', cursor)
  return requestAndParse(client, `/v1/report-runs/${runId}/results?${query}`, { method: 'GET', signal }, parseReportResultPage, '报表结果加载失败。')
}

export async function createReportExport(client: ReportCenterClient, runId: number): Promise<ReportAPIResult<ReportExport>> {
  return requestAndParse(client, `/v1/report-runs/${runId}/export`, { method: 'POST' }, parseReportExport, '正式导出创建失败。')
}

export async function getReportExport(client: ReportCenterClient, exportId: number, signal?: AbortSignal): Promise<ReportAPIResult<ReportExport>> {
  return requestAndParse(client, `/v1/report-exports/${exportId}`, { method: 'GET', signal }, parseReportExport, '导出状态加载失败。')
}

export async function getReportExportDownload(client: ReportCenterClient, exportId: number): Promise<ReportAPIResult<{ url: string; expiresAt: string | null }>> {
  return requestAndParse(client, `/v1/report-exports/${exportId}/download`, { method: 'GET' }, parseReportExportDownload, '下载地址获取失败。')
}

export function parseReportRunContract(payload: unknown): ReportRunContract {
  const data = unwrapData(payload)
  const definitionId = positiveInteger(data.definitionId)
  const versionId = positiveInteger(data.versionId)
  if (!definitionId || !versionId) throw new Error('invalid run contract identity')
  const rawParameters = firstArray(data.parameters)
  const parameters = rawParameters.flatMap((value) => {
    const parameter = parseReportParameter(value)
    return parameter ? [parameter] : []
  })
  if (parameters.length !== rawParameters.length) throw new Error('invalid run contract parameters')
  return {
    definitionId,
    versionId,
    code: publicString(data.code, 64),
    name: publicString(data.name, 128),
    description: publicString(data.description, 500),
    parameters: parameters.sort((left, right) => left.displayOrder - right.displayOrder || left.position - right.position),
  }
}

export function parseReportRun(payload: unknown): ReportRun {
  const data = unwrapData(payload)
  const id = positiveInteger(data.id)
  const definitionId = positiveInteger(data.definitionId)
  const versionId = positiveInteger(data.versionId)
  if (!id || !definitionId || !versionId) throw new Error('invalid report run')
  return {
    id,
    runUuid: publicString(data.runUuid, 64),
    definitionId,
    versionId,
    status: reportRunStatus(data.status),
    rowCount: nonNegativeInteger(data.rowCount),
    cancelRequested: data.cancelRequested === true,
    createdAt: publicDate(data.createdAt),
    startedAt: publicDate(data.startedAt),
    finishedAt: publicDate(data.finishedAt),
    resultExpiresAt: publicDate(data.resultExpiresAt),
    errorCode: publicString(data.errorCode, 100),
    errorMessage: publicString(data.errorMessage, 500),
    canCancel: data.canCancel === true,
    resultAvailable: data.resultAvailable === true,
  }
}

export function parseReportResultPage(payload: unknown): ReportResultPage {
  const data = unwrapData(payload)
  const run = parseReportRun(data.run)
  const pagination = isRecord(data.pagination) ? data.pagination : {}
  const rawColumns = firstArray(data.columns)
  const columns = rawColumns.flatMap((value) => {
    if (!isRecord(value)) return []
    const fieldId = publicString(value.fieldId, 64)
    const code = publicString(value.code, 64)
    if (!fieldId || !code) return []
    return [{ fieldId, code, header: publicString(value.header, 128) || code, valueType: publicString(value.valueType, 32), nullable: value.nullable === true, nullDisplay: publicString(value.nullDisplay, 32) }]
  })
  const rawRows = firstArray(data.rows)
  const rows = rawRows.flatMap((value) => {
    if (!isRecord(value) || !isRecord(value.values)) return []
    const key = publicString(value.key, 128)
    return key ? [{ key, values: value.values }] : []
  })
  if (columns.length !== rawColumns.length || rows.length !== rawRows.length) throw new Error('invalid report result page')
  const nextCursor = publicString(pagination.nextCursor, 1024)
  if (pagination.hasMore === true && !nextCursor) throw new Error('invalid report result cursor')
  return {
    run,
    columns,
    rows,
    pagination: {
      pageSize: positiveInteger(pagination.pageSize) ?? 100,
      hasMore: pagination.hasMore === true,
      nextCursor,
    },
  }
}

export function parseReportExport(payload: unknown): ReportExport {
  const data = unwrapData(payload)
  const id = positiveInteger(data.id)
  const runId = positiveInteger(data.runId)
  if (!id || !runId) throw new Error('invalid report export')
  return {
    id, runId,
    exportUuid: publicString(data.exportUuid, 64),
    status: reportExportStatus(data.status),
    processedRows: nonNegativeInteger(data.processedRows),
    exportedRows: nonNegativeInteger(data.exportedRows),
    currentSheet: publicString(data.currentSheet, 64),
    sheetCount: nonNegativeInteger(data.sheetCount),
    truncatedCellCount: nonNegativeInteger(data.truncatedCellCount),
    fileSizeBytes: nonNegativeInteger(data.fileSizeBytes),
    createdAt: publicDate(data.createdAt), startedAt: publicDate(data.startedAt), readyAt: publicDate(data.readyAt),
    expiresAt: publicDate(data.expiresAt), purgedAt: publicDate(data.purgedAt),
    errorCode: publicString(data.errorCode, 100), errorMessage: publicString(data.errorMessage, 500),
    canDownload: data.canDownload === true,
  }
}

async function requestAndParse<T>(client: ReportCenterClient, path: string, options: Parameters<ReportCenterClient>[1], parse: (payload: unknown) => T, fallback: string): Promise<ReportAPIResult<T>> {
  const response = await client(path, { ...options, showResult: false, silentLoading: true })
  if (!response.ok) return { ok: false, error: response.error?.message ?? fallback }
  try {
    return { ok: true, data: parse(response.data) }
  } catch {
    return { ok: false, error: '服务返回的数据格式不完整，请稍后重试。' }
  }
}

function parseReportExportDownload(payload: unknown) {
  const data = unwrapData(payload)
  const allowLocalHTTP = typeof window !== 'undefined' && ['localhost', '127.0.0.1', '::1'].includes(window.location.hostname)
  const url = typeof data.url === 'string' && (/^https:\/\//.test(data.url) || (allowLocalHTTP && /^http:\/\/(?:localhost|127\.0\.0\.1|\[::1\])(?::\d+)?\//.test(data.url))) ? data.url : ''
  if (!url) throw new Error('invalid report download')
  return { url, expiresAt: publicDate(data.expiresAt) }
}

function parseReportParameter(value: unknown): ReportParameter | null {
  if (!isRecord(value)) return null
  const code = publicString(value.code, 64)
  if (!code) return null
  const allowedValues = Array.isArray(value.allowedValues) ? value.allowedValues.filter((item): item is string => typeof item === 'string') : []
  return {
    code,
    label: publicString(value.label, 128) || code,
    displayOrder: nonNegativeInteger(value.displayOrder),
    controlType: reportControlType(value.controlType),
    logicalType: reportLogicalType(value.logicalType),
    cardinality: value.cardinality === 'MULTIPLE' ? 'MULTIPLE' : 'SINGLE',
    procedureArgName: publicString(value.procedureArgName, 128),
    position: positiveInteger(value.position) ?? 0,
    oracleType: publicString(value.oracleType, 64),
    precision: nullableInteger(value.precision), scale: nullableInteger(value.scale), maxLength: nullableInteger(value.maxLength),
    required: value.required === true, nullable: value.nullable === true, systemInjected: value.systemInjected === true, sensitive: value.sensitive === true,
    defaultValue: value.defaultValue,
    allowedValues,
    validation: isRecord(value.validation) ? value.validation : {},
    timezone: publicString(value.timezone, 64),
    errorMessage: publicString(value.errorMessage, 300),
  }
}

function parseReportSummary(value: unknown): ReportSummary | null {
  if (!isRecord(value)) return null
  const definition = isRecord(value.definition) ? value.definition : value
  const id = positiveInteger(definition.id)
  const code = publicString(definition.code, 64)
  const name = publicString(definition.name, 128)
  if (!id || !code || !name) return null
  return {
    id,
    code,
    name,
    category: publicString(definition.category, 64),
    description: publicString(definition.description, 500),
    datasourceId: positiveInteger(definition.datasourceId) ?? 0,
    status: reportStatus(definition.status),
    currentDraftVersionId: positiveInteger(definition.currentDraftVersionId) ?? 0,
    currentPublishedVersionId: positiveInteger(definition.currentPublishedVersionId) ?? 0,
    lockVersion: positiveInteger(value.lockVersion) ?? 0,
    updatedAt: publicDate(definition.updatedAt),
  }
}

function unwrapData(payload: unknown): JsonRecord {
  if (!isRecord(payload)) return {}
  return isRecord(payload.data) ? payload.data : payload
}

function firstArray(...values: unknown[]) {
  return values.find(Array.isArray) ?? []
}

function isRecord(value: unknown): value is JsonRecord {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function publicString(value: unknown, maximumLength: number) {
  return typeof value === 'string' ? value.trim().slice(0, maximumLength) : ''
}

function positiveInteger(value: unknown) {
  if (typeof value === 'number' && Number.isSafeInteger(value) && value > 0) return value
  if (typeof value !== 'string' || !/^[1-9]\d*$/.test(value)) return null
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) ? parsed : null
}

function nonNegativeInteger(value: unknown) {
  if (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0) return value
  if (typeof value !== 'string' || !/^\d+$/.test(value)) return 0
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) ? parsed : 0
}

function nullableInteger(value: unknown) {
  if (typeof value === 'number' && Number.isSafeInteger(value)) return value
  return null
}

function publicDate(value: unknown) {
  if (typeof value !== 'string' || !value.trim()) return null
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? null : value
}

function reportStatus(value: unknown): ReportDefinitionStatus {
  return value === 'ACTIVE' || value === 'DISABLED' ? value : 'DRAFT'
}

function reportRunStatus(value: unknown): ReportRunStatus {
  const allowed: ReportRunStatus[] = ['QUEUED', 'RUNNING', 'CANCEL_REQUESTED', 'SUCCEEDED', 'FAILED', 'CANCELLED', 'UNKNOWN', 'RECONCILING', 'EXPORTING', 'EXPORTED', 'RESULT_PURGING', 'RESULT_PURGED']
  return typeof value === 'string' && allowed.includes(value as ReportRunStatus) ? value as ReportRunStatus : 'UNKNOWN'
}

function reportExportStatus(value: unknown): ReportExport['status'] {
  const allowed: ReportExport['status'][] = ['PENDING', 'RUNNING', 'READY', 'FAILED', 'CANCELLED', 'EXPIRED']
  return typeof value === 'string' && allowed.includes(value as ReportExport['status']) ? value as ReportExport['status'] : 'FAILED'
}

function reportControlType(value: unknown): ReportParameter['controlType'] {
  const allowed: ReportParameter['controlType'][] = ['TEXT', 'TEXTAREA', 'NUMBER', 'CHECKBOX', 'DATE', 'DATETIME', 'SELECT', 'MULTI_SELECT']
  return typeof value === 'string' && allowed.includes(value as ReportParameter['controlType']) ? value as ReportParameter['controlType'] : 'TEXT'
}

function reportLogicalType(value: unknown): ReportParameter['logicalType'] {
  const allowed: ReportParameter['logicalType'][] = ['string', 'integer', 'decimal', 'boolean', 'date', 'datetime', 'enum', 'multi_enum', 'json']
  return typeof value === 'string' && allowed.includes(value as ReportParameter['logicalType']) ? value as ReportParameter['logicalType'] : 'string'
}
