import { FormEvent, ReactNode, useCallback, useEffect, useMemo, useState } from 'react'
import {
  Activity,
  Database,
  FileJson,
  LogOut,
  Play,
  Plus,
  RefreshCcw,
  Send,
  Settings,
  ShieldCheck,
} from 'lucide-react'
import './App.css'

type ApiResult = {
  ok: boolean
  status: number
  data: unknown
}

type ApiClientOptions = {
  method?: 'GET' | 'POST' | 'PUT'
  body?: unknown
  showResult?: boolean
  silentLoading?: boolean
}

type ApiClient = (path: string, options?: ApiClientOptions) => Promise<ApiResult>
type StageType = 'fetch' | 'process' | 'push' | 'log'
type MethodType = 'request' | 'extract' | 'mapping' | 'validate' | 'db_query' | 'db_write' | 'template' | 'delivery' | 'log'
type ResultRecord = Record<string, unknown>

type PipelineDefinition = {
  id: number
  name: string
  code: string
  description: string
  enabled: boolean
  created_at: number
}

type PipelineStage = {
  id: number
  pipeline_id: number
  stage_type: StageType
  name: string
  order_index: number
  enabled: boolean
}

type StageGeneratedConfig = {
  id: number
  stage_id: number
  stage_type: StageType
  generated_config_json: string
  target_ref_type: string
  target_ref_id: number
  version: number
}

type MethodParam = {
  id?: number
  location: string
  name: string
  value_source: string
  value: string
  value_type: string
  required: boolean
  secret: boolean
  description: string
  order_index: number
}

type MethodOutput = {
  id?: number
  name: string
  source_path: string
  value_type: string
  required: boolean
  description: string
  order_index: number
}

type MethodStep = {
  id: number
  pipeline_id: number
  stage_id: number
  code: string
  name: string
  method_type: MethodType
  order_index: number
  enabled: boolean
  timeout_seconds: number
  generated_config_json: string
}

type MethodStepDetail = {
  step: MethodStep
  params: MethodParam[]
  outputs: MethodOutput[]
}

type PipelineStageDetail = {
  stage: PipelineStage
  steps: MethodStepDetail[]
  generated_config?: StageGeneratedConfig | null
}

type PipelineDetail = {
  pipeline: PipelineDefinition
  stages: PipelineStageDetail[]
  steps: MethodStepDetail[]
}

type PipelineRun = {
  id: number
  trace_id: string
  run_type: string
  trigger_type: string
  status: string
  total_count: number
  success_count: number
  failed_count: number
  started_at: string | null
  finished_at: string | null
}

type StepRun = {
  id: number
  run_id: number
  step_code: string
  method_type: string
  status: string
  input_json: string
  output_json: string
  error_message: string
}

type SourceDefinition = {
  id: number
  name: string
  code: string
  source_type: string
  enabled: boolean
}

type DestinationDefinition = {
  id: number
  name: string
  code: string
  destination_type: string
  enabled: boolean
}

const tokenStorageKey = 'warehouse-token'
const stageOrder: StageType[] = ['fetch', 'process', 'push', 'log']
const methodLibrary: Array<{ type: MethodType; label: string; stage: StageType; description: string }> = [
  { type: 'request', label: 'Request 请求', stage: 'fetch', description: '配置 URL、Query、Header、Body 和响应捕获' },
  { type: 'extract', label: 'Extract 提取', stage: 'fetch', description: '从响应中提取 records 或业务字段' },
  { type: 'mapping', label: 'Mapping 清洗', stage: 'process', description: '字段映射、类型转换、业务主键' },
  { type: 'validate', label: 'Validate 校验', stage: 'process', description: '必填、枚举、数据质量规则' },
  { type: 'template', label: 'Template 模板', stage: 'push', description: '把清洗数据渲染成推送报文' },
  { type: 'delivery', label: 'Delivery 推送', stage: 'push', description: 'HTTP/SOAP 推送和响应记录' },
  { type: 'log', label: 'Log 记录', stage: 'log', description: '记录运行、输入、输出、错误和耗时' },
]

