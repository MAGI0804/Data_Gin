export type PipelineRunResult = {
  runID: number
  traceID: string
  successCount: number
  failedCount: number
}

export function pipelineListPath() {
  return '/v1/pipelines'
}

export function pipelineRunPath(pipelineID: number) {
  if (!Number.isSafeInteger(pipelineID) || pipelineID < 1) throw new Error('invalid pipeline id')
  return `/v1/pipelines/${pipelineID}/run`
}

export function parsePipelineRunResult(payload: unknown): PipelineRunResult | null {
  if (!isRecord(payload) || payload.code !== 200 || !isRecord(payload.data) || !isRecord(payload.data.result)) return null
  const result = payload.data.result
  if (!positiveInteger(result.run_id) || !safeText(result.trace_id, 128) || !nonNegativeInteger(result.success_count) || !nonNegativeInteger(result.failed_count)) return null
  return { runID: result.run_id, traceID: result.trace_id, successCount: result.success_count, failedCount: result.failed_count }
}

function isRecord(value: unknown): value is Record<string, unknown> { return Boolean(value) && typeof value === 'object' && !Array.isArray(value) }
function positiveInteger(value: unknown): value is number { return typeof value === 'number' && Number.isSafeInteger(value) && value > 0 }
function nonNegativeInteger(value: unknown): value is number { return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 }
function safeText(value: unknown, maximum: number): value is string { return typeof value === 'string' && value.trim() === value && value.length > 0 && Array.from(value).length <= maximum }
