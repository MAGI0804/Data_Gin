import assert from 'node:assert/strict'
import test from 'node:test'
import { businessOverviewPaymentsPath, createMockBusinessSummaries, emptyBusinessSummary, filterBusinessSummary, parseBusinessOverviewPayments, queryMockBusinessOverview, recordPaymentDetail, recordTotal } from '../.test-dist/businessOverview.js'
import { canViewNavigationItem } from '../.test-dist/appShell/navigationPermissions.js'

test('aggregates the full day from cashier-level payment records', () => {
  const summary = createMockBusinessSummaries('2026-09-01')['2026-09-01']
  assert.equal(summary.storeAmount, 1379.7)
  assert.deepEqual(summary.payments, [{ name: '支付宝', amount: 663.3 }, { name: '微信', amount: 716.4 }])
  assert.equal(summary.actualAmount, 1348.7)
  assert.equal(summary.records.reduce((total, record) => total + recordTotal(record), 0), summary.storeAmount)
  assert.equal(summary.records.reduce((total, record) => total + record.unsettled, 0), summary.unsettledAmount)
  assert.equal(summary.unsettledAmount, 1379.7)
})

test('keeps cashier payment totals and detail text on one source of truth', () => {
  const summary = createMockBusinessSummaries('2026-09-01')['2026-09-01']
  const cashier = filterBusinessSummary(summary, 'counter-01')
  assert.equal(cashier.storeAmount, 736.2)
  assert.equal(cashier.unsettledAmount, 736.2)
  assert.deepEqual(cashier.payments, [{ name: '支付宝', amount: 358.3 }, { name: '微信', amount: 377.9 }])
  assert.equal(cashier.publicExpense, 8)
  assert.equal(recordPaymentDetail(cashier.records[0]), '支付宝 358.3 / 微信 377.9')
})

test('builds mock data relative to the supplied business date', () => {
  const summaries = createMockBusinessSummaries('2027-01-01')
  assert.deepEqual(Object.keys(summaries), ['2027-01-01', '2026-12-31'])
  assert.equal(emptyBusinessSummary('2027-01-02').records.length, 0)
})

test('uses mall code as the mock business overview query input', () => {
  const firstMallCode = 'SH-PD-001'
  const secondMallCode = 'SH-JA-001'
  const firstMall = queryMockBusinessOverview({ mallCode: firstMallCode, date: '2026-09-01', cashierID: 'all' }, '2026-09-01')
  const secondMall = queryMockBusinessOverview({ mallCode: secondMallCode, date: '2026-09-01', cashierID: 'all' }, '2026-09-01')
  assert.equal(firstMall.storeAmount, 1379.7)
  assert.equal(secondMall.storeAmount, 1131.36)
  assert.equal(secondMall.unsettledAmount, secondMall.storeAmount)
  assert.equal(Math.round(secondMall.records.reduce((total, record) => total + record.unsettled, 0) * 100) / 100, secondMall.unsettledAmount)
  assert.notEqual(firstMall.records[0].id, secondMall.records[0].id)
  assert.match(secondMall.records[0].id, new RegExp(`^${secondMallCode}-`))
})

test('guards business overview navigation with its dedicated permission', () => {
  assert.equal(canViewNavigationItem('business_overview', []), false)
  assert.equal(canViewNavigationItem('business_overview', ['mall.read']), false)
  assert.equal(canViewNavigationItem('business_overview', ['business_overview.read']), true)
})

test('builds the payment query path from ISO date and mall code', () => {
  assert.equal(
    businessOverviewPaymentsPath('2026-09-01', 'abcn001a002'),
    '/v1/business-overview/payments?date=20260901&mallCode=ABCN001A002',
  )
  assert.throws(() => businessOverviewPaymentsPath('2026-02-30', 'ABCN001A002'))
  assert.throws(() => businessOverviewPaymentsPath('2026-09-01', "A' OR 1=1--"))
})

test('maps Oracle payment rows into the existing business summary', () => {
  const summary = parseBusinessOverviewPayments({ code: 0, data: {
    date: '20260901',
    mallCode: 'ABCN001A002',
    items: [
      { billDate: 20260901, storeId: 462, storeName: 'ALLBLU（上海徐汇区徐汇万科广场店）', storeCode: 'ABCN001A002', paywayId: 24, payAmount: 3164.76, paywayName: '微信' },
      { billDate: 20260901, storeId: 462, storeName: 'ALLBLU（上海徐汇区徐汇万科广场店）', storeCode: 'ABCN001A002', paywayId: 25, payAmount: 1836.22, paywayName: '支付宝' },
      { billDate: 20260901, storeId: 462, storeName: 'ALLBLU（上海徐汇区徐汇万科广场店）', storeCode: 'ABCN001A002', paywayId: 28, payAmount: 1627.2, paywayName: '刷卡POS支付' },
    ],
  } }, '2026-09-01', 'ABCN001A002')
  assert.ok(summary)
  assert.equal(summary.storeAmount, 6628.18)
  assert.equal(summary.actualAmount, 6628.18)
  assert.deepEqual(summary.payments, [
    { name: '微信', amount: 3164.76 },
    { name: '支付宝', amount: 1836.22 },
    { name: '刷卡POS支付', amount: 1627.2 },
  ])
  assert.equal(summary.records[0].cashierName, '全部收银机')
})

test('accepts empty payment data and rejects mismatched or malformed rows', () => {
  const empty = parseBusinessOverviewPayments({ code: 0, data: { date: '20260901', mallCode: 'ABCN001A002', items: [] } }, '2026-09-01', 'ABCN001A002')
  assert.ok(empty)
  assert.equal(empty.records.length, 0)
  assert.equal(parseBusinessOverviewPayments({ code: 0, data: { date: '20260902', mallCode: 'ABCN001A002', items: [] } }, '2026-09-01', 'ABCN001A002'), null)
  assert.equal(parseBusinessOverviewPayments({ code: 0, data: { date: '20260901', mallCode: 'ABCN001A002', items: [
    { billDate: 20260901, storeId: 462, storeName: 'store', storeCode: 'ABCN001A002', paywayId: 24, payAmount: '3164.76', paywayName: '微信' },
  ] } }, '2026-09-01', 'ABCN001A002'), null)
})
