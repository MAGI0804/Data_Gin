import assert from 'node:assert/strict'
import test from 'node:test'
import { createMockBusinessSummaries, emptyBusinessSummary, filterBusinessSummary, recordPaymentDetail, recordTotal } from '../.test-dist/businessOverview.js'
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

test('guards business overview navigation with mall permissions', () => {
  assert.equal(canViewNavigationItem('business_overview', []), false)
  assert.equal(canViewNavigationItem('business_overview', ['mall.read']), true)
  assert.equal(canViewNavigationItem('business_overview', ['mall.manage']), true)
})
