import { useMemo, useState, type ReactNode } from 'react'
import { GitCompareArrows, History, Plus, Settings2 } from 'lucide-react'
import { Button, FeedbackState, FilterToolbar, PageCanvas, PageHeader, Section, StatusTag } from '../../../ui'
import type { ReportCenterClient } from '../../api'
import { ReportConfigDrawer } from '../../components/ReportConfigDrawer/ReportConfigDrawer'
import { ReportValidationResultDrawer } from '../../components/ReportValidationResultDrawer/ReportValidationResultDrawer'
import { ReportVersionDrawer } from '../../components/ReportVersionDrawer/ReportVersionDrawer'
import { ReportAuditDrawer } from '../../components/ReportAuditDrawer/ReportAuditDrawer'
import { ReportDatasourcePanel } from '../../components/ReportDatasourcePanel/ReportDatasourcePanel'
import type { ReportPublication, ReportSummary } from '../../types'
import { useReportCatalog } from '../../useReportCatalog'
import { useReportDatasources } from '../../useReportDatasources'
import styles from './ReportConfigurationPage.module.css'

export function ReportConfigurationPage({ client, navigation }: { client: ReportCenterClient; navigation?: ReactNode }) {
  const query = useMemo(() => ({ limit: 50 }), [])
  const { items, loading, loadingMore, error, hasMore, reload, loadMore } = useReportCatalog(client, query)
  const ownedItems = items.filter((item) => item.isOwner)
  const datasources = useReportDatasources(client)
  const [selected, setSelected] = useState<ReportSummary | null>(null)
  const [creating, setCreating] = useState(false)
  const [auditsOpen, setAuditsOpen] = useState(false)
  const [publication, setPublication] = useState<ReportPublication | null>(null)
  const [versionReport, setVersionReport] = useState<ReportSummary | null>(null)

  return <PageCanvas>{navigation}<PageHeader eyebrow="MYSQL CONTRACTS" title="报表配置" description="维护 MySQL 中的数据源、Oracle 过程绑定、筛选条件、权限和 Excel 映射。" actions={<><button type="button" onClick={() => setAuditsOpen(true)}><History aria-hidden="true" />审计记录</button><Button variant="primary" onClick={() => setCreating(true)}><Plus aria-hidden="true" />新增配置</Button></>} /><FilterToolbar summary="Oracle 只负责执行 JSON 游标过程与读取结果"><StatusTag tone="info">配置存储于 MySQL</StatusTag></FilterToolbar><Section title="Oracle 数据源管理" description="统一维护报表运行连接；凭据只在保存时传入并由服务端加密。" flush><ReportDatasourcePanel client={client} datasources={datasources.items} loading={datasources.loading} error={datasources.error} onReload={datasources.reload} /></Section><Section title="配置工作台" description="选择报表后查询并绑定 Oracle 已有过程；发布时在线核验 JSON 输入与 REF CURSOR 协议。" flush>{loading ? <FeedbackState kind="loading" title="正在读取报表配置" /> : null}{error ? <FeedbackState kind="error" title="配置列表不可用" description={error} action={<button type="button" onClick={reload}>重试</button>} /> : null}{!loading && !error && ownedItems.length === 0 ? <FeedbackState kind="empty" title={hasMore ? '当前批次暂无自有报表' : '暂无可配置报表'} description={hasMore ? '当前批次均为共享报表，可继续加载后续配置。' : '共享报表仅在目录和查询页展示；请创建自己的报表定义。'} /> : null}{ownedItems.length > 0 ? <div className={styles.list}>{ownedItems.map((report) => <div className={styles.item} key={report.id}><button type="button" onClick={() => setSelected(report)}><Settings2 aria-hidden="true" /><span><strong>{report.name}</strong><small>{report.code} · {report.category || '未分类'}</small></span></button><button type="button" aria-label={`查看 ${report.name} 版本差异`} onClick={() => setVersionReport(report)}><GitCompareArrows aria-hidden="true" /></button><StatusTag tone={report.status === 'ACTIVE' ? 'success' : 'warning'}>{report.status === 'ACTIVE' ? '已发布' : '草稿'}</StatusTag></div>)}</div> : null}{items.length > 0 && hasMore ? <div className={styles.pagination}><button type="button" onClick={() => void loadMore()} disabled={loadingMore}>{loadingMore ? '正在加载…' : '加载更多配置'}</button></div> : null}</Section>{creating || selected ? <ReportConfigDrawer client={client} report={selected} datasources={datasources.items} datasourcesLoading={datasources.loading} datasourcesError={datasources.error} onPublished={setPublication} onSaved={reload} onClose={() => { setCreating(false); setSelected(null) }} /> : null}<ReportValidationResultDrawer publication={publication} onClose={() => setPublication(null)} /><ReportVersionDrawer client={client} report={versionReport} onClose={() => setVersionReport(null)} /><ReportAuditDrawer client={client} open={auditsOpen} reports={ownedItems} onClose={() => setAuditsOpen(false)} /></PageCanvas>
}
