import type { ClientResponse } from '../../api/client'
import type { PipelineStepDetail, PipelineSummary } from '../../pipelineComposer'
import type { TransformRule } from '../ruleContracts'
import type { DeliveryTask, DestinationDefinition, SourceDefinition } from '../types'

export type MethodKind = 'configured' | 'builtin'

export type ToggleTarget =
  | { type: 'source'; id: number }
  | { type: 'transform_rule'; id: number }
  | { type: 'destination'; id: number }
  | { type: 'delivery_task'; id: number }

export type MethodDisplay = {
  key: string
  kind: MethodKind
  name: string
  code: string
  methodType: string
  category: string
  owner: string
  description: string
  enabled: boolean
  toggle?: ToggleTarget
}

export type CoreMethod = {
  key: string
  title: string
  category: string
  description: string
  enabled: boolean
  status: string
  refs: ToggleTarget[]
}

export type LegacyTask = {
  code: string
  name: string
  category: string
  source_code: string
  source_name: string
  cron_expr: string
  input_table: string
  output_table: string
  target_system: string
  description: string
}

export type LegacyTransformRule = {
  code: string
  name: string
  source_code: string
  source_name: string
  rule_type: string
  trigger_mode: string
  input_table: string
  output_table: string
  description: string
}

const builtins: MethodDisplay[] = [
  { key: 'builtin-md32', kind: 'builtin', name: 'MD32 验证', code: 'md32_verify', methodType: 'utility', category: '前置验证方法', owner: '系统内置', description: '对传入字段生成或校验 32 位 MD5 摘要。', enabled: true },
  { key: 'builtin-uniqstr', kind: 'builtin', name: '生成唯一 uniqstr', code: 'generate_uniqstr', methodType: 'utility', category: '内置工具方法', owner: '系统内置', description: '生成可用于幂等键、批次号或外部请求追踪的唯一字符串。', enabled: true },
  { key: 'builtin-http-push', kind: 'builtin', name: 'HTTP 推送方法', code: 'http_delivery', methodType: 'delivery', category: '推送方法', owner: '系统内置', description: '根据推送目标配置发送 HTTP 请求并记录响应。', enabled: true },
  { key: 'builtin-api-pull', kind: 'builtin', name: 'API 拉取方法', code: 'api_poll', methodType: 'request', category: '数据拉取方法', owner: '系统内置', description: '按数据源配置请求第三方 API 并落原始数据。', enabled: true },
  { key: 'builtin-bojun-signed-request', kind: 'builtin', name: '伯俊签名请求', code: 'bojun_signed_request', methodType: 'bojun_signed_request', category: '数据拉取方法', owner: '系统内置', description: 'Go 系统方法 pkg/bojun.SendSignedRequest，凭据从 BOJUN_* 环境变量读取。', enabled: true },
  ...[
    ['builtin-push-jialicheng', '推送嘉里城', 'push_jialicheng', 'jialicheng'],
    ['builtin-push-panlong', '推送蟠龙', 'push_panlong', 'panlong'],
    ['builtin-push-qiantan', '推送前滩', 'push_qiantan', 'qiantan'],
    ['builtin-push-shangsheng', '推送上生新所', 'push_shangsheng', 'shangsheng'],
    ['builtin-push-xintiandi', '推送新天地', 'push_xintiandi', 'xintiandi'],
  ].map(([key, name, code, target]) => ({
    key, kind: 'builtin' as const, name, code, methodType: 'shanghai_mall_push', category: '商场推送方法', owner: '系统内置',
    description: `Go 系统方法 pkg/shanghaimall.Push，target=${target}，按正常单、换货和退货生成对应商场请求。`, enabled: true,
  })),
]

export const builtinMethods = Object.freeze(builtins)

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function dataList(payload: unknown, key: string): unknown[] | null {
  if (!isRecord(payload) || payload.code !== 200 || !isRecord(payload.data) || !Array.isArray(payload.data[key])) return null
  return payload.data[key]
}

function requiredString(value: unknown): value is string {
  return typeof value === 'string' && value.length <= 4096
}

