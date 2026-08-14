import { useEffect, useRef, useState } from 'react'
import { ArrowRight, GitCompareArrows } from 'lucide-react'
import { DataTable, Drawer, FeedbackState, StatusTag } from '../../../ui'
import { getReportVersionDiff, getReportVersions, type ReportCenterClient } from '../../api'
import type { ReportSummary, ReportVersionDiff, ReportVersionSummary } from '../../types'
import { createLatestRequestGuard } from './requestGuard'
import styles from './ReportVersionDrawer.module.css'

export function ReportVersionDrawer({ client, report, onClose }: { client: ReportCenterClient; report: ReportSummary | null; onClose: () => void }) {
  const [versions, setVersions] = useState<ReportVersionSummary[]>([])
  const [diff, setDiff] = useState<ReportVersionDiff | null>(null)
  const [selection, setSelection] = useState({ base: 0, target: 0 })
  const [page, setPage] = useState({ hasMore: false, nextAfterId: 0 })
  const [state, setState] = useState({ loading: false, error: '' })
  const requestGuard = useRef(createLatestRequestGuard())
  const [reloadKey, setReloadKey] = useState(0)
  useEffect(() => {
    if (!report) return requestGuard.current.cancel
    const request = requestGuard.current.begin()
    setState({ loading: true, error: '' })
    setVersions([])
    setDiff(null)
    void getReportVersions(client, report.id, 0, request.signal).then((response) => {
      if (!request.isCurrent()) return
      if (!response.ok) { setState({ loading: false, error: response.error }); return }
      const items = response.data.items
      setVersions(items)
      setPage({ hasMore: response.data.hasMore, nextAfterId: response.data.nextAfterId })
      setSelection({ base: items[1]?.id ?? 0, target: items[0]?.id ?? 0 })
      setState({ loading: false, error: '' })
    })
    return requestGuard.current.cancel
  }, [client, report, reloadKey])
  async function loadMore() {
    if (!report || !page.hasMore || state.loading) return
    const request = requestGuard.current.begin()
    setState({ loading: true, error: '' })
    const response = await getReportVersions(client, report.id, page.nextAfterId, request.signal)
    if (!request.isCurrent()) return
    if (!response.ok) { setState({ loading: false, error: response.error }); return }
    setVersions((items) => [...items, ...response.data.items])
    setPage({ hasMore: response.data.hasMore, nextAfterId: response.data.nextAfterId })
    setState({ loading: false, error: '' })
  }
  async function compare() {
    if (!report || !selection.base || !selection.target || selection.base === selection.target) return
    const request = requestGuard.current.begin()
    setState({ loading: true, error: '' })
    const response = await getReportVersionDiff(client, report.id, selection.base, selection.target, request.signal)
    if (!request.isCurrent()) return
    if (!response.ok) { setState({ loading: false, error: response.error }); return }
    setDiff(response.data)
    setState({ loading: false, error: '' })
  }
  function close() { requestGuard.current.cancel(); onClose() }
  return <Drawer open={Boolean(report)} title="版本差异" description={report ? `${report.name} · 仅比较 MySQL 不可变契约摘要，不访问 Oracle。` : undefined} size="wide" onClose={close}>{state.loading && !versions.length ? <FeedbackState kind="loading" title="正在读取版本历史" /> : null}{state.error ? <FeedbackState kind="error" title="版本数据不可用" description={state.error} action={!versions.length ? <button type="button" onClick={() => setReloadKey((value) => value + 1)}>重试</button> : undefined} /> : null}{!state.loading && !state.error && !versions.length ? <FeedbackState kind="empty" title="暂无已发布版本" /> : null}{versions.length ? <><div className={styles.compare}><VersionSelect label="基准版本" value={selection.base} versions={versions} disabled={state.loading} onChange={(base) => { setSelection((value) => ({ ...value, base })); setDiff(null) }} /><ArrowRight aria-hidden="true" /><VersionSelect label="目标版本" value={selection.target} versions={versions} disabled={state.loading} onChange={(target) => { setSelection((value) => ({ ...value, target })); setDiff(null) }} /><button type="button" disabled={state.loading || !selection.base || !selection.target || selection.base === selection.target} onClick={() => void compare()}><GitCompareArrows aria-hidden="true" />比较契约</button></div><DataTable density="compact" minWidth={760} scrollLabel="报表版本历史"><thead><tr><th scope="col">版本</th><th scope="col">发布时间</th><th scope="col">条件 / 字段 / 授权</th><th scope="col">契约指纹</th></tr></thead><tbody>{versions.map((item) => <tr key={item.id}><td>v{item.version}</td><td>{formatDate(item.publishedAt)}</td><td>{item.parameterCount} / {item.columnCount} / {item.grantCount}</td><td><code>{item.contractFingerprint}…</code></td></tr>)}</tbody></DataTable>{page.hasMore ? <div className={styles.more}><button type="button" disabled={state.loading} onClick={() => void loadMore()}>{state.loading ? '加载中…' : '加载更多版本'}</button></div> : null}{diff ? <DiffView diff={diff} /> : <p className={styles.hint}>选择两个版本查看过程、筛选条件、Excel 映射和权限摘要差异。</p>}</> : null}</Drawer>
}
function VersionSelect({ label, value, versions, disabled, onChange }: { label: string; value: number; versions: ReportVersionSummary[]; disabled: boolean; onChange: (value: number) => void }) { return <label>{label}<select value={value || ''} disabled={disabled} onChange={(event) => onChange(Number(event.currentTarget.value))}><option value="">请选择</option>{versions.map((item) => <option key={item.id} value={item.id}>v{item.version}</option>)}</select></label> }
function DiffView({ diff }: { diff: ReportVersionDiff }) { const total = diff.sections.reduce((count, section) => count + section.changes.length, 0); return <section className={styles.diff}><header><h3>v{diff.base.version} → v{diff.target.version}</h3><StatusTag tone={total ? 'warning' : 'success'}>{total ? `${total} 项变化` : '无差异'}</StatusTag></header>{diff.sections.map((section) => <section key={section.key}><h4>{section.label}</h4>{section.changes.length ? <ul>{section.changes.map((change) => <li key={change.key}><strong>{change.label}</strong><span><code>{String(change.before ?? '-')}</code><ArrowRight aria-hidden="true" /><code>{String(change.after ?? '-')}</code></span></li>)}</ul> : <p>无变化</p>}</section>)}</section> }
function formatDate(value: string | null) { if (!value) return '-'; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN') }
