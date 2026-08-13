import type { ReactNode } from 'react'
import { Download, RefreshCcw } from 'lucide-react'
import { DataTable, StatusTag } from '../ui'
import { excelFieldSelectOptions, excelModelSelectOptions, type ExcelMatchModel, type ExcelMatchModelField } from '../excelMatchConfig'
import {
  canDownloadExcelJob,
  compactText,
  excelJobOperation,
  excelJobOperationLabel,
  excelJobProgressPercent,
  excelJobStatusLabel,
  excelLogLevelLabel,
  excelPreviewStat,
  excelPreviewStatusLabel,
  formatDate,
  formatUnixTime,
  type ExcelMatchJob,
  type ExcelMatchJobLog,
  type ExcelMatchPreviewResult,
  type ExcelMatchScheme,
} from './excelPageSupport'
import styles from './ExcelMatchPage.module.css'

export function ExcelSchemeList({ schemes, deletingSchemeID, onDelete, onOpen }: { schemes: ExcelMatchScheme[]; deletingSchemeID: number | null; onDelete: (scheme: ExcelMatchScheme) => void; onOpen: (id: number) => void }) {
  if (schemes.length === 0) return <EmptyState text="暂无已保存方案。" />
  return (
    <DataTable className={styles.schemeTable} density="compact" minWidth={620} scrollLabel="Excel 匹配方案">
      <thead><tr><th scope="col">方案名称</th><th scope="col">步骤数</th><th scope="col">更新时间</th><th scope="col">操作</th></tr></thead>
      <tbody>{schemes.map((scheme) => (
        <tr key={scheme.id}>
          <td>{scheme.name}</td>
          <td>{scheme.operation === 'export_match' ? (scheme.config.steps?.length || 1) : '-'}</td>
          <td>{formatUnixTime(scheme.updated_at)}</td>
          <td><div className={styles.tableActions}><button type="button" onClick={() => onOpen(scheme.id)} disabled={deletingSchemeID !== null}>打开配置</button><button type="button" className={styles.danger} onClick={() => onDelete(scheme)} disabled={deletingSchemeID !== null}>{deletingSchemeID === scheme.id ? '删除中…' : '删除'}</button></div></td>
        </tr>
      ))}</tbody>
    </DataTable>
  )
}

export function ExcelJobHistoryTable({ jobs, loading, downloadingJobID, selectedJobID, onView, onDownload }: { jobs: ExcelMatchJob[]; loading: boolean; downloadingJobID: number | null; selectedJobID: number | null; onView: (id: number, trigger: HTMLButtonElement) => void; onDownload: (id: number) => void }) {
  if (jobs.length === 0) return <EmptyState text="暂无 Excel 任务历史。" />
  return (
    <DataTable className={styles.historyTable} density="compact" minWidth={900} scrollLabel="Excel 任务列表">
      <thead><tr><th scope="col">ID</th><th scope="col">文件</th><th scope="col">类型</th><th scope="col">状态</th><th scope="col">处理行</th><th scope="col">匹配/未匹配</th><th scope="col">创建时间</th><th scope="col">操作</th></tr></thead>
      <tbody>{jobs.map((item) => (
        <tr className={item.id === selectedJobID ? styles.historyRowSelected : undefined} key={item.id}>
          <td>#{item.id}</td><td>{item.source_file_name || '-'}</td><td>{excelJobOperationLabel(excelJobOperation(item))}</td>
          <td><StatusTag tone={excelJobStatusTone(item.status)}>{excelJobStatusLabel(item.status)}</StatusTag></td>
          <td><div className={styles.jobRowProgress}><span>{item.processed_rows.toLocaleString('en-US')} / {item.total_rows.toLocaleString('en-US')}</span><span>{excelJobProgressPercent(item)}%</span><progress value={excelJobProgressPercent(item)} max="100" aria-label={`任务 #${item.id} 处理进度`} /></div></td>
          <td><span className={styles.jobMatchCount}>{item.matched_rows.toLocaleString('en-US')}</span> / <span className={styles.jobUnmatchedCount}>{item.unmatched_rows.toLocaleString('en-US')}</span></td>
          <td>{formatUnixTime(item.created_at)}</td>
          <td><div className={styles.tableActions}><button type="button" onClick={(event) => onView(item.id, event.currentTarget)} disabled={loading}>查看</button><button type="button" onClick={() => onDownload(item.id)} disabled={loading || downloadingJobID === item.id || !canDownloadExcelJob(item)} title={item.download_message || undefined}>{downloadingJobID === item.id ? '下载中' : '下载'}</button></div></td>
        </tr>
      ))}</tbody>
    </DataTable>
  )
}

