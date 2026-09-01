import { useEffect, useMemo, useState } from 'react'
import { BadgeDollarSign, CalendarDays, Save } from 'lucide-react'
import { PageCanvas } from '../ui'
import styles from './BusinessOverviewPage.module.css'

type CashierID = 'all' | 'counter-01' | 'counter-02'

type PaymentSummary = {
  name: string
  amount: number
}

type ReconciliationRecord = {
  id: string
  date: string
  cashierID: Exclude<CashierID, 'all'>
  cashierName: string
  total: number
  received: number
  unsettled: number
  detail: string
}

type DailyBusinessSummary = {
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

const cashierOptions: Array<{ id: CashierID; label: string }> = [
  { id: 'all', label: '全部' },
  { id: 'counter-01', label: '收银机 01' },
  { id: 'counter-02', label: '收银机 02' },
]

const mockSummaries: Record<string, DailyBusinessSummary> = {
  '2026-09-01': {
    date: '2026-09-01',
    payments: [
      { name: '支付宝', amount: 663.3 },
      { name: '微信', amount: 716.4 },
    ],
    storeAmount: 1379.7,
    cloudAmount: 0,
    actualAmount: 1348.7,
    publicExpense: 31,
    depositAmount: 1200,
    presaleAmount: 0,
    records: [
      { id: 'REC-0901-01', date: '2026-09-01', cashierID: 'counter-01', cashierName: '收银机 01', total: 736.2, received: 720.2, unsettled: 16, detail: '支付宝 358.3 / 微信 377.9' },
      { id: 'REC-0901-02', date: '2026-09-01', cashierID: 'counter-02', cashierName: '收银机 02', total: 643.5, received: 628.5, unsettled: 15, detail: '支付宝 305.0 / 微信 338.5' },
    ],
  },
  '2026-08-31': {
    date: '2026-08-31',
    payments: [
      { name: '支付宝', amount: 582.5 },
      { name: '微信', amount: 649.8 },
    ],
    storeAmount: 1232.3,
    cloudAmount: 0,
    actualAmount: 1210.3,
    publicExpense: 22,
    depositAmount: 1000,
    presaleAmount: 0,
    records: [
      { id: 'REC-0831-01', date: '2026-08-31', cashierID: 'counter-01', cashierName: '收银机 01', total: 670.4, received: 658.4, unsettled: 12, detail: '支付宝 320.5 / 微信 349.9' },
      { id: 'REC-0831-02', date: '2026-08-31', cashierID: 'counter-02', cashierName: '收银机 02', total: 561.9, received: 551.9, unsettled: 10, detail: '支付宝 262.0 / 微信 299.9' },
    ],
  },
}

const emptySummary: DailyBusinessSummary = {
  date: '',
  payments: [],
  storeAmount: 0,
  cloudAmount: 0,
  actualAmount: 0,
  publicExpense: 0,
  depositAmount: 0,
  presaleAmount: 0,
  records: [],
}

export function BusinessOverviewPage() {
  const today = shanghaiDate(new Date())
  const yesterday = shiftDate(today, -1)
  const [selectedDate, setSelectedDate] = useState(today)
  const [cashierID, setCashierID] = useState<CashierID>('all')
  const [savedMessage, setSavedMessage] = useState('')
  const summary = useMemo(() => filterSummary(
    mockSummaries[selectedDate] ?? { ...emptySummary, date: selectedDate },
    cashierID,
  ), [cashierID, selectedDate])
  const totalAmount = summary.storeAmount + summary.cloudAmount

  function saveReconciliation() {
    setSavedMessage(`${selectedDate} 的对账数据已保存（模拟）`)
  }

  useEffect(() => {
    const handleShortcut = (event: KeyboardEvent) => {
      if (event.key !== 'F2') return
      event.preventDefault()
      setSavedMessage(`${selectedDate} 的对账数据已保存（模拟）`)
    }
    window.addEventListener('keydown', handleShortcut)
    return () => window.removeEventListener('keydown', handleShortcut)
  }, [selectedDate])

  return (
    <PageCanvas className={styles.page}>
      <header className={styles.titleBar}>
        <h1>营业概况</h1>
      </header>

      <section className={styles.overviewPanel} aria-label="营业概况日结核对">
        <div className={styles.filterBar}>
          <strong className={styles.filterTitle}>销售日期：</strong>
          <div className={styles.quickDates} aria-label="快捷选择销售日期">
            <button className={selectedDate === today ? styles.active : ''} type="button" onClick={() => setSelectedDate(today)}>今天</button>
            <button className={selectedDate === yesterday ? styles.active : ''} type="button" onClick={() => setSelectedDate(yesterday)}>昨天</button>
          </div>
          <label className={styles.dateField}>
            <span>指定日期</span>
            <span className={styles.dateInputWrap}>
              <input type="date" value={selectedDate} onChange={(event) => setSelectedDate(event.currentTarget.value)} />
              <CalendarDays aria-hidden="true" />
            </span>
          </label>
          <button className={styles.saveButton} type="button" onClick={saveReconciliation}>
            <Save aria-hidden="true" />保存 F2
          </button>
          <label className={styles.cashierField}>
            <span>收银机号：</span>
            <select value={cashierID} onChange={(event) => setCashierID(event.currentTarget.value as CashierID)}>
              {cashierOptions.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
            </select>
          </label>
        </div>

        {savedMessage ? <div className={styles.savedMessage} role="status" aria-live="polite">{savedMessage}</div> : null}

        <div className={styles.summaryArea}>
          <div className={styles.amountSummary}>
            <div className={styles.unsettledAmount}>
              <strong>{formatAmount(totalAmount)}</strong>
              <span>末日结金额（元）</span>
            </div>
            <div className={styles.channelAmounts}>
              <div><span>门店： </span><strong>{formatAmount(summary.storeAmount)}（元）</strong></div>
              <div><span>云仓： </span><strong>{formatAmount(summary.cloudAmount)}（元）</strong></div>
            </div>
            <div className={styles.reconciliationTip}>
              <span className={styles.tipIcon}><BadgeDollarSign aria-hidden="true" /></span>
              <p>亲，日结时请核对收款金额<br />是否正确哦～</p>
            </div>
          </div>

          <div className={styles.paymentSummary}>
            <div className={styles.paymentList}>
              {summary.payments.length > 0 ? summary.payments.map((payment) => (
                <div className={styles.paymentRow} key={payment.name}>
                  <span>{payment.name}</span>
                  <output>{formatAmount(payment.amount)}</output>
                </div>
              )) : <div className={styles.emptyPayments}>所选日期暂无收款数据</div>}
            </div>
            <div className={styles.adjustments}>
              <MetricField label="当日实存" value={summary.actualAmount} />
              <MetricField label="公关费用" value={summary.publicExpense} />
              <MetricField label="预订金额" value={summary.depositAmount} />
              <MetricField label="预售金额" value={summary.presaleAmount} />
            </div>
          </div>
        </div>

        <section className={styles.reconciliationSection} aria-labelledby="reconciliation-title">
          <h2 id="reconciliation-title">门店对账单</h2>
          <div className={styles.tableWrap} role="region" aria-label="门店对账记录，可横向滚动" tabIndex={0}>
            <table>
              <thead><tr><th scope="col">销售日期</th><th scope="col">收银机号</th><th scope="col">总计</th><th scope="col">实收金额</th><th scope="col">末日结金额</th><th scope="col">日结明细</th></tr></thead>
              <tbody>
                {summary.records.length > 0 ? summary.records.map((record) => (
                  <tr key={record.id}>
                    <td>{record.date}</td><td>{record.cashierName}</td><td>{formatAmount(record.total)}</td><td>{formatAmount(record.received)}</td><td>{formatAmount(record.unsettled)}</td><td>{record.detail}</td>
                  </tr>
                )) : <tr><td className={styles.emptyTable} colSpan={6}>当前筛选条件下暂无门店对账记录</td></tr>}
              </tbody>
            </table>
          </div>
        </section>
      </section>
    </PageCanvas>
  )
}

function MetricField({ label, value }: { label: string; value: number }) {
  return <label className={styles.metricField}><span>{label}</span><output>{formatAmount(value)}</output></label>
}

function filterSummary(summary: DailyBusinessSummary, cashierID: CashierID): DailyBusinessSummary {
  if (cashierID === 'all') return summary
  const records = summary.records.filter((record) => record.cashierID === cashierID)
  const ratio = summary.storeAmount > 0 ? records.reduce((total, record) => total + record.total, 0) / summary.storeAmount : 0
  return {
    ...summary,
    payments: summary.payments.map((payment) => ({ ...payment, amount: payment.amount * ratio })),
    storeAmount: records.reduce((total, record) => total + record.total, 0),
    actualAmount: records.reduce((total, record) => total + record.received, 0),
    publicExpense: records.reduce((total, record) => total + record.unsettled, 0),
    depositAmount: summary.depositAmount * ratio,
    presaleAmount: summary.presaleAmount * ratio,
    records,
  }
}

function formatAmount(value: number) {
  return new Intl.NumberFormat('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 2 }).format(value)
}

function shanghaiDate(date: Date) {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(date)
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]))
  return `${values.year}-${values.month}-${values.day}`
}

function shiftDate(date: string, days: number) {
  const value = new Date(`${date}T12:00:00+08:00`)
  value.setUTCDate(value.getUTCDate() + days)
  return shanghaiDate(value)
}
