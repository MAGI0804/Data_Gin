import assert from 'node:assert/strict'
import test from 'node:test'

import { effectiveApiStatus } from '../.test-dist/apiResponse.js'
import { apiURL } from '../.test-dist/apiURL.js'

test('uses application auth status when the legacy API returns HTTP 200', () => {
  assert.equal(effectiveApiStatus(200, { code: 100401 }), 401)
  assert.equal(effectiveApiStatus(200, { code: 100403 }), 403)
})

test('preserves real HTTP and unknown application statuses', () => {
  assert.equal(effectiveApiStatus(503, { code: 100403 }), 503)
  assert.equal(effectiveApiStatus(200, { code: 0 }), 200)
  assert.equal(effectiveApiStatus(200, { code: 199999 }), 200)
  assert.equal(effectiveApiStatus(0, 'network error'), 0)
})

test('builds same-origin API paths unless an explicit base URL is configured', () => {
  assert.equal(apiURL('/v1/malls/7/weather/hourly'), '/api/v1/malls/7/weather/hourly')
  assert.equal(apiURL('/api/auth/login'), '/api/auth/login')
  assert.equal(apiURL('v1/malls', 'https://example.test///'), 'https://example.test/api/v1/malls')
})
