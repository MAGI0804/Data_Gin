import assert from 'node:assert/strict'
import test from 'node:test'

import { buildSourceSavePayload, newSourceDraft, parseLegacySourceList, parseSourceDetail, parseSourcePage, sourceDraftFrom } from '../.test-dist/configurationPages/sourceContracts.js'
import { buildDestinationSavePayload, destinationDraftFrom, newDestinationDraft, parseDestinationDetail, parseDestinationPage, parseDestinationTestResult, parseLegacyDestinationList } from '../.test-dist/configurationPages/destinationContracts.js'
import { buildRuleSavePayload, newRuleDraft, parseLegacyTransformRules, parseRuleTestContent, parseTransformRuleDetail, parseTransformRulePage, readRuleTestResult, ruleDraftFrom } from '../.test-dist/configurationPages/ruleContracts.js'
import { buildDeliveryTaskSavePayload, deliveryTaskDraftFrom, newDeliveryTaskDraft, parseDeliveryRunResult, parseDeliveryTaskDetail, parseDeliveryTaskPage, parseLegacyDeliveryTasks } from '../.test-dist/configurationPages/deliveryTaskContracts.js'
import { buildOrderPushPolicyDraft, buildOrderPushPolicyPayload, parseOrderPushPolicyEnvelope, parseSavedOrderPushPolicy, policyDeliveryRatio } from '../.test-dist/configurationPages/orderPushPolicyContracts.js'

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

const destination = {
  id: 11,
  name: '订单中台',
  code: 'order_hub',
  destination_type: 'http',
  config_json: '{"token":"[已隐藏]","url":"https://example.invalid"}',
  has_secret: true,
  enabled: true,
}

test('accepts only strict destination page and detail envelopes', () => {
  const envelope = { data: { destinations: [destination], pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 } } }
  assert.deepEqual(parseDestinationPage(envelope)?.list, [destination])
  assert.deepEqual(parseDestinationDetail({ data: { destination } }), destination)
  assert.equal(parseDestinationDetail({ destination }), null)
  assert.equal(parseDestinationPage({ data: { ...envelope.data, pagination: { ...envelope.data.pagination, page: '1' } } }), null)
  assert.equal(parseDestinationPage({ data: { ...envelope.data, destinations: [{ ...destination, id: '11' }] } }), null)
  assert.equal(parseDestinationPage({ data: { ...envelope.data, destinations: [{ ...destination, enabled: 'true' }] } }), null)
  assert.equal(parseDestinationPage({ data: { ...envelope.data, destinations: [{ ...destination, has_secret: 'true' }] } }), null)
  assert.equal(parseDestinationPage({ data: { ...envelope.data, destinations: [{ ...destination, config_json: {} }] } }), null)
  assert.equal(parseDestinationPage({ data: { ...envelope.data, destinations: [{ ...destination, id: 0 }] } }), null)
  assert.equal(parseDestinationPage({ data: { ...envelope.data, destinations: [{ ...destination, id: 1.5 }] } }), null)
})

test('uses destination legacy response only when pagination is absent', () => {
  assert.deepEqual(parseLegacyDestinationList({ data: { destinations: [destination] } }), [destination])
  assert.equal(parseLegacyDestinationList({ data: { destinations: [destination], pagination: { page: 0 } } }), null)
  assert.equal(parseLegacyDestinationList({ destinations: [destination] }), null)
})

test('accepts only an explicit matching destination test success result', () => {
  assert.equal(parseDestinationTestResult({ data: { destination_id: 11, status: 'success' } }, 11), true)
  assert.equal(parseDestinationTestResult({ data: { destination_id: 12, status: 'success' } }, 11), false)
  assert.equal(parseDestinationTestResult({ data: { destination_id: '11', status: 'success' } }, 11), false)
  assert.equal(parseDestinationTestResult({ data: { destination_id: 11, status: 'failed' } }, 11), false)
  assert.equal(parseDestinationTestResult({ destination_id: 11, status: 'success' }, 11), false)
})

test('keeps destination redaction placeholders for edits', () => {
  const draft = destinationDraftFrom(destination)
  assert.equal(draft.hasSecret, true)
  assert.match(draft.configJSON, /\[已隐藏\]/)
  const result = buildDestinationSavePayload(draft)
  assert.equal(result.ok, true)
  if (result.ok) assert.equal(result.payload.config_json, draft.configJSON)
  assert.equal(destinationDraftFrom({ ...destination, config_json: '{invalid' }).configJSON, '{invalid')
})

