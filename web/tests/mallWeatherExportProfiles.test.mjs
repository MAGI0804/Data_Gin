import assert from 'node:assert/strict'
import test from 'node:test'

import {
  emptyMallWeatherExportProfileForm,
  mallWeatherExportProfileForm,
  mallWeatherExportProfileListPath,
  mallWeatherExportProfileReadOnly,
  mallWeatherExportProfileSaveRequest,
  parseMallWeatherExportProfile,
  parseMallWeatherExportProfilePage,
} from '../.test-dist/mallWeatherExportProfiles.js'

const profile = {
  id: 9, code: 'north_weather', name: '北区天气', version: 3, enabled: true,
  timeZone: 'Asia/Shanghai', unitSystem: 'metric', dateFormat: '2006-01-02', dateTimeFormat: '2006-01-02 15:04:05',
  fileNameTemplate: 'north_weather.xlsx', filters: { mallIds: [2], cities: ['shanghai'], mallStatuses: ['active'], qualityStatuses: ['valid'] },
  datasets: [{ kind: 'hourly', sheetName: '逐小时', freezeHeader: true, autoFilter: true }],
  createdBy: 17, updatedBy: 17, createdAt: '2026-01-02T03:04:05Z', updatedAt: '2026-01-03T03:04:05Z',
}

test('builds the exact bounded Profile list query and parses paged DTOs', () => {
  assert.equal(mallWeatherExportProfileListPath('', ''), '/v1/weather-export-profiles?pageSize=50')
  assert.equal(mallWeatherExportProfileListPath('true', 'cursor-value'), '/v1/weather-export-profiles?pageSize=50&enabled=true&cursor=cursor-value')
  const page = parseMallWeatherExportProfilePage({ code: 0, data: { items: [profile], pagination: { pageSize: 50, nextCursor: 'next' } } })
  assert.deepEqual(page, {
    items: [{
      ...profile,
      filters: { ...profile.filters },
      datasets: [{ ...profile.datasets[0], columns: [], conditionalFormats: [] }],
    }],
    pageSize: 50,
    nextCursor: 'next',
  })
  assert.equal(parseMallWeatherExportProfilePage({ code: 0, data: { items: [profile], pagination: { pageSize: 101 } } }), null)
  assert.equal(parseMallWeatherExportProfilePage({ code: 0, data: { items: [{ ...profile, datasets: [] }], pagination: { pageSize: 50 } } }), null)
})

test('builds complete create and optimistic update requests without unsafe fields', () => {
  const create = mallWeatherExportProfileSaveRequest({ ...emptyMallWeatherExportProfileForm(), code: 'north_weather', name: '北区天气', mallIds: '2,1', cities: 'Shanghai', start: '2026-01-02T03:04:05Z', end: '2026-01-03T03:04:05Z' })
  assert.deepEqual(create.filters, { mallIds: [1, 2], cities: ['shanghai'], mallStatuses: ['active'], qualityStatuses: [], start: '2026-01-02T03:04:05Z', end: '2026-01-03T03:04:05Z' })
  assert.equal('expectedVersion' in create, false)
  const parsed = parseMallWeatherExportProfile({ code: 0, data: profile })
  assert.ok(parsed)
  const update = mallWeatherExportProfileSaveRequest(mallWeatherExportProfileForm(parsed), parsed.version)
  assert.equal(update.expectedVersion, 3)
  assert.throws(() => mallWeatherExportProfileSaveRequest({ ...emptyMallWeatherExportProfileForm(), code: 'bad', name: 'x', fileNameTemplate: 'unsafe.csv' }), /invalid export profile/)
  assert.throws(() => mallWeatherExportProfileSaveRequest({ ...emptyMallWeatherExportProfileForm(), code: 'north_weather', name: 'x', datasets: [{ kind: 'hourly', sheetName: 'same', columns: [], freezeHeader: true, autoFilter: true, conditionalFormats: [] }, { kind: 'daily', sheetName: 'same', columns: [], freezeHeader: true, autoFilter: true, conditionalFormats: [] }] }), /invalid export profile/)
})

test('recognizes the fixed export Profiles as read-only', () => {
  assert.equal(mallWeatherExportProfileReadOnly('mall_weather_excel_fixed'), true)
  assert.equal(mallWeatherExportProfileReadOnly('mall_weather_excel_fixed_17'), true)
  assert.equal(mallWeatherExportProfileReadOnly('north_weather'), false)
})
