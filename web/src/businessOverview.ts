export type CashierID = 'all' | 'counter-01' | 'counter-02'

export type BusinessOverviewQuery = {
  mallCode: string
  date: string
  cashierID: CashierID
}

export type PaymentSummary = { name: string; amount: number }

export type ReconciliationRecord = {
  id: string
  date: string
  cashierID: Exclude<CashierID, 'all'>
  cashierName: string
  payments: PaymentSummary[]
  received: number
  unsettled: number
  publicExpense: number
  depositAmount: number
  presaleAmount: number
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

function shanghaiDate(date: Date) {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(date)
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]))
  return `${values.year}-${values.month}-${values.day}`
}