export function ExcelJobDetailContent({ job, logs, progress, autoRefreshText, loading, downloading, onRefresh, onDownload }: { job: ExcelMatchJob; logs: ExcelMatchJobLog[]; progress: number; autoRefreshText: string; loading: boolean; downloading: boolean; onRefresh: () => void; onDownload: () => void }) {
  return (
    <section className={styles.jobDetailPanel} aria-label={`Excel 任务 ${job.id} 执行详情`}>
      {autoRefreshText && <p className={styles.modeNote} role="status">{autoRefreshText}</p>}
      <dl className={styles.jobDetailMeta}><div><dt>源文件</dt><dd>{job.source_file_name || '-'}</dd></div><div><dt>类型</dt><dd>{excelJobOperationLabel(excelJobOperation(job))}</dd></div></dl>
      <div className={styles.jobDetailCounts}><Metric label="匹配行" value={job.matched_rows.toLocaleString('en-US')} /><Metric label="未匹配行" value={job.unmatched_rows.toLocaleString('en-US')} /></div>
      <div className={styles.jobProgress}><strong>处理进度</strong><span>{job.processed_rows.toLocaleString('en-US')} / {job.total_rows.toLocaleString('en-US')}（{progress}%）</span><progress value={progress} max="100" aria-label={`Excel 任务 #${job.id} 处理进度`} /></div>
      <dl className={styles.jobExpiry}><dt>过期时间</dt><dd>{formatDate(job.expires_at)}</dd></dl>
      <div className={styles.jobLogHeading}>实时日志</div><ExcelJobLogList logs={logs} />
      <div className={styles.jobActionHeading}>操作</div>
      <div className={styles.detailActions}><button type="button" onClick={onRefresh} disabled={loading}><RefreshCcw aria-hidden="true" />刷新状态</button><button type="button" onClick={onDownload} disabled={loading || downloading || !canDownloadExcelJob(job)}><Download aria-hidden="true" />{downloading ? '下载中' : '下载结果'}</button>{!canDownloadExcelJob(job) && <span>{job.download_message || '任务完成并生成结果后可下载。'}</span>}</div>
      {job.status === 'failed' && <div className={styles.errorBanner} role="alert">任务执行失败，请查看受控服务日志。</div>}
    </section>
  )
}

export function ExcelMatchPreviewPanel({ preview }: { preview: ExcelMatchPreviewResult }) {
  return (
    <div className={styles.previewPanel}>
      <div className={styles.previewMetrics}><Metric label="扫描行" value={excelPreviewStat(preview.stats, 'TotalRows')} /><Metric label="参与步骤行" value={excelPreviewStat(preview.stats, 'FilteredRows')} /><Metric label="已匹配" value={excelPreviewStat(preview.stats, 'MatchedRows')} /><Metric label="未匹配" value={excelPreviewStat(preview.stats, 'UnmatchedRows')} /><Metric label="扫描上限" value={preview.truncated ? `${preview.scanLimit}+` : preview.scanLimit} /></div>
      {preview.samples.length > 0 ? (
        <DataTable className={styles.previewTable} density="compact" minWidth={960} scrollLabel="Excel 匹配预览">
          <thead><tr><th scope="col">行号</th><th scope="col">匹配键</th><th scope="col">状态</th><th scope="col">追加值</th><th scope="col">步骤结果</th><th scope="col">原因</th><th scope="col">Excel 行内容</th></tr></thead>
          <tbody>{preview.samples.map((sample) => (
            <tr key={`${sample.rowNumber}-${sample.matchKey}-${sample.status}`}>
              <td>{sample.rowNumber}</td><td>{sample.matchKey || '-'}</td><td>{excelPreviewStatusLabel(sample.status)}</td><td>{sample.matchedValue || '-'}</td>
              <td>{sample.stepResults?.length ? <div className={styles.previewSteps}>{sample.stepResults.map((step) => <span key={`${step.stepIndex}-${step.stepName}`}>{step.stepIndex}. {step.stepName || '未命名'}：{step.matchedValue || excelPreviewStatusLabel(step.status)}</span>)}</div> : '-'}</td>
              <td>{sample.reason || '-'}</td><td>{compactText(JSON.stringify(sample.values || {})) || '-'}</td>
            </tr>
          ))}</tbody>
        </DataTable>
      ) : <EmptyState text="预览没有返回样例。" />}
    </div>
  )
}