const samplePipelineSteps: Array<{
  stage_type: StageType
  code: string
  name: string
  method_type: MethodType
  order_index: number
  params: MethodParam[]
  outputs: MethodOutput[]
}> = [
  {
    stage_type: 'fetch',
    code: 'get_token',
    name: '获取有赞 Token',
    method_type: 'request',
    order_index: 1,
    params: [
      param('url', 'url', 'config', 'cfg.youzan.token_url', 'string', true, false, 'Token URL'),
      param('request', 'method', 'static', 'POST', 'string', true, false, 'HTTP 方法'),
      param('header', 'Content-Type', 'static', 'application/json', 'string', true, false, '请求内容类型'),
      param('body', 'client_id', 'config', 'cfg.youzan.client_id', 'string', true, true, '客户端 ID'),
      param('body', 'client_secret', 'config', 'cfg.youzan.client_secret', 'string', true, true, '客户端密钥'),
      param('body', 'grant_id', 'config', 'cfg.youzan.grant_id', 'string', true, false, '授权主体'),
    ],
    outputs: [output('access_token', 'data.access_token', 'string', true, '授权令牌')],
  },
  {
    stage_type: 'fetch',
    code: 'fetch_orders',
    name: '拉取有赞订单',
    method_type: 'request',
    order_index: 2,
    params: [
      param('url', 'url', 'config', 'cfg.youzan.orders_url', 'string', true, false, '订单接口地址'),
      param('request', 'method', 'static', 'POST', 'string', true, false, 'HTTP 方法'),
      param('query', 'access_token', 'binding', 'steps.get_token.outputs.access_token', 'string', true, true, '上一步 token'),
      param('header', 'Content-Type', 'static', 'application/json', 'string', true, false, '请求内容类型'),
      param('body', 'page_size', 'static', '100', 'int', true, false, '分页大小'),
      param('response', 'records_path', 'static', 'data.full_order_info_list', 'string', true, false, '订单列表路径'),
    ],
    outputs: [output('records', 'data.full_order_info_list', 'array', true, '订单列表')],
  },
  {
    stage_type: 'process',
    code: 'map_order',
    name: '清洗订单字段',
    method_type: 'mapping',
    order_index: 1,
    params: [
      param('mapping', 'table_name', 'static', 'clean_orders', 'string', true, false, '清洗表'),
      param('mapping', 'business_key_field', 'static', 'order_no', 'string', true, false, '业务主键'),
      param('field', 'order_no', 'static', '$.order_info.tid', 'string', true, false, '订单号'),
      param('field', 'actual_amount', 'static', '$.pay_info.payment', 'decimal', false, false, '实付金额'),
    ],
    outputs: [output('record', 'record', 'object', true, '清洗记录')],
  },
  {
    stage_type: 'push',
    code: 'push_sales',
    name: '推送销售数据',
    method_type: 'delivery',
    order_index: 1,
    params: [
      param('url', 'url', 'config', 'cfg.henglong.sales_url', 'string', true, false, '推送地址'),
      param('request', 'method', 'static', 'POST', 'string', true, false, 'HTTP 方法'),
      param('header', 'Content-Type', 'static', 'application/json', 'string', true, false, '请求内容类型'),
      param('body', 'order_no', 'binding', 'steps.map_order.outputs.record', 'string', true, false, '清洗记录'),
    ],
    outputs: [output('http_status', 'http_status', 'int', true, 'HTTP 状态码')],
  },
  {
    stage_type: 'log',
    code: 'record_step_run',
    name: '记录步骤日志',
    method_type: 'log',
    order_index: 1,
    params: [param('runtime', 'trace_id', 'binding', 'steps.push_sales.outputs.http_status', 'string', false, false, '运行上下文')],
    outputs: [output('logged', 'success', 'bool', false, '日志写入结果')],
  },
]

