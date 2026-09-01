import { useCallback, useEffect, useMemo, useState } from 'react'
import { BadgeDollarSign, CalendarDays, Save } from 'lucide-react'
import {
  createMockBusinessSummaries,
  emptyBusinessSummary,
  filterBusinessSummary,
  recordPaymentDetail,
  recordTotal,
  shiftBusinessDate,
  type CashierID,
} from '../businessOverview'
import { PageCanvas } from '../ui'
import styles from './BusinessOverviewPage.module.css'

const cashierOptions: Array<{ id: CashierID; label: string }> = [
  { id: 'all', label: '全部' },
  { id: 'counter-01', label: '收银机 01' },
  { id: 'counter-02', label: '收银机 02' },
]

export function BusinessOverviewPage() {
  const today = shanghaiDate(new Date())
  const yesterday = shiftBusinessDate(today, -1)
  const [selectedDate, setSelectedDate] = useState(today)
  const [cashierID, setCashierID] = useState<CashierID>('all')
  const [savedMessage, setSavedMessage] = useState('')
  const mockSummaries = useMemo(() => createMockBusinessSummaries(today), [today])
  const summary = useMemo(() => filterBusinessSummary(
    mockSummaries[selectedDate] ?? emptyBusinessSummary(selectedDate),
    cashierID,
  ), [cashierID, mockSummaries, selectedDate])
  const totalAmount = summary.storeAmount + summary.cloudAmount

  const saveReconciliation = useCallback(() => {
    setSavedMessage(`${selectedDate} 的对账数据已保存（模拟）`)
  }, [selectedDate])

  function changeDate(date: string) {
    setSelectedDate(date)
    setSavedMessage('')
  }

  function changeCashier(value: string) {
    if (!cashierOptions.some((option) => option.id === value)) return
    setCashierID(value as CashierID)
    setSavedMessage('')
  }

  useEffect(() => {
    const handleShortcut = (event: KeyboardEvent) => {
      if (event.key !== 'F2') return
      event.preventDefault()
      saveReconciliation()
    }
    window.addEventListener('keydown', handleShortcut)
    return () => window.removeEventListener('keydown', handleShortcut)
  }, [saveReconciliation])

  return (
    <PageCanvas className={styles.page}>
      <header className={styles.titleBar}><h1>营业概况</h1></header>

      <section className={styles.overviewPanel} aria-label="营业概况日结核对">
        <div className={styles.filterBar}>
          <strong className={styles.filterTitle}>销售日期：</strong>
          <div className={styles.quickDates} aria-label="快捷选择销售日期">
            <button aria-pressed={selectedDate === today} className={selectedDate === today ? styles.active : ''} type="button" onClick={() => changeDate(today)}>今天</button>
            <button aria-pressed={selectedDate === yesterday} className={selectedDate === yesterday ? styles.active : ''} type="button" onClick={() => changeDate(yesterday)}>昨天</button>
          </div>
          <label className={styles.dateField}>
            <span>指定日期</span>
            <span className={styles.dateInputWrap}>
              <input type="date" value={selectedDate} onChange={(event) => changeDate(event.currentTarget.value)} />
              <CalendarDays aria-hidden="true" />
            </span>
          </label>
          <button className={styles.saveButton} type="button" onClick={saveReconciliation}><Save aria-hidden="true" />保存 F2</button>
          <label className={styles.cashierField}>
            <span>收银机号：</span>
            <select value={cashierID} onChange={(event) => changeCashier(event.currentTarget.value)}>
              {cashierOptions.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
            </select>
          </label>
        </div>

        {savedMessage ? <div className={styles.savedMessage} role="status" aria-live="polite">{savedMessage}</div> : null}

        <div className={styles.summaryArea}>
          <div className={styles.amountSummary}>
            <div className={styles.unsettledAmount}><strong>{formatAmount(totalAmount)}</strong><span>末日结金额（元）</span></div>
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
                <div className={styles.paymentRow} key={payment.name}><span>{payment.name}</span><output>{formatAmount(payment.amount)}</output></div>
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
                    <td>{record.date}</td><td>{record.cashierName}</td><td>{formatAmount(recordTotal(record))}</td><td>{formatAmount(record.received)}</td><td>{formatAmount(record.unsettled)}</td><td>{recordPaymentDetail(record)}</td>
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

function formatAmount(value: number) {
  return new Intl.NumberFormat('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 2 }).format(value)
}

function shanghaiDate(date: Date) {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(date)
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]))
  return `${values.year}-${values.month}-${values.day}`
}
