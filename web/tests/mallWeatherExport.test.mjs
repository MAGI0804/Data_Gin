import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clearMallWeatherExportSession,
  loadMallWeatherExportSession,
  mallWeatherDefaultExportProfileRequest,
  mallWeatherExportCreateRequest,
  mallWeatherExportDownloadReadiness,
  mallWeatherExportDownloadPath,
  mallWeatherExportJobPath,
  mallWeatherExportJobTerminal,
  mallWeatherExportKey,
  mallWeatherExportProfilesPath,
  mallWeatherExportProgress,
  mallWeatherExportRequestMatches,
  parseMallWeatherExportCreateResult,
  parseMallWeatherExportDownload,
  parseMallWeatherExportJob,
  parseMallWeatherExportSafeErrorMessage,
  parseMallWeatherExportProfile,
  parseMallWeatherExportProfilePage,
  saveMallWeatherExportSession,
  selectMallWeatherCompleteExportProfile,
  selectMallWeatherExportProfile,
} from '../.test-dist/mallWeatherExport.js'

const jobID = '123e4567-e89b-42d3-a456-426614174000'

const profile = (overrides = {}) => ({
  id: 9,
  code: 'mall_weather_full',
  name: '完整天气',
  version: 3,
  enabled: true,
  timeZone: 'Asia/Shanghai',
  datasets: [
    { kind: 'minutely', sheetName: '分钟降水', latest: true },
    { kind: 'hourly', sheetName: '小时预报', latest: true },
  ],
  ...overrides,
})

const envelope = (data) => ({ code: 0, data })
const memoryStorage = () => {
  const values = new Map()
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: (key) => values.delete(key),
  }
}

test('strictly parses enabled export profiles and bounded pagination', () => {
  const parsed = parseMallWeatherExportProfilePage(envelope({
    items: [profile()],
    pagination: { pageSize: 100, nextCursor: 'next' },
  }))
  assert.equal(parsed?.items[0].datasets[0].kind, 'minutely')
  assert.equal(parsed?.pagination.nextCursor, 'next')
  assert.equal(parseMallWeatherExportProfile(envelope(profile()))?.version, 3)

  assert.equal(parseMallWeatherExportProfilePage(envelope({
    items: [profile({ enabled: false })],
    pagination: { pageSize: 100 },
  }))?.items[0].enabled, false)
  assert.equal(parseMallWeatherExportProfilePage(envelope({ items: [profile({ datasets: [{ kind: 'secret', sheetName: 'bad' }] })], pagination: { pageSize: 100 } })), null)
  assert.equal(parseMallWeatherExportProfilePage(envelope({ items: [profile(), profile()], pagination: { pageSize: 100 } })), null)
  assert.equal(parseMallWeatherExportProfilePage({ code: 500, data: {} }), null)
})

test('prefers a profile containing both minutely and hourly datasets', () => {
  const realtimeOnly = parseMallWeatherExportProfile(envelope(profile({
    id: 1,
    code: 'realtime_only',
    datasets: [{ kind: 'realtime', sheetName: '实况', latest: true }],
  })))
  const forecast = parseMallWeatherExportProfile(envelope(profile()))
  assert.equal(selectMallWeatherExportProfile([realtimeOnly, forecast].filter(Boolean))?.id, 9)
  assert.equal(selectMallWeatherExportProfile([]), null)
})

test('selects only the fixed complete weather export template', () => {
  const datasets = mallWeatherDefaultExportProfileRequest().datasets.map(({ kind, sheetName, latest }) => ({
    kind, sheetName, ...(latest ? { latest } : {}),
  }))
  const incomplete = parseMallWeatherExportProfile(envelope(profile()))
  const complete = parseMallWeatherExportProfile(envelope(profile({ datasets, version: 4 })))
  const dynamic = parseMallWeatherExportProfile(envelope(profile({ id: 10, code: 'custom_weather', datasets, version: 9 })))
  assert.equal(selectMallWeatherCompleteExportProfile([incomplete, dynamic, complete].filter(Boolean))?.version, 4)
  assert.equal(selectMallWeatherCompleteExportProfile([incomplete, dynamic].filter(Boolean)), null)
})

