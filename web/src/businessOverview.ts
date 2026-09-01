export type CashierID = 'all' | 'counter-01' | 'counter-02'

export type BusinessOverviewQuery = {
  mallCode: string
  date: string
  cashierID: CashierID
}

export type PaymentSummary = { name: string; amount: number }

export type BusinessOverviewMall = { id: number; mallCode: string; nameCn: string }

export type BusinessOverviewMallList = { items: BusinessOverviewMall[]; nextAfterId: number }

export type ReconciliationRecord = {
  id: string
  date: string
  cashierID: CashierID
  cashierName: string
  payments: PaymentSummary[]
  received: number
  unsettled: number
  publicExpense: number
  depositAmount: number
  presaleAmount: number
}

type BusinessOverviewPaymentItem = {
  billDate: number
  storeId: number
  storeName: string
  storeCode: string
  paywayId: number
  payAmount: number
  paywayName: string
}

export type DailyBusinessSummary = {
  date: string
  payments: PaymentSummary[]
  storeAmount: number
  cloudAmount: number
  unsettledAmount: number
  actualAmount: number
  publicExpense: number
  depositAmount: number
  presaleAmount: number
  records: ReconciliationRecord[]
}

type RecordTemplate = Omit<ReconciliationRecord, 'date'>

const todayRecords: RecordTemplate[] = [
  { id: 'REC-TODAY-01', cashierID: 'counter-01', cashierName: '收银机 01', payments: [{ name: '支付宝', amount: 358.3 }, { name: '微信', amount: 377.9 }], received: 720.2, unsettled: 736.2, publicExpense: 8, depositAmount: 0, presaleAmount: 0 },
  { id: 'REC-TODAY-02', cashierID: 'counter-02', cashierName: '收银机 02', payments: [{ name: '支付宝', amount: 305 }, { name: '微信', amount: 338.5 }], received: 628.5, unsettled: 643.5, publicExpense: 7, depositAmount: 0, presaleAmount: 0 },
]

const yesterdayRecords: RecordTemplate[] = [
  { id: 'REC-YESTERDAY-01', cashierID: 'counter-01', cashierName: '收银机 01', payments: [{ name: '支付宝', amount: 320.5 }, { name: '微信', amount: 349.9 }], received: 658.4, unsettled: 670.4, publicExpense: 6, depositAmount: 0, presaleAmount: 0 },
  { id: 'REC-YESTERDAY-02', cashierID: 'counter-02', cashierName: '收银机 02', payments: [{ name: '支付宝', amount: 262 }, { name: '微信', amount: 299.9 }], received: 551.9, unsettled: 561.9, publicExpense: 5, depositAmount: 0, presaleAmount: 0 },
]

export function createMockBusinessSummaries(today: string, mallCode = 'SH-PD-001'): Record<string, DailyBusinessSummary> {
  const yesterday = shiftBusinessDate(today, -1)
  const factor = mallMockFactor(mallCode)
  return {
    [today]: summarizeBusinessDay(today, recordsForDate(todayRecords, today, mallCode, factor)),
    [yesterday]: summarizeBusinessDay(yesterday, recordsForDate(yesterdayRecords, yesterday, mallCode, factor)),
  }
}

export function queryMockBusinessOverview(query: BusinessOverviewQuery, today: string): DailyBusinessSummary {
  const summaries = createMockBusinessSummaries(today, query.mallCode)
  return filterBusinessSummary(
    summaries[query.date] ?? emptyBusinessSummary(query.date),
    query.cashierID,
  )
}

export function businessOverviewPaymentsPath(date: string, mallCode: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || !validISODate(date)) throw new Error('invalid business date')
  const normalizedMallCode = mallCode.trim().toUpperCase()
  if (!/^[A-Z0-9][A-Z0-9_-]{1,63}$/.test(normalizedMallCode)) throw new Error('invalid mall code')
  const query = new URLSearchParams({ date: date.replace(/-/g, ''), mallCode: normalizedMallCode })
  return `/v1/business-overview/payments?${query.toString()}`
}

