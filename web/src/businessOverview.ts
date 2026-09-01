export type CashierID = 'all' | 'counter-01' | 'counter-02'

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
  actualAmount: number
  publicExpense: number
  depositAmount: number
  presaleAmount: number
  records: ReconciliationRecord[]
}

type RecordTemplate = Omit<ReconciliationRecord, 'date'>

const todayRecords: RecordTemplate[] = [
  { id: 'REC-TODAY-01', cashierID: 'counter-01', cashierName: '收银机 01', payments: [{ name: '支付宝', amount: 358.3 }, { name: '微信', amount: 377.9 }], received: 720.2, unsettled: 16, publicExpense: 8, depositAmount: 0, presaleAmount: 0 },
  { id: 'REC-TODAY-02', cashierID: 'counter-02', cashierName: '收银机 02', payments: [{ name: '支付宝', amount: 305 }, { name: '微信', amount: 338.5 }], received: 628.5, unsettled: 15, publicExpense: 7, depositAmount: 0, presaleAmount: 0 },
]

const yesterdayRecords: RecordTemplate[] = [
  { id: 'REC-YESTERDAY-01', cashierID: 'counter-01', cashierName: '收银机 01', payments: [{ name: '支付宝', amount: 320.5 }, { name: '微信', amount: 349.9 }], received: 658.4, unsettled: 12, publicExpense: 6, depositAmount: 0, presaleAmount: 0 },
  { id: 'REC-YESTERDAY-02', cashierID: 'counter-02', cashierName: '收银机 02', payments: [{ name: '支付宝', amount: 262 }, { name: '微信', amount: 299.9 }], received: 551.9, unsettled: 10, publicExpense: 5, depositAmount: 0, presaleAmount: 0 },
]

export function createMockBusinessSummaries(today: string): Record<string, DailyBusinessSummary> {
  const yesterday = shiftBusinessDate(today, -1)
  return {
    [today]: summarizeBusinessDay(today, recordsForDate(todayRecords, today)),
    [yesterday]: summarizeBusinessDay(yesterday, recordsForDate(yesterdayRecords, yesterday)),
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
    actualAmount: sum(records.map((record) => record.received)),
    publicExpense: sum(records.map((record) => record.publicExpense)),
    depositAmount: sum(records.map((record) => record.depositAmount)),
    presaleAmount: sum(records.map((record) => record.presaleAmount)),
    records,
  }
}

function recordsForDate(records: RecordTemplate[], date: string) {
  return records.map((record) => ({ ...record, date, payments: record.payments.map((payment) => ({ ...payment })) }))
}

function sum(values: number[]) {
  return values.reduce((total, value) => total + value, 0)
}

function formatPlainAmount(value: number) {
  return new Intl.NumberFormat('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 2 }).format(value)
}

function shanghaiDate(date: Date) {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(date)
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]))
  return `${values.year}-${values.month}-${values.day}`
}
