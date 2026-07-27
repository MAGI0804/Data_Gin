import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clearMallWeatherPendingRefresh,
  clearMallWeatherPendingCreate,
  clearMallWeatherPendingSheetPush,
  loadAllMallWeatherPages,
  loadMallWeatherForecastDatasets,
  loadMallWeatherPendingCreate,
  loadMallWeatherPendingRefresh,
  loadMallWeatherPendingSheetPush,
  mallWeatherChartSegments,
  mallWeatherCreateKey,
  mallWeatherCreateRequest,
  mallWeatherForecastQueryWindows,
  mallWeatherDailyForecastDays,
  mallWeatherFreshnessLabel,
  mallWeatherHourlyForecastHours,
  mallWeatherMinutelyForecastMinutes,
  mallWeatherMetric,
  mallWeatherOverviewHasBusinessData,
  mallWeatherOverviewHasHourlyTemperature,
  mallWeatherOverviewReadiness,
  mallWeatherOverviewPath,
  mallWeatherGeocodeCandidatesPath,
  mallWeatherGeocodeConfirmPath,
  mallWeatherCandidateConfirmationRequest,
  mallWeatherCoordinateAdjustmentRequest,
  mallWeatherGeocodeRunTerminal,
  mallWeatherGeocodeTriggerPath,
  mallWeatherMallReady,
  mallWeatherMallDeletePath,
  mallWeatherMallPatchRequest,
  mallWeatherManualCoordinateConfirmationRequest,
  mallWeatherSheetPushKey,
  mallWeatherSheetPushRequest,
  mallWeatherSheetPushRequestMatchesOption,
  mallWeatherSheetPushResultMatchesRequest,
  mergeMallWeatherMalls,
  mallWeatherRefreshKey,
  mallWeatherRefreshDisposition,
  mallWeatherFetchRunsPath,
  mallWeatherFetchRunTerminal,
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
  parseMallWeatherMinutelyPage,
  parseMallWeatherOverview,
  parseMallWeatherFetchRuns,
  parseMallWeatherRefreshResult,
  pollMallWeatherFetchRun,
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

const manualCorrelationID = `manual:${'a'.repeat(48)}`

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

const hourlyItem = (hour, temperatureC = hour) => {
  const forecastTimeUtc = new Date(Date.UTC(2026, 6, 22) + hour * 60 * 60 * 1000)
  const forecastTimeLocal = new Date(forecastTimeUtc.getTime() + 8 * 60 * 60 * 1000)
  return {
    forecastTimeUtc: forecastTimeUtc.toISOString(),
    forecastTimeLocal: `${forecastTimeLocal.toISOString().slice(0, 19)}+08:00`,
    ...completeWeatherTimes,
    temperatureC,
    qualityStatus: 'VALID',
    qualityWarnings: [],
  }
}

const minutelyItem = (minute, precipitationMmH = 0.2) => ({
  forecastMinuteUtc: `2026-07-22T00:${String(minute).padStart(2, '0')}:00Z`,
  forecastMinuteLocal: `2026-07-22T08:${String(minute).padStart(2, '0')}:00+08:00`,
  ...completeWeatherTimes,
  minuteOffset: minute,
  precipitationMmH,
  probabilityPct: 65,
  datasource: 'radar',
  description: '附近有小雨',
  forecastKeypoint: '十分钟后雨势减弱',
  qualityStatus: 'VALID',
  qualityWarnings: [],
})

