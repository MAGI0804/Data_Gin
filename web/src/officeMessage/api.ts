import type { WorkspaceApiClient } from '../appShell/WorkspaceRouter'
import {
  parseOfficeMessage,
  parseOfficeMessages,
  parseOfficeFeishuBots,
  parseOfficeRuns,
  parseOfficeTargets,
  parseProcedureSummaries,
  parseResultColumns,
  parseResultTableSummaries,
  parseSelectColumns,
} from './contracts'
import type { OfficeMessageDraft, OfficePushTarget, OfficeQueryParameter } from './types'

type OfficeResult<T> = { ok: true; data: T } | { ok: false; error: string }

async function request<T>(operation: () => Promise<{ ok: boolean; data: unknown; error?: { message: string } }>, parser: (payload: unknown) => T, fallback: string): Promise<OfficeResult<T>> {
  try {
    const response = await operation()
    if (!response.ok) return { ok: false, error: response.error?.message || fallback }
    return { ok: true, data: parser(response.data) }
  } catch {
    return { ok: false, error: fallback }
  }
}

const quiet = { showResult: false, silentLoading: true, acceptSafeErrorMessage: true }

export function getOfficeMessages(client: WorkspaceApiClient, signal?: AbortSignal) {
  return request(() => client('/v1/office-messages', { method: 'GET', signal, ...quiet }), parseOfficeMessages, '办公消息列表加载失败。')
}

export function saveOfficeMessage(client: WorkspaceApiClient, draft: OfficeMessageDraft, body: unknown) {
  return request(() => client(draft.id ? `/v1/office-messages/${draft.id}` : '/v1/office-messages', { method: draft.id ? 'PUT' : 'POST', body, ...quiet }), parseOfficeMessage, '办公消息保存失败。')
}

export async function deleteOfficeMessage(client: WorkspaceApiClient, id: number, lockVersion: number): Promise<OfficeResult<{ id: number }>> {
  const query = new URLSearchParams({ expectedLockVersion: String(lockVersion) })
  const response = await client(`/v1/office-messages/${id}?${query}`, { method: 'DELETE', ...quiet })
  return response.ok ? { ok: true, data: { id } } : { ok: false, error: response.error?.message || '办公消息删除失败。' }
}

export function getOfficeTargets(client: WorkspaceApiClient, signal?: AbortSignal) {
  return request(() => client('/v1/office-push-targets', { method: 'GET', signal, ...quiet }), parseOfficeTargets, '推送配置加载失败。')
}

export function getOfficeFeishuBots(client: WorkspaceApiClient, signal?: AbortSignal) {
  return request(() => client('/v1/office-feishu-bots', { method: 'GET', signal, ...quiet }), parseOfficeFeishuBots, '飞书机器人加载失败。')
}

export async function saveOfficeTarget(client: WorkspaceApiClient, target: { id?: number | null }, body: unknown): Promise<OfficeResult<OfficePushTarget>> {
  return request(() => client(target.id ? `/v1/office-push-targets/${target.id}` : '/v1/office-push-targets', { method: target.id ? 'PUT' : 'POST', body, ...quiet }), (payload) => {
    const parsed = parseOfficeTargets({ items: [payloadData(payload)] })
    if (parsed.length !== 1) throw new Error('invalid target')
    return parsed[0]
  }, '推送配置保存失败。')
}

export async function deleteOfficeTarget(client: WorkspaceApiClient, id: number, lockVersion: number): Promise<OfficeResult<{ id: number }>> {
  const query = new URLSearchParams({ expectedLockVersion: String(lockVersion) })
  const response = await client(`/v1/office-push-targets/${id}?${query}`, { method: 'DELETE', ...quiet })
  return response.ok ? { ok: true, data: { id } } : { ok: false, error: response.error?.message || '推送配置删除失败。' }
}

export function getOfficeRuns(client: WorkspaceApiClient, signal?: AbortSignal) {
  return request(() => client('/v1/office-push-runs?limit=100', { method: 'GET', signal, ...quiet }), parseOfficeRuns, '推送记录加载失败。')
}

export async function createOfficeRun(client: WorkspaceApiClient, targetId: number, requestId: string, parameters: Record<string, string>): Promise<OfficeResult<{ id: number }>> {
  const response = await client(`/v1/office-push-targets/${targetId}/runs`, { method: 'POST', body: { requestId, parameters }, ...quiet })
  if (!response.ok) return { ok: false, error: response.error?.message || '推送任务创建失败。' }
  const id = Number(payloadData(response.data).id)
  return Number.isInteger(id) && id > 0 ? { ok: true, data: { id } } : { ok: false, error: '推送任务响应格式无效。' }
}

export function searchOfficeProcedures(client: WorkspaceApiClient, owner: string, search: string) {
  const query = new URLSearchParams({ owner, search, limit: '50' })
  return request(() => client(`/v1/office-oracle/procedures?${query}`, { method: 'GET', ...quiet }), parseProcedureSummaries, 'Oracle 存储过程查询失败。')
}

export function searchOfficeResultTables(client: WorkspaceApiClient, owner: string, search: string) {
  const query = new URLSearchParams({ owner, search, limit: '50' })
  return request(() => client(`/v1/office-oracle/result-tables?${query}`, { method: 'GET', ...quiet }), parseResultTableSummaries, 'Oracle 结果表查询失败。')
}

export function getOfficeResultColumns(client: WorkspaceApiClient, owner: string, name: string) {
  const query = new URLSearchParams({ owner, name })
  return request(() => client(`/v1/office-oracle/result-table-schema?${query}`, { method: 'GET', ...quiet }), parseResultColumns, 'Oracle 结果表字段读取失败。')
}

export function testOfficeSelect(client: WorkspaceApiClient, selectSql: string, parameters: OfficeQueryParameter[], values: Record<string, string>) {
  return request(() => client('/v1/office-oracle/select-tests', { method: 'POST', body: { selectSql, parameters, values }, timeoutMs: 35_000, ...quiet }), parseSelectColumns, 'SELECT 测试失败，请检查 SQL 与测试参数。')
}

function payloadData(payload: unknown): Record<string, unknown> {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) throw new Error('invalid office payload')
  const root = payload as Record<string, unknown>
  const value = root.data ?? root
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid office payload')
  return value as Record<string, unknown>
}
