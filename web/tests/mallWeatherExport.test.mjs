import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clearMallWeatherExportSession,
  loadMallWeatherExportSession,
  mallWeatherExportCreateDisposition,
  mallWeatherExportCreateRequest,
  mallWeatherExportCreateResultMatchesRequest,
  mallWeatherExportDownloadReadiness,
  mallWeatherExportDownloadRequestTimeoutMilliseconds,
  mallWeatherExportMaximumPollRetryDelayMilliseconds,
  mallWeatherExportMaximumTransientPollRetries,
  mallWeatherExportContentPath,
  mallWeatherExportDownloadPath,
  mallWeatherExportJobPath,
  mallWeatherExportJobTerminal,
  mallWeatherExportKey,
  mallWeatherExportProgress,
  mallWeatherExportPollRetryDelayMilliseconds,
  mallWeatherExportPollStatusRetryable,
  mallWeatherExportRequestTimeoutMilliseconds,
  mallWeatherExportRequestMatches,
  parseMallWeatherExportCreateResult,
  parseMallWeatherExportJob,
  parseMallWeatherExportSafeErrorMessage,
  resolveMallWeatherExportStorage,
  saveMallWeatherExportSession,
  waitForMallWeatherExportDownload,
  waitForMallWeatherExportRequest,
} from '../.test-dist/mallWeatherExport.js'

const jobID = '123e4567-e89b-42d3-a456-426614174000'
const envelope = (data) => ({ code: 0, data })
const memoryStorage = () => {
  const values = new Map()
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: (key) => values.delete(key),
  }
}
const createdResult = (overrides = {}) => ({
  jobId: jobID,
  status: 'PENDING',
  profileId: 9,
  profileVersion: 3,
  estimatedRows: 480,
  createdBy: 17,
  createdAt: '2026-07-27T10:00:00Z',
  ...overrides,
})
const legacyRequest = (overrides = {}) => ({
  profileId: 9,
  expectedProfileVersion: 3,
  filters: {
    mallIds: [7],
    start: '2026-07-26T10:00:00.000Z',
    end: '2026-08-12T10:00:00.000Z',
  },
  ...overrides,
})

test('builds a fixed mall-scoped request without client-managed profile selectors', () => {
  const request = mallWeatherExportCreateRequest(7)
  assert.deepEqual(request, { filters: { mallIds: [7] } })
  assert.equal(Object.hasOwn(request, 'profileId'), false)
  assert.equal(Object.hasOwn(request, 'expectedProfileVersion'), false)
  assert.equal(mallWeatherExportRequestMatches(request, 7), true)
  assert.equal(mallWeatherExportRequestMatches(request, 8), false)
  assert.equal(mallWeatherExportKey('12345678-abcd'), 'weather-export:12345678-abcd')
  assert.throws(() => mallWeatherExportKey('bad/key'), /invalid mall weather export key/)
  assert.throws(() => mallWeatherExportCreateRequest(0), /invalid mall weather export request/)
})

test('persists fixed and legacy pending requests without changing their idempotent body', () => {
  const storage = memoryStorage()
  const fixedPending = {
    key: mallWeatherExportKey('12345678-fixed'),
    body: mallWeatherExportCreateRequest(7),
  }
  assert.equal(saveMallWeatherExportSession('17', 7, { pending: fixedPending, jobId: '' }, storage), true)
  assert.deepEqual(loadMallWeatherExportSession('17', 7, storage), { pending: fixedPending, jobId: '' })
  assert.equal(loadMallWeatherExportSession('17', 8, storage), null)

  const legacyPending = {
    key: mallWeatherExportKey('12345678-legacy'),
    body: legacyRequest(),
  }
  saveMallWeatherExportSession('17', 7, { pending: legacyPending, jobId: '' }, storage)
  assert.deepEqual(loadMallWeatherExportSession('17', 7, storage), { pending: legacyPending, jobId: '' })

  const rangedFixedPending = {
    key: mallWeatherExportKey('12345678-ranged'),
    body: { filters: legacyRequest().filters },
  }
  saveMallWeatherExportSession('17', 7, { pending: rangedFixedPending, jobId: '' }, storage)
  assert.deepEqual(loadMallWeatherExportSession('17', 7, storage), { pending: rangedFixedPending, jobId: '' })

  saveMallWeatherExportSession('17', 7, { pending: null, jobId: jobID }, storage)
  assert.deepEqual(loadMallWeatherExportSession('17', 7, storage), { pending: null, jobId: jobID })
  assert.equal(saveMallWeatherExportSession('17', 7, { pending: null, jobId: '' }, storage), true)
  assert.equal(loadMallWeatherExportSession('17', 7, storage), null)
  assert.equal(saveMallWeatherExportSession('17', 7, { pending: null, jobId: jobID }, storage), true)
  assert.equal(clearMallWeatherExportSession('17', 7, storage), true)
  assert.equal(loadMallWeatherExportSession('17', 7, storage), null)
  assert.throws(() => loadMallWeatherExportSession('actor', 7, storage), /invalid actor id/)
})

