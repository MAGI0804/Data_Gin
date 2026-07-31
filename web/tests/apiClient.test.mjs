import assert from 'node:assert/strict'
import test from 'node:test'

import { createApiClient } from '../.test-dist/api/client.js'

function payload(data = {}) {
  return { code: 200, data }
}

function response(status, data = {}, headers = {}) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  })
}

function clientOptions(overrides = {}) {
  return {
    getToken: () => 'secret-token-that-must-not-leak',
    onTokenRefreshed: () => {},
    onUnauthorized: () => {},
    maxGetRetries: 0,
    wait: async () => {},
    ...overrides,
  }
}

test('GET requests do not include a body', async () => {
  let received
  const client = createApiClient(clientOptions({
    fetch: async (_input, init) => {
      received = init
      return response(200, payload({ items: [] }))
    },
  }))

  const result = await client.request('/v1/runs', { method: 'GET' })

  assert.equal(result.ok, true)
  assert.equal(received.method, 'GET')
  assert.equal(received.body, undefined)
})

test('concurrent unauthorized GETs use one refresh and replay once', async () => {
  let token = 'expired-token'
  let refreshes = 0
  let protectedRequests = 0
  let releasedRefresh
  const refreshGate = new Promise((resolve) => { releasedRefresh = resolve })
  const client = createApiClient(clientOptions({
    getToken: () => token,
    onTokenRefreshed: (nextToken) => { token = nextToken },
    fetch: async (input) => {
      const path = String(input)
      if (path.endsWith('/auth/token/refresh')) {
        refreshes += 1
        await refreshGate
        return response(200, payload({ token: 'fresh-token' }))
      }
      protectedRequests += 1
      return token === 'expired-token'
        ? response(401, { code: 100401, data: {} })
        : response(200, payload({ token: 'fresh-token', records: [] }))
    },
  }))

  const first = client.request('/v1/runs', { method: 'GET' })
  const second = client.request('/v1/delivery-logs', { method: 'GET' })
  await new Promise((resolve) => setImmediate(resolve))
  releasedRefresh()
  const [firstResult, secondResult] = await Promise.all([first, second])

  assert.equal(refreshes, 1)
  assert.equal(protectedRequests, 4)
  assert.equal(firstResult.ok, true)
  assert.equal(secondResult.ok, true)
})

test('failed refresh exits the session and does not loop', async () => {
  let unauthorized = 0
  let requests = 0
  const client = createApiClient(clientOptions({
    onUnauthorized: () => { unauthorized += 1 },
    fetch: async (input) => {
      requests += 1
      return String(input).endsWith('/auth/token/refresh')
        ? response(401, { code: 100401, data: {} })
        : response(401, { code: 100401, data: {} })
    },
  }))

  const result = await client.request('/v1/runs', { method: 'GET' })

  assert.equal(requests, 2)
  assert.equal(unauthorized, 1)
  assert.equal(result.ok, false)
  assert.equal(result.error?.kind, 'unauthorized')
})

test('unauthorized POST is not refreshed or replayed', async () => {
  let calls = 0
  let refreshes = 0
  let unauthorized = 0
  const client = createApiClient(clientOptions({
    onUnauthorized: () => { unauthorized += 1 },
    fetch: async (input) => {
      calls += 1
      if (String(input).endsWith('/auth/token/refresh')) refreshes += 1
      return response(401, { code: 100401, data: {} })
    },
  }))

  const result = await client.request('/v1/sources', { method: 'POST', body: { name: 'source' } })

  assert.equal(calls, 1)
  assert.equal(refreshes, 0)
  assert.equal(unauthorized, 1)
  assert.equal(result.error?.kind, 'unauthorized')
})

test('classifies public failures without exposing credentials', async (t) => {
  const cases = [
    ['forbidden', response(403, { code: 100403, data: { token: 'backend-secret' } })],
    ['rate_limited', response(429, { code: 429, data: { authorization: 'backend-secret' } }, { 'Retry-After': '12' })],
    ['server', response(500, { code: 500, data: { token: 'backend-secret' } })],
  ]

  for (const [expectedKind, nextResponse] of cases) {
    await t.test(String(expectedKind), async () => {
      const client = createApiClient(clientOptions({ fetch: async () => nextResponse }))
      const result = await client.request('/v1/protected', { method: 'GET', retry: false })
      const serialized = JSON.stringify(result)

      assert.equal(result.ok, false)
      assert.equal(result.error?.kind, expectedKind)
      assert.doesNotMatch(serialized, /secret-token-that-must-not-leak|backend-secret/i)
      if (expectedKind === 'rate_limited') assert.equal(result.error?.retryAfterSeconds, 12)
    })
  }
})

test('classifies network and caller cancellation failures', async () => {
  const networkClient = createApiClient(clientOptions({
    fetch: async () => { throw new TypeError('connection reset') },
  }))
  const network = await networkClient.request('/v1/runs', { method: 'GET', retry: false })
  assert.equal(network.error?.kind, 'network')
  assert.doesNotMatch(JSON.stringify(network), /secret-token-that-must-not-leak/i)

  const controller = new AbortController()
  controller.abort()
  const cancelledClient = createApiClient(clientOptions({
    fetch: async () => { throw new DOMException('aborted', 'AbortError') },
  }))
  const cancelled = await cancelledClient.request('/v1/runs', { method: 'GET', signal: controller.signal })
  assert.equal(cancelled.error?.kind, 'aborted')
  assert.doesNotMatch(JSON.stringify(cancelled), /secret-token-that-must-not-leak/i)
})
