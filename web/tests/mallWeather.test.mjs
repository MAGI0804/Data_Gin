import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clearMallWeatherPendingRefresh,
  clearMallWeatherPendingCreate,
  clearMallWeatherPendingSheetPush,
  loadAllMallWeatherPages,
  loadMallWeatherPendingCreate,
  loadMallWeatherPendingRefresh,
  loadMallWeatherPendingSheetPush,
  mallWeatherChartSegments,
  mallWeatherCreateKey,
  mallWeatherCreateRequest,
  mallWeatherForecastQueryWindows,
  mallWeatherFreshnessLabel,
  mallWeatherMetric,
  mallWeatherOverviewPath,
  mallWeatherGeocodeCandidatesPath,
  mallWeatherGeocodeConfirmPath,
  mallWeatherGeocodeRunTerminal,
  mallWeatherGeocodeTriggerPath,
  mallWeatherMallReady,
  mallWeatherSheetPushKey,
  mallWeatherSheetPushRequest,
  mallWeatherSheetPushRequestMatchesOption,
  mallWeatherSheetPushResultMatchesRequest,
  mergeMallWeatherMalls,
  mallWeatherRefreshKey,
  mallWeatherRefreshDisposition,
  mallWeatherRefreshPath,
  mallWeatherRefreshRequest,
  mallWeatherRefreshResultMessage,
  mallWeatherSeriesPath,
  saveMallWeatherPendingRefresh,
  saveMallWeatherPendingCreate,
  saveMallWeatherPendingSheetPush,
  mallWeatherSkyconLabel,
  parseMallWeatherMallList,
  parseMallWeatherCreateResult,
  parseMallWeatherGeocodeCandidates,
  parseMallWeatherSheetPushDryRun,
  parseMallWeatherSheetPushOptions,
  parseMallWeatherSheetPushResult,
  parseMallWeatherDailyPage,
  parseMallWeatherHourlyPage,
  parseMallWeatherLifeIndexPage,
  parseMallWeatherOverview,
  parseMallWeatherRefreshResult,
} from '../.test-dist/mallWeather.js'

class MemoryStorage {
  values = new Map()

  getItem(key) { return this.values.get(key) ?? null }
  setItem(key, value) { this.values.set(key, String(value)) }
  removeItem(key) { this.values.delete(key) }
}

const completeWeatherMeta = {
  provider: 'CAIYUN',
  apiVersion: 'v2.6',
  representativePoint: 'mall_center',
  longitude: 121.47,
  latitude: 31.23,
  coordinateSystem: 'WGS84',
  samplingMode: 'point',
  coverageRadiusM: 1000,
  spatialResolution: '9-13km',
  timeZone: 'Asia/Shanghai',
  unit: 'metric:v2',
  freshnessStatus: 'FRESH',
}

const completeWeatherTimes = {
  issuedAtUtc: '2026-07-22T00:00:00Z',
  issuedAtLocal: '2026-07-22T08:00:00+08:00',
  fetchedAtUtc: '2026-07-22T00:01:00Z',
  fetchedAtLocal: '2026-07-22T08:01:00+08:00',
}

const pageEnvelope = (items, nextCursor = '', meta = completeWeatherMeta) => ({ code: 0, data: {
  items,
  meta,
  pagination: { pageSize: 200, ...(nextCursor ? { nextCursor } : {}) },
} })

const hourlyItem = (hour, temperatureC = hour) => ({
  forecastTimeUtc: `2026-07-22T${String(hour).padStart(2, '0')}:00:00Z`,
  forecastTimeLocal: `2026-07-22T${String((hour + 8) % 24).padStart(2, '0')}:00:00+08:00`,
  ...completeWeatherTimes,
  temperatureC,
  qualityStatus: 'VALID',
  qualityWarnings: [],
})

