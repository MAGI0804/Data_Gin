import type { ClientResponse, HTTPMethod } from '../api/client'
import type { ReportAuditPage, ReportAuditQuery, ReportCatalogPage, ReportCatalogQuery, ReportColumn, ReportDatasource, ReportDatasourceInput, ReportDatasourceTest, ReportDefinitionStatus, ReportDraft, ReportExport, ReportExportPage, ReportFilterOperator, ReportGrant, ReportParameter, ReportResultPage, ReportResultQuery, ReportRun, ReportRunContract, ReportRunStatus, ReportSummary } from './types'

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

export async function getReportDatasources(client: ReportCenterClient, signal?: AbortSignal): Promise<ReportAPIResult<ReportDatasource[]>> {
  return requestAndParse(client, '/v1/report-datasources', { method: 'GET', signal }, parseReportDatasources, 'Oracle 数据源加载失败。')
}

export async function createReportDatasource(client: ReportCenterClient, input: ReportDatasourceInput): Promise<ReportAPIResult<ReportDatasource>> {
  return requestAndParse(client, '/v1/report-datasources', { method: 'POST', body: serializeReportDatasource(input) }, parseReportDatasource, 'Oracle 数据源创建失败。')
}

export async function updateReportDatasource(client: ReportCenterClient, datasourceId: number, input: ReportDatasourceInput): Promise<ReportAPIResult<ReportDatasource>> {
  return requestAndParse(client, `/v1/report-datasources/${datasourceId}`, { method: 'PUT', body: serializeReportDatasource(input) }, parseReportDatasource, 'Oracle 数据源更新失败。')
}

export async function testReportDatasource(client: ReportCenterClient, datasourceId: number): Promise<ReportAPIResult<ReportDatasourceTest>> {
  return requestAndParse(client, `/v1/report-datasources/${datasourceId}/test`, { method: 'POST' }, parseReportDatasourceTest, 'Oracle 连接测试失败。')
}

export function parseReportDatasources(payload: unknown): ReportDatasource[] {
  const data = unwrapData(payload)
  const rawItems = firstArray(data.items)
  const items = rawItems.flatMap((value) => {
    try { return [parseReportDatasource(value)] } catch { return [] }
  })
  if (items.length !== rawItems.length) throw new Error('invalid report datasource list')
  return items
}

export function parseReportDatasource(payload: unknown): ReportDatasource {
  const data = unwrapData(payload)
  for (const forbidden of ['password', 'passwordCiphertext', 'credentialKeyVersion', 'sessionInitJSON', 'dsn']) {
    if (Object.prototype.hasOwnProperty.call(data, forbidden)) throw new Error('sensitive report datasource field')
  }
  const id = positiveInteger(data.id)
  const code = publicString(data.code, 64)
  const name = publicString(data.name, 128)
  const host = publicString(data.host, 255)
  const port = positiveInteger(data.port)
  const serviceName = publicString(data.serviceName, 128)
  const sid = publicString(data.sid, 128)
  const username = publicString(data.username, 128)
  if (!id || !code || !name || data.driver !== 'ORACLE' || !host || !port || port > 65535 || !username || (Boolean(serviceName) === Boolean(sid)) || typeof data.enabled !== 'boolean' || typeof data.hasPassword !== 'boolean') {
    throw new Error('invalid report datasource')
  }
  return {
    id, code, name, driver: 'ORACLE', host, port, serviceName, sid, username,
    hasPassword: data.hasPassword,
    sessionTimezone: publicString(data.sessionTimezone, 64) || 'Asia/Shanghai',
    connectTimeoutSeconds: positiveInteger(data.connectTimeoutSeconds) ?? 5,
    queryTimeoutSeconds: positiveInteger(data.queryTimeoutSeconds) ?? 300,
    maxOpenConnections: positiveInteger(data.maxOpenConnections) ?? 10,
    maxIdleConnections: nonNegativeInteger(data.maxIdleConnections),
    prefetchRows: positiveInteger(data.prefetchRows) ?? 1000,
    arraySize: positiveInteger(data.arraySize) ?? 1000,
    enabled: data.enabled,
    lastTestStatus: data.lastTestStatus === 'SUCCESS' || data.lastTestStatus === 'FAILED' ? data.lastTestStatus : 'NOT_TESTED',
    lastTestError: publicString(data.lastTestError, 300),
    lastTestedAt: publicDate(data.lastTestedAt),
  }
}

