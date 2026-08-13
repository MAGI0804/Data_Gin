import { RefreshCcw } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  clearMallWeatherPendingSheetPush,
  loadMallWeatherPendingSheetPush,
  mallWeatherSheetPushKey,
  mallWeatherSheetPushRequest,
  mallWeatherSheetPushRequestMatchesOption,
  mallWeatherSheetPushResultMatchesRequest,
  mallWeatherSheetPushRunMatchesResult,
  parseMallWeatherSheetPushDryRun,
  parseMallWeatherSheetPushOptions,
  parseMallWeatherSheetPushResult,
  pollMallWeatherSheetPushRun,
  saveMallWeatherPendingSheetPush,
  type MallWeatherMall,
  type MallWeatherPendingSheetPush,
  type MallWeatherSheetPushDryRun,
  type MallWeatherSheetPushOption,
  type MallWeatherSheetPushRun,
} from './mallWeather'
import styles from './MallWeatherSheetPushPanel.module.css'

type ApiResult = { ok: boolean; status: number; data: unknown }
type ApiClient = (path: string, options?: { method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'; body?: unknown; headers?: Record<string, string>; showResult?: boolean; silentLoading?: boolean; signal?: AbortSignal }) => Promise<ApiResult>
type LoadState = 'idle' | 'loading' | 'success' | 'error'

export function MallWeatherSheetPushPanel({ actorID, mall, client }: {
  actorID: string
  mall: MallWeatherMall
  client: ApiClient
}) {
  const restored = useMemo(() => loadMallWeatherPendingSheetPush(actorID, mall.id, window.sessionStorage), [actorID, mall.id])
  const [options, setOptions] = useState<MallWeatherSheetPushOption[]>([])
  const [selectedDestinationID, setSelectedDestinationID] = useState(restored?.body.destinationId || 0)
  const [optionState, setOptionState] = useState<LoadState>('loading')
  const [dryRun, setDryRun] = useState<MallWeatherSheetPushDryRun | null>(null)
  const [pending, setPending] = useState<MallWeatherPendingSheetPush | null>(restored)
  const [checking, setChecking] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [monitoring, setMonitoring] = useState(false)
  const [run, setRun] = useState<MallWeatherSheetPushRun | null>(null)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const optionRequestSequence = useRef(0)
  const pollController = useRef<AbortController | null>(null)

  const loadOptions = useCallback(async (signal?: AbortSignal) => {
    const sequence = ++optionRequestSequence.current
    setOptionState('loading')
    setError('')
    const response = await client('/v1/weather-sheet-push-options', { method: 'GET', showResult: false, silentLoading: true, signal })
    if (signal?.aborted || sequence !== optionRequestSequence.current) return null
    if (!response.ok) {
      setOptionState('error')
      setError(weatherPushError(response.status, '已有推送目标加载失败'))
      return null
    }
    const parsed = parseMallWeatherSheetPushOptions(response.data)
    if (!parsed) {
      setOptionState('error')
      setError('已有推送目标响应格式不正确，请联系管理员')
      return null
    }
    setOptions(parsed)
    setSelectedDestinationID((current) => parsed.some((option) => option.destinationId === current) ? current : parsed[0]?.destinationId || 0)
    setOptionState('success')
    return parsed
  }, [client])

  useEffect(() => {
    const controller = new AbortController()
    void loadOptions(controller.signal)
    return () => controller.abort()
  }, [loadOptions])

  useEffect(() => () => pollController.current?.abort(), [])

  const selectedOption = options.find((option) => option.destinationId === selectedDestinationID)
  const pendingMatchesOption = Boolean(selectedOption && pending && mallWeatherSheetPushRequestMatchesOption(pending.body, selectedOption, mall.id))

  function changeOption(value: string) {
    const destinationID = Number(value)
    if (!Number.isSafeInteger(destinationID) || destinationID <= 0) return
    setSelectedDestinationID(destinationID)
    setDryRun(null)
    setError(pending ? '已保留结果待确认的原请求；如需改用新目标，请先明确放弃原请求，再重新验证绑定。' : '')
    setMessage('')
  }

  async function checkBinding() {
    if (!selectedOption) {
      setError('请选择一个可用的已有推送目标。')
      return
    }
    const body = mallWeatherSheetPushRequest(selectedOption, mall.id)
    setChecking(true)
    setError('')
    setMessage('')
    const response = await client('/v1/weather-sheet-pushes/dry-run', { method: 'POST', body, showResult: false, silentLoading: true })
    setChecking(false)
    if (!response.ok) {
      setDryRun(null)
      setError(weatherPushError(response.status, '推送绑定验证失败'))
      return
    }
    const parsed = parseMallWeatherSheetPushDryRun(response.data)
    if (!parsed || parsed.destinationId !== selectedOption.destinationId || parsed.profileId !== selectedOption.profileId || parsed.profileVersion !== selectedOption.profileVersion) {
      setDryRun(null)
      setError('推送试算响应与所选目标不一致，请刷新目标后重试。')
      return
    }
    setDryRun(parsed)
    setMessage(parsed.canExecute ? '绑定验证通过，可以发起该商场的天气推送。' : '绑定验证完成，但目标当前不可执行。')
  }

  async function submitPush() {
    if (!selectedOption || !dryRun?.canExecute || dryRun.destinationId !== selectedOption.destinationId) {
      setError('请先完成当前目标的绑定验证。')
      return
    }
    if (pending && !pendingMatchesOption) {
      setError('保留的原请求与当前目标版本不同；请先放弃原请求，再重新验证绑定。')
      return
    }
    let request = pending
    if (!request) {
      request = { key: mallWeatherSheetPushKey(), body: mallWeatherSheetPushRequest(selectedOption, mall.id) }
      setPending(request)
      saveMallWeatherPendingSheetPush(actorID, mall.id, request, window.sessionStorage)
    }
    setSubmitting(true)
    setError('')
    setMessage('')
    const response = await client('/v1/weather-sheet-pushes', {
      method: 'POST', body: request.body, headers: { 'Idempotency-Key': request.key }, showResult: false, silentLoading: true,
    })
    if (!response.ok) {
      if (response.status === 409) {
        setDryRun(null)
        const refreshed = await loadOptions()
        const exactOption = refreshed?.find((option) => mallWeatherSheetPushRequestMatchesOption(request.body, option, mall.id))
        if (exactOption) {
          setSelectedDestinationID(exactOption.destinationId)
          setError('推送请求发生冲突，但原目标版本仍可用。已保留原请求；请重新验证绑定后重试原请求。')
        } else if (refreshed) {
          setError('原推送目标或 Profile 已删除、停用或升版。原请求仍保留；请放弃原请求，再选择最新目标重新验证。')
        } else {
          setError('推送请求发生冲突，且暂时无法确认目标版本。原请求仍保留；请稍后刷新目标。')
        }
      } else if (response.status === 0 || response.status === 408 || response.status >= 500) {
        setError('推送结果暂不确定，已保留原请求；请重新验证绑定后重试原请求确认。')
      } else {
        setPending(null)
        clearMallWeatherPendingSheetPush(actorID, mall.id, window.sessionStorage)
        setError(weatherPushError(response.status, '天气推送发起失败'))
      }
      setSubmitting(false)
      return
    }
    const result = parseMallWeatherSheetPushResult(response.data)
    if (!result) {
      setSubmitting(false)
      setError('推送已提交，但响应格式不正确。原请求已保留；请到运行记录中确认后再决定是否放弃。')
      return
    }
    if (!mallWeatherSheetPushResultMatchesRequest(result, request.body)) {
      setSubmitting(false)
      setError('推送响应与原请求的目标或 Profile 版本不一致。结果暂不确定，原请求已保留；请到运行记录中确认。')
      return
    }
    setSubmitting(false)
    setPending(null)
    clearMallWeatherPendingSheetPush(actorID, mall.id, window.sessionStorage)
    setRun({
      runId: result.runId, traceId: result.traceId, status: result.status === 'RUNNING' ? 'RUNNING' : 'PENDING',
      destinationId: result.destinationId, profileId: result.profileId, profileVersion: result.profileVersion,
      totalCount: result.estimatedRows, successCount: 0, failedCount: 0,
    })
    setMonitoring(true)
    setMessage(`推送任务 #${result.runId} 已创建，正在查询真实执行状态。`)
    pollController.current?.abort()
    const controller = new AbortController()
    pollController.current = controller
    const pollResult = await pollMallWeatherSheetPushRun(client, result.runId, {
      signal: controller.signal,
      isPageVisible: () => document.visibilityState === 'visible',
    })
    if (controller.signal.aborted) return
    setMonitoring(false)
    if (pollResult.kind === 'timed_out') {
      setError('推送任务仍在处理中，60 秒内未到达终止状态。已停止轮询，可稍后刷新页面确认。')
      return
    }
    if (pollResult.kind === 'query_error') {
      setError(weatherPushError(pollResult.status, '推送运行状态查询失败'))
      return
    }
    if (pollResult.kind !== 'terminal' || !mallWeatherSheetPushRunMatchesResult(pollResult.run, result)) {
      setError('推送运行记录与创建结果不一致，已停止轮询；请刷新页面后确认。')
      return
    }
    setRun(pollResult.run)
    if (pollResult.run.status === 'FAILED') {
      setError(`推送任务 #${pollResult.run.runId} 失败：成功 ${pollResult.run.successCount} 行，失败 ${pollResult.run.failedCount} 行。`)
      return
    }
    setMessage(`推送任务 #${pollResult.run.runId} ${pollResult.run.status === 'SUCCESS' ? '已完成' : '部分完成'}：成功 ${pollResult.run.successCount} 行，失败 ${pollResult.run.failedCount} 行。`)
  }

  function abandonPending() {
    setPending(null)
    setDryRun(null)
    clearMallWeatherPendingSheetPush(actorID, mall.id, window.sessionStorage)
    setMessage('已放弃待确认请求。为避免误推，请重新验证绑定后再创建新任务。')
    setError('')
  }

  return (
    <section className={styles.panel}>
      <div className={styles['sectionTitle']}><div><strong>绑定已有推送目标</strong><span>以 {mall.nameCn} 作为 mallIds 过滤条件，先验证再发起推送</span></div><span>飞书天气表</span></div>
      {optionState === 'loading' && <LoadingState label="正在加载已有推送目标" />}
      {optionState === 'error' && <RequestError message={error} onRetry={() => void loadOptions()} />}
      {optionState === 'success' && options.length === 0 && <EmptyState title="没有可用推送目标" detail="请先启用 feishu_sheet 推送目标，并关联已启用的天气导出 Profile。" />}
      {optionState === 'success' && options.length > 0 && <div className={styles['controls']}>
        <label><span>已有推送目标</span><select name="weatherSheetPushTarget" value={selectedDestinationID} onChange={(event) => changeOption(event.currentTarget.value)} disabled={checking || submitting || monitoring}>
          {options.map((option) => <option value={option.destinationId} key={option.destinationId}>{option.name} · {option.code} · {option.profileCode} v{option.profileVersion}</option>)}
        </select></label>
        <button type="button" onClick={() => void checkBinding()} disabled={checking || submitting || monitoring}>{checking ? '验证中' : '验证绑定'}</button>
        <button className={styles['primary']} type="button" onClick={() => void submitPush()} disabled={checking || submitting || monitoring || !dryRun?.canExecute || Boolean(pending && !pendingMatchesOption)}>{submitting ? '提交中' : pending ? '重试原请求' : '绑定并发起推送'}</button>
      </div>}
      {pending && <div className={styles['pending']} role="status">
        <span>存在结果待确认的原请求：商场 #{pending.body.filters.mallIds[0]}，目标 #{pending.body.destinationId}，Profile #{pending.body.profileId} v{pending.body.expectedProfileVersion}。{pendingMatchesOption ? ' 重试前仍需重新验证绑定。' : ' 当前选择与原请求不一致。'}</span>
        <button type="button" onClick={abandonPending} disabled={submitting}>放弃原请求</button>
      </div>}
      {dryRun && <div className={styles['summary']} aria-live="polite">
        <MetaItem label="写入模式" value={dryRun.writeMode} />
        <MetaItem label="预计行数" value={String(dryRun.totalEstimatedRows)} />
        <MetaItem label="预计单元格" value={String(dryRun.totalEstimatedCells)} />
        <MetaItem label="可执行" value={dryRun.canExecute ? '是' : '否'} />
      </div>}
      {dryRun && dryRun.warnings.length > 0 && <ul className={styles['warnings']}>{dryRun.warnings.map((warning) => <li key={warning}>{warning}</li>)}</ul>}
      {dryRun && dryRun.datasets.length > 0 && <div className={styles['datasets']}>
        {dryRun.datasets.map((dataset, datasetIndex) => <section key={`${dataset.datasetKind}:${datasetIndex}`}>
          <strong>{dataset.datasetKind}</strong>
          <span>预计 {dataset.estimatedRows} 行 / {dataset.estimatedCells} 单元格 · 可执行：{dataset.canExecute ? '是' : '否'}</span>
          {dataset.warnings.length > 0 && <ul className={styles['warnings']}>{dataset.warnings.map((warning, warningIndex) => <li key={`${warningIndex}:${warning}`}>{warning}</li>)}</ul>}
        </section>)}
      </div>}
      {run && <div className={styles['summary']} aria-live="polite">
        <MetaItem label="运行状态" value={monitoring ? `${run.status}（查询中）` : run.status} />
        <MetaItem label="成功行数" value={String(run.successCount)} />
        <MetaItem label="失败行数" value={String(run.failedCount)} />
        <MetaItem label="总行数" value={String(run.totalCount)} />
      </div>}
      {message && <p className={styles['message']} role="status">{message}</p>}
      {error && optionState !== 'error' && <p className={[styles['message'], styles['error']].join(' ')} role="alert">{error}</p>}
    </section>
  )
}


function MetaItem({ label, value }: { label: string; value: string }) {
  return <div className={styles['mall-weather-meta-item']}><span>{label}</span><strong>{value || '—'}</strong></div>
}

function RequestError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <div className={[styles['mall-weather-request-state'], styles.error].join(' ')} role="alert"><strong>加载失败</strong><span>{message}</span><button type="button" onClick={onRetry}>重试</button></div>
}

function LoadingState({ label }: { label: string }) {
  return <div className={styles['mall-weather-request-state']} role="status" aria-busy="true"><RefreshCcw aria-hidden="true" /><span>{label}</span></div>
}

function EmptyState({ title, detail }: { title: string; detail: string }) {
  return <div className={styles['empty-state']} role="status"><strong>{title}</strong><span>{detail}</span></div>
}

function weatherPushError(status: number, fallback: string) {
  if (status === 0) return '无法连接服务，请检查网络后重试'
  if (status === 403) return '飞书天气推送未开启，或当前账号缺少 weather.feishu.push 权限'
  if (status === 404) return '所选推送目标或天气导出 Profile 不存在，请刷新目标'
  if (status === 409) return '推送目标或 Profile 已更新，请刷新后重新验证绑定'
  if (status === 422) return '该商场或推送配置未通过校验，请检查坐标、时间范围和目标配置'
  return `${fallback}（HTTP ${status}）`
}
