import type { MonitoringPage } from '../monitoringRecords'

export type TransformRule = {
  id: number
  source_id: number
  name: string
  rule_type: string
  order_index: number
  config_json: string
  has_secret?: boolean
  enabled: boolean
}

export type RuleDraft = {
  id: number | null
  sourceID: string
  name: string
  ruleType: string
  orderIndex: string
  configJSON: string
  enabled: boolean
  hasSecret: boolean
}

type RuleSaveResult =
  | { ok: true; payload: { source_id: number; name: string; rule_type: string; order_index: number; config_json: string; enabled: boolean } }
  | { ok: false; error: string }

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function dataField(payload: unknown, key: string): unknown {
  if (!isRecord(payload) || !isRecord(payload.data)) return undefined
  return payload.data[key]
}

function isNonNegativeSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

export function parseTransformRule(value: unknown): TransformRule | null {
  if (!isRecord(value)) return null
  if (typeof value.id !== 'number' || !Number.isSafeInteger(value.id) || value.id <= 0
    || typeof value.source_id !== 'number' || !Number.isSafeInteger(value.source_id) || value.source_id <= 0
    || typeof value.name !== 'string' || typeof value.rule_type !== 'string'
    || typeof value.order_index !== 'number' || !Number.isSafeInteger(value.order_index)
    || typeof value.config_json !== 'string' || typeof value.enabled !== 'boolean'
    || (value.has_secret !== undefined && typeof value.has_secret !== 'boolean')) return null
  return {
    id: value.id, source_id: value.source_id, name: value.name, rule_type: value.rule_type,
    order_index: value.order_index, config_json: value.config_json, enabled: value.enabled,
    has_secret: typeof value.has_secret === 'boolean' ? value.has_secret : undefined,
  }
}

export function parseTransformRulePage(payload: unknown): MonitoringPage<TransformRule> | null {
  if (!isRecord(payload) || !isRecord(payload.data)) return null
  const { rules, pagination } = payload.data
  if (!Array.isArray(rules) || !isRecord(pagination)) return null
  if (!isNonNegativeSafeInteger(pagination.page) || pagination.page < 1
    || !isNonNegativeSafeInteger(pagination.page_size) || pagination.page_size < 1 || pagination.page_size > 100
    || !isNonNegativeSafeInteger(pagination.total)
    || !isNonNegativeSafeInteger(pagination.total_pages)) return null
  const list = rules.map(parseTransformRule)
  if (!list.every((item): item is TransformRule => item !== null)) return null
  return {
    list,
    pagination: {
      page: pagination.page,
      pageSize: pagination.page_size,
      total: pagination.total,
      totalPages: pagination.total_pages,
    },
  }
}

export function parseLegacyTransformRules(payload: unknown): TransformRule[] | null {
  if (isRecord(payload) && isRecord(payload.data) && Object.prototype.hasOwnProperty.call(payload.data, 'pagination')) return null
  const list = dataField(payload, 'rules')
  if (!Array.isArray(list)) return null
  const rules = list.map(parseTransformRule)
  return rules.every((item): item is TransformRule => item !== null) ? rules : null
}

export function parseTransformRuleDetail(payload: unknown): TransformRule | null {
  return parseTransformRule(dataField(payload, 'rule'))
}

function containsPlaceholder(value: unknown): boolean {
  if (value === '[已隐藏]') return true
  if (Array.isArray(value)) return value.some(containsPlaceholder)
  return isRecord(value) && Object.values(value).some(containsPlaceholder)
}

export function buildRuleSavePayload(draft: RuleDraft): RuleSaveResult {
  const sourceID = Number(draft.sourceID)
  const orderIndex = Number(draft.orderIndex)
  if (!Number.isSafeInteger(sourceID) || sourceID <= 0 || !draft.name.trim() || !Number.isSafeInteger(orderIndex)) {
    return { ok: false, error: '请填写来源、名称和有效的顺序号。' }
  }
  try {
    const config = JSON.parse(draft.configJSON) as unknown
    if (draft.id === null && containsPlaceholder(config)) return { ok: false, error: '新增规则不能使用“[已隐藏]”占位符，请填写真实配置值。' }
  } catch {
    return { ok: false, error: '配置必须是有效 JSON。' }
  }
  return { ok: true, payload: { source_id: sourceID, name: draft.name.trim(), rule_type: draft.ruleType, order_index: orderIndex, config_json: draft.configJSON, enabled: draft.enabled } }
}

export function ruleDraftFrom(rule: TransformRule): RuleDraft {
  return { id: rule.id, sourceID: String(rule.source_id), name: rule.name, ruleType: rule.rule_type, orderIndex: String(rule.order_index), configJSON: rule.config_json || '{}', enabled: rule.enabled, hasSecret: Boolean(rule.has_secret) }
}

export function newRuleDraft(sourceID: number | undefined): RuleDraft {
  return { id: null, sourceID: sourceID ? String(sourceID) : '', name: '', ruleType: 'mapping', orderIndex: '0', configJSON: '{\n  "table_name": "",\n  "business_key_field": "",\n  "fields": []\n}', enabled: true, hasSecret: false }
}

export function parseRuleTestContent(rawContent: string, configJSON: string): { ok: true; rawContent: Record<string, unknown> } | { ok: false; error: string } {
  try {
    const content = JSON.parse(rawContent) as unknown
    JSON.parse(configJSON)
    if (!isRecord(content)) return { ok: false, error: '测试原始内容必须是 JSON 对象。' }
    return { ok: true, rawContent: content }
  } catch {
    return { ok: false, error: '测试内容和规则配置都必须是有效 JSON。' }
  }
}

export function readRuleTestResult(payload: unknown): unknown | null {
  const value = dataField(payload, 'clean_content')
  return isRecord(value) ? value : null
}