test('parses valid malls and preserves the list cursor', () => {
  const result = parseMallWeatherMallList({
    code: 0,
    data: {
      nextAfterId: 7,
      items: [
        { id: 7, version: 3, mallCode: 'SH-001', nameCn: '示例商场', province: '上海市', city: '上海', district: '黄浦区', address: '示例路 1 号', longitude: 121.47, latitude: 31.23, coordinateSystem: 'GCJ02', geocodeStatus: 'confirmed', weatherEnabled: true, weatherProvider: 'caiyun', detailProfile: 'full', coverageRadiusM: 1000, timeZone: 'America/New_York', status: 'active' },
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
      timeZone: 'America/New_York',
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

test('builds versioned mall edits, deletes, and manual GCJ-02 confirmation', () => {
  const mall = {
    id: 7, version: 3, mallCode: 'SH-001', nameCn: '示例商场', province: '上海市', city: '上海市', district: '黄浦区',
    address: '示例路 1 号', coordinateSystem: '', geocodeStatus: 'FAILED', weatherEnabled: false,
    detailProfile: 'full', coverageRadiusM: 1000, status: 'draft',
  }
  assert.deepEqual(mallWeatherMallPatchRequest(mall, {
    nameCn: ' 示例商场 ', province: '上海市', city: '上海市', district: '浦东新区', address: ' 世纪大道 1 号 ',
  }), { expectedMallVersion: 3, district: '浦东新区', address: '世纪大道 1 号' })
  assert.equal(mallWeatherMallPatchRequest(mall, {
    nameCn: '示例商场', province: '上海市', city: '上海市', district: '黄浦区', address: '示例路 1 号',
  }), null)
  assert.equal(mallWeatherMallDeletePath(7, 3), '/v1/malls/7?expectedMallVersion=3')
  assert.deepEqual(mallWeatherManualCoordinateConfirmationRequest(4, '121.5', '31.2', ' 人工确认商场入口 '), {
    manualCoordinate: { longitude: 121.5, latitude: 31.2, coordinateSystem: 'GCJ02', reason: '人工确认商场入口' },
    expectedMallVersion: 4,
    weatherEnabled: true,
  })
  assert.throws(() => mallWeatherMallDeletePath(7, 0), /invalid mall delete version/)
  assert.throws(() => mallWeatherManualCoordinateConfirmationRequest(0, '121.5', '31.2', 'bad'), /invalid mall coordinate version/)
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
      longitude: 121.5, latitude: 31.2, coordinateSystem: 'GCJ02', level: '门牌号', confidenceScore: 96, selected: false,
    }],
  } })
  assert.equal(candidates?.items[0].formattedAddress, '上海市浦东新区世纪大道 1 号')
  assert.equal(candidates?.items[0].confidenceScore, 96)
  assert.equal(candidates?.mallVersion, 2)
  assert.equal(parseMallWeatherGeocodeCandidates({ code: 0, data: { mallId: 9, mallVersion: 2, items: [{ id: 1, candidateNo: 1, formattedAddress: 'bad', longitude: 200, latitude: 31, coordinateSystem: 'GCJ02', confidenceScore: 1 }] } }), null)
  assert.equal(parseMallWeatherGeocodeCandidates({ code: 0, data: { mallId: 9, mallVersion: 2, items: [{ id: 1, candidateNo: 1, formattedAddress: 'bad', longitude: 121, latitude: 31, coordinateSystem: 'WGS84', confidenceScore: 80 }] } }), null)
  assert.deepEqual(mallWeatherCandidateConfirmationRequest(candidates.items[0], '121.5', '31.2', ' unchanged ', 2), {
    candidateId: 11, expectedMallVersion: 2, weatherEnabled: true,
  })
  assert.deepEqual(mallWeatherCandidateConfirmationRequest(candidates.items[0], '121.5001', '31.2001', ' 调整到主入口 ', 2), {
    manualCoordinate: { longitude: 121.5001, latitude: 31.2001, coordinateSystem: 'GCJ02', reason: '调整到主入口' },
    expectedMallVersion: 2, weatherEnabled: true,
  })
  assert.throws(() => mallWeatherCandidateConfirmationRequest(candidates.items[0], '', '31.2', 'bad', 2), /invalid coordinate adjustment/)
  for (const status of ['NO_CANDIDATES', 'AUTO_CONFIRMED', 'REVIEW_REQUIRED', 'FAILED', 'STALE']) assert.equal(mallWeatherGeocodeRunTerminal(status), true)
  assert.equal(mallWeatherGeocodeRunTerminal('RUNNING'), false)
})

test('recognizes only active confirmed weather-enabled malls as queryable', () => {
  const mall = parseMallWeatherMallList({ code: 0, data: { items: [{
    id: 7, version: 1, mallCode: 'SH-001', nameCn: '示例商场', province: '上海市', city: '上海市', address: '示例路 1 号',
    longitude: 121.47, latitude: 31.23, coordinateSystem: 'GCJ02', geocodeStatus: 'confirmed', weatherEnabled: true,
    weatherProvider: 'caiyun', detailProfile: 'full', coverageRadiusM: 1000, timeZone: 'Asia/Shanghai', status: 'active',
  }] } })?.items[0]
  assert.equal(mallWeatherMallReady(mall), true)
  assert.equal(mallWeatherMallReady({ ...mall, weatherEnabled: false }), false)
  assert.equal(mallWeatherMallReady({ ...mall, longitude: undefined }), false)
  assert.equal(mallWeatherMallReady({ ...mall, coordinateSystem: 'WGS84' }), false)
  assert.equal(parseMallWeatherMallList({ code: 0, data: { items: [{ ...mall, weatherProvider: undefined }] } }), null)
  assert.equal(parseMallWeatherMallList({ code: 0, data: { items: [{ ...mall, timeZone: undefined }] } }), null)
  assert.equal(parseMallWeatherMallList({ code: 0, data: { items: [{ ...mall, timeZone: ' ' }] } }), null)
  assert.equal(parseMallWeatherMallList({ code: 0, data: { items: [{ ...mall, coordinateSystem: 'WGS84' }] } }), null)
  assert.deepEqual(mallWeatherCoordinateAdjustmentRequest(mall, '121.4701', '31.2301', ' 调整高德坐标 '), {
    manualCoordinate: { longitude: 121.4701, latitude: 31.2301, coordinateSystem: 'GCJ02', reason: '调整高德坐标' },
    expectedMallVersion: 1, weatherEnabled: true,
  })
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

test('treats an overview as hourly-ready only when it contains a finite temperature', () => {
  assert.equal(mallWeatherOverviewHasHourlyTemperature({ hourly: [] }), false)
  assert.equal(mallWeatherOverviewHasHourlyTemperature({ hourly: [{ temperatureC: undefined }] }), false)
  assert.equal(mallWeatherOverviewHasHourlyTemperature({ hourly: [{ temperatureC: 0 }] }), true)
})

test('distinguishes an empty initial overview from partial data waiting for hourly temperature', () => {
  const empty = parseMallWeatherOverview({ code: 0, data: {
    meta: { provider: 'caiyun', freshnessStatus: 'UNAVAILABLE' },
    realtime: null, minutely: [], hourly: [], alerts: [],
  } })
  const partial = parseMallWeatherOverview({ code: 0, data: {
    meta: { provider: 'caiyun', freshnessStatus: 'FRESH' },
    realtime: { temperatureC: 31 }, minutely: [], hourly: [{ forecastTimeLocal: '2026-07-27 10:00:00' }], alerts: [],
  } })
  const ready = parseMallWeatherOverview({ code: 0, data: {
    meta: { provider: 'caiyun', freshnessStatus: 'FRESH' },
    realtime: null, minutely: [], hourly: [{ forecastTimeLocal: '2026-07-27 10:00:00', temperatureC: 0 }], alerts: [],
  } })
  assert.equal(mallWeatherOverviewReadiness(empty), 'waiting-empty')
  assert.equal(mallWeatherOverviewReadiness(partial), 'waiting-hourly-temperature')
  assert.equal(mallWeatherOverviewReadiness(ready), 'ready')
  assert.equal(mallWeatherOverviewHasBusinessData(empty), false)
  assert.equal(mallWeatherOverviewHasBusinessData(partial), true)
})

test('preserves local and nearest precipitation details from realtime data', () => {
  const overview = parseMallWeatherOverview({ code: 0, data: {
    meta: { provider: 'caiyun', freshnessStatus: 'FRESH' },
    realtime: {
      pressurePa: 100123, windDirectionDeg: 165, cloudrateRatio: 0.7, dswrfWM2: 544,
      localPrecipitationStatus: 'ok', localPrecipitationMmH: 0.4, localPrecipitationSource: 'radar',
      nearestPrecipitationStatus: 'ok', nearestPrecipitationDistanceKm: 0.8, nearestPrecipitationMmH: 1.2,
      aqiUsa: 48, aqiDescriptionUsa: '良', comfortIndex: 5, comfortDescription: '闷热',
      ultravioletIndex: 4, ultravioletDescription: '强',
      qualityStatus: 'VALID', qualityWarnings: [],
    },
    minutely: [], hourly: [], alerts: [],
  } })

  assert.equal(overview?.realtime?.localPrecipitationStatus, 'ok')
  assert.equal(overview?.realtime?.localPrecipitationSource, 'radar')
  assert.equal(overview?.realtime?.nearestPrecipitationStatus, 'ok')
  assert.equal(overview?.realtime?.nearestPrecipitationDistanceKm, 0.8)
  assert.equal(overview?.realtime?.nearestPrecipitationMmH, 1.2)
  assert.equal(overview?.realtime?.pressurePa, 100123)
  assert.equal(overview?.realtime?.aqiUsa, 48)
  assert.equal(overview?.realtime?.comfortDescription, '闷热')
  assert.equal(overview?.realtime?.ultravioletDescription, '强')
})

test('builds encoded weather overview paths and rejects invalid mall ids', () => {
  assert.equal(mallWeatherOverviewPath(7), '/v1/malls/7/weather/overview')
  assert.equal(mallWeatherOverviewPath(7, 'Asia/Shanghai'), '/v1/malls/7/weather/overview?timeZone=Asia%2FShanghai')
  assert.throws(() => mallWeatherOverviewPath(0), /invalid mall id/)
})

test('builds bounded complete-series paths and parses paged forecast contracts', () => {
  const start = new Date('2026-07-22T00:00:00.000Z')
  const end = new Date('2026-08-06T00:00:00.000Z')
  const latestURL = new URL(mallWeatherSeriesPath(7, 'hourly', start, end), 'https://example.test')
  assert.equal(latestURL.searchParams.get('latest'), 'true')
  assert.equal(latestURL.searchParams.has('asOf'), false)
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
  const minutelyURL = new URL(mallWeatherSeriesPath(7, 'minutely', start, new Date('2026-07-22T02:00:00.000Z'), '', 'Asia/Shanghai', asOf), 'https://example.test')
  assert.equal(minutelyURL.pathname, '/v1/malls/7/weather/minutely')
  assert.throws(() => mallWeatherSeriesPath(7, 'daily', start, new Date('2026-09-01T00:00:00Z')), /invalid weather range/)

  const hourly = parseMallWeatherHourlyPage(pageEnvelope([{ ...hourlyItem(1, 30), apparentTemperatureC: 33, humidityPct: 74,
    pressurePa: 100800, precipitationProbabilityPct: 60, windDirectionDeg: 180, visibilityKm: 12,
    pm25UgM3: 18, hourlyDescription: '午后有阵雨' }], 'hourly-next'))
  assert.equal(hourly?.items[0].temperatureC, 30)
  assert.equal(hourly?.items[0].apparentTemperatureC, 33)
  assert.equal(hourly?.items[0].humidityPct, 74)
  assert.equal(hourly?.items[0].pressurePa, 100800)
  assert.equal(hourly?.items[0].windDirectionDeg, 180)
  assert.equal(hourly?.items[0].visibilityKm, 12)
  assert.equal(hourly?.items[0].hourlyDescription, '午后有阵雨')
  assert.equal(hourly?.pagination.nextCursor, 'hourly-next')
  const minutely = parseMallWeatherMinutelyPage(pageEnvelope([minutelyItem(1)], 'minutely-next'))
  assert.equal(minutely?.items[0].forecastMinuteLocal, '2026-07-22T08:01:00+08:00')
  assert.equal(minutely?.items[0].probabilityPct, 65)
  assert.equal(minutely?.items[0].forecastKeypoint, '十分钟后雨势减弱')
  assert.equal(minutely?.pagination.nextCursor, 'minutely-next')
  const daily = parseMallWeatherDailyPage(pageEnvelope([{
    forecastDateLocal: '2026-07-22', ...completeWeatherTimes,
    temperatureMinC: 24, temperatureMaxC: 32, temperatureAvgC: 28,
    dayTemperatureMaxC: 32, nightTemperatureMinC: 24,
    precipitationMaxMmH: 1.2, dayPrecipitationProbabilityPct: 60, nightPrecipitationProbabilityPct: 20,
    windMaxSpeedKph: 18, windMaxDirectionDeg: 180, humidityAvgPct: 72, cloudrateAvgRatio: 0.65,
    pressureAvgPa: 100800, visibilityMinKm: 8, dswrfMaxWM2: 600, pm25AvgUgM3: 18,
    aqiMaxChn: 72, aqiAvgUsa: 51, skycon: 'PARTLY_CLOUDY_DAY', daySkycon: 'CLEAR_DAY', nightSkycon: 'CLOUDY',
    sunriseLocalTime: '05:08', sunsetLocalTime: '18:52', qualityStatus: 'VALID', qualityWarnings: [],
  }]))
  assert.equal(daily?.items[0].temperatureMaxC, 32)
  assert.equal(daily?.items[0].nightTemperatureMinC, 24)
  assert.equal(daily?.items[0].dayPrecipitationProbabilityPct, 60)
  assert.equal(daily?.items[0].cloudrateAvgRatio, 0.65)
  assert.equal(daily?.items[0].aqiAvgUsa, 51)
  const life = parseMallWeatherLifeIndexPage(pageEnvelope([{ sourceApi: 'v26_daily', forecastDateLocal: '2026-07-22', indexType: 1, indexCode: 'comfort', isUnknownType: false, ...completeWeatherTimes, qualityStatus: 'VALID', qualityWarnings: [] }]))
  assert.equal(life?.items[0].indexCode, 'comfort')
  assert.equal(parseMallWeatherHourlyPage(pageEnvelope([{ ...hourlyItem(2), forecastTimeLocal: 'bad' }])), null)
  assert.equal(parseMallWeatherHourlyPage(pageEnvelope([{ ...hourlyItem(2), humidityPct: '74' }])), null)
  assert.equal(parseMallWeatherHourlyPage(pageEnvelope([{ ...hourlyItem(2), qualityWarnings: [{ code: 7, path: '' }] }])), null)
  assert.equal(parseMallWeatherMinutelyPage(pageEnvelope([{ ...minutelyItem(2), minuteOffset: 1.5 }])), null)
  assert.equal(parseMallWeatherMinutelyPage(pageEnvelope([{ ...minutelyItem(2), precipitationMmH: '0.2' }])), null)
  assert.equal(parseMallWeatherMinutelyPage(pageEnvelope([{ ...minutelyItem(2), forecastMinuteLocal: 'bad' }])), null)
  assert.equal(parseMallWeatherDailyPage(pageEnvelope([{ forecastDateLocal: '2026-02-30', ...completeWeatherTimes, qualityStatus: 'VALID', qualityWarnings: [] }])), null)
  assert.equal(parseMallWeatherLifeIndexPage(pageEnvelope([{ sourceApi: 'v3_lifeindex', forecastDateLocal: '2026-07-22', indexType: 1, indexCode: 'comfort', isUnknownType: false, ...completeWeatherTimes, qualityStatus: 'VALID', qualityWarnings: [] }])), null)
  assert.equal(parseMallWeatherLifeIndexPage(pageEnvelope([{ sourceApi: 'v3_lifeindex', forecastDateLocal: '2026-07-22', indexType: 1.5, indexCode: 'bad', isUnknownType: false, ...completeWeatherTimes, qualityStatus: 'VALID', qualityWarnings: [] }])), null)
  assert.equal(parseMallWeatherLifeIndexPage(pageEnvelope([], '', { ...completeWeatherMeta, timeZone: '' })), null)
})

test('builds exact 120-minute, 360-hour, and 15-local-day forecast windows', () => {
  const shanghai = mallWeatherForecastQueryWindows(new Date('2026-07-22T02:34:56.000Z'), 'Asia/Shanghai')
  assert.equal(mallWeatherMinutelyForecastMinutes, 120)
  assert.equal(mallWeatherHourlyForecastHours, 360)
  assert.equal(mallWeatherDailyForecastDays, 15)
  assert.equal(shanghai.minutely.start.toISOString(), '2026-07-22T02:34:00.000Z')
  assert.equal(shanghai.minutely.end.toISOString(), '2026-07-22T04:34:00.000Z')
  assert.equal(shanghai.hourly.start.toISOString(), '2026-07-22T02:00:00.000Z')
  assert.equal(shanghai.hourly.end.toISOString(), '2026-08-06T02:00:00.000Z')
  assert.equal(shanghai.hourly.end.getTime() - shanghai.hourly.start.getTime(), 360 * 60 * 60 * 1000)
  assert.equal(shanghai.daily.start.toISOString(), '2026-07-21T16:00:00.000Z')
  assert.equal(shanghai.daily.end.toISOString(), '2026-08-05T16:00:00.000Z')

  const newYorkAcrossDST = mallWeatherForecastQueryWindows(new Date('2026-03-07T17:00:00.000Z'), 'America/New_York')
  assert.equal(newYorkAcrossDST.daily.start.toISOString(), '2026-03-07T05:00:00.000Z')
  assert.equal(newYorkAcrossDST.daily.end.toISOString(), '2026-03-22T04:00:00.000Z')

  const beirutMidnightGap = mallWeatherForecastQueryWindows(new Date('2026-03-29T12:00:00.000Z'), 'Asia/Beirut')
  assert.equal(beirutMidnightGap.daily.start.toISOString(), '2026-03-28T22:00:00.000Z')
  assert.equal(beirutMidnightGap.daily.end.toISOString(), '2026-04-12T21:00:00.000Z')
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
  const forecasts = Array.from({ length: 360 }, (_, index) => hourlyItem(index))
  const responses = [
    pageEnvelope(forecasts.slice(0, 200), 'opaque+/=cursor'),
    pageEnvelope(forecasts.slice(200)),
  ]
  const result = await loadAllMallWeatherPages(async (path) => {
    paths.push(path)
    return { ok: true, status: 200, data: responses.shift() }
  }, 7, 'hourly', window, 'Asia/Shanghai', asOf, parseMallWeatherHourlyPage)

  assert.equal(result.items.length, 360)
  assert.equal(result.items[0].temperatureC, 0)
  assert.equal(result.items[359].temperatureC, 359)
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

test('loads every minutely page with one fixed 120-minute window and snapshot time', async () => {
  const window = {
    start: new Date('2026-07-22T02:34:00.000Z'),
    end: new Date('2026-07-22T04:34:00.000Z'),
  }
  const asOf = new Date('2026-07-22T02:34:56.000Z')
  const paths = []
  const responses = [pageEnvelope([minutelyItem(1)], 'minute-cursor'), pageEnvelope([minutelyItem(2)])]
  const result = await loadAllMallWeatherPages(async (path) => {
    paths.push(path)
    return { ok: true, status: 200, data: responses.shift() }
  }, 7, 'minutely', window, 'Asia/Shanghai', asOf, parseMallWeatherMinutelyPage)

  assert.deepEqual(result.items.map((item) => item.minuteOffset), [1, 2])
  assert.equal(paths.length, 2)
  const first = new URL(paths[0], 'https://example.test')
  const second = new URL(paths[1], 'https://example.test')
  for (const parameter of ['start', 'end', 'timeZone', 'latest', 'asOf', 'pageSize']) {
    assert.equal(second.searchParams.get(parameter), first.searchParams.get(parameter))
  }
  assert.equal(first.searchParams.get('start'), window.start.toISOString())
  assert.equal(first.searchParams.get('end'), window.end.toISOString())
  assert.equal(first.searchParams.get('asOf'), asOf.toISOString())
  assert.equal(second.searchParams.get('cursor'), 'minute-cursor')
})

test('loads all forecast datasets with one snapshot and isolates a failed series', async () => {
  const requestedAt = new Date('2026-07-22T02:34:56.000Z')
  const paths = []
  let hourlyPage = 0
  const datasets = loadMallWeatherForecastDatasets(async (path) => {
    paths.push(path)
    const url = new URL(path, 'https://example.test')
    const series = url.pathname.split('/').at(-1)
    if (series === 'hourly') {
      hourlyPage++
      if (hourlyPage === 1) {
        return { ok: true, status: 200, data: pageEnvelope([hourlyItem(0)], 'hourly-next') }
      }
      return { ok: false, status: 500, data: {} }
    }
    return { ok: true, status: 200, data: pageEnvelope([]) }
  }, 7, 'Asia/Shanghai', requestedAt)

  const results = await Promise.allSettled([
    datasets.minutely,
    datasets.hourly,
    datasets.daily,
    datasets.life,
  ])
  assert.deepEqual(results.map((result) => result.status), [
    'fulfilled',
    'rejected',
    'fulfilled',
    'fulfilled',
  ])
  assert.match(results[1].reason.message, /HTTP 500/)
  assert.deepEqual(results[0].value.items, [])
  assert.deepEqual(results[2].value.items, [])
  assert.deepEqual(results[3].value.items, [])

  assert.equal(paths.length, 5)
  assert.deepEqual(
    new Set(paths.map((path) => new URL(path, 'https://example.test').searchParams.get('asOf'))),
    new Set([requestedAt.toISOString()]),
  )
  assert.deepEqual(
    new Set(paths.map((path) => new URL(path, 'https://example.test').searchParams.get('timeZone'))),
    new Set(['Asia/Shanghai']),
  )
  const hourlyPaths = paths.filter((path) => new URL(path, 'https://example.test').pathname.endsWith('/hourly'))
  assert.equal(hourlyPaths.length, 2)
  assert.equal(new URL(hourlyPaths[1], 'https://example.test').searchParams.get('cursor'), 'hourly-next')
})

test('builds validated manual refresh requests and idempotency keys', () => {
  assert.equal(mallWeatherRefreshPath(7), '/v1/malls/7/weather-refresh')
	assert.deepEqual(mallWeatherRefreshRequest(['V26_FULL'], ' 管理端补采 '), {
		kinds: ['V26_FULL'],
    force: false,
    reason: '管理端补采',
  })
  assert.equal(mallWeatherRefreshKey('12345678-abcd'), 'weather-refresh:12345678-abcd')
  assert.throws(() => mallWeatherRefreshKey('bad key'), /invalid refresh key/)
  assert.throws(() => mallWeatherRefreshRequest([], '补采'), /invalid refresh kinds/)
	assert.throws(() => mallWeatherRefreshRequest(['V26_FULL', 'V26_FULL'], '补采'), /invalid refresh kinds/)
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
		body: mallWeatherRefreshRequest(['V26_FULL'], '另一账号补采'),
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
    requestedAt: '2026-07-27T08:00:00Z',
    correlationId: manualCorrelationID,
    kinds: [{ kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 }],
  } })
  assert.equal(mallWeatherRefreshResultMessage(result), '1 个采集任务已进入异步队列；稍后重新加载天气即可查看结果。')

  const skipped = parseMallWeatherRefreshResult({ code: 0, data: {
    jobId: 32,
    mallId: 7,
    force: false,
    reason: '管理端补采',
    requestedBy: 42,
    requestedAt: '2026-07-27T08:00:00Z',
    correlationId: manualCorrelationID,
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
    requestedAt: '2026-07-27T08:00:00Z',
    correlationId: manualCorrelationID,
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
    requestedAt: '2026-07-27T08:00:00Z',
    correlationId: manualCorrelationID,
    kinds: [{ kind: 'V3_LIFE_INDEX', status: 'SKIPPED_FRESH', outboxJobId: 42 }],
  } }), null)
})

test('keeps the original idempotent request for every uncertain refresh outcome', () => {
  const request = mallWeatherRefreshRequest(['V26_FULL'], '管理端补采')
  const acceptedData = { code: 0, data: {
    jobId: 31,
    mallId: 7,
    force: false,
    reason: '管理端补采',
    requestedBy: 42,
    requestedAt: '2026-07-27T08:00:00Z',
    correlationId: manualCorrelationID,
    kinds: [{ kind: 'V26_FULL', status: 'QUEUED', outboxJobId: 41 }],
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
    requestedAt: '2026-07-27T08:00:00Z',
    correlationId: manualCorrelationID,
    kinds: [{ kind: 'V3_LIFE_INDEX', status: 'SKIPPED_FRESH' }],
  } } }, '42', 7, request).kind, 'uncertain')
})

