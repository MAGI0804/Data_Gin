import { useEffect, useRef, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { Button, Drawer } from '../../../ui'
import { getReportDraft, publishReportDraft, saveReportDraft, type ReportCenterClient } from '../../api'
import { reportParameterFlagDisabled, updateReportParameterFlag } from '../../parameterConfig'
import type { ReportColumn, ReportDatasource, ReportDraft, ReportParameter, ReportSummary } from '../../types'
import styles from './ReportConfigDrawer.module.css'

type Tab = 'basic' | 'procedure' | 'parameters' | 'fields' | 'excel' | 'permissions'
const tabs: Array<{ key: Tab; label: string }> = [{ key: 'basic', label: '基本信息' }, { key: 'procedure', label: '存储过程' }, { key: 'parameters', label: '{{形参}}' }, { key: 'fields', label: '结果字段' }, { key: 'excel', label: 'Excel' }, { key: 'permissions', label: '权限' }]

export function ReportConfigDrawer({ client, report, datasources, datasourcesLoading = false, datasourcesError = '', embedded = false, onClose, onSaved }: { client: ReportCenterClient; report: ReportSummary | null; datasources: ReportDatasource[]; datasourcesLoading?: boolean; datasourcesError?: string; embedded?: boolean; onClose: () => void; onSaved?: () => void }) {
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
      else { setDraft(response.data); setSavedFingerprint(draftFingerprint(response.data)); setState((current) => ({ ...current, loading: false })) }
    })
    return () => controller.abort()
  }, [client, report])

  async function save() {
    if (bodyRef.current?.querySelector('[aria-invalid="true"]')) { setState((current) => ({ ...current, error: '请先修正标红的 JSON 配置。' })); return }
    setState((current) => ({ ...current, saving: true, error: '', notice: '' }))
    const response = await saveReportDraft(client, draft)
    if (!response.ok) { setState((current) => ({ ...current, saving: false, error: response.error })); return }
    setDraft(response.data); setSavedFingerprint(draftFingerprint(response.data)); setState((current) => ({ ...current, saving: false, notice: '草稿已保存到 MySQL。' })); onSaved?.()
  }
  async function publish() {
    if (!draft.id || !draft.lockVersion || draftFingerprint(draft) !== savedFingerprint) return
    if (bodyRef.current?.querySelector('[aria-invalid="true"]')) { setState((current) => ({ ...current, error: '请先修正标红的 JSON 配置。' })); return }
    setState((current) => ({ ...current, saving: true, error: '', notice: '' }))
    const response = await publishReportDraft(client, draft.id, draft.lockVersion)
    if (!response.ok) { setState((current) => ({ ...current, saving: false, error: response.error })); return }
    setState((current) => ({ ...current, saving: false, notice: `Oracle 契约核验通过，已发布版本 #${response.data.versionId}。` })); onSaved?.(); onClose()
  }

  const dirty = draftFingerprint(draft) !== savedFingerprint
  const footer = <><span className={styles.version}>版本锁 {draft.lockVersion || '新建'} · {dirty ? '有未保存修改' : '已保存'}</span><button type="button" onClick={onClose}>取消</button><button type="button" onClick={() => void save()} disabled={state.loading || state.saving || !dirty}>{state.saving ? '处理中…' : '保存草稿'}</button><Button variant="primary" onClick={() => void publish()} disabled={!draft.id || state.saving || dirty} title={dirty ? '请先保存草稿，再核验并发布' : undefined}>核验并发布</Button></>
  const editor = <><div className={styles.tabs} role="tablist" aria-label="报表配置步骤" onKeyDown={(event) => { const current = tabs.findIndex((item) => item.key === tab); const delta = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0; if (!delta) return; event.preventDefault(); const next = tabs[(current + delta + tabs.length) % tabs.length]; setTab(next.key); document.getElementById(`report-config-tab-${next.key}`)?.focus() }}>{tabs.map((item) => <button id={`report-config-tab-${item.key}`} type="button" role="tab" aria-selected={tab === item.key} aria-controls="report-config-panel" tabIndex={tab === item.key ? 0 : -1} className={tab === item.key ? styles.active : ''} onClick={() => setTab(item.key)} key={item.key}>{item.label}</button>)}</div><div id="report-config-panel" role="tabpanel" aria-labelledby={`report-config-tab-${tab}`} tabIndex={0} ref={bodyRef} className={styles.body}>{state.loading ? <p>正在读取草稿…</p> : <Editor tab={tab} draft={draft} datasources={datasources} datasourcesLoading={datasourcesLoading} datasourcesError={datasourcesError} onChange={setDraft} />}{state.error ? <div className={styles.error} role="alert">{state.error}</div> : null}{state.notice ? <div className={styles.notice} role="status">{state.notice}</div> : null}</div></>
  if (embedded) return <section className={styles.embedded} aria-label={report ? `编辑报表配置：${report.name}` : '创建报表配置'}><header className={styles.embeddedHeader}><div><strong>{report ? report.name : '创建报表配置'}</strong><span>配置保存于 MySQL；发布时在线核验 Oracle 过程签名和结果 Schema。</span></div></header>{editor}<footer className={styles.embeddedFooter}>{footer}</footer></section>
  return <Drawer open title={report ? '编辑报表配置' : '创建报表配置'} description="配置保存于 MySQL；发布时在线核验 Oracle 过程签名和结果 Schema。" size="wide" closeDisabled={state.saving} onClose={onClose} footer={footer}>{editor}</Drawer>
}

