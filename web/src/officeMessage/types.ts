export type OfficeMessageSourceType = 'EDITED' | 'ORACLE_PROCEDURE' | 'ORACLE_QUERY'
export type OfficeParameterType = 'string' | 'integer' | 'decimal' | 'date'
export type OfficeDateFormat = 'yyyyMMdd' | 'yyyy-MM-dd' | 'yyyy-MM-dd HH:mm:ss'
export type OfficeColumnValueType = 'string' | 'integer' | 'decimal' | 'date' | 'datetime' | 'boolean'

export type OfficeQueryParameter = {
  code: string
  label: string
  valueType: OfficeParameterType
  format?: OfficeDateFormat
  required: boolean
}

export type OfficeColumnMapping = {
  sourceColumn: string
  header: string
  valueType: OfficeColumnValueType
  order: number
  width: number
}

export type OfficeMessage = {
  id: number
  name: string
  sourceType: OfficeMessageSourceType
  content: string
  procedureOwner: string
  packageName: string
  procedureName: string
  procedureOverload: string
  resultTableOwner: string
  resultTableName: string
  selectSql: string
  parameters: OfficeQueryParameter[]
  columnMapping: OfficeColumnMapping[]
  enabled: boolean
  lockVersion: number
  updatedAt: string
}

export type OfficeMessageDraft = Omit<OfficeMessage, 'id' | 'updatedAt'> & { id: number | null }

export type OfficePushTarget = {
  id: number
  name: string
  messageId: number
  channel: 'FEISHU'
  receiveIdType: 'chat_id' | 'open_id' | 'user_id' | 'union_id' | 'email'
  receiveId: string
  enabled: boolean
  lockVersion: number
  updatedAt: string
}

export type OfficePushRunStatus = 'QUEUED' | 'RUNNING' | 'SUCCEEDED' | 'FAILED' | 'UNKNOWN'

export type OfficePushRun = {
  id: number
  runUuid: string
  targetId: number
  messageId: number
  status: OfficePushRunStatus
  attemptCount: number
  rowCount: number
  errorMessage: string
  createdAt: string
  finishedAt: string
}

export type OfficeProcedureSummary = {
  owner: string
  packageName: string
  name: string
  overload: string
  argumentCount: number
}

export type OfficeResultTableSummary = { owner: string; name: string; columnCount: number }
export type OfficeResultColumn = { name: string; position: number; dataType: string; nullable: boolean }
export type OfficeSelectColumn = { name: string; databaseType: string; nullable: boolean }