test('builds and parses bounded fetch-run audit queries for weather polling', () => {
  const start = new Date('2026-07-27T08:00:00Z')
  const end = new Date('2026-07-27T08:10:00Z')
  const path = new URL(mallWeatherFetchRunsPath(7, start, end, 'MANUAL', manualCorrelationID), 'https://example.test')
  assert.equal(path.pathname, '/v1/malls/7/weather/fetch-runs')
  assert.equal(path.searchParams.get('start'), start.toISOString())
  assert.equal(path.searchParams.get('end'), end.toISOString())
  assert.equal(path.searchParams.get('taskKind'), 'MANUAL')
  assert.equal(path.searchParams.get('endpointKind'), 'v26_weather')
  assert.equal(path.searchParams.get('correlationId'), manualCorrelationID)
  assert.equal(path.searchParams.get('pageSize'), '10')

  const parsed = parseMallWeatherFetchRuns({ code: 0, data: {
    items: [{
      runUuid: 'run-20260727-1',
      correlationId: manualCorrelationID,
      provider: 'CAIYUN',
      endpointKind: 'v26_weather',
      taskKind: 'MANUAL',
      requestedHourlySteps: 360,
      requestedDailySteps: 15,
      attemptCount: 1,
      status: 'SUCCESS',
      durationMs: 1200,
      rowCounts: { hourly: 360, daily: 15 },
      parseWarnings: [],
      createdAtUtc: '2026-07-27T08:00:02Z',
      createdAtLocal: '2026-07-27T16:00:02+08:00',
      updatedAtUtc: '2026-07-27T08:00:03Z',
      updatedAtLocal: '2026-07-27T16:00:03+08:00',
      finishedAtUtc: '2026-07-27T08:00:03Z',
      finishedAtLocal: '2026-07-27T16:00:03+08:00',
    }],
    meta: { timeZone: 'Asia/Shanghai' },
    pagination: { pageSize: 10 },
  } })
  assert.equal(parsed.items[0].rowCounts.hourly, 360)
  assert.equal(parsed.items[0].status, 'SUCCESS')
  assert.equal(mallWeatherFetchRunTerminal('SUCCESS'), true)
  assert.equal(mallWeatherFetchRunTerminal('PARTIAL_SUCCESS'), true)
  assert.equal(mallWeatherFetchRunTerminal('FAILED'), true)
  assert.equal(mallWeatherFetchRunTerminal('RUNNING'), false)

  assert.throws(() => mallWeatherFetchRunsPath(7, end, start, 'MANUAL', manualCorrelationID), /invalid weather fetch run range/)
  assert.equal(parseMallWeatherFetchRuns({ code: 0, data: {
    items: [{ ...parsed.items[0], rowCounts: { hourly: -1 } }],
    meta: { timeZone: 'Asia/Shanghai' },
    pagination: { pageSize: 10 },
  } }), null)
})

