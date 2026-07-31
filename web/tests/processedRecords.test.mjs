import assert from 'node:assert/strict'
import test from 'node:test'
import { buildProcessedRecordsQuery, parseProcessedRecordsPage } from '../.test-dist/processedRecords.js'

test('builds bounded server-side processed-record filters', () => {
  assert.equal(buildProcessedRecordsQuery({ page: 2, pageSize: 20, dataType: ' order ', minQuality: '80', maxQuality: '100', createdFrom: '1', createdTo: '9' }), 'page=2&page_size=20&data_type=order&min_quality=80&max_quality=100&created_from=1&created_to=9')
  assert.equal(buildProcessedRecordsQuery({ page: 0, pageSize: 101, minQuality: '-1' }), 'page=1&page_size=20')
})

test('parses only complete processed-record paging envelopes', () => {
  assert.deepEqual(parseProcessedRecordsPage({ data: { list: [{ id: 1 }], total: 1, page: 1, page_size: 20, total_pages: 1, summary: { avg_quality: 99 } } }), { list: [{ id: 1 }], total: 1, page: 1, pageSize: 20, totalPages: 1, averageQuality: 99 })
  assert.equal(parseProcessedRecordsPage({ data: { list: [], total: 0, page: 1, page_size: 20, total_pages: 0 } }), null)
})
