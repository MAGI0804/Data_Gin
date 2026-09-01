import assert from 'node:assert/strict'
import test from 'node:test'

import {
  accountRoleIDs,
  accessManagementCapabilities,
  canReplaceAccountRoles,
  parseAccessAccounts,
  parseCreatedAccessRole,
  updateRoleSelection,
} from '../.test-dist/accessManagement.js'

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

test('parses a created custom role from the mutation envelope', () => {
  assert.deepEqual(parseCreatedAccessRole({ data: { role: {
    id: 12,
    code: 'offline_sales_viewer',
    name: '线下销售查看',
    description: '查看线下销售数据',
    status: 'ACTIVE',
    isSystem: false,
    isSuper: false,
    permissions: ['business_overview.read', 'report.read'],
  } } }), {
    id: 12,
    code: 'offline_sales_viewer',
    name: '线下销售查看',
    description: '查看线下销售数据',
    status: 'ACTIVE',
    isSystem: false,
    isSuper: false,
    permissions: ['business_overview.read', 'report.read'],
  })
  assert.equal(parseCreatedAccessRole({ data: { role: { id: 0 } } }), null)
  assert.equal(parseCreatedAccessRole({ data: { role: { id: 12, code: 'custom', name: '自定义', description: '', status: 'ACTIVE', isSystem: false, isSuper: false, permissions: ['report.read', null] } } }), null)
})

test('maps assigned account role codes to current role ids', () => {
  const account = parseAccessAccounts({ data: { accounts: [{
    id: 7,
    account: 'offline_sales',
    roles: [{ code: 'offline_sales_viewer', name: '线下销售查看' }],
  }] } })[0]
  assert.deepEqual(accountRoleIDs(account, [
    { id: 4, code: 'viewer', name: '只读', description: '', status: 'ACTIVE', isSystem: true, isSuper: false, permissions: [] },
    { id: 12, code: 'offline_sales_viewer', name: '线下销售查看', description: '', status: 'ACTIVE', isSystem: false, isSuper: false, permissions: ['business_overview.read'] },
  ]), [12])
})

test('checks account and role capabilities without implicit manage-to-read expansion', () => {
  assert.deepEqual(accessManagementCapabilities(['system.role.manage']), {
    canAccountRead: false,
    canAccountManage: false,
    canRoleRead: false,
    canRoleManage: true,
    canAuditRead: false,
  })
})

test('only allows replacing roles after the role catalog is ready', () => {
  assert.equal(canReplaceAccountRoles('idle'), false)
  assert.equal(canReplaceAccountRoles('loading'), false)
  assert.equal(canReplaceAccountRoles('error'), false)
  assert.equal(canReplaceAccountRoles('ready'), true)
})

test('updates role selection without dropping existing choices or adding duplicates', () => {
  assert.deepEqual(updateRoleSelection([4, 7], 12, true), [4, 7, 12])
  assert.deepEqual(updateRoleSelection([4, 7, 12], 12, true), [4, 7, 12])
  assert.deepEqual(updateRoleSelection([4, 7, 12], 7, false), [4, 12])
})