test('parses valid malls and preserves the list cursor', () => {
  const result = parseMallWeatherMallList({
    code: 0,
    data: {
      nextAfterId: 7,
      items: [
        { id: 7, version: 3, mallCode: 'SH-001', nameCn: '示例商场', province: '上海市', city: '上海', district: '黄浦区', address: '示例路 1 号', longitude: 121.47, latitude: 31.23, coordinateSystem: 'GCJ02', geocodeStatus: 'confirmed', weatherEnabled: true, weatherProvider: 'caiyun', detailProfile: 'full', coverageRadiusM: 1000, status: 'active' },
      ],
    },
  })

  assert.deepEqual(result, {
    nextAfterId: 7,
    items: [{
      id: 7,
      mallCode: 'SH-001',
      nameCn: '示例商场',
      province: '上海市',
      city: '上海',
      district: '黄浦区',
      address: '示例路 1 号',
      longitude: 121.47,
      latitude: 31.23,
      coordinateSystem: 'GCJ02',
      geocodeStatus: 'confirmed',
      weatherEnabled: true,
      detailProfile: 'full',
      coverageRadiusM: 1000,
      status: 'active',
      version: 3,
    }],
  })
  assert.equal(parseMallWeatherMallList({ code: 0, data: { items: [{ id: 0, version: 1, mallCode: 'BAD', nameCn: '无效商场' }] } }), null)
  assert.equal(parseMallWeatherMallList({ code: 0, data: { items: [], nextAfterId: 'bad' } }), null)
})

test('builds a normalized mall onboarding request and stable operation paths', () => {
  assert.deepEqual(mallWeatherCreateRequest({
    mallCode: ' sh-002 ', nameCn: ' 新商场 ', province: ' 上海市 ', city: ' 上海市 ', district: ' 浦东新区 ', address: ' 世纪大道 1 号 ',
  }), {
    mallCode: 'SH-002', nameCn: '新商场', province: '上海市', city: '上海市', district: '浦东新区', address: '世纪大道 1 号',
    weather: { detailProfile: 'full', coverageRadiusM: 1000 },
  })
  assert.equal(mallWeatherCreateKey('12345678-abcd'), 'mall-create:12345678-abcd')
  assert.equal(mallWeatherGeocodeCandidatesPath(7), '/v1/malls/7/geocode-candidates')
  assert.equal(mallWeatherGeocodeTriggerPath(7), '/v1/malls/7/geocode')
  assert.equal(mallWeatherGeocodeConfirmPath(7), '/v1/malls/7/geocode-confirm')
  assert.throws(() => mallWeatherCreateRequest({ mallCode: '!', nameCn: '', province: '', city: '', district: '', address: '' }), /invalid mall create request/)
  assert.throws(() => mallWeatherGeocodeConfirmPath(0), /invalid mall id/)
})

test('parses mall creation and geocode candidate contracts', () => {
  assert.deepEqual(parseMallWeatherCreateResult({ code: 0, data: {
    id: 9, mallCode: 'SH-002', status: 'DRAFT', geocodeStatus: 'PENDING', weatherStatus: 'WAITING_FOR_COORDINATE', version: 1,
  } }), {
    id: 9, mallCode: 'SH-002', status: 'DRAFT', geocodeStatus: 'PENDING', weatherStatus: 'WAITING_FOR_COORDINATE', version: 1,
  })
  const candidates = parseMallWeatherGeocodeCandidates({ code: 0, data: {
    mallId: 9, mallVersion: 2, runId: 4, runStatus: 'SUCCEEDED', items: [{
      id: 11, candidateNo: 1, formattedAddress: '上海市浦东新区世纪大道 1 号', province: '上海市', city: '上海市', district: '浦东新区',
      longitude: 121.5, latitude: 31.2, coordinateSystem: 'GCJ02', level: '门牌号', confidenceScore: 0.96, selected: false,
    }],
  } })
  assert.equal(candidates?.items[0].formattedAddress, '上海市浦东新区世纪大道 1 号')
  assert.equal(candidates?.mallVersion, 2)
  assert.equal(parseMallWeatherGeocodeCandidates({ code: 0, data: { mallId: 9, mallVersion: 2, items: [{ id: 1, candidateNo: 1, formattedAddress: 'bad', longitude: 200, latitude: 31, coordinateSystem: 'GCJ02', confidenceScore: 1 }] } }), null)
  for (const status of ['NO_CANDIDATES', 'AUTO_CONFIRMED', 'REVIEW_REQUIRED', 'FAILED', 'STALE']) assert.equal(mallWeatherGeocodeRunTerminal(status), true)
  assert.equal(mallWeatherGeocodeRunTerminal('RUNNING'), false)
})