function ExcelJobLogList({ logs }: { logs: ExcelMatchJobLog[] }) {
  if (logs.length === 0) return <EmptyState text="暂无任务日志。" />
  return <div className={styles.jobLogList}>{logs.map((log) => <article className={styles.recordRow} key={log.id}><div><strong>{log.message || '任务日志已记录'}</strong><span>{excelLogLevelLabel(log.level)} / {formatUnixTime(log.created_at)}</span></div></article>)}</div>
}

export function Panel({ title, icon, meta, children }: { title: string; icon: ReactNode; meta: string; children: ReactNode }) { return <section className={styles.panel}><div className={styles.panelTitle}>{icon}<div><h3>{title}</h3><span>{meta}</span></div></div>{children}</section> }
export function SelectFilter({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: Array<{ value: string; label: string }> }) { return <label>{label}<select name={`filter-${label}`} value={value} onChange={(event) => onChange(event.currentTarget.value)}><option value="all">全部</option>{options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></label> }
function excelJobStatusTone(status: string): 'neutral' | 'success' | 'warning' | 'danger' { if (status === 'success') return 'success'; if (status === 'failed' || status === 'expired') return 'danger'; if (status === 'pending' || status === 'running') return 'warning'; return 'neutral' }
export function Metric({ label, value }: { label: string; value: ReactNode }) { return <div className={styles.metric}><span>{label}</span><strong>{value}</strong></div> }
export function EmptyState({ text }: { text: string }) { return <div className={styles.emptyState}>{text}</div> }
export function Field({ label, name, defaultValue = '', type = 'text', value, onChange, required = false }: { label: string; name: string; defaultValue?: string; type?: string; value?: string; onChange?: (value: string) => void; required?: boolean }) { return <label>{label}<input name={name} defaultValue={value === undefined ? defaultValue : undefined} value={value} type={type} required={required} onChange={onChange ? (event) => onChange(event.currentTarget.value) : undefined} /></label> }
export function ExcelModelSelector({ name, models, value, onChange }: { name: string; models: ExcelMatchModel[]; value: string; onChange: (value: string) => void }) { const selectedModel = models.find((model) => model.tableName === value); const options = excelModelSelectOptions(models, value); return <label className={styles.catalogControl}>模型名称<select aria-label="模型名称" name={name} value={value} required onChange={(event) => onChange(event.currentTarget.value)}><option value="">选择模型名称</option>{options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select>{selectedModel ? <ExcelCatalogExplanation title={selectedModel.mapping} detail={selectedModel.description} /> : value ? <ExcelCatalogExplanation title={`当前配置 → 数据库表 ${value}`} detail="该表不在当前模型目录中，保留它是为了兼容历史方案；保存前请确认表仍然存在。" /> : <ExcelCatalogExplanation title="模型名称 → 数据库表" detail="选择模型后，这里会直接解释对应的数据表，无需另行查表。" />}</label> }
export function ExcelModelFieldSelector({ label, name, models, tableName, value, onChange }: { label: string; name: string; models: ExcelMatchModel[]; tableName: string; value: string; onChange: (value: string) => void }) { const selectedModel = models.find((model) => model.tableName === tableName); const fields = selectedModel?.fields ?? []; const selectedField = fields.find((field) => field.columnName === value); const options = excelFieldSelectOptions(fields, value); return <label className={styles.catalogControl}>{label}<select aria-label={label} name={name} value={value} required disabled={!tableName} onChange={(event) => onChange(event.currentTarget.value)}><option value="">选择模型字段</option>{options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select>{selectedField ? <ExcelModelFieldExplanation field={selectedField} /> : value ? <ExcelCatalogExplanation title={`当前配置字段 → ${tableName}.${value}`} detail="该字段不在当前模型目录中，已作为历史配置保留；保存前请确认字段仍然存在。" /> : selectedModel ? <ExcelCatalogExplanation title={`${selectedModel.modelName}.字段 → ${selectedModel.tableName}.数据库列`} detail={`当前模型提供 ${fields.length} 个字段可选。`} /> : <ExcelCatalogExplanation title="模型字段 → 数据库列" detail="请先选择模型，再从该模型的字段列表中选择。" />}</label> }
function ExcelModelFieldExplanation({ field }: { field: ExcelMatchModelField }) { const typeDetail = field.dataType && !field.description.includes(field.dataType) ? `；数据库类型 ${field.dataType}` : ''; return <ExcelCatalogExplanation title={field.mapping} detail={`${field.description}${typeDetail}；${field.nullable ? '允许为空' : '不允许为空'}`} /> }
function ExcelCatalogExplanation({ title, detail }: { title: string; detail: string }) { return <span className={styles.catalogExplanation}><strong>{title}</strong><small>{detail}</small></span> }