export function parseReportDatasourceTest(payload: unknown): ReportDatasourceTest {
  const data = unwrapData(payload)
  const status = data.status === 'SUCCESS' || data.status === 'FAILED' ? data.status : null
  const testedAt = publicDate(data.testedAt)
  const message = publicString(data.message, 300)
  if (!status || !testedAt || !message) throw new Error('invalid report datasource test')
  return { status, testedAt, latencyMs: nonNegativeInteger(data.latencyMs), errorCode: publicString(data.errorCode, 100), message }
}

export async function getReportDraft(client: ReportCenterClient, reportId: number, signal?: AbortSignal): Promise<ReportAPIResult<ReportDraft>> {
  return requestAndParse(client, `/v1/reports/${reportId}`, { method: 'GET', signal }, parseReportDraft, '报表草稿加载失败。')
}

export async function saveReportDraft(client: ReportCenterClient, draft: ReportDraft): Promise<ReportAPIResult<ReportDraft>> {
  const creating = draft.id === 0
  const body = serializeReportDraft(draft, creating)
  return requestAndParse(client, creating ? '/v1/reports' : `/v1/reports/${draft.id}`, { method: creating ? 'POST' : 'PUT', body }, parseReportDraft, '报表草稿保存失败。')
}

export async function publishReportDraft(client: ReportCenterClient, reportId: number, expectedLockVersion: number): Promise<ReportAPIResult<{ definitionId: number; versionId: number; status: string }>> {
  return requestAndParse(client, `/v1/reports/${reportId}/publish`, { method: 'POST', body: { expectedLockVersion } }, parsePublication, '报表发布与 Oracle 契约核验失败。')
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

export async function queryReportResults(client: ReportCenterClient, runId: number, query: ReportResultQuery, cursor = '', limit = 100, signal?: AbortSignal): Promise<ReportAPIResult<ReportResultPage>> {
	return requestAndParse(client, `/v1/report-runs/${runId}/results/query`, { method: 'POST', body: { ...query, cursor, limit }, signal }, parseReportResultPage, '报表结果加载失败。')
}

export async function createReportExport(client: ReportCenterClient, runId: number, query: ReportResultQuery): Promise<ReportAPIResult<ReportExport>> {
	return requestAndParse(client, `/v1/report-runs/${runId}/export`, { method: 'POST', body: query }, parseReportExport, '正式导出创建失败。')
}

export async function getReportExport(client: ReportCenterClient, exportId: number, signal?: AbortSignal): Promise<ReportAPIResult<ReportExport>> {
  return requestAndParse(client, `/v1/report-exports/${exportId}`, { method: 'GET', signal }, parseReportExport, '导出状态加载失败。')
}

export async function getReportExports(client: ReportCenterClient, query: { afterId?: number; limit?: number; status?: string }, signal?: AbortSignal): Promise<ReportAPIResult<ReportExportPage>> {
	const search = new URLSearchParams(); if (query.afterId) search.set('afterId', String(query.afterId)); if (query.limit) search.set('limit', String(query.limit)); if (query.status) search.set('status', query.status)
	return requestAndParse(client, `/v1/report-exports?${search}`, { method: 'GET', signal }, parseReportExportPage, '导出任务加载失败。')
}

export async function getReportExportDownload(client: ReportCenterClient, exportId: number): Promise<ReportAPIResult<{ url: string; expiresAt: string | null }>> {
  return requestAndParse(client, `/v1/report-exports/${exportId}/download`, { method: 'GET' }, parseReportExportDownload, '下载地址获取失败。')
}

export async function getReportAudits(client: ReportCenterClient, query: ReportAuditQuery, signal?: AbortSignal): Promise<ReportAPIResult<ReportAuditPage>> {
  const search = new URLSearchParams()
  if (query.action?.trim()) search.set('action', query.action.trim())
  if (query.targetType?.trim()) search.set('targetType', query.targetType.trim())
  if (query.targetId && query.targetId > 0) search.set('targetId', String(query.targetId))
  if (query.afterId && query.afterId > 0) search.set('afterId', String(query.afterId))
  search.set('limit', String(Math.min(100, Math.max(1, query.limit ?? 50))))
  return requestAndParse(client, `/v1/report-audits?${search}`, { method: 'GET', signal }, parseReportAuditPage, '报表审计记录加载失败。')
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

export function parseReportDraft(payload: unknown): ReportDraft {
  const data = unwrapData(payload)
  const id = positiveInteger(data.id)
  const datasourceId = positiveInteger(data.datasourceId)
  const procedure = isRecord(data.procedure) ? data.procedure : {}
  const result = isRecord(data.result) ? data.result : {}
  const rawParameters = firstArray(data.parameters)
  const parameters = rawParameters.flatMap((value) => { const item = parseReportParameter(value); return item ? [item] : [] })
  const rawColumns = firstArray(data.columns)
  const columns = rawColumns.flatMap((value) => { const item = parseReportColumn(value); return item ? [item] : [] })
  const rawGrants = firstArray(data.grants)
  const grants = rawGrants.flatMap((value) => { const item = parseReportGrant(value); return item ? [item] : [] })
  if (!id || !datasourceId || parameters.length !== rawParameters.length || columns.length !== rawColumns.length || grants.length !== rawGrants.length) throw new Error('invalid report draft')
  return {
    id, datasourceId, code: publicString(data.code, 64), name: publicString(data.name, 128), category: publicString(data.category, 64), description: publicString(data.description, 500),
    status: reportStatus(data.status), lockVersion: positiveInteger(data.lockVersion) ?? 0,
    procedure: { owner: publicString(procedure.owner, 128), package: publicString(procedure.package, 128), name: publicString(procedure.name, 128), overload: publicString(procedure.overload, 32) },
    result: { tableOwner: publicString(result.tableOwner, 128), tableName: publicString(result.tableName, 128), runIdColumn: publicString(result.runIdColumn, 128), rowIdColumn: publicString(result.rowIdColumn, 128) },
    callTemplate: typeof data.callTemplate === 'string' ? data.callTemplate.slice(0, 65536) : '', parameters, columns, grants,
    createdAt: publicDate(data.createdAt), updatedAt: publicDate(data.updatedAt),
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
		const allowedOperators = firstArray(value.allowedOperators).map(reportFilterOperator)
		return [{ fieldId, code, header: publicString(value.header, 128) || code, valueType: publicString(value.valueType, 32), nullable: value.nullable === true, nullDisplay: publicString(value.nullDisplay, 32), filterable: value.filterable === true, sortable: value.sortable === true, allowedOperators }]
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

function reportFilterOperator(value: unknown): ReportFilterOperator {
	const operator = publicString(value, 32) as ReportFilterOperator
	if (!['EQ','NE','GT','GTE','LT','LTE','IN','NOT_IN','IS_NULL','IS_NOT_NULL','CONTAINS','STARTS_WITH','BETWEEN'].includes(operator)) throw new Error('invalid report filter operator')
	return operator
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
		reportName: publicString(data.reportName, 128), purgedRows: nonNegativeInteger(data.purgedRows), purgeStartedAt: publicDate(data.purgeStartedAt),
  }
}

export function parseReportExportPage(payload: unknown): ReportExportPage {
	const data = unwrapData(payload); const rawItems = firstArray(data.items); const items = rawItems.map((item) => parseReportExport({ data: item }));
	const hasMore = data.hasMore === true
	const nextAfterId = nonNegativeInteger(data.nextAfterId)
	if (hasMore && nextAfterId < 1) throw new Error('invalid report export cursor')
	return { items, hasMore, nextAfterId }
}

export function parseReportAuditPage(payload: unknown): ReportAuditPage {
  const data = unwrapData(payload)
  const rawItems = firstArray(data.items)
  const items = rawItems.map((value) => {
    if (!isRecord(value)) throw new Error('invalid report audit')
    const id = positiveInteger(value.id)
    const actorUserId = positiveInteger(value.actorUserId)
    const action = publicString(value.action, 64)
    const targetType = publicString(value.targetType, 32)
    const targetId = positiveInteger(value.targetId)
    const requestId = publicString(value.requestId, 128)
    const createdAt = publicDate(value.createdAt)
    if (!id || !actorUserId || !action || !targetType || !targetId || !requestId || !createdAt) throw new Error('invalid report audit')
    return { id, actorUserId, action, targetType, targetId, requestId, detail: isRecord(value.detail) ? value.detail : {}, createdAt }
  })
  const hasMore = data.hasMore === true
  const nextAfterId = nonNegativeInteger(data.nextAfterId)
  if (hasMore && nextAfterId < 1) throw new Error('invalid report audit cursor')
  return { items, hasMore, nextAfterId }
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
    normalizer: isRecord(value.normalizer) ? value.normalizer : {},
    valueSource: isRecord(value.valueSource) ? value.valueSource : {},
    timezone: publicString(value.timezone, 64),
    nullPolicy: publicString(value.nullPolicy, 32) || 'TYPED_NULL',
    errorMessage: publicString(value.errorMessage, 300),
    collectionEncoding: publicString(value.collectionEncoding, 32),
  }
}

function parseReportColumn(value: unknown): ReportColumn | null {
  if (!isRecord(value)) return null
  const fieldId = publicString(value.fieldId, 36)
  const logicalCode = publicString(value.logicalCode, 64)
  const databaseColumn = publicString(value.databaseColumn, 128)
  if (!fieldId || !logicalCode || !databaseColumn) return null
  return {
    fieldId, logicalCode, databaseColumn, sourceOracleType: publicString(value.sourceOracleType, 64), precision: nullableInteger(value.precision), scale: nullableInteger(value.scale), nullable: value.nullable === true,
    valueType: publicString(value.valueType, 32), previewHeader: publicString(value.previewHeader, 255), excelHeader: publicString(value.excelHeader, 255), displayOrder: nonNegativeInteger(value.displayOrder), exportOrder: nonNegativeInteger(value.exportOrder),
    previewVisible: value.previewVisible === true, exportVisible: value.exportVisible === true, filterable: value.filterable === true, sortable: value.sortable === true, exportAllowed: value.exportAllowed === true,
    allowedOperators: value.allowedOperators, format: value.format, dictionaryVersion: value.dictionaryVersion, maskingPolicy: value.maskingPolicy, excelWidth: typeof value.excelWidth === 'number' ? value.excelWidth : 0, nullDisplay: publicString(value.nullDisplay, 64),
  }
}

function parseReportGrant(value: unknown): ReportGrant | null {
  if (!isRecord(value) || (value.subjectType !== 'USER' && value.subjectType !== 'ROLE')) return null
  const subjectId = positiveInteger(value.subjectId)
  const actions = Array.isArray(value.actions) ? value.actions.filter((item): item is string => item === 'QUERY' || item === 'EXPORT') : []
  return subjectId && actions.length ? { subjectType: value.subjectType, subjectId, actions } : null
}

function serializeReportDraft(draft: ReportDraft, creating: boolean) {
  return {
    code: draft.code, name: draft.name, category: draft.category, description: draft.description, datasourceId: draft.datasourceId,
    ...(creating ? {} : { expectedLockVersion: draft.lockVersion }), procedure: draft.procedure, result: draft.result, callTemplate: draft.callTemplate,
    parameters: draft.parameters.map((parameter) => ({ ...parameter, defaultValue: parameter.sensitive ? undefined : parameter.defaultValue, allowedValues: parameter.allowedValues.length ? parameter.allowedValues : undefined, validation: Object.keys(parameter.validation).length ? parameter.validation : undefined, normalizer: Object.keys(parameter.normalizer).length ? parameter.normalizer : undefined, valueSource: Object.keys(parameter.valueSource).length ? parameter.valueSource : undefined, nullPolicy: parameter.nullPolicy || 'TYPED_NULL' })),
    columns: draft.columns,
    grants: draft.grants,
  }
}

function serializeReportDatasource(input: ReportDatasourceInput) {
  return {
    code: input.code,
    name: input.name,
    host: input.host,
    port: input.port,
    serviceName: input.serviceName,
    sid: input.sid,
    username: input.username,
    password: input.password || undefined,
    sessionTimezone: input.sessionTimezone,
    connectTimeoutSeconds: input.connectTimeoutSeconds,
    queryTimeoutSeconds: input.queryTimeoutSeconds,
    maxOpenConnections: input.maxOpenConnections,
    maxIdleConnections: input.maxIdleConnections,
    prefetchRows: input.prefetchRows,
    arraySize: input.arraySize,
    enabled: input.enabled,
  }
}

function parsePublication(payload: unknown) {
  const data = unwrapData(payload)
  const definitionId = positiveInteger(data.definitionId)
  const versionId = positiveInteger(data.versionId)
  if (!definitionId || !versionId) throw new Error('invalid publication')
  return { definitionId, versionId, status: publicString(data.status, 32) }
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
    isOwner: value.isOwner === true,
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
