import { Download, FileSpreadsheet, RefreshCcw } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  clearMallWeatherExportSession,
  loadMallWeatherExportSession,
  mallWeatherDefaultExportProfileRequest,
  mallWeatherExportCreateRequest,
  mallWeatherExportDownloadPath,
  mallWeatherExportJobPath,
  mallWeatherExportJobTerminal,
  mallWeatherExportKey,
  mallWeatherExportMaximumPollAttempts,
  mallWeatherExportPollIntervalMilliseconds,
  mallWeatherExportProfilesPath,
  mallWeatherExportProgress,
  mallWeatherExportRequestMatches,
  parseMallWeatherExportCreateResult,
  parseMallWeatherExportDownload,
  parseMallWeatherExportJob,
  parseMallWeatherExportProfile,
  parseMallWeatherExportProfilePage,
  saveMallWeatherExportSession,
  selectMallWeatherExportProfile,
  type MallWeatherExportJob,
  type MallWeatherExportPendingCreate,
  type MallWeatherExportProfile,
} from './mallWeatherExport'

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

type MallWeatherExportPanelProps = {
  actorID: string
  mallID: number
  mallName: string
  client: WeatherExportAPIClient
  onDownloadURL?: (url: string) => void
}

type LoadState = 'loading' | 'success' | 'error'

const maximumProfilePages = 5

