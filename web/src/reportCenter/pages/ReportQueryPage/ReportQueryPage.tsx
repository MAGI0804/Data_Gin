import { useMemo, useState } from 'react'
import { Play } from 'lucide-react'
import { FeedbackState, FilterToolbar, PageCanvas, PageHeader, Section, StatusTag } from '../../../ui'
import type { ReportCenterClient } from '../../api'
import { useReportCatalog } from '../../useReportCatalog'
import styles from './ReportQueryPage.module.css'

export function ReportQueryPage({ client }: { client: ReportCenterClient }) {
  const query = useMemo(() => ({ limit: 100 }), [])
  const { items, loading, error, reload } = useReportCatalog(client, query)
  const published = items.filter((report) => report.status === 'ACTIVE')
  const [selectedId, setSelectedId] = useState('')
  return <PageCanvas><PageHeader eyebrow="ORACLE EXECUTION" title="报表查询" description="选择已发布报表并填写运行参数。执行接口未接入前不会创建任务或伪造结果。" /><FilterToolbar summary={<StatusTag tone="warning">执行能力待接入</StatusTag>}><label className={styles.selector}>选择报表<select className="ui-control-radius" value={selectedId} onChange={(event) => setSelectedId(event.currentTarget.value)} disabled={loading || published.length === 0}><option value="">请选择已发布报表</option>{published.map((report) => <option value={report.id} key={report.id}>{report.name}</option>)}</select></label></FilterToolbar><Section title="运行参数" description="参数控件将由已发布的 {{形参}} Schema 动态生成。"><div className={styles.parameterPlaceholder}><span>参数 Schema 接口尚未接入</span><button className="primary ui-control-radius" type="button" disabled><Play aria-hidden="true" />运行报表</button></div></Section><Section title="结果预览" flush>{loading ? <FeedbackState kind="loading" title="正在读取可用报表" /> : error ? <FeedbackState kind="error" title="可用报表加载失败" description={error} action={<button className="ui-control-radius" type="button" onClick={reload}>重试</button>} /> : <FeedbackState kind="empty" title={published.length === 0 ? '暂无已发布报表' : '尚未执行报表'} description={published.length === 0 ? '发布版本可用后会出现在上方选择器。' : '运行接口接入后，结果将在这里分页展示。'} />}</Section></PageCanvas>
}