test('validates destination identity and config object before saving', () => {
  assert.deepEqual(buildDestinationSavePayload({ ...newDestinationDraft(), name: ' ', code: '' }), { ok: false, error: '请填写目标名称和编码。' })
  assert.deepEqual(buildDestinationSavePayload({ ...newDestinationDraft(), name: '目标', code: 'target', configJSON: '{' }), { ok: false, error: '配置必须为 JSON 对象。' })
  assert.deepEqual(buildDestinationSavePayload({ ...newDestinationDraft(), name: '目标', code: 'target', configJSON: '[]' }), { ok: false, error: '配置必须为 JSON 对象。' })
  assert.deepEqual(buildDestinationSavePayload({ ...newDestinationDraft(), name: '目标', code: 'target', configJSON: '{"token":"[已隐藏]"}' }), { ok: false, error: '新增推送目标不能使用“[已隐藏]”占位符，请填写真实配置值。' })
  assert.deepEqual(buildDestinationSavePayload({ ...newDestinationDraft(), name: '目标', code: 'target', configJSON: '{"headers":[{"name":"Authorization","value":"[已隐藏]"}]}' }), { ok: false, error: '新增推送目标不能使用“[已隐藏]”占位符，请填写真实配置值。' })
})

test('builds complete trimmed destination payload and defaults', () => {
  const draft = { ...newDestinationDraft(), name: ' 目标 ', code: ' target ', destinationType: 'soap', enabled: false, configJSON: '{"url":"https://example.invalid"}' }
  const result = buildDestinationSavePayload(draft)
  assert.deepEqual(result, { ok: true, payload: { name: '目标', code: 'target', destination_type: 'soap', config_json: draft.configJSON, enabled: false } })
  assert.deepEqual(newDestinationDraft(), { id: null, name: '', code: '', destinationType: 'http', configJSON: '{\n  "url": "",\n  "method": "POST"\n}', enabled: true, hasSecret: false })
})

const rule = { id: 11, source_id: 7, name: '订单映射', rule_type: 'mapping', order_index: 2, config_json: '{"token":"[已隐藏]"}', has_secret: true, enabled: true }

test('strictly parses transform rule envelopes without masking malformed pagination', () => {
  assert.deepEqual(parseTransformRulePage({ data: { rules: [rule], pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 } } })?.list, [rule])
  assert.deepEqual(parseTransformRuleDetail({ data: { rule } }), rule)
  assert.deepEqual(parseLegacyTransformRules({ data: { rules: [rule] } }), [rule])
  assert.equal(parseTransformRulePage({ data: { rules: [rule], pagination: { page: '1', page_size: 20, total: 1, total_pages: 1 } } }), null)
  assert.equal(parseTransformRulePage({ data: { rules: [{ ...rule, source_id: '7' }], pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 } } }), null)
  assert.equal(parseLegacyTransformRules({ data: { rules: [rule], pagination: { page: 0 } } }), null)
})

test('builds exact transform rule saves and preserves redacted edits', () => {
  const draft = ruleDraftFrom(rule)
  assert.deepEqual(buildRuleSavePayload(draft), { ok: true, payload: { source_id: 7, name: '订单映射', rule_type: 'mapping', order_index: 2, config_json: '{"token":"[已隐藏]"}', enabled: true } })
  assert.deepEqual(buildRuleSavePayload({ ...newRuleDraft(7), name: '新增', configJSON: '{"token":"[已隐藏]"}' }), { ok: false, error: '新增规则不能使用“[已隐藏]”占位符，请填写真实配置值。' })
  assert.deepEqual(buildRuleSavePayload({ ...newRuleDraft(undefined), name: '' }), { ok: false, error: '请填写来源、名称和有效的顺序号。' })
})

test('accepts only object rule test input and explicit result fields', () => {
  assert.deepEqual(parseRuleTestContent('{"order_id":1}', '{}'), { ok: true, rawContent: { order_id: 1 } })
  assert.deepEqual(parseRuleTestContent('[]', '{}'), { ok: false, error: '测试原始内容必须是 JSON 对象。' })
  assert.deepEqual(readRuleTestResult({ data: { clean_content: { order_id: 1 } } }), { order_id: 1 })
  assert.equal(readRuleTestResult({ data: { clean_content: 'unsafe' } }), null)
  assert.equal(readRuleTestResult({ clean_content: {} }), null)
})

const deliveryTask = { id: 19, name: '商场订单推送', source_id: 7, clean_table: 'clean_orders', destination_id: 11, trigger_type: 'manual', cron_expr: '', filter_json: '{}', payload_template: '{"id":"{{business_key}}"}', enabled: true }

