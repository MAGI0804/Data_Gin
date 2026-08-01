import assert from 'node:assert/strict'
import test from 'node:test'
import {
  parsePipelineDetail,
  parsePipelinePreview,
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