export function businessOverviewMallsPath(afterID = 0, limit = 50) {
  if (!Number.isSafeInteger(afterID) || afterID < 0 || !Number.isSafeInteger(limit) || limit < 1 || limit > 200) {
    throw new Error('invalid business overview mall pagination')
  }
  const query = new URLSearchParams({ limit: String(limit) })
  if (afterID > 0) query.set('afterId', String(afterID))
  return `/v1/business-overview/malls?${query.toString()}`
}

export function parseBusinessOverviewMalls(payload: unknown): BusinessOverviewMallList | null {
  if (!isRecord(payload) || (payload.code !== 0 && payload.code !== 200) || !isRecord(payload.data) ||
    !Array.isArray(payload.data.items) || !Number.isSafeInteger(payload.data.nextAfterId) || Number(payload.data.nextAfterId) < 0) return null
  const items: BusinessOverviewMall[] = []
  for (const item of payload.data.items) {
    if (!isRecord(item) || !Number.isSafeInteger(item.id) || Number(item.id) < 1 ||
      typeof item.mallCode !== 'string' || !/^[A-Z0-9][A-Z0-9_-]{1,63}$/.test(item.mallCode) ||
      typeof item.nameCn !== 'string' || !item.nameCn.trim()) return null
    items.push({ id: Number(item.id), mallCode: item.mallCode, nameCn: item.nameCn.trim() })
  }
  return { items, nextAfterId: Number(payload.data.nextAfterId) }
}

export function mergeBusinessOverviewMalls(current: BusinessOverviewMall[], incoming: BusinessOverviewMall[]) {
  const byID = new Map(current.map((mall) => [mall.id, mall]))
  incoming.forEach((mall) => byID.set(mall.id, mall))
  return Array.from(byID.values())
}

export function parseBusinessOverviewPayments(
  payload: unknown,
  expectedDate: string,
  expectedMallCode: string,
): DailyBusinessSummary | null {
  if (!isRecord(payload) || (payload.code !== 0 && payload.code !== 200) || !isRecord(payload.data)) return null
  const data = payload.data
  const expectedBillDate = expectedDate.replace(/-/g, '')
  const normalizedMallCode = expectedMallCode.trim().toUpperCase()
  if (data.date !== expectedBillDate || data.mallCode !== normalizedMallCode || !Array.isArray(data.items)) return null

  const items: BusinessOverviewPaymentItem[] = []
  for (const value of data.items) {
    if (!isRecord(value) || value.billDate !== Number(expectedBillDate) || value.storeCode !== normalizedMallCode ||
      !Number.isSafeInteger(value.storeId) || !Number.isSafeInteger(value.paywayId) ||
      typeof value.storeName !== 'string' || !value.storeName.trim() || typeof value.paywayName !== 'string' || !value.paywayName.trim() ||
      typeof value.payAmount !== 'number' || !Number.isFinite(value.payAmount)) return null
    items.push({
      billDate: Number(value.billDate),
      storeId: Number(value.storeId),
      storeName: value.storeName.trim(),
      storeCode: value.storeCode,
      paywayId: Number(value.paywayId),
      payAmount: value.payAmount,
      paywayName: value.paywayName.trim(),
    })
  }

  const paymentTotals = new Map<string, number>()
  items.forEach((item) => paymentTotals.set(item.paywayName, sum([paymentTotals.get(item.paywayName) ?? 0, item.payAmount])))
  const payments = Array.from(paymentTotals, ([name, amount]) => ({ name, amount }))
  const total = sum(payments.map((payment) => payment.amount))
  const records: ReconciliationRecord[] = items.length === 0 ? [] : [{
    id: `${normalizedMallCode}-${expectedBillDate}-all`,
    date: expectedDate,
    cashierID: 'all',
    cashierName: '全部收银机',
    payments,
    received: total,
    unsettled: total,
    publicExpense: 0,
    depositAmount: 0,
    presaleAmount: 0,
  }]
  return {
    date: expectedDate,
    payments,
    storeAmount: total,
    cloudAmount: 0,
    unsettledAmount: total,
    actualAmount: total,
    publicExpense: 0,
    depositAmount: 0,
    presaleAmount: 0,
    records,
  }
}

