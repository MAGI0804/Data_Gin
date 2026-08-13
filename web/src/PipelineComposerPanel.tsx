import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ClientResponse } from './api/client'
import {
  parsePipelineDetail,
  parsePipelinePreview,
  parsePipelineWriteResult,
  parseStageWriteResult,
  parseStepWriteResult,
  parseStageGeneratedConfig,
  parseStepConfigList,
  isMaskedMethodParam,
  pipelinePath,
  pipelineStagePath,
  pipelineStepPath,
  stageConfigPath,
  stageMethodTypes,
  stageStepPath,
  type MethodOutput,
  type MethodParam,
  type PipelineDetail,
  type PipelineStageDetail,
  type PipelineSummary,
  type PipelineStepDetail,
  type StageType,
} from './pipelineComposer'
import styles from './PipelineComposerPanel.module.css'

type ComposerClient = (path: string, options: { method: 'GET' | 'POST' | 'PUT'; body?: unknown; signal?: AbortSignal; showResult: false; silentLoading: true }) => Promise<ClientResponse>
type PipelineDraft = { id: number | null; name: string; code: string; description: string; enabled: boolean }
type StageDraft = { id: number | null; stageType: StageType; name: string; orderIndex: string; enabled: boolean }
type StepDraft = { id: number | null; stageID: number; code: string; name: string; methodType: string; orderIndex: string; timeoutSeconds: string; enabled: boolean; paramsJSON: string; outputsJSON: string; hasMaskedSecret: boolean }

const emptyParams = '[]'
const emptyOutputs = '[]'

function emptyPipelineDraft(): PipelineDraft {
  return { id: null, name: '', code: '', description: '', enabled: true }
}

function stageDraftFrom(stage: PipelineStageDetail['stage']): StageDraft {
  return { id: stage.id, stageType: stage.stage_type, name: stage.name, orderIndex: String(stage.order_index), enabled: stage.enabled }
}

function stepDraftFrom(detail: PipelineStepDetail): StepDraft {
  return {
    id: detail.step.id,
    stageID: detail.step.stage_id,
    code: detail.step.code,
    name: detail.step.name,
    methodType: detail.step.method_type,
    orderIndex: String(detail.step.order_index),
    timeoutSeconds: String(detail.step.timeout_seconds),
    enabled: detail.step.enabled,
    paramsJSON: JSON.stringify(detail.params, null, 2),
    outputsJSON: JSON.stringify(detail.outputs, null, 2),
    hasMaskedSecret: detail.params.some(isMaskedMethodParam),
  }
}

function emptyStepDraft(stage: PipelineStageDetail['stage']): StepDraft {
  const methodType = stageMethodTypes(stage.stage_type)[0]
  return { id: null, stageID: stage.id, code: '', name: '', methodType, orderIndex: '0', timeoutSeconds: '30', enabled: true, paramsJSON: emptyParams, outputsJSON: emptyOutputs, hasMaskedSecret: false }
}

function safeMessage(status: number, operation: string) {
  if (status === 0) return '网络连接未完成，已保留当前编辑内容。'
  if (status === 401) return '登录已失效，请重新登录。'
  if (status === 403) return '当前账号没有执行此操作的权限。'
  if (status === 404) return '目标已不存在，请刷新后重试。'
  if (status === 409) return '数据已变化，请刷新后确认。'
  if (status >= 500) return `${operation}未完成，请稍后刷新确认状态。`
  return `${operation}未完成，请检查输入后重试。`
}

function integerField(value: string, minimum: number, fallback: number | null = null) {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed >= minimum ? parsed : fallback
}

function StageTypeLabel({ value }: { value: StageType }) {
  const labels: Record<StageType, string> = { fetch: '获取', process: '处理', push: '推送', log: '日志' }
  return <span className={styles.stageKind}>{labels[value]}</span>
}