test('recognizes only active confirmed weather-enabled malls as queryable', () => {
  const mall = parseMallWeatherMallList({ code: 0, data: { items: [{
    id: 7, version: 1, mallCode: 'SH-001', nameCn: '示例商场', province: '上海市', city: '上海市', address: '示例路 1 号',
    longitude: 121.47, latitude: 31.23, coordinateSystem: 'GCJ02', geocodeStatus: 'confirmed', weatherEnabled: true,
    weatherProvider: 'caiyun', detailProfile: 'full', coverageRadiusM: 1000, status: 'active',
  }] } })?.items[0]
  assert.equal(mallWeatherMallReady(mall), true)
  assert.equal(mallWeatherMallReady({ ...mall, weatherEnabled: false }), false)
  assert.equal(mallWeatherMallReady({ ...mall, longitude: undefined }), false)
  assert.equal(parseMallWeatherMallList({ code: 0, data: { items: [{ ...mall, weatherProvider: undefined }] } }), null)
  const newer = { ...mall, version: 2, weatherEnabled: true }
  const older = { ...mall, version: 1, weatherEnabled: false }
  assert.deepEqual(mergeMallWeatherMalls([newer], [older]), [newer])
  assert.deepEqual(mergeMallWeatherMalls([older], [newer]), [newer])
})

test('persists uncertain mall creation requests per actor', () => {
  const storage = new MemoryStorage()
  const body = mallWeatherCreateRequest({ mallCode: 'SH-002', nameCn: '新商场', province: '上海市', city: '上海市', district: '', address: '世纪大道 1 号' })
  const pending = { key: mallWeatherCreateKey('12345678-abcd'), body }
  saveMallWeatherPendingCreate('42', pending, storage)
  assert.deepEqual(loadMallWeatherPendingCreate('42', storage), pending)
  assert.equal(loadMallWeatherPendingCreate('43', storage), null)
  clearMallWeatherPendingCreate('42', storage)
  assert.equal(loadMallWeatherPendingCreate('42', storage), null)
  assert.throws(() => saveMallWeatherPendingCreate('invalid', pending, storage), /invalid actor id/)
})

test('builds, parses, and persists a mall-scoped existing-target push', () => {
  const options = parseMallWeatherSheetPushOptions({ code: 0, data: { items: [{
    destinationId: 8, name: '天气数据表', code: 'weather_sheet', profileId: 9, profileCode: 'mall_weather_full', profileVersion: 3,
  }] } })
  assert.equal(options?.length, 1)
  const body = mallWeatherSheetPushRequest(options[0], 7)
  assert.deepEqual(body, { destinationId: 8, profileId: 9, expectedProfileVersion: 3, filters: { mallIds: [7] } })
  const dryRun = parseMallWeatherSheetPushDryRun({ code: 0, data: {
    destinationId: 8, destinationCode: 'weather_sheet', profileId: 9, profileCode: 'mall_weather_full', profileVersion: 3,
    writeMode: 'APPEND', totalEstimatedRows: 42, totalEstimatedCells: 420, canExecute: false, warnings: ['PROFILE_WARNING'],
    spreadsheetTokenEnv: 'SECRET_TOKEN_ENV',
    datasets: [{
      datasetKind: 'hourly', estimatedRows: 42, estimatedCells: 420, canExecute: false,
      warnings: ['HEADER_MISMATCH_REWRITE_DISABLED'], sheetId: 'secret-sheet-id',
    }],
  } })
  assert.equal(dryRun?.totalEstimatedRows, 42)
  assert.deepEqual(dryRun?.datasets, [{
    datasetKind: 'hourly', estimatedRows: 42, estimatedCells: 420, canExecute: false, warnings: ['HEADER_MISMATCH_REWRITE_DISABLED'],
  }])
  assert.equal(JSON.stringify(dryRun).includes('SECRET_TOKEN_ENV'), false)
  assert.equal(JSON.stringify(dryRun).includes('secret-sheet-id'), false)
  const result = parseMallWeatherSheetPushResult({ code: 0, data: {
    runId: 31, traceId: 'trace-31', status: 'PENDING', destinationId: 8, profileId: 9, profileVersion: 3, estimatedRows: 42,
  } })
  assert.equal(result?.runId, 31)
  assert.equal(mallWeatherSheetPushRequestMatchesOption(body, options[0], 7), true)
  assert.equal(mallWeatherSheetPushRequestMatchesOption(body, { ...options[0], profileVersion: 4 }, 7), false)
  assert.equal(mallWeatherSheetPushRequestMatchesOption(body, options[0], 8), false)
  assert.equal(mallWeatherSheetPushResultMatchesRequest(result, body), true)
  assert.equal(mallWeatherSheetPushResultMatchesRequest({ ...result, destinationId: 10 }, body), false)

  const storage = new MemoryStorage()
  const pending = { key: mallWeatherSheetPushKey('12345678-abcd'), body }
  saveMallWeatherPendingSheetPush('42', 7, pending, storage)
  assert.deepEqual(loadMallWeatherPendingSheetPush('42', 7, storage), pending)
  assert.equal(loadMallWeatherPendingSheetPush('42', 8, storage), null)
  clearMallWeatherPendingSheetPush('42', 7, storage)
  assert.equal(loadMallWeatherPendingSheetPush('42', 7, storage), null)
  assert.equal(parseMallWeatherSheetPushOptions({ code: 0, data: { items: [{ destinationId: 8, name: 'bad' }] } }), null)
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
  assert.equal(mallWeatherOverviewPath(7), '/v1/malls/7/weather/overview')
  assert.equal(mallWeatherOverviewPath(7, 'Asia/Shanghai'), '/v1/malls/7/weather/overview?timeZone=Asia%2FShanghai')
  assert.throws(() => mallWeatherOverviewPath(0), /invalid mall id/)
})