function App() {
  const [authenticated, setAuthenticated] = useState(() => Boolean(sessionStorage.getItem(tokenStorageKey)))
  const [token, setToken] = useState(() => sessionStorage.getItem(tokenStorageKey) ?? '')
  const [loading, setLoading] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [result, setResult] = useState<ApiResult | null>(null)
  const [pipelines, setPipelines] = useState<PipelineDefinition[]>([])
  const [selectedPipelineId, setSelectedPipelineId] = useState<number | null>(null)
  const [pipelineDetail, setPipelineDetail] = useState<PipelineDetail | null>(null)
  const [selectedStageId, setSelectedStageId] = useState<number | null>(null)
  const [selectedStepId, setSelectedStepId] = useState<number | 'new'>('new')
  const [preview, setPreview] = useState<unknown>(null)
  const [runs, setRuns] = useState<PipelineRun[]>([])
  const [stepRuns, setStepRuns] = useState<StepRun[]>([])
  const [sources, setSources] = useState<SourceDefinition[]>([])
  const [destinations, setDestinations] = useState<DestinationDefinition[]>([])

  const client = useCallback<ApiClient>(
    async (path, options = {}) => {
      const method = options.method ?? 'POST'
      if (!options.silentLoading) setLoading(true)
      try {
        const response = await fetch(`/api${path}`, {
          method,
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { token } : {}),
          },
          body: method === 'GET' || options.body === undefined ? undefined : JSON.stringify(options.body),
        })
        const data = await response.json().catch(() => ({}))
        const nextResult = { ok: response.ok && isSuccessPayload(data), status: response.status, data }
        if (options.showResult !== false) setResult(nextResult)
        return nextResult
      } catch (error) {
        const nextResult = { ok: false, status: 0, data: error instanceof Error ? error.message : String(error) }
        if (options.showResult !== false) setResult(nextResult)
        return nextResult
      } finally {
        if (!options.silentLoading) setLoading(false)
      }
    },
    [token],
  )

  const refreshAll = useCallback(
    async (showResult = false) => {
      if (!token) return
      setRefreshing(true)
      try {
        const [pipelineResult, runResult, sourceResult, destinationResult] = await Promise.all([
          client('/v1/pipelines', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/runs?limit=50', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/sources', { method: 'GET', showResult: false, silentLoading: true }),
          client('/v1/destinations', { method: 'GET', showResult: false, silentLoading: true }),
        ])
        if (pipelineResult.ok) {
          const nextPipelines = readList<PipelineDefinition>(pipelineResult, 'pipelines')
          setPipelines(nextPipelines)
          if (!selectedPipelineId && nextPipelines.length > 0) setSelectedPipelineId(nextPipelines[0].id)
        }
        if (runResult.ok) setRuns(readList<PipelineRun>(runResult, 'runs'))
        if (sourceResult.ok) setSources(readList<SourceDefinition>(sourceResult, 'sources'))
        if (destinationResult.ok) setDestinations(readList<DestinationDefinition>(destinationResult, 'destinations'))
        if (showResult) setResult({ ok: pipelineResult.ok && runResult.ok, status: pipelineResult.status, data: pipelineResult.data })
      } finally {
        setRefreshing(false)
      }
    },
    [client, selectedPipelineId, token],
  )

  const refreshDetail = useCallback(
    async (pipelineId: number | null) => {
      if (!pipelineId) {
        setPipelineDetail(null)
        setPreview(null)
        return
      }
      const [detailResult, previewResult] = await Promise.all([
        client(`/v1/pipelines/${pipelineId}`, { method: 'GET', showResult: false, silentLoading: true }),
        client(`/v1/pipelines/${pipelineId}/preview-json`, { method: 'GET', showResult: false, silentLoading: true }),
      ])
      if (detailResult.ok) {
        const detail = readObject<PipelineDetail>(detailResult, 'pipeline')
        setPipelineDetail(detail)
        const firstStage = detail?.stages?.[0]?.stage.id ?? null
        if (!selectedStageId && firstStage) setSelectedStageId(firstStage)
      }
      if (previewResult.ok) setPreview(readObject<unknown>(previewResult, 'preview'))
    },
    [client, selectedStageId],
  )

  useEffect(() => {
    if (authenticated) void refreshAll(false)
  }, [authenticated, refreshAll])

  useEffect(() => {
    if (authenticated) void refreshDetail(selectedPipelineId)
  }, [authenticated, refreshDetail, selectedPipelineId])

  function handleLogin(nextToken: string) {
    sessionStorage.setItem(tokenStorageKey, nextToken)
    setToken(nextToken)
    setAuthenticated(true)
  }

  function handleLogout() {
    sessionStorage.removeItem(tokenStorageKey)
    setToken('')
    setAuthenticated(false)
    setPipelines([])
    setPipelineDetail(null)
    setPreview(null)
    setResult(null)
  }

  async function createPipeline(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const response = await client('/v1/pipelines', {
      body: {
        name: formValue(form, 'name'),
        code: formValue(form, 'code'),
        description: formValue(form, 'description'),
        enabled: true,
      },
    })
    if (response.ok) {
      const created = readObject<PipelineDefinition>(response, 'pipeline')
      await refreshAll(false)
      if (created?.id) setSelectedPipelineId(created.id)
      event.currentTarget.reset()
    }
  }

  async function saveStep(payload: StepEditorPayload) {
    if (!selectedPipelineId || !payload.stageId) return
    const path =
      payload.id === 'new'
        ? `/v1/pipeline-stages/${payload.stageId}/steps`
        : `/v1/pipeline-stages/${payload.stageId}/steps/${payload.id}`
    const response = await client(path, { method: payload.id === 'new' ? 'POST' : 'PUT', body: payload.body })
    if (response.ok) {
      await refreshDetail(selectedPipelineId)
      await refreshAll(false)
    }
  }

  async function generateStageConfig(stageId: number) {
    const response = await client(`/v1/pipeline-stages/${stageId}/generate-config`)
    if (response.ok) await refreshDetail(selectedPipelineId)
  }

  async function publishStageConfig(stageId: number) {
    const response = await client(`/v1/pipeline-stages/${stageId}/publish-config`)
    if (response.ok) await refreshDetail(selectedPipelineId)
  }

  async function runSelectedPipeline() {
    if (!selectedPipelineId) return
    const response = await client(`/v1/pipelines/${selectedPipelineId}/run`)
    if (response.ok) {
      await refreshAll(false)
      const resultObject = readObject<ResultRecord>(response, 'result')
      const runId = typeof resultObject?.run_id === 'number' ? resultObject.run_id : null
      if (runId) await loadStepRuns(runId)
    }
  }

  async function loadStepRuns(runId: number) {
    const response = await client(`/v1/pipeline-runs/${runId}/steps`, { method: 'GET' })
    if (response.ok) setStepRuns(readList<StepRun>(response, 'step_runs'))
  }

  async function createSamplePipeline() {
    const pipelineResponse = await client('/v1/pipelines', {
      body: {
        name: '有赞订单大块流水线',
        code: `youzan_order_${Date.now()}`,
        description: '数据获取 -> 数据处理 -> 数据推送 -> 日志记录',
        enabled: true,
      },
    })
    const pipeline = readObject<PipelineDefinition>(pipelineResponse, 'pipeline')
    if (!pipelineResponse.ok || !pipeline) return

    const stageResult = await client(`/v1/pipelines/${pipeline.id}/stages`, { method: 'GET', showResult: false })
    const stages = readList<PipelineStageDetail>(stageResult, 'stages')
    for (const step of samplePipelineSteps) {
      const stage = stages.find((item) => item.stage.stage_type === step.stage_type)
      if (!stage) continue
      await client(`/v1/pipeline-stages/${stage.stage.id}/steps`, {
        body: {
          stage_id: stage.stage.id,
          code: step.code,
          name: step.name,
          method_type: step.method_type,
          order_index: step.order_index,
          timeout_seconds: 30,
          enabled: true,
          params: step.params,
          outputs: step.outputs,
        },
        showResult: false,
      })
    }
    setSelectedPipelineId(pipeline.id)
    await refreshAll(false)
    await refreshDetail(pipeline.id)
  }

  const selectedStage = useMemo(() => {
    if (!pipelineDetail || !selectedStageId) return null
    return pipelineDetail.stages.find((item) => item.stage.id === selectedStageId) ?? null
  }, [pipelineDetail, selectedStageId])

  const selectedStep = useMemo(() => {
    if (!pipelineDetail || selectedStepId === 'new') return null
    return pipelineDetail.steps.find((item) => item.step.id === selectedStepId) ?? null
  }, [pipelineDetail, selectedStepId])

  if (!authenticated) return <LoginScreen onLogin={handleLogin} />

  return (
    <main className="pipeline-shell">
      <aside className="pipeline-sidebar" aria-label="流水线和基础方法块">
        <div className="brand">
          <Database aria-hidden="true" />
          <div>
            <h1>数据仓库</h1>
            <span>大块流水线工作台</span>
          </div>
        </div>
        <div className="sidebar-actions">
          <button type="button" onClick={() => refreshAll(true)} disabled={refreshing}>
            <RefreshCcw aria-hidden="true" />
            刷新
          </button>
          <button type="button" onClick={handleLogout}>
            <LogOut aria-hidden="true" />
            退出
          </button>
        </div>
        <PipelineList pipelines={pipelines} selectedId={selectedPipelineId} onSelect={setSelectedPipelineId} />
        <MethodLibrary selectedStageType={selectedStage?.stage.stage_type ?? 'fetch'} />
        <form className="compact-form" onSubmit={createPipeline}>
          <h2>新建流水线</h2>
          <Field label="名称" name="name" defaultValue="订单同步流水线" />
          <Field label="编码" name="code" defaultValue={`pipeline_${Date.now()}`} />
          <label>
            描述
            <textarea name="description" rows={3} defaultValue="由四个大块阶段和多个基础方法拼接" />
          </label>
          <button className="primary" type="submit" disabled={loading}>
            <Plus aria-hidden="true" />
            创建
          </button>
          <button type="button" onClick={createSamplePipeline} disabled={loading}>
            <FileJson aria-hidden="true" />
            创建有赞样例
          </button>
        </form>
      </aside>

      <section className="pipeline-workspace">
        <header className="workspace-header">
          <div>
            <p className="eyebrow">staged method pipeline</p>
            <h2>{pipelineDetail?.pipeline.name ?? '选择或创建流水线'}</h2>
          </div>
          <div className="header-actions">
            <button type="button" onClick={() => setSelectedStepId('new')} disabled={!selectedStage}>
              <Plus aria-hidden="true" />
              新增基础方法
            </button>
            <button className="primary" type="button" onClick={runSelectedPipeline} disabled={!selectedPipelineId || loading}>
              <Play aria-hidden="true" />
              执行流水线
            </button>
          </div>
        </header>

        <section className="overview-grid">
          <Metric label="流水线" value={pipelines.length} />
          <Metric label="大块阶段" value={pipelineDetail?.stages.length ?? 0} />
          <Metric label="基础方法" value={pipelineDetail?.steps.length ?? 0} />
          <Metric label="旧配置出口" value={`${sources.length}/${destinations.length}`} />
        </section>

        <div className="stage-workbench">
          <section className="workbench-panel stage-board-panel">
            <PanelTitle icon={<Settings />} title="四阶段编排" meta="基础方法块拼成业务大块" />
            <StageBoard
              stages={orderedStages(pipelineDetail?.stages ?? [])}
              selectedStageId={selectedStageId}
              selectedStepId={selectedStepId}
              onSelectStage={(id) => {
                setSelectedStageId(id)
                setSelectedStepId('new')
              }}
              onSelectStep={(stageId, stepId) => {
                setSelectedStageId(stageId)
                setSelectedStepId(stepId)
              }}
              onGenerate={generateStageConfig}
              onPublish={publishStageConfig}
            />
          </section>

          <section className="workbench-panel editor-panel">
            <PanelTitle icon={<ShieldCheck />} title="基础方法配置" meta="编辑入参、出参和绑定" />
            <StepEditor
              key={`${selectedStage?.stage.id ?? 'none'}-${selectedStep?.step.id ?? 'new'}`}
              selected={selectedStep}
              selectedStage={selectedStage?.stage ?? null}
              disabled={!selectedStage}
              onSave={saveStep}
            />
          </section>

          <section className="workbench-panel preview-panel">
            <PanelTitle icon={<FileJson />} title="大块配置预览" meta={selectedStage ? stageLabel(selectedStage.stage.stage_type) : '选择阶段'} />
            <ReadonlyJSON value={selectedStage?.generated_config?.generated_config_json ?? selectedStageConfigFromPreview(preview, selectedStage?.stage.stage_type)} />
          </section>
        </div>

        <section className="bottom-grid">
          <RunTable runs={runs} onLoadSteps={loadStepRuns} />
          <StepRunTable stepRuns={stepRuns} />
        </section>
      </section>

      <ResultPanel result={result} />
    </main>
  )
}

