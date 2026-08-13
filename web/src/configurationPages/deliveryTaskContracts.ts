import type { MonitoringPage } from '../monitoringRecords'
import type { DeliveryTask, DeliveryTaskDraft } from './types'

type DeliveryTaskSaveResult =
  | { ok: true; payload: { name: string; source_id: number; clean_table: string; destination_id: number; trigger_type: DeliveryTask['trigger_type']; cron_expr: string; filter_json: string; payload_template: string; enabled: boolean } }
  | { ok: false; error: string }

export type DeliveryRunResult = {
  traceID: string
  totalCount: number
  successCount: number
  failedCount: number
  skippedCount: number
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function dataField(payload: unknown, key: string): unknown {
  if (!isRecord(payload) || !isRecord(payload.data)) return undefined
  return payload.data[key]
}

function isPositiveSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

function isNonNegativeSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

function isTriggerType(value: unknown): value is DeliveryTask['trigger_type'] {
  return value === 'manual' || value === 'schedule' || value === 'event'
}

export function parseDeliveryTask(value: unknown): DeliveryTask | null {
  if (!isRecord(value)
    || !isPositiveSafeInteger(value.id)
    || typeof value.name !== 'string'
    || !isPositiveSafeInteger(value.source_id)
    || typeof value.clean_table !== 'string'
    || !isPositiveSafeInteger(value.destination_id)
    || !isTriggerType(value.trigger_type)
    || typeof value.cron_expr !== 'string'
    || typeof value.filter_json !== 'string'
    || typeof value.payload_template !== 'string'
    || typeof value.enabled !== 'boolean') return null
  return {
    id: value.id,
    name: value.name,
    source_id: value.source_id,
    clean_table: value.clean_table,
    destination_id: value.destination_id,
    trigger_type: value.trigger_type,
    cron_expr: value.cron_expr,
    filter_json: value.filter_json,
    payload_template: value.payload_template,
    enabled: value.enabled,
  }
}

export function parseDeliveryTaskPage(payload: unknown): MonitoringPage<DeliveryTask> | null {
  if (!isRecord(payload) || !isRecord(payload.data)) return null
  const { tasks, pagination } = payload.data
  if (!Array.isArray(tasks) || !isRecord(pagination)
    || !isPositiveSafeInteger(pagination.page)
    || !isPositiveSafeInteger(pagination.page_size) || pagination.page_size > 100
    || !isNonNegativeSafeInteger(pagination.total)
    || !isNonNegativeSafeInteger(pagination.total_pages)) return null
  const list = tasks.map(parseDeliveryTask)
  if (!list.every((task): task is DeliveryTask => task !== null)) return null
  return { list, pagination: { page: pagination.page, pageSize: pagination.page_size, total: pagination.total, totalPages: pagination.total_pages } }
}

export function parseLegacyDeliveryTasks(payload: unknown): DeliveryTask[] | null {
  if (!isRecord(payload) || !isRecord(payload.data) || Object.prototype.hasOwnProperty.call(payload.data, 'pagination')) return null
  if (!Array.isArray(payload.data.tasks)) return null
  const tasks = payload.data.tasks.map(parseDeliveryTask)
  return tasks.every((task): task is DeliveryTask => task !== null) ? tasks : null
}

export function parseDeliveryTaskDetail(payload: unknown): DeliveryTask | null {
  return parseDeliveryTask(dataField(payload, 'task'))
}

export function buildDeliveryTaskSavePayload(draft: DeliveryTaskDraft): DeliveryTaskSaveResult {
  const name = draft.name.trim()
  const cleanTable = draft.cleanTable.trim()
  const sourceID = Number(draft.sourceID)
  const destinationID = Number(draft.destinationID)
  const cronExpr = draft.cronExpr.trim()
  if (!name || !cleanTable || !isPositiveSafeInteger(sourceID) || !isPositiveSafeInteger(destinationID)) {
    return { ok: false, error: '请填写任务名称、来源、清洗表和推送目标。' }
  }
  if (!isTriggerType(draft.triggerType)) return { ok: false, error: '请选择有效的触发方式。' }
  if (draft.triggerType === 'schedule' && !cronExpr) return { ok: false, error: '定时触发任务必须填写 Cron 表达式。' }
  try {
    const filter = JSON.parse(draft.filterJSON) as unknown
    if (!isRecord(filter)) return { ok: false, error: '筛选条件必须是 JSON 对象。' }
  } catch {
    return { ok: false, error: '筛选条件必须是有效 JSON。' }
  }
  return { ok: true, payload: { name, source_id: sourceID, clean_table: cleanTable, destination_id: destinationID, trigger_type: draft.triggerType, cron_expr: cronExpr, filter_json: draft.filterJSON, payload_template: draft.payloadTemplate, enabled: draft.enabled } }
}

export function deliveryTaskDraftFrom(task: DeliveryTask): DeliveryTaskDraft {
  return { id: task.id, name: task.name, sourceID: String(task.source_id), cleanTable: task.clean_table, destinationID: String(task.destination_id), triggerType: task.trigger_type, cronExpr: task.cron_expr, filterJSON: task.filter_json || '{}', payloadTemplate: task.payload_template, enabled: task.enabled }
}

export function newDeliveryTaskDraft(sourceID?: number, destinationID?: number): DeliveryTaskDraft {
  return { id: null, name: '', sourceID: sourceID ? String(sourceID) : '', cleanTable: '', destinationID: destinationID ? String(destinationID) : '', triggerType: 'manual', cronExpr: '', filterJSON: '{}', payloadTemplate: '', enabled: true }
}

export function parseDeliveryRunResult(payload: unknown): DeliveryRunResult | null {
  const value = dataField(payload, 'result')
  if (!isRecord(value)
    || typeof value.trace_id !== 'string' || !value.trace_id || value.trace_id.length > 64
    || !isNonNegativeSafeInteger(value.total_count)
    || !isNonNegativeSafeInteger(value.success_count)
    || !isNonNegativeSafeInteger(value.failed_count)
    || !isNonNegativeSafeInteger(value.skipped_count)
    || value.total_count !== value.success_count + value.failed_count
    || value.skipped_count > value.success_count) return null
  return { traceID: value.trace_id, totalCount: value.total_count, successCount: value.success_count, failedCount: value.failed_count, skippedCount: value.skipped_count }
}

export function deliveryTaskTriggerLabel(value: DeliveryTask['trigger_type']): string {
  return { manual: '手动', schedule: '定时', event: '事件' }[value]
}
