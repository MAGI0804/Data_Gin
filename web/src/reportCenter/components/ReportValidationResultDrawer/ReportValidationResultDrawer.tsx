import { CheckCircle2 } from 'lucide-react'
import { Drawer, StatusTag } from '../../../ui'
import type { ReportPublication } from '../../types'
import styles from './ReportValidationResultDrawer.module.css'

export function ReportValidationResultDrawer({ publication, onClose }: { publication: ReportPublication | null; onClose: () => void }) {
  const result = publication?.validation
  return (
    <Drawer
      open={Boolean(publication)}
      title="发布测试结果"
      description={publication ? `版本 #${publication.version} 已形成不可变契约；以下结果来自本次发布核验。` : undefined}
      size="medium"
      onClose={onClose}
      footer={<button type="button" onClick={onClose}>完成</button>}
    >
      {publication && result ? <div className={styles.content}>
        <div className={styles.summary} role="status"><CheckCircle2 aria-hidden="true" /><div><strong>Oracle 契约核验通过</strong><span>{formatDate(result.validatedAt)}</span></div><StatusTag tone="success">{publication.status || 'PUBLISHED'}</StatusTag></div>
        <ResultSection title="存储过程" rows={[
          ['过程', qualifiedName(result.procedure.owner, result.procedure.package, result.procedure.name)],
          ['Overload', result.procedure.overload || '—'],
          ['过程参数数量', result.procedure.argumentCount],
          ['签名 Hash', result.procedure.signatureHash],
        ]} />
        <ResultSection title="结果表与快照" rows={[
          ['结果表', qualifiedName(result.result.tableOwner, '', result.result.tableName)],
          ['字段数量', result.result.columnCount],
          ['结果 Schema Hash', result.result.schemaHash],
          ['run_id 字段', result.snapshot.runIdColumn],
          ['行游标字段', result.snapshot.rowIdColumn],
          ['唯一键校验', result.snapshot.uniqueKeyValidated ? '通过' : '未通过'],
        ]} />
        <ResultSection title="Excel 与发布契约" rows={[
          ['允许导出字段', result.export.exportableColumnCount],
          ['导出 Schema Hash', result.export.schemaHash],
          ['完整契约 Hash', publication.contractHash],
          ['发布时间', publication.publishedAt ? formatDate(publication.publishedAt) : '—'],
        ]} />
      </div> : publication ? <div className={styles.compatibility} role="status"><strong>报表已成功发布</strong><p>当前服务实例未返回校验摘要。请在服务滚动升级完成后发布新版本，即可查看过程签名、结果 Schema 与 Excel 契约核验详情。</p></div> : null}
    </Drawer>
  )
}

function ResultSection({ title, rows }: { title: string; rows: Array<[string, unknown]> }) {
  return <section className={styles.section}><h3>{title}</h3><dl>{rows.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{String(value ?? '—')}</dd></div>)}</dl></section>
}

function qualifiedName(owner: string, packageName: string, name: string) {
  return [owner, packageName, name].filter(Boolean).join('.') || '—'
}

function formatDate(value: string) {
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString('zh-CN')
}