type StepEditorPayload = {
  id: number | 'new'
  stageId: number
  body: {
    stage_id: number
    code: string
    name: string
    method_type: MethodType
    order_index: number
    timeout_seconds: number
    enabled: boolean
    params: MethodParam[]
    outputs: MethodOutput[]
  }
}

function LoginScreen({ onLogin }: { onLogin: (token: string) => void }) {
  const [error, setError] = useState('')

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const response = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: formValue(form, 'username'), password: formValue(form, 'password') }),
    })
    const data: unknown = await response.json().catch(() => ({}))
    const token = readToken(data)
    if (!response.ok || !token) {
      setError(readMessage(data) || '登录失败')
      return
    }
    onLogin(token)
  }

  return (
    <main className="login-shell">
      <section className="login-panel">
        <div className="login-title">
          <Database aria-hidden="true" />
          <div>
            <h1>数据仓库</h1>
            <p>登录后配置大块流水线</p>
          </div>
        </div>
        <form className="login-form" onSubmit={submit}>
          <Field label="用户名" name="username" defaultValue="admin" />
          <Field label="密码" name="password" defaultValue="123456" type="password" />
          {error && <div className="login-error">{error}</div>}
          <button className="primary" type="submit">登录</button>
        </form>
      </section>
    </main>
  )
}

