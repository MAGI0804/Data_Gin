import assert from 'node:assert/strict'
import test from 'node:test'
import { buildRawRecordsRequest, buildWarehouseRawRecordsQuery, parseRawRecordsPage } from '../.test-dist/rawRecords.js'

test('builds the supported server-side raw-record query fields', () => {
  assert.deepEqual(buildRawRecordsRequest({
    page: 2,
    pageSize: 20,
    source: ' source-a ',
    dataType: ' order ',
    status: ' pending ',
    businessKey: ' ERP-1 ',
    startTime: '2026-07-01 00:00:00',
    endTime: '2026-07-31 23:59:59',
    origin: 'pull',
  }), {
    page: 2,
    page_size: 20,
    source: 'source-a',
    data_type: 'order',
    status: 'pending',
    business_key: 'ERP-1',
    start_time: '2026-07-01 00:00:00',
    end_time: '2026-07-31 23:59:59',
    origin: 'pull',
  })
})

test('bounds raw-record pagination to the backend contract', () => {
  assert.deepEqual(buildRawRecordsRequest({ page: 0, pageSize: 101, origin: 'receive' }), {
    page: 1,
    page_size: 20,
    source: '',
    data_type: '',
    status: '',
    business_key: '',
    start_time: '',
    end_time: '',
    origin: 'receive',
  })
})

test('builds a GET query for the safe warehouse raw-record contract', () => {
  assert.equal(buildWarehouseRawRecordsQuery({
    page: 2,
    pageSize: 20,
    source: ' source-a ',
    startTime: '2026-07-01 00:00:00',
    endTime: '2026-07-31 23:59:59',
    origin: 'receive',
    status: 'failed',
    traceID: ' trace-1 ',
  }), 'page=2&page_size=20&origin=receive&source=source-a&start_time=2026-07-01+00%3A00%3A00&end_time=2026-07-31+23%3A59%3A59&status=failed&trace_id=trace-1')
})

test('accepts only a complete raw-record paging envelope', () => {
  const page = parseRawRecordsPage({ data: { list: [{ id: 9 }], total: 21, page: 1, page_size: 20, total_pages: 2 } })
  assert.deepEqual(page, { list: [{ id: 9 }], total: 21, page: 1, pageSize: 20, totalPages: 2 })
  assert.equal(parseRawRecordsPage({ data: { list: [], total: 0, page: 0, page_size: 20, total_pages: 0 } }), null)
})
