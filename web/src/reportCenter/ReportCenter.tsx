import type { ReactNode } from 'react'
import { ReportCatalogPage } from './pages/ReportCatalogPage/ReportCatalogPage'
import { ReportConfigurationPage } from './pages/ReportConfigurationPage/ReportConfigurationPage'
import { ReportQueryPage } from './pages/ReportQueryPage/ReportQueryPage'
import { ReportExportsPage } from './pages/ReportExportsPage/ReportExportsPage'
import type { ReportCenterClient } from './api'
import type { ReportCenterSection } from './types'
import { FeedbackState, PageCanvas } from '../ui'
import styles from './ReportCenter.module.css'

const sections: Array<{ key: ReportCenterSection; label: string; permission: string }> = [
  { key: 'catalog', label: '报表目录', permission: 'report.read' },
  { key: 'configuration', label: '报表配置', permission: 'report.manage' },
  { key: 'query', label: '报表查询', permission: 'report.execute' },
  { key: 'exports', label: '导出中心', permission: 'report.export' },
]

export function ReportCenter({ client, permissions, section, onNavigate }: {
  client: ReportCenterClient
  permissions: string[]
  section: ReportCenterSection
  onNavigate: (section: ReportCenterSection) => void
}) {
  const visibleSections = sections.filter((item) => hasSectionPermission(permissions, item.key))
  const allowed = visibleSections.some((item) => item.key === section)
  const navigation = <nav className={styles.tabs} aria-label="报表中心模块">
    {visibleSections.map((item) => <button className={item.key === section ? styles.active : ''} type="button" aria-current={item.key === section ? 'page' : undefined} onClick={() => onNavigate(item.key)} key={item.key}>{item.label}</button>)}
  </nav>
  let content: ReactNode
  if (!allowed) content = <PageCanvas>{navigation}<FeedbackState kind="error" title="当前账号无权访问此报表模块" description="请从侧栏选择已授权模块，或联系管理员补充报表中心权限。" /></PageCanvas>
  else if (section === 'configuration') content = <ReportConfigurationPage client={client} navigation={navigation} />
  else if (section === 'query') content = <ReportQueryPage client={client} navigation={navigation} />
  else if (section === 'exports') content = <ReportExportsPage client={client} navigation={navigation} />
  else content = <ReportCatalogPage client={client} canManage={hasReportPermission(permissions, 'report.manage')} navigation={navigation} />

  return content
}

function hasReportPermission(permissions: string[], required: string) {
  return permissions.includes(required)
}

function hasSectionPermission(permissions: string[], section: ReportCenterSection) {
  if (!permissions.includes('report.read')) return false
  if (section === 'configuration') return permissions.includes('report.manage')
  if (section === 'query') return permissions.includes('report.execute')
  if (section === 'exports') return permissions.includes('report.export')
  return true
}
