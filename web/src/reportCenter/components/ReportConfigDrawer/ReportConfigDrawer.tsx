import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { Button, Drawer } from '../../../ui'
import { getReportDraft, publishReportDraft, saveAndPublishReportDraft, saveReportDraft, type ReportCenterClient } from '../../api'
import { newReportInputField, parseExcelMappingDocument, parseReportInputSchemaDocument, excelMappingFromColumns } from '../../refCursorConfig'
import type { ReportDatasource, ReportDraft, ReportGrant, ReportPublication, ReportSummary } from '../../types'
import { ReportExcelMappingEditor } from './ReportExcelMappingEditor'
import { ReportInputSchemaEditor } from './ReportInputSchemaEditor'
import { ReportProcedureEditor } from './ReportProcedureEditor'
import styles from './ReportConfigDrawer.module.css'

type Tab = 'basic' | 'procedure' | 'conditions' | 'excel' | 'permissions'
const tabs: Array<{ key: Tab; label: string }> = [{ key: 'basic', label: '基本信息' }, { key: 'procedure', label: '存储过程' }, { key: 'conditions', label: '筛选条件' }, { key: 'excel', label: 'Excel 映射' }, { key: 'permissions', label: '权限' }]
const oracleIdentifierPattern = /^[A-Za-z][A-Za-z0-9_$#]{0,127}$/

export function ReportConfigDrawer({ client, report, datasources, datasourcesLoading = false, datasourcesError = '', onClose, onPublished, onSaved }: { client: ReportCenterClient; report: ReportSummary | null; datasources: ReportDatasource[]; datasourcesLoading?: boolean; datasourcesError?: string; onClose: () => void; onPublished?: (publication: ReportPublication) => void; onSaved?: () => void }) {
  const bodyRef = useRef<HTMLDivElement>(null)
  const [tab, setTab] = useState<Tab>('basic')
  const [draft, setDraft] = useState<ReportDraft>(() => emptyDraft())
  const [savedFingerprint, setSavedFingerprint] = useState('')
  const [state, setState] = useState({ loading: Boolean(report), saving: false, error: '', notice: '' })

  useEffect(() => {
    if (!report) return
    const controller = new AbortController()
    void getReportDraft(client, report.id, controller.signal).then((response) => {
      if (controller.signal.aborted) return
      if (!response.ok) setState((current) => ({ ...current, loading: false, error: response.error }))
      else {
        setDraft(response.data)
        setSavedFingerprint(draftFingerprint(response.data))
        setState((current) => ({ ...current, loading: false }))
      }
    })
    return () => controller.abort()
  }, [client, report])

  function validationError() {
    if (bodyRef.current?.querySelector('[aria-invalid="true"]')) return '请先修正标红的 JSON 配置。'
    if (!draft.name.trim() || draft.name.trim().length > 128 || !/^[A-Za-z][A-Za-z0-9_-]{2,63}$/.test(draft.code.trim()) || !draft.datasourceId) return '请完整填写报表名称、合法编码和 Oracle 数据源。'
    if (draft.category.trim().length > 64 || draft.description.trim().length > 500) return '分类最多 64 字，说明最多 500 字。'
    if (datasources.find((item) => item.id === draft.datasourceId)?.enabled === false) return '当前 Oracle 数据源已停用，请先启用或更换数据源。'
    if (!draft.procedure.owner || !draft.procedure.name) return '请从 Oracle 查询结果中选择存储过程。'
    if (!draft.procedure.jsonInputArgName || draft.procedure.resultCursorArgName) return '所选过程必须只有一个 JSON 输入参数，且不能包含任何出参。'
    if (!draft.result.tableOwner || !draft.result.tableName) return '请绑定 Oracle 结果表。'
    if (![draft.procedure.owner, draft.procedure.name, draft.procedure.jsonInputArgName, draft.result.tableOwner, draft.result.tableName].every((value) => oracleIdentifierPattern.test(value))) return 'Oracle 对象或字段名不合法，请从 Oracle 搜索结果中重新选择。'
    if (draft.procedure.package && !oracleIdentifierPattern.test(draft.procedure.package)) return 'Oracle 包名不合法，请重新选择存储过程。'
    try { parseReportInputSchemaDocument(draft.inputSchema) } catch (error) { return error instanceof Error ? error.message : '筛选条件配置不完整。' }
    try {
      const mapping = parseExcelMappingDocument(excelMappingFromColumns(draft.columns))
      if (Object.keys(mapping).length === 0) return '请至少配置一个 Oracle 结果表字段到 Excel 表头的映射。'
    } catch (error) { return error instanceof Error ? error.message : 'Excel 字段映射不完整。' }
    if (draft.grants.some((grant) => grant.subjectId <= 0 || grant.actions.length === 0)) return '每个权限主体都必须填写大于 0 的 ID，并至少选择查询或导出权限。'
    return ''
  }

  async function save() {
    const error = validationError()
    if (error) { setState((current) => ({ ...current, error })); return }
    setState((current) => ({ ...current, saving: true, error: '', notice: '' }))
    const response = await saveReportDraft(client, draft)
    if (!response.ok) { setState((current) => ({ ...current, saving: false, error: response.error })); return }
    setDraft(response.data)
    setSavedFingerprint(draftFingerprint(response.data))
    setState((current) => ({ ...current, saving: false, notice: '草稿已保存到 MySQL。' }))
    onSaved?.()
  }

  async function publish() {
    const error = validationError()
    if (error) { setState((current) => ({ ...current, error })); return }
    setState((current) => ({ ...current, saving: true, error: '', notice: '' }))
    let publication: ReportPublication
    if (dirty || !draft.id || !draft.lockVersion) {
      const response = await saveAndPublishReportDraft(client, draft)
      if (!response.ok) {
        if (response.draft) {
          setDraft(response.draft)
          setSavedFingerprint(draftFingerprint(response.draft))
        }
        setState((current) => ({ ...current, saving: false, error: response.error }))
        return
      }
      setDraft(response.draft)
      setSavedFingerprint(draftFingerprint(response.draft))
      publication = response.publication
    } else {
      const response = await publishReportDraft(client, draft.id, draft.lockVersion)
      if (!response.ok) { setState((current) => ({ ...current, saving: false, error: response.error })); return }
      publication = response.data
    }
    setState((current) => ({ ...current, saving: false, notice: `Oracle 契约核验通过，已发布版本 #${publication.versionId}。` }))
    onSaved?.()
    onClose()
    onPublished?.(publication)
  }

  const dirty = draftFingerprint(draft) !== savedFingerprint
  const footer = <><span className={styles.version}>版本锁 {draft.lockVersion || '新建'} · {dirty ? '有未发布修改' : '草稿已保存'}</span><button type="button" onClick={onClose}>取消</button><button type="button" onClick={() => void save()} disabled={state.loading || state.saving || !dirty}>{state.saving ? '处理中…' : '仅保存草稿'}</button><Button variant="primary" onClick={() => void publish()} disabled={state.loading || state.saving}>{state.saving ? '处理中…' : dirty || !draft.id ? '保存并核验发布' : '核验并发布'}</Button></>
  return <Drawer open title={report ? '编辑报表配置' : '创建报表配置'} description="配置保存于 MySQL；Oracle 过程仅接收一份 JSON，系统整表读取运行结果并在导出成功后清理。" size="wide" closeDisabled={state.saving} onClose={onClose} footer={footer}>
    <div className={styles.tabs} role="tablist" aria-label="报表配置步骤" onKeyDown={(event) => { const current = tabs.findIndex((item) => item.key === tab); const delta = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0; if (!delta) return; event.preventDefault(); const next = tabs[(current + delta + tabs.length) % tabs.length]; setTab(next.key); document.getElementById(`report-config-tab-${next.key}`)?.focus() }}>{tabs.map((item) => <button id={`report-config-tab-${item.key}`} type="button" role="tab" aria-selected={tab === item.key} aria-controls="report-config-panel" tabIndex={tab === item.key ? 0 : -1} className={tab === item.key ? styles.active : ''} onClick={() => setTab(item.key)} key={item.key}>{item.label}</button>)}</div>
    <div id="report-config-panel" role="tabpanel" aria-labelledby={`report-config-tab-${tab}`} tabIndex={0} ref={bodyRef} className={styles.body}>
      {state.loading ? <p>正在读取草稿…</p> : <fieldset className={styles.editorFieldset} disabled={state.saving} aria-busy={state.saving || undefined}><Editor tab={tab} client={client} draft={draft} datasources={datasources} datasourcesLoading={datasourcesLoading} datasourcesError={datasourcesError} onChange={setDraft} /></fieldset>}
      {state.error ? <div className={styles.error} role="alert">{state.error}</div> : null}
      {state.notice ? <div className={styles.notice} role="status">{state.notice}</div> : null}
    </div>
  </Drawer>
}

function Editor({ tab, client, draft, datasources, datasourcesLoading, datasourcesError, onChange }: { tab: Tab; client: ReportCenterClient; draft: ReportDraft; datasources: ReportDatasource[]; datasourcesLoading: boolean; datasourcesError: string; onChange: Dispatch<SetStateAction<ReportDraft>> }) {
  const set = <K extends keyof ReportDraft>(key: K, value: ReportDraft[K]) => onChange((current) => ({ ...current, [key]: value }))
  if (tab === 'basic') {
    const selected = datasources.find((item) => item.id === draft.datasourceId)
    const unavailable = draft.datasourceId > 0 && !selected
    return <div className={styles.form}>
      <Field label="报表名称"><input value={draft.name} onChange={(event) => set('name', event.currentTarget.value)} /></Field>
      <Field label="报表编码"><input value={draft.code} onChange={(event) => set('code', event.currentTarget.value)} /></Field>
      <Field label="分类"><input value={draft.category} onChange={(event) => set('category', event.currentTarget.value)} /></Field>
      <Field label="Oracle 数据源"><select value={draft.datasourceId || ''} disabled={datasourcesLoading} onChange={(event) => {
        const datasourceId = Number(event.currentTarget.value)
        onChange(datasourceId === draft.datasourceId ? draft : { ...draft, datasourceId, procedure: emptyProcedure(), executionMode: 'TABLE_SNAPSHOT', result: emptyResult() })
      }}><option value="">{datasourcesLoading ? '正在加载数据源…' : '请选择数据源'}</option>{unavailable ? <option value={draft.datasourceId}>#{draft.datasourceId}（当前不可读取，请勿误切换）</option> : null}{datasources.map((item) => <option value={item.id} disabled={!item.enabled} key={item.id}>{item.name} · {item.code}{item.enabled ? '' : '（已停用）'}</option>)}</select>{datasourcesError ? <small className={styles.fieldError}>{datasourcesError}</small> : null}{selected && !selected.enabled ? <small className={styles.fieldError}>当前绑定数据源已停用，不能发布或创建新运行。</small> : null}</Field>
      <Field label="说明" wide><textarea rows={4} value={draft.description} onChange={(event) => set('description', event.currentTarget.value)} /></Field>
    </div>
  }
  if (tab === 'procedure') return <ReportProcedureEditor client={client} draft={draft} onChange={onChange} />
  if (tab === 'conditions') return <ReportInputSchemaEditor schema={draft.inputSchema} onChange={(inputSchema) => set('inputSchema', inputSchema)} />
  if (tab === 'excel') return <ReportExcelMappingEditor columns={draft.columns} onChange={(columns) => set('columns', columns)} />
  return <PermissionEditor grants={draft.grants} onChange={(grants) => set('grants', grants)} />
}

function PermissionEditor({ grants, onChange }: { grants: ReportGrant[]; onChange: (grants: ReportGrant[]) => void }) {
  return <div className={styles.list}><div className={styles.listHeader}><strong>报表级用户/角色授权</strong><button type="button" onClick={() => onChange([...grants, { subjectType: 'ROLE', subjectId: 0, actions: ['QUERY'] }])}><Plus aria-hidden="true" />新增</button></div>{grants.map((grant, index) => <div className={styles.row} key={`${grant.subjectType}-${index}`}><select aria-label="主体类型" value={grant.subjectType} onChange={(event) => onChange(replaceAt(grants, index, { ...grant, subjectType: event.currentTarget.value as 'USER' | 'ROLE' }))}><option value="ROLE">角色</option><option value="USER">用户</option></select><input type="number" min="1" aria-label="主体 ID" value={grant.subjectId || ''} onChange={(event) => onChange(replaceAt(grants, index, { ...grant, subjectId: Number(event.currentTarget.value) }))} /><label><input type="checkbox" checked={grant.actions.includes('QUERY')} onChange={(event) => onChange(replaceAt(grants, index, { ...grant, actions: toggle(grant.actions, 'QUERY', event.currentTarget.checked) }))} />查询</label><label><input type="checkbox" checked={grant.actions.includes('EXPORT')} onChange={(event) => onChange(replaceAt(grants, index, { ...grant, actions: toggle(grant.actions, 'EXPORT', event.currentTarget.checked) }))} />导出</label><button className={styles.delete} type="button" aria-label="删除此授权" onClick={() => onChange(grants.filter((_, itemIndex) => itemIndex !== index))}><Trash2 aria-hidden="true" /></button></div>)}</div>
}

function Field({ label, wide, children }: { label: string; wide?: boolean; children: React.ReactNode }) { return <label className={wide ? styles.wide : ''}>{label}{children}</label> }
function emptyProcedure(): ReportDraft['procedure'] { return { owner: '', package: '', name: '', overload: '', jsonInputArgName: '', resultCursorArgName: '' } }
function emptyResult(): ReportDraft['result'] { return { tableOwner: '', tableName: '' } }
function emptyDraft(): ReportDraft { const [code, field] = newReportInputField(0); return { id: 0, code: '', name: '', category: '', description: '', datasourceId: 0, status: 'DRAFT', lockVersion: 0, executionMode: 'TABLE_SNAPSHOT', procedure: emptyProcedure(), inputSchema: { [code]: field }, result: emptyResult(), callTemplate: '', parameters: [], columns: [], grants: [], createdAt: null, updatedAt: null } }
function replaceAt<T>(items: T[], index: number, item: T) { return items.map((value, position) => position === index ? item : value) }
function toggle(values: string[], value: string, checked: boolean) { return checked ? [...new Set([...values, value])] : values.filter((item) => item !== value) }
function draftFingerprint(draft: ReportDraft) { return JSON.stringify(draft) }