test('polls serially until the matching manual weather run becomes terminal', async () => {
  const requestedAt = '2026-07-27T08:00:00Z'
  const responses = [
    { items: [] },
    { items: [
      { status: 'SUCCESS', createdAtUtc: '2026-07-27T08:00:02Z', correlationId: `manual:${'b'.repeat(48)}` },
      { status: 'RUNNING', createdAtUtc: '2026-07-27T08:00:01Z' },
    ] },
    { items: [{ status: 'SUCCESS', createdAtUtc: '2026-07-27T08:00:01Z', rowCounts: { hourly: 360 } }] },
  ]
  let active = 0
  let maximumActive = 0
  let waits = 0
  const request = async (path) => {
    active++
    maximumActive = Math.max(maximumActive, active)
    assert.equal(new URL(path, 'https://example.test').searchParams.get('taskKind'), 'MANUAL')
    const current = responses.shift()
    active--
    return { ok: true, status: 200, data: { code: 0, data: {
      items: current.items.map((item, index) => ({
        runUuid: `run-${responses.length}-${index}`,
        correlationId: manualCorrelationID,
        provider: 'CAIYUN',
        endpointKind: 'v26_weather',
        taskKind: 'MANUAL',
        requestedHourlySteps: 360,
        requestedDailySteps: 15,
        attemptCount: 1,
        durationMs: 100,
        rowCounts: {},
        parseWarnings: [],
        createdAtLocal: '2026-07-27T16:00:01+08:00',
        updatedAtUtc: '2026-07-27T08:00:02Z',
        updatedAtLocal: '2026-07-27T16:00:02+08:00',
        ...item,
      })),
      meta: { timeZone: 'Asia/Shanghai' },
      pagination: { pageSize: 10 },
    } } }
  }
  const result = await pollMallWeatherFetchRun(request, 7, requestedAt, 'MANUAL', manualCorrelationID, {
    maxAttempts: 5,
    now: () => new Date('2026-07-27T08:02:00Z'),
    wait: async () => { waits++ },
  })
  assert.equal(result.kind, 'terminal')
  assert.equal(result.run.status, 'SUCCESS')
  assert.equal(result.run.rowCounts.hourly, 360)
  assert.equal(waits, 2)
  assert.equal(maximumActive, 1)
})