test('builds bounded complete-series paths and parses paged forecast contracts', () => {
  const start = new Date('2026-07-22T00:00:00.000Z')
  const end = new Date('2026-08-06T00:00:00.000Z')
  const asOf = new Date('2026-07-22T00:02:00.000Z')
  const url = new URL(mallWeatherSeriesPath(7, 'hourly', start, end, 'next-page', 'Asia/Shanghai', asOf), 'https://example.test')
  assert.equal(url.pathname, '/v1/malls/7/weather/hourly')
  assert.equal(url.searchParams.get('start'), start.toISOString())
  assert.equal(url.searchParams.get('end'), end.toISOString())
  assert.equal(url.searchParams.get('latest'), 'true')
  assert.equal(url.searchParams.get('pageSize'), '200')
  assert.equal(url.searchParams.get('timeZone'), 'Asia/Shanghai')
  assert.equal(url.searchParams.get('asOf'), asOf.toISOString())
  assert.equal(url.searchParams.get('cursor'), 'next-page')
  assert.throws(() => mallWeatherSeriesPath(7, 'daily', start, new Date('2026-09-01T00:00:00Z')), /invalid weather range/)

  const hourly = parseMallWeatherHourlyPage(pageEnvelope([hourlyItem(1, 30)], 'hourly-next'))
  assert.equal(hourly?.items[0].temperatureC, 30)
  assert.equal(hourly?.pagination.nextCursor, 'hourly-next')
  const daily = parseMallWeatherDailyPage(pageEnvelope([{ forecastDateLocal: '2026-07-22', ...completeWeatherTimes, temperatureMinC: 24, temperatureMaxC: 32, daySkycon: 'CLEAR_DAY', qualityStatus: 'VALID', qualityWarnings: [] }]))
  assert.equal(daily?.items[0].temperatureMaxC, 32)
  const life = parseMallWeatherLifeIndexPage(pageEnvelope([{ sourceApi: 'v3_lifeindex', forecastDateLocal: '2026-07-22', indexType: 1, indexCode: 'comfort', isUnknownType: false, ...completeWeatherTimes, qualityStatus: 'VALID', qualityWarnings: [] }]))
  assert.equal(life?.items[0].indexCode, 'comfort')
  assert.equal(parseMallWeatherHourlyPage(pageEnvelope([{ ...hourlyItem(2), forecastTimeLocal: 'bad' }])), null)
  assert.equal(parseMallWeatherHourlyPage(pageEnvelope([{ ...hourlyItem(2), qualityWarnings: [{ code: 7, path: '' }] }])), null)
  assert.equal(parseMallWeatherDailyPage(pageEnvelope([{ forecastDateLocal: '2026-02-30', ...completeWeatherTimes, qualityStatus: 'VALID', qualityWarnings: [] }])), null)
  assert.equal(parseMallWeatherLifeIndexPage(pageEnvelope([{ sourceApi: 'v3_lifeindex', forecastDateLocal: '2026-07-22', indexType: 1.5, indexCode: 'bad', isUnknownType: false, ...completeWeatherTimes, qualityStatus: 'VALID', qualityWarnings: [] }])), null)
  assert.equal(parseMallWeatherLifeIndexPage(pageEnvelope([], '', { ...completeWeatherMeta, timeZone: '' })), null)
})