function Editor({ tab, draft, datasources, datasourcesLoading, datasourcesError, onChange }: { tab: Tab; draft: ReportDraft; datasources: ReportDatasource[]; datasourcesLoading: boolean; datasourcesError: string; onChange: (draft: ReportDraft) => void }) {
  const set = <K extends keyof ReportDraft>(key: K, value: ReportDraft[K]) => onChange({ ...draft, [key]: value })
  if (tab === 'basic') {
    const selected = datasources.find((item) => item.id === draft.datasourceId)
    const unavailable = draft.datasourceId > 0 && !selected
    return <div className={styles.form}><Field label="报表名称"><input value={draft.name} onChange={(e) => set('name', e.currentTarget.value)} /></Field><Field label="报表编码"><input value={draft.code} onChange={(e) => set('code', e.currentTarget.value)} /></Field><Field label="分类"><input value={draft.category} onChange={(e) => set('category', e.currentTarget.value)} /></Field><Field label="Oracle 数据源"><select value={draft.datasourceId || ''} disabled={datasourcesLoading} onChange={(e) => set('datasourceId', Number(e.currentTarget.value))}><option value="">{datasourcesLoading ? '正在加载数据源…' : '请选择数据源'}</option>{unavailable ? <option value={draft.datasourceId}>#{draft.datasourceId}（当前不可读取，请勿误切换）</option> : null}{datasources.map((item) => <option value={item.id} disabled={!item.enabled} key={item.id}>{item.name} · {item.code}{item.enabled ? '' : '（已停用）'}</option>)}</select>{datasourcesError ? <small className={styles.fieldError}>{datasourcesError}</small> : null}{selected && !selected.enabled ? <small className={styles.fieldError}>当前绑定数据源已停用；历史任务仍可读取，但不能发布或创建新运行。</small> : null}</Field><Field label="说明" wide><textarea rows={4} value={draft.description} onChange={(e) => set('description', e.currentTarget.value)} /></Field></div>
  }
  if (tab === 'procedure') return <div className={styles.form}>{(['owner', 'package', 'name', 'overload'] as const).map((key) => <Field label={({ owner: 'Owner', package: 'Package（可选）', name: 'Procedure', overload: 'Overload（可选）' })[key]} key={key}><input className={styles.mono} value={draft.procedure[key]} onChange={(e) => set('procedure', { ...draft.procedure, [key]: e.currentTarget.value })} /></Field>)}{(['tableOwner', 'tableName', 'runIdColumn', 'rowIdColumn'] as const).map((key) => <Field label={({ tableOwner: '结果表 Owner', tableName: '结果表名', runIdColumn: 'run_id 字段', rowIdColumn: '行游标字段' })[key]} key={key}><input className={styles.mono} value={draft.result[key]} onChange={(e) => set('result', { ...draft.result, [key]: e.currentTarget.value })} /></Field>)}<Field label="调用模板（只允许 {{形参}} 绑定）" wide><textarea className={styles.mono} rows={7} value={draft.callTemplate} onChange={(e) => set('callTemplate', e.currentTarget.value)} /></Field></div>
  if (tab === 'parameters') return <ListEditor title="参数契约" onAdd={() => set('parameters', [...draft.parameters, newParameter(nextOrder(draft.parameters.map((item) => item.position)))])}>{draft.parameters.map((item, index) => <ParameterRow item={item} key={`${item.code}-${index}`} onChange={(next) => set('parameters', replaceAt(draft.parameters, index, next))} onDelete={() => set('parameters', removeAt(draft.parameters, index))} />)}</ListEditor>
  if (tab === 'fields' || tab === 'excel') return <ListEditor title={tab === 'fields' ? '稳定逻辑字段 → Oracle 物理字段' : '页面字段 → Excel 表头'} onAdd={() => set('columns', [...draft.columns, newColumn(nextOrder(draft.columns.flatMap((item) => [item.displayOrder, item.exportOrder])))])}>{draft.columns.map((item, index) => <ColumnRow excel={tab === 'excel'} item={item} key={item.fieldId} onChange={(next) => set('columns', replaceAt(draft.columns, index, next))} onDelete={() => set('columns', removeAt(draft.columns, index))} />)}</ListEditor>
  return <ListEditor title="报表级用户/角色授权" onAdd={() => set('grants', [...draft.grants, { subjectType: 'ROLE', subjectId: 0, actions: ['QUERY'] }])}>{draft.grants.map((grant, index) => <div className={styles.row} key={`${grant.subjectType}-${index}`}><select value={grant.subjectType} onChange={(e) => set('grants', replaceAt(draft.grants, index, { ...grant, subjectType: e.currentTarget.value as 'USER' | 'ROLE' }))}><option value="ROLE">角色</option><option value="USER">用户</option></select><input type="number" min="1" aria-label="主体 ID" value={grant.subjectId || ''} onChange={(e) => set('grants', replaceAt(draft.grants, index, { ...grant, subjectId: Number(e.currentTarget.value) }))} /><label><input type="checkbox" checked={grant.actions.includes('QUERY')} onChange={(e) => set('grants', replaceAt(draft.grants, index, { ...grant, actions: toggle(grant.actions, 'QUERY', e.currentTarget.checked) }))} />查询</label><label><input type="checkbox" checked={grant.actions.includes('EXPORT')} onChange={(e) => set('grants', replaceAt(draft.grants, index, { ...grant, actions: toggle(grant.actions, 'EXPORT', e.currentTarget.checked) }))} />导出</label><Delete onClick={() => set('grants', removeAt(draft.grants, index))} /></div>)}</ListEditor>
}

function ParameterRow({ item, onChange, onDelete }: { item: ReportParameter; onChange: (item: ReportParameter) => void; onDelete: () => void }) {
  const setLogicalType = (logicalType: ReportParameter['logicalType']) => {
    const source = item.valueSource.source
    const compatibleSource = (source === 'RUN_ID' && logicalType === 'string') || (source === 'ACTOR_ID' && logicalType === 'integer')
    const supportsNormalizer = logicalType === 'string' || logicalType === 'enum' || logicalType === 'multi_enum'
    onChange({
      ...item,
      logicalType,
      controlType: logicalType === 'boolean' ? 'CHECKBOX' : logicalType === 'date' ? 'DATE' : logicalType === 'datetime' ? 'DATETIME' : logicalType === 'enum' ? 'SELECT' : logicalType === 'multi_enum' ? 'MULTI_SELECT' : logicalType === 'integer' || logicalType === 'decimal' ? 'NUMBER' : logicalType === 'json' ? 'TEXTAREA' : item.controlType,
      cardinality: logicalType === 'multi_enum' ? 'MULTIPLE' : 'SINGLE',
      collectionEncoding: logicalType === 'multi_enum' ? 'JSON_CLOB' : '',
      normalizer: supportsNormalizer ? item.normalizer : {},
      valueSource: compatibleSource ? item.valueSource : {},
    })
  }
  return <div className={styles.contractRow}>
    <ContractField label="参数编码"><input className={styles.mono} value={item.code} onChange={(event) => onChange({ ...item, code: event.currentTarget.value })} /></ContractField>
    <ContractField label="显示名称"><input value={item.label} onChange={(event) => onChange({ ...item, label: event.currentTarget.value })} /></ContractField>
    <ContractField label="显示顺序"><input type="number" min="0" step="1" value={item.displayOrder} onChange={(event) => onChange({ ...item, displayOrder: Number(event.currentTarget.value) })} /></ContractField>
    <ContractField label="过程参数名"><input className={styles.mono} value={item.procedureArgName} onChange={(event) => onChange({ ...item, procedureArgName: event.currentTarget.value })} /></ContractField>
    <ContractField label="过程位置"><input type="number" min="1" step="1" value={item.position} onChange={(event) => onChange({ ...item, position: Number(event.currentTarget.value) })} /></ContractField>
    <ContractField label="控件类型"><select value={item.controlType} onChange={(event) => onChange({ ...item, controlType: event.currentTarget.value as ReportParameter['controlType'] })}>{['TEXT','TEXTAREA','NUMBER','CHECKBOX','DATE','DATETIME','SELECT','MULTI_SELECT'].map((value) => <option key={value}>{value}</option>)}</select></ContractField>
    <ContractField label="业务类型"><select value={item.logicalType} onChange={(event) => setLogicalType(event.currentTarget.value as ReportParameter['logicalType'])}>{['string','integer','decimal','boolean','date','datetime','enum','multi_enum','json'].map((value) => <option key={value}>{value}</option>)}</select></ContractField>
    <ContractField label="参数基数"><select value={item.cardinality} onChange={(event) => onChange({ ...item, cardinality: event.currentTarget.value as ReportParameter['cardinality'], collectionEncoding: event.currentTarget.value === 'MULTIPLE' ? 'JSON_CLOB' : '' })}><option value="SINGLE">单值</option><option value="MULTIPLE">多值</option></select></ContractField>
    <ContractField label="Oracle 类型"><input className={styles.mono} value={item.oracleType} onChange={(event) => onChange({ ...item, oracleType: event.currentTarget.value })} /></ContractField>
    <NullableNumber label="精度" min={1} max={38} value={item.precision} onChange={(precision) => onChange({ ...item, precision })} />
    <NullableNumber label="小数位" min={-84} max={127} value={item.scale} onChange={(scale) => onChange({ ...item, scale })} />
    <NullableNumber label="最大长度" min={1} max={1000000} value={item.maxLength} onChange={(maxLength) => onChange({ ...item, maxLength })} />
    <ContractField label="允许值（逗号分隔）"><input value={item.allowedValues.join(',')} onChange={(event) => onChange({ ...item, allowedValues: event.currentTarget.value.split(',').map((value) => value.trim()).filter(Boolean) })} /></ContractField>
    <ContractField label="时区"><input value={item.timezone} onChange={(event) => onChange({ ...item, timezone: event.currentTarget.value })} /></ContractField>
    <ContractField label="空值策略"><select value={item.nullPolicy} onChange={(event) => onChange({ ...item, nullPolicy: event.currentTarget.value })}><option value="TYPED_NULL">TYPED_NULL</option></select></ContractField>
    <ContractField label="集合编码"><select value={item.collectionEncoding ?? ''} disabled={item.cardinality !== 'MULTIPLE'} onChange={(event) => onChange({ ...item, collectionEncoding: event.currentTarget.value })}><option value="">不使用</option><option value="JSON_CLOB">JSON_CLOB</option></select></ContractField>
    <JsonInput label="默认值 JSON" value={item.defaultValue} disabled={item.sensitive} onChange={(defaultValue) => onChange({ ...item, defaultValue })} />
    <JsonInput label="校验规则 JSON" value={item.validation} shape="object" onChange={(validation) => onChange({ ...item, validation: validation as Record<string, unknown> })} />
    <NormalizerField value={item.normalizer} disabled={item.systemInjected} onChange={(normalizer) => onChange({ ...item, normalizer })} />
    <ValueSourceField code={item.code} logicalType={item.logicalType} systemInjected={item.systemInjected} value={item.valueSource} onChange={(valueSource) => onChange({ ...item, valueSource })} />
    <ContractField label="校验提示"><input value={item.errorMessage} onChange={(event) => onChange({ ...item, errorMessage: event.currentTarget.value })} /></ContractField>
    <label className={styles.flag}><input type="checkbox" checked={item.required} onChange={(event) => onChange({ ...item, required: event.currentTarget.checked, nullable: event.currentTarget.checked ? false : item.nullable })} />必填</label>
    <label className={styles.flag}><input type="checkbox" checked={item.nullable} disabled={item.required} onChange={(event) => onChange({ ...item, nullable: event.currentTarget.checked })} />可空</label>
    <label className={styles.flag}><input type="checkbox" checked={item.systemInjected} disabled={reportParameterFlagDisabled(item, 'systemInjected')} onChange={(event) => onChange(updateReportParameterFlag(item, 'systemInjected', event.currentTarget.checked))} />系统注入</label>
    <label className={styles.flag}><input type="checkbox" checked={item.sensitive} disabled={reportParameterFlagDisabled(item, 'sensitive')} onChange={(event) => onChange(updateReportParameterFlag(item, 'sensitive', event.currentTarget.checked))} />敏感</label>
    {item.systemInjected && item.sensitive ? <span className={styles.fieldError} role="alert">历史配置不允许同时启用“系统注入”和“敏感”，请取消其中一项。</span> : null}
    <Delete onClick={onDelete} />
  </div>
}
function ColumnRow({ item, excel, onChange, onDelete }: { item: ReportColumn; excel: boolean; onChange: (item: ReportColumn) => void; onDelete: () => void }) {
  return <div className={styles.contractRow}>{excel ? <>
    <ContractField label="页面表头"><input value={item.previewHeader} onChange={(event) => onChange({ ...item, previewHeader: event.currentTarget.value })} /></ContractField>
    <ContractField label="Excel 表头"><input value={item.excelHeader} onChange={(event) => onChange({ ...item, excelHeader: event.currentTarget.value })} /></ContractField>
    <ContractField label="页面顺序"><input type="number" min="0" step="1" value={item.displayOrder} onChange={(event) => onChange({ ...item, displayOrder: Number(event.currentTarget.value) })} /></ContractField>
    <ContractField label="导出顺序"><input type="number" min="0" step="1" value={item.exportOrder} onChange={(event) => onChange({ ...item, exportOrder: Number(event.currentTarget.value) })} /></ContractField>
    <ContractField label="Excel 宽度"><input type="number" min="0" max="255" step="0.1" value={item.excelWidth} onChange={(event) => onChange({ ...item, excelWidth: Number(event.currentTarget.value) })} /></ContractField>
    <ContractField label="空值显示"><input value={item.nullDisplay} onChange={(event) => onChange({ ...item, nullDisplay: event.currentTarget.value })} /></ContractField>
    <JsonInput label="格式 JSON" value={item.format} shape="object" onChange={(format) => onChange({ ...item, format })} />
    <JsonInput label="字典版本 JSON" value={item.dictionaryVersion} shape="object" onChange={(dictionaryVersion) => onChange({ ...item, dictionaryVersion })} />
    <JsonInput label="掩码策略 JSON" value={item.maskingPolicy} shape="object" onChange={(maskingPolicy) => onChange({ ...item, maskingPolicy })} />
    <label className={styles.flag}><input type="checkbox" checked={item.previewVisible} onChange={(event) => onChange({ ...item, previewVisible: event.currentTarget.checked })} />预览可见</label>
    <label className={styles.flag}><input type="checkbox" checked={item.exportVisible} onChange={(event) => onChange({ ...item, exportVisible: event.currentTarget.checked })} />导出可见</label>
    <label className={styles.flag}><input type="checkbox" checked={item.exportAllowed} onChange={(event) => onChange({ ...item, exportAllowed: event.currentTarget.checked })} />允许导出</label>
  </> : <>
    <ContractField label="稳定字段 ID"><input className={styles.mono} value={item.fieldId} readOnly /></ContractField>
    <ContractField label="逻辑字段"><input className={styles.mono} value={item.logicalCode} onChange={(event) => onChange({ ...item, logicalCode: event.currentTarget.value })} /></ContractField>
    <ContractField label="Oracle 字段"><input className={styles.mono} value={item.databaseColumn} onChange={(event) => onChange({ ...item, databaseColumn: event.currentTarget.value })} /></ContractField>
    <ContractField label="Oracle 类型"><input className={styles.mono} value={item.sourceOracleType} onChange={(event) => onChange({ ...item, sourceOracleType: event.currentTarget.value })} /></ContractField>
    <NullableNumber label="精度" min={1} max={38} value={item.precision} onChange={(precision) => onChange({ ...item, precision })} />
    <NullableNumber label="小数位" min={-84} max={127} value={item.scale} onChange={(scale) => onChange({ ...item, scale })} />
    <ContractField label="展示类型"><select value={item.valueType} onChange={(event) => onChange({ ...item, valueType: event.currentTarget.value })}>{['string','integer','decimal','boolean','date','datetime','enum','json'].map((value) => <option key={value}>{value}</option>)}</select></ContractField>
    <ContractField label="页面顺序"><input type="number" min="0" step="1" value={item.displayOrder} onChange={(event) => onChange({ ...item, displayOrder: Number(event.currentTarget.value) })} /></ContractField>
    <JsonInput label="允许操作符 JSON" value={item.allowedOperators} shape="array" onChange={(allowedOperators) => onChange({ ...item, allowedOperators })} />
    <label className={styles.flag}><input type="checkbox" checked={item.nullable} onChange={(event) => onChange({ ...item, nullable: event.currentTarget.checked })} />可空</label>
    <label className={styles.flag}><input type="checkbox" checked={item.filterable} onChange={(event) => onChange({ ...item, filterable: event.currentTarget.checked })} />允许筛选</label>
    <label className={styles.flag}><input type="checkbox" checked={item.sortable} onChange={(event) => onChange({ ...item, sortable: event.currentTarget.checked })} />允许排序</label>
  </>}<Delete onClick={onDelete} /></div>
}
function Field({ label, wide, children }: { label: string; wide?: boolean; children: React.ReactNode }) { return <label className={wide ? styles.wide : ''}>{label}{children}</label> }
function ListEditor({ title, onAdd, children }: { title: string; onAdd: () => void; children: React.ReactNode }) { return <div className={styles.list}><div className={styles.listHeader}><strong>{title}</strong><button type="button" onClick={onAdd}><Plus aria-hidden="true" />新增</button></div>{children}</div> }
function Delete({ onClick }: { onClick: () => void }) { return <button className={styles.delete} type="button" aria-label="删除此行" onClick={onClick}><Trash2 aria-hidden="true" /></button> }
function ContractField({ label, children }: { label: string; children: React.ReactNode }) { return <label className={styles.contractField}><span>{label}</span>{children}</label> }
function NullableNumber({ label, value, min, max, onChange }: { label: string; value: number | null; min: number; max: number; onChange: (value: number | null) => void }) { return <ContractField label={label}><input type="number" min={min} max={max} step="1" value={value ?? ''} onChange={(event) => onChange(event.currentTarget.value === '' ? null : Number(event.currentTarget.value))} /></ContractField> }
function NormalizerField({ value, disabled, onChange }: { value: Record<string, unknown>; disabled: boolean; onChange: (value: Record<string, unknown>) => void }) { const trim = value.trim === true; const letterCase = value.case === 'UPPER' || value.case === 'LOWER' ? value.case : ''; return <fieldset className={styles.contractGroup} disabled={disabled}><legend>归一化</legend><label className={styles.flag}><input type="checkbox" checked={trim} onChange={(event) => onChange(compactObject({ trim: event.currentTarget.checked, case: letterCase }))} />去除首尾空格</label><select aria-label="大小写归一化" value={letterCase} onChange={(event) => onChange(compactObject({ trim, case: event.currentTarget.value }))}><option value="">保留大小写</option><option value="UPPER">转大写</option><option value="LOWER">转小写</option></select></fieldset> }
function ValueSourceField({ code, logicalType, systemInjected, value, onChange }: { code: string; logicalType: ReportParameter['logicalType']; systemInjected: boolean; value: Record<string, unknown>; onChange: (value: Record<string, unknown>) => void }) { const fallback = systemInjected && code === 'runId' && logicalType === 'string' ? 'RUN_ID' : ''; const source = value.source === 'RUN_ID' || value.source === 'ACTOR_ID' ? value.source : fallback; return <ContractField label="系统值来源"><select disabled={!systemInjected} value={source} onChange={(event) => onChange(event.currentTarget.value ? { source: event.currentTarget.value } : {})}><option value="">请选择</option><option value="RUN_ID" disabled={logicalType !== 'string'}>运行 UUID（字符串）</option><option value="ACTOR_ID" disabled={logicalType !== 'integer'}>当前用户 ID（整数）</option></select></ContractField> }
function JsonInput({ label, value, disabled, shape, onChange }: { label: string; value: unknown; disabled?: boolean; shape?: 'object' | 'array'; onChange: (value: unknown) => void }) { const serialized = value === undefined ? '' : JSON.stringify(value); const [text, setText] = useState(serialized); const [invalid, setInvalid] = useState(false); useEffect(() => { setText(serialized); setInvalid(false) }, [serialized]); return <label className={styles.jsonField}><span>{label}</span><input className={styles.mono} value={text} disabled={disabled} aria-invalid={invalid || undefined} onChange={(event) => setText(event.currentTarget.value)} onBlur={() => { if (!text.trim()) { setInvalid(false); onChange(undefined); return } try { const parsed = JSON.parse(text) as unknown; if ((shape === 'object' && (!parsed || typeof parsed !== 'object' || Array.isArray(parsed))) || (shape === 'array' && !Array.isArray(parsed))) { setInvalid(true); return } setInvalid(false); onChange(parsed) } catch { setInvalid(true) } }} /></label> }
function compactObject(value: { trim: boolean; case: string }) { return { ...(value.trim ? { trim: true } : {}), ...(value.case ? { case: value.case } : {}) } }
function replaceAt<T>(items: T[], index: number, item: T) { return items.map((value, position) => position === index ? item : value) }
function removeAt<T>(items: T[], index: number) { return items.filter((_, position) => position !== index) }
function toggle(values: string[], value: string, checked: boolean) { return checked ? [...new Set([...values, value])] : values.filter((item) => item !== value) }
function emptyDraft(): ReportDraft { return { id: 0, code: '', name: '', category: '', description: '', datasourceId: 0, status: 'DRAFT', lockVersion: 0, procedure: { owner: '', package: '', name: '', overload: '' }, result: { tableOwner: '', tableName: '', runIdColumn: 'RUN_ID', rowIdColumn: 'ROW_NO' }, callTemplate: '', parameters: [newParameter(0, true)], columns: [newColumn(0)], grants: [], createdAt: null, updatedAt: null } }
function newParameter(index: number, systemInjected = false): ReportParameter { return { code: systemInjected ? 'runId' : `param${index + 1}`, label: systemInjected ? '运行编号' : `参数 ${index + 1}`, displayOrder: index, controlType: 'TEXT', logicalType: 'string', cardinality: 'SINGLE', procedureArgName: systemInjected ? 'P_RUN_ID' : `P_PARAM_${index + 1}`, position: index + 1, oracleType: 'VARCHAR2', precision: null, scale: null, maxLength: 4000, required: true, nullable: false, systemInjected, sensitive: false, defaultValue: undefined, allowedValues: [], validation: {}, normalizer: {}, valueSource: {}, timezone: 'Asia/Shanghai', nullPolicy: 'TYPED_NULL', errorMessage: '', collectionEncoding: '' } }
function newColumn(index: number): ReportColumn { return { fieldId: crypto.randomUUID(), logicalCode: `field${index + 1}`, databaseColumn: `FIELD_${index + 1}`, sourceOracleType: 'VARCHAR2', precision: null, scale: null, nullable: true, valueType: 'string', previewHeader: `字段 ${index + 1}`, excelHeader: `字段 ${index + 1}`, displayOrder: index, exportOrder: index, previewVisible: true, exportVisible: true, filterable: false, sortable: false, exportAllowed: true, allowedOperators: undefined, format: undefined, dictionaryVersion: undefined, maskingPolicy: undefined, excelWidth: 16, nullDisplay: '-' } }
function nextOrder(values: number[]) { return values.length ? Math.max(...values) + 1 : 0 }
function draftFingerprint(draft: ReportDraft) { return JSON.stringify(draft) }
