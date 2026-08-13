import assert from 'node:assert/strict'
import test from 'node:test'
import {
  parsePipelineDetail,
  parsePipelinePreview,
  parseStageWriteResult,
  parseStepWriteResult,
  parseStageGeneratedConfig,
  parseStepConfigList,
  isMaskedMethodParam,
  pipelinePath,
  pipelineStagePath,
  pipelineStepPath,
  stageConfigPath,
  stageMethodTypes,
  stageStepPath,
} from '../.test-dist/pipelineComposer.js'

const param = { location: 'header', name: 'X-Trace', value_source: 'literal', value: 'trace', value_type: 'string', required: true, secret: false, description: '', order_index: 0 }
const output = { name: 'result', source_path: '$.result', value_type: 'string', required: true, description: '', order_index: 0 }
const step = { step: { id: 11, pipeline_id: 1, stage_id: 3, code: 'fetch_orders', name: '获取订单', method_type: 'request', order_index: 1, enabled: true, timeout_seconds: 30 }, params: [param], outputs: [output] }

test('builds only positive scoped pipeline composition paths', () => {
  assert.equal(pipelinePath(1), '/v1/pipelines/1')
  assert.equal(pipelineStagePath(3), '/v1/pipeline-stages/3')
  assert.equal(pipelineStepPath(1, 11), '/v1/pipelines/1/steps/11')
  assert.equal(stageStepPath(3), '/v1/pipeline-stages/3/steps')
  assert.equal(stageStepPath(3, 11), '/v1/pipeline-stages/3/steps/11')
  assert.equal(stageConfigPath(3, 'generate-config'), '/v1/pipeline-stages/3/generate-config')
  assert.throws(() => pipelinePath(0))
  assert.throws(() => pipelineStepPath(1, 0))
})

test('parses only complete safe pipeline detail and generated-config envelopes', () => {
  const payload = {
    code: 200,
    data: {
      pipeline: {
        pipeline: { id: 1, name: '订单流水线', code: 'orders', description: '同步订单', enabled: true },
        stages: [{ stage: { id: 3, pipeline_id: 1, stage_type: 'fetch', name: '数据获取', order_index: 1, enabled: true }, steps: [step], generated_config: null }],
        steps: [step],
      },
    },
  }
  assert.deepEqual(parsePipelineDetail(payload)?.stages[0].steps[0].params, [param])
  assert.equal(parsePipelineDetail({ ...payload, data: { pipeline: { ...payload.data.pipeline, stages: [{ stage: payload.data.pipeline.stages[0].stage }] } } }), null)
  assert.deepEqual(parseStageGeneratedConfig({ code: 200, data: { config: { id: 8, pipeline_id: 1, stage_id: 3, stage_type: 'fetch', generated_config_json: '{}', target_ref_type: 'source_definition', target_ref_id: 0, version: 2 } } })?.version, 2)
  assert.equal(parseStageGeneratedConfig({ code: 200, data: { config: { id: 8 } } }), null)
})

test('validates stage and step write responses before accepting a save', () => {
  assert.deepEqual(parseStageWriteResult({ code: 200, data: { stage: { id: 3, pipeline_id: 1, stage_type: 'fetch', name: '数据获取', order_index: 1, enabled: true } } }), { id: 3, pipeline_id: 1, stage_type: 'fetch', name: '数据获取', order_index: 1, enabled: true })
  assert.equal(parseStageWriteResult({ code: 200, data: { stage: { id: 3 } } }), null)
  assert.deepEqual(parseStepWriteResult({ code: 200, data: { step } }), step)
  assert.equal(parseStepWriteResult({ code: 200, data: { step: { step: { id: 11 }, params: [], outputs: [] } } }), null)
})

test('keeps unbounded pipeline descriptions returned from the backend', () => {
  const description = '流水线说明'.repeat(1000)
  const payload = {
    code: 200,
    data: {
      pipeline: {
        pipeline: { id: 1, name: '订单流水线', code: 'orders', description, enabled: true },
        stages: [],
        steps: [],
      },
    },
  }
  assert.equal(parsePipelineDetail(payload)?.pipeline.description, description)
})

test('assigns legacy stage-zero steps to the backend default stages and keeps them visible', () => {
  const stages = [
    { id: 3, stage_type: 'fetch', name: '数据获取' },
    { id: 4, stage_type: 'process', name: '数据处理' },
    { id: 5, stage_type: 'push', name: '数据推送' },
    { id: 6, stage_type: 'log', name: '日志记录' },
  ].map((stage, index) => ({
    stage: { ...stage, pipeline_id: 1, order_index: index + 1, enabled: true },
    steps: [],
    generated_config: null,
  }))
  const steps = [
    ['request', 3],
    ['mapping', 4],
    ['delivery', 5],
    ['log', 6],
  ].map(([methodType], index) => ({
    ...step,
    step: { ...step.step, id: 20 + index, stage_id: 0, method_type: methodType },
  }))
  stages[0].steps = [steps[2], steps[2]]
  stages[1].steps = [steps[0]]
  const payload = {
    code: 200,
    data: {
      pipeline: {
        pipeline: { id: 1, name: '历史流水线', code: 'legacy', description: '', enabled: true },
        stages,
        steps,
      },
    },
  }

  const detail = parsePipelineDetail(payload)
  assert.deepEqual(detail?.steps.map((item) => item.step.stage_id), [3, 4, 5, 6])
  assert.deepEqual(detail?.stages.map((stage) => stage.steps.map((item) => item.step.id)), [[20], [21], [22], [23]])
})

test('limits step method choices and accepts only complete parameter/output arrays', () => {
  assert.deepEqual(stageMethodTypes('push'), ['template', 'delivery', 'request', 'shanghai_mall_push'])
  assert.deepEqual(parseStepConfigList(JSON.stringify([param]), 'params'), [param])
  assert.deepEqual(parseStepConfigList(JSON.stringify([output]), 'outputs'), [output])
  assert.deepEqual(parseStepConfigList(JSON.stringify([{ location: 'query', name: 'page', value_source: 'literal' }]), 'params'), [{ location: 'query', name: 'page', value_source: 'literal', value: '', value_type: 'string', required: false, secret: false, description: '', order_index: 0 }])
  assert.deepEqual(parseStepConfigList(JSON.stringify([{ name: 'id' }]), 'outputs'), [{ name: 'id', source_path: '', value_type: 'string', required: false, description: '', order_index: 0 }])
  assert.equal(parseStepConfigList(JSON.stringify([{ name: 'missing-required-fields' }]), 'params'), null)
  assert.equal(parseStepConfigList('{}', 'outputs'), null)
  assert.deepEqual(parsePipelinePreview({ code: 200, data: { preview: { pipeline: { id: 1 } } } }), { pipeline: { id: 1 } })
  assert.equal(parsePipelinePreview({ code: 200, data: { preview: [] } }), null)
})

test('treats every backend-masked parameter as non-rewritable', () => {
  assert.equal(isMaskedMethodParam({ ...param, value: '[已隐藏]', secret: false, name: 'Authorization' }), true)
  assert.equal(isMaskedMethodParam({ ...param, value: '[已隐藏]', secret: false, value_source: 'secret' }), true)
  assert.equal(isMaskedMethodParam(param), false)
})