test('ignores old terminal runs and stops polling on timeout or cancellation', async () => {
  const oldRunResponse = { ok: true, status: 200, data: { code: 0, data: {
    items: [{
      runUuid: 'old-run',
      correlationId: manualCorrelationID,
      provider: 'CAIYUN',
      endpointKind: 'v26_weather',
      taskKind: 'MANUAL',
      requestedHourlySteps: 360,
      requestedDailySteps: 15,
      attemptCount: 1,
      status: 'SUCCESS',
      durationMs: 100,
      rowCounts: { hourly: 360 },
      parseWarnings: [],
      createdAtUtc: '2026-07-27T07:59:59Z',
      createdAtLocal: '2026-07-27T15:59:59+08:00',
      updatedAtUtc: '2026-07-27T08:00:00Z',
      updatedAtLocal: '2026-07-27T16:00:00+08:00',
    }],
    meta: { timeZone: 'Asia/Shanghai' },
    pagination: { pageSize: 10 },
  } } }
  const timedOut = await pollMallWeatherFetchRun(async () => oldRunResponse, 7, '2026-07-27T08:00:00Z', 'MANUAL', manualCorrelationID, {
    maxAttempts: 2,
    now: () => new Date('2026-07-27T08:02:00Z'),
    wait: async () => {},
  })
  assert.deepEqual(timedOut, { kind: 'timed_out' })

  const controller = new AbortController()
  controller.abort()
  const cancelled = await pollMallWeatherFetchRun(async () => oldRunResponse, 7, '2026-07-27T08:00:00Z', 'MANUAL', manualCorrelationID, {
    signal: controller.signal,
  })
  assert.deepEqual(cancelled, { kind: 'cancelled' })
})

