import { useMemo, useState } from 'react'
import { Plus, Settings2 } from 'lucide-react'
import { FeedbackState, FilterToolbar, PageCanvas, PageHeader, Section, StatusTag } from '../../../ui'
import type { ReportCenterClient } from '../../api'
import { ReportConfigDrawer } from '../../components/ReportConfigDrawer/ReportConfigDrawer'
import { ReportDatasourcePanel } from '../../components/ReportDatasourcePanel/ReportDatasourcePanel'
import type { ReportSummary } from '../../types'
import { useReportCatalog } from '../../useReportCatalog'
import { useReportDatasources } from '../../useReportDatasources'
import styles from './ReportConfigurationPage.module.css'

export function ReportConfigurationPage({ client }: { client: ReportCenterClient }) {
  const query = useMemo(() => ({ limit: 50 }), [])
  const { items, loading, error, reload } = useReportCatalog(client, query)
  const datasources = useReportDatasources(client)
  const [selected, setSelected] = useState<ReportSummary | null>(null)
  const [creating, setCreating] = useState(false)

  return <PageCanvas><PageHeader eyebrow="MYSQL CONTRACTS" title="报表配置" description="维护 MySQL 中的数据源、过程映射、{{形参}}、结果字段、权限和 Excel 契约。" actions={<button className="primary ui-control-radius" type="button" onClick={() => setCreating(true)}><Plus aria-hidden="true" />新增配置</button>} /><FilterToolbar summary="Oracle 只负责执行与读取结果"><StatusTag tone="info">配置存储于 MySQL</StatusTag></FilterToolbar><Section title="Oracle 数据源管理" description="统一维护报表运行连接；凭据只在保存时传入并由服务端加密。" flush><ReportDatasourcePanel client={client} datasources={datasources.items} loading={datasources.loading} error={datasources.error} onReload={datasources.reload} /></Section><Section title="配置工作台" description="选择报表后在右侧维护完整草稿；发布时在线核验 Oracle 过程签名和结果 Schema。" flush>{loading ? <FeedbackState kind="loading" title="正在读取报表配置" /> : null}{error ? <FeedbackState kind="error" title="配置列表不可用" description={error} action={<button className="ui-control-radius" type="button" onClick={reload}>重试</button>} /> : null}{!loading && !error && items.length === 0 ? <FeedbackState kind="empty" title="暂无可配置报表" description="请先创建报表定义。" /> : null}{items.length > 0 ? <div className={styles.list}>{items.map((report) => <button className={styles.item} type="button" onClick={() => setSelected(report)} key={report.id}><Settings2 aria-hidden="true" /><span><strong>{report.name}</strong><small>{report.code} · {report.category || '未分类'}</small></span><StatusTag tone={report.status === 'ACTIVE' ? 'success' : 'warning'}>{report.status === 'ACTIVE' ? '已发布' : '草稿'}</StatusTag></button>)}</div> : null}</Section>{creating || selected ? <ReportConfigDrawer client={client} report={selected} datasources={datasources.items} datasourcesLoading={datasources.loading} datasourcesError={datasources.error} onSaved={reload} onClose={() => { setCreating(false); setSelected(null) }} /> : null}</PageCanvas>
}
