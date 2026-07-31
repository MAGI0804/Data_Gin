import assert from 'node:assert/strict'
import test from 'node:test'
import { parsePipelineRunResult, pipelineRunPath } from '../.test-dist/pipelineRun.js'

test('builds only valid pipeline run paths', () => {
  assert.equal(pipelineRunPath(17), '/v1/pipelines/17/run')
  assert.throws(() => pipelineRunPath(0), /invalid pipeline id/)
  assert.throws(() => pipelineRunPath(1.5), /invalid pipeline id/)
})

test('accepts only complete safe manual pipeline results', () => {
  const payload = { code: 200, data: { result: { run_id: 7, trace_id: 'trace-7', success_count: 3, failed_count: 0 } } }
  assert.deepEqual(parsePipelineRunResult(payload), { runID: 7, traceID: 'trace-7', successCount: 3, failedCount: 0 })
  assert.equal(parsePipelineRunResult({ ...payload, code: 0 }), null)
  assert.equal(parsePipelineRunResult({ ...payload, data: { result: { ...payload.data.result, trace_id: '' } } }), null)
  assert.equal(parsePipelineRunResult({ ...payload, data: { result: { ...payload.data.result, failed_count: -1 } } }), null)
})
