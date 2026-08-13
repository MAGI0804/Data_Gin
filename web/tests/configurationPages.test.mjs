import assert from 'node:assert/strict'
import test from 'node:test'

import { buildSourceSavePayload, newSourceDraft, parseLegacySourceList, parseSourceDetail, parseSourcePage, sourceDraftFrom } from '../.test-dist/configurationPages/sourceContracts.js'

const source = {
  id: 7,
  name: '订单 API',
  code: 'orders_api',
  source_type: 'api_poll',
  enabled: true,
  auth_type: 'bearer',
  config_json: '{"token":"[已隐藏]"}',
  has_secret: true,
  schema_json: '{}',
  dedupe_keys: '["id"]',
  source_query_key: 'shop_id',
}

test('accepts only complete source list and detail envelopes', () => {
  assert.deepEqual(parseSourcePage({ data: { sources: [source], pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 } } })?.list, [source])
  assert.deepEqual(parseSourceDetail({ data: { source } }), source)
  assert.deepEqual(parseLegacySourceList({ data: { sources: [source] } }), [source])
  assert.equal(parseSourcePage({ data: { sources: [{ ...source, enabled: 'true' }], pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 } } }), null)
  assert.equal(parseSourcePage({ data: { sources: [{ ...source, id: '7' }], pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 } } }), null)
  assert.equal(parseLegacySourceList({ data: { sources: [source], pagination: { page: 0 } } }), null)
  assert.equal(parseSourceDetail({ source }), null)
})

test('keeps redacted placeholders and normalizes editable JSON', () => {
  const draft = sourceDraftFrom(source)
  assert.equal(draft.hasSecret, true)
  assert.match(draft.configJSON, /\[已隐藏\]/)
  const result = buildSourceSavePayload(draft)
  assert.equal(result.ok, true)
  if (result.ok) assert.equal(result.payload.config_json, draft.configJSON)
})

test('validates source identity and JSON shapes before saving', () => {
  assert.deepEqual(buildSourceSavePayload({ ...newSourceDraft(), name: ' ', code: '' }), { ok: false, error: '请填写数据源名称和编码。' })
  assert.deepEqual(buildSourceSavePayload({ ...newSourceDraft(), name: 'API', code: 'api', configJSON: '[]' }), { ok: false, error: '配置和 Schema 必须为 JSON 对象，去重键必须为 JSON 数组。' })
  assert.deepEqual(buildSourceSavePayload({ ...newSourceDraft(), name: 'API', code: 'api', dedupeKeys: '{}' }), { ok: false, error: '配置和 Schema 必须为 JSON 对象，去重键必须为 JSON 数组。' })
  assert.deepEqual(buildSourceSavePayload({ ...newSourceDraft(), name: 'API', code: 'api', configJSON: '{"token":"[已隐藏]"}' }), { ok: false, error: '新增数据源不能使用“[已隐藏]”占位符，请填写真实配置值。' })
})
