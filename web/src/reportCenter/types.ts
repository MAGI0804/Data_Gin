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
  isOwner: boolean
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

export type ReportDatasourceTestStatus = 'SUCCESS' | 'FAILED' | 'NOT_TESTED'

export type ReportDatasource = {
  id: number
  code: string
  name: string
  driver: 'ORACLE'
  host: string
  port: number
  serviceName: string
  sid: string
  username: string
  hasPassword: boolean
  sessionTimezone: string
  connectTimeoutSeconds: number
  queryTimeoutSeconds: number
  maxOpenConnections: number
  maxIdleConnections: number
  prefetchRows: number
  arraySize: number
  enabled: boolean
  lastTestStatus: ReportDatasourceTestStatus
  lastTestError: string
  lastTestedAt: string | null
}

export type ReportDatasourceInput = Omit<ReportDatasource,
  'id' | 'driver' | 'hasPassword' | 'lastTestStatus' | 'lastTestError' | 'lastTestedAt'
> & {
  password: string
}

export type ReportDatasourceTest = {
  status: Exclude<ReportDatasourceTestStatus, 'NOT_TESTED'>
  testedAt: string
  latencyMs: number
  errorCode: string
  message: string
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
  normalizer: Record<string, unknown>
  valueSource: Record<string, unknown>
  timezone: string
  nullPolicy: string
  errorMessage: string
  collectionEncoding?: string
}

export type ReportColumn = {
  fieldId: string
  logicalCode: string
  databaseColumn: string
  sourceOracleType: string
  precision: number | null
  scale: number | null
  nullable: boolean
  valueType: string
  previewHeader: string
  excelHeader: string
  displayOrder: number
  exportOrder: number
  previewVisible: boolean
  exportVisible: boolean
  filterable: boolean
  sortable: boolean
  exportAllowed: boolean
  allowedOperators: unknown
  format: unknown
  dictionaryVersion: unknown
  maskingPolicy: unknown
  excelWidth: number
  nullDisplay: string
}

export type ReportGrant = { subjectType: 'USER' | 'ROLE'; subjectId: number; actions: string[] }

export type ReportDraft = {
  id: number
  code: string
  name: string
  category: string
  description: string
  datasourceId: number
  status: ReportDefinitionStatus
  lockVersion: number
  procedure: { owner: string; package: string; name: string; overload: string }
  result: { tableOwner: string; tableName: string; runIdColumn: string; rowIdColumn: string }
  callTemplate: string
  parameters: ReportParameter[]
  columns: ReportColumn[]
  grants: ReportGrant[]
  createdAt: string | null
  updatedAt: string | null
}

export type ReportValidationSummary = {
  validatedAt: string
  procedure: {
    owner: string
    package: string
    name: string
    overload: string
    argumentCount: number
    signatureHash: string
  }
  result: {
    tableOwner: string
    tableName: string
    columnCount: number
    schemaHash: string
  }
  snapshot: {
    runIdColumn: string
    rowIdColumn: string
    uniqueKeyValidated: boolean
  }
  export: {
    exportableColumnCount: number
    schemaHash: string
  }
}

export type ReportPublication = {
  definitionId: number
  versionId: number
  version: number
  status: string
  contractHash: string
  publishedAt: string | null
  validation: ReportValidationSummary | null
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
	filterable: boolean
	sortable: boolean
	allowedOperators: ReportFilterOperator[]
}

export type ReportFilterOperator = 'EQ' | 'NE' | 'GT' | 'GTE' | 'LT' | 'LTE' | 'IN' | 'NOT_IN' | 'IS_NULL' | 'IS_NOT_NULL' | 'CONTAINS' | 'STARTS_WITH' | 'BETWEEN'
export type ReportResultFilter = { field: string; operator: ReportFilterOperator; value?: unknown }
export type ReportResultSort = { field: string; direction: 'ASC' | 'DESC' }
export type ReportResultQuery = { filters: ReportResultFilter[]; sort: ReportResultSort[] }

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
	reportName: string
	purgedRows: number
	purgeStartedAt: string | null
}

export type ReportExportPage = { items: ReportExport[]; hasMore: boolean; nextAfterId: number }

export type ReportAudit = {
  id: number
  actorType: 'USER' | 'SYSTEM'
  actorUserId: number
  action: string
  targetType: string
  targetId: number
  requestId: string
  detail: Record<string, unknown>
  createdAt: string
}

export type ReportAuditQuery = {
  action?: string
  targetType?: string
  targetId?: number
  afterId?: number
  limit?: number
}

export type ReportAuditPage = {
  items: ReportAudit[]
  hasMore: boolean
  nextAfterId: number
}
