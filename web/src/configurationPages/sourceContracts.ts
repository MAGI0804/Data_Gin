import { parseMonitoringPage, type MonitoringPage } from '../monitoringRecords'
import type { SourceDefinition, SourceDraft } from './types'

type SourceSavePayload = {
  name: string
  code: string
  source_type: string
  enabled: boolean
  auth_type: string
  config_json: string
  schema_json: string
  dedupe_keys: string
  source_query_key: string
}

type SourceDraftValidation =
  | { ok: true; payload: SourceSavePayload }
  | { ok: false; error: string }

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function dataField(payload: unknown, key: string): unknown {
  if (!isRecord(payload) || !isRecord(payload.data)) return undefined
  return payload.data[key]
}

export function parseSourceDefinition(value: unknown): SourceDefinition | null {
  if (!isRecord(value)) return null
  const id = value.id
  if (typeof id !== 'number' || !Number.isSafeInteger(id) || id <= 0
    || typeof value.name !== 'string'
    || typeof value.code !== 'string'
    || typeof value.source_type !== 'string'
    || typeof value.enabled !== 'boolean'
    || typeof value.auth_type !== 'string'
    || typeof value.config_json !== 'string'
    || typeof value.schema_json !== 'string'
    || typeof value.dedupe_keys !== 'string'
    || typeof value.source_query_key !== 'string'
    || (value.has_secret !== undefined && typeof value.has_secret !== 'boolean')) return null

  return {
    id,
    name: value.name,
    code: value.code,
    source_type: value.source_type,
    enabled: value.enabled,
    auth_type: value.auth_type,
    config_json: value.config_json,
    has_secret: typeof value.has_secret === 'boolean' ? value.has_secret : undefined,
    schema_json: value.schema_json,
    dedupe_keys: value.dedupe_keys,
    source_query_key: value.source_query_key,
  }
}

export function parseSourcePage(payload: unknown): MonitoringPage<SourceDefinition> | null {
  const page = parseMonitoringPage<unknown>(payload, 'sources')
  if (!page) return null
  const list = page.list.map(parseSourceDefinition)
  if (!list.every((source): source is SourceDefinition => source !== null)) return null
  return { ...page, list }
}

export function parseLegacySourceList(payload: unknown): SourceDefinition[] | null {
  if (isRecord(payload) && isRecord(payload.data) && Object.prototype.hasOwnProperty.call(payload.data, 'pagination')) return null
  const list = dataField(payload, 'sources')
  if (!Array.isArray(list)) return null
  const sources = list.map(parseSourceDefinition)
  return sources.every((source): source is SourceDefinition => source !== null) ? sources : null
}

export function parseSourceDetail(payload: unknown): SourceDefinition | null {
  return parseSourceDefinition(dataField(payload, 'source'))
}

function jsonValue(text: string): unknown {
  return JSON.parse(text) as unknown
}

function containsRedactedPlaceholder(value: unknown): boolean {
  if (value === '[已隐藏]') return true
  if (Array.isArray(value)) return value.some(containsRedactedPlaceholder)
  return isRecord(value) && Object.values(value).some(containsRedactedPlaceholder)
}

export function buildSourceSavePayload(draft: SourceDraft): SourceDraftValidation {
  const name = draft.name.trim()
  const code = draft.code.trim()
  if (!name || !code) return { ok: false, error: '请填写数据源名称和编码。' }

  try {
    const config = jsonValue(draft.configJSON)
    const schema = jsonValue(draft.schemaJSON)
    const dedupeKeys = jsonValue(draft.dedupeKeys)
    if (!isRecord(config) || !isRecord(schema) || !Array.isArray(dedupeKeys)) {
      return { ok: false, error: '配置和 Schema 必须为 JSON 对象，去重键必须为 JSON 数组。' }
    }
    if (draft.id === null && containsRedactedPlaceholder(config)) {
      return { ok: false, error: '新增数据源不能使用“[已隐藏]”占位符，请填写真实配置值。' }
    }
  } catch {
    return { ok: false, error: '配置和 Schema 必须为 JSON 对象，去重键必须为 JSON 数组。' }
  }

  return {
    ok: true,
    payload: {
      name,
      code,
      source_type: draft.sourceType,
      enabled: draft.enabled,
      auth_type: draft.authType.trim() || 'none',
      config_json: draft.configJSON,
      schema_json: draft.schemaJSON,
      dedupe_keys: draft.dedupeKeys,
      source_query_key: draft.sourceQueryKey.trim(),
    },
  }
}

function normalizedJSONText(value: string, fallback: string): string {
  if (!value) return fallback
  try {
    return JSON.stringify(JSON.parse(value) as unknown, null, 2)
  } catch {
    return value
  }
}

export function sourceDraftFrom(source: SourceDefinition): SourceDraft {
  return {
    id: source.id,
    name: source.name,
    code: source.code,
    sourceType: source.source_type,
    enabled: source.enabled,
    authType: source.auth_type,
    configJSON: normalizedJSONText(source.config_json, '{}'),
    schemaJSON: normalizedJSONText(source.schema_json, '{}'),
    dedupeKeys: normalizedJSONText(source.dedupe_keys, '[]'),
    sourceQueryKey: source.source_query_key,
    hasSecret: Boolean(source.has_secret),
  }
}

export function newSourceDraft(): SourceDraft {
  return {
    id: null,
    name: '',
    code: '',
    sourceType: 'api_poll',
    enabled: true,
    authType: 'none',
    configJSON: '{\n  "url": "",\n  "method": "GET",\n  "records_path": "data"\n}',
    schemaJSON: '{}',
    dedupeKeys: '[]',
    sourceQueryKey: '',
    hasSecret: false,
  }
}
