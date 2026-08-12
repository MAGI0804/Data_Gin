import type { ClientResponse, HTTPMethod } from '../api/client'
import type { ReportCatalogPage, ReportCatalogQuery, ReportDefinitionStatus, ReportSummary } from './types'

type JsonRecord = Record<string, unknown>

export type ReportCenterClient = (path: string, options?: {
  method?: HTTPMethod
  signal?: AbortSignal
  showResult?: boolean
  silentLoading?: boolean
}) => Promise<ClientResponse>

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

function publicDate(value: unknown) {
  if (typeof value !== 'string' || !value.trim()) return null
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? null : value
}

function reportStatus(value: unknown): ReportDefinitionStatus {
  return value === 'ACTIVE' || value === 'DISABLED' ? value : 'DRAFT'
}
