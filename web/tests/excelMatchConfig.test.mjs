import assert from 'node:assert/strict'
import test from 'node:test'

import { buildExcelExportConfig, migrateExcelMatchSteps } from '../.test-dist/excelMatchConfig.js'

const fallbackStep = {
  name: '匹配伯俊门店',
  filters: [],
  tableName: 'bojun_retail_orders',
  matchExcelColumn: '原始线上订单号',
  dbMatchField: 'matched_docno',
  dbValueField: 'c_store_name',
  outputColumnName: '线下店名称',
}

test('migrateExcelMatchSteps moves legacy top-level filters to the first step only', () => {
  const source = {
    filters: [{ column: '类型', op: 'eq', value: 'A' }],
    steps: [
      { ...fallbackStep, name: '步骤 1' },
      { ...fallbackStep, name: '步骤 2', tableName: 'other_orders', outputColumnName: '第二步结果' },
    ],
  }

  const steps = migrateExcelMatchSteps(source, fallbackStep)

  assert.deepEqual(steps[0].filters, [{ column: '类型', op: 'eq', value: 'A' }])
  assert.deepEqual(steps[1].filters, [])
  assert.deepEqual(source.steps[0].filters, [])
})

test('migrateExcelMatchSteps preserves independent filters on every step', () => {
  const steps = migrateExcelMatchSteps({
    steps: [
      { ...fallbackStep, filters: [{ column: '类型', op: 'eq', value: 'A' }] },
      { ...fallbackStep, filters: [{ column: '类型', op: 'eq', value: 'B' }], outputColumnName: 'B结果' },
    ],
  }, fallbackStep)

  assert.deepEqual(steps.map((step) => step.filters), [
    [{ column: '类型', op: 'eq', value: 'A' }],
    [{ column: '类型', op: 'eq', value: 'B' }],
  ])
})

test('buildExcelExportConfig emits step filters without a top-level filter', () => {
  const config = buildExcelExportConfig({
    sheetName: ' Sheet1 ',
    steps: [{
      ...fallbackStep,
      name: ' 第一步 ',
      filters: [
        { column: ' 类型 ', op: ' contains ', value: ' A ' },
        { column: ' 备注 ', op: ' empty ', value: '旧值' },
        { column: ' ', op: 'eq', value: '' },
      ],
    }],
    exportColumnFormats: [],
    batchSize: 1000,
  })

  assert.equal(Object.hasOwn(config, 'filters'), false)
  assert.deepEqual(config.steps[0].filters, [
    { column: '类型', op: 'contains', value: 'A' },
    { column: '备注', op: 'empty', value: '' },
  ])
  assert.equal(config.steps[0].name, '第一步')
})
