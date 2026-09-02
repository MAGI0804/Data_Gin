import assert from 'node:assert/strict'
import test from 'node:test'

import {
  accountRoleIDs,
  accessMallCatalogPath,
  accessMallScopeRequest,
  accessManagementCapabilities,
  canReplaceAccountRoles,
  mergeAccessMalls,
  parseAccessMallCatalog,
  parseAccessAccounts,
  parseCreatedAccessRole,
  updateRoleSelection,
} from '../.test-dist/accessManagement.js'

test('builds a valid account mall scope request', () => {
  assert.deepEqual(accessMallScopeRequest('ALL', [7, 8]), { mallScopeMode: 'ALL', mallIds: [] })
  assert.deepEqual(accessMallScopeRequest('SELECTED', [9, 2, 9]), { mallScopeMode: 'SELECTED', mallIds: [9, 2] })
  assert.equal(accessMallScopeRequest('SELECTED', []), null)
  assert.equal(accessMallScopeRequest('SELECTED', [0, Number.NaN]), null)
})

test('parses and merges the grantable mall catalog', () => {
  assert.equal(accessMallCatalogPath(), '/v1/access/malls?limit=200')
  assert.equal(accessMallCatalogPath(8, 25), '/v1/access/malls?limit=25&afterId=8')
  assert.throws(() => accessMallCatalogPath(0, 201), /invalid access mall pagination/)
  const page = parseAccessMallCatalog({ data: { items: [
    { id: 8, mallCode: 'SH-008', nameCn: ' 浦东商场 ' },
    { id: 3, mallCode: 'SH-003', nameCn: '徐汇商场' },
  ], nextAfterId: 8 } })
  assert.deepEqual(page, { items: [
    { id: 8, mallCode: 'SH-008', nameCn: '浦东商场' },
    { id: 3, mallCode: 'SH-003', nameCn: '徐汇商场' },
  ], nextAfterId: 8 })
  assert.deepEqual(mergeAccessMalls([{ id: 3, mallCode: 'OLD', nameCn: '旧名称' }], page.items), [
    { id: 3, mallCode: 'SH-003', nameCn: '徐汇商场' },
    { id: 8, mallCode: 'SH-008', nameCn: '浦东商场' },
  ])
  assert.equal(parseAccessMallCatalog({ data: { items: [{ id: 0 }], nextAfterId: 0 } }), null)
})

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