export function emptyBusinessSummary(date: string): DailyBusinessSummary {
  return summarizeBusinessDay(date, [])
}

export function filterBusinessSummary(summary: DailyBusinessSummary, cashierID: CashierID): DailyBusinessSummary {
  if (cashierID === 'all') return summary
  return summarizeBusinessDay(summary.date, summary.records.filter((record) => record.cashierID === cashierID), summary.cloudAmount)
}

export function recordTotal(record: ReconciliationRecord) {
  return sum(record.payments.map((payment) => payment.amount))
}

export function recordPaymentDetail(record: ReconciliationRecord) {
  return record.payments.map((payment) => `${payment.name} ${formatPlainAmount(payment.amount)}`).join(' / ')
}

export function shiftBusinessDate(date: string, days: number) {
  const value = new Date(`${date}T12:00:00+08:00`)
  value.setUTCDate(value.getUTCDate() + days)
  return shanghaiDate(value)
}

function summarizeBusinessDay(date: string, records: ReconciliationRecord[], cloudAmount = 0): DailyBusinessSummary {
  const paymentTotals = new Map<string, number>()
  records.forEach((record) => record.payments.forEach((payment) => paymentTotals.set(payment.name, (paymentTotals.get(payment.name) ?? 0) + payment.amount)))
  return {
    date,
    payments: Array.from(paymentTotals, ([name, amount]) => ({ name, amount })),
    storeAmount: sum(records.map(recordTotal)),
    cloudAmount,
    unsettledAmount: sum(records.map((record) => record.unsettled)) + cloudAmount,
    actualAmount: sum(records.map((record) => record.received)),
    publicExpense: sum(records.map((record) => record.publicExpense)),
    depositAmount: sum(records.map((record) => record.depositAmount)),
    presaleAmount: sum(records.map((record) => record.presaleAmount)),
    records,
  }
}

function recordsForDate(records: RecordTemplate[], date: string, mallCode: string, factor: number) {
  return records.map((record) => {
    const payments = record.payments.map((payment) => ({ ...payment, amount: scaledAmount(payment.amount, factor) }))
    return {
      ...record,
      id: `${mallCode}-${record.id}`,
      date,
      payments,
      received: scaledAmount(record.received, factor),
      unsettled: sum(payments.map((payment) => payment.amount)),
      publicExpense: scaledAmount(record.publicExpense, factor),
      depositAmount: scaledAmount(record.depositAmount, factor),
      presaleAmount: scaledAmount(record.presaleAmount, factor),
    }
  })
}

function mallMockFactor(mallCode: string) {
  const factors: Record<string, number> = {
    'SH-PD-001': 1,
    'SH-JA-001': 0.82,
    'SH-XH-001': 1.16,
  }
  if (factors[mallCode]) return factors[mallCode]
  const hash = Array.from(mallCode).reduce((total, character) => (total * 31 + character.charCodeAt(0)) % 35, 0)
  return 0.82 + hash / 100
}

function scaledAmount(value: number, factor: number) {
  return Math.round(value * factor * 100) / 100
}

function sum(values: number[]) {
  return Math.round(values.reduce((total, value) => total + value, 0) * 100) / 100
}

function formatPlainAmount(value: number) {
  return new Intl.NumberFormat('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 2 }).format(value)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function validISODate(value: string) {
  const parsed = new Date(`${value}T00:00:00Z`)
  return !Number.isNaN(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value
}

function shanghaiDate(date: Date) {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(date)
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]))
  return `${values.year}-${values.month}-${values.day}`
}
