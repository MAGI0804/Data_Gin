import assert from 'node:assert/strict'
import test from 'node:test'

import {
  mallWeatherChartSegments,
  mallWeatherFreshnessLabel,
  mallWeatherMetric,
  mallWeatherOverviewPath,
  mallWeatherSkyconLabel,
  parseMallWeatherMallList,
  parseMallWeatherOverview,
} from '../.test-dist/mallWeather.js'

test('parses valid malls, ignores malformed rows, and preserves the list cursor', () => {
  const result = parseMallWeatherMallList({
    code: 0,
    data: {
      nextAfterId: 7,
      items: [
        { id: 7, mallCode: 'SH-001', nameCn: '示例商场', city: '上海', weatherEnabled: true, status: 'active' },
        { id: 0, nameCn: '无效商场' },
        { id: 8 },
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
