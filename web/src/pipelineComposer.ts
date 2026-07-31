export type PipelineSummary = {
  id: number
  name: string
  code: string
  description?: string
  enabled: boolean
}

export type PipelineStage = {
  id: number
  pipeline_id: number
  stage_type: StageType
  name: string
  order_index: number
  enabled: boolean
}

export type StageType = 'fetch' | 'process' | 'push' | 'log'

export type MethodParam = {
  location: string
  name: string
  value_source: string
  value: string
  value_type: string
  required: boolean
  secret: boolean
  description: string
  order_index: number
}

export type MethodOutput = {
  name: string
  source_path: string
  value_type: string
  required: boolean
  description: string
  order_index: number
}

export type PipelineStep = {
  id: number
  pipeline_id: number
  stage_id: number
  code: string
  name: string
  method_type: string
  order_index: number
  enabled: boolean
  timeout_seconds: number
}

export type PipelineStepDetail = {
  step: PipelineStep
  params: MethodParam[]
  outputs: MethodOutput[]
}

export type StageGeneratedConfig = {
  id: number
  pipeline_id: number
  stage_id: number
  stage_type: StageType
  generated_config_json: string
  target_ref_type: string
  target_ref_id: number
  version: number
}

export type PipelineStageDetail = {
  stage: PipelineStage
  steps: PipelineStepDetail[]
  generated_config: StageGeneratedConfig | null
}

export type PipelineDetail = {
  pipeline: PipelineSummary
  stages: PipelineStageDetail[]
  steps: PipelineStepDetail[]
}

export type PipelineRequest = {
  name: string
  code: string
  description: string
  enabled: boolean
}

export type StageRequest = {
  stage_type: StageType
  name: string
  order_index: number
  enabled: boolean
}

export type StepRequest = {
  stage_id: number
  code: string
  name: string
  method_type: string
  order_index: number
  enabled: boolean
  timeout_seconds: number
  params: MethodParam[]
  outputs: MethodOutput[]
}

const maskedParameterValue = '[已隐藏]'

const stageTypes: readonly StageType[] = ['fetch', 'process', 'push', 'log']

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function stringField(value: unknown, max = 4096) {
  return typeof value === 'string' && value.length <= max ? value : null
}

function finiteInteger(value: unknown) {
  return typeof value === 'number' && Number.isSafeInteger(value) ? value : null
}

function positiveID(value: unknown) {
  const id = finiteInteger(value)
  return id !== null && id > 0 ? id : null
}

function boolField(value: unknown) {
  return typeof value === 'boolean' ? value : null
}

function stageType(value: unknown): StageType | null {
  return typeof value === 'string' && stageTypes.includes(value as StageType) ? value as StageType : null
}

function parsePipeline(value: unknown): PipelineSummary | null {
  if (!isRecord(value)) return null
  const id = positiveID(value.id)
  const name = stringField(value.name, 100)
  const code = stringField(value.code, 100)
  const description = stringField(value.description) ?? ''
  const enabled = boolField(value.enabled)
  return id !== null && name !== null && code !== null && enabled !== null ? { id, name, code, description, enabled } : null
}

function parseParam(value: unknown): MethodParam | null {
  if (!isRecord(value)) return null
  const location = stringField(value.location, 50)
  const name = stringField(value.name, 100)
  const valueSource = stringField(value.value_source, 50)
  const paramValue = stringField(value.value)
  const valueType = stringField(value.value_type, 50)
  const required = boolField(value.required)
  const secret = boolField(value.secret)
  const description = stringField(value.description)
  const orderIndex = finiteInteger(value.order_index)
  return location !== null && name !== null && valueSource !== null && paramValue !== null && valueType !== null && required !== null && secret !== null && description !== null && orderIndex !== null
    ? { location, name, value_source: valueSource, value: paramValue, value_type: valueType, required, secret, description, order_index: orderIndex }
    : null
}

function parseOutput(value: unknown): MethodOutput | null {
  if (!isRecord(value)) return null
  const name = stringField(value.name, 100)
  const sourcePath = stringField(value.source_path, 255)
  const valueType = stringField(value.value_type, 50)
  const required = boolField(value.required)
  const description = stringField(value.description)
  const orderIndex = finiteInteger(value.order_index)
  return name !== null && sourcePath !== null && valueType !== null && required !== null && description !== null && orderIndex !== null
    ? { name, source_path: sourcePath, value_type: valueType, required, description, order_index: orderIndex }
    : null
}

