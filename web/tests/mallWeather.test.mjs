import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clearMallWeatherPendingRefresh,
  loadMallWeatherPendingRefresh,
  mallWeatherChartSegments,
  mallWeatherFreshnessLabel,
  mallWeatherMetric,
  mallWeatherOverviewPath,
  mallWeatherRefreshKey,
  mallWeatherRefreshDisposition,
  mallWeatherRefreshPath,
  mallWeatherRefreshRequest,
  mallWeatherRefreshResultMessage,
  saveMallWeatherPendingRefresh,
  mallWeatherSkyconLabel,
  parseMallWeatherMallList,
  parseMallWeatherOverview,
  parseMallWeatherRefreshResult,
} from '../.test-dist/mallWeather.js'

class MemoryStorage {
  values = new Map()

  getItem(key) { return this.values.get(key) ?? null }
  setItem(key, value) { this.values.set(key, String(value)) }
  removeItem(key) { this.values.delete(key) }
}

test('parses valid malls and preserves the list cursor', () => {
  const result = parseMallWeatherMallList({
    code: 0,
    data: {
      nextAfterId: 7,
      items: [
        { id: 7, mallCode: 'SH-001', nameCn: '示例商场', city: '上海', weatherEnabled: true, status: 'active' },
      ],
    },
  })

  assert.deepEqual(result, {
    nextAfterId: 7,
    items: [{
      id: 7,
      mallCode: 'SH-001',
      nameCn: '示例商场',
      city: '上海',
      address: '',
      geocodeStatus: '',
      weatherEnabled: true,
      status: 'active',
    }],
  })
  assert.equal(parseMallWeatherMallList({ code: 0, data: { items: [{ id: 0, nameCn: '无效商场' }] } }), null)
  assert.equal(parseMallWeatherMallList({ code: 0, data: { items: [], nextAfterId: 'bad' } }), null)
})

test('rejects unsuccessful or structurally incomplete overview payloads', () => {
  assert.equal(parseMallWeatherOverview({ code: 500, data: {} }), null)
  assert.equal(parseMallWeatherOverview({ code: 0, data: { meta: {}, minutely: [], hourly: [] } }), null)
})

test('keeps overview datasets and normalizes a missing realtime snapshot', () => {
  const overview = parseMallWeatherOverview({
    code: 0,
    data: {
      meta: { provider: 'caiyun', freshnessStatus: 'FRESH' },
      minutely: [{ minuteOffset: 0, precipitationMmH: 0.2 }],
      hourly: [{ forecastTimeLocal: '2026-07-26 10:00:00', temperatureC: 31 }],
      alerts: [],
    },
  })

  assert.equal(overview?.realtime, null)
  assert.equal(overview?.minutely[0].precipitationMmH, 0.2)
  assert.equal(overview?.hourly[0].temperatureC, 31)
})

test('builds encoded weather overview paths and rejects invalid mall ids', () => {
  assert.equal(mallWeatherOverviewPath(7, 'Asia/Shanghai'), '/v1/malls/7/weather/overview?timeZone=Asia%2FShanghai')
  assert.throws(() => mallWeatherOverviewPath(0), /invalid mall id/)
})

test('builds validated manual refresh requests and idempotency keys', () => {
  assert.equal(mallWeatherRefreshPath(7), '/v1/malls/7/weather-refresh')
  assert.deepEqual(mallWeatherRefreshRequest(['V3_LIFE_INDEX', 'V26_FULL', 'V26_FULL'], ' 管理端补采 '), {
    kinds: ['V26_FULL', 'V3_LIFE_INDEX'],
    force: false,
    reason: '管理端补采',
  })
  assert.equal(mallWeatherRefreshKey('12345678-abcd'), 'weather-refresh:12345678-abcd')
  assert.throws(() => mallWeatherRefreshKey('bad key'), /invalid refresh key/)
  assert.throws(() => mallWeatherRefreshRequest([], '补采'), /invalid refresh kinds/)
  assert.throws(() => mallWeatherRefreshRequest(['V26_FULL'], 'bad\nreason'), /invalid refresh reason/)
  assert.doesNotThrow(() => mallWeatherRefreshRequest(['V26_FULL'], '😀'.repeat(500)))
})