test('builds the complete default export profile with latest forecast semantics', () => {
  const request = mallWeatherDefaultExportProfileRequest()
  assert.equal(request.code, 'mall_weather_full')
  assert.equal(request.datasets.length, 7)
  assert.deepEqual(request.datasets.map((dataset) => dataset.kind), [
    'malls', 'realtime', 'minutely', 'hourly', 'daily', 'alerts', 'life_indices',
  ])
  assert.equal('latest' in request.datasets[0], false)
  for (const dataset of request.datasets.slice(1)) {
    assert.equal(dataset.latest, true)
    assert.equal(dataset.freezeHeader, true)
    assert.equal(dataset.autoFilter, true)
  }
  assert.equal(request.datasets[2].sheetName, '约1公里分钟降水')
  assert.match(request.fileNameTemplate, /\.xlsx$/)
  assert.equal(mallWeatherDefaultExportProfileRequest(7).expectedVersion, 7)
})

test('creates a mall-scoped deterministic time window and reusable idempotency key', () => {
  const parsedProfile = parseMallWeatherExportProfile(envelope(profile()))
  assert.ok(parsedProfile)
  const request = mallWeatherExportCreateRequest(parsedProfile, 7, new Date('2026-07-27T10:37:00Z'))
  assert.deepEqual(request, {
    profileId: 9,
    expectedProfileVersion: 3,
    filters: {
      mallIds: [7],
      start: '2026-07-26T10:00:00.000Z',
      end: '2026-08-12T10:00:00.000Z',
    },
  })
  assert.equal(mallWeatherExportRequestMatches(request, parsedProfile, 7), true)
  assert.equal(mallWeatherExportRequestMatches(request, parsedProfile, 8), false)
  assert.equal(mallWeatherExportKey('12345678-abcd'), 'weather-export:12345678-abcd')
  assert.throws(() => mallWeatherExportKey('bad/key'), /invalid mall weather export key/)
  assert.throws(() => mallWeatherExportCreateRequest(parsedProfile, 0), /invalid mall weather export request/)
})

test('persists and restores mall-scoped idempotent requests and active jobs', () => {
  const storage = memoryStorage()
  const parsedProfile = parseMallWeatherExportProfile(envelope(profile()))
  assert.ok(parsedProfile)
  const pending = {
    key: mallWeatherExportKey('12345678-session'),
    body: mallWeatherExportCreateRequest(parsedProfile, 7, new Date('2026-07-27T10:37:00Z')),
  }
  saveMallWeatherExportSession('17', 7, { pending, jobId: '' }, storage)
  assert.deepEqual(loadMallWeatherExportSession('17', 7, storage), { pending, jobId: '' })
  assert.equal(loadMallWeatherExportSession('17', 8, storage), null)

  saveMallWeatherExportSession('17', 7, { pending: null, jobId: jobID }, storage)
  assert.deepEqual(loadMallWeatherExportSession('17', 7, storage), { pending: null, jobId: jobID })
  clearMallWeatherExportSession('17', 7, storage)
  assert.equal(loadMallWeatherExportSession('17', 7, storage), null)

  assert.throws(() => saveMallWeatherExportSession('17', 7, {
    pending: { ...pending, body: { ...pending.body, filters: { ...pending.body.filters, mallIds: [8] } } },
    jobId: '',
  }, storage), /invalid mall weather export session/)
  assert.throws(() => loadMallWeatherExportSession('actor', 7, storage), /invalid actor id/)
})

