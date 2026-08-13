export type OrderPushPolicyTarget = { target_code: string; target_name: string; cycle: number; skip: number }
export type OrderPushPolicyConfig = { targets: OrderPushPolicyTarget[] }
export type OrderPushTargetOption = { code: string; name: string }
export type OrderPushPolicyDraft = { code: string; name: string; cycle: string; skip: string }

type PolicyPayloadResult = { ok: true; payload: OrderPushPolicyConfig } | { ok: false; error: string }

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function isNonNegativeSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

function parseTarget(value: unknown): OrderPushPolicyTarget | null {
  if (!isRecord(value) || typeof value.target_code !== 'string' || !value.target_code.trim() || typeof value.target_name !== 'string' || !isNonNegativeSafeInteger(value.cycle) || !isNonNegativeSafeInteger(value.skip)) return null
  if ((value.cycle === 0 && value.skip !== 0) || (value.cycle > 0 && value.skip >= value.cycle)) return null
  return { target_code: value.target_code.trim(), target_name: value.target_name.trim(), cycle: value.cycle, skip: value.skip }
}

function parseConfig(value: unknown): OrderPushPolicyConfig | null {
  if (!isRecord(value) || !Array.isArray(value.targets)) return null
  const targets = value.targets.map(parseTarget)
  if (!targets.every((target): target is OrderPushPolicyTarget => target !== null)) return null
  const codes = targets.map((target) => target.target_code.toLowerCase())
  return new Set(codes).size === codes.length ? { targets } : null
}

function parseOptions(value: unknown): OrderPushTargetOption[] | null {
  if (!Array.isArray(value)) return null
  const options = value.map((item): OrderPushTargetOption | null => {
    if (!isRecord(item) || typeof item.code !== 'string' || !item.code.trim() || typeof item.name !== 'string') return null
    return { code: item.code.trim(), name: item.name.trim() || item.code.trim() }
  })
  if (!options.every((option): option is OrderPushTargetOption => option !== null)) return null
  const codes = options.map((option) => option.code.toLowerCase())
  return new Set(codes).size === codes.length ? options : null
}

export function parseOrderPushPolicyEnvelope(payload: unknown): { config: OrderPushPolicyConfig; options: OrderPushTargetOption[] } | null {
  if (!isRecord(payload) || !isRecord(payload.data)) return null
  const config = parseConfig(payload.data.config)
  const options = parseOptions(payload.data.targets)
  return config && options ? { config, options } : null
}

export function parseSavedOrderPushPolicy(payload: unknown): OrderPushPolicyConfig | null {
  if (!isRecord(payload) || !isRecord(payload.data)) return null
  return parseConfig(payload.data.config)
}

export function buildOrderPushPolicyDraft(config: OrderPushPolicyConfig, options: OrderPushTargetOption[]): OrderPushPolicyDraft[] {
  return options.map((option) => {
    const configured = config.targets.find((target) => target.target_code.toLowerCase() === option.code.toLowerCase())
    return { code: option.code, name: option.name, cycle: String(configured?.cycle ?? 0), skip: String(configured?.skip ?? 0) }
  })
}

export function buildOrderPushPolicyPayload(draft: OrderPushPolicyDraft[], options: OrderPushTargetOption[]): PolicyPayloadResult {
  if (draft.length !== options.length) return { ok: false, error: '推送目标配置已变化，请刷新后重试。' }
  const targets: OrderPushPolicyTarget[] = []
  for (let index = 0; index < options.length; index += 1) {
    const item = draft[index]
    const option = options[index]
    if (!item || item.code.toLowerCase() !== option.code.toLowerCase()) return { ok: false, error: '推送目标配置已变化，请刷新后重试。' }
    if (!/^\d+$/.test(item.cycle.trim()) || !/^\d+$/.test(item.skip.trim())) return { ok: false, error: '循环和少推数量必须是非负整数。' }
    const cycle = Number(item.cycle)
    const skip = Number(item.skip)
    if (!isNonNegativeSafeInteger(cycle) || !isNonNegativeSafeInteger(skip) || (cycle === 0 && skip !== 0) || (cycle > 0 && skip >= cycle)) return { ok: false, error: '循环为 0 时少推单数必须为 0；启用少推时少推单数必须小于循环总单数。' }
    targets.push({ target_code: option.code, target_name: option.name, cycle: cycle === 0 || skip === 0 ? 0 : cycle, skip: cycle === 0 || skip === 0 ? 0 : skip })
  }
  return { ok: true, payload: { targets } }
}

export function policyEnabled(item: OrderPushPolicyDraft): boolean {
  const cycle = Number(item.cycle)
  const skip = Number(item.skip)
  return Number.isSafeInteger(cycle) && Number.isSafeInteger(skip) && cycle > 0 && skip > 0 && skip < cycle
}

export function policyDeliveryRatio(item: OrderPushPolicyDraft): string {
  if (!policyEnabled(item)) return '100.0%'
  return `${(((Number(item.cycle) - Number(item.skip)) / Number(item.cycle)) * 100).toFixed(1)}%`
}
