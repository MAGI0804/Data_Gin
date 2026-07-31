import assert from 'node:assert/strict'
import test from 'node:test'

import { mallImportRequestWithinLimit, parseMallImportCSV, parseMallImportResult } from '../.test-dist/mallImport.js'

const header = 'mallCode,nameCn,province,city,district,address'

test('parses a UTF-8 BOM CSV with quoted values and normalizes mall codes', () => {
  const rows = parseMallImportCSV(`\uFEFF${header}\r\nsh-001,示例商场,上海市,上海市,浦东新区,"世纪大道, 1 号"\r\n`)
  assert.deepEqual(rows, [{
    row: 2,
    item: { mallCode: 'SH-001', nameCn: '示例商场', province: '上海市', city: '上海市', district: '浦东新区', address: '世纪大道, 1 号' },
  }])
})

test('reports row-local CSV validation errors and rejects structurally invalid files', () => {
  const rows = parseMallImportCSV(`${header}\nOK-01,有效商场,上海市,上海市,,有效地址\n!,无效商场,上海市,上海市,,有效地址\n`)
  assert.equal(rows[0].item?.mallCode, 'OK-01')
  assert.deepEqual(rows[1], { row: 3, error: '商场编码格式无效' })
  assert.throws(() => parseMallImportCSV(`mallCode,nameCn\nSH-001,商场\n`), /CSV 表头/)
  assert.throws(() => parseMallImportCSV(`${header}\n"SH-001,商场,上海市,上海市,,地址\n`), /引号未闭合/)
  assert.throws(() => parseMallImportCSV(`${header}\n${Array.from({ length: 201 }, (_, index) => `SH-${index + 100},商场,上海市,上海市,,地址`).join('\n')}`), /1 至 200/)
  assert.deepEqual(parseMallImportCSV(`${header}\nSH-001,${'名'.repeat(256)},上海市,上海市,,地址`), [{ row: 2, error: '字段长度超过限制' }])
  assert.equal(mallImportRequestWithinLimit([{ mallCode: 'SH-001', nameCn: '商场', province: '上海市', city: '上海市', address: '地址' }]), true)
  assert.equal(mallImportRequestWithinLimit([{ mallCode: 'SH-001', nameCn: '商场', province: '上海市', city: '上海市', address: '地'.repeat(1_048_576) }]), false)
})

test('parses only complete and internally consistent import result summaries', () => {
  const payload = { code: 0, data: {
    created: 1, replayed: 1, failed: 1,
    rows: [
      { row: 1, status: 'CREATED', reviewStatus: 'PENDING_GEOCODE', mall: { mallCode: 'SH-001' } },
      { row: 2, status: 'REPLAYED', reviewStatus: 'PENDING_GEOCODE', mall: { mallCode: 'SH-002' } },
      { row: 3, status: 'FAILED', errorCode: 'CONFLICT' },
    ],
  } }
  assert.deepEqual(parseMallImportResult(payload, 3), {
    created: 1,
    replayed: 1,
    failed: 1,
    rows: [
      { row: 1, status: 'CREATED', reviewStatus: 'PENDING_GEOCODE', mallCode: 'SH-001' },
      { row: 2, status: 'REPLAYED', reviewStatus: 'PENDING_GEOCODE', mallCode: 'SH-002' },
      { row: 3, status: 'FAILED', errorCode: 'CONFLICT' },
    ],
  })
  assert.equal(parseMallImportResult({ ...payload, data: { ...payload.data, failed: 0 } }, 3), null)
  assert.equal(parseMallImportResult({ ...payload, data: { ...payload.data, rows: payload.data.rows.slice(0, 2) } }, 3), null)
  assert.equal(parseMallImportResult({ ...payload, data: { ...payload.data, rows: [{ row: 1, status: 'FAILED', errorCode: 'secret' }] } }, 1), null)
})
