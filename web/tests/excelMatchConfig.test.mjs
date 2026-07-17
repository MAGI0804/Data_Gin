import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildExcelExportConfig,
  excelFieldSelectOptions,
  excelJobDetailHash,
  excelJobIDFromHash,
  excelModelSelectOptions,
  migrateExcelMatchSteps,
  selectExcelMatchStepModel,
} from '../.test-dist/excelMatchConfig.js'

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

const modelCatalog = [{
  name: '伯俊零售订单',
  modelName: 'BojunRetailOrder',
  tableName: 'bojun_retail_orders',
  description: '伯俊零售单头数据',
  mapping: '伯俊零售订单（BojunRetailOrder） → 数据库表 bojun_retail_orders',
  fields: [
    {
      name: '伯俊零售单号',
      modelField: 'DocNo',
      columnName: 'docno',
      dataType: 'varchar(255)',
      description: '伯俊系统生成的零售单号',
      mapping: 'BojunRetailOrder.DocNo → bojun_retail_orders.docno',
      nullable: false,
    },
    {
      name: '伯俊门店名称',
      modelField: 'StoreName',
      columnName: 'c_store_name',
      dataType: 'varchar(255)',
      description: '伯俊订单所属门店名称',
      mapping: 'BojunRetailOrder.StoreName → bojun_retail_orders.c_store_name',
      nullable: true,
    },
  ],
}, {
  name: '企迈订单',
  modelName: 'QIMAI_ORDER_DATA',
  tableName: 'qimai_order_data',
  description: '企迈渠道订单数据',
  mapping: '企迈订单（QIMAI_ORDER_DATA） → 数据库表 qimai_order_data',
  fields: [{
    name: '业务订单号',
    modelField: 'OrderNo',
    columnName: 'order_no',
    dataType: 'varchar(100)',
    description: '企迈业务订单号',
    mapping: 'QIMAI_ORDER_DATA.OrderNo → qimai_order_data.order_no',
    nullable: false,
  }],
}]

test('model and field option labels explain model-to-database mappings', () => {
  const modelOptions = excelModelSelectOptions(modelCatalog, '')
  const fieldOptions = excelFieldSelectOptions(modelCatalog[0].fields, '')

  assert.equal(modelOptions[0].label, '伯俊零售订单（BojunRetailOrder → bojun_retail_orders）')
  assert.equal(fieldOptions[0].label, '伯俊零售单号（DocNo → docno）')
})

test('selectExcelMatchStepModel clears fields missing from the selected model', () => {
  const step = {
    ...fallbackStep,
    dbMatchField: 'docno',
    dbValueField: 'c_store_name',
  }

  const switched = selectExcelMatchStepModel(step, 'qimai_order_data', modelCatalog)

  assert.equal(switched.tableName, 'qimai_order_data')
  assert.equal(switched.dbMatchField, '')
  assert.equal(switched.dbValueField, '')
})

test('catalog selectors preserve missing legacy table and field values as current options', () => {
  const modelOptions = excelModelSelectOptions(modelCatalog, 'legacy_orders')
  const fieldOptions = excelFieldSelectOptions(modelCatalog[0].fields, 'legacy_field')

  assert.deepEqual(modelOptions[0], {
    value: 'legacy_orders',
    label: '当前配置：legacy_orders（目录中不存在）',
    currentOnly: true,
  })
  assert.deepEqual(fieldOptions[0], {
    value: 'legacy_field',
    label: '当前配置：legacy_field（模型中不存在）',
    currentOnly: true,
  })
})

test('excel job detail navigation uses and parses a stable task-specific hash', () => {
  assert.equal(excelJobDetailHash(321), 'excel_jobs?job=321')
  assert.equal(excelJobDetailHash(0), 'excel_jobs')
  assert.equal(excelJobIDFromHash('#excel_jobs?job=321'), 321)
  assert.equal(excelJobIDFromHash('#/excel_jobs?job=321'), 321)
  assert.equal(excelJobIDFromHash('#excel_jobs?job=invalid'), null)
  assert.equal(excelJobIDFromHash('#excel_schemes?job=321'), null)
})
