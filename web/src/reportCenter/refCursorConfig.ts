import type { ReportColumn, ReportInputControl, ReportInputField, ReportInputSchema } from './types'

const conditionCodePattern = /^[A-Za-z][A-Za-z0-9_]{0,63}$/
const oracleFieldPattern = /^[A-Za-z][A-Za-z0-9_$#]{0,127}$/

export const reportInputTypes = ['VARCHAR2', 'NVARCHAR2', 'CHAR', 'NCHAR', 'CLOB', 'NCLOB', 'NUMBER', 'DATE', 'TIMESTAMP', 'BOOLEAN'] as const
export const reportInputControls: Array<ReportInputControl | ''> = ['', 'TEXT', 'TEXTAREA', 'NUMBER', 'CHECKBOX', 'DATE', 'DATETIME', 'SELECT', 'MULTI_SELECT']

export function parseReportInputSchemaDocument(value: unknown, allowEmpty = false): ReportInputSchema {
  if (!isRecord(value) || (!allowEmpty && Object.keys(value).length === 0)) throw new Error('筛选条件必须是非空 JSON 对象。')
  if (Object.keys(value).length > 128) throw new Error('筛选条件最多配置 128 个字段。')
  const result: ReportInputSchema = {}
  for (const [code, rawField] of Object.entries(value)) {
    if (!conditionCodePattern.test(code)) throw new Error(`筛选字段 ${code || '（空）'} 的编码不合法。`)
    if (!isRecord(rawField)) throw new Error(`筛选字段 ${code} 必须使用 JSON 对象配置。`)
    const unknownKeys = Object.keys(rawField).filter((key) => !['type', 'displayName', 'control', 'required', 'multiple', 'example', 'default', 'allowedValues'].includes(key))
    if (unknownKeys.length) throw new Error(`筛选字段 ${code} 含有未知配置：${unknownKeys.join('、')}。`)
    const type = normalizedString(rawField.type).toUpperCase()
    const displayName = normalizedString(rawField.displayName)
    const control = normalizedString(rawField.control).toUpperCase()
    if (!(reportInputTypes as readonly string[]).includes(type)) throw new Error(`筛选字段 ${code} 的 Oracle 类型不受支持。`)
    if (!displayName || displayName.length > 128) throw new Error(`筛选字段 ${code} 必须填写筛选显示名。`)
    if (!reportInputControls.includes(control as ReportInputControl | '')) throw new Error(`筛选字段 ${code} 的控件类型不受支持。`)
    if (rawField.required !== undefined && typeof rawField.required !== 'boolean') throw new Error(`筛选字段 ${code} 的 required 必须是布尔值。`)
    if (rawField.multiple !== undefined && typeof rawField.multiple !== 'boolean') throw new Error(`筛选字段 ${code} 的 multiple 必须是布尔值。`)
    if (rawField.allowedValues !== undefined && (!Array.isArray(rawField.allowedValues) || rawField.allowedValues.length === 0)) throw new Error(`筛选字段 ${code} 的 allowedValues 必须是非空数组。`)
    result[code] = compactInputField({
      type,
      displayName,
      control: control as ReportInputControl | '',
      required: rawField.required === true,
      multiple: rawField.multiple === true,
      ...(Object.prototype.hasOwnProperty.call(rawField, 'example') ? { example: rawField.example } : {}),
      ...(Object.prototype.hasOwnProperty.call(rawField, 'default') ? { default: rawField.default } : {}),
      ...(Array.isArray(rawField.allowedValues) ? { allowedValues: [...rawField.allowedValues] } : {}),
    })
  }
  return result
}

export function reportInputSchemaDocument(schema: ReportInputSchema) {
  return Object.fromEntries(Object.entries(schema).map(([code, field]) => [code, compactInputField(field)]))
}

export function renameReportInputField(schema: ReportInputSchema, currentCode: string, nextCode: string): ReportInputSchema {
  const code = nextCode.trim()
  if (code !== currentCode && Object.keys(schema).some((item) => item.toUpperCase() === code.toUpperCase())) return schema
  return Object.fromEntries(Object.entries(schema).map(([itemCode, field]) => [itemCode === currentCode ? code : itemCode, field]))
}

export function parseExcelMappingDocument(value: unknown): Record<string, string> {
  if (!isRecord(value)) throw new Error('Excel 映射必须是 JSON 对象。')
  if (Object.keys(value).length > 512) throw new Error('Excel 最多映射 512 个字段。')
  const headers = new Set<string>()
  const fields = new Set<string>()
  const result: Record<string, string> = {}
  for (const [databaseColumn, rawHeader] of Object.entries(value)) {
    const header = normalizedString(rawHeader)
    if (!oracleFieldPattern.test(databaseColumn)) throw new Error(`Oracle 游标字段 ${databaseColumn || '（空）'} 不合法。`)
    if (fields.has(databaseColumn.toUpperCase())) throw new Error(`Oracle 游标字段 ${databaseColumn} 重复。`)
    if (!header || header.length > 255) throw new Error(`Oracle 游标字段 ${databaseColumn} 必须配置 Excel 表头。`)
    if (headers.has(header)) throw new Error(`Excel 表头 ${header} 重复。`)
    headers.add(header)
    fields.add(databaseColumn.toUpperCase())
    result[databaseColumn] = header
  }
  return result
}

export function excelMappingFromColumns(columns: ReportColumn[]): Record<string, string> {
  return Object.fromEntries([...columns]
    .filter((column) => column.exportVisible && column.exportAllowed)
    .sort((left, right) => left.exportOrder - right.exportOrder)
    .map((column) => [column.databaseColumn, column.excelHeader]))
}

export function applyExcelMapping(columns: ReportColumn[], mapping: Record<string, string>, createFieldId: () => string = () => crypto.randomUUID()): ReportColumn[] {
  const existing = new Map(columns.map((column) => [column.databaseColumn.toUpperCase(), column]))
  const usedLogicalCodes = new Set<string>()
  return Object.entries(mapping).map(([databaseColumn, excelHeader], index) => {
    const previous = existing.get(databaseColumn.toUpperCase())
    const logicalCode = uniqueLogicalCode(previous?.logicalCode || logicalCodeFromOracleField(databaseColumn, index), usedLogicalCodes)
    usedLogicalCodes.add(logicalCode.toUpperCase())
    return {
      ...(previous ?? newReportColumn(index, createFieldId())),
      logicalCode,
      databaseColumn,
      previewHeader: excelHeader,
      excelHeader,
      displayOrder: index,
      exportOrder: index,
      previewVisible: previous?.previewVisible ?? true,
      exportVisible: true,
      exportAllowed: true,
    }
  })
}

export function renameExcelMappingField(columns: ReportColumn[], currentField: string, nextField: string): ReportColumn[] {
  return columns.map((column) => column.databaseColumn === currentField ? { ...column, databaseColumn: nextField } : column)
}

export function newReportInputField(index: number): [string, ReportInputField] {
  return [`condition${index + 1}`, {
    type: 'VARCHAR2',
    displayName: `筛选条件 ${index + 1}`,
    control: 'TEXT',
    required: false,
    multiple: false,
  }]
}

export function initialReportConditionValues(schema: ReportInputSchema): Record<string, unknown> {
  return Object.fromEntries(Object.entries(schema).map(([code, field]) => {
    if (Object.prototype.hasOwnProperty.call(field, 'default')) return [code, editableConditionValue(field.default, field)]
    return [code, field.multiple ? [] : '']
  }))
}

export function buildReportConditions(schema: ReportInputSchema, values: Record<string, unknown>): { ok: true; conditions: Record<string, unknown> } | { ok: false; error: string } {
  const conditions: Record<string, unknown> = {}
  for (const [code, field] of Object.entries(schema)) {
    const rawValue = values[code]
    if (rawValue === '' || rawValue === undefined || rawValue === null || (Array.isArray(rawValue) && rawValue.length === 0)) {
      if (field.required && !Object.prototype.hasOwnProperty.call(field, 'default')) return { ok: false, error: `${field.displayName} 为必填筛选条件。` }
      continue
    }
    let value: unknown = rawValue
    if (field.multiple && typeof rawValue === 'string') {
      try { value = JSON.parse(rawValue) as unknown } catch { return { ok: false, error: `${field.displayName} 必须填写 JSON 数组。` } }
    }
    if (field.multiple && !Array.isArray(value)) return { ok: false, error: `${field.displayName} 必须填写 JSON 数组。` }
    if (!field.multiple && Array.isArray(value)) return { ok: false, error: `${field.displayName} 只能填写单值。` }
    const items = field.multiple ? value as unknown[] : [value]
    if (items.some((item) => !conditionValueMatchesType(item, field.type))) return { ok: false, error: `${field.displayName} 与 ${field.type} 类型不匹配。` }
    if (field.allowedValues?.length) {
      const allowed = new Set(field.allowedValues.map(canonicalComparableValue))
      if (items.some((item) => !allowed.has(canonicalComparableValue(item)))) return { ok: false, error: `${field.displayName} 只能选择指定值。` }
    }
    conditions[code] = value
  }
  return { ok: true, conditions }
}

function compactInputField(field: ReportInputField): ReportInputField {
  return {
    type: field.type,
    displayName: field.displayName,
    control: field.control,
    required: field.required,
    multiple: field.multiple,
    ...(Object.prototype.hasOwnProperty.call(field, 'example') ? { example: field.example } : {}),
    ...(Object.prototype.hasOwnProperty.call(field, 'default') ? { default: field.default } : {}),
    ...(field.allowedValues ? { allowedValues: [...field.allowedValues] } : {}),
  }
}

function editableConditionValue(value: unknown, field: ReportInputField) {
  if (field.multiple) return Array.isArray(value) ? [...value] : []
  if (field.allowedValues?.length) return value
  if (field.type === 'BOOLEAN') return typeof value === 'boolean' ? value : ''
  return value === undefined || value === null ? '' : String(value)
}

function conditionValueMatchesType(value: unknown, oracleType: string) {
  if (oracleType === 'BOOLEAN') return typeof value === 'boolean'
  if (oracleType === 'NUMBER') return (typeof value === 'string' && /^-?\d+(?:\.\d+)?$/.test(value)) || (typeof value === 'number' && Number.isFinite(value))
  return typeof value === 'string' && value.length > 0
}

function canonicalComparableValue(value: unknown) {
  return JSON.stringify(value)
}

function newReportColumn(index: number, fieldId: string): ReportColumn {
  return {
    fieldId,
    logicalCode: `field${index + 1}`,
    databaseColumn: `FIELD_${index + 1}`,
    sourceOracleType: 'VARCHAR2',
    precision: null,
    scale: null,
    nullable: true,
    valueType: 'string',
    previewHeader: '',
    excelHeader: '',
    displayOrder: index,
    exportOrder: index,
    previewVisible: true,
    exportVisible: true,
    filterable: false,
    sortable: false,
    exportAllowed: true,
    allowedOperators: undefined,
    format: undefined,
    dictionaryVersion: undefined,
    maskingPolicy: undefined,
    excelWidth: 16,
    nullDisplay: '-',
  }
}

function logicalCodeFromOracleField(databaseColumn: string, index: number) {
  const normalized = databaseColumn.toLowerCase().replace(/[^a-z0-9_]/g, '_')
  return /^[a-z]/.test(normalized) ? normalized.slice(0, 64) : `field${index + 1}`
}

function uniqueLogicalCode(preferred: string, used: Set<string>) {
  let result = preferred
  let suffix = 2
  while (used.has(result.toUpperCase())) {
    const ending = String(suffix)
    result = `${preferred.slice(0, 64 - ending.length)}${ending}`
    suffix += 1
  }
  return result
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function normalizedString(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}