test('strictly parses export creation and job progress without leaking unsafe shapes', () => {
  const created = parseMallWeatherExportCreateResult(envelope({
    jobId: jobID,
    status: 'PENDING',
    profileId: 9,
    profileVersion: 3,
    estimatedRows: 480,
    createdBy: 17,
    createdAt: '2026-07-27T10:00:00Z',
  }))
  assert.equal(created?.estimatedRows, 480)
  assert.equal(parseMallWeatherExportCreateResult(envelope({ ...created, status: 'RUNNING' })), null)

  const job = parseMallWeatherExportJob(envelope({
    jobId: jobID,
    profileId: 9,
    profileVersion: 3,
    status: 'RUNNING',
    totalRows: 480,
    processedRows: 120,
    currentSheet: '小时预报',
    cancelRequested: false,
    fileSizeBytes: 0,
  }))
  assert.equal(job?.currentSheet, '小时预报')
  assert.equal(mallWeatherExportProgress(job), 25)
  assert.equal(mallWeatherExportJobTerminal('RUNNING'), false)
  assert.equal(mallWeatherExportJobTerminal('SUCCEEDED'), true)
  assert.equal(parseMallWeatherExportJob(envelope({ ...job, processedRows: 481 })), null)
  assert.equal(parseMallWeatherExportJob(envelope({ ...job, status: 'UNKNOWN' })), null)
})

test('requires a succeeded job with at least one minute of download lifetime', () => {
  const now = new Date('2026-07-27T10:00:00Z')
  assert.equal(mallWeatherExportDownloadReadiness({ status: 'RUNNING' }, now), 'not-ready')
  assert.equal(mallWeatherExportDownloadReadiness({ status: 'EXPIRED' }, now), 'expired')
  assert.equal(mallWeatherExportDownloadReadiness({ status: 'SUCCEEDED' }, now), 'expired')
  assert.equal(mallWeatherExportDownloadReadiness({ status: 'SUCCEEDED', expiresAt: '2026-07-27T10:00:59Z' }, now), 'expired')
  assert.equal(mallWeatherExportDownloadReadiness({ status: 'SUCCEEDED', expiresAt: '2026-07-27T10:01:00Z' }, now), 'ready')
})

test('accepts only short HTTPS download URLs and builds stable resource paths', () => {
  const download = parseMallWeatherExportDownload(envelope({
    url: 'https://bucket.oss-cn-shanghai.aliyuncs.com/result.xlsx?signature=short-lived',
    expiresAt: '2026-07-27T10:05:00Z',
  }))
  assert.match(download?.url ?? '', /^https:/)
  assert.equal(parseMallWeatherExportDownload(envelope({ url: 'http://example.com/result.xlsx', expiresAt: '2026-07-27T10:05:00Z' })), null)
  assert.equal(parseMallWeatherExportDownload(envelope({ url: 'https://user:secret@example.com/result.xlsx', expiresAt: '2026-07-27T10:05:00Z' })), null)
  assert.equal(mallWeatherExportJobPath(jobID), `/v1/weather-exports/${jobID}`)
  assert.equal(mallWeatherExportDownloadPath(jobID), `/v1/weather-exports/${jobID}/download`)
  assert.equal(mallWeatherExportProfilesPath('next cursor'), '/v1/weather-export-profiles?enabled=true&pageSize=100&cursor=next+cursor')
  assert.equal(mallWeatherExportProfilesPath('', false), '/v1/weather-export-profiles?enabled=false&pageSize=100')
  assert.throws(() => mallWeatherExportJobPath('bad'), /invalid mall weather export job id/)
})

test('reads only bounded safe API error messages', () => {
  assert.equal(parseMallWeatherExportSafeErrorMessage({ code: 40901, msg: '天气导出文件已过期', data: null }), '天气导出文件已过期')
  assert.equal(parseMallWeatherExportSafeErrorMessage({ code: 0, msg: 'success', data: {} }), null)
  assert.equal(parseMallWeatherExportSafeErrorMessage({ code: 40901, msg: '', data: null }), null)
  assert.equal(parseMallWeatherExportSafeErrorMessage({ code: 40901, msg: 'x'.repeat(201), data: null }), null)
})
