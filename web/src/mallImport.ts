export type MallImportItem = { mallCode: string; nameCn: string; province: string; city: string; district?: string; address: string }
export type MallImportRow = { row: number; item?: MallImportItem; error?: string }
export type MallImportResultRow = {
  row: number
  status: 'CREATED' | 'REPLAYED' | 'FAILED'
  reviewStatus?: string
  mallCode?: string
  errorCode?: 'INVALID_INPUT' | 'CONFLICT' | 'UNAVAILABLE'
}
export type MallImportResult = {
  rows: MallImportResultRow[]
  created: number
  replayed: number
  failed: number
}

const headers = ['mallCode', 'nameCn', 'province', 'city', 'district', 'address']

export function parseMallImportCSV(text: string): MallImportRow[] {
  if (new TextEncoder().encode(text).byteLength > 1024 * 1024) throw new Error('导入文件不能超过 1 MiB')
  const rows = csvRows(text.replace(/^\uFEFF/, ''))
  if (rows.length < 2 || headers.some((header, index) => rows[0][index] !== header) || rows[0].length !== headers.length) throw new Error('CSV 表头必须为 mallCode,nameCn,province,city,district,address')
  const result = rows.slice(1).filter((row) => row.some((value) => value.trim())).map((row, index) => {
    const values = headers.map((_, column) => (row[column] ?? '').trim())
    const [mallCode, nameCn, province, city, district, address] = values
    if (row.length !== headers.length || !mallCode || !nameCn || !province || !city || !address) return { row: index + 2, error: '必填字段缺失或列数不正确' }
    const normalizedCode = mallCode.toUpperCase()
    if (!/^[A-Z0-9][A-Z0-9_-]{1,63}$/.test(normalizedCode)) return { row: index + 2, error: '商场编码格式无效' }
    if (nameCn.length > 255 || province.length > 128 || city.length > 128 || district.length > 128 || address.length > 1000) return { row: index + 2, error: '字段长度超过限制' }
    return { row: index + 2, item: { mallCode: normalizedCode, nameCn, province, city, ...(district ? { district } : {}), address } }
  })
  if (!result.length || result.length > 200) throw new Error('CSV 必须包含 1 至 200 条数据')
  return result
}

export function mallImportRequestWithinLimit(items: MallImportItem[]) {
  return new TextEncoder().encode(JSON.stringify({ items })).byteLength <= 1024 * 1024
}

export function parseMallImportResult(payload: unknown, expectedRows: number): MallImportResult | null {
  const data = envelopeData(payload)
  if (!data || !Array.isArray(data.rows) || !nonNegativeSafeInteger(data.created) || !nonNegativeSafeInteger(data.replayed) || !nonNegativeSafeInteger(data.failed) ||
    !Number.isSafeInteger(expectedRows) || expectedRows < 1 || data.rows.length !== expectedRows) return null

  const rows: MallImportResultRow[] = []
  let created = 0
  let replayed = 0
  let failed = 0
  for (let index = 0; index < data.rows.length; index++) {
    const value = data.rows[index]
    if (!isRecord(value) || value.row !== index + 1 || typeof value.status !== 'string') return null
    if (value.status === 'CREATED' || value.status === 'REPLAYED') {
      if (typeof value.reviewStatus !== 'string' || !value.reviewStatus.trim() || !isRecord(value.mall) || typeof value.mall.mallCode !== 'string' || !value.mall.mallCode.trim()) return null
      rows.push({ row: value.row, status: value.status, reviewStatus: value.reviewStatus, mallCode: value.mall.mallCode })
      if (value.status === 'CREATED') created++
      else replayed++
      continue
    }
    if (value.status !== 'FAILED' || !isErrorCode(value.errorCode)) return null
    rows.push({ row: value.row, status: 'FAILED', errorCode: value.errorCode })
    failed++
  }
  if (created !== data.created || replayed !== data.replayed || failed !== data.failed) return null
  return { rows, created, replayed, failed }
}

function csvRows(text: string) {
  const rows: string[][] = [[]]; let value = ''; let quoted = false
  for (let index = 0; index < text.length; index++) {
    const char = text[index]
    if (char === '"') { if (quoted && text[index + 1] === '"') { value += '"'; index++ } else quoted = !quoted; continue }
    if (!quoted && char === ',') { rows[rows.length - 1].push(value); value = ''; continue }
    if (!quoted && (char === '\n' || char === '\r')) { if (char === '\r' && text[index + 1] === '\n') index++; rows[rows.length - 1].push(value); rows.push([]); value = ''; continue }
    value += char
  }
  if (quoted) throw new Error('CSV 引号未闭合')
  rows[rows.length - 1].push(value)
  return rows
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function envelopeData(payload: unknown): Record<string, unknown> | null {
  if (!isRecord(payload) || (payload.code !== 0 && payload.code !== 200) || !isRecord(payload.data)) return null
  return payload.data
}

function nonNegativeSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

function isErrorCode(value: unknown): value is MallImportResultRow['errorCode'] {
  return value === 'INVALID_INPUT' || value === 'CONFLICT' || value === 'UNAVAILABLE'
}
