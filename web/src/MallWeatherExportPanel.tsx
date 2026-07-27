import { Download, FileSpreadsheet, RefreshCcw } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  clearMallWeatherExportSession,
  mallWeatherExportCreateDisposition,
  loadMallWeatherExportSession,
  mallWeatherExportCreateRequest,
  mallWeatherExportContentPath,
  mallWeatherExportJobPath,
  mallWeatherExportJobTerminal,
  mallWeatherExportKey,
  mallWeatherExportMaximumPollAttempts,
  mallWeatherExportMaximumTransientPollRetries,
  mallWeatherExportPollFailureAction,
  mallWeatherExportPollIntervalMilliseconds,
  mallWeatherExportPollingActive,
  mallWeatherExportPollRetryDelayMilliseconds,
  mallWeatherExportProgress,
  mallWeatherExportRequestMatches,
  MallWeatherExportDownloadTimeoutError,
  MallWeatherExportRequestTimeoutError,
  parseMallWeatherExportJob,
  parseMallWeatherExportSafeErrorMessage,
  resolveMallWeatherExportStorage,
  saveMallWeatherExportSession,
  waitForMallWeatherExportDownload,
  waitForMallWeatherExportRequest,
  type MallWeatherExportJob,
  type MallWeatherExportPendingCreate,
  type MallWeatherExportSession,
} from './mallWeatherExport'

const exportStorageWarning = '浏览器无法更新导出恢复信息；当前页面可继续使用，请勿刷新或关闭页面。'

type WeatherExportAPIResult = {
  ok: boolean
  status: number
  data: unknown
}

type WeatherExportAPIClient = (
  path: string,
  options: {
    method: 'GET' | 'POST'
    body?: unknown
    headers?: Record<string, string>
    showResult: false
    silentLoading: true
    signal?: AbortSignal
  },
) => Promise<WeatherExportAPIResult>

type WeatherExportFileClient = (
  path: string,
  fileName: string,
  signal: AbortSignal,
) => Promise<WeatherExportAPIResult>

type MallWeatherExportPanelProps = {
  actorID: string
  mallID: number
  mallName: string
  client: WeatherExportAPIClient
  downloadFile: WeatherExportFileClient
}

