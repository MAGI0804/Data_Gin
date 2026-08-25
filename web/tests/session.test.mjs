import assert from 'node:assert/strict'
import test from 'node:test'

import { classifyAuthResponse, isSuccessfulPayload, readEnvelopeToken, readSessionUser, readTokenInfo, verifySessionResponses } from '../.test-dist/api/auth.js'

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

test('classifies authentication responses using both HTTP and application status', () => {
  assert.deepEqual(classifyAuthResponse(true, 200, { code: 200, data: { token: 'session-token' } }), {
    status: 200,
    successful: true,
    token: 'session-token',
  })
  assert.deepEqual(classifyAuthResponse(true, 200, { code: 100500, data: { token: 'must-not-use' } }), {
    status: 500,
    successful: false,
    token: '',
  })
  assert.equal(classifyAuthResponse(false, 503, { code: 200, data: { token: 'must-not-use' } }).successful, false)
  assert.equal(classifyAuthResponse(true, 200, null).successful, false)
})

test('parses complete authenticated-session details', () => {
  assert.deepEqual(readSessionUser({
    code: 200,
    data: { id: 7, account: 'operator', nickname: '运营人员', phone: '138****8000', accountType: 'CONSOLE', status: 'ACTIVE', mallScopeMode: 'SELECTED', roles: [{ code: 'operator', name: '运营员' }], permissions: ['mall.read'], mallIds: [2, 9] },
  }), {
    id: 7,
    account: 'operator',
    nickname: '运营人员',
    phone: '138****8000',
    accountType: 'CONSOLE',
    status: 'ACTIVE',
    mallScopeMode: 'SELECTED',
    roles: [{ code: 'operator', name: '运营员' }],
    permissions: ['mall.read'],
    mallIds: [2, 9],
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

const validProfileResponse = {
  ok: true,
  data: { code: 200, data: { id: 7, account: 'operator', nickname: '运营人员', phone: '138****8000', accountType: 'CONSOLE', status: 'ACTIVE', mallScopeMode: 'SELECTED', roles: [], permissions: ['mall.read'], mallIds: [2] } },
}

const validTokenInfoResponse = {
  ok: true,
  data: { code: 200, data: { user_id: 7, token_type: 'refreshable', expire_time: 2_000_000_000, issued_time: 1_999_000_000, ttl: 10_000 } },
}

test('classifies only a complete matching session as valid', () => {
  const verification = verifySessionResponses(validProfileResponse, validTokenInfoResponse)
  assert.equal(verification.kind, 'valid')
  if (verification.kind === 'valid') assert.equal(verification.user.id, verification.tokenInfo.userID)
})

test('keeps temporary session validation failures distinct from invalid credentials', () => {
  assert.equal(verifySessionResponses({ ok: false, data: null, error: { kind: 'offline' } }, validTokenInfoResponse).kind, 'transient')
  assert.equal(verifySessionResponses(validProfileResponse, { ok: false, data: null, error: { kind: 'rate_limited' } }).kind, 'transient')
  assert.equal(verifySessionResponses({ ok: false, data: null, error: { kind: 'unauthorized' } }, validTokenInfoResponse).kind, 'unauthorized')
  assert.equal(verifySessionResponses(validProfileResponse, { ...validTokenInfoResponse, data: { code: 200, data: { user_id: 8, token_type: 'refreshable', expire_time: 2_000_000_000, issued_time: 1_999_000_000, ttl: 10_000 } } }).kind, 'invalid')
})
