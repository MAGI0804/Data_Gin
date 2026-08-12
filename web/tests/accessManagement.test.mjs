import assert from 'node:assert/strict'
import test from 'node:test'

import { parseAccessAccounts } from '../.test-dist/accessManagement.js'

test('normalizes null account roles and mall ids to empty arrays', () => {
  const accounts = parseAccessAccounts({ data: { accounts: [{
    id: 1,
    account: 'admin',
    phone: '',
    nickname: '管理员',
    status: 'ACTIVE',
    mallScopeMode: 'ALL',
    roles: null,
    mallIds: null,
  }] } })

  assert.equal(accounts.length, 1)
  assert.deepEqual(accounts[0].roles, [])
  assert.deepEqual(accounts[0].mallIds, [])
})

test('keeps valid account roles and positive mall ids only', () => {
  const accounts = parseAccessAccounts({ data: { accounts: [{
    id: 7,
    account: 'operator',
    phone: '138****8000',
    nickname: '运营人员',
    status: 'ACTIVE',
    mallScopeMode: 'SELECTED',
    roles: [{ code: 'operator', name: '操作员' }, null],
    mallIds: [9, 0, '2'],
  }, null] } })

  assert.deepEqual(accounts, [{
    id: 7,
    account: 'operator',
    phone: '138****8000',
    nickname: '运营人员',
    status: 'ACTIVE',
    mallScopeMode: 'SELECTED',
    roles: [{ code: 'operator', name: '操作员' }],
    mallIds: [9],
  }])
})
