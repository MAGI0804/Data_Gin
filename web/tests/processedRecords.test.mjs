import assert from 'node:assert/strict'
import test from 'node:test'
import { buildCleanRecordsQuery, buildProcessedRecordsQuery, parseProcessedRecordsPage } from '../.test-dist/processedRecords.js'

test('builds bounded server-side processed-record filters', () => {
  assert.equal(buildProcessedRecordsQuery({ page: 2, pageSize: 20, dataType: ' order ', minQuality: '80', maxQuality: '100', createdFrom: '1', createdTo: '9' }), 'page=2&page_size=20&data_type=order&min_quality=80&max_quality=100&created_from=1&created_to=9')
  assert.equal(buildProcessedRecordsQuery({ page: 0, pageSize: 101, minQuality: '-1' }), 'page=1&page_size=20')
})

test('builds bounded clean-record filters', () => {
  assert.equal(buildCleanRecordsQuery({ page: 2, pageSize: 30, sourceID: '7', tableName: 'orders', businessKey: 'SO-9', status: 'ready', minQuality: '80', maxQuality: '99.5', createdFrom: '100', createdTo: '200' }), 'page=2&page_size=30&source_id=7&table_name=orders&business_key=SO-9&status=ready&min_quality=80&max_quality=99.5&created_from=100&created_to=200')
  assert.equal(buildCleanRecordsQuery({ page: 0, pageSize: 999, sourceID: '3.5' }), 'page=1&page_size=20')
})

test('parses only complete processed-record paging envelopes', () => {
  assert.deepEqual(parseProcessedRecordsPage({ data: { list: [{ id: 1 }], total: 1, page: 1, page_size: 20, total_pages: 1, summary: { avg_quality: 99 } } }), { list: [{ id: 1 }], total: 1, page: 1, pageSize: 20, totalPages: 1, averageQuality: 99 })
  assert.equal(parseProcessedRecordsPage({ data: { list: [], total: 0, page: 1, page_size: 20, total_pages: 0 } }), null)
})