test('retries temporary fetch-run failures but stops on deterministic authorization errors', async () => {
  const successResponse = { ok: true, status: 200, data: { code: 0, data: {
    items: [{
      runUuid: 'manual-run-after-retry',
      correlationId: manualCorrelationID,
      provider: 'CAIYUN',
      endpointKind: 'v26_weather',
      taskKind: 'MANUAL',
      requestedHourlySteps: 360,
      requestedDailySteps: 15,
      attemptCount: 1,
      status: 'SUCCESS',
      durationMs: 100,
      rowCounts: { hourly: 360 },
      parseWarnings: [],
      createdAtUtc: '2026-07-27T08:00:01Z',
      createdAtLocal: '2026-07-27T16:00:01+08:00',
      updatedAtUtc: '2026-07-27T08:00:02Z',
      updatedAtLocal: '2026-07-27T16:00:02+08:00',
      finishedAtUtc: '2026-07-27T08:00:02Z',
      finishedAtLocal: '2026-07-27T16:00:02+08:00',
    }],
    meta: { timeZone: 'Asia/Shanghai' },
    pagination: { pageSize: 10 },
  } } }
  let temporaryCalls = 0
  const recovered = await pollMallWeatherFetchRun(async () => {
    temporaryCalls++
    return temporaryCalls === 1 ? { ok: false, status: 503, data: {} } : successResponse
  }, 7, '2026-07-27T08:00:00Z', 'MANUAL', manualCorrelationID, {
    maxAttempts: 3,
    now: () => new Date('2026-07-27T08:02:00Z'),
    wait: async () => {},
  })
  assert.equal(recovered.kind, 'terminal')
  assert.equal(temporaryCalls, 2)

  let forbiddenCalls = 0
  const forbidden = await pollMallWeatherFetchRun(async () => {
    forbiddenCalls++
    return { ok: false, status: 403, data: {} }
  }, 7, '2026-07-27T08:00:00Z', 'MANUAL', manualCorrelationID, {
    maxAttempts: 3,
    now: () => new Date('2026-07-27T08:02:00Z'),
    wait: async () => {},
  })
  assert.deepEqual(forbidden, { kind: 'query_error', status: 403 })
  assert.equal(forbiddenCalls, 1)

  for (const status of [0, 503]) {
    let unavailableCalls = 0
    const unavailable = await pollMallWeatherFetchRun(async () => {
      unavailableCalls++
      return { ok: false, status, data: {} }
    }, 7, '2026-07-27T08:00:00Z', 'MANUAL', manualCorrelationID, {
      maxAttempts: 2,
      now: () => new Date('2026-07-27T08:02:00Z'),
      wait: async () => {},
    })
    assert.deepEqual(unavailable, { kind: 'query_error', status })
    assert.equal(unavailableCalls, 2)
  }
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