export function PipelineComposerPanel({ pipelines, client, canManage = true, onBusyChange, onRefresh }: { pipelines: PipelineSummary[]; client: ComposerClient; canManage?: boolean; onBusyChange?: (busy: boolean) => void; onRefresh: () => void }) {
  const [selectedID, setSelectedID] = useState<number | null>(null)
  const [detail, setDetail] = useState<PipelineDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')
  const [pipelineDraft, setPipelineDraft] = useState<PipelineDraft | null>(null)
  const [stageDraft, setStageDraft] = useState<StageDraft | null>(null)
  const [stepDraft, setStepDraft] = useState<StepDraft | null>(null)
  const [preview, setPreview] = useState<Record<string, unknown> | null>(null)
  const [publishingStageID, setPublishingStageID] = useState<number | null>(null)
  const [publishAcknowledged, setPublishAcknowledged] = useState(false)
  const detailControllerRef = useRef<AbortController | null>(null)
  const previewControllerRef = useRef<AbortController | null>(null)
  const activePipelineIDRef = useRef<number | null>(null)
  const writeInFlightRef = useRef(false)
  const onBusyChangeRef = useRef(onBusyChange)

  onBusyChangeRef.current = onBusyChange
  useEffect(() => () => onBusyChangeRef.current?.(false), [])

  function beginWrite() {
    writeInFlightRef.current = true
    onBusyChangeRef.current?.(true)
    setSaving(true)
  }

  function finishWrite() {
    writeInFlightRef.current = false
    onBusyChangeRef.current?.(false)
    setSaving(false)
  }

  const selected = useMemo(() => pipelines.find((pipeline) => pipeline.id === selectedID) ?? pipelines[0] ?? null, [pipelines, selectedID])
  const activeDetail = detail?.pipeline.id === selected?.id ? detail : null
  const availableStageTypes = useMemo(() => activeDetail ? (['fetch', 'process', 'push', 'log'] as StageType[]).filter((kind) => !activeDetail.stages.some((stage) => stage.stage.stage_type === kind)) : [], [activeDetail])

  const loadDetail = useCallback(async (pipelineID: number) => {
    detailControllerRef.current?.abort()
    const controller = new AbortController()
    detailControllerRef.current = controller
    setLoading(true)
    try {
      const response = await client(pipelinePath(pipelineID), { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
      if (controller.signal.aborted || activePipelineIDRef.current !== pipelineID) return
      const next = response.ok ? parsePipelineDetail(response.data) : null
      if (next?.pipeline.id === pipelineID) setDetail(next)
      else setMessage(response.ok ? '流水线详情格式不正确，请刷新后重试。' : safeMessage(response.status, '加载流水线详情'))
    } catch {
      if (!controller.signal.aborted && activePipelineIDRef.current === pipelineID) setMessage('加载流水线详情未完成，请稍后重试。')
    } finally {
      if (!controller.signal.aborted && activePipelineIDRef.current === pipelineID) setLoading(false)
    }
  }, [client])

  useEffect(() => () => { detailControllerRef.current?.abort(); previewControllerRef.current?.abort() }, [])
  useEffect(() => {
    previewControllerRef.current?.abort()
    setPreview(null)
    setPipelineDraft(null)
    setStageDraft(null)
    setStepDraft(null)
    setPublishingStageID(null)
    setPublishAcknowledged(false)
    if (selected) {
      activePipelineIDRef.current = selected.id
      setDetail(null)
      void loadDetail(selected.id)
    } else {
      activePipelineIDRef.current = null
      setDetail(null)
    }
  }, [selected, loadDetail])

  function refreshAfterWrite(pipelineID: number) {
    setPreview(null)
    onRefresh()
    void loadDetail(pipelineID)
  }

  async function savePipeline(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canManage || !pipelineDraft || saving || writeInFlightRef.current) return
    if (pipelineDraft.id !== null && pipelineDraft.id !== selected?.id) { setMessage('当前流水线已变化，请重新打开编辑。'); return }
    const name = pipelineDraft.name.trim()
    const code = pipelineDraft.code.trim()
    if (!name || !code || name.length > 100 || code.length > 100) { setMessage('请填写不超过 100 个字符的流水线名称和编码。'); return }
    beginWrite()
    try {
      const response = await client(pipelineDraft.id ? pipelinePath(pipelineDraft.id) : '/v1/pipelines', { method: pipelineDraft.id ? 'PUT' : 'POST', showResult: false, silentLoading: true, body: { name, code, description: pipelineDraft.description.trim(), enabled: pipelineDraft.enabled } })
      const saved = response.ok ? parsePipelineWriteResult(response.data) : null
      if (!saved || (pipelineDraft.id !== null && saved.id !== pipelineDraft.id)) { setMessage(response.ok ? '流水线保存响应格式不正确，请刷新确认。' : safeMessage(response.status, '保存流水线')); return }
      setPipelineDraft(null); setMessage('流水线已保存。'); onRefresh(); setSelectedID(saved.id)
    } catch { setMessage('保存流水线未完成，已保留当前编辑内容。') } finally { finishWrite() }
  }

  async function saveStage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canManage || !detail || detail.pipeline.id !== selected?.id || !stageDraft || saving || writeInFlightRef.current) return
    const orderIndex = integerField(stageDraft.orderIndex, 0)
    if (!stageDraft.name.trim() || orderIndex === null) { setMessage('请填写阶段名称和非负顺序。'); return }
    const pipelineID = detail.pipeline.id
    beginWrite()
    try {
      const response = await client(stageDraft.id ? pipelineStagePath(stageDraft.id) : `${pipelinePath(pipelineID)}/stages`, { method: stageDraft.id ? 'PUT' : 'POST', showResult: false, silentLoading: true, body: { stage_type: stageDraft.stageType, name: stageDraft.name.trim(), order_index: orderIndex, enabled: stageDraft.enabled } })
      const saved = response.ok ? parseStageWriteResult(response.data) : null
      if (!saved || saved.pipeline_id !== pipelineID || (stageDraft.id !== null && saved.id !== stageDraft.id)) { setMessage(response.ok ? '阶段保存响应格式不正确，请刷新确认。' : safeMessage(response.status, '保存阶段')); return }
      setStageDraft(null); setMessage('阶段已保存。'); refreshAfterWrite(pipelineID)
    } catch { setMessage('保存阶段未完成，已保留当前编辑内容。') } finally { finishWrite() }
  }

  async function saveStep(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canManage || !detail || detail.pipeline.id !== selected?.id || !stepDraft || saving || writeInFlightRef.current || stepDraft.hasMaskedSecret) return
    const stage = detail.stages.find((item) => item.stage.id === stepDraft.stageID)
    const orderIndex = integerField(stepDraft.orderIndex, 0)
    const timeoutSeconds = integerField(stepDraft.timeoutSeconds, 1)
    const params = parseStepConfigList(stepDraft.paramsJSON, 'params')
    const outputs = parseStepConfigList(stepDraft.outputsJSON, 'outputs')
    if (!stage || !stepDraft.name.trim() || !stepDraft.code.trim() || !stageMethodTypes(stage.stage.stage_type).includes(stepDraft.methodType) || orderIndex === null || timeoutSeconds === null || timeoutSeconds > 86_400 || params === null || outputs === null) {
      setMessage('请填写步骤字段；参数和输出必须是符合接口结构的 JSON 数组。')
      return
    }
    const pipelineID = detail.pipeline.id
    beginWrite()
    const path = stepDraft.id ? pipelineStepPath(detail.pipeline.id, stepDraft.id) : stageStepPath(stage.stage.id)
    try {
      const response = await client(path, { method: stepDraft.id ? 'PUT' : 'POST', showResult: false, silentLoading: true, body: { stage_id: stage.stage.id, code: stepDraft.code.trim(), name: stepDraft.name.trim(), method_type: stepDraft.methodType, order_index: orderIndex, enabled: stepDraft.enabled, timeout_seconds: timeoutSeconds, params: params as MethodParam[], outputs: outputs as MethodOutput[] } })
      const saved = response.ok ? parseStepWriteResult(response.data) : null
      if (!saved || saved.step.pipeline_id !== pipelineID || saved.step.stage_id !== stage.stage.id || (stepDraft.id !== null && saved.step.id !== stepDraft.id)) { setMessage(response.ok ? '步骤保存响应格式不正确，请刷新确认。' : safeMessage(response.status, '保存步骤')); return }
      setStepDraft(null); setMessage('步骤已保存。'); refreshAfterWrite(pipelineID)
    } catch { setMessage('保存步骤未完成，已保留当前编辑内容。') } finally { finishWrite() }
  }

  async function loadPreview() {
    if (!detail || detail.pipeline.id !== selected?.id || loading) return
    previewControllerRef.current?.abort()
    const controller = new AbortController()
    const pipelineID = detail.pipeline.id
    previewControllerRef.current = controller
    setPreview(null)
    try {
      const response = await client(`${pipelinePath(pipelineID)}/preview-json`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
      if (controller.signal.aborted || activePipelineIDRef.current !== pipelineID) return
      const next = response.ok ? parsePipelinePreview(response.data) : null
      if (next) setPreview(next)
      else setMessage(response.ok ? '预览内容格式不正确，请刷新后重试。' : safeMessage(response.status, '生成预览'))
    } catch {
      if (!controller.signal.aborted && activePipelineIDRef.current === pipelineID) setMessage('生成预览未完成，请稍后重试。')
    }
  }

  async function generateStageConfig(stage: PipelineStageDetail) {
    if (!canManage || !detail || detail.pipeline.id !== selected?.id || stage.stage.pipeline_id !== detail.pipeline.id || saving || writeInFlightRef.current) return
    const pipelineID = detail.pipeline.id
    beginWrite()
    try {
      const response = await client(stageConfigPath(stage.stage.id, 'generate-config'), { method: 'POST', showResult: false, silentLoading: true })
      const config = response.ok ? parseStageGeneratedConfig(response.data) : null
      if (!config || config.pipeline_id !== pipelineID || config.stage_id !== stage.stage.id) { setMessage(response.ok ? '阶段配置响应归属不正确，请刷新确认。' : safeMessage(response.status, '生成阶段配置')); return }
      setMessage(`已生成“${stage.stage.name}”的阶段配置，尚未发布。`); refreshAfterWrite(pipelineID)
    } catch { setMessage('生成阶段配置未完成，请稍后重试。') } finally { finishWrite() }
  }

  async function publishStageConfig(stage: PipelineStageDetail) {
    if (!canManage || !detail || detail.pipeline.id !== selected?.id || stage.stage.pipeline_id !== detail.pipeline.id || saving || writeInFlightRef.current || publishingStageID !== stage.stage.id || !publishAcknowledged) return
    const pipelineID = detail.pipeline.id
    beginWrite()
    try {
      const response = await client(stageConfigPath(stage.stage.id, 'publish-config'), { method: 'POST', showResult: false, silentLoading: true })
      const config = response.ok ? parseStageGeneratedConfig(response.data) : null
      if (!config || config.pipeline_id !== pipelineID || config.stage_id !== stage.stage.id) { setMessage(response.ok ? '发布响应归属不正确，请刷新确认。' : safeMessage(response.status, '发布阶段配置')); return }
      setPublishingStageID(null); setPublishAcknowledged(false)
      setMessage(config.target_ref_id > 0 ? `阶段已发布，已关联 ${config.target_ref_type} #${config.target_ref_id}。` : '日志阶段配置已发布。'); refreshAfterWrite(pipelineID)
    } catch { setMessage('发布阶段配置未完成，请刷新确认状态。') } finally { finishWrite() }
  }

  return <section className={styles.composer} aria-busy={loading || saving}>
    <div className={styles.head}>
      <div><strong>流水线编排</strong><span>创建、编辑、预览、生成与发布均使用真实接口；发布会创建或关联真实配置资源。</span></div>
      <div className={styles.actions}>{canManage ? <button type="button" onClick={() => { setMessage(''); setPipelineDraft(emptyPipelineDraft()) }} disabled={saving}>新增流水线</button> : null}{activeDetail && <>{canManage ? <button type="button" onClick={() => setPipelineDraft({ id: activeDetail.pipeline.id, name: activeDetail.pipeline.name, code: activeDetail.pipeline.code, description: activeDetail.pipeline.description ?? '', enabled: activeDetail.pipeline.enabled })} disabled={saving}>编辑流水线</button> : null}<button type="button" onClick={() => void loadPreview()} disabled={saving || loading}>预览 JSON</button></>}</div>
    </div>
    {message && <p className={styles.message} role="status">{message}</p>}
    {pipelines.length === 0 ? <p role="status">尚无流水线{canManage ? '，请先创建' : ''}。</p> : <div className={styles.layout}>
      <label>选择流水线<select value={selected?.id ?? ''} onChange={(event) => setSelectedID(Number(event.currentTarget.value))} disabled={saving || Boolean(pipelineDraft || stageDraft || stepDraft || publishingStageID)}>{pipelines.map((pipeline) => <option key={pipeline.id} value={pipeline.id}>{pipeline.name} · {pipeline.code}{pipeline.enabled ? '' : '（停用）'}</option>)}</select></label>
      <div className={styles.workspace}>
        {loading && !activeDetail ? <p role="status">正在加载流水线详情…</p> : activeDetail && <>
          <div className={styles.detailTitle}><strong>{activeDetail.pipeline.name}</strong><span>{activeDetail.pipeline.description || '未填写说明'} / {activeDetail.pipeline.enabled ? '已启用' : '已停用'}</span>{canManage && availableStageTypes.length > 0 ? <button type="button" onClick={() => setStageDraft({ id: null, stageType: availableStageTypes[0], name: '', orderIndex: String(activeDetail.stages.length + 1), enabled: true })} disabled={saving}>补充阶段</button> : null}</div>
          {activeDetail.stages.length === 0 ? <p role="status">暂无阶段。</p> : activeDetail.stages.map((stage) => <article className={styles.stage} key={stage.stage.id}>
            <div className={styles.stageHead}><div><StageTypeLabel value={stage.stage.stage_type} /><strong>{stage.stage.name}</strong><span>顺序 {stage.stage.order_index} / {stage.stage.enabled ? '启用' : '停用'} / {stage.steps.length} 个步骤</span></div>{canManage ? <div className={styles.actions}><button type="button" onClick={() => setStageDraft(stageDraftFrom(stage.stage))} disabled={saving}>编辑</button><button type="button" onClick={() => setStepDraft(emptyStepDraft(stage.stage))} disabled={saving}>新增步骤</button><button type="button" onClick={() => void generateStageConfig(stage)} disabled={saving}>生成配置</button></div> : null}</div>
            {stage.generated_config && <div className={styles.generated}><span>配置版本 {stage.generated_config.version} / {stage.generated_config.target_ref_id > 0 ? `已发布至 ${stage.generated_config.target_ref_type} #${stage.generated_config.target_ref_id}` : '未发布'}</span>{canManage ? publishingStageID === stage.stage.id ? <label className={styles.publishConfirm}><input type="checkbox" checked={publishAcknowledged} onChange={(event) => setPublishAcknowledged(event.currentTarget.checked)} />我确认发布会创建或关联真实资源。<button className={styles.danger} type="button" onClick={() => void publishStageConfig(stage)} disabled={saving || !publishAcknowledged}>确认发布</button><button type="button" onClick={() => { setPublishingStageID(null); setPublishAcknowledged(false) }} disabled={saving}>取消</button></label> : <button className={styles.danger} type="button" onClick={() => { setPublishingStageID(stage.stage.id); setPublishAcknowledged(false) }} disabled={saving || stage.generated_config.target_ref_id > 0}>{stage.generated_config.target_ref_id > 0 ? '已发布' : '发布配置'}</button> : null}</div>}
            {stage.steps.length === 0 ? <p className={styles.emptyStep}>暂无步骤。</p> : <div className={styles.stepList}>{stage.steps.map((item) => <div className={styles.step} key={item.step.id}><div><strong>{item.step.name}</strong><span>{item.step.code} · {item.step.method_type} · 超时 {item.step.timeout_seconds} 秒 · {item.step.enabled ? '启用' : '停用'}</span></div>{canManage ? <button type="button" onClick={() => setStepDraft(stepDraftFrom(item))} disabled={saving}>编辑步骤</button> : null}</div>)}</div>}
          </article>)}
        </>}
      </div>
    </div>}
    {pipelineDraft && <form className={styles.editor} onSubmit={savePipeline}><h4>{pipelineDraft.id ? '编辑流水线' : '新增流水线'}</h4><label>名称<input value={pipelineDraft.name} required maxLength={100} onChange={(event) => setPipelineDraft({ ...pipelineDraft, name: event.currentTarget.value })} /></label><label>编码<input value={pipelineDraft.code} required maxLength={100} onChange={(event) => setPipelineDraft({ ...pipelineDraft, code: event.currentTarget.value })} /></label><label>说明<textarea rows={3} value={pipelineDraft.description} onChange={(event) => setPipelineDraft({ ...pipelineDraft, description: event.currentTarget.value })} /></label><label className={styles.checkbox}><input type="checkbox" checked={pipelineDraft.enabled} onChange={(event) => setPipelineDraft({ ...pipelineDraft, enabled: event.currentTarget.checked })} />启用流水线</label><div className={styles.actions}><button type="button" onClick={() => setPipelineDraft(null)} disabled={saving}>取消</button><button className={styles.primary} type="submit" disabled={saving}>{saving ? '保存中…' : '保存'}</button></div></form>}
    {stageDraft && <form className={styles.editor} onSubmit={saveStage}><h4>{stageDraft.id ? '编辑阶段' : '补充阶段'}</h4><label>阶段类型<select value={stageDraft.stageType} onChange={(event) => setStageDraft({ ...stageDraft, stageType: event.currentTarget.value as StageType })} disabled={Boolean(stageDraft.id)}>{(stageDraft.id ? [stageDraft.stageType] : availableStageTypes).map((kind) => <option key={kind} value={kind}>{kind}</option>)}</select></label><label>名称<input value={stageDraft.name} required maxLength={100} onChange={(event) => setStageDraft({ ...stageDraft, name: event.currentTarget.value })} /></label><label>顺序<input type="number" min="0" value={stageDraft.orderIndex} onChange={(event) => setStageDraft({ ...stageDraft, orderIndex: event.currentTarget.value })} /></label><label className={styles.checkbox}><input type="checkbox" checked={stageDraft.enabled} onChange={(event) => setStageDraft({ ...stageDraft, enabled: event.currentTarget.checked })} />启用阶段</label><div className={styles.actions}><button type="button" onClick={() => setStageDraft(null)} disabled={saving}>取消</button><button className={styles.primary} type="submit" disabled={saving}>{saving ? '保存中…' : '保存'}</button></div></form>}
    {stepDraft && <form className={styles.editor} onSubmit={saveStep}><h4>{stepDraft.id ? '编辑步骤' : '新增步骤'}</h4>{stepDraft.hasMaskedSecret && <p className={styles.error} role="alert">该步骤含有已隐藏的密钥；为防止保存时覆盖真实值，当前不允许编辑。请在后端提供保留密钥的更新契约后再操作。</p>}<label>编码<input value={stepDraft.code} required maxLength={100} onChange={(event) => setStepDraft({ ...stepDraft, code: event.currentTarget.value })} /></label><label>名称<input value={stepDraft.name} required maxLength={100} onChange={(event) => setStepDraft({ ...stepDraft, name: event.currentTarget.value })} /></label><label>方法类型<select value={stepDraft.methodType} onChange={(event) => setStepDraft({ ...stepDraft, methodType: event.currentTarget.value })}>{activeDetail?.stages.find((stage) => stage.stage.id === stepDraft.stageID) && stageMethodTypes(activeDetail.stages.find((stage) => stage.stage.id === stepDraft.stageID)!.stage.stage_type).map((kind) => <option key={kind} value={kind}>{kind}</option>)}</select></label><label>顺序<input type="number" min="0" value={stepDraft.orderIndex} onChange={(event) => setStepDraft({ ...stepDraft, orderIndex: event.currentTarget.value })} /></label><label>超时秒数<input type="number" min="1" max="86400" value={stepDraft.timeoutSeconds} onChange={(event) => setStepDraft({ ...stepDraft, timeoutSeconds: event.currentTarget.value })} /></label><label className={styles.checkbox}><input type="checkbox" checked={stepDraft.enabled} onChange={(event) => setStepDraft({ ...stepDraft, enabled: event.currentTarget.checked })} />启用步骤</label><label>参数 JSON<textarea rows={8} value={stepDraft.paramsJSON} onChange={(event) => setStepDraft({ ...stepDraft, paramsJSON: event.currentTarget.value })} /></label><label>输出 JSON<textarea rows={8} value={stepDraft.outputsJSON} onChange={(event) => setStepDraft({ ...stepDraft, outputsJSON: event.currentTarget.value })} /></label><p className={styles.contractNote}>参数需要 location、name、value_source；输出需要 name。秘密参数只在新建时填写，且不会从服务端回显。</p><div className={styles.actions}><button type="button" onClick={() => setStepDraft(null)} disabled={saving}>取消</button><button className={styles.primary} type="submit" disabled={saving || stepDraft.hasMaskedSecret}>{saving ? '保存中…' : '保存'}</button></div></form>}
    {preview && <pre className={styles.preview} aria-label="流水线 JSON 预览">{JSON.stringify(preview, null, 2)}</pre>}
  </section>
}