test('persists pending refreshes by actor and mall without cross-account leakage', () => {
  const storage = new MemoryStorage()
  const pending = {
    key: mallWeatherRefreshKey('12345678-abcd'),
    body: mallWeatherRefreshRequest(['V26_FULL'], '网络结果未知'),
  }
  const otherPending = {
    key: mallWeatherRefreshKey('87654321-dcba'),
    body: mallWeatherRefreshRequest(['V3_LIFE_INDEX'], '另一账号补采'),
  }

  saveMallWeatherPendingRefresh('42', 7, pending, storage)
  assert.deepEqual(loadMallWeatherPendingRefresh('42', 7, storage), pending)
  assert.equal(loadMallWeatherPendingRefresh('43', 7, storage), null)
  assert.equal(loadMallWeatherPendingRefresh('42', 8, storage), null)
  saveMallWeatherPendingRefresh('43', 7, otherPending, storage)
  assert.deepEqual(loadMallWeatherPendingRefresh('43', 7, storage), otherPending)
  assert.deepEqual(loadMallWeatherPendingRefresh('42', 7, storage), pending)
  clearMallWeatherPendingRefresh('43', 7, storage)
  assert.equal(loadMallWeatherPendingRefresh('43', 7, storage), null)
  assert.deepEqual(loadMallWeatherPendingRefresh('42', 7, storage), pending)
  assert.throws(() => saveMallWeatherPendingRefresh('invalid', 7, pending, storage), /invalid actor id/)
})

test('parses queued and fresh-skipped refresh results without over-reporting queued work', () => {
  const result = parseMallWeatherRefreshResult({ code: 0, data: {
    jobId: 31,
    mallId: 7,
    kinds: [
      { kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 },
      { kind: 'V3_LIFE_INDEX', status: 'SKIPPED_FRESH' },
    ],
  } })
  assert.equal(mallWeatherRefreshResultMessage(result), '1 个采集任务已入队，1 项数据仍新鲜并已跳过。')

  const skipped = parseMallWeatherRefreshResult({ code: 0, data: {
    jobId: 32,
    mallId: 7,
    kinds: [{ kind: 'V26_FULL', status: 'SKIPPED_FRESH' }],
  } })
  assert.equal(mallWeatherRefreshResultMessage(skipped), '1 项数据仍新鲜，本次未重复入队。')
  assert.equal(parseMallWeatherRefreshResult({ code: 0, data: { jobId: 33, mallId: 7, kinds: [{ kind: 'V26_FULL', status: 'QUEUED' }] } }), null)
  assert.equal(parseMallWeatherRefreshResult({ code: 0, data: {
    jobId: 34,
    mallId: 7,
    kinds: [
      { kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 },
      { kind: 'V26_FULL', status: 'SKIPPED_FRESH' },
    ],
  } }), null)
  assert.equal(parseMallWeatherRefreshResult({ code: 0, data: {
    jobId: 35,
    mallId: 7,
    kinds: [{ kind: 'V3_LIFE_INDEX', status: 'SKIPPED_FRESH', outboxJobId: 42 }],
  } }), null)
})

test('keeps the original idempotent request for every uncertain refresh outcome', () => {
  const request = mallWeatherRefreshRequest(['V26_FULL', 'V3_LIFE_INDEX'], '管理端补采')
  const acceptedData = { code: 0, data: {
    jobId: 31,
    mallId: 7,
    kinds: [
      { kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 },
      { kind: 'V3_LIFE_INDEX', status: 'SKIPPED_FRESH' },
    ],
  } }

  assert.equal(mallWeatherRefreshDisposition({ ok: true, status: 202, data: acceptedData }, 7, request).kind, 'accepted')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 202, data: {} }, 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 0, data: {} }, 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 408, data: {} }, 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 409, data: {} }, 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 503, data: {} }, 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 403, data: {} }, 7, request).kind, 'rejected')
  assert.equal(mallWeatherRefreshDisposition({ ok: true, status: 200, data: acceptedData }, 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: true, status: 202, data: { code: 0, data: {
    jobId: 31,
    mallId: 7,
    kinds: [{ kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 }],
  } } }, 7, request).kind, 'uncertain')
})

test('formats weather statuses, conditions, metrics, and chart points', () => {
  assert.equal(mallWeatherFreshnessLabel('stale'), '数据已过期')
  assert.equal(mallWeatherFreshnessLabel('critical'), '数据严重过期')
  assert.equal(mallWeatherSkyconLabel('HEAVY_RAIN'), '大雨')
  assert.equal(mallWeatherSkyconLabel('NEW_CONDITION'), 'NEW_CONDITION')
  assert.equal(mallWeatherMetric(31.25, '°C'), '31.3°C')
  assert.equal(mallWeatherMetric(Number.NaN, '°C'), '—')
  assert.deepEqual(mallWeatherChartSegments([1, undefined, 3, 2], 100, 40), ['0.0,40.0', '66.7,0.0 100.0,20.0'])
  assert.deepEqual(mallWeatherChartSegments([], 100, 40), [])
})
