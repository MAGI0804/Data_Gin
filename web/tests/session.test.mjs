import assert from 'node:assert/strict'
import test from 'node:test'

import { isSuccessfulPayload, readEnvelopeToken, readSessionUser, readTokenInfo } from '../.test-dist/api/auth.js'

test('accepts both supported successful API envelope codes', () => {
  assert.equal(isSuccessfulPayload({ code: 0, data: {} }), true)
  assert.equal(isSuccessfulPayload({ code: 200, data: {} }), true)
  assert.equal(isSuccessfulPayload({ code: 201, data: {} }), false)
  assert.equal(isSuccessfulPayload(null), false)
})

test('reads a refresh token only from a valid response envelope', () => {
  assert.equal(readEnvelopeToken({ code: 200, data: { token: 'new-token' } }), 'new-token')
  assert.equal(readEnvelopeToken({ code: 200, data: { token: '  ' } }), '')
  assert.equal(readEnvelopeToken({ code: 200, token: 'wrong-level' }), '')
})

test('parses complete authenticated-session details', () => {
  assert.deepEqual(readSessionUser({
    code: 200,
    data: { id: 7, account: 'operator', nickname: '运营人员', email: 'ops@example.test', consoleManaged: true },
  }), {
    id: 7,
    account: 'operator',
    nickname: '运营人员',
    email: 'ops@example.test',
    consoleManaged: true,
  })
  assert.equal(readSessionUser({ code: 200, data: { id: 0, account: 'operator' } }), null)
})

test('parses token expiry details without retaining the raw token', () => {
  const info = readTokenInfo({
    code: 200,
    data: { user_id: 7, token_type: 'refreshable', expire_time: 2_000_000_000, issued_time: 1_999_000_000, ttl: 10_000, token: 'never-retain-this-value' },
  })

  assert.deepEqual(info, {
    userID: 7,
    tokenType: 'refreshable',
    expireTime: 2_000_000_000,
    issuedTime: 1_999_000_000,
    ttl: 10_000,
  })
  assert.doesNotMatch(JSON.stringify(info), /never-retain-this-value/)
  assert.equal(readTokenInfo({ code: 200, data: { user_id: 7, token_type: 'refreshable', ttl: 10_000 } }), null)
})

test('parses the string user ID returned by the token-info endpoint', () => {
  assert.deepEqual(readTokenInfo({
    code: 0,
    data: { user_id: '7', token_type: 'r', expire_time: 2_000_000_000, issued_time: 1_999_000_000, ttl: 10_000 },
  }), {
    userID: 7,
    tokenType: 'r',
    expireTime: 2_000_000_000,
    issuedTime: 1_999_000_000,
    ttl: 10_000,
  })
  assert.equal(readTokenInfo({
    code: 0,
    data: { user_id: '7x', token_type: 'r', expire_time: 2_000_000_000, issued_time: 1_999_000_000, ttl: 10_000 },
  }), null)
})
