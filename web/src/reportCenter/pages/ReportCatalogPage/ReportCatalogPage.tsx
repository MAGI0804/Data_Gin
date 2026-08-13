import { useMemo, useState, type ReactNode } from 'react'
import { FileText, Plus, RefreshCcw, Search } from 'lucide-react'
import { DataTable, FeedbackState, FilterToolbar, PageCanvas, PageHeader, Section, StatusTag, type StatusTagTone } from '../../../ui'
import type { ReportCenterClient } from '../../api'
import { ReportConfigDrawer } from '../../components/ReportConfigDrawer/ReportConfigDrawer'
import type { ReportSummary } from '../../types'
import { useReportCatalog } from '../../useReportCatalog'
import { useReportDatasources } from '../../useReportDatasources'
import styles from './ReportCatalogPage.module.css'

export function ReportCatalogPage({ client, canManage, navigation }: { client: ReportCenterClient; canManage: boolean; navigation?: ReactNode }) {
  const [draftSearch, setDraftSearch] = useState('')
  const [search, setSearch] = useState('')
  const [drawerState, setDrawerState] = useState<{ open: boolean; report: ReportSummary | null }>({ open: false, report: null })
  const query = useMemo(() => ({ search, limit: 50 }), [search])
  const { items, loading, error, reload } = useReportCatalog(client, query)
  const datasources = useReportDatasources(client, canManage)

  return (
    <PageCanvas>
      {navigation}
      <PageHeader eyebrow="REPORT CENTER" title="报表目录" description="从 MySQL 配置中心查看报表定义、发布状态和当前版本。" actions={<><button className="ui-control-radius" type="button" onClick={reload} disabled={loading}><RefreshCcw aria-hidden="true" />刷新</button><button className="primary ui-control-radius" type="button" onClick={() => setDrawerState({ open: true, report: null })} disabled={!canManage}><Plus aria-hidden="true" />创建报表</button></>} />
      <FilterToolbar summary={`当前加载 ${items.length} 个报表`}>
        <form className={styles.search} onSubmit={(event) => { event.preventDefault(); setSearch(draftSearch.trim()) }}>
          <label><span>搜索报表</span><span className={styles.input}><Search aria-hidden="true" /><input className="ui-control-radius" type="search" value={draftSearch} onChange={(event) => setDraftSearch(event.currentTarget.value)} placeholder="名称或编码" /></span></label>
          <button className="ui-control-radius" type="submit" disabled={loading}>查询</button>
        </form>
      </FilterToolbar>
      <Section title="已登记报表" description="目录只展示后端真实返回的数据，不创建演示记录。" flush>
        {loading && items.length === 0 ? <FeedbackState kind="loading" title="正在加载报表目录" description="正在请求 GET /v1/reports。" /> : null}
        {error ? <FeedbackState kind="error" title="报表目录加载失败" description={error} action={<button className="ui-control-radius" type="button" onClick={reload}>重试</button>} /> : null}
        {!loading && !error && items.length === 0 ? <FeedbackState kind="empty" title="暂无报表" description="后端尚未返回可查看的报表定义。" action={canManage ? <button className="ui-control-radius" type="button" onClick={() => setDrawerState({ open: true, report: null })}>创建第一份报表</button> : null} /> : null}
        {items.length > 0 ? <ReportTable reports={items} onEdit={canManage ? (report) => setDrawerState({ open: true, report }) : undefined} /> : null}
      </Section>
      {drawerState.open ? <ReportConfigDrawer client={client} report={drawerState.report} datasources={datasources.items} datasourcesLoading={datasources.loading} datasourcesError={datasources.error} onSaved={reload} onClose={() => setDrawerState({ open: false, report: null })} /> : null}
    </PageCanvas>
  )
}

function ReportTable({ reports, onEdit }: { reports: ReportSummary[]; onEdit?: (report: ReportSummary) => void }) {
  return <DataTable minWidth={900} scrollLabel="报表目录列表"><thead><tr><th scope="col">报表</th><th scope="col">分类</th><th scope="col">数据源</th><th scope="col">版本</th><th scope="col">状态</th><th scope="col">更新时间</th><th scope="col">操作</th></tr></thead><tbody>{reports.map((report) => <tr key={report.id}><td><span className={styles.reportName}><FileText aria-hidden="true" /><span><strong>{report.name}</strong><code>{report.code}</code></span></span></td><td>{report.category || '未分类'}</td><td>{report.datasourceId ? `#${report.datasourceId}` : report.isOwner ? '-' : '共享'}</td><td>{report.currentPublishedVersionId ? `已发布 #${report.currentPublishedVersionId}` : report.lockVersion ? `草稿 v${report.lockVersion}` : report.currentDraftVersionId ? `草稿 #${report.currentDraftVersionId}` : '-'}</td><td><StatusTag tone={statusTone(report.status)}>{statusLabel(report.status)}</StatusTag></td><td>{formatDate(report.updatedAt)}</td><td><button type="button" onClick={() => onEdit?.(report)} disabled={!onEdit || !report.isOwner}>{report.isOwner ? '编辑配置' : '共享报表'}</button></td></tr>)}</tbody></DataTable>
}

function statusTone(status: ReportSummary['status']): StatusTagTone { return status === 'ACTIVE' ? 'success' : status === 'DISABLED' ? 'danger' : 'warning' }
function statusLabel(status: ReportSummary['status']) { return status === 'ACTIVE' ? '已发布' : status === 'DISABLED' ? '已停用' : '草稿' }
function formatDate(value: string | null) { if (!value) return '-'; const date = new Date(value); return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN') }