function PipelineList({ pipelines, selectedId, onSelect }: { pipelines: PipelineDefinition[]; selectedId: number | null; onSelect: (id: number) => void }) {
  if (pipelines.length === 0) return <div className="empty-state">暂无流水线，先创建一个业务链路。</div>
  return (
    <div className="pipeline-list">
      {pipelines.map((pipeline) => (
        <button className={pipeline.id === selectedId ? 'pipeline-item active' : 'pipeline-item'} key={pipeline.id} type="button" onClick={() => onSelect(pipeline.id)}>
          <strong>{pipeline.name}</strong>
          <span>{pipeline.code}</span>
        </button>
      ))}
    </div>
  )
}

function MethodLibrary({ selectedStageType }: { selectedStageType: StageType }) {
  return (
    <section className="method-library">
      <h2>基础方法块</h2>
      {methodLibrary.map((item) => (
        <div className={item.stage === selectedStageType ? 'method-chip active' : 'method-chip'} key={item.type}>
          <strong>{item.label}</strong>
          <span>{stageLabel(item.stage)} / {item.description}</span>
        </div>
      ))}
    </section>
  )
}

function StageBoard({
  stages,
  selectedStageId,
  selectedStepId,
  onSelectStage,
  onSelectStep,
  onGenerate,
  onPublish,
}: {
  stages: PipelineStageDetail[]
  selectedStageId: number | null
  selectedStepId: number | 'new'
  onSelectStage: (id: number) => void
  onSelectStep: (stageId: number, stepId: number) => void
  onGenerate: (stageId: number) => void
  onPublish: (stageId: number) => void
}) {
  if (stages.length === 0) return <div className="empty-state">还没有阶段。创建流水线后会自动生成四个大块。</div>
  return (
    <div className="stage-lanes">
      {stages.map((detail) => (
        <section className={detail.stage.id === selectedStageId ? 'stage-lane active' : 'stage-lane'} key={detail.stage.id}>
          <button className="stage-title" type="button" onClick={() => onSelectStage(detail.stage.id)}>
            <span>{detail.stage.order_index}</span>
            <strong>{detail.stage.name}</strong>
            <small>{detail.stage.stage_type}</small>
          </button>
          <div className="stage-actions">
            <button type="button" onClick={() => onGenerate(detail.stage.id)}>生成配置</button>
            <button type="button" onClick={() => onPublish(detail.stage.id)}>发布</button>
          </div>
          <div className="step-timeline">
            {detail.steps.length === 0 ? (
              <div className="empty-state compact">这个大块还没有基础方法。</div>
            ) : (
              detail.steps.map((step) => (
                <button
                  className={step.step.id === selectedStepId ? 'step-card active' : 'step-card'}
                  key={step.step.id}
                  type="button"
                  onClick={() => onSelectStep(detail.stage.id, step.step.id)}
                >
                  <span className="step-index">{step.step.order_index || '-'}</span>
                  <span>
                    <strong>{step.step.name}</strong>
                    <small>{methodTypeLabel(step.step.method_type)} / {step.step.code}</small>
                  </span>
                </button>
              ))
            )}
          </div>
        </section>
      ))}
    </div>
  )
}

