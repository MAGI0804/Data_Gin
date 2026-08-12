import { Download } from 'lucide-react'
import { FeedbackState, FilterToolbar, PageCanvas, PageHeader, Section, StatusTag } from '../../../ui'
import styles from './ReportExportsPage.module.css'

export function ReportExportsPage() {
  return <PageCanvas><PageHeader eyebrow="EXPORT ARCHIVE" title="导出中心" description="查看 Excel 生成、文件留存和 Oracle 结果清理状态。" actions={<button className="ui-control-radius" type="button" disabled><Download aria-hidden="true" />下载所选文件</button>} /><FilterToolbar summary={<StatusTag tone="neutral">导出接口尚未接入</StatusTag>}><div className={styles.filters}><label>任务状态<select className="ui-control-radius" disabled><option>全部状态</option></select></label><label>报表名称<input className="ui-control-radius" disabled placeholder="接口接入后可筛选" /></label></div></FilterToolbar><Section title="导出任务" description="文件状态、行数、清理进度和过期时间将来自后端真实任务。" flush><FeedbackState kind="empty" title="导出任务接口尚未接入" description="当前页面不会展示示例任务，也不会提供无效下载链接。" /></Section></PageCanvas>
}