export function MallWeatherExportPanel({ actorID, mallID, mallName, client, onDownloadURL }: MallWeatherExportPanelProps) {
  const restoredSession = useMemo(
    () => loadMallWeatherExportSession(actorID, mallID, window.sessionStorage),
    [actorID, mallID],
  )
  const [profiles, setProfiles] = useState<MallWeatherExportProfile[]>([])
  const [profileState, setProfileState] = useState<LoadState>('loading')
  const [selectedProfileID, setSelectedProfileID] = useState(0)
  const [profileError, setProfileError] = useState('')
  const [creatingProfile, setCreatingProfile] = useState(false)
  const [creatingJob, setCreatingJob] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [job, setJob] = useState<MallWeatherExportJob | null>(
    () => restoredSession?.jobId ? pendingJob(restoredSession.jobId) : null,
  )
  const [actionError, setActionError] = useState('')
  const [pollError, setPollError] = useState('')
  const [pollRevision, setPollRevision] = useState(0)
  const [, setPendingRevision] = useState(0)
  const actionController = useRef<AbortController | null>(null)
  const pendingCreate = useRef<MallWeatherExportPendingCreate | null>(restoredSession?.pending ?? null)

  const selectedProfile = useMemo(
    () => profiles.find((profile) => profile.id === selectedProfileID) ?? null,
    [profiles, selectedProfileID],
  )
  const polledJobID = job?.jobId ?? ''
  const polledJobStatus = job?.status

  function replacePendingCreate(next: MallWeatherExportPendingCreate | null) {
    pendingCreate.current = next
    setPendingRevision((current) => current + 1)
  }

  const loadProfiles = useCallback(async (signal?: AbortSignal) => {
    setProfileState('loading')
    setProfileError('')
    const collected: MallWeatherExportProfile[] = []
    const seenIDs = new Set<number>()
    const seenCursors = new Set<string>()
    let cursor = ''
    try {
      for (let pageNumber = 0; pageNumber < maximumProfilePages; pageNumber++) {
        const response = await client(mallWeatherExportProfilesPath(cursor), {
          method: 'GET', showResult: false, silentLoading: true, signal,
        })
        if (signal?.aborted) return
        if (!response.ok) throw new Error(exportRequestError(
          response.status,
          '天气导出方案加载失败',
          '当前账号缺少 weather.export 权限',
        ))
        const page = parseMallWeatherExportProfilePage(response.data)
        if (!page || page.items.some((profile) => !profile.enabled)) {
          throw new Error('天气导出方案响应格式不正确，请联系管理员')
        }
        for (const profile of page.items) {
          if (seenIDs.has(profile.id)) throw new Error('天气导出方案分页数据重复，请联系管理员')
          seenIDs.add(profile.id)
          collected.push(profile)
        }
        if (!page.pagination.nextCursor) {
          setProfiles(collected)
          setSelectedProfileID((current) => {
            if (collected.some((profile) => profile.id === current)) return current
            const restoredProfileID = pendingCreate.current?.body.profileId ?? 0
            if (collected.some((profile) => profile.id === restoredProfileID)) return restoredProfileID
            return selectMallWeatherExportProfile(collected)?.id ?? 0
          })
          setProfileState('success')
          return collected
        }
        if (seenCursors.has(page.pagination.nextCursor)) throw new Error('天气导出方案分页游标重复，请联系管理员')
        seenCursors.add(page.pagination.nextCursor)
        cursor = page.pagination.nextCursor
      }
      throw new Error('天气导出方案数量超过安全上限，请联系管理员')
    } catch (error) {
      if (signal?.aborted) return
      setProfileState('error')
      setProfileError(error instanceof Error ? error.message : '天气导出方案加载失败')
      return null
    }
  }, [client])

  const findDisabledDefaultProfile = useCallback(async (signal: AbortSignal) => {
    const seenCursors = new Set<string>()
    let cursor = ''
    for (let pageNumber = 0; pageNumber < maximumProfilePages; pageNumber++) {
      const response = await client(mallWeatherExportProfilesPath(cursor, false), {
        method: 'GET', showResult: false, silentLoading: true, signal,
      })
      if (signal.aborted) return null
      if (!response.ok) throw new Error(exportRequestError(
        response.status,
        '停用天气导出方案加载失败',
        '当前账号缺少 weather.export 权限',
      ))
      const page = parseMallWeatherExportProfilePage(response.data)
      if (!page || page.items.some((profile) => profile.enabled)) {
        throw new Error('停用天气导出方案响应格式不正确，请联系管理员')
      }
      const defaultProfile = page.items.find((profile) => profile.code === 'mall_weather_full')
      if (defaultProfile) return defaultProfile
      if (!page.pagination.nextCursor) return null
      if (seenCursors.has(page.pagination.nextCursor)) {
        throw new Error('停用天气导出方案分页游标重复，请联系管理员')
      }
      seenCursors.add(page.pagination.nextCursor)
      cursor = page.pagination.nextCursor
    }
    throw new Error('停用天气导出方案数量超过安全上限，请联系管理员')
  }, [client])

  useEffect(() => {
    const controller = new AbortController()
    actionController.current?.abort()
    actionController.current = null
    replacePendingCreate(restoredSession?.pending ?? null)
    setProfiles([])
    setSelectedProfileID(0)
    setJob(restoredSession?.jobId ? pendingJob(restoredSession.jobId) : null)
    setActionError('')
    setPollError('')
    setPollRevision(0)
    setCreatingProfile(false)
    setCreatingJob(false)
    setDownloading(false)
    void loadProfiles(controller.signal)
    return () => {
      controller.abort()
      actionController.current?.abort()
    }
  }, [loadProfiles, mallID, restoredSession])

  useEffect(() => {
    if (!polledJobID || !polledJobStatus || mallWeatherExportJobTerminal(polledJobStatus)) return
    const jobID = polledJobID
    const controller = new AbortController()
    let timer = 0
    let attempt = 0
    const poll = async () => {
      attempt++
      if (attempt > mallWeatherExportMaximumPollAttempts) {
        setPollError('导出任务仍在处理，已停止自动刷新；可点击“继续查询”。')
        return
      }
      try {
        const response = await client(mallWeatherExportJobPath(jobID), {
          method: 'GET', showResult: false, silentLoading: true, signal: controller.signal,
        })
        if (controller.signal.aborted) return
        if (!response.ok && response.status === 404) {
          clearMallWeatherExportSession(actorID, mallID, window.sessionStorage)
          setJob(null)
          setPollError('原导出任务已不存在，已清理本地记录；可以重新生成 Excel。')
          return
        }
        if (!response.ok) throw new Error(exportRequestError(
          response.status,
          '导出进度查询失败',
          '当前账号缺少 weather.export 权限',
        ))
        const nextJob = parseMallWeatherExportJob(response.data)
        if (!nextJob || nextJob.jobId !== jobID) throw new Error('导出任务响应格式不正确，请联系管理员')
        setJob(nextJob)
        setPollError('')
        if (nextJob.status === 'FAILED' || nextJob.status === 'CANCELLED' || nextJob.status === 'EXPIRED') {
          clearMallWeatherExportSession(actorID, mallID, window.sessionStorage)
        } else {
          saveMallWeatherExportSession(actorID, mallID, {
            pending: null,
            jobId: nextJob.jobId,
          }, window.sessionStorage)
        }
        if (!mallWeatherExportJobTerminal(nextJob.status)) {
          timer = window.setTimeout(() => void poll(), mallWeatherExportPollIntervalMilliseconds)
        }
      } catch (error) {
        if (controller.signal.aborted) return
        setPollError(error instanceof Error ? error.message : '导出进度查询失败')
      }
    }
    timer = window.setTimeout(() => void poll(), mallWeatherExportPollIntervalMilliseconds)
    return () => {
      window.clearTimeout(timer)
      controller.abort()
    }
  }, [actorID, client, mallID, polledJobID, polledJobStatus, pollRevision])

  function retryProfiles() {
    actionController.current?.abort()
    const controller = new AbortController()
    actionController.current = controller
    void loadProfiles(controller.signal)
  }

  async function createDefaultProfile() {
    actionController.current?.abort()
    const controller = new AbortController()
    actionController.current = controller
    setCreatingProfile(true)
    setActionError('')
    try {
      let response = await client('/v1/weather-export-profiles', {
        method: 'POST', body: mallWeatherDefaultExportProfileRequest(), showResult: false, silentLoading: true,
        signal: controller.signal,
      })
      if (controller.signal.aborted) return
      if (!response.ok && response.status === 409) {
        const refreshed = await loadProfiles(controller.signal)
        if (controller.signal.aborted) return
        if (!refreshed) throw new Error('默认方案发生冲突，且暂时无法刷新可用方案。')
        let existingDefault = refreshed.find((profile) => profile.code === 'mall_weather_full') ?? null
        if (!existingDefault) {
          existingDefault = await findDisabledDefaultProfile(controller.signal)
          if (controller.signal.aborted) return
        }
        if (!existingDefault) {
          throw new Error('默认方案编码发生冲突，但未找到可恢复的方案，请联系管理员。')
        }
        response = await client('/v1/weather-export-profiles', {
          method: 'POST',
          body: mallWeatherDefaultExportProfileRequest(existingDefault.version),
          showResult: false,
          silentLoading: true,
          signal: controller.signal,
        })
        if (controller.signal.aborted) return
      }
      if (!response.ok) {
        throw new Error(exportRequestError(
          response.status,
          '默认导出方案创建失败',
          '当前账号缺少 weather.config.manage 权限',
        ))
      }
      const profile = parseMallWeatherExportProfile(response.data)
      if (!profile || !profile.enabled) throw new Error('默认导出方案响应格式不正确，请联系管理员')
      setProfiles((current) => [profile, ...current.filter((item) => item.id !== profile.id)])
      setSelectedProfileID(profile.id)
      setProfileState('success')
    } catch (error) {
      if (!controller.signal.aborted) setActionError(error instanceof Error ? error.message : '默认导出方案创建失败')
    } finally {
      if (!controller.signal.aborted) setCreatingProfile(false)
    }
  }

  async function createJob() {
    if (!selectedProfile) return
    let pending = pendingCreate.current
    if (pending && !mallWeatherExportRequestMatches(pending.body, selectedProfile, mallID)) {
      setActionError('保留的原请求使用旧方案版本；请先放弃原请求，再使用当前方案生成 Excel。')
      return
    }
    if (!pending) {
      try {
        pending = {
          key: mallWeatherExportKey(),
          body: mallWeatherExportCreateRequest(selectedProfile, mallID),
        }
      } catch {
        setActionError('当前商场或导出方案无效，请刷新后重试')
        return
      }
      replacePendingCreate(pending)
      setJob(null)
      saveMallWeatherExportSession(actorID, mallID, { pending, jobId: '' }, window.sessionStorage)
    }
    actionController.current?.abort()
    const controller = new AbortController()
    actionController.current = controller
    setCreatingJob(true)
    setActionError('')
    setPollError('')
    try {
      const response = await client('/v1/weather-exports', {
        method: 'POST', body: pending.body, headers: { 'Idempotency-Key': pending.key },
        showResult: false, silentLoading: true, signal: controller.signal,
      })
      if (controller.signal.aborted) return
      if (!response.ok) {
        if (response.status === 409) {
          const refreshed = await loadProfiles(controller.signal)
          if (controller.signal.aborted) return
          const exactProfile = refreshed?.find((profile) =>
            profile.id === pending.body.profileId && profile.version === pending.body.expectedProfileVersion)
          if (exactProfile) {
            setSelectedProfileID(exactProfile.id)
            throw new Error('导出请求正在处理或发生冲突，原方案版本仍可用；请重试原请求确认结果。')
          }
          throw new Error(refreshed
            ? '原导出方案已删除、停用或升版。原请求仍保留；请放弃原请求后使用最新方案。'
            : '导出请求发生冲突，且暂时无法确认方案版本；请稍后刷新方案。')
        }
        const uncertain = response.status === 0 || response.status === 408 || response.status >= 500
        if (uncertain) {
          throw new Error('导出请求结果暂不确定，已保留原请求；请点击“重试原请求”。')
        }
        replacePendingCreate(null)
        clearMallWeatherExportSession(actorID, mallID, window.sessionStorage)
        throw new Error(exportRequestError(
          response.status,
          '天气导出任务创建失败',
          '当前账号缺少 weather.export 权限',
        ))
      }
      const result = parseMallWeatherExportCreateResult(response.data)
      if (!result || result.profileId !== pending.body.profileId ||
        result.profileVersion !== pending.body.expectedProfileVersion) {
        throw new Error('导出任务已提交，但响应格式不正确；请勿重复修改方案并联系管理员')
      }
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
      saveMallWeatherExportSession(actorID, mallID, {
        pending: null,
        jobId: acceptedJob.jobId,
      }, window.sessionStorage)
    } catch (error) {
      if (!controller.signal.aborted) setActionError(error instanceof Error ? error.message : '天气导出任务创建失败')
    } finally {
      if (!controller.signal.aborted) setCreatingJob(false)
    }
  }

  async function downloadResult() {
    if (!job || job.status !== 'SUCCEEDED') return
    actionController.current?.abort()
    const controller = new AbortController()
    actionController.current = controller
    setDownloading(true)
    setActionError('')
    try {
      const response = await client(mallWeatherExportDownloadPath(job.jobId), {
        method: 'GET', showResult: false, silentLoading: true, signal: controller.signal,
      })
      if (controller.signal.aborted) return
      if (!response.ok) throw new Error(exportRequestError(
        response.status,
        '下载链接生成失败',
        '当前账号缺少 weather.export 权限',
      ))
      const download = parseMallWeatherExportDownload(response.data)
      if (!download) throw new Error('下载链接响应格式不正确，请联系管理员')
      if (onDownloadURL) onDownloadURL(download.url)
      else window.location.assign(download.url)
    } catch (error) {
      if (!controller.signal.aborted) setActionError(error instanceof Error ? error.message : '下载链接生成失败')
    } finally {
      if (!controller.signal.aborted) setDownloading(false)
    }
  }

  const progress = job ? mallWeatherExportProgress(job) : 0
  const hasForecastDatasets = selectedProfile ? profileHasForecastDatasets(selectedProfile) : false
  const hasCompleteExportProfile = profiles.some(profileHasCompleteWeatherData)

  function abandonPendingCreate() {
    replacePendingCreate(null)
    clearMallWeatherExportSession(actorID, mallID, window.sessionStorage)
    setActionError('')
    setPollError('')
    setJob(null)
  }

  return (
    <section className="workbench-panel mall-weather-export-panel" id="mall-weather-export" tabIndex={-1}
      aria-busy={profileState === 'loading' || creatingProfile || creatingJob || downloading ||
        Boolean(job && !mallWeatherExportJobTerminal(job.status))}>
      <div className="mall-weather-section-title">
        <div><strong>导出 Excel</strong><span>{mallName} · 当前商场天气数据</span></div>
        <FileSpreadsheet aria-hidden="true" />
      </div>

      {profileState === 'loading' && <p role="status">正在加载可用导出方案…</p>}
      {profileState === 'error' && (
        <div className="mall-weather-action-message error" role="alert">
          {profileError}
          <button type="button" onClick={retryProfiles}>重试加载</button>
        </div>
      )}
      {profileState === 'success' && profiles.length === 0 && (
        <div className="empty-state" role="status">
          <strong>没有可用天气导出方案</strong>
          <span>可创建包含商场、实况、约 1 km 分辨率分钟降水、小时及逐日预报、预警和生活指数的默认方案。</span>
          <button className="primary" type="button" onClick={() => void createDefaultProfile()}
            disabled={creatingProfile || Boolean(pendingCreate.current)}>
            {creatingProfile ? '创建中' : '创建默认方案'}
          </button>
        </div>
      )}
      {profiles.length > 0 && (
        <div className="mall-weather-refresh-form">
          <label>
            <span>导出方案</span>
            <select value={selectedProfileID} onChange={(event) => {
              setSelectedProfileID(Number(event.currentTarget.value))
              setJob(null)
              clearMallWeatherExportSession(actorID, mallID, window.sessionStorage)
              setActionError('')
              setPollError('')
            }} disabled={Boolean(pendingCreate.current) || creatingJob ||
              Boolean(job && !mallWeatherExportJobTerminal(job.status))}>
              {profiles.map((profile) => (
                <option key={profile.id} value={profile.id}>{profile.name}（v{profile.version}）</option>
              ))}
            </select>
            <small>{hasForecastDatasets ? '包含约 1 km 分辨率分钟降水和逐小时预报' : '该方案未同时包含分钟降水和逐小时预报'}</small>
          </label>
          <button className="primary" type="button" onClick={() => void createJob()}
            disabled={!selectedProfile || creatingJob || Boolean(job && !mallWeatherExportJobTerminal(job.status))}>
            {creatingJob ? '提交中' : pendingCreate.current ? '重试原请求' : '生成 Excel'}
          </button>
        </div>
      )}
      {profileState === 'success' && profiles.length > 0 && !hasCompleteExportProfile && (
        <div className="mall-weather-request-state">
          <strong>当前方案不含完整天气数据</strong>
          <span>可创建或恢复包含分钟降水、逐小时、逐日、预警和生活指数的完整默认方案。</span>
          <button type="button" onClick={() => void createDefaultProfile()}
            disabled={creatingProfile || Boolean(pendingCreate.current)}>
            {creatingProfile ? '恢复中' : '创建或恢复完整方案'}
          </button>
        </div>
      )}
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
              <Download aria-hidden="true" />{downloading ? '正在生成链接' : '下载 Excel'}
            </button>
          )}
        </div>
      )}
      {pollError && (
        <div className="mall-weather-action-message error" role="alert">
          {pollError}
          {job && !mallWeatherExportJobTerminal(job.status) && (
            <button type="button" onClick={() => {
              setPollError('')
              setPollRevision((current) => current + 1)
            }}><RefreshCcw aria-hidden="true" />继续查询</button>
          )}
        </div>
      )}
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

function profileHasForecastDatasets(profile: MallWeatherExportProfile) {
  const kinds = new Set(profile.datasets.map((dataset) => dataset.kind))
  return kinds.has('minutely') && kinds.has('hourly')
}

function profileHasCompleteWeatherData(profile: MallWeatherExportProfile) {
  const kinds = new Set(profile.datasets.map((dataset) => dataset.kind))
  return ['malls', 'realtime', 'minutely', 'hourly', 'daily', 'alerts', 'life_indices']
    .every((kind) => kinds.has(kind as MallWeatherExportProfile['datasets'][number]['kind']))
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

function exportRequestError(status: number, fallback: string, forbiddenMessage: string) {
  if (status === 0) return '无法连接服务，请检查网络后重试'
  if (status === 403) return forbiddenMessage
  if (status === 404) return '导出方案或任务不存在，请刷新后重试'
  if (status === 409) return '导出方案或任务状态已变化，请刷新后重试'
  if (status === 422) return '导出范围或方案未通过校验'
  return `${fallback}（HTTP ${status}）`
}
