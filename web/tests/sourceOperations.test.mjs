import assert from 'node:assert/strict'
import test from 'node:test'
import { parseSourceFetchSummary } from '../.test-dist/sourceOperations.js'

test('reads only complete non-sensitive source fetch summaries', () => {
  assert.deepEqual(parseSourceFetchSummary({ data: { result: {
    trace_id: 'run-123', total_count: 3, success_count: 2, failed_count: 1,
  } } }), { traceID: 'run-123', totalCount: 3, successCount: 2, failedCount: 1 })
  assert.equal(parseSourceFetchSummary({ data: { result: {
    trace_id: 'run-123', total_count: 1, success_count: 2, failed_count: 0,
  } } }), null)
  assert.equal(parseSourceFetchSummary({ data: { result: {
    trace_id: 'run-123', total_count: '3', success_count: 2, failed_count: 1,
  } } }), null)
})
