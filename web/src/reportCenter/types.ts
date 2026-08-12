export type ReportDefinitionStatus = 'DRAFT' | 'ACTIVE' | 'DISABLED'

export type ReportSummary = {
  id: number
  code: string
  name: string
  category: string
  description: string
  datasourceId: number
  status: ReportDefinitionStatus
  currentDraftVersionId: number
  currentPublishedVersionId: number
  lockVersion: number
  updatedAt: string | null
}

export type ReportCatalogPage = {
  items: ReportSummary[]
  hasMore: boolean
  nextAfterId: number
}

export type ReportCatalogQuery = {
  search?: string
  category?: string
  afterId?: number
  limit?: number
}

export type ReportCenterSection = 'catalog' | 'configuration' | 'query' | 'exports'

export type ReportParameter = {
  code: string
  label: string
  displayOrder: number
  controlType: 'TEXT' | 'TEXTAREA' | 'NUMBER' | 'CHECKBOX' | 'DATE' | 'DATETIME' | 'SELECT' | 'MULTI_SELECT'
  logicalType: 'string' | 'integer' | 'decimal' | 'boolean' | 'date' | 'datetime' | 'enum' | 'multi_enum' | 'json'
  cardinality: 'SINGLE' | 'MULTIPLE'
  procedureArgName: string
  position: number
  oracleType: string
  precision: number | null
  scale: number | null
  maxLength: number | null
  required: boolean
  nullable: boolean
  systemInjected: boolean
  sensitive: boolean
  defaultValue: unknown
  allowedValues: string[]
  validation: Record<string, unknown>
  timezone: string
  errorMessage: string
}

export type ReportRunContract = {
  definitionId: number
  versionId: number
  code: string
  name: string
  description: string
  parameters: ReportParameter[]
}

export type ReportRunStatus = 'QUEUED' | 'RUNNING' | 'CANCEL_REQUESTED' | 'SUCCEEDED' | 'FAILED' | 'CANCELLED' | 'UNKNOWN' | 'RECONCILING' | 'EXPORTING' | 'EXPORTED' | 'RESULT_PURGING' | 'RESULT_PURGED'

export type ReportRun = {
  id: number
  runUuid: string
  definitionId: number
  versionId: number
  status: ReportRunStatus
  rowCount: number
  cancelRequested: boolean
  createdAt: string | null
  startedAt: string | null
  finishedAt: string | null
  resultExpiresAt: string | null
  errorCode: string
  errorMessage: string
  canCancel: boolean
  resultAvailable: boolean
}

export type ReportResultColumn = {
  fieldId: string
  code: string
  header: string
  valueType: string
  nullable: boolean
  nullDisplay: string
}

export type ReportResultRow = { key: string; values: Record<string, unknown> }

export type ReportResultPage = {
  run: ReportRun
  columns: ReportResultColumn[]
  rows: ReportResultRow[]
  pagination: { pageSize: number; hasMore: boolean; nextCursor: string }
}

export type ReportExportStatus = 'PENDING' | 'RUNNING' | 'READY' | 'FAILED' | 'CANCELLED' | 'EXPIRED'

export type ReportExport = {
  id: number
  exportUuid: string
  runId: number
  status: ReportExportStatus
  processedRows: number
  exportedRows: number
  currentSheet: string
  sheetCount: number
  truncatedCellCount: number
  fileSizeBytes: number
  createdAt: string | null
  startedAt: string | null
  readyAt: string | null
  expiresAt: string | null
  purgedAt: string | null
  errorCode: string
  errorMessage: string
  canDownload: boolean
}