export function MallWeatherExportPanel({ actorID, mallID, mallName, client, downloadFile }: MallWeatherExportPanelProps) {
  const exportStorage = useMemo(
    () => resolveMallWeatherExportStorage(() => window.sessionStorage),
    [],
  )
  const restoredSession = useMemo(
    () => loadMallWeatherExportSession(actorID, mallID, exportStorage),
    [actorID, exportStorage, mallID],
  )
  const [creatingJob, setCreatingJob] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [job, setJob] = useState<MallWeatherExportJob | null>(
    () => restoredSession?.jobId ? pendingJob(restoredSession.jobId) : null,
  )
  const [actionError, setActionError] = useState('')
  const [pollError, setPollError] = useState('')
  const [storageWarning, setStorageWarning] = useState(() => exportStorage ? '' : exportStorageWarning)
  const [pollRevision, setPollRevision] = useState(0)
  const [, setPendingRevision] = useState(0)
  const actionController = useRef<AbortController | null>(null)
  const pendingCreate = useRef<MallWeatherExportPendingCreate | null>(restoredSession?.pending ?? null)
  const polledJobID = job?.jobId ?? ''
  const polledJobStatus = job?.status

  const persistSession = useCallback((session: MallWeatherExportSession) => {
    const persisted = saveMallWeatherExportSession(actorID, mallID, session, exportStorage)
    setStorageWarning(persisted ? '' : exportStorageWarning)
    return persisted
  }, [actorID, exportStorage, mallID])

  const clearSession = useCallback(() => {
    const cleared = clearMallWeatherExportSession(actorID, mallID, exportStorage)
    setStorageWarning(cleared ? '' : exportStorageWarning)
    return cleared
  }, [actorID, exportStorage, mallID])

  function replacePendingCreate(next: MallWeatherExportPendingCreate | null) {
    pendingCreate.current = next
    setPendingRevision((current) => current + 1)
  }

  useEffect(() => {
    actionController.current?.abort()
    actionController.current = null
    replacePendingCreate(restoredSession?.pending ?? null)
    setJob(restoredSession?.jobId ? pendingJob(restoredSession.jobId) : null)
    setActionError('')
    setPollError('')
    setPollRevision(0)
    setCreatingJob(false)
    setDownloading(false)
    return () => {
      const controller = actionController.current
      actionController.current = null
      controller?.abort()
    }
  }, [mallID, restoredSession])

  useEffect(() => {
    if (!polledJobID || !polledJobStatus || mallWeatherExportJobTerminal(polledJobStatus)) return
    const jobID = polledJobID
    const controller = new AbortController()
    let timer = 0
    let attempt = 0
    let transientFailureCount = 0
    const schedulePoll = (delayMilliseconds: number) => {
      timer = window.setTimeout(() => void poll(), delayMilliseconds)
    }
    const retryTransientFailure = (message: string) => {
      transientFailureCount++
      if (transientFailureCount > mallWeatherExportMaximumTransientPollRetries) {
        setPollError(`${message}；已停止自动刷新，可点击“继续查询”。`)
        return
      }
      schedulePoll(mallWeatherExportPollRetryDelayMilliseconds(transientFailureCount))
    }
    const poll = async () => {
      attempt++
      if (attempt > mallWeatherExportMaximumPollAttempts) {
        setPollError('导出任务仍在处理，已停止自动刷新；可点击“继续查询”。')
        return
      }
      const requestController = new AbortController()
      const cancelRequest = () => requestController.abort()
      if (controller.signal.aborted) requestController.abort()
      else controller.signal.addEventListener('abort', cancelRequest, { once: true })
      try {
        const response = await waitForMallWeatherExportRequest(
          client(mallWeatherExportJobPath(jobID), {
            method: 'GET', showResult: false, silentLoading: true, signal: requestController.signal,
          }),
          requestController,
        )
        if (controller.signal.aborted) return
        const failureAction = response.ok ? null : mallWeatherExportPollFailureAction(response.status)
        if (failureAction === 'forget') {
          clearSession()
          setJob(null)
          setPollError(response.status === 422
            ? '原导出任务记录已失效，已清理本地记录；可以重新生成 Excel。'
            : '原导出任务已不存在，已清理本地记录；可以重新生成 Excel。')
          return
        }
        if (failureAction === 'retry') {
          retryTransientFailure(exportRequestError(
            response.status,
            '导出进度查询暂时失败',
            '当前账号缺少 weather.export 权限',
          ))
          return
        }
        if (!response.ok) throw new Error(exportRequestError(
          response.status,
          '导出进度查询失败',
          '当前账号缺少 weather.export 权限',
        ))
        const nextJob = parseMallWeatherExportJob(response.data)
        if (!nextJob || nextJob.jobId !== jobID) throw new Error('导出任务响应格式不正确，请联系管理员')
        transientFailureCount = 0
        setJob(nextJob)
        setPollError('')
        if (nextJob.status === 'FAILED' || nextJob.status === 'CANCELLED' || nextJob.status === 'EXPIRED') {
          clearSession()
        } else {
          persistSession({
            pending: null,
            jobId: nextJob.jobId,
          })
        }
        if (!mallWeatherExportJobTerminal(nextJob.status)) {
          schedulePoll(mallWeatherExportPollIntervalMilliseconds)
        }
      } catch (error) {
        if (controller.signal.aborted) return
        if (error instanceof MallWeatherExportRequestTimeoutError) {
          retryTransientFailure('导出进度查询超时')
          return
        }
        setPollError(error instanceof Error ? error.message : '导出进度查询失败')
      } finally {
        controller.signal.removeEventListener('abort', cancelRequest)
      }
    }
    schedulePoll(mallWeatherExportPollIntervalMilliseconds)
    return () => {
      window.clearTimeout(timer)
      controller.abort()
    }
  }, [clearSession, client, persistSession, polledJobID, polledJobStatus, pollRevision])

  async function createJob() {
    if (actionController.current || creatingJob || downloading) return
    let pending = pendingCreate.current
    if (pending && !mallWeatherExportRequestMatches(pending.body, mallID)) {
      setActionError('保留的原请求不属于当前商场；请先放弃原请求后重新生成。')
      return
    }
    if (!pending) {
      try {
        pending = {
          key: mallWeatherExportKey(),
          body: mallWeatherExportCreateRequest(mallID),
        }
      } catch {
        setActionError('当前商场或导出范围无效，请刷新后重试')
        return
      }
      replacePendingCreate(pending)
      setJob(null)
      persistSession({ pending, jobId: '' })
    }
    const controller = new AbortController()
    actionController.current = controller
    setCreatingJob(true)
    setActionError('')
    setPollError('')
    try {
      const response = await waitForMallWeatherExportRequest(
        client('/v1/weather-exports', {
          method: 'POST', body: pending.body, headers: { 'Idempotency-Key': pending.key },
          showResult: false, silentLoading: true, signal: controller.signal,
        }),
        controller,
      )
      if (controller.signal.aborted) return
      const disposition = mallWeatherExportCreateDisposition(response, pending.body)
      if (disposition.kind !== 'accepted') {
        if (response.status === 409) {
          throw new Error('导出请求正在处理或发生冲突；已保留原请求，请点击“重试原请求”确认结果。')
        }
        if (!response.ok && response.status !== 0 && response.status !== 408 && response.status < 500) {
          throw new Error(`${exportRequestError(
            response.status,
            '天气导出任务创建失败',
            '当前账号缺少 weather.export 权限',
            response.data,
          )}；已保留原请求，可修复问题后重试或主动放弃。`)
        }
        if (!response.ok) {
          throw new Error('导出请求结果暂不确定，已保留原请求；请点击“重试原请求”。')
        }
        throw new Error('导出任务响应无法确认，已保留原请求；请勿生成新请求并联系管理员。')
      }
      const result = disposition.result
      replacePendingCreate(null)
      const acceptedJob: MallWeatherExportJob = {
        jobId: result.jobId,
        profileId: result.profileId,
        profileVersion: result.profileVersion,
        status: result.status,
        totalRows: result.estimatedRows,
        processedRows: 0,
        currentSheet: '',
        cancelRequested: false,
        fileSizeBytes: 0,
        errorMessageSafe: '',
      }
      setJob(acceptedJob)
      persistSession({
        pending: null,
        jobId: acceptedJob.jobId,
      })
    } catch (error) {
      if (error instanceof MallWeatherExportRequestTimeoutError) {
        setActionError('导出请求超时，结果暂不确定；已保留原请求，请点击“重试原请求”。')
      } else if (!controller.signal.aborted) {
        setActionError(error instanceof Error ? error.message : '天气导出任务创建失败')
      }
    } finally {
      if (actionController.current === controller) {
        actionController.current = null
        setCreatingJob(false)
      }
    }
  }

  async function downloadResult() {
    if (!job || job.status !== 'SUCCEEDED' || actionController.current || downloading) return
    const controller = new AbortController()
    actionController.current = controller
    setDownloading(true)
    setActionError('')
    try {
      const response = await waitForMallWeatherExportDownload(
        downloadFile(
          mallWeatherExportContentPath(job.jobId),
          `mall_weather_export_${job.jobId}.xlsx`,
          controller.signal,
        ),
        controller,
      )
      if (controller.signal.aborted) return
      if (!response.ok && response.status === 409) {
        const safeMessage = parseMallWeatherExportSafeErrorMessage(response.data)
        if (safeMessage === '天气导出文件已过期') {
          setJob({ ...job, status: 'EXPIRED' })
          clearSession()
          throw new Error('天气导出文件已过期，请重新生成')
        }
        if (safeMessage === '天气导出文件尚未生成') {
          setJob({ ...job, status: 'RUNNING' })
          persistSession({ pending: null, jobId: job.jobId })
          setPollRevision((current) => current + 1)
          throw new Error('天气导出文件尚未生成，已恢复任务查询，请稍后重试')
        }
      }
      if (!response.ok) throw new Error(exportRequestError(
        response.status,
        'Excel 文件下载失败',
        '当前账号缺少 weather.export 权限',
        response.data,
      ))
      persistSession({ pending: null, jobId: job.jobId })
    } catch (error) {
      if (error instanceof MallWeatherExportDownloadTimeoutError) {
        setActionError('Excel 文件下载超时，请检查网络后重试')
      } else if (!controller.signal.aborted) {
        setActionError(error instanceof Error ? error.message : 'Excel 文件下载失败')
      }
    } finally {
      if (actionController.current === controller) {
        actionController.current = null
        setDownloading(false)
      }
    }
  }

  const progress = job ? mallWeatherExportProgress(job) : 0
  const pollingPaused = Boolean(pollError && job && !mallWeatherExportJobTerminal(job.status))

  function abandonPendingCreate() {
    replacePendingCreate(null)
    clearSession()
    setActionError('')
    setPollError('')
    setJob(null)
  }

  return (
    <section className="workbench-panel mall-weather-export-panel" id="mall-weather-export" tabIndex={-1}
      aria-busy={creatingJob || downloading || mallWeatherExportPollingActive(job?.status, pollingPaused)}>
      <div className="mall-weather-section-title">
        <div><strong>导出 Excel</strong><span>{mallName} · 当前商场天气数据</span></div>
        <FileSpreadsheet aria-hidden="true" />
      </div>

      <div className="mall-weather-request-state">
        <strong>完整天气 Excel</strong>
        <span>固定导出商场资料、实况、约 1 km 分钟降水、逐小时与逐日预报、预警和生活指数。</span>
        <button className="primary" type="button" onClick={() => void createJob()}
          disabled={creatingJob || downloading || Boolean(job && !mallWeatherExportJobTerminal(job.status))}>
          {creatingJob ? '提交中' : pendingCreate.current ? '重试原请求' : '生成 Excel'}
        </button>
      </div>
      {pendingCreate.current && (
        <button type="button" onClick={abandonPendingCreate} disabled={creatingJob}>放弃原请求</button>
      )}

      {job && (
        <div className="mall-weather-export-progress">
          <div role="status" aria-live="polite" aria-atomic="true">
            <strong>{exportStatusLabel(job.status)}</strong><span>{job.processedRows} / {job.totalRows} 行 · {progress}%</span>
          </div>
          <progress max="100" value={progress} aria-label="天气 Excel 生成进度">{progress}%</progress>
          {job.currentSheet && <small>正在处理：{job.currentSheet}</small>}
          {job.status === 'FAILED' && <p className="mall-weather-action-message error" role="alert">{job.errorMessageSafe || '导出文件生成失败，请重试'}</p>}
          {job.status === 'CANCELLED' && <p className="mall-weather-action-message error" role="alert">导出任务已取消</p>}
          {job.status === 'EXPIRED' && <p className="mall-weather-action-message error" role="alert">导出文件已过期，请重新生成</p>}
          {job.status === 'SUCCEEDED' && (
            <button className="primary" type="button" onClick={() => void downloadResult()} disabled={downloading}>
              <Download aria-hidden="true" />{downloading ? '正在下载文件' : '下载 Excel'}
            </button>
          )}
        </div>
      )}
      {pollError && (
        <div className="mall-weather-action-message error" role="alert">
          {pollError}
          {job && !mallWeatherExportJobTerminal(job.status) && (
            <div className="mall-weather-export-recovery-actions">
              <button type="button" onClick={() => {
                setPollError('')
                setPollRevision((current) => current + 1)
              }}><RefreshCcw aria-hidden="true" />继续查询</button>
              <button type="button" onClick={abandonPendingCreate}>清除本地任务记录</button>
            </div>
          )}
        </div>
      )}
      {storageWarning && <p className="mall-weather-action-message" role="status">{storageWarning}</p>}
      {actionError && <p className="mall-weather-action-message error" role="alert">{actionError}</p>}
    </section>
  )
}

