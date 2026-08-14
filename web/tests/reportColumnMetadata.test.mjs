import assert from 'node:assert/strict'
import test from 'node:test'

import { refreshReportColumnMetadata, reportColumnsFromResultSchema } from '../.test-dist/reportCenter/refCursorConfig.js'

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
