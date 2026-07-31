export type MallImportItem = { mallCode: string; nameCn: string; province: string; city: string; district?: string; address: string }
export type MallImportRow = { row: number; item?: MallImportItem; error?: string }

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
    return { row: index + 2, item: { mallCode: normalizedCode, nameCn, province, city, ...(district ? { district } : {}), address } }
  })
  if (!result.length || result.length > 200) throw new Error('CSV 必须包含 1 至 200 条数据')
  return result
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