export function parsePipelineSummaries(payload: unknown): PipelineSummary[] | null {
  const list = dataList(payload, 'pipelines')
  if (!list) return null
  const result: PipelineSummary[] = []
  for (const value of list) {
    if (!isRecord(value) || typeof value.id !== 'number' || !Number.isSafeInteger(value.id) || value.id <= 0
      || !requiredString(value.name) || !requiredString(value.code) || typeof value.enabled !== 'boolean'
      || (value.description !== undefined && typeof value.description !== 'string')) return null
    result.push({ id: value.id, name: value.name, code: value.code, description: typeof value.description === 'string' ? value.description : '', enabled: value.enabled })
  }
  return result
}

export function parseLegacyTasks(payload: unknown): LegacyTask[] | null {
  const list = dataList(payload, 'tasks')
  if (!list) return null
  const fields: Array<keyof LegacyTask> = ['code', 'name', 'category', 'source_code', 'source_name', 'cron_expr', 'input_table', 'output_table', 'target_system', 'description']
  const result: LegacyTask[] = []
  for (const value of list) {
    if (!isRecord(value) || !fields.every((field) => requiredString(value[field]))) return null
    result.push(Object.fromEntries(fields.map((field) => [field, value[field]])) as LegacyTask)
  }
  return result
}

export function parseLegacyTransformRules(payload: unknown): LegacyTransformRule[] | null {
  const list = dataList(payload, 'rules')
  if (!list) return null
  const fields: Array<keyof LegacyTransformRule> = ['code', 'name', 'source_code', 'source_name', 'rule_type', 'trigger_mode', 'input_table', 'output_table', 'description']
  const result: LegacyTransformRule[] = []
  for (const value of list) {
    if (!isRecord(value) || !fields.every((field) => requiredString(value[field]))) return null
    result.push(Object.fromEntries(fields.map((field) => [field, value[field]])) as LegacyTransformRule)
  }
  return result
}

export function methodCategory(methodType: string): string {
  const categories: Record<string, string> = {
    request: '数据拉取方法', bojun_signed_request: '数据拉取方法', extract: '数据拉取方法', mapping: '数据处理方法',
    validate: '前置验证方法', db_query: '数据处理方法', db_write: '数据处理方法', template: '数据推送方法',
    delivery: '推送方法', shanghai_mall_push: '商场推送方法', log: '日志方法', utility: '内置工具方法',
  }
  return categories[methodType] ?? '其它方法'
}

export function methodTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    request: 'Request 请求', bojun_signed_request: '伯俊签名请求', extract: 'Extract 提取', mapping: 'Mapping 清洗', validate: 'Validate 校验',
    db_query: 'DB Query 查询', db_write: 'DB Write 写入', template: 'Template 模板', delivery: 'Delivery 推送',
    shanghai_mall_push: '上海商场推送', log: 'Log 记录', utility: 'Utility 工具',
  }
  return labels[type] ?? type
}

export function pipelineStepMethodDisplay(detail: PipelineStepDetail, owner: string): MethodDisplay {
  return {
    key: `configured-${detail.step.id}`, kind: 'configured', name: detail.step.name, code: detail.step.code,
    methodType: detail.step.method_type, category: methodCategory(detail.step.method_type), owner,
    description: `${methodTypeLabel(detail.step.method_type)}，入参 ${detail.params.length} 个，出参 ${detail.outputs.length} 个。`, enabled: detail.step.enabled,
  }
}

