import { RefreshCcw } from 'lucide-react'
import { type FormEvent, useEffect, useRef, useState } from 'react'
import {
  clearMallWeatherPendingRefresh,
  loadMallWeatherPendingRefresh,
  mallWeatherRefreshDisposition,
  mallWeatherRefreshKey,
  mallWeatherRefreshPath,
  mallWeatherRefreshRequest,
  mallWeatherRefreshResultMessage,
  pollMallWeatherFetchRun,
  saveMallWeatherPendingRefresh,
  type MallWeatherFetchRun,
  type MallWeatherMall,
  type MallWeatherPendingRefresh,
  type MallWeatherRefreshRequest,
} from './mallWeather'
import styles from './MallWeatherManualRefresh.module.css'

type ApiResult = { ok: boolean; status: number; data: unknown }
type ApiClient = (path: string, options?: { method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'; body?: unknown; headers?: Record<string, string>; showResult?: boolean; silentLoading?: boolean; signal?: AbortSignal }) => Promise<ApiResult>
type WeatherOverviewReloadResult = 'ready' | 'waiting' | 'failed' | 'aborted'

export function MallWeatherManualRefresh({ actorID, mall, client, onWeatherUpdated }: {
  actorID: string
  mall: MallWeatherMall
  client: ApiClient
  onWeatherUpdated: (signal: AbortSignal) => Promise<WeatherOverviewReloadResult>
}) {
  const [pending, setPending] = useState<MallWeatherPendingRefresh | null>(() => loadMallWeatherPendingRefresh(actorID, mall.id, window.sessionStorage))
  const [reason, setReason] = useState(() => pending?.body.reason || '管理端手工刷新')
  const [submitting, setSubmitting] = useState(false)
  const [monitoring, setMonitoring] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const operationController = useRef<AbortController | null>(null)
  const reasonHelpID = `mall-weather-refresh-reason-help-${actorID}-${mall.id}`

  useEffect(() => () => operationController.current?.abort(), [])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    let request = pending
    if (!request) {
      let body: MallWeatherRefreshRequest
      try {
        body = mallWeatherRefreshRequest(['V26_FULL'], reason)
      } catch {
        setError('请填写单行刷新原因，最多 500 个字符')
        setMessage('')
        return
      }
      request = { key: mallWeatherRefreshKey(), body }
      saveMallWeatherPendingRefresh(actorID, mall.id, request, window.sessionStorage)
      setPending(request)
    }
    operationController.current?.abort()
    const controller = new AbortController()
    operationController.current = controller
    setSubmitting(true)
    setError('')
    setMessage('')
    try {
      const response = await client(mallWeatherRefreshPath(mall.id), {
        method: 'POST',
        body: request.body,
        headers: { 'Idempotency-Key': request.key },
        showResult: false,
        silentLoading: true,
        signal: controller.signal,
      })
      if (controller.signal.aborted) return
      setSubmitting(false)
      const disposition = mallWeatherRefreshDisposition(response, actorID, mall.id, request.body)
      if (disposition.kind === 'rejected') {
        clearMallWeatherPendingRefresh(actorID, mall.id, window.sessionStorage)
        setPending(null)
        setError(weatherRequestError(response.status, '天气刷新任务提交失败', '当前账号缺少 weather.refresh 权限'))
        return
      }
      if (disposition.kind === 'uncertain') {
        setError('刷新响应暂不确定；已保留原请求，请使用“重试原请求”确认。')
        return
      }
      clearMallWeatherPendingRefresh(actorID, mall.id, window.sessionStorage)
      setPending(null)
      const queued = disposition.result.kinds.some((item) => item.status === 'QUEUED')
      if (!queued) {
        const reloaded = await onWeatherUpdated(controller.signal)
        if (controller.signal.aborted) return
        if (reloaded === 'failed') {
          setError('数据仍然新鲜，但页面重新加载失败，请点击“重新加载”重试。')
          return
        }
        if (reloaded === 'waiting') {
          setMessage('数据仍然新鲜，页面正在等待未来逐小时温度同步。')
          return
        }
        if (reloaded === 'aborted') return
        setMessage(mallWeatherRefreshResultMessage(disposition.result))
        return
      }

      setMonitoring(true)
      setMessage('采集任务已入队，正在等待彩云综合天气数据写入…')
      const pollResult = await pollMallWeatherFetchRun(
        client,
        mall.id,
        disposition.result.requestedAt,
        'MANUAL',
        disposition.result.correlationId,
        { signal: controller.signal },
      )
      if (controller.signal.aborted) return
      if (pollResult.kind === 'timed_out') {
        setError('任务已入队，但 60 秒内未发现完成记录。请确认 MALL_WEATHER_ENABLED=true 且 weather 队列消费进程正在运行，然后重试。')
        setMessage('')
        return
      }
      if (pollResult.kind === 'query_error') {
        setError(weatherRequestError(pollResult.status, '天气采集状态查询失败', '当前账号缺少 weather.read 权限'))
        setMessage('')
        return
      }
      if (pollResult.kind !== 'terminal') return
      if (pollResult.run.status !== 'SUCCESS' && pollResult.run.status !== 'PARTIAL_SUCCESS') {
        setError(weatherFetchRunFailureMessage(pollResult.run))
        setMessage('')
        return
      }
      const hourlyRows = pollResult.run.rowCounts.hourly ?? 0
      if (hourlyRows < 1) {
        setError('采集任务已完成，但没有写入未来逐小时数据。请查看任务的解析告警和服务端日志。')
        setMessage('')
        return
      }
      const reloaded = await onWeatherUpdated(controller.signal)
      if (controller.signal.aborted) return
      if (reloaded === 'failed') {
        setError('采集已完成，但天气数据重新加载失败，请点击页面上的“重新加载”重试。')
        setMessage('')
        return
      }
      if (reloaded === 'waiting') {
        setMessage(`采集已完成并写入 ${hourlyRows} 条逐小时数据，页面正在等待温度读模型同步。`)
        return
      }
      if (reloaded === 'aborted') return
      setMessage(`天气已更新并重新加载，共写入 ${hourlyRows} 条未来逐小时数据。`)
    } catch {
      if (controller.signal.aborted) return
      setError('天气采集状态查询异常，请检查网络后重试。')
      setMessage('')
    } finally {
      if (operationController.current === controller) {
        operationController.current = null
        if (!controller.signal.aborted) {
          setSubmitting(false)
          setMonitoring(false)
        }
      }
    }
  }

  function changeReason(value: string) {
    setReason(value)
  }

  const refreshPanel = (
    <section className={styles.panel}>
      <div className={styles['sectionTitle']}><div><strong>手工刷新</strong><span>提交异步采集任务，不阻塞等待供应商</span></div><RefreshCcw aria-hidden="true" /></div>
      <form className={styles['form']} onSubmit={submit} aria-busy={submitting || monitoring}>
        <label><span>采集范围</span><input name="weatherRefreshScope" value="综合天气（含实况、分钟、小时、逐日、预警、生活指数）" disabled />
          <small>固定提交全部天气数据类型</small>
        </label>
        <label><span>刷新原因</span><input name="weatherRefreshReason" value={reason} onChange={(event) => changeReason(event.currentTarget.value)} disabled={submitting || monitoring || Boolean(pending)} aria-describedby={reasonHelpID} />
          <small id={reasonHelpID}>必填单行文本，最多 500 个字符</small>
        </label>
        <div className={styles['submit']}>
          <span>操作</span>
          <button className={styles['primary']} type="submit" disabled={submitting || monitoring}>
            {submitting ? '提交中' : monitoring ? '等待采集完成' : pending ? '重试原请求' : '提交刷新'}
          </button>
          <small>提交后异步执行并跟踪结果</small>
        </div>
      </form>
      {message && <p className={styles['message']} role="status">{message}</p>}
      {error && <p className={[styles['message'], styles['error']].join(' ')} role="alert">{error}</p>}
    </section>
  )

  return (
    <details className={styles['compact']}>
      <summary><RefreshCcw aria-hidden="true" />综合天气刷新</summary>
      <div className={styles['popover']}>{refreshPanel}</div>
    </details>
  )
}

function weatherFetchRunFailureMessage(run: MallWeatherFetchRun) {
  const detail = run.errorMessageSafe || run.errorCode || '未返回安全错误信息'
  return `天气采集失败：${detail}`
}


function weatherRequestError(status: number, fallback: string, forbidden: string) {
  if (status === 0) return '无法连接服务，请检查网络后重试'
  if (status === 403) return forbidden
  if (status === 404) return '商场或天气数据不存在'
  if (status === 422) return '商场坐标尚未确认，暂时无法查询天气'
  return `${fallback}（HTTP ${status}）`
}