function parseStepDetail(value: unknown): PipelineStepDetail | null {
  if (!isRecord(value) || !isRecord(value.step) || !Array.isArray(value.params) || !Array.isArray(value.outputs)) return null
  const raw = value.step
  const id = positiveID(raw.id)
  const pipelineID = positiveID(raw.pipeline_id)
  const stageID = positiveID(raw.stage_id)
  const code = stringField(raw.code, 100)
  const name = stringField(raw.name, 100)
  const methodType = stringField(raw.method_type, 50)
  const orderIndex = finiteInteger(raw.order_index)
  const enabled = boolField(raw.enabled)
  const timeoutSeconds = finiteInteger(raw.timeout_seconds)
  const params = value.params.map(parseParam)
  const outputs = value.outputs.map(parseOutput)
  if (id === null || pipelineID === null || stageID === null || code === null || name === null || methodType === null || orderIndex === null || enabled === null || timeoutSeconds === null || params.some((item) => item === null) || outputs.some((item) => item === null)) return null
  return { step: { id, pipeline_id: pipelineID, stage_id: stageID, code, name, method_type: methodType, order_index: orderIndex, enabled, timeout_seconds: timeoutSeconds }, params: params as MethodParam[], outputs: outputs as MethodOutput[] }
}

function parseStageConfigDTO(value: unknown): StageGeneratedConfig | null {
  if (!isRecord(value)) return null
  const id = positiveID(value.id)
  const pipelineID = positiveID(value.pipeline_id)
  const stageID = positiveID(value.stage_id)
  const kind = stageType(value.stage_type)
  const json = stringField(value.generated_config_json, 4096)
  const targetType = stringField(value.target_ref_type, 50)
  const targetID = finiteInteger(value.target_ref_id)
  const version = positiveID(value.version)
  return id !== null && pipelineID !== null && stageID !== null && kind !== null && json !== null && targetType !== null && targetID !== null && targetID >= 0 && version !== null
    ? { id, pipeline_id: pipelineID, stage_id: stageID, stage_type: kind, generated_config_json: json, target_ref_type: targetType, target_ref_id: targetID, version }
    : null
}

function parseStageDetail(value: unknown): PipelineStageDetail | null {
  if (!isRecord(value) || !isRecord(value.stage) || !Array.isArray(value.steps)) return null
  const raw = value.stage
  const id = positiveID(raw.id)
  const pipelineID = positiveID(raw.pipeline_id)
  const kind = stageType(raw.stage_type)
  const name = stringField(raw.name, 100)
  const orderIndex = finiteInteger(raw.order_index)
  const enabled = boolField(raw.enabled)
  const steps = value.steps.map(parseStepDetail)
  const generated = value.generated_config === null ? null : parseStageConfigDTO(value.generated_config)
  if (id === null || pipelineID === null || kind === null || name === null || orderIndex === null || enabled === null || steps.some((item) => item === null) || (value.generated_config !== null && generated === null)) return null
  return { stage: { id, pipeline_id: pipelineID, stage_type: kind, name, order_index: orderIndex, enabled }, steps: steps as PipelineStepDetail[], generated_config: generated }
}

export function parsePipelineDetail(payload: unknown): PipelineDetail | null {
  if (!isRecord(payload) || payload.code !== 200 || !isRecord(payload.data) || !isRecord(payload.data.pipeline)) return null
  const pipeline = parsePipeline(payload.data.pipeline.pipeline)
  const stages = Array.isArray(payload.data.pipeline.stages) ? payload.data.pipeline.stages.map(parseStageDetail) : null
  const steps = Array.isArray(payload.data.pipeline.steps) ? payload.data.pipeline.steps.map(parseStepDetail) : null
  return pipeline && stages && steps && !stages.some((item) => item === null) && !steps.some((item) => item === null) ? { pipeline, stages: stages as PipelineStageDetail[], steps: steps as PipelineStepDetail[] } : null
}

export function parsePipelineWriteResult(payload: unknown): PipelineSummary | null {
  if (!isRecord(payload) || payload.code !== 200 || !isRecord(payload.data)) return null
  return parsePipeline(payload.data.pipeline)
}

export function parsePipelinePreview(payload: unknown): Record<string, unknown> | null {
  if (!isRecord(payload) || payload.code !== 200 || !isRecord(payload.data) || !isRecord(payload.data.preview)) return null
  return payload.data.preview
}