test('builds an exact 360-hour window and 15 target-time-zone calendar days', () => {
  const shanghai = mallWeatherForecastQueryWindows(new Date('2026-07-22T02:34:56.000Z'), 'Asia/Shanghai')
  assert.equal(shanghai.hourly.start.toISOString(), '2026-07-22T02:00:00.000Z')
  assert.equal(shanghai.hourly.end.toISOString(), '2026-08-06T02:00:00.000Z')
  assert.equal(shanghai.daily.start.toISOString(), '2026-07-21T16:00:00.000Z')
  assert.equal(shanghai.daily.end.toISOString(), '2026-08-05T16:00:00.000Z')

  const newYorkAcrossDST = mallWeatherForecastQueryWindows(new Date('2026-03-07T17:00:00.000Z'), 'America/New_York')
  assert.equal(newYorkAcrossDST.daily.start.toISOString(), '2026-03-07T05:00:00.000Z')
  assert.equal(newYorkAcrossDST.daily.end.toISOString(), '2026-03-22T04:00:00.000Z')
  assert.throws(() => mallWeatherForecastQueryWindows(new Date('invalid')), /invalid weather query time/)
  assert.throws(() => mallWeatherForecastQueryWindows(new Date(), 'Not/A_Time_Zone'), /time zone/i)
})

