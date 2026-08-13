export type SourceFetchSummary = {
  traceID: string
  totalCount: number
  successCount: number
  failedCount: number
}

export function parseSourceFetchSummary(payload: unknown): SourceFetchSummary | null {
  if (!payload || typeof payload !== 'object') return null
  const envelope = payload as { data?: unknown }
  if (!envelope.data || typeof envelope.data !== 'object') return null
  const result = (envelope.data as { result?: unknown }).result
  if (!result || typeof result !== 'object') return null
  const value = result as Record<string, unknown>
  const traceID = typeof value.trace_id === 'string' ? value.trace_id : ''
  const totalCount = value.total_count
  const successCount = value.success_count
  const failedCount = value.failed_count
  if (!traceID || typeof totalCount !== 'number' || typeof successCount !== 'number' || typeof failedCount !== 'number'
    || !Number.isSafeInteger(totalCount) || !Number.isSafeInteger(successCount) || !Number.isSafeInteger(failedCount)
    || totalCount < 0 || successCount < 0 || failedCount < 0 || successCount + failedCount > totalCount) return null
  return { traceID, totalCount, successCount, failedCount }
}