export function parseStageGeneratedConfig(payload: unknown): StageGeneratedConfig | null {
  if (!isRecord(payload) || payload.code !== 200 || !isRecord(payload.data)) return null
  return parseStageConfigDTO(payload.data.config)
}

export function isMaskedMethodParam(param: MethodParam) {
  return param.value === maskedParameterValue
}

export function pipelinePath(id: number) {
  if (!Number.isSafeInteger(id) || id < 1) throw new Error('invalid pipeline id')
  return `/v1/pipelines/${id}`
}

export function pipelineStagePath(id: number) {
  if (!Number.isSafeInteger(id) || id < 1) throw new Error('invalid stage id')
  return `/v1/pipeline-stages/${id}`
}

export function pipelineStepPath(pipelineID: number, stepID: number) {
  if (!Number.isSafeInteger(pipelineID) || pipelineID < 1 || !Number.isSafeInteger(stepID) || stepID < 1) throw new Error('invalid pipeline step')
  return `${pipelinePath(pipelineID)}/steps/${stepID}`
}

export function stageStepPath(stageID: number, stepID?: number) {
  const base = `${pipelineStagePath(stageID)}/steps`
  if (stepID === undefined) return base
  if (!Number.isSafeInteger(stepID) || stepID < 1) throw new Error('invalid stage step')
  return `${base}/${stepID}`
}

export function stageConfigPath(stageID: number, action: 'generate-config' | 'publish-config') {
  return `${pipelineStagePath(stageID)}/${action}`
}

export function stageMethodTypes(kind: StageType) {
  const types: Record<StageType, readonly string[]> = {
    fetch: ['request', 'bojun_signed_request', 'extract', 'db_query'],
    process: ['mapping', 'validate', 'db_query', 'db_write', 'template', 'request', 'bojun_signed_request'],
    push: ['template', 'delivery', 'request', 'shanghai_mall_push'],
    log: ['log', 'db_write', 'delivery'],
  }
  return types[kind]
}

export function parseStepConfigList(text: string, type: 'params'): MethodParam[] | null
export function parseStepConfigList(text: string, type: 'outputs'): MethodOutput[] | null
export function parseStepConfigList(text: string, type: 'params' | 'outputs'): MethodParam[] | MethodOutput[] | null {
  try {
    const value: unknown = JSON.parse(text)
    if (!Array.isArray(value) || value.length > 100) return null
    const parsed = type === 'params' ? value.map((item, index) => parseParamInput(item, index)) : value.map((item, index) => parseOutputInput(item, index))
    return parsed.some((item) => item === null) ? null : parsed as MethodParam[] | MethodOutput[]
  } catch {
    return null
  }
}

function optionalString(value: unknown, max: number, fallback: string) {
  return value === undefined ? fallback : stringField(value, max)
}

function optionalBool(value: unknown, fallback: boolean) {
  return value === undefined ? fallback : boolField(value)
}

function optionalInteger(value: unknown, fallback: number) {
  return value === undefined ? fallback : finiteInteger(value)
}

function parseParamInput(value: unknown, index: number): MethodParam | null {
  if (!isRecord(value)) return null
  const location = stringField(value.location, 50)
  const name = stringField(value.name, 100)
  const valueSource = stringField(value.value_source, 50)
  const paramValue = optionalString(value.value, 4096, '')
  const valueType = optionalString(value.value_type, 50, 'string')
  const required = optionalBool(value.required, false)
  const secret = optionalBool(value.secret, false)
  const description = optionalString(value.description, 4096, '')
  const orderIndex = optionalInteger(value.order_index, index)
  return location !== null && name !== null && valueSource !== null && paramValue !== null && valueType !== null && required !== null && secret !== null && description !== null && orderIndex !== null
    ? { location, name, value_source: valueSource, value: paramValue, value_type: valueType, required, secret, description, order_index: orderIndex }
    : null
}

function parseOutputInput(value: unknown, index: number): MethodOutput | null {
  if (!isRecord(value)) return null
  const name = stringField(value.name, 100)
  const sourcePath = optionalString(value.source_path, 255, '')
  const valueType = optionalString(value.value_type, 50, 'string')
  const required = optionalBool(value.required, false)
  const description = optionalString(value.description, 4096, '')
  const orderIndex = optionalInteger(value.order_index, index)
  return name !== null && sourcePath !== null && valueType !== null && required !== null && description !== null && orderIndex !== null
    ? { name, source_path: sourcePath, value_type: valueType, required, description, order_index: orderIndex }
    : null
}