test('keeps exports usable when browser session storage is unavailable', () => {
  const unavailableStorage = {
    getItem: () => { throw new DOMException('blocked', 'SecurityError') },
    setItem: () => { throw new DOMException('quota', 'QuotaExceededError') },
    removeItem: () => { throw new DOMException('blocked', 'SecurityError') },
  }
  const pending = {
    key: mallWeatherExportKey('12345678-unavailable'),
    body: mallWeatherExportCreateRequest(7),
  }

  assert.equal(resolveMallWeatherExportStorage(() => unavailableStorage), unavailableStorage)
  assert.equal(resolveMallWeatherExportStorage(() => {
    throw new DOMException('blocked', 'SecurityError')
  }), null)
  assert.equal(loadMallWeatherExportSession('17', 7, null), null)
  assert.equal(loadMallWeatherExportSession('17', 7, unavailableStorage), null)
  assert.equal(saveMallWeatherExportSession(
    '17', 7, { pending, jobId: '' }, unavailableStorage,
  ), false)
  assert.equal(saveMallWeatherExportSession(
    '17', 7, { pending: null, jobId: '' }, unavailableStorage,
  ), false)
  assert.equal(saveMallWeatherExportSession('17', 7, { pending, jobId: '' }, null), false)
  assert.equal(clearMallWeatherExportSession('17', 7, unavailableStorage), false)
  assert.equal(clearMallWeatherExportSession('17', 7, null), false)

  assert.throws(
    () => loadMallWeatherExportSession('actor', 7, unavailableStorage),
    /invalid actor id/,
  )
  assert.throws(
    () => saveMallWeatherExportSession('actor', 7, { pending, jobId: '' }, null),
    /invalid actor id/,
  )
  assert.throws(
    () => clearMallWeatherExportSession('17', 0, null),
    /invalid mall id/,
  )
  assert.throws(
    () => saveMallWeatherExportSession('17', 7, {
      pending: { ...pending, body: { filters: { mallIds: [8] } } },
      jobId: '',
    }, unavailableStorage),
    /invalid mall weather export session/,
  )
  const circularSession = { pending, jobId: '' }
  circularSession.self = circularSession
  assert.throws(
    () => saveMallWeatherExportSession('17', 7, circularSession, null),
    /circular|cyclic/i,
  )
})

test('rejects incomplete, null, zero and cross-mall stored request selectors', () => {
  const storage = memoryStorage()
  const saveInvalid = (body) => saveMallWeatherExportSession('17', 7, {
    pending: { key: mallWeatherExportKey('12345678-invalid'), body },
    jobId: '',
  }, storage)

  assert.throws(() => saveInvalid({ profileId: 9, filters: legacyRequest().filters }), /invalid mall weather export session/)
  assert.throws(() => saveInvalid({ expectedProfileVersion: 3, filters: legacyRequest().filters }), /invalid mall weather export session/)
  assert.throws(() => saveInvalid({ profileId: 0, expectedProfileVersion: 3, filters: legacyRequest().filters }), /invalid mall weather export session/)
  assert.throws(() => saveInvalid({ profileId: null, expectedProfileVersion: null, filters: legacyRequest().filters }), /invalid mall weather export session/)
  assert.throws(() => saveInvalid({ filters: { mallIds: [8] } }), /invalid mall weather export session/)
  assert.throws(() => saveInvalid({ filters: { mallIds: [7], start: '2026-07-26T10:00:00Z' } }), /invalid mall weather export session/)
  assert.throws(() => saveInvalid({ profileId: 9, expectedProfileVersion: 3, filters: { mallIds: [7] } }), /invalid mall weather export session/)
  assert.throws(() => saveInvalid({
    filters: { mallIds: [7], start: '2026-01-01T00:00:00Z', end: '2027-01-03T00:00:00Z' },
  }), /invalid mall weather export session/)
})

test('matches fixed create results while preserving strict legacy profile matching', () => {
  const result = parseMallWeatherExportCreateResult(envelope(createdResult()))
  assert.ok(result)
  assert.equal(mallWeatherExportCreateResultMatchesRequest(result, mallWeatherExportCreateRequest(7)), true)
  assert.equal(mallWeatherExportCreateResultMatchesRequest(result, legacyRequest()), true)
  assert.equal(mallWeatherExportCreateResultMatchesRequest(result, legacyRequest({ profileId: 10 })), false)
  assert.equal(mallWeatherExportCreateResultMatchesRequest(result, legacyRequest({ expectedProfileVersion: 4 })), false)
})

test('accepts only a trusted 202 and preserves the pending request for every other outcome', () => {
  const fixed = mallWeatherExportCreateRequest(7)
  const accepted = envelope(createdResult())
  assert.equal(mallWeatherExportCreateDisposition(
    { ok: true, status: 202, data: accepted },
    fixed,
  ).kind, 'accepted')

  for (const status of [0, 200, 400, 403, 404, 408, 409, 422, 500, 503]) {
    assert.equal(mallWeatherExportCreateDisposition(
      { ok: status >= 200 && status < 300, status, data: accepted },
      fixed,
    ).kind, 'uncertain')
  }
  assert.equal(mallWeatherExportCreateDisposition(
    { ok: true, status: 202, data: { code: 0, data: { invalid: true } } },
    fixed,
  ).kind, 'uncertain')
  assert.equal(mallWeatherExportCreateDisposition(
    { ok: true, status: 202, data: accepted },
    legacyRequest({ profileId: 10 }),
  ).kind, 'uncertain')
})

