type JsonRecord = Record<string, unknown>

export type DataStatisticsSummary = {
  totalCount: number
  processedCount: number
  errorCount: number
  averageQualityScore: number | null
  groupCount: number
}

export type MallWeatherMetricsSummary = {
  totalAlerts: number
  criticalAlerts: number
  warningAlerts: number
  firingAlerts: number
  fetchTotal: number
  failedFetches: number
  providerRateLimited: number
  providerAuthFailures: number
  maxDataAgeSeconds: number | null
  maxQueueLagSeconds: number | null
}

export type HealthSummary = {
  healthy: boolean
  status: 'ok'
  service: string
}

export const redactedMonitoringValue = '[已脱敏]'

const sensitiveKeyMarkers = [
  'token',
  'password',
  'passwd',
  'secret',
  'authorization',
  'header',
  'cookie',
  'signature',
  'apikey',
  'credential',
  'privatekey',
  'accesskey',
]

function isRecord(value: unknown): value is JsonRecord {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function nonNegativeFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
}

function nonNegativeSafeInteger(value: unknown): value is number {
  return nonNegativeFiniteNumber(value) && Number.isSafeInteger(value)
}

function monitoringEnvelopeData(payload: unknown, successCode: number): JsonRecord | null {
  if (!isRecord(payload) || payload.code !== successCode || !isRecord(payload.data)) return null
  return payload.data
}

function metricCounterTotal(counters: unknown[], name: string, status?: string) {
  return counters.reduce<number>((total, counter) => {
    if (!isRecord(counter) || counter.name !== name || !nonNegativeSafeInteger(counter.value)) return total
    const labels = counter.labels
    if (status !== undefined && (!isRecord(labels) || labels.status !== status)) return total
    return total + counter.value
  }, 0)
}

function maximumGauge(gauges: unknown[], name: string) {
  let maximum: number | null = null
  for (const gauge of gauges) {
    if (!isRecord(gauge) || gauge.name !== name || !nonNegativeFiniteNumber(gauge.value)) continue
    maximum = maximum === null ? gauge.value : Math.max(maximum, gauge.value)
  }
  return maximum
}

/**
 * Parses the legacy statistics response without passing source records to the UI.
 * The controller returns code: 200 and nests the actual rows in data.data.
 */
export function parseDataStatisticsSummary(payload: unknown): DataStatisticsSummary | null {
  const data = monitoringEnvelopeData(payload, 200)
  if (!data || !Array.isArray(data.data)) return null

  let totalCount = 0
  let processedCount = 0
  let errorCount = 0
  let qualityWeight = 0
  let weightedQualityTotal = 0

  for (const item of data.data) {
    if (!isRecord(item) || !nonNegativeSafeInteger(item.total_count) ||
      !nonNegativeSafeInteger(item.processed_count) || !nonNegativeSafeInteger(item.error_count) ||
      !nonNegativeFiniteNumber(item.avg_quality_score)) return null

    totalCount += item.total_count
    processedCount += item.processed_count
    errorCount += item.error_count
    if (!Number.isSafeInteger(totalCount) || !Number.isSafeInteger(processedCount) || !Number.isSafeInteger(errorCount)) return null
    if (item.total_count > 0) {
      qualityWeight += item.total_count
      weightedQualityTotal += item.avg_quality_score * item.total_count
      if (!Number.isSafeInteger(qualityWeight) || !Number.isFinite(weightedQualityTotal)) return null
    }
  }

  return {
    totalCount,
    processedCount,
    errorCount,
    averageQualityScore: qualityWeight > 0 ? weightedQualityTotal / qualityWeight : null,
    groupCount: data.data.length,
  }
}

/**
 * Extracts only aggregate operational values from the protected weather metrics
 * endpoint. Metric labels and definitions are intentionally not exposed here.
 */
export function parseMallWeatherMetricsSummary(payload: unknown): MallWeatherMetricsSummary | null {
  const data = monitoringEnvelopeData(payload, 0)
  if (!data || !Array.isArray(data.counters) || !Array.isArray(data.gauges) || !isRecord(data.summary) ||
    typeof data.summary.total !== 'number' || !nonNegativeSafeInteger(data.summary.total) ||
    typeof data.summary.critical !== 'number' || !nonNegativeSafeInteger(data.summary.critical) ||
    typeof data.summary.warning !== 'number' || !nonNegativeSafeInteger(data.summary.warning)) return null

  const firingAlerts = Array.isArray(data.summary.byStatus)
    ? data.summary.byStatus.reduce<number>((total, item) =>
      isRecord(item) && item.status === 'FIRING' && nonNegativeSafeInteger(item.count) ? total + item.count : total, 0)
    : 0

  return {
    totalAlerts: data.summary.total,
    criticalAlerts: data.summary.critical,
    warningAlerts: data.summary.warning,
    firingAlerts,
    fetchTotal: metricCounterTotal(data.counters, 'mall_weather_fetch_total'),
    failedFetches: metricCounterTotal(data.counters, 'mall_weather_fetch_total', 'failed'),
    providerRateLimited: metricCounterTotal(data.counters, 'mall_weather_provider_rate_limited_total'),
    providerAuthFailures: metricCounterTotal(data.counters, 'mall_weather_provider_auth_failures_total'),
    maxDataAgeSeconds: maximumGauge(data.gauges, 'mall_weather_data_age_seconds'),
    maxQueueLagSeconds: maximumGauge(data.gauges, 'mall_weather_queue_lag_seconds'),
  }
}

/** Parses the unwrapped health route, which only reports an expected static service. */
export function parseHealthSummary(payload: unknown): HealthSummary | null {
  if (!isRecord(payload) || payload.status !== 'ok' || payload.service !== 'gin-biz-web-api') return null
  return { healthy: true, status: 'ok', service: payload.service }
}

function normalizedKey(key: string) {
  return key.toLowerCase().replace(/[^a-z0-9]/g, '')
}

export function isSensitiveMonitoringKey(key: string) {
  const normalized = normalizedKey(key)
  return sensitiveKeyMarkers.some((marker) => normalized.includes(marker))
}

/**
 * Creates a JSON-safe deep copy for log and detail viewers. Sensitive values are
 * replaced before rendering, and cyclic programmatic input is made serializable.
 */
export function redactMonitoringJSON(value: unknown): unknown {
  const ancestors = new WeakSet<object>()

  const visit = (candidate: unknown): unknown => {
    if (candidate === null || typeof candidate === 'string' || typeof candidate === 'boolean') return candidate
    if (typeof candidate === 'number') return Number.isFinite(candidate) ? candidate : null
    if (typeof candidate !== 'object') return null
    if (ancestors.has(candidate)) return '[循环引用]'

    ancestors.add(candidate)
    try {
      if (Array.isArray(candidate)) return candidate.map(visit)
      const sanitized: JsonRecord = {}
      for (const [key, item] of Object.entries(candidate)) {
        sanitized[key] = isSensitiveMonitoringKey(key) ? redactedMonitoringValue : visit(item)
      }
      return sanitized
    } finally {
      ancestors.delete(candidate)
    }
  }

  return visit(value)
}
