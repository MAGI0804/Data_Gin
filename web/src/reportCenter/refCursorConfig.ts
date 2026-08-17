import type { ReportColumn, ReportInputControl, ReportInputField, ReportInputFormat, ReportInputSchema, ReportInputType, ReportResultTableColumn } from './types'

const conditionCodePattern = /^[A-Za-z][A-Za-z0-9_]{0,63}$/
const oracleFieldPattern = /^[A-Za-z][A-Za-z0-9_$#]{0,127}$/
const jsonNumberPattern = /^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$/
const unsafeNumberError = ' 超出 JavaScript 安全数字范围或无法无损表示，请改用 str 类型。'

export const reportInputTypes: ReportInputType[] = ['str', 'number', 'bool', 'list[str]', 'list[number]', 'list[bool]', 'json']
export const reportInputControls: Array<ReportInputControl | ''> = ['', 'TEXT', 'TEXTAREA', 'NUMBER', 'CHECKBOX', 'DATE', 'DATETIME', 'SELECT']
export const reportDateFormats: ReportInputFormat[] = ['YYYYMMDD', 'YYYY-MM-DD']
export const reportDateTimeFormats: ReportInputFormat[] = ['YYYYMMDDHHmmss', 'YYYY-MM-DD HH:mm:ss', 'ISO8601']

export function parseReportInputSchemaText(source: string): ReportInputSchema {
  if (reportJSONContainsUnsafeNumber(source)) throw new Error(`筛选条件 JSON 中的数字${unsafeNumberError}`)
  return parseReportInputSchemaDocument(JSON.parse(source) as unknown)
}

export function parseReportInputSchemaDocument(value: unknown, allowEmpty = false): ReportInputSchema {
  if (!isRecord(value) || (!allowEmpty && Object.keys(value).length === 0)) throw new Error('筛选条件必须是非空 JSON 对象。')
  if (Object.keys(value).length > 128) throw new Error('筛选条件最多配置 128 个字段。')
  if (new TextEncoder().encode(JSON.stringify(value)).byteLength > 64 * 1024) throw new Error('筛选条件 JSON 不能超过 64 KiB。')
  const result: ReportInputSchema = {}
  for (const [code, rawField] of Object.entries(value)) {
    if (!conditionCodePattern.test(code)) throw new Error(`筛选字段 ${code || '（空）'} 的编码不合法。`)
    if (!isRecord(rawField)) throw new Error(`筛选字段 ${code} 必须使用 JSON 对象配置。`)
    const unknownKeys = Object.keys(rawField).filter((key) => !['type', 'displayName', 'control', 'format', 'required', 'multiple', 'example', 'default', 'allowedValues'].includes(key))
    if (unknownKeys.length) throw new Error(`筛选字段 ${code} 含有未知配置：${unknownKeys.join('、')}。`)
    if (rawField.multiple !== undefined && typeof rawField.multiple !== 'boolean') throw new Error(`筛选字段 ${code} 的 multiple 必须是布尔值。`)
    const normalizedType = normalizeReportInputType(rawField.type, rawField.multiple === true)
    if (!normalizedType) throw new Error(`筛选字段 ${code} 的 JSON 类型不受支持。`)
    const displayName = normalizedString(rawField.displayName)
    const control = normalizeReportInputControl(rawField.control, normalizedType.legacyControl)
    const rawFormat = normalizedString(rawField.format)
    const parsedFormat = normalizeReportInputFormat(rawField.format)
    const format = parsedFormat || (control === 'DATE' ? normalizedType.legacyFormat ?? 'YYYYMMDD' : control === 'DATETIME' ? normalizedType.legacyFormat ?? 'YYYY-MM-DD HH:mm:ss' : '')
    if (!displayName || displayName.length > 128) throw new Error(`筛选字段 ${code} 必须填写筛选显示名。`)
    if (control === null) throw new Error(`筛选字段 ${code} 的控件类型不受支持。`)
    if (rawFormat && !parsedFormat) throw new Error(`筛选字段 ${code} 的日期格式不受支持。`)
    if (format && !validFormatForControl(format, control)) throw new Error(`筛选字段 ${code} 的日期格式与查询控件不匹配。`)
    if ((control === 'DATE' || control === 'DATETIME') && normalizedType.type !== 'str') throw new Error(`筛选字段 ${code} 的日期控件必须使用 str 类型。`)
    if (rawField.required !== undefined && typeof rawField.required !== 'boolean') throw new Error(`筛选字段 ${code} 的 required 必须是布尔值。`)
    if (rawField.allowedValues !== undefined && (!Array.isArray(rawField.allowedValues) || rawField.allowedValues.length === 0)) throw new Error(`筛选字段 ${code} 的 allowedValues 必须是非空数组。`)
    const field = compactInputField({
      type: normalizedType.type,
      displayName,
      control,
      required: rawField.required === true,
      ...(format ? { format } : {}),
      ...(Object.prototype.hasOwnProperty.call(rawField, 'example') ? { example: rawField.example } : {}),
      ...(Object.prototype.hasOwnProperty.call(rawField, 'default') ? { default: rawField.default } : {}),
      ...(Array.isArray(rawField.allowedValues) ? { allowedValues: [...rawField.allowedValues] } : {}),
    })
    validateReportInputFieldMetadata(code, field)
    result[code] = field
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
    if (!oracleFieldPattern.test(databaseColumn)) throw new Error(`Oracle 结果字段 ${databaseColumn || '（空）'} 不合法。`)
    if (fields.has(databaseColumn.toUpperCase())) throw new Error(`Oracle 结果字段 ${databaseColumn} 重复。`)
    if (!header || header.length > 255) throw new Error(`Oracle 结果字段 ${databaseColumn} 必须配置 Excel 表头。`)
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

export function reportColumnsFromResultSchema(columns: ReportResultTableColumn[], createFieldId: () => string = () => crypto.randomUUID()): ReportColumn[] {
  return reconcileReportColumnsWithResultSchema(columns, [], createFieldId)
}

export function reconcileReportColumnsWithResultSchema(columns: ReportResultTableColumn[], existingColumns: ReportColumn[], createFieldId: () => string = () => crypto.randomUUID()): ReportColumn[] {
  const existingByName = new Map(existingColumns.map((column) => [column.databaseColumn.toUpperCase(), column]))
  const mapping = Object.fromEntries(columns.map((column) => [column.name, existingByName.get(column.name.toUpperCase())?.excelHeader || column.name]))
  const initial = applyExcelMapping(existingColumns, mapping, createFieldId)
  const schemaByName = new Map(columns.map((column) => [column.name.toUpperCase(), column]))
  return initial.map((column) => {
    const schema = schemaByName.get(column.databaseColumn.toUpperCase())
    if (!schema) return column
    const valueType = reportValueTypeCompatible(column.valueType, schema.oracleType, schema.scale)
      ? column.valueType
      : reportValueTypeFromOracle(schema.oracleType, schema.scale)
    return {
      ...column,
      sourceOracleType: schema.oracleType,
      precision: schema.precision,
      scale: schema.scale,
      nullable: schema.nullable,
      valueType,
    }
  })
}

export function refreshReportColumnMetadata(columns: ReportResultTableColumn[], existingColumns: ReportColumn[]): ReportColumn[] {
  const schemaByName = new Map(columns.map((column) => [column.name.toUpperCase(), column]))
  return existingColumns.filter((column) => schemaByName.has(column.databaseColumn.toUpperCase())).map((column) => {
    const schema = schemaByName.get(column.databaseColumn.toUpperCase())!
    const valueType = reportValueTypeCompatible(column.valueType, schema.oracleType, schema.scale)
      ? column.valueType
      : reportValueTypeFromOracle(schema.oracleType, schema.scale)
    return {
      ...column,
      sourceOracleType: schema.oracleType,
      precision: schema.precision,
      scale: schema.scale,
      nullable: schema.nullable,
      valueType,
    }
  })
}

export function renameExcelMappingField(columns: ReportColumn[], currentField: string, nextField: string): ReportColumn[] {
  return columns.map((column) => column.databaseColumn === currentField ? { ...column, databaseColumn: nextField } : column)
}

export function newReportInputField(index: number): [string, ReportInputField] {
  return [`condition${index + 1}`, {
    type: 'str',
    displayName: `筛选条件 ${index + 1}`,
    control: 'TEXT',
    required: false,
  }]
}

export function isReportInputListType(type: ReportInputType): type is 'list[str]' | 'list[number]' | 'list[bool]' {
  return type.startsWith('list[')
}

export function reportJSONContainsUnsafeNumber(source: string) {
  for (let index = 0; index < source.length;) {
    if (source[index] === '"') {
      index += 1
      while (index < source.length) {
        if (source[index] === '\\') { index += 2; continue }
        if (source[index] === '"') { index += 1; break }
        index += 1
      }
      continue
    }
    if (source[index] === '-' || /\d/.test(source[index])) {
      const matched = /^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/.exec(source.slice(index))
      if (matched) {
        if ('error' in normalizeSafeNumber(matched[0])) return true
        index += matched[0].length
        continue
      }
    }
    index += 1
  }
  return false
}

export function initialReportConditionValues(schema: ReportInputSchema): Record<string, unknown> {
  return Object.fromEntries(Object.entries(schema).map(([code, field]) => {
    if (Object.prototype.hasOwnProperty.call(field, 'default')) return [code, editableReportConditionValue(field.default, field)]
    return [code, isReportInputListType(field.type) ? [] : '']
  }))
}

export function buildReportConditions(schema: ReportInputSchema, values: Record<string, unknown>): { ok: true; conditions: Record<string, unknown> } | { ok: false; error: string } {
  const conditions: Record<string, unknown> = {}
  for (const [code, field] of Object.entries(schema)) {
    const rawValue = values[code]
    if (rawValue === '' || rawValue === undefined || rawValue === null || (Array.isArray(rawValue) && rawValue.length === 0)) {
      if (field.required && !Object.prototype.hasOwnProperty.call(field, 'default')) return { ok: false, error: `${field.displayName} 为必填筛选条件。` }
      conditions[code] = Object.prototype.hasOwnProperty.call(field, 'default') ? field.default : emptyReportConditionValue(field)
      continue
    }
    const normalized = normalizeConditionValue(rawValue, field)
    if ('error' in normalized) return { ok: false, error: `${field.displayName}${normalized.error}` }
    const value = normalized.value
    const items = isReportInputListType(field.type) ? value as unknown[] : [value]
    if (field.allowedValues?.length) {
      const allowed = new Set(field.allowedValues.map(canonicalComparableValue))
      if (items.some((item) => !allowed.has(canonicalComparableValue(item)))) return { ok: false, error: `${field.displayName} 只能选择指定值。` }
    }
    conditions[code] = value
  }
  return { ok: true, conditions }
}

function emptyReportConditionValue(field: ReportInputField): unknown {
  if (isReportInputListType(field.type)) return []
  if (field.type === 'str') return ''
  return null
}

function normalizeReportInputType(value: unknown, legacyMultiple: boolean): { type: ReportInputType; legacyControl: ReportInputControl | ''; legacyFormat?: ReportInputFormat } | null {
  const raw = normalizedString(value)
  const canonical = reportInputTypes.find((item) => item.toLowerCase() === raw.toLowerCase())
  if (canonical) return { type: legacyMultiple && canonical === 'str' ? 'list[str]' : legacyMultiple && canonical === 'number' ? 'list[number]' : legacyMultiple && canonical === 'bool' ? 'list[bool]' : canonical, legacyControl: '' }
  const oracleType = raw.toUpperCase()
  if (['VARCHAR', 'VARCHAR2', 'NVARCHAR2', 'CHAR', 'NCHAR', 'CLOB', 'NCLOB', 'STRING'].includes(oracleType)) return { type: legacyMultiple ? 'list[str]' : 'str', legacyControl: '' }
  if (['NUMBER', 'INTEGER', 'DECIMAL', 'FLOAT', 'BINARY_FLOAT', 'BINARY_DOUBLE'].includes(oracleType)) return { type: legacyMultiple ? 'list[number]' : 'number', legacyControl: '' }
  if (oracleType === 'BOOLEAN') return { type: legacyMultiple ? 'list[bool]' : 'bool', legacyControl: '' }
  if (oracleType === 'DATE') return { type: legacyMultiple ? 'list[str]' : 'str', legacyControl: legacyMultiple ? '' : 'DATE', ...(legacyMultiple ? {} : { legacyFormat: 'YYYYMMDD' as const }) }
  if (oracleType.startsWith('TIMESTAMP')) return { type: legacyMultiple ? 'list[str]' : 'str', legacyControl: legacyMultiple ? '' : 'DATETIME', ...(legacyMultiple ? {} : { legacyFormat: 'ISO8601' as const }) }
  return null
}

function validateReportInputFieldMetadata(code: string, field: ReportInputField) {
  const allowed = field.allowedValues ?? []
  const allowedField = isReportInputListType(field.type) ? { ...field, type: listItemType(field.type), required: false } : field
  for (const value of allowed) {
    if (!metadataValueMatchesField(value, allowedField)) throw new Error(`筛选字段 ${code} 的允许值与 ${field.type} 类型或日期格式不匹配。`)
  }
  for (const [label, value] of [['示例值', field.example], ['默认值', field.default]] as const) {
    if (!Object.prototype.hasOwnProperty.call(field, label === '示例值' ? 'example' : 'default')) continue
    if (!metadataValueMatchesField(value, field)) throw new Error(`筛选字段 ${code} 的${label}与 ${field.type} 类型或日期格式不匹配。`)
    const values = isReportInputListType(field.type) && Array.isArray(value) ? value : [value]
    if (allowed.length && values.some((item) => !allowed.some((candidate) => canonicalComparableValue(candidate) === canonicalComparableValue(item)))) {
      throw new Error(`筛选字段 ${code} 的${label}不在允许值中。`)
    }
  }
}

function metadataValueMatchesField(value: unknown, field: ReportInputField): boolean {
  if (field.type === 'str') {
    if (typeof value !== 'string' || !value.length) return false
    if (field.control !== 'DATE' && field.control !== 'DATETIME') return true
    const editable = field.control === 'DATE' ? editableDateValue(value, field.format) : editableDateTimeValue(value, field.format)
    const normalized = normalizeConditionValue(editable, field)
    return 'value' in normalized && normalized.value === value
  }
  if (field.type === 'number') return typeof value === 'number' && !('error' in normalizeSafeNumber(value))
  if (field.type === 'bool') return typeof value === 'boolean'
  if (field.type === 'json') return value !== undefined && value !== null && isJSONValue(value)
  if (!Array.isArray(value) || (field.required && value.length === 0)) return false
  const itemField = { ...field, type: listItemType(field.type), required: false }
  return value.every((item) => metadataValueMatchesField(item, itemField))
}

function listItemType(type: 'list[str]' | 'list[number]' | 'list[bool]'): 'str' | 'number' | 'bool' {
  if (type === 'list[number]') return 'number'
  if (type === 'list[bool]') return 'bool'
  return 'str'
}

function normalizeReportInputControl(value: unknown, fallback: ReportInputControl | ''): ReportInputControl | '' | null {
  const raw = normalizedString(value).toUpperCase()
  if (!raw) return fallback
  if (raw === 'MULTI_SELECT') return 'SELECT'
  return reportInputControls.includes(raw as ReportInputControl) ? raw as ReportInputControl : null
}

function normalizeReportInputFormat(value: unknown): ReportInputFormat | '' {
  const raw = normalizedString(value)
  return [...reportDateFormats, ...reportDateTimeFormats].find((item) => item.toUpperCase() === raw.toUpperCase()) ?? ''
}

function validFormatForControl(format: ReportInputFormat, control: ReportInputControl | '') {
  if (control === 'DATE') return reportDateFormats.includes(format)
  if (control === 'DATETIME') return reportDateTimeFormats.includes(format)
  return false
}

function normalizeConditionValue(rawValue: unknown, field: ReportInputField): { ok: true; value: unknown } | { ok: false; error: string } {
  if (field.type === 'str') {
    if (typeof rawValue !== 'string' || !rawValue.length) return { ok: false, error: ' 必须填写字符串。' }
    if (field.control === 'DATE') return formatDateCondition(rawValue, field.format)
    if (field.control === 'DATETIME') return formatDateTimeCondition(rawValue, field.format)
    return { ok: true, value: rawValue }
  }
  if (field.type === 'number') {
    return normalizeSafeNumber(rawValue)
  }
  if (field.type === 'bool') return typeof rawValue === 'boolean' ? { ok: true, value: rawValue } : { ok: false, error: ' 必须填写布尔值。' }
  if (isReportInputListType(field.type)) {
    let value = rawValue
    if (typeof rawValue === 'string') {
      try { value = JSON.parse(rawValue) as unknown } catch { return { ok: false, error: ' 必须填写 JSON 数组。' } }
    }
    if (!Array.isArray(value)) return { ok: false, error: ' 必须填写 JSON 数组。' }
    if (field.type === 'list[number]') {
      const rawNumberItems = typeof rawValue === 'string' ? jsonNumberArrayItems(rawValue) : value
      if (!rawNumberItems || value.some((item) => typeof item !== 'number')) return { ok: false, error: ` 与 ${field.type} 类型不匹配。` }
      const normalizedItems: number[] = []
      for (const item of rawNumberItems) {
        const normalized = normalizeSafeNumber(item)
        if ('error' in normalized) return { ok: false, error: unsafeNumberError }
        normalizedItems.push(normalized.value)
      }
      return { ok: true, value: normalizedItems }
    }
    const itemType = field.type.slice(5, -1)
    const valid = value.every((item) => itemType === 'str' ? typeof item === 'string' : typeof item === 'boolean')
    if (!valid) return { ok: false, error: ` 与 ${field.type} 类型不匹配。` }
    return { ok: true, value }
  }
  let value = rawValue
  if (typeof rawValue === 'string') {
    try { value = JSON.parse(rawValue) as unknown } catch { return { ok: false, error: ' 必须填写合法 JSON。' } }
  }
  return isJSONValue(value) ? { ok: true, value } : { ok: false, error: ' 必须填写合法 JSON。' }
}

function normalizeSafeNumber(rawValue: unknown): { ok: true; value: number } | { ok: false; error: string } {
  if (typeof rawValue === 'number') {
    if (!Number.isFinite(rawValue)) return { ok: false, error: ' 必须填写有效数字。' }
    return Number.isInteger(rawValue) && !Number.isSafeInteger(rawValue) ? { ok: false, error: unsafeNumberError } : { ok: true, value: rawValue }
  }
  const text = typeof rawValue === 'string' ? rawValue.trim() : ''
  if (!jsonNumberPattern.test(text)) return { ok: false, error: ' 必须填写有效数字。' }
  const value = Number(text)
  if (!Number.isFinite(value) || (Number.isInteger(value) && !Number.isSafeInteger(value)) || !sameDecimalValue(text, String(value))) return { ok: false, error: unsafeNumberError }
  return { ok: true, value }
}

function jsonNumberArrayItems(value: string): string[] | null {
  const text = value.trim()
  if (!text.startsWith('[') || !text.endsWith(']')) return null
  const body = text.slice(1, -1).trim()
  if (!body) return []
  const items = body.split(',').map((item) => item.trim())
  return items.every((item) => jsonNumberPattern.test(item)) ? items : null
}

function sameDecimalValue(left: string, right: string) {
  const leftValue = decimalSignature(left)
  const rightValue = decimalSignature(right)
  return Boolean(leftValue && rightValue && leftValue.negative === rightValue.negative && leftValue.digits === rightValue.digits && leftValue.exponent === rightValue.exponent)
}

function decimalSignature(value: string): { negative: boolean; digits: string; exponent: number } | null {
  const matched = /^(-?)(\d+)(?:\.(\d+))?(?:[eE]([+-]?\d+))?$/.exec(value)
  if (!matched) return null
  const fraction = matched[3] ?? ''
  const digits = `${matched[2]}${fraction}`.replace(/^0+/, '')
  if (!digits) return { negative: false, digits: '0', exponent: 0 }
  const normalizedDigits = digits.replace(/0+$/, '')
  return {
    negative: matched[1] === '-',
    digits: normalizedDigits,
    exponent: Number(matched[4] ?? '0') - fraction.length + digits.length - normalizedDigits.length,
  }
}

function formatDateCondition(value: string, format: ReportInputFormat | undefined): { ok: true; value: string } | { ok: false; error: string } {
  const parts = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!parts || !validDateParts(Number(parts[1]), Number(parts[2]), Number(parts[3]))) return { ok: false, error: ' 必须填写有效日期。' }
  return { ok: true, value: format === 'YYYYMMDD' ? `${parts[1]}${parts[2]}${parts[3]}` : `${parts[1]}-${parts[2]}-${parts[3]}` }
}

function formatDateTimeCondition(value: string, format: ReportInputFormat | undefined): { ok: true; value: string } | { ok: false; error: string } {
  const parts = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(value)
  if (!parts || !validDateParts(Number(parts[1]), Number(parts[2]), Number(parts[3])) || Number(parts[4]) > 23 || Number(parts[5]) > 59 || Number(parts[6] ?? '0') > 59) return { ok: false, error: ' 必须填写有效日期时间。' }
  const second = parts[6] ?? '00'
  if (format === 'YYYYMMDDHHmmss') return { ok: true, value: `${parts[1]}${parts[2]}${parts[3]}${parts[4]}${parts[5]}${second}` }
  if (format === 'YYYY-MM-DD HH:mm:ss') return { ok: true, value: `${parts[1]}-${parts[2]}-${parts[3]} ${parts[4]}:${parts[5]}:${second}` }
  return { ok: true, value: `${parts[1]}-${parts[2]}-${parts[3]}T${parts[4]}:${parts[5]}:${second}` }
}

function validDateParts(year: number, month: number, day: number) {
  if (month < 1 || month > 12 || day < 1) return false
  return day <= new Date(Date.UTC(year, month, 0)).getUTCDate()
}

function isJSONValue(value: unknown): boolean {
  if (value === null || typeof value === 'string' || typeof value === 'boolean') return true
  if (typeof value === 'number') return Number.isFinite(value)
  if (Array.isArray(value)) return value.every(isJSONValue)
  return isRecord(value) && Object.values(value).every(isJSONValue)
}

function compactInputField(field: ReportInputField): ReportInputField {
  return {
    type: field.type,
    displayName: field.displayName,
    control: field.control,
    required: field.required,
    ...(field.format ? { format: field.format } : {}),
    ...(Object.prototype.hasOwnProperty.call(field, 'example') ? { example: field.example } : {}),
    ...(Object.prototype.hasOwnProperty.call(field, 'default') ? { default: field.default } : {}),
    ...(field.allowedValues ? { allowedValues: [...field.allowedValues] } : {}),
  }
}

export function editableReportConditionValue(value: unknown, field: ReportInputField) {
  if (isReportInputListType(field.type)) return Array.isArray(value) ? [...value] : []
  if (field.type === 'bool') return typeof value === 'boolean' ? value : ''
  if (typeof value === 'string' && field.control === 'DATE') return editableDateValue(value, field.format)
  if (typeof value === 'string' && field.control === 'DATETIME') return editableDateTimeValue(value, field.format)
  if (field.allowedValues?.length) return value
  return value === undefined || value === null ? '' : String(value)
}

function editableDateValue(value: string, format: ReportInputFormat | undefined) {
  if (format === 'YYYYMMDD') {
    const parts = /^(\d{4})(\d{2})(\d{2})$/.exec(value)
    return parts ? `${parts[1]}-${parts[2]}-${parts[3]}` : value
  }
  return value
}

function editableDateTimeValue(value: string, format: ReportInputFormat | undefined) {
  if (format === 'YYYYMMDDHHmmss') {
    const parts = /^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})$/.exec(value)
    return parts ? `${parts[1]}-${parts[2]}-${parts[3]}T${parts[4]}:${parts[5]}:${parts[6]}` : value
  }
  if (format === 'YYYY-MM-DD HH:mm:ss') return value.replace(' ', 'T')
  return value
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

function reportValueTypeFromOracle(oracleType: string, scale: number | null = null) {
  const normalized = oracleType.trim().toUpperCase()
  if (normalized === 'NUMBER') return scale === 0 ? 'integer' : 'decimal'
  if (normalized === 'BINARY_FLOAT' || normalized === 'BINARY_DOUBLE') return 'decimal'
  if (normalized === 'DATE') return 'date'
  if (normalized.startsWith('TIMESTAMP')) return 'datetime'
  if (normalized === 'BOOLEAN') return 'boolean'
  return 'string'
}

function reportValueTypeCompatible(valueType: string, oracleType: string, scale: number | null) {
  const logical = valueType.trim().toLowerCase()
  const oracle = oracleType.trim().toUpperCase()
  if (logical === 'string' || logical === 'enum') return ['CHAR', 'NCHAR', 'VARCHAR2', 'NVARCHAR2', 'CLOB', 'NCLOB'].includes(oracle)
  if (logical === 'integer') return oracle === 'NUMBER' && scale === 0
  if (logical === 'decimal') return ['NUMBER', 'BINARY_FLOAT', 'BINARY_DOUBLE'].includes(oracle)
  if (logical === 'boolean') return oracle === 'BOOLEAN' || oracle === 'NUMBER' || ['CHAR', 'NCHAR', 'VARCHAR2', 'NVARCHAR2'].includes(oracle)
  if (logical === 'date') return oracle === 'DATE'
  if (logical === 'datetime') return oracle === 'DATE' || oracle.startsWith('TIMESTAMP')
  if (logical === 'multi_enum' || logical === 'json') return oracle === 'CLOB' || oracle === 'NCLOB'
  return false
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
