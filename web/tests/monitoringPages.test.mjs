import assert from 'node:assert/strict'
import test from 'node:test'

import { monitoringDurationLabel, parseMonitoringJSON, parsePipelineRun, parseStepRun, parseStepRunsResponse, pipelineRunStatusLabel, stepRunStatusLabel } from '../.test-dist/monitoringPages/contracts.js'

test('strictly parses safe pipeline run rows', () => {
  const row = { id: 7, trace_id: 'trace-7', run_type: 'delivery', trigger_type: 'manual', status: 'success', total_count: 3, success_count: 3, failed_count: 0, source_id: 0, destination_id: 9, started_at: '2026-08-13T08:00:00+08:00', finished_at: '2026-08-13T08:01:02+08:00' }
  assert.deepEqual(parsePipelineRun(row), row)
  assert.equal(parsePipelineRun({ ...row, status: 'unknown' }), null)
  assert.equal(parsePipelineRun({ ...row, total_count: -1 }), null)
  assert.equal(pipelineRunStatusLabel('partial_success'), '部分成功')
  assert.equal(monitoringDurationLabel(row.started_at, row.finished_at), '1 分 2 秒')
})

test('strictly parses protected step rows without exposing invalid shapes', () => {
  const row = { id: 11, run_id: 7, pipeline_id: 2, step_id: 4, step_code: 'deliver', method_type: 'delivery', status: 'failed', input_json: '{"token":"secret"}', output_json: '{}', generated_config_json: '{}', error_message: 'protected', started_at: null, finished_at: null }
  assert.deepEqual(parseStepRun(row), row)
  assert.equal(parseStepRun({ ...row, status: 'partial_success' }), null)
  assert.equal(stepRunStatusLabel('skipped'), '已跳过')
  assert.deepEqual(parseMonitoringJSON('{"value":1}'), { value: 1 })
  assert.equal(parseMonitoringJSON('invalid-json'), 'invalid-json')
  assert.deepEqual(parseStepRunsResponse({ data: { step_runs: [row] } }), [row])
  assert.equal(parseStepRunsResponse({ step_runs: [row] }), null)
  assert.equal(parseStepRunsResponse({ data: { steps: [row] } }), null)
})