function exportStatusLabel(status: MallWeatherExportJob['status']) {
  const labels: Record<MallWeatherExportJob['status'], string> = {
    PENDING: '等待生成',
    RUNNING: '正在生成',
    SUCCEEDED: 'Excel 已生成',
    FAILED: '生成失败',
    CANCELLED: '任务已取消',
    EXPIRED: '文件已过期',
  }
  return labels[status]
}

function pendingJob(jobId: string): MallWeatherExportJob {
  return {
    jobId,
    profileId: 1,
    profileVersion: 1,
    status: 'PENDING',
    totalRows: 0,
    processedRows: 0,
    currentSheet: '',
    cancelRequested: false,
    fileSizeBytes: 0,
    errorMessageSafe: '',
  }
}

function exportRequestError(status: number, fallback: string, forbiddenMessage: string, payload?: unknown) {
  if (status === 0) return '无法连接服务，请检查网络后重试'
  const safeMessage = parseMallWeatherExportSafeErrorMessage(payload)
  if (safeMessage) return safeMessage
  if (status === 403) return forbiddenMessage
  if (status === 404) return '导出任务不存在，请刷新后重试'
  if (status === 409) return '导出任务状态已变化，请刷新后重试'
  if (status === 422) return '导出范围未通过校验'
  return `${fallback}（HTTP ${status}）`
}