test('loads every opaque cursor page without changing the original query window', async () => {
  const window = {
    start: new Date('2026-07-22T02:00:00.000Z'),
    end: new Date('2026-08-06T02:00:00.000Z'),
  }
  const asOf = new Date('2026-07-22T00:02:00.000Z')
  const envelope = (hour, nextCursor = '') => pageEnvelope([hourlyItem(hour)], nextCursor)
  const paths = []
  const responses = [envelope(10, 'opaque+/=cursor'), envelope(11)]
  const result = await loadAllMallWeatherPages(async (path) => {
    paths.push(path)
    return { ok: true, status: 200, data: responses.shift() }
  }, 7, 'hourly', window, 'Asia/Shanghai', asOf, parseMallWeatherHourlyPage)

  assert.deepEqual(result.items.map((item) => item.temperatureC), [10, 11])
  assert.equal(paths.length, 2)
  const first = new URL(paths[0], 'https://example.test')
  const second = new URL(paths[1], 'https://example.test')
  for (const parameter of ['start', 'end', 'timeZone', 'latest', 'asOf', 'pageSize']) {
    assert.equal(second.searchParams.get(parameter), first.searchParams.get(parameter))
  }
  assert.equal(first.searchParams.has('cursor'), false)
  assert.equal(second.searchParams.get('cursor'), 'opaque+/=cursor')

  let repeatedCalls = 0
  await assert.rejects(loadAllMallWeatherPages(async () => {
    repeatedCalls++
    return { ok: true, status: 200, data: envelope(11 + repeatedCalls, 'same-cursor') }
  }, 7, 'hourly', window, 'Asia/Shanghai', asOf, parseMallWeatherHourlyPage), /分页游标重复/)
  assert.equal(repeatedCalls, 2)

  let duplicateCalls = 0
  await assert.rejects(loadAllMallWeatherPages(async () => {
    duplicateCalls++
    return { ok: true, status: 200, data: envelope(14, duplicateCalls === 1 ? 'next' : '') }
  }, 7, 'hourly', window, 'Asia/Shanghai', asOf, parseMallWeatherHourlyPage), /分页数据重复/)
  assert.equal(duplicateCalls, 2)

  let excessiveCalls = 0
  await assert.rejects(loadAllMallWeatherPages(async () => {
    excessiveCalls++
    return { ok: true, status: 200, data: envelope(excessiveCalls, `cursor-${excessiveCalls}`) }
  }, 7, 'hourly', window, 'Asia/Shanghai', asOf, parseMallWeatherHourlyPage), /分页数量超过安全上限/)
  assert.equal(excessiveCalls, 10)
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
    force: false,
    reason: '管理端补采',
    requestedBy: 42,
    kinds: [
      { kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 },
      { kind: 'V3_LIFE_INDEX', status: 'SKIPPED_FRESH' },
    ],
  } })
  assert.equal(mallWeatherRefreshResultMessage(result), '1 个采集任务已入队，1 项数据仍新鲜并已跳过。')

  const skipped = parseMallWeatherRefreshResult({ code: 0, data: {
    jobId: 32,
    mallId: 7,
    force: false,
    reason: '管理端补采',
    requestedBy: 42,
    kinds: [{ kind: 'V26_FULL', status: 'SKIPPED_FRESH' }],
  } })
  assert.equal(mallWeatherRefreshResultMessage(skipped), '1 项数据仍新鲜，本次未重复入队。')
  assert.equal(parseMallWeatherRefreshResult({ code: 0, data: { jobId: 33, mallId: 7, force: false, reason: '管理端补采', requestedBy: 42, kinds: [{ kind: 'V26_FULL', status: 'QUEUED' }] } }), null)
  assert.equal(parseMallWeatherRefreshResult({ code: 0, data: {
    jobId: 34,
    mallId: 7,
    force: false,
    reason: '管理端补采',
    requestedBy: 42,
    kinds: [
      { kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 },
      { kind: 'V26_FULL', status: 'SKIPPED_FRESH' },
    ],
  } }), null)
  assert.equal(parseMallWeatherRefreshResult({ code: 0, data: {
    jobId: 35,
    mallId: 7,
    force: false,
    reason: '管理端补采',
    requestedBy: 42,
    kinds: [{ kind: 'V3_LIFE_INDEX', status: 'SKIPPED_FRESH', outboxJobId: 42 }],
  } }), null)
})

test('keeps the original idempotent request for every uncertain refresh outcome', () => {
  const request = mallWeatherRefreshRequest(['V26_FULL', 'V3_LIFE_INDEX'], '管理端补采')
  const acceptedData = { code: 0, data: {
    jobId: 31,
    mallId: 7,
    force: false,
    reason: '管理端补采',
    requestedBy: 42,
    kinds: [
      { kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 },
      { kind: 'V3_LIFE_INDEX', status: 'SKIPPED_FRESH' },
    ],
  } }

  assert.equal(mallWeatherRefreshDisposition({ ok: true, status: 202, data: acceptedData }, '42', 7, request).kind, 'accepted')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 202, data: {} }, '42', 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 0, data: {} }, '42', 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 408, data: {} }, '42', 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 409, data: {} }, '42', 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 503, data: {} }, '42', 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: false, status: 403, data: {} }, '42', 7, request).kind, 'rejected')
  assert.equal(mallWeatherRefreshDisposition({ ok: true, status: 200, data: acceptedData }, '42', 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: true, status: 202, data: { ...acceptedData, data: { ...acceptedData.data, requestedBy: 43 } } }, '42', 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: true, status: 202, data: { ...acceptedData, data: { ...acceptedData.data, reason: '其他原因' } } }, '42', 7, request).kind, 'uncertain')
  assert.equal(mallWeatherRefreshDisposition({ ok: true, status: 202, data: { code: 0, data: {
    jobId: 31,
    mallId: 7,
    force: false,
    reason: '管理端补采',
    requestedBy: 42,
    kinds: [{ kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 }],
  } } }, '42', 7, request).kind, 'uncertain')
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