test('strictly parses delivery task page, detail and legacy envelopes', () => {
  const page = { data: { tasks: [deliveryTask], pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 } } }
  assert.deepEqual(parseDeliveryTaskPage(page)?.list, [deliveryTask])
  assert.deepEqual(parseDeliveryTaskDetail({ data: { task: deliveryTask } }), deliveryTask)
  assert.deepEqual(parseLegacyDeliveryTasks({ data: { tasks: [deliveryTask] } }), [deliveryTask])
  assert.equal(parseDeliveryTaskPage({ data: { ...page.data, pagination: { ...page.data.pagination, page: '1' } } }), null)
  assert.equal(parseDeliveryTaskPage({ data: { ...page.data, tasks: [{ ...deliveryTask, trigger_type: 'cron' }] } }), null)
  assert.equal(parseLegacyDeliveryTasks({ data: { tasks: [deliveryTask], pagination: { page: 0 } } }), null)
})

test('builds exact delivery task saves with trigger and policy validation', () => {
  const draft = deliveryTaskDraftFrom(deliveryTask)
  assert.deepEqual(buildDeliveryTaskSavePayload(draft), { ok: true, payload: { name: deliveryTask.name, source_id: 7, clean_table: 'clean_orders', destination_id: 11, trigger_type: 'manual', cron_expr: '', filter_json: '{}', payload_template: deliveryTask.payload_template, enabled: true } })
  assert.match(buildDeliveryTaskSavePayload({ ...draft, triggerType: 'schedule', cronExpr: '' }).error, /Cron/)
  assert.match(buildDeliveryTaskSavePayload({ ...draft, filterJSON: '[]' }).error, /JSON 对象/)
  assert.deepEqual(newDeliveryTaskDraft(7, 11), { id: null, name: '', sourceID: '7', cleanTable: '', destinationID: '11', triggerType: 'manual', cronExpr: '', filterJSON: '{}', payloadTemplate: '', enabled: true })
})

test('accepts only internally consistent delivery run summaries', () => {
  const envelope = { data: { result: { trace_id: 'delivery-19', total_count: 5, success_count: 4, failed_count: 1, skipped_count: 2 } } }
  assert.deepEqual(parseDeliveryRunResult(envelope), { traceID: 'delivery-19', totalCount: 5, successCount: 4, failedCount: 1, skippedCount: 2 })
  assert.equal(parseDeliveryRunResult({ data: { result: { ...envelope.data.result, total_count: '5' } } }), null)
  assert.equal(parseDeliveryRunResult({ data: { result: { ...envelope.data.result, total_count: 6 } } }), null)
  assert.equal(parseDeliveryRunResult({ data: { result: { ...envelope.data.result, skipped_count: 5 } } }), null)
})

const policyOptions = [{ code: 'qiantan', name: '伯俊-前滩' }, { code: 'custom', name: '自定义目标' }]
const policyConfig = { targets: [{ target_code: 'QIantan', target_name: '旧名称', cycle: 5, skip: 1 }] }

test('strictly parses order push policy GET and PUT envelopes', () => {
  const envelope = { data: { config: policyConfig, targets: policyOptions } }
  assert.deepEqual(parseOrderPushPolicyEnvelope(envelope), { config: policyConfig, options: policyOptions })
  assert.deepEqual(parseSavedOrderPushPolicy({ data: { config: policyConfig } }), policyConfig)
  assert.equal(parseOrderPushPolicyEnvelope({ data: { config: { targets: [{ ...policyConfig.targets[0], cycle: '5' }] }, targets: policyOptions } }), null)
  assert.equal(parseOrderPushPolicyEnvelope({ data: { config: policyConfig, targets: [...policyOptions, { code: 'QIAnTan', name: '重复' }] } }), null)
  assert.equal(parseSavedOrderPushPolicy({ config: policyConfig }), null)
})

test('builds authoritative policy drafts and normalized payloads', () => {
  const draft = buildOrderPushPolicyDraft(policyConfig, policyOptions)
  assert.deepEqual(draft, [{ code: 'qiantan', name: '伯俊-前滩', cycle: '5', skip: '1' }, { code: 'custom', name: '自定义目标', cycle: '0', skip: '0' }])
  assert.equal(policyDeliveryRatio(draft[0]), '80.0%')
  assert.deepEqual(buildOrderPushPolicyPayload(draft, policyOptions), { ok: true, payload: { targets: [{ target_code: 'qiantan', target_name: '伯俊-前滩', cycle: 5, skip: 1 }, { target_code: 'custom', target_name: '自定义目标', cycle: 0, skip: 0 }] } })
  assert.match(buildOrderPushPolicyPayload([{ ...draft[0], skip: '5' }, draft[1]], policyOptions).error, /小于/)
  assert.match(buildOrderPushPolicyPayload([{ ...draft[0], cycle: '' }, draft[1]], policyOptions).error, /非负整数/)
})
