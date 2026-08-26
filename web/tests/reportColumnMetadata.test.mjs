import assert from 'node:assert/strict'
import test from 'node:test'

import { appendReportColumnFromResultSchema, countReportColumnsMissingFromResultSchema, refreshReportColumnMetadata, refreshReportColumnMetadataPreservingUnknown, replaceExcelMappingFieldWithResultSchema, reportColumnsFromResultSchema } from '../.test-dist/reportCenter/refCursorConfig.js'

test('result metadata refresh keeps selected fields and compatible presentation types', () => {
  const existing = [
    { ...reportColumnsFromResultSchema([{ name: 'STALE_ID', position: 1, oracleType: 'VARCHAR2', dataLength: 36, precision: null, scale: null, nullable: false }], () => 'stale-field')[0] },
    { ...reportColumnsFromResultSchema([{ name: 'CREATED_AT', position: 1, oracleType: 'DATE', dataLength: 7, precision: null, scale: null, nullable: true }], () => 'date-field')[0], valueType: 'datetime' },
    { ...reportColumnsFromResultSchema([{ name: 'QUANTITY', position: 1, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 0, nullable: true }], () => 'number-field')[0], valueType: 'integer' },
    { ...reportColumnsFromResultSchema([{ name: 'PAYLOAD', position: 1, oracleType: 'CLOB', dataLength: null, precision: null, scale: null, nullable: true }], () => 'json-field')[0], valueType: 'json' },
  ]
  const schema = [
    { name: 'CREATED_AT', position: 1, oracleType: 'DATE', dataLength: 7, precision: null, scale: null, nullable: false },
    { name: 'QUANTITY', position: 2, oracleType: 'NUMBER', dataLength: 22, precision: 12, scale: 0, nullable: false },
    { name: 'PAYLOAD', position: 3, oracleType: 'CLOB', dataLength: null, precision: null, scale: null, nullable: false },
    { name: 'NOT_SELECTED', position: 4, oracleType: 'VARCHAR2', dataLength: 20, precision: null, scale: null, nullable: true },
  ]

  const refreshed = refreshReportColumnMetadata(schema, existing)
  assert.deepEqual(refreshed.map((column) => column.databaseColumn), ['CREATED_AT', 'QUANTITY', 'PAYLOAD'])
  assert.deepEqual(refreshed.map((column) => column.valueType), ['datetime', 'integer', 'json'])
  assert.deepEqual(refreshed.map((column) => column.nullable), [false, false, false])
  assert.equal(refreshed[1].precision, 12)
  assert.equal(refreshed[1].scale, 0)
})

test('scaled NUMBER metadata falls back from integer to decimal', () => {
  const existing = reportColumnsFromResultSchema([
    { name: 'AMOUNT', position: 1, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 0, nullable: true },
  ], () => 'amount-field')
  existing[0].valueType = 'integer'

  const refreshed = refreshReportColumnMetadata([
    { name: 'AMOUNT', position: 1, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 2, nullable: true },
  ], existing)
  assert.equal(refreshed[0].valueType, 'decimal')
})

test('Excel mapping selection only accepts result table fields and refreshes Oracle metadata', () => {
  const schema = [
    { name: 'OLD_NAME', position: 1, oracleType: 'VARCHAR2', dataLength: 30, precision: null, scale: null, nullable: true },
    { name: 'AMOUNT', position: 2, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 2, nullable: false },
  ]
  const existing = reportColumnsFromResultSchema([schema[0]], () => 'stable-field')
  existing[0].excelHeader = '金额'
  existing[0].previewHeader = '金额'

  const selected = replaceExcelMappingFieldWithResultSchema(existing, 'OLD_NAME', 'AMOUNT', schema)
  assert.equal(selected[0].fieldId, 'stable-field')
  assert.equal(selected[0].logicalCode, existing[0].logicalCode)
  assert.equal(selected[0].databaseColumn, 'AMOUNT')
  assert.equal(selected[0].sourceOracleType, 'NUMBER')
  assert.equal(selected[0].precision, 18)
  assert.equal(selected[0].scale, 2)
  assert.equal(selected[0].nullable, false)
  assert.equal(selected[0].valueType, 'decimal')
  assert.equal(selected[0].excelHeader, '金额')

  assert.throws(
    () => replaceExcelMappingFieldWithResultSchema(existing, 'OLD_NAME', 'FIELD_6', schema),
    /不在当前结果表中/,
  )
})

test('Excel metadata refresh preserves invalid legacy mappings for explicit repair', () => {
  const existing = reportColumnsFromResultSchema([
    { name: 'FIELD_6', position: 1, oracleType: 'VARCHAR2', dataLength: 30, precision: null, scale: null, nullable: true },
  ], () => 'legacy-field')
  const refreshed = refreshReportColumnMetadataPreservingUnknown([
    { name: 'AMOUNT', position: 1, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 2, nullable: false },
  ], existing)
  assert.equal(refreshed.length, 1)
  assert.equal(refreshed[0].fieldId, 'legacy-field')
  assert.equal(refreshed[0].databaseColumn, 'FIELD_6')
  assert.equal(countReportColumnsMissingFromResultSchema(refreshed, [
    { name: 'AMOUNT', position: 1, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 2, nullable: false },
  ]), 1)
})

test('adding an Excel result field preserves existing columns and creates a unique logical code', () => {
  const existing = reportColumnsFromResultSchema([
    { name: 'PRODUCT$ID', position: 1, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 0, nullable: false },
  ], () => 'existing-field')
  existing[0].exportVisible = false
  const appended = appendReportColumnFromResultSchema(existing, {
    name: 'PRODUCT#ID', position: 2, oracleType: 'VARCHAR2', dataLength: 30, precision: null, scale: null, nullable: true,
  }, () => 'new-field')
  assert.equal(appended.length, 2)
  assert.equal(appended[0].fieldId, 'existing-field')
  assert.equal(appended[0].exportVisible, false)
  assert.equal(appended[1].fieldId, 'new-field')
  assert.notEqual(appended[1].logicalCode.toUpperCase(), appended[0].logicalCode.toUpperCase())
})
