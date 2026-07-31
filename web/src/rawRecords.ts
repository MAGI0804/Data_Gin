export type RawRecordOrigin = 'receive' | 'pull'

export type RawRecordsQuery = {
  page: number
  pageSize: number
  source?: string
  startTime?: string
  endTime?: string
  origin: RawRecordOrigin
}

export type RawRecordsPage<T> = {
  list: T[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

export type WarehouseRawRecordsQuery = RawRecordsQuery & {
  status?: string
  traceID?: string
}

const defaultPageSize = 20

function boundedInteger(value: number, fallback: number, minimum: number, maximum: number) {
  if (!Number.isInteger(value) || value < minimum || value > maximum) return fallback
  return value
}

export function buildRawRecordsRequest(query: RawRecordsQuery) {
  return {
    page: boundedInteger(query.page, 1, 1, Number.MAX_SAFE_INTEGER),
    page_size: boundedInteger(query.pageSize, defaultPageSize, 1, 100),
    source: query.source?.trim() ?? '',
    start_time: query.startTime?.trim() ?? '',
    end_time: query.endTime?.trim() ?? '',
    origin: query.origin,
  }
}

export function buildWarehouseRawRecordsQuery(query: WarehouseRawRecordsQuery) {
  const params = new URLSearchParams()
  const request = buildRawRecordsRequest(query)
  params.set('page', String(request.page))
  params.set('page_size', String(request.page_size))
  params.set('origin', request.origin)
  if (request.source) params.set('source', request.source.slice(0, 100))
  if (request.start_time) params.set('start_time', request.start_time)
  if (request.end_time) params.set('end_time', request.end_time)
  const status = query.status?.trim() ?? ''
  if (['received', 'queued', 'cleaning', 'cleaned', 'failed'].includes(status)) params.set('status', status)
  const traceID = query.traceID?.trim() ?? ''
  if (traceID) params.set('trace_id', traceID.slice(0, 64))
  return params.toString()
}

export function parseRawRecordsPage<T>(payload: unknown): RawRecordsPage<T> | null {
  if (!payload || typeof payload !== 'object') return null
  const envelope = payload as { data?: unknown }
  if (!envelope.data || typeof envelope.data !== 'object') return null
  const data = envelope.data as Record<string, unknown>
  if (!Array.isArray(data.list)) return null
  const total = Number(data.total)
  const page = Number(data.page)
  const pageSize = Number(data.page_size)
  const totalPages = Number(data.total_pages)
  if (!Number.isInteger(total) || total < 0
    || !Number.isInteger(page) || page < 1
    || !Number.isInteger(pageSize) || pageSize < 1 || pageSize > 100
    || !Number.isInteger(totalPages) || totalPages < 0) return null
  return { list: data.list as T[], total, page, pageSize, totalPages }
}
