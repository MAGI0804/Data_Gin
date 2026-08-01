import assert from 'node:assert/strict'
import test from 'node:test'

import {
  isSensitiveMonitoringKey,
  parseDataStatisticsSummary,
  parseHealthSummary,
  parseMallWeatherMetricsSummary,
  redactMonitoringJSON,
  redactedMonitoringValue,
} from '../.test-dist/monitoring.js'

test('parses the data statistics controller envelope into display-only totals', () => {
  const summary = parseDataStatisticsSummary({
    code: 200,
    data: {
      data: [
        { id: 1, stat_date: '2026-07-01', data_type: 'orders', data_source_id: 3, total_count: 10, processed_count: 8, error_count: 2, avg_quality_score: 80 },
        { id: 2, stat_date: '2026-07-02', data_type: 'orders', data_source_id: 3, total_count: 30, processed_count: 30, error_count: 0, avg_quality_score: 95 },
      ],
      meta: { total: 2 },
    },
  })

  assert.deepEqual(summary, {
    totalCount: 40,
    processedCount: 38,
    errorCount: 2,
    averageQualityScore: 91.25,
    groupCount: 2,
  })
  assert.equal(parseDataStatisticsSummary({ code: 0, data: { data: [{ total_count: -1 }] } }), null)
})

test('parses protected mall weather metric aggregates without forwarding labels', () => {
  const summary = parseMallWeatherMetricsSummary({
    code: 0,
    data: {
      counters: [
        { name: 'mall_weather_fetch_total', labels: { status: 'success', token: 'must-not-leak' }, value: 8 },
        { name: 'mall_weather_fetch_total', labels: { status: 'failed' }, value: 2 },
        { name: 'mall_weather_provider_rate_limited_total', value: 3 },
        { name: 'mall_weather_provider_auth_failures_total', labels: { endpoint: 'weather' }, value: 1 },
      ],
      gauges: [
        { name: 'mall_weather_data_age_seconds', labels: { kind: 'full' }, value: 12.5 },
        { name: 'mall_weather_data_age_seconds', labels: { kind: 'daily' }, value: 18 },
        { name: 'mall_weather_queue_lag_seconds', value: 75 },
      ],
      summary: { total: 3, critical: 1, warning: 2, byStatus: [{ status: 'FIRING', count: 2 }, { status: 'RESOLVED', count: 1 }] },
    },
  })

  assert.deepEqual(summary, {
    totalAlerts: 3,
    criticalAlerts: 1,
    warningAlerts: 2,
    firingAlerts: 2,
    fetchTotal: 10,
    failedFetches: 2,
    providerRateLimited: 3,
    providerAuthFailures: 1,
    maxDataAgeSeconds: 18,
    maxQueueLagSeconds: 75,
  })
  assert.equal(parseMallWeatherMetricsSummary({ code: 0, data: { counters: [], gauges: [], summary: { total: '3', critical: 0, warning: 0 } } }), null)
})

test('accepts only the health route contract', () => {
  assert.deepEqual(parseHealthSummary({ status: 'ok', service: 'gin-biz-web-api' }), {
    healthy: true,
    status: 'ok',
    service: 'gin-biz-web-api',
  })
  assert.equal(parseHealthSummary({ code: 0, data: { status: 'ok' } }), null)
  assert.equal(parseHealthSummary({ status: 'down', service: 'gin-biz-web-api' }), null)
})

test('deeply redacts credential and header fields without mutating the source', () => {
  const source = {
    request: {
      Authorization: 'Bearer token-123',
      nested: [{ password: 'p@ss', normal: 'retained' }],
      response_headers: { 'x-request-id': 'trace', cookie: 'session' },
    },
    content: { ok: true, amount: 4 },
  }
  const redacted = redactMonitoringJSON(source)

  assert.deepEqual(redacted, {
    request: {
      Authorization: redactedMonitoringValue,
      nested: [{ password: redactedMonitoringValue, normal: 'retained' }],
      response_headers: redactedMonitoringValue,
    },
    content: { ok: true, amount: 4 },
  })
  assert.equal(source.request.Authorization, 'Bearer token-123')
  assert.equal(source.request.nested[0].password, 'p@ss')
  assert.equal(isSensitiveMonitoringKey('x_api-key'), true)
  assert.equal(isSensitiveMonitoringKey('displayName'), false)
})

test('redacts cyclic programmatic input into serializable JSON', () => {
  const source = { token: 'secret' }
  source.self = source
  const redacted = redactMonitoringJSON(source)

  assert.equal(JSON.stringify(redacted), '{"token":"[已脱敏]","self":"[循环引用]"}')
})
