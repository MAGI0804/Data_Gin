export type ProcessedRecordsQuery = {
  page: number
  pageSize: number
  dataType?: string
  minQuality?: string
  maxQuality?: string
  createdFrom?: string
  createdTo?: string
}

export type ProcessedRecordsPage<T> = {
  list: T[]
  total: number
  page: number
  pageSize: number
  totalPages: number
  averageQuality: number
}

function boundedInteger(value: number, fallback: number, minimum: number, maximum: number) {
  return Number.isInteger(value) && value >= minimum && value <= maximum ? value : fallback
}

function optionalQuality(value: string | undefined) {
  if (!value?.trim()) return undefined
  const number = Number(value)
  return Number.isFinite(number) && number >= 0 && number <= 100 ? String(number) : undefined
}

export function buildProcessedRecordsQuery(query: ProcessedRecordsQuery) {
  const parameters = new URLSearchParams({
    page: String(boundedInteger(query.page, 1, 1, Number.MAX_SAFE_INTEGER)),
    page_size: String(boundedInteger(query.pageSize, 20, 1, 100)),
  })
  const optional = {
    data_type: query.dataType?.trim(),
    min_quality: optionalQuality(query.minQuality),
    max_quality: optionalQuality(query.maxQuality),
    created_from: query.createdFrom?.trim(),
    created_to: query.createdTo?.trim(),
  }
  for (const [key, value] of Object.entries(optional)) if (value) parameters.set(key, value)
  return parameters.toString()
}

export function parseProcessedRecordsPage<T>(payload: unknown): ProcessedRecordsPage<T> | null {
  if (!payload || typeof payload !== 'object') return null
  const data = (payload as { data?: unknown }).data
  if (!data || typeof data !== 'object') return null
  const value = data as Record<string, unknown>
  if (!Array.isArray(value.list)) return null
  const total = Number(value.total)
  const page = Number(value.page)
  const pageSize = Number(value.page_size)
  const totalPages = Number(value.total_pages)
  const summary = value.summary
  const averageQuality = summary && typeof summary === 'object' ? Number((summary as Record<string, unknown>).avg_quality) : NaN
  if (!Number.isInteger(total) || total < 0 || !Number.isInteger(page) || page < 1 || !Number.isInteger(pageSize) || pageSize < 1 || pageSize > 100 || !Number.isInteger(totalPages) || totalPages < 0 || !Number.isFinite(averageQuality)) return null
  return { list: value.list as T[], total, page, pageSize, totalPages, averageQuality }
}
