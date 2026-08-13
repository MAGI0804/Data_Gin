import type { MonitoringPage } from '../monitoringRecords'
import type { DestinationDefinition, DestinationDraft } from './types'

type DestinationSavePayload = {
  name: string
  code: string
  destination_type: string
  config_json: string
  enabled: boolean
}

type DestinationDraftValidation =
  | { ok: true; payload: DestinationSavePayload }
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

export function parseDestinationDefinition(value: unknown): DestinationDefinition | null {
  if (!isRecord(value)) return null
  if (typeof value.id !== 'number' || !Number.isSafeInteger(value.id) || value.id <= 0
    || typeof value.name !== 'string'
    || typeof value.code !== 'string'
    || typeof value.destination_type !== 'string'
    || typeof value.config_json !== 'string'
    || typeof value.enabled !== 'boolean'
    || (value.has_secret !== undefined && typeof value.has_secret !== 'boolean')) return null

  return {
    id: value.id,
    name: value.name,
    code: value.code,
    destination_type: value.destination_type,
    config_json: value.config_json,
    has_secret: typeof value.has_secret === 'boolean' ? value.has_secret : undefined,
    enabled: value.enabled,
  }
}

export function parseDestinationPage(payload: unknown): MonitoringPage<DestinationDefinition> | null {
  if (!isRecord(payload) || !isRecord(payload.data)) return null
  const { destinations, pagination } = payload.data
  if (!Array.isArray(destinations) || !isRecord(pagination)) return null
  if (!isNonNegativeSafeInteger(pagination.page) || pagination.page < 1
    || !isNonNegativeSafeInteger(pagination.page_size) || pagination.page_size < 1 || pagination.page_size > 100
    || !isNonNegativeSafeInteger(pagination.total)
    || !isNonNegativeSafeInteger(pagination.total_pages)) return null

  const list = destinations.map(parseDestinationDefinition)
  if (!list.every((destination): destination is DestinationDefinition => destination !== null)) return null
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

export function parseLegacyDestinationList(payload: unknown): DestinationDefinition[] | null {
  if (!isRecord(payload) || !isRecord(payload.data)) return null
  if (Object.prototype.hasOwnProperty.call(payload.data, 'pagination')) return null
  const destinations = payload.data.destinations
  if (!Array.isArray(destinations)) return null
  const list = destinations.map(parseDestinationDefinition)
  return list.every((destination): destination is DestinationDefinition => destination !== null) ? list : null
}

export function parseDestinationDetail(payload: unknown): DestinationDefinition | null {
  return parseDestinationDefinition(dataField(payload, 'destination'))
}

export function parseDestinationTestResult(payload: unknown, expectedDestinationID: number): boolean {
  if (!isRecord(payload) || !isRecord(payload.data)) return false
  return payload.data.destination_id === expectedDestinationID
    && typeof payload.data.destination_id === 'number'
    && Number.isSafeInteger(payload.data.destination_id)
    && payload.data.destination_id > 0
    && payload.data.status === 'success'
}

function containsRedactedPlaceholder(value: unknown): boolean {
  if (value === '[已隐藏]') return true
  if (Array.isArray(value)) return value.some(containsRedactedPlaceholder)
  return isRecord(value) && Object.values(value).some(containsRedactedPlaceholder)
}

export function buildDestinationSavePayload(draft: DestinationDraft): DestinationDraftValidation {
  const name = draft.name.trim()
  const code = draft.code.trim()
  if (!name || !code) return { ok: false, error: '请填写目标名称和编码。' }

  try {
    const config = JSON.parse(draft.configJSON) as unknown
    if (!isRecord(config)) return { ok: false, error: '配置必须为 JSON 对象。' }
    if (draft.id === null && containsRedactedPlaceholder(config)) {
      return { ok: false, error: '新增推送目标不能使用“[已隐藏]”占位符，请填写真实配置值。' }
    }
  } catch {
    return { ok: false, error: '配置必须为 JSON 对象。' }
  }

  return {
    ok: true,
    payload: {
      name,
      code,
      destination_type: draft.destinationType,
      config_json: draft.configJSON,
      enabled: draft.enabled,
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

export function destinationDraftFrom(destination: DestinationDefinition): DestinationDraft {
  return {
    id: destination.id,
    name: destination.name,
    code: destination.code,
    destinationType: destination.destination_type,
    configJSON: normalizedJSONText(destination.config_json, '{}'),
    enabled: destination.enabled,
    hasSecret: Boolean(destination.has_secret),
  }
}

export function newDestinationDraft(): DestinationDraft {
  return {
    id: null,
    name: '',
    code: '',
    destinationType: 'http',
    configJSON: '{\n  "url": "",\n  "method": "POST"\n}',
    enabled: true,
    hasSecret: false,
  }
}