function StepEditor({
  selected,
  selectedStage,
  disabled,
  onSave,
}: {
  selected: MethodStepDetail | null
  selectedStage: PipelineStage | null
  disabled: boolean
  onSave: (payload: StepEditorPayload) => Promise<void>
}) {
  const allowedTypes = methodTypesForStage(selectedStage?.stage_type ?? 'fetch')
  const defaultType = selected?.step.method_type ?? allowedTypes[0]
  const [code, setCode] = useState(selected?.step.code ?? defaultCodeForMethod(defaultType))
  const [name, setName] = useState(selected?.step.name ?? methodTypeLabel(defaultType))
  const [methodType, setMethodType] = useState<MethodType>(defaultType)
  const [orderIndex, setOrderIndex] = useState(String(selected?.step.order_index ?? 1))
  const [timeoutSeconds, setTimeoutSeconds] = useState(String(selected?.step.timeout_seconds || 30))
  const [enabled, setEnabled] = useState(selected?.step.enabled ?? true)
  const [params, setParams] = useState<MethodParam[]>(selected?.params?.length ? selected.params : defaultParams(defaultType))
  const [outputs, setOutputs] = useState<MethodOutput[]>(selected?.outputs?.length ? selected.outputs : defaultOutputs(defaultType))

  function changeMethodType(nextType: MethodType) {
    setMethodType(nextType)
    if (!selected) {
      setCode(defaultCodeForMethod(nextType))
      setName(methodTypeLabel(nextType))
      setParams(defaultParams(nextType))
      setOutputs(defaultOutputs(nextType))
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedStage) return
    await onSave({
      id: selected?.step.id ?? 'new',
      stageId: selectedStage.id,
      body: {
        stage_id: selectedStage.id,
        code,
        name,
        method_type: methodType,
        order_index: Number(orderIndex),
        timeout_seconds: Number(timeoutSeconds),
        enabled,
        params: params.map((item, index) => ({ ...item, order_index: index + 1 })),
        outputs: outputs.map((item, index) => ({ ...item, order_index: index + 1 })),
      },
    })
  }

  return (
    <form className="step-editor" onSubmit={submit}>
      <div className="selected-stage-strip">
        <strong>{selectedStage ? stageLabel(selectedStage.stage_type) : '未选择阶段'}</strong>
        <span>基础方法会保存到这个大块阶段中。</span>
      </div>
      <div className="form-grid">
        <label>
          方法类型
          <select value={methodType} onChange={(event) => changeMethodType(event.target.value as MethodType)} disabled={disabled}>
            {allowedTypes.map((type) => <option key={type} value={type}>{methodTypeLabel(type)}</option>)}
          </select>
        </label>
        <label>
          步骤编码
          <input value={code} onChange={(event) => setCode(event.target.value)} disabled={disabled} />
        </label>
        <label>
          步骤名称
          <input value={name} onChange={(event) => setName(event.target.value)} disabled={disabled} />
        </label>
        <label>
          顺序
          <input value={orderIndex} onChange={(event) => setOrderIndex(event.target.value)} disabled={disabled} />
        </label>
        <label>
          超时秒数
          <input value={timeoutSeconds} onChange={(event) => setTimeoutSeconds(event.target.value)} disabled={disabled} />
        </label>
        <label className="checkbox-label">
          <input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} disabled={disabled} />
          启用
        </label>
      </div>
      <ParamTable params={params} onChange={setParams} disabled={disabled} methodType={methodType} />
      <OutputTable outputs={outputs} onChange={setOutputs} disabled={disabled} />
      <button className="primary" type="submit" disabled={disabled}>
        <Settings aria-hidden="true" />
        保存基础方法
      </button>
    </form>
  )
}

