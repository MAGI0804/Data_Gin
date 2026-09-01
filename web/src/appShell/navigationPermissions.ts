export function canViewNavigationItem(key: string, permissions: readonly string[]) {
  if (key === 'access_management') return permissions.some((permission) => permission.startsWith('system.'))
  if (key === 'report_catalog') return permissions.includes('report.read')
  if (key === 'report_configuration') return permissions.includes('report.read') && permissions.includes('report.manage')
  if (key === 'report_query') return permissions.includes('report.read') && permissions.includes('report.execute')
  if (key === 'report_exports') return permissions.includes('report.read') && permissions.includes('report.export')

  const required = navPermission(key)
  return !required || permissions.includes(required) || permissions.includes(required.replace(/\.read$/, '.manage'))
}

function navPermission(key: string) {
  const permissions: Record<string, string> = {
    business_overview: 'mall.read', store_info: 'mall.read', mall_weather: 'weather.read', sources: 'source.read', receive: 'data.read', pull_records: 'data.read',
    backfill: 'data.manage', youzan_distribution: 'data.manage', rules: 'pipeline.read', processed: 'data.read', methods: 'pipeline.read',
    destinations: 'delivery.read', tasks: 'delivery.read', push_policy: 'delivery.read', runs: 'pipeline.read', step_runs: 'pipeline.read',
    delivery_logs: 'delivery.read', excel_jobs: 'excel.read', excel_schemes: 'excel.read', excel_write: 'excel.manage',
    office_messages: 'office_message.read', office_push: 'office_message.read',
  }
  return permissions[key]
}
