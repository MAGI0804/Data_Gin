import { Drawer, StatusTag } from '../../../ui'
import type { ReportColumn, ReportResultColumn } from '../../types'
import styles from './ReportFieldDetailDrawer.module.css'

export function ReportFieldDetailDrawer({ column, onClose }: { column: ReportColumn | ReportResultColumn | null; onClose: () => void }) {
  return <Drawer open={Boolean(column)} title="字段详情" description="核对稳定逻辑字段、Oracle 物理字段与页面及 Excel 展示契约。" size="medium" onClose={onClose}>
    {column && isFullColumn(column) ? <div className={styles.content}>
      <section aria-labelledby="field-identity-heading"><h3 id="field-identity-heading">字段标识</h3><dl><Item label="稳定字段 ID" value={column.fieldId} mono /><Item label="逻辑字段" value={column.logicalCode} mono /><Item label="Oracle 字段" value={column.databaseColumn} mono /><Item label="Oracle 类型" value={column.sourceOracleType} mono /><Item label="精度 / 小数位" value={`${displayNumber(column.precision)} / ${displayNumber(column.scale)}`} /></dl></section>
      <section aria-labelledby="field-display-heading"><h3 id="field-display-heading">展示映射</h3><dl><Item label="页面表头" value={column.previewHeader} /><Item label="Excel 表头" value={column.excelHeader} /><Item label="展示类型" value={column.valueType} mono /><Item label="页面 / 导出顺序" value={`${column.displayOrder} / ${column.exportOrder}`} /><Item label="Excel 宽度" value={String(column.excelWidth)} /><Item label="空值显示" value={column.nullDisplay || '空字符串'} /></dl></section>
      <section aria-labelledby="field-capability-heading"><h3 id="field-capability-heading">能力与权限</h3><div className={styles.tags}><Flag enabled={column.nullable} label="可空" /><Flag enabled={column.previewVisible} label="预览可见" /><Flag enabled={column.filterable} label="允许筛选" /><Flag enabled={column.sortable} label="允许排序" /><Flag enabled={column.exportVisible} label="导出可见" /><Flag enabled={column.exportAllowed} label="允许导出" /></div><dl><Item label="允许操作符" value={stringList(column.allowedOperators)} mono /></dl></section>
      <section aria-labelledby="field-policy-heading"><h3 id="field-policy-heading">格式策略</h3><dl className={styles.policy}><JSONItem label="格式 JSON" value={column.format} /><JSONItem label="字典版本 JSON" value={column.dictionaryVersion} /><JSONItem label="掩码策略 JSON" value={column.maskingPolicy} /></dl></section>
    </div> : column ? <div className={styles.content}>
      <section aria-labelledby="result-field-identity-heading"><h3 id="result-field-identity-heading">结果字段</h3><dl><Item label="稳定字段 ID" value={column.fieldId} mono /><Item label="数据字段" value={column.code} mono /><Item label="页面表头" value={column.header} /><Item label="展示类型" value={column.valueType} mono /><Item label="空值显示" value={column.nullDisplay || '空字符串'} /></dl></section>
      <section aria-labelledby="result-field-capability-heading"><h3 id="result-field-capability-heading">当前运行能力</h3><div className={styles.tags}><Flag enabled={column.nullable} label="可空" /><Flag enabled={column.filterable} label="允许筛选" /><Flag enabled={column.sortable} label="允许排序" /></div><dl><Item label="允许操作符" value={stringList(column.allowedOperators)} mono /></dl></section>
      <p className={styles.snapshotNotice}>这是当前运行快照返回的公开字段契约；完整 Oracle 与 Excel 映射请在报表配置中查看。</p>
    </div> : null}
  </Drawer>
}

function isFullColumn(column: ReportColumn | ReportResultColumn): column is ReportColumn { return 'logicalCode' in column }

function Item({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return <div><dt>{label}</dt><dd className={mono ? styles.mono : undefined}>{value || '-'}</dd></div>
}

function JSONItem({ label, value }: { label: string; value: unknown }) {
  return <div><dt>{label}</dt><dd><pre>{value === undefined ? '未配置' : JSON.stringify(value, null, 2)}</pre></dd></div>
}

function Flag({ enabled, label }: { enabled: boolean; label: string }) {
  return <StatusTag tone={enabled ? 'success' : 'neutral'}>{label}：{enabled ? '是' : '否'}</StatusTag>
}

function displayNumber(value: number | null) { return value === null ? '-' : String(value) }
function stringList(value: unknown) { return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string').join(', ') || '未配置' : '未配置' }