export function buildConfiguredMethodDisplays(sources: SourceDefinition[], rules: TransformRule[], destinations: DestinationDefinition[], tasks: DeliveryTask[]): MethodDisplay[] {
  return [
    ...sources.map((source) => ({ key: `source-${source.id}`, kind: 'configured' as const, name: source.name, code: source.code, methodType: 'request', category: isYouzanText(`${source.name} ${source.code}`) ? '有赞数据拉取' : '数据拉取方法', owner: '数据源配置', description: `${source.source_type} 数据源，接收键 ${source.source_query_key || '-'}。`, enabled: source.enabled, toggle: { type: 'source' as const, id: source.id } })),
    ...rules.map((rule) => ({ key: `rule-${rule.id}`, kind: 'configured' as const, name: rule.name, code: `transform_rule_${rule.id}`, methodType: rule.rule_type === 'validator' ? 'validate' : 'mapping', category: isQimaiText(`${rule.name} ${rule.config_json}`) ? '企迈数据处理' : '数据处理方法', owner: '清洗规则配置', description: `${rule.rule_type} 规则，source #${rule.source_id}，顺序 ${rule.order_index}。`, enabled: rule.enabled, toggle: { type: 'transform_rule' as const, id: rule.id } })),
    ...destinations.map((destination) => ({ key: `destination-${destination.id}`, kind: 'configured' as const, name: destination.name, code: destination.code, methodType: 'delivery', category: isMallText(`${destination.name} ${destination.code}`) ? '商场数据推送' : '推送方法', owner: '推送目标配置', description: `${destination.destination_type} 推送目标。`, enabled: destination.enabled, toggle: { type: 'destination' as const, id: destination.id } })),
    ...tasks.map((task) => ({ key: `delivery-task-${task.id}`, kind: 'configured' as const, name: task.name, code: `delivery_task_${task.id}`, methodType: 'delivery', category: isMallText(`${task.name} ${task.clean_table}`) ? '商场数据推送' : '推送方法', owner: '推送任务配置', description: `${task.clean_table} -> destination #${task.destination_id}，触发方式 ${task.trigger_type}。`, enabled: task.enabled, toggle: { type: 'delivery_task' as const, id: task.id } })),
  ]
}

export function buildLegacyMethodDisplays(tasks: LegacyTask[], rules: LegacyTransformRule[]): MethodDisplay[] {
  return [
    ...tasks.map((task) => ({ key: `legacy-task-${task.code}`, kind: 'builtin' as const, name: task.name, code: task.code, methodType: task.category === 'delivery' ? 'delivery' : task.category === 'process' ? 'mapping' : 'request', category: legacyCategory(task), owner: '旧任务注册表', description: task.description, enabled: true })),
    ...rules.map((rule) => ({ key: `legacy-rule-${rule.code}`, kind: 'builtin' as const, name: rule.name, code: rule.code, methodType: rule.rule_type === 'http_enrich' ? 'request' : 'mapping', category: rule.source_code === 'qimai_order' ? '企迈数据处理' : '数据处理方法', owner: '旧清洗规则', description: rule.description, enabled: true })),
  ]
}