test('strictly parses export creation and job progress without leaking unsafe shapes', () => {
  const created = parseMallWeatherExportCreateResult(envelope(createdResult()))
  assert.equal(created?.estimatedRows, 480)
  assert.equal(parseMallWeatherExportCreateResult(envelope(createdResult({ status: 'RUNNING' }))), null)

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
  const underestimated = parseMallWeatherExportJob(envelope({ ...job, totalRows: 1, processedRows: 3 }))
  assert.equal(underestimated?.totalRows, 3)
  assert.equal(underestimated?.processedRows, 3)
  assert.equal(mallWeatherExportProgress({ totalRows: 1, processedRows: 3 }), 100)
  assert.equal(parseMallWeatherExportJob(envelope({ ...job, totalRows: 0, processedRows: 3 }))?.totalRows, 3)
  for (const processedRows of [-1, 1.5, Number.MAX_SAFE_INTEGER + 1]) {
    assert.equal(parseMallWeatherExportJob(envelope({ ...job, processedRows })), null)
  }
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

test('builds stable authenticated download resource paths', () => {
  assert.equal(mallWeatherExportJobPath(jobID), `/v1/weather-exports/${jobID}`)
  assert.equal(mallWeatherExportDownloadPath(jobID), `/v1/weather-exports/${jobID}/download`)
  assert.equal(mallWeatherExportContentPath(jobID), `/v1/weather-exports/${jobID}/content`)
  assert.throws(() => mallWeatherExportJobPath('bad'), /invalid mall weather export job id/)
})

test('bounds authenticated file downloads and aborts clients that do not settle', async () => {
  assert.equal(mallWeatherExportDownloadRequestTimeoutMilliseconds, 900_000)

  const completedController = new AbortController()
  assert.equal(await waitForMallWeatherExportDownload(
    Promise.resolve('ready'), completedController, 100,
  ), 'ready')
  assert.equal(completedController.signal.aborted, false)

  const stalledController = new AbortController()
  await assert.rejects(
    waitForMallWeatherExportDownload(new Promise(() => {}), stalledController, 5),
    { name: 'MallWeatherExportDownloadTimeoutError' },
  )
  assert.equal(stalledController.signal.aborted, true)

  const cancelledController = new AbortController()
  const cancelledRequest = waitForMallWeatherExportRequest(
    new Promise(() => {}), cancelledController, 100,
  )
  cancelledController.abort()
  await assert.rejects(cancelledRequest, { name: 'AbortError' })
})

test('bounds export API requests and classifies finite transient polling retries', async () => {
  assert.equal(mallWeatherExportRequestTimeoutMilliseconds, 30_000)
  assert.equal(mallWeatherExportMaximumTransientPollRetries, 5)
  assert.equal(mallWeatherExportMaximumPollRetryDelayMilliseconds, 30_000)
  for (const status of [0, 408, 429, 500, 502, 503, 599]) {
    assert.equal(mallWeatherExportPollStatusRetryable(status), true)
  }
  for (const status of [200, 400, 401, 403, 404, 409, 422, 600]) {
    assert.equal(mallWeatherExportPollStatusRetryable(status), false)
  }
  assert.deepEqual(
    [1, 2, 3, 4, 5, 6].map(mallWeatherExportPollRetryDelayMilliseconds),
    [2_000, 4_000, 8_000, 16_000, 30_000, 30_000],
  )
  assert.throws(() => mallWeatherExportPollRetryDelayMilliseconds(0), /invalid mall weather export transient failure count/)

  const completedController = new AbortController()
  assert.equal(await waitForMallWeatherExportRequest(
    Promise.resolve('ready'), completedController, 100,
  ), 'ready')
  assert.equal(completedController.signal.aborted, false)

  const stalledController = new AbortController()
  await assert.rejects(
    waitForMallWeatherExportRequest(new Promise(() => {}), stalledController, 5),
    { name: 'MallWeatherExportRequestTimeoutError' },
  )
  assert.equal(stalledController.signal.aborted, true)
})

test('reads only bounded safe API error messages', () => {
  assert.equal(parseMallWeatherExportSafeErrorMessage({ code: 40901, msg: '天气导出文件已过期', data: null }), '天气导出文件已过期')
  assert.equal(parseMallWeatherExportSafeErrorMessage({ code: 0, msg: 'success', data: {} }), null)
  assert.equal(parseMallWeatherExportSafeErrorMessage({ code: 40901, msg: '', data: null }), null)
  assert.equal(parseMallWeatherExportSafeErrorMessage({ code: 40901, msg: 'x'.repeat(201), data: null }), null)
})
