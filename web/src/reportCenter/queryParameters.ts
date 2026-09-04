import type { ReportExport, ReportExportStatus, ReportParameter, ReportResultPage, ReportResultQuery, ReportRun, ReportRunStatus } from './types'

export const terminalReportRunStatuses = new Set<ReportRunStatus>(['SUCCEEDED', 'FAILED', 'CANCELLED', 'EXPORTED', 'RESULT_PURGED', 'SUPERSEDED'])
export const terminalReportExportStatuses = new Set<ReportExportStatus>(['READY', 'FAILED', 'CANCELLED', 'EXPIRED'])

export type NewReportRunState = {
  values: Record<string, unknown>
  run: ReportRun | null
  result: ReportResultPage | null
  reportExport: ReportExport | null
  resultQuery: ReportResultQuery
  appliedQuery: ReportResultQuery
  cursorHistory: string[]
  cursorIndex: number
  filtersOpen: boolean
  parametersOpen: boolean
  operation: { busy: boolean; error: string }
}

export function visibleReportParameters(parameters: ReportParameter[]) {
  return parameters.filter((parameter) => !parameter.systemInjected)
}

export function initialReportParameterValues(parameters: ReportParameter[]) {
  return visibleReportParameters(parameters).reduce<Record<string, unknown>>((result, parameter) => {
    result[parameter.code] = editableDefaultValue(parameter)
    return result
  }, {})
}

export function canStartNewReportRun(runStatus: ReportRunStatus, reportExport: Pick<ReportExport, 'status' | 'purgedAt'> | null, busy: boolean) {
  const runFinished = terminalReportRunStatuses.has(runStatus)
  const exportFinished = isReportExportSettled(reportExport)
  return !busy && runFinished && exportFinished
}

export function isReportExportSettled(reportExport: Pick<ReportExport, 'status' | 'purgedAt'> | null) {
  if (reportExport === null) return true
  if (!terminalReportExportStatuses.has(reportExport.status)) return false
  return reportExport.status !== 'READY' || reportExport.purgedAt !== null
}

export function buildNewReportRunState(parameters: ReportParameter[]): NewReportRunState {
  return {
    values: initialReportParameterValues(parameters),
    run: null,
    result: null,
    reportExport: null,
    resultQuery: { filters: [], sort: [] },
    appliedQuery: { filters: [], sort: [] },
    cursorHistory: [''],
    cursorIndex: 0,
    filtersOpen: false,
    parametersOpen: true,
    operation: { busy: false, error: '' },
  }
}

function editableDefaultValue(parameter: ReportParameter): unknown {
  const value = parameter.defaultValue
  if (value === null || value === undefined) return parameter.logicalType === 'multi_enum' ? [] : ''

  switch (parameter.logicalType) {
    case 'integer':
    case 'decimal':
      return String(value)
    case 'json':
      return JSON.stringify(value)
    case 'multi_enum':
      return Array.isArray(value) ? value.map(String) : []
    case 'boolean':
      return typeof value === 'boolean' ? value : ''
    default:
      return String(value)
  }
}
