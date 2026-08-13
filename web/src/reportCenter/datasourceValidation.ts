import type { ReportDatasourceInput } from './types'

const datasourceCodePattern = /^[a-z][a-z0-9_]{2,63}$/

export function normalizeDatasourceCode(value: string) {
  return value.trim().toLowerCase()
}

export function validateDatasourceConnection(input: ReportDatasourceInput, editing: boolean) {
  if (!input.host.trim()) return '请填写主机地址。'
  if (!Number.isInteger(input.port) || input.port < 1 || input.port > 65535) return '端口必须是 1 到 65535 之间的整数。'
  if (Boolean(input.serviceName.trim()) === Boolean(input.sid.trim())) return 'Service Name 与 SID 必须且只能填写一个。'
  if (!input.username.trim()) return '请填写 Oracle 用户名。'
  if (!editing && !input.password) return '创建数据源时必须填写密码。'
  if (!Number.isInteger(input.connectTimeoutSeconds) || input.connectTimeoutSeconds < 1 || input.connectTimeoutSeconds > 60) return '连接超时必须是 1 到 60 秒之间的整数。'
  if (!Number.isInteger(input.queryTimeoutSeconds) || input.queryTimeoutSeconds < 1 || input.queryTimeoutSeconds > 86400) return '查询超时必须是 1 到 86400 秒之间的整数。'
  if (!Number.isInteger(input.maxOpenConnections) || input.maxOpenConnections < 1 || input.maxOpenConnections > 100) return '最大连接数必须是 1 到 100 之间的整数。'
  if (!Number.isInteger(input.maxIdleConnections) || input.maxIdleConnections < 0 || input.maxIdleConnections > input.maxOpenConnections) return '最大空闲连接必须是非负整数，且不能超过最大连接数。'
  if (!Number.isInteger(input.prefetchRows) || input.prefetchRows < 1 || input.prefetchRows > 10000) return '预取行数必须是 1 到 10000 之间的整数。'
  if (!Number.isInteger(input.arraySize) || input.arraySize < 1 || input.arraySize > 10000) return '批量数组大小必须是 1 到 10000 之间的整数。'
  return ''
}

export function validateDatasourceSave(input: ReportDatasourceInput, editing: boolean) {
  if (!input.name.trim()) return '请填写数据源名称。'
  if (!datasourceCodePattern.test(normalizeDatasourceCode(input.code))) return '数据源编码需以字母开头，只能包含字母、数字和下划线，长度为 3 到 64 位。'
  return validateDatasourceConnection(input, editing)
}