export function buildCoreMethods({ sources, transformRules, destinations, deliveryTasks, legacyTasks, legacyRules }: { sources: SourceDefinition[]; transformRules: TransformRule[]; destinations: DestinationDefinition[]; deliveryTasks: DeliveryTask[]; legacyTasks: LegacyTask[]; legacyRules: LegacyTransformRule[] }): CoreMethod[] {
  const youzanSources = sources.filter((source) => isYouzanText(`${source.name} ${source.code} ${source.source_type}`))
  const youzanLegacy = legacyTasks.filter((task) => task.category === 'fetch' && isYouzanText(`${task.name} ${task.code} ${task.source_code}`))
  const qimaiRules = transformRules.filter((rule) => isQimaiText(`${rule.name} ${rule.config_json}`))
  const qimaiLegacy = [...legacyTasks.filter((task) => task.code === 'qimai_order_enrich'), ...legacyRules.filter((rule) => rule.code === 'qimai_order_http_enrich')]
  const mallDestinations = destinations.filter((destination) => isMallText(`${destination.name} ${destination.code} ${destination.config_json}`))
  const mallTasks = deliveryTasks.filter((task) => isMallText(`${task.name} ${task.clean_table} ${task.payload_template}`) || mallDestinations.some((destination) => destination.id === task.destination_id))
  const mallLegacy = legacyTasks.filter((task) => task.category === 'delivery' && isMallText(`${task.name} ${task.target_system} ${task.output_table}`))
  return [
    { key: 'interface_ingest', title: '接口数据接收', category: '数据接收方法', description: '对方通过数据接收接口发送数据，系统保存原始数据并进入后续处理。', enabled: true, status: '接口入口已存在，无单独配置开关。', refs: [] },
    { key: 'youzan_fetch', title: '有赞数据拉取', category: '数据拉取方法', description: '现有有赞订单和退款拉取能力，包含旧任务和数据源配置。', enabled: youzanSources.length > 0 ? youzanSources.some((source) => source.enabled) : youzanLegacy.length > 0, status: youzanSources.length > 0 ? `${youzanSources.length} 个数据源配置` : `${youzanLegacy.length} 个旧任务注册`, refs: youzanSources.map((source) => ({ type: 'source', id: source.id })) },
    { key: 'bojun_order_fetch', title: '伯俊订单拉取', category: '数据拉取方法', description: '定时拉取伯俊订单，也支持按时间范围手动补拉。', enabled: true, status: '系统定时任务执行；补拉不会覆盖已存在订单。', refs: [] },
    { key: 'qimai_process', title: '企迈标签数据处理', category: '数据处理方法', description: '处理带企迈标签的原始数据并写入清洗结果。', enabled: qimaiRules.length > 0 ? qimaiRules.some((rule) => rule.enabled) : qimaiLegacy.length > 0, status: qimaiRules.length > 0 ? `${qimaiRules.length} 条清洗规则配置` : `${qimaiLegacy.length} 条旧处理规则`, refs: qimaiRules.map((rule) => ({ type: 'transform_rule', id: rule.id })) },
    { key: 'mall_push', title: '商场数据推送', category: '数据推送方法', description: '推送商场销售数据；启用状态同时取决于目标和任务。', enabled: [...mallDestinations, ...mallTasks].length > 0 ? mallDestinations.some((destination) => destination.enabled) && mallTasks.some((task) => task.enabled) : mallLegacy.length > 0, status: mallTasks.length > 0 ? `${mallTasks.length} 个推送任务，${mallDestinations.length} 个推送目标` : `${mallLegacy.length} 个旧推送任务`, refs: [...mallDestinations.map((destination) => ({ type: 'destination' as const, id: destination.id })), ...mallTasks.map((task) => ({ type: 'delivery_task' as const, id: task.id }))] },
  ]
}

export function permissionForToggle(target: ToggleTarget): string {
  if (target.type === 'source') return 'source.manage'
  if (target.type === 'transform_rule') return 'pipeline.manage'
  return 'delivery.manage'
}

export function canToggleTarget(target: ToggleTarget, permissions: readonly string[]): boolean {
  return permissions.includes(permissionForToggle(target))
}

export function updateTargetEnabled(client: (path: string, options: { method: 'PATCH'; body: unknown; showResult?: boolean; silentLoading?: boolean }) => Promise<ClientResponse>, target: ToggleTarget, enabled: boolean): Promise<ClientResponse> {
  const roots: Record<ToggleTarget['type'], string> = {
    source: '/v1/sources',
    transform_rule: '/v1/transform-rules',
    destination: '/v1/destinations',
    delivery_task: '/v1/delivery-tasks',
  }
  return client(`${roots[target.type]}/${target.id}/enabled`, { method: 'PATCH', showResult: false, silentLoading: true, body: { enabled } })
}

function legacyCategory(task: LegacyTask): string {
  if (task.category === 'fetch' && isYouzanText(`${task.name} ${task.code}`)) return '有赞数据拉取'
  if (task.category === 'process' && isQimaiText(`${task.name} ${task.code}`)) return '企迈数据处理'
  if (task.category === 'delivery' && isMallText(`${task.name} ${task.target_system}`)) return '商场数据推送'
  return methodCategory(task.category === 'delivery' ? 'delivery' : task.category === 'process' ? 'mapping' : 'request')
}

function isYouzanText(value: string): boolean { return /youzan|有赞/i.test(value) }
function isQimaiText(value: string): boolean { return /qimai|企迈/i.test(value) }
function isMallText(value: string): boolean { return /商场|商城|mall|henglong|恒隆|西岸|xian|plaza/i.test(value) }
