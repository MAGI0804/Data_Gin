import test from 'node:test'
import assert from 'node:assert/strict'
import {
  authorizationExpiryISO,
  dataAuthorizationMessage,
  defaultAuthorizationExpiry,
  parseCreatedAuthorization,
  parseDataAuthorizationAccounts,
  parseDataAuthorizationAudits,
} from '../.test-dist/dataAuthorization.js'

test('parses safe account envelope and rejects malformed rows', () => {
  const parsed = parseDataAuthorizationAccounts({ data: {
    accounts: [
      { id: 7, account: 'partner', email: 'owner@example.com', nickname: '合作方', credentialStatus: 'ACTIVE', tokenPrefix: 'dg_open_abcd', issuedAt: '2026-07-28T08:00:00Z', createdAt: '2026-07-28T08:00:00Z', permissions: [{ permission: 'weather.read', label: '天气数据查询', scope: '全模块数据', status: 'ACTIVE', expiresAt: '2026-08-28T08:00:00Z' }] },
      { id: 0, account: 'invalid' },
      { id: 8, account: 'bad-permission', permissions: [{ permission: 'mall.write', status: 'ACTIVE' }] },
    ],
    pagination: { pageSize: 20, nextBeforeId: 7, hasMore: true },
  } })
  assert.equal(parsed.accounts.length, 2)
  assert.equal(parsed.accounts[0].permissions[0].permission, 'weather.read')
  assert.deepEqual(parsed.accounts[1].permissions, [])
  assert.equal(parsed.pagination.nextBeforeId, 7)
})

test('parses one-time token only from the expected envelope', () => {
  const parsed = parseCreatedAuthorization({ data: { account: { id: 9, account: 'partner' }, token: 'dg_open_secret', oneTimeTokenAvailable: true } })
  assert.equal(parsed.account?.id, 9)
  assert.equal(parsed.token, 'dg_open_secret')
  assert.equal(parsed.oneTimeTokenAvailable, true)
  assert.equal(parseCreatedAuthorization({ token: 'outside' }).token, '')
})

test('parses audit pagination and safe messages', () => {
  const parsed = parseDataAuthorizationAudits({ data: { audits: [{ id: 3, targetUserId: 9, targetAccount: 'partner', permission: 'weather.read', action: 'GRANT', actorUserId: 1, reason: '接入', createdAt: '2026-07-28T08:00:00Z' }], pagination: { hasMore: false } } })
  assert.equal(parsed.audits[0].action, 'GRANT')
  assert.equal(dataAuthorizationMessage({ msg: '禁止访问' }, 'fallback'), '禁止访问')
  assert.equal(dataAuthorizationMessage({ msg: 1 }, 'fallback'), 'fallback')
})

test('normalizes default and explicit expiry', () => {
  const now = new Date('2026-07-28T00:00:00Z')
  assert.equal(defaultAuthorizationExpiry(now).length, 16)
  assert.match(authorizationExpiryISO('2026-08-27T12:30'), /^2026-08-/)
  assert.equal(authorizationExpiryISO('invalid'), '')
})