function ParamTable({ params, onChange, disabled, methodType }: { params: MethodParam[]; onChange: (params: MethodParam[]) => void; disabled: boolean; methodType: MethodType }) {
  function update(index: number, patch: Partial<MethodParam>) {
    onChange(params.map((item, itemIndex) => (itemIndex === index ? { ...item, ...patch } : item)))
  }
  function remove(index: number) {
    onChange(params.filter((_, itemIndex) => itemIndex !== index))
  }
  return (
    <section className="config-section">
      <div className="section-heading">
        <h3>入参配置</h3>
        <button type="button" onClick={() => onChange([...params, defaultParamForMethod(methodType)])} disabled={disabled}>
          <Plus aria-hidden="true" />
          增加入参
        </button>
      </div>
      <div className="param-table">
        <div className="param-head">
          <span>位置</span>
          <span>参数名</span>
          <span>来源</span>
          <span>值/绑定</span>
          <span>类型</span>
          <span>选项</span>
        </div>
        {params.map((item, index) => (
          <div className="param-row" key={`${item.location}-${item.name}-${index}`}>
            <select value={item.location} onChange={(event) => update(index, { location: event.target.value })} disabled={disabled}>
              {locationOptions(methodType).map((location) => <option key={location} value={location}>{location}</option>)}
            </select>
            <input value={item.name} onChange={(event) => update(index, { name: event.target.value })} disabled={disabled} />
            <select value={item.value_source} onChange={(event) => update(index, { value_source: event.target.value })} disabled={disabled}>
              {['static', 'binding', 'config', 'env', 'secret', 'time'].map((source) => <option key={source} value={source}>{source}</option>)}
            </select>
            <input value={item.value} onChange={(event) => update(index, { value: event.target.value })} disabled={disabled} />
            <select value={item.value_type} onChange={(event) => update(index, { value_type: event.target.value })} disabled={disabled}>
              {['string', 'int', 'decimal', 'bool', 'json', 'array', 'object'].map((type) => <option key={type} value={type}>{type}</option>)}
            </select>
            <div className="row-flags">
              <label><input type="checkbox" checked={item.required} onChange={(event) => update(index, { required: event.target.checked })} /> 必填</label>
              <label><input type="checkbox" checked={item.secret} onChange={(event) => update(index, { secret: event.target.checked })} /> 敏感</label>
              <button type="button" onClick={() => remove(index)} disabled={disabled}>删除</button>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

function OutputTable({ outputs, onChange, disabled }: { outputs: MethodOutput[]; onChange: (outputs: MethodOutput[]) => void; disabled: boolean }) {
  function update(index: number, patch: Partial<MethodOutput>) {
    onChange(outputs.map((item, itemIndex) => (itemIndex === index ? { ...item, ...patch } : item)))
  }
  return (
    <section className="config-section">
      <div className="section-heading">
        <h3>出参捕获</h3>
        <button type="button" onClick={() => onChange([...outputs, output('value', 'data.value', 'string', false, '')])} disabled={disabled}>
          <Plus aria-hidden="true" />
          增加出参
        </button>
      </div>
      <div className="output-list">
        {outputs.map((item, index) => (
          <div className="output-row" key={`${item.name}-${index}`}>
            <input value={item.name} onChange={(event) => update(index, { name: event.target.value })} disabled={disabled} />
            <input value={item.source_path} onChange={(event) => update(index, { source_path: event.target.value })} disabled={disabled} />
            <select value={item.value_type} onChange={(event) => update(index, { value_type: event.target.value })} disabled={disabled}>
              {['string', 'int', 'decimal', 'bool', 'object', 'array'].map((type) => <option key={type} value={type}>{type}</option>)}
            </select>
            <label><input type="checkbox" checked={item.required} onChange={(event) => update(index, { required: event.target.checked })} /> 必填</label>
          </div>
        ))}
      </div>
    </section>
  )
}

function RunTable({ runs, onLoadSteps }: { runs: PipelineRun[]; onLoadSteps: (runId: number) => void }) {
  return (
    <section className="workbench-panel">
      <PanelTitle icon={<Activity />} title="运行记录" meta="最近 50 次" />
      <div className="data-table-wrap">
        <table className="data-table">
          <thead><tr><th>ID</th><th>状态</th><th>数量</th><th>开始</th><th>操作</th></tr></thead>
          <tbody>
            {runs.map((run) => (
              <tr key={run.id}>
                <td>{run.id}</td>
                <td>{run.status}</td>
                <td>{run.success_count}/{run.total_count}</td>
                <td>{run.started_at ?? '-'}</td>
                <td><button type="button" onClick={() => onLoadSteps(run.id)}>步骤</button></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}

function StepRunTable({ stepRuns }: { stepRuns: StepRun[] }) {
  return (
    <section className="workbench-panel">
      <PanelTitle icon={<Send />} title="步骤明细" meta="输入、输出和错误" />
      {stepRuns.length === 0 ? <div className="empty-state">选择运行记录查看步骤明细。</div> : (
        <div className="step-run-list">
          {stepRuns.map((run) => (
            <details key={run.id}>
              <summary>{run.step_code} / {run.method_type} / {run.status}</summary>
              <ReadonlyJSON value={{ input: parseJsonText(run.input_json), output: parseJsonText(run.output_json), error: run.error_message }} />
            </details>
          ))}
        </div>
      )}
    </section>
  )
}

function ResultPanel({ result }: { result: ApiResult | null }) {
  return (
    <aside className="result-panel">
      <PanelTitle icon={<FileJson />} title="接口结果" meta={result ? String(result.status) : '等待操作'} />
      <ReadonlyJSON value={result?.data ?? {}} />
    </aside>
  )
}

function ReadonlyJSON({ value }: { value: unknown }) {
  return <pre className="json-preview" aria-label="只读 JSON">{jsonText(value)}</pre>
}

function PanelTitle({ icon, title, meta }: { icon: ReactNode; title: string; meta: string }) {
  return (
    <div className="panel-title">
      {icon}
      <div>
        <h3>{title}</h3>
        <span>{meta}</span>
      </div>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: ReactNode }) {
  return <div className="metric"><span>{label}</span><strong>{value}</strong></div>
}

function Field({ label, name, defaultValue, type = 'text' }: { label: string; name: string; defaultValue: string; type?: string }) {
  return (
    <label>
      {label}
      <input name={name} defaultValue={defaultValue} type={type} />
    </label>
  )
}

function methodTypesForStage(stageType: StageType): MethodType[] {
  switch (stageType) {
    case 'fetch':
      return ['request', 'extract', 'db_query']
    case 'process':
      return ['mapping', 'validate', 'db_query', 'db_write', 'template', 'request']
    case 'push':
      return ['template', 'delivery', 'request']
    case 'log':
      return ['log', 'db_write', 'delivery']
  }
}

function defaultParams(type: MethodType) {
  switch (type) {
    case 'request':
      return [
        param('url', 'url', 'config', 'cfg.api.url', 'string', true, false, '请求地址'),
        param('request', 'method', 'static', 'POST', 'string', true, false, 'HTTP 方法'),
        param('header', 'Content-Type', 'static', 'application/json', 'string', true, false, '内容类型'),
      ]
    case 'extract':
      return [param('response', 'records_path', 'static', 'data.items', 'string', true, false, '响应列表路径')]
    case 'mapping':
      return [
        param('mapping', 'table_name', 'static', 'clean_records', 'string', true, false, '目标表'),
        param('field', 'business_key', 'static', '$.id', 'string', false, false, '业务主键'),
      ]
    case 'validate':
      return [param('rule', 'required_fields', 'static', '[]', 'json', false, false, '必填字段')]
    case 'db_query':
      return [param('query', 'table', 'static', 'source_table', 'string', true, false, '查询表')]
    case 'db_write':
      return [param('write', 'table', 'static', 'target_table', 'string', true, false, '写入表')]
    case 'template':
      return [param('template', 'payload', 'static', '{}', 'json', true, false, '报文模板')]
    case 'delivery':
      return [
        param('url', 'url', 'config', 'cfg.destination.url', 'string', true, false, '推送地址'),
        param('request', 'method', 'static', 'POST', 'string', true, false, 'HTTP 方法'),
      ]
    case 'log':
      return [param('runtime', 'trace_id', 'binding', 'steps.previous.outputs.trace_id', 'string', false, false, '追踪号')]
  }
}

function defaultOutputs(type: MethodType) {
  switch (type) {
    case 'request':
      return [output('response_body', 'response_body', 'string', false, '响应文本')]
    case 'extract':
      return [output('records', 'data.items', 'array', true, '提取结果')]
    case 'mapping':
      return [output('record', 'record', 'object', true, '清洗结果')]
    case 'validate':
      return [output('valid', 'valid', 'bool', false, '校验结果')]
    case 'db_query':
      return [output('rows', 'rows', 'array', false, '查询结果')]
    case 'db_write':
      return [output('affected_rows', 'affected_rows', 'int', false, '写入行数')]
    case 'template':
      return [output('payload', 'payload', 'object', false, '渲染报文')]
    case 'delivery':
      return [output('http_status', 'http_status', 'int', true, 'HTTP 状态')]
    case 'log':
      return [output('logged', 'success', 'bool', false, '日志结果')]
  }
}

function defaultParamForMethod(type: MethodType) {
  if (type === 'request' || type === 'delivery') return param('query', 'name', 'static', '', 'string', false, false, '')
  if (type === 'mapping') return param('field', 'field_name', 'static', '$.path', 'string', false, false, '')
  if (type === 'template') return param('template', 'field', 'static', '', 'string', false, false, '')
  return param('runtime', 'name', 'static', '', 'string', false, false, '')
}

function locationOptions(type: MethodType) {
  if (type === 'request' || type === 'delivery') return ['url', 'request', 'query', 'header', 'body', 'response']
  if (type === 'mapping') return ['mapping', 'field']
  if (type === 'template') return ['template', 'body', 'runtime']
  if (type === 'validate') return ['rule', 'field', 'runtime']
  if (type === 'db_query') return ['query', 'where', 'runtime']
  if (type === 'db_write') return ['write', 'field', 'runtime']
  return ['runtime', 'log']
}

function methodTypeLabel(type: MethodType) {
  const labels: Record<MethodType, string> = {
    request: 'Request 请求',
    extract: 'Extract 提取',
    mapping: 'Mapping 清洗',
    validate: 'Validate 校验',
    db_query: 'DB Query 查询',
    db_write: 'DB Write 写入',
    template: 'Template 模板',
    delivery: 'Delivery 推送',
    log: 'Log 记录',
  }
  return labels[type]
}

function stageLabel(type: StageType) {
  const labels: Record<StageType, string> = {
    fetch: '数据获取',
    process: '数据处理',
    push: '数据推送',
    log: '日志记录',
  }
  return labels[type]
}

function defaultCodeForMethod(type: MethodType) {
  return `${type}_${Date.now()}`
}

function orderedStages(stages: PipelineStageDetail[]) {
  return [...stages].sort((a, b) => {
    const left = stageOrder.indexOf(a.stage.stage_type)
    const right = stageOrder.indexOf(b.stage.stage_type)
    return left === right ? a.stage.order_index - b.stage.order_index : left - right
  })
}

function selectedStageConfigFromPreview(preview: unknown, stageType?: StageType) {
  if (!preview || !stageType || typeof preview !== 'object') return {}
  const stages = (preview as { stages?: unknown }).stages
  if (!Array.isArray(stages)) return {}
  return stages.find((item) => item && typeof item === 'object' && (item as { stage_type?: unknown }).stage_type === stageType) ?? {}
}

function param(location: string, name: string, valueSource: string, value: string, valueType: string, required: boolean, secret: boolean, description: string): MethodParam {
  return { location, name, value_source: valueSource, value, value_type: valueType, required, secret, description, order_index: 0 }
}

function output(name: string, sourcePath: string, valueType: string, required: boolean, description: string): MethodOutput {
  return { name, source_path: sourcePath, value_type: valueType, required, description, order_index: 0 }
}

function readList<T>(result: ApiResult, key: string): T[] {
  const value = readDataField(result.data, key)
  return Array.isArray(value) ? (value as T[]) : []
}

function readObject<T>(result: ApiResult, key: string): T | null {
  const value = readDataField(result.data, key)
  return value && typeof value === 'object' ? (value as T) : null
}

function readDataField(data: unknown, key: string) {
  if (!data || typeof data !== 'object') return undefined
  const envelope = data as { data?: Record<string, unknown> }
  return envelope.data?.[key]
}

function readToken(data: unknown) {
  const value = readDataField(data, 'token')
  return typeof value === 'string' ? value : ''
}

function readMessage(data: unknown) {
  if (!data || typeof data !== 'object') return ''
  const envelope = data as { msg?: unknown }
  return typeof envelope.msg === 'string' ? envelope.msg : ''
}

function isSuccessPayload(data: unknown) {
  if (!data || typeof data !== 'object') return false
  const envelope = data as { code?: unknown }
  return envelope.code === 0 || envelope.code === 200
}

function formValue(form: FormData, key: string) {
  const value = form.get(key)
  return typeof value === 'string' ? value : ''
}

function jsonText(value: unknown) {
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  return JSON.stringify(value ?? {}, null, 2)
}

function parseJsonText(value: string) {
  if (!value) return {}
  try {
    return JSON.parse(value) as unknown
  } catch {
    return value
  }
}

export default App
