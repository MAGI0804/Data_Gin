import { type FormEvent, type ReactNode, useCallback, useEffect, useRef, useState } from 'react'
import { Database, Download, FileJson, ListChecks, RefreshCcw, Search, Upload } from 'lucide-react'
import type { ApiRequestOptions, ClientResponse, HTTPMethod } from '../api/client'
import { buildExcelMatchJobListQuery, normalizeMonitoringPageNumber, parseMonitoringPage, type MonitoringPagination } from '../monitoringRecords'
import { DataTable, Dialog, Drawer, FeedbackState, FilterToolbar, MetricStrip, PageCanvas, PageHeader, PaginationControls, Section, StatusTag } from '../ui'
import { buildExcelExportConfig, cloneExcelEmptyCellFills, cloneExcelMatchSteps, excelMatchSchemePath, excelFieldSelectOptions, excelModelSelectOptions, selectExcelMatchStepModel, type ExcelMatchFilterConfig, type ExcelEmptyCellFillConfig, type ExcelMatchModel, type ExcelMatchModelField, type ExcelMatchStepConfig } from '../excelMatchConfig'
import { bojunMatchFieldOptions, canDownloadExcelJob, compactText, defaultExcelExportScheme, defaultExcelImportScheme, excelChunkSize, excelJobOperation, excelJobOperationLabel, excelJobPollMaxAttempts, excelJobProgressPercent, excelJobStatusLabel, excelLogLevelLabel, excelMatchFilterOperatorOptions, excelPreviewStat, excelPreviewStatusLabel, exportSchemeDefaults, filterSensitiveExcelModels, formValue, formatDate, formatUnixTime, importSchemeDefaults, isExcelJobActive, isExcelMatchStepComplete, parseExportColumnFormats, readDataField, readList, readObject, replaceExcelJobHistoryItem, sameExcelFile, submitExcelDownloadForm, type ExcelDialogMode, type ExcelExportSchemeConfig, type ExcelImportSchemeConfig, type ExcelMatchJob, type ExcelMatchJobLog, type ExcelMatchPreviewResult, type ExcelMatchScheme, type ExcelUploadRef, type ExcelUploadSession, type ExcelUploadSlot, type PendingSchemeSave } from './excelPageSupport'
import styles from './ExcelMatchPage.module.css'

export type ExcelPageClientOptions = Omit<ApiRequestOptions, 'method'> & { method?: HTTPMethod; showResult?: boolean; silentLoading?: boolean }
export type ExcelPageClient = (path: string, options?: ExcelPageClientOptions) => Promise<ClientResponse>

export function ExcelMatchPage({
  section,
  client,
  token,
  loading,
  refreshVersion,
  setLoading,
  setResult,
  onNavigateToJobs,
}: {
  section: 'jobs' | 'schemes' | 'write'
  client: ExcelPageClient
  token: string
  loading: boolean
  refreshVersion: number
  setLoading: (value: boolean) => void
  setResult: (value: ClientResponse | null) => void
  onNavigateToJobs: () => void
}) {
  const [jobID, setJobID] = useState('')
  const [job, setJob] = useState<ExcelMatchJob | null>(null)
  const [jobDetailOpen, setJobDetailOpen] = useState(false)
  const jobDetailTriggerRef = useRef<HTMLButtonElement | null>(null)
  const [jobHistory, setJobHistory] = useState<ExcelMatchJob[]>([])
  const [jobLogs, setJobLogs] = useState<ExcelMatchJobLog[]>([])
  const [trackingJobID, setTrackingJobID] = useState<number | null>(null)
  const [autoRefreshText, setAutoRefreshText] = useState('')
  const [downloadingJobID, setDownloadingJobID] = useState<number | null>(null)
  const [selectedExportFileName, setSelectedExportFileName] = useState('')
  const [selectedImportFileName, setSelectedImportFileName] = useState('')
  const [selectedClearFileName, setSelectedClearFileName] = useState('')
  const [excelDialog, setExcelDialog] = useState<ExcelDialogMode | null>(null)
  const [previewResult, setPreviewResult] = useState<ExcelMatchPreviewResult | null>(null)
  const [uploadRefs, setUploadRefs] = useState<Partial<Record<ExcelUploadSlot, ExcelUploadRef>>>({})
  const [uploadProgress, setUploadProgress] = useState('')
  const [exportSchemes, setExportSchemes] = useState<ExcelMatchScheme[]>([])
  const [importSchemes, setImportSchemes] = useState<ExcelMatchScheme[]>([])
  const [exportDefaults, setExportDefaults] = useState<ExcelExportSchemeConfig>(defaultExcelExportScheme)
  const [exportSteps, setExportSteps] = useState<ExcelMatchStepConfig[]>(cloneExcelMatchSteps(defaultExcelExportScheme.steps))
  const [exportEmptyCellFills, setExportEmptyCellFills] = useState<ExcelEmptyCellFillConfig[]>(cloneExcelEmptyCellFills(defaultExcelExportScheme.emptyCellFills))
  const [excelModels, setExcelModels] = useState<ExcelMatchModel[]>([])
  const [excelModelsLoading, setExcelModelsLoading] = useState(false)
  const [excelModelsError, setExcelModelsError] = useState('')
  const [importDefaults, setImportDefaults] = useState<ExcelImportSchemeConfig>(defaultExcelImportScheme)
  const [exportFormKey, setExportFormKey] = useState(0)
  const [importFormKey, setImportFormKey] = useState(0)
  const [selectedExportSchemeID, setSelectedExportSchemeID] = useState('')
  const [selectedImportSchemeID, setSelectedImportSchemeID] = useState('')
  const [pendingSchemeDelete, setPendingSchemeDelete] = useState<ExcelMatchScheme | null>(null)
  const [pendingSchemeSave, setPendingSchemeSave] = useState<PendingSchemeSave | null>(null)
  const [schemeSaveError, setSchemeSaveError] = useState('')
  const schemeSaveInFlightRef = useRef(false)
  const [deletingSchemeID, setDeletingSchemeID] = useState<number | null>(null)
  const [pendingWrite, setPendingWrite] = useState<{ slot: 'import' | 'clear'; file: File; config: unknown; message: string } | null>(null)
  const [writeMode, setWriteMode] = useState<'import' | 'clear'>('import')
  const [latestWriteJob, setLatestWriteJob] = useState<ExcelMatchJob | null>(null)
  const [jobQuery, setJobQuery] = useState('')
  const [jobStatus, setJobStatus] = useState('all')
  const [jobOperation, setJobOperation] = useState('all')
  const [appliedJobHistoryFilters, setAppliedJobHistoryFilters] = useState({ keyword: '', status: '', operation: '' })
  const [jobHistoryPage, setJobHistoryPage] = useState(1)
  const [jobHistoryPagination, setJobHistoryPagination] = useState<MonitoringPagination | null>(null)
  const [jobHistoryLoading, setJobHistoryLoading] = useState(false)
  const [jobHistoryError, setJobHistoryError] = useState('')
  const [jobHistoryReloadVersion, setJobHistoryReloadVersion] = useState(0)
  const jobHistoryRequestRef = useRef<AbortController | null>(null)
  const selectedJobProgress = job && job.total_rows > 0
    ? Math.min(100, Math.round(job.processed_rows / job.total_rows * 100))
    : 0
  const pendingSchemeNameConflict = pendingSchemeSave
    ? (pendingSchemeSave.operation === 'export_match' ? exportSchemes : importSchemes)
      .find((scheme) => scheme.name === pendingSchemeSave.name.trim()) ?? null
    : null
  const requestErrorMessage = (response: ClientResponse, fallback: string) => response.error?.message || fallback

  const applyJobResult = useCallback((result: ClientResponse, options: { track?: boolean } = {}) => {
    const nextJob = readObject<ExcelMatchJob>(result, 'job')
    if (nextJob) {
      setJob(nextJob)
      setJobID(String(nextJob.id))
      setJobHistory((current) => replaceExcelJobHistoryItem(current, nextJob))
      if (options.track !== false) {
        setTrackingJobID(isExcelJobActive(nextJob) ? nextJob.id : null)
      }
    }
    setJobLogs(readList<ExcelMatchJobLog>(result, 'logs'))
    return nextJob
  }, [])

  function clearUploadRef(slot: ExcelUploadSlot) {
    setUploadRefs((current) => ({ ...current, [slot]: undefined }))
    setUploadProgress('')
  }

  function resetExcelDialogFiles() {
    setSelectedExportFileName('')
    setSelectedImportFileName('')
    setSelectedClearFileName('')
    setPreviewResult(null)
    setUploadRefs({})
    setUploadProgress('')
  }

  function closeExcelDialog() {
    setPendingWrite(null)
    setPendingSchemeSave(null)
    setSchemeSaveError('')
    setExcelDialog(null)
    resetExcelDialogFiles()
  }

  function buildExportConfig(form: FormData) {
    return buildExcelExportConfig({
      sheetName: formValue(form, 'sheetName').trim() || 'Sheet1',
      steps: exportSteps,
      emptyCellFills: exportEmptyCellFills,
      exportColumnFormats: parseExportColumnFormats(formValue(form, 'exportColumnFormats')),
      batchSize: Number(formValue(form, 'batchSize') || 1000),
    })
  }

  function updateExportStep(index: number, key: Exclude<keyof ExcelMatchStepConfig, 'filters'>, value: string) {
    setExportSteps((current) => current.map((step, stepIndex) => stepIndex === index ? { ...step, [key]: value } : step))
  }

  function selectExportStepModel(index: number, tableName: string) {
    setExportSteps((current) => current.map((step, stepIndex) => stepIndex === index
      ? selectExcelMatchStepModel(step, tableName, excelModels)
      : step))
  }

  function updateExportStepFilter(stepIndex: number, filterIndex: number, key: keyof ExcelMatchFilterConfig, value: string) {
    setExportSteps((current) => current.map((step, currentStepIndex) => currentStepIndex === stepIndex
      ? {
          ...step,
          filters: step.filters.map((filter, currentFilterIndex) => currentFilterIndex === filterIndex ? { ...filter, [key]: value } : filter),
        }
      : step))
  }

  function addExportStepFilter(stepIndex: number) {
    setExportSteps((current) => current.map((step, currentStepIndex) => currentStepIndex === stepIndex
      ? { ...step, filters: [...step.filters, { column: '', op: 'eq', value: '' }] }
      : step))
  }

  function removeExportStepFilter(stepIndex: number, filterIndex: number) {
    setExportSteps((current) => current.map((step, currentStepIndex) => currentStepIndex === stepIndex
      ? { ...step, filters: step.filters.filter((_, currentFilterIndex) => currentFilterIndex !== filterIndex) }
      : step))
  }

  function addExportEmptyCellFill() {
    setExportEmptyCellFills((current) => [...current, { targetColumn: '', sourceColumn: '' }])
  }

  function updateExportEmptyCellFill(index: number, key: keyof ExcelEmptyCellFillConfig, value: string) {
    setExportEmptyCellFills((current) => current.map((fill, fillIndex) => fillIndex === index ? { ...fill, [key]: value } : fill))
  }

  function removeExportEmptyCellFill(index: number) {
    setExportEmptyCellFills((current) => current.filter((_, fillIndex) => fillIndex !== index))
  }

  function addExportStep() {
    setExportSteps((current) => {
      if (current.length >= 20) return current
      return [...current, {
        name: `步骤 ${current.length + 1}`,
        filters: [],
        matchMode: 'field',
        tableName: '',
        matchExcelColumn: current[current.length - 1]?.outputColumnName ?? '',
        dbMatchField: '',
        dbValueField: '',
        outputColumnName: '',
        specExcelColumn: '',
        priceExcelColumn: '',
        qtyExcelColumn: '',
      }]
    })
  }

  function removeExportStep(index: number) {
    setExportSteps((current) => current.length === 1 ? current : current.filter((_, stepIndex) => stepIndex !== index))
  }

  function moveExportStep(index: number, direction: -1 | 1) {
    setExportSteps((current) => {
      const nextIndex = index + direction
      if (nextIndex < 0 || nextIndex >= current.length) return current
      const next = [...current]
      ;[next[index], next[nextIndex]] = [next[nextIndex], next[index]]
      return next
    })
  }

  function buildImportConfig(form: FormData, confirmWrite: boolean) {
    return {
      operation: 'import_update',
      sheetName: formValue(form, 'sheetName').trim() || 'Sheet1',
      tableName: formValue(form, 'tableName').trim(),
      dbMatchField: formValue(form, 'dbMatchField').trim(),
      matchExcelColumn: formValue(form, 'matchExcelColumn').trim(),
      dbWriteField: formValue(form, 'dbWriteField').trim(),
      writeExcelColumn: formValue(form, 'writeExcelColumn').trim(),
      batchSize: Number(formValue(form, 'batchSize') || 1000),
      dryRun: !confirmWrite,
      confirmWrite,
    }
  }

  function buildConfigPayload(uploadId: string, config: unknown) {
    const payload = new FormData()
    payload.append('uploadId', uploadId)
    payload.append('config', JSON.stringify(config))
    return payload
  }

  const fetchSchemes = useCallback(async (operation: 'export_match' | 'import_update') => {
    const response = await client(`/v1/excel-match-jobs/schemes?operation=${operation}`, { method: 'GET', showResult: false, silentLoading: true })
    if (!response.ok) throw new Error(requestErrorMessage(response, '查询 Excel 方案失败'))
    const value = readDataField(response.data, 'schemes')
    return Array.isArray(value) ? (value as ExcelMatchScheme[]) : []
  }, [client])

  const loadExcelModels = useCallback(async () => {
    setExcelModelsLoading(true)
    setExcelModelsError('')
    try {
      const response = await client('/v1/excel-match-jobs/models', { method: 'GET', showResult: false, silentLoading: true })
      if (!response.ok) throw new Error(requestErrorMessage(response, '查询模型与字段目录失败'))
      const value = readDataField(response.data, 'models')
      setExcelModels(Array.isArray(value) ? filterSensitiveExcelModels(value as ExcelMatchModel[]) : [])
    } catch (error) {
      setExcelModelsError(error instanceof Error ? error.message : '查询模型与字段目录失败')
    } finally {
      setExcelModelsLoading(false)
    }
  }, [client])

  const loadSchemes = useCallback(async () => {
    try {
      const [nextExportSchemes, nextImportSchemes] = await Promise.all([
        fetchSchemes('export_match'),
        fetchSchemes('import_update'),
      ])
      setExportSchemes(nextExportSchemes)
      setImportSchemes(nextImportSchemes)
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    }
  }, [fetchSchemes, setResult])

  const loadJobHistory = useCallback(async () => {
    jobHistoryRequestRef.current?.abort()
    const controller = new AbortController()
    jobHistoryRequestRef.current = controller
    setJobHistoryLoading(true)
    setJobHistoryError('')
    const query = buildExcelMatchJobListQuery({ page: jobHistoryPage, pageSize: 20, ...appliedJobHistoryFilters })
    try {
      const response = await client(`/v1/excel-match-jobs?${query}`, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
      if (controller.signal.aborted) return
      const parsed = response.ok ? parseMonitoringPage<ExcelMatchJob>(response.data, 'jobs') : null
      if (parsed) {
        const nextPage = normalizeMonitoringPageNumber(jobHistoryPage, parsed.pagination.totalPages)
        if (nextPage !== jobHistoryPage) {
          setJobHistoryPage(nextPage)
          return
        }
        setJobHistory(parsed.list)
        setJobHistoryPagination(parsed.pagination)
        return
      }
      const legacyItems = readDataField(response.data, 'jobs')
      if (response.ok && Array.isArray(legacyItems)) {
        const pageSize = 20
        if (jobHistoryPage !== 1) {
          setJobHistoryPage(1)
          return
        }
        setJobHistory(legacyItems.slice(0, pageSize) as ExcelMatchJob[])
        setJobHistoryPagination({ page: 1, pageSize, total: legacyItems.length, totalPages: legacyItems.length ? 1 : 0 })
        setJobHistoryError('当前服务暂不支持 Excel 任务分页或筛选，已显示未筛选的兼容数据。')
        return
      }
      setJobHistoryError(response.error?.message || 'Excel 任务历史暂时不可用，请稍后重试。')
    } finally {
      if (!controller.signal.aborted) setJobHistoryLoading(false)
    }
  }, [appliedJobHistoryFilters, client, jobHistoryPage])

  useEffect(() => {
    if (!token) return
    if (section === 'jobs') void loadJobHistory()
    if (section === 'schemes' || section === 'write') void loadSchemes()
    return () => jobHistoryRequestRef.current?.abort()
  }, [jobHistoryReloadVersion, loadJobHistory, loadSchemes, refreshVersion, section, token])

  useEffect(() => {
    if (!token || section !== 'schemes') return
    void loadExcelModels()
  }, [loadExcelModels, section, token])

  useEffect(() => {
    setPendingWrite(null)
    setExcelDialog(null)
    setSelectedExportFileName('')
    setSelectedImportFileName('')
    setSelectedClearFileName('')
    setPreviewResult(null)
    setUploadRefs({})
    setUploadProgress('')
    if (section !== 'jobs') setJobDetailOpen(false)
  }, [section])

  const refreshJobByID = useCallback(async (id: number, options: { silent?: boolean; track?: boolean; signal?: AbortSignal } = {}) => {
    if (!options.silent) setLoading(true)
    try {
      const nextResult = await client(`/v1/excel-match-jobs/${id}`, { method: 'GET', signal: options.signal, showResult: false, silentLoading: true })
      if (!options.silent && !nextResult.ok) setResult(nextResult)
      if (nextResult.ok) {
        applyJobResult(nextResult, { track: options.track })
        if (!options.silent) await loadJobHistory()
        return readObject<ExcelMatchJob>(nextResult, 'job')
      }
      if (options.silent) {
        setAutoRefreshText(`自动刷新失败：${requestErrorMessage(nextResult, '请稍后重试。')}`)
      }
    } catch {
      if (options.signal?.aborted) return null
      if (!options.silent) {
        setResult({ ok: false, status: 0, data: { message: '查询 Excel 任务失败，请稍后重试。' } })
      } else {
        setAutoRefreshText('自动刷新失败，请稍后重试。')
      }
    } finally {
      if (!options.silent) setLoading(false)
    }
    return null
  }, [applyJobResult, client, loadJobHistory, setLoading, setResult])

  async function openJobDetail(id: number, trigger: HTMLButtonElement) {
    jobDetailTriggerRef.current = trigger
    const nextJob = await refreshJobByID(id)
    if (nextJob) {
      setJobDetailOpen(true)
    } else {
      trigger.focus()
    }
  }

  useEffect(() => {
    if (!token || !trackingJobID) return

    let cancelled = false
    let attempts = 0
    let consecutiveFailures = 0
    let timer: number | null = null
    let inFlight = false
    const pollingState = { resumeWhenVisible: false }
    const controller = new AbortController()
    const isPageVisible = () => document.visibilityState !== 'hidden'

    const clearScheduledRefresh = () => {
      if (timer !== null) {
        window.clearTimeout(timer)
        timer = null
      }
    }

    const stopPolling = (message: string) => {
      clearScheduledRefresh()
      setAutoRefreshText(message)
      setTrackingJobID(null)
    }

    const scheduleRefresh = (delayMilliseconds: number) => {
      clearScheduledRefresh()
      if (cancelled || document.visibilityState === 'hidden') return
      if (inFlight) return
      timer = window.setTimeout(() => {
        timer = null
        void refreshTrackedJob()
      }, delayMilliseconds)
    }

    const refreshTrackedJob = async () => {
      if (cancelled || document.visibilityState === 'hidden') return
      if (inFlight) {
        pollingState.resumeWhenVisible = true
        return
      }
      if (attempts >= excelJobPollMaxAttempts) {
        stopPolling(`自动刷新已在 ${excelJobPollMaxAttempts} 次后停止，请手动查询任务状态。`)
        return
      }

      attempts += 1
      inFlight = true
      let nextDelay: number | null = null
      try {
        const nextJob = await refreshJobByID(trackingJobID, { silent: true, signal: controller.signal })
        if (cancelled || controller.signal.aborted) return
        if (!nextJob) {
          consecutiveFailures += 1
          const delayMilliseconds = Math.min(30_000, 2_000 * 2 ** Math.min(consecutiveFailures, 4))
          if (attempts >= excelJobPollMaxAttempts) {
            stopPolling(`自动刷新已在 ${excelJobPollMaxAttempts} 次后停止，请手动查询任务状态。`)
            return
          }
          setAutoRefreshText(`自动刷新失败，将在 ${Math.ceil(delayMilliseconds / 1000)} 秒后重试。`)
          nextDelay = delayMilliseconds
          return
        }

        consecutiveFailures = 0
        setAutoRefreshText(`自动刷新中：任务 #${nextJob.id}，${excelJobStatusLabel(nextJob.status)}，${new Date().toLocaleTimeString()}`)
        if (!isExcelJobActive(nextJob)) {
          setTrackingJobID(null)
          void loadJobHistory()
          return
        }
        nextDelay = 2_000
      } finally {
        inFlight = false
        if (pollingState.resumeWhenVisible && !cancelled && !controller.signal.aborted && isPageVisible()) {
          pollingState.resumeWhenVisible = false
          scheduleRefresh(0)
        } else if (nextDelay !== null) {
          scheduleRefresh(nextDelay)
        }
      }
    }

    void refreshTrackedJob()
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        clearScheduledRefresh()
        setAutoRefreshText('页面已隐藏，任务自动刷新已暂停。')
        return
      }
      if (inFlight) {
        pollingState.resumeWhenVisible = true
        return
      }
      scheduleRefresh(0)
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      cancelled = true
      clearScheduledRefresh()
      controller.abort()
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [loadJobHistory, refreshJobByID, token, trackingJobID])

  async function ensureExcelUpload(slot: ExcelUploadSlot, file: File) {
    const existing = uploadRefs[slot]
    if (existing && sameExcelFile(file, existing)) {
      return existing.uploadId
    }

    const totalChunks = Math.ceil(file.size / excelChunkSize)
    setUploadProgress(`准备上传 ${file.name}，共 ${totalChunks} 个分片`)

    const createResult = await client('/v1/excel-match-jobs/uploads', {
      method: 'POST',
      body: { fileName: file.name, totalChunks },
      showResult: false,
      silentLoading: true,
      retry: false,
    })
    if (!createResult.ok) throw new Error(requestErrorMessage(createResult, '创建分片上传会话失败'))
    const session = readObject<ExcelUploadSession>(createResult, 'upload')
    if (!session?.uploadId) throw new Error('上传会话返回缺少 uploadId')

    for (let index = 0; index < totalChunks; index++) {
      const start = index * excelChunkSize
      const end = Math.min(file.size, start + excelChunkSize)
      const chunkForm = new FormData()
      chunkForm.append('index', String(index))
      chunkForm.append('totalChunks', String(totalChunks))
      chunkForm.append('chunk', file.slice(start, end), `${file.name}.part${index}`)
      setUploadProgress(`上传分片 ${index + 1}/${totalChunks}`)
      const chunkResult = await client(`/v1/excel-match-jobs/uploads/${encodeURIComponent(session.uploadId)}/chunks`, {
        method: 'POST',
        body: chunkForm,
        showResult: false,
        silentLoading: true,
        retry: false,
        timeoutMs: 120_000,
      })
      if (!chunkResult.ok) throw new Error(requestErrorMessage(chunkResult, `上传分片 ${index + 1} 失败`))
    }

    setUploadProgress('合并 Excel 分片')
    const completeResult = await client(`/v1/excel-match-jobs/uploads/${encodeURIComponent(session.uploadId)}/complete`, {
      method: 'POST',
      body: { totalChunks },
      showResult: false,
      silentLoading: true,
      retry: false,
    })
    if (!completeResult.ok) throw new Error(requestErrorMessage(completeResult, '合并 Excel 分片失败'))

    const nextRef = {
      uploadId: session.uploadId,
      fileName: file.name,
      size: file.size,
      lastModified: file.lastModified,
      totalChunks,
    }
    setUploadRefs((current) => ({ ...current, [slot]: nextRef }))
    setUploadProgress(`上传完成：${file.name}`)
    return session.uploadId
  }

  function beginSchemeSave(formElement: HTMLFormElement, operation: 'export_match' | 'import_update', mode: 'current' | 'new') {
    const selectedSchemeID = operation === 'export_match' ? selectedExportSchemeID : selectedImportSchemeID
    const schemes = operation === 'export_match' ? exportSchemes : importSchemes
    const selectedScheme = schemes.find((item) => String(item.id) === selectedSchemeID)
    const form = new FormData(formElement)
    const config = operation === 'export_match'
      ? buildExportConfig(form)
      : buildImportConfig(form, false)

    if (mode === 'current' && selectedScheme?.name) {
      void persistScheme(operation, config, selectedScheme.name)
      return
    }
    setSchemeSaveError('')
    setPendingSchemeSave({ operation, config, name: '', overwriteConfirmed: false })
  }

  async function persistScheme(operation: 'export_match' | 'import_update', config: unknown, name: string) {
    if (schemeSaveInFlightRef.current) return false
    schemeSaveInFlightRef.current = true
    setLoading(true)
    try {
      const nextResult = await client('/v1/excel-match-jobs/schemes', {
        method: 'POST',
        body: { name: name.trim(), operation, config },
        showResult: false,
        silentLoading: true,
        retry: false,
      })
      setResult({ ok: nextResult.ok, status: nextResult.status, data: { message: nextResult.ok ? 'Excel 方案已保存。' : requestErrorMessage(nextResult, '保存 Excel 方案失败。') }, error: nextResult.error })
      if (nextResult.ok) {
        const savedScheme = readObject<ExcelMatchScheme>(nextResult, 'scheme')
        if (savedScheme?.id) {
          if (operation === 'export_match') {
            setSelectedExportSchemeID(String(savedScheme.id))
          } else {
            setSelectedImportSchemeID(String(savedScheme.id))
          }
        }
        await loadSchemes()
      }
      return nextResult.ok
    } catch {
      setResult({ ok: false, status: 0, data: { message: '保存 Excel 方案失败，请稍后重试。' } })
      return false
    } finally {
      schemeSaveInFlightRef.current = false
      setLoading(false)
    }
  }

  async function confirmPendingSchemeSave() {
    if (!pendingSchemeSave || loading) return
    const name = pendingSchemeSave.name.trim()
    if (name.length < 1 || name.length > 100) {
      setSchemeSaveError('方案名称应为 1 至 100 个字符。')
      return
    }
    if (pendingSchemeNameConflict && !pendingSchemeSave.overwriteConfirmed) {
      setSchemeSaveError('存在同类型同名方案；请确认覆盖后再保存。')
      return
    }
    const saved = await persistScheme(pendingSchemeSave.operation, pendingSchemeSave.config, name)
    if (saved) {
      setPendingSchemeSave(null)
      setSchemeSaveError('')
    }
  }

  async function deleteScheme(scheme: ExcelMatchScheme) {
    setDeletingSchemeID(scheme.id)
    try {
      const nextResult = await client(excelMatchSchemePath(scheme.id), {
        method: 'DELETE',
        showResult: false,
        silentLoading: true,
        retry: false,
      })
      setResult({ ok: nextResult.ok, status: nextResult.status, data: { message: nextResult.ok ? 'Excel 方案已删除。' : requestErrorMessage(nextResult, '删除 Excel 方案失败。') }, error: nextResult.error })
      if (!nextResult.ok) return

      if (scheme.operation === 'export_match' && selectedExportSchemeID === String(scheme.id)) {
        applyExportScheme('')
      }
      if (scheme.operation === 'import_update' && selectedImportSchemeID === String(scheme.id)) {
        applyImportScheme('')
      }
      await loadSchemes()
      setPendingSchemeDelete(null)
    } catch {
      setResult({ ok: false, status: 0, data: { message: '删除 Excel 方案失败，请稍后重试。' } })
    } finally {
      setDeletingSchemeID(null)
    }
  }

  function applyExportScheme(schemeID: string) {
    setSelectedExportSchemeID(schemeID)
    if (!schemeID) {
      setExportDefaults(defaultExcelExportScheme)
      setExportSteps(cloneExcelMatchSteps(defaultExcelExportScheme.steps))
      setExportEmptyCellFills(cloneExcelEmptyCellFills(defaultExcelExportScheme.emptyCellFills))
      setExportFormKey((value) => value + 1)
      setPreviewResult(null)
      setSelectedExportFileName('')
      clearUploadRef('export')
      return
    }
    const scheme = exportSchemes.find((item) => String(item.id) === schemeID)
    if (!scheme) return
    const defaults = exportSchemeDefaults(scheme.config)
    setExportDefaults(defaults)
    setExportSteps(cloneExcelMatchSteps(defaults.steps))
    setExportEmptyCellFills(cloneExcelEmptyCellFills(defaults.emptyCellFills))
    setExportFormKey((value) => value + 1)
    setPreviewResult(null)
    setSelectedExportFileName('')
    clearUploadRef('export')
  }

  function applyImportScheme(schemeID: string) {
    setSelectedImportSchemeID(schemeID)
    if (!schemeID) {
      setImportDefaults(defaultExcelImportScheme)
      setImportFormKey((value) => value + 1)
      setSelectedImportFileName('')
      clearUploadRef('import')
      return
    }
    const scheme = importSchemes.find((item) => String(item.id) === schemeID)
    if (!scheme) return
    setImportDefaults(importSchemeDefaults(scheme.config))
    setImportFormKey((value) => value + 1)
    setSelectedImportFileName('')
    clearUploadRef('import')
  }

  async function createExportJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const file = form.get('file')
    if (!(file instanceof File) || file.size === 0) {
      setResult({ ok: false, status: 0, data: '请选择 .xlsx 文件' })
      return
    }

    setLoading(true)
    try {
      const uploadId = await ensureExcelUpload('export', file)
      const payload = buildConfigPayload(uploadId, buildExportConfig(form))
      const nextResult = await client('/v1/excel-match-jobs', {
        method: 'POST',
        body: payload,
        showResult: false,
        silentLoading: true,
        retry: false,
        timeoutMs: 120_000,
      })
      if (nextResult.ok) {
        showCreatedJob(nextResult)
      } else {
        setResult(nextResult)
      }
    } catch {
      setResult({ ok: false, status: 0, data: { message: '创建 Excel 导出任务失败，请稍后重试。' } })
    } finally {
      setLoading(false)
    }
  }

  async function previewExportJob(formElement: HTMLFormElement) {
    const form = new FormData(formElement)
    const file = form.get('file')
    if (!(file instanceof File) || file.size === 0) {
      setResult({ ok: false, status: 0, data: '请选择 .xlsx 文件' })
      return
    }

    setLoading(true)
    try {
      const uploadId = await ensureExcelUpload('export', file)
      const payload = buildConfigPayload(uploadId, buildExportConfig(form))
      const nextResult = await client('/v1/excel-match-jobs/preview', {
        method: 'POST',
        body: payload,
        showResult: false,
        silentLoading: true,
        retry: false,
        timeoutMs: 120_000,
      })
      setResult({ ok: nextResult.ok, status: nextResult.status, data: { message: nextResult.ok ? 'Excel 匹配预览已更新。' : requestErrorMessage(nextResult, '预览 Excel 匹配失败。') }, error: nextResult.error })
      if (nextResult.ok) {
        setPreviewResult(readObject<ExcelMatchPreviewResult>(nextResult, 'preview'))
      }
    } catch {
      setResult({ ok: false, status: 0, data: { message: '预览 Excel 匹配失败，请稍后重试。' } })
    } finally {
      setLoading(false)
    }
  }

  async function createExcelWriteJob(slot: 'import' | 'clear', file: File, config: unknown) {
    setLoading(true)
    try {
      const uploadId = await ensureExcelUpload(slot, file)
      const nextResult = await client('/v1/excel-match-jobs', {
        method: 'POST',
        body: buildConfigPayload(uploadId, config),
        showResult: false,
        silentLoading: true,
        retry: false,
        timeoutMs: 120_000,
      })
      if (nextResult.ok) showCreatedWriteJob(nextResult)
      else setResult(nextResult)
    } catch {
      setResult({ ok: false, status: 0, data: { message: '创建 Excel 写入任务失败，请稍后重试。' } })
    } finally {
      setLoading(false)
    }
  }

  async function confirmPendingWrite() {
    if (!pendingWrite || loading) return
    await createExcelWriteJob(pendingWrite.slot, pendingWrite.file, pendingWrite.config)
    setPendingWrite(null)
  }

  async function createImportJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const file = form.get('file')
    if (!(file instanceof File) || file.size === 0) {
      setResult({ ok: false, status: 0, data: '请选择 .xlsx 文件' })
      return
    }

    const confirmWrite = form.get('confirmWrite') === 'on'
    const writeField = formValue(form, 'dbWriteField').trim()
    const confirmMessage = writeField === 'completed_at'
      ? '确认写入数据库？本次只会填充为空的订单完成时间，不会覆盖已有 completed_at。'
      : '确认写入数据库？本次只会填充空的 matched_docno，不会覆盖已有匹配单号。'
    const config = buildImportConfig(form, confirmWrite)
    if (confirmWrite) {
      setPendingWrite({ slot: 'import', file, config, message: confirmMessage })
      return
    }
    await createExcelWriteJob('import', file, config)
  }

  async function createClearMatchedJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const file = form.get('file')
    if (!(file instanceof File) || file.size === 0) {
      setResult({ ok: false, status: 0, data: '请选择 .xlsx 文件' })
      return
    }

    const confirmWrite = form.get('confirmWrite') === 'on'
    const config = {
      operation: 'clear_matched_docno',
      sheetName: formValue(form, 'sheetName').trim() || 'Sheet1',
      tableName: formValue(form, 'tableName').trim(),
      dbMatchField: formValue(form, 'dbMatchField').trim(),
      matchExcelColumn: formValue(form, 'matchExcelColumn').trim(),
      dbWriteField: 'matched_docno',
      batchSize: Number(formValue(form, 'batchSize') || 1000),
      dryRun: !confirmWrite,
      confirmWrite,
    }
    if (confirmWrite) {
      setPendingWrite({ slot: 'clear', file, config, message: '确认清空命中行的 matched_docno？该操作会将这些订单退回未匹配状态。' })
      return
    }
    await createExcelWriteJob('clear', file, config)
  }

  function showCreatedJob(result: ClientResponse) {
    const createdJob = applyJobResult(result)
    if (createdJob) setJobDetailOpen(true)
    setAutoRefreshText('')
    setResult(null)
    closeExcelDialog()
    setJobHistoryReloadVersion((version) => version + 1)
    onNavigateToJobs()
  }

  function showCreatedWriteJob(result: ClientResponse) {
    const createdJob = applyJobResult(result)
    if (createdJob) {
      setLatestWriteJob(createdJob)
    }
    setAutoRefreshText('')
    setResult(null)
    setJobHistoryReloadVersion((version) => version + 1)
  }

  async function refreshJob() {
    const id = Number(jobID)
    if (!id) {
      setResult({ ok: false, status: 0, data: '请输入任务 ID' })
      return
    }
    const nextJob = await refreshJobByID(id)
    if (nextJob) {
      closeExcelDialog()
      setJobDetailOpen(true)
    }
  }

  async function downloadJob(targetID?: number) {
    const id = targetID ?? Number(jobID || job?.id)
    if (!id) {
      setResult({ ok: false, status: 0, data: '请输入任务 ID' })
      return
    }
    const targetJob = job?.id === id ? job : jobHistory.find((item) => item.id === id) ?? null
    if (targetJob && !canDownloadExcelJob(targetJob)) {
      setResult({ ok: false, status: 0, data: targetJob.download_message || '结果文件尚未上传到OSS，上传成功后才能下载，请稍后刷新任务状态' })
      return
    }

    setDownloadingJobID(id)
    setResult({ ok: true, status: 0, data: `正在提交任务 ${id} 的下载请求，浏览器会接管文件下载。` })
    try {
      submitExcelDownloadForm(id, token)
      setResult({ ok: true, status: 0, data: `任务 ${id} 下载请求已提交，请查看浏览器下载栏。` })
      await loadJobHistory()
    } catch (error) {
      setResult({ ok: false, status: 0, data: error instanceof Error ? error.message : String(error) })
    } finally {
      setDownloadingJobID(null)
    }
  }

  function renderPendingSchemeSave() {
    if (!pendingSchemeSave) return null
    return (
      <form className={styles.viewStack} onSubmit={(event) => { event.preventDefault(); void confirmPendingSchemeSave() }}>
        <p>将保存当前表单的配置快照；取消后可继续编辑，已填写内容不会丢失。</p>
        <label>
          方案名称
          <input
            value={pendingSchemeSave.name}
            maxLength={100}
            data-autofocus
            onChange={(event) => {
              const name = event.currentTarget.value
              setPendingSchemeSave((current) => current ? { ...current, name, overwriteConfirmed: false } : current)
              setSchemeSaveError('')
            }}
          />
        </label>
        {pendingSchemeNameConflict && (
          <label className={styles.checkboxLabel}>
            <input
              type="checkbox"
              checked={pendingSchemeSave.overwriteConfirmed}
              onChange={(event) => setPendingSchemeSave((current) => current ? { ...current, overwriteConfirmed: event.currentTarget.checked } : current)}
            />
            覆盖同类型的“{pendingSchemeNameConflict.name}”方案
          </label>
        )}
        {schemeSaveError && <p className={styles.errorBanner} role="alert">{schemeSaveError}</p>}
        <div className={styles.formActions}>
          <button type="button" disabled={loading} onClick={() => { setPendingSchemeSave(null); setSchemeSaveError('') }}>返回编辑</button>
          <button className={styles.primary} type="submit" disabled={loading}>{loading ? '保存中…' : '保存方案'}</button>
        </div>
      </form>
    )
  }

  function renderExportMatchForm() {
    return (
      <form className={`${styles.uploadForm} ${styles.schemeForm}`} onSubmit={createExportJob} key={exportFormKey}>
        <div className={styles.schemeToolbar}><label className={styles.fileInputLabel}>Excel 文件<input name="file" type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={(event) => { setSelectedExportFileName(event.currentTarget.files?.[0]?.name ?? ''); clearUploadRef('export'); setPreviewResult(null) }} /><span>{selectedExportFileName || '请选择 Excel 文件'}</span></label><Field label="Sheet" name="sheetName" defaultValue={exportDefaults.sheetName} /><div className={styles.schemeToolbarActions}><button type="button" onClick={addExportStep} disabled={exportSteps.length >= 20}>添加步骤</button><button type="button" onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) beginSchemeSave(form, 'export_match', selectedExportSchemeID ? 'current' : 'new') }} disabled={loading}>保存方案</button><button type="button" onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) void previewExportJob(form) }} disabled={loading}><FileJson aria-hidden="true" />预览匹配</button><button className={styles.primary} type="submit" disabled={loading}><Upload aria-hidden="true" />创建导出任务</button></div></div>
        <div className={styles.stepEditor}>
          <div className={styles.stepEditorTitle}><div><strong>自定义匹配流程</strong><span>步骤按顺序执行，并可引用前序步骤输出。</span></div></div>
          {excelModelsLoading && <p className={styles.modeNote}>正在加载模型与字段目录…</p>}
          {excelModelsError && <div className={styles.catalogError} role="alert"><span>模型与字段目录加载失败：{excelModelsError}</span><button type="button" onClick={() => void loadExcelModels()}>重试加载</button></div>}
          {!excelModelsLoading && !excelModelsError && excelModels.length === 0 && <p className={styles.modeNote}>当前数据库没有返回可选择的模型；历史配置仍可查看，但新步骤需要先确认数据库连接和模型表。</p>}
          {exportSteps.map((step, index) => (
            <article className={styles.stepCard} key={`${exportFormKey}-${index}`}>
              <div className={styles.stepHeading}><strong><span className={styles.stepIndex} aria-hidden="true">{index + 1}</span><input className={styles.stepNameInput} name={`step_name_${index}`} value={step.name} onChange={(event) => updateExportStep(index, 'name', event.currentTarget.value)} aria-label={`步骤 ${index + 1} 名称`} required />{index > 0 && <small>数据来源：使用上一步输出（{exportSteps[index - 1]?.outputColumnName || '待配置'}）</small>}</strong><div className={styles.tableActions}><span className={isExcelMatchStepComplete(step) ? styles.stepComplete : styles.stepIncomplete}>{isExcelMatchStepComplete(step) ? '配置完整' : '待完善'}</span><button type="button" onClick={() => moveExportStep(index, -1)} disabled={index === 0}>↑ 上移</button><button type="button" onClick={() => moveExportStep(index, 1)} disabled={index === exportSteps.length - 1}>↓ 下移</button><button className={styles.danger} type="button" onClick={() => removeExportStep(index)} disabled={exportSteps.length === 1}>删除</button></div></div>
              <div className={styles.stepFields}>
                <label>匹配模式<select name={`step_mode_${index}`} value={step.matchMode} onChange={(event) => updateExportStep(index, 'matchMode', event.currentTarget.value)}><option value="field">普通字段匹配</option><option value="order_item_sku">订单商品 SKU 匹配</option></select></label>
                <ExcelModelSelector name={`step_table_${index}`} models={excelModels} value={step.tableName} onChange={(value) => selectExportStepModel(index, value)} />
                <Field label={step.matchMode === 'order_item_sku' ? '订单号 Excel 列' : 'Excel 输入列'} name={`step_excel_${index}`} value={step.matchExcelColumn} onChange={(value) => updateExportStep(index, 'matchExcelColumn', value)} required />
                <ExcelModelFieldSelector label={step.matchMode === 'order_item_sku' ? '数据库订单号字段' : '匹配模型字段'} name={`step_match_${index}`} models={excelModels} tableName={step.tableName} value={step.dbMatchField} onChange={(value) => updateExportStep(index, 'dbMatchField', value)} />
                <ExcelModelFieldSelector label={step.matchMode === 'order_item_sku' ? '数据库购物明细字段' : '取值模型字段'} name={`step_value_${index}`} models={excelModels} tableName={step.tableName} value={step.dbValueField} onChange={(value) => updateExportStep(index, 'dbValueField', value)} />
                <Field label={step.matchMode === 'order_item_sku' ? 'SKU 输出列' : '追加输出列'} name={`step_output_${index}`} value={step.outputColumnName} onChange={(value) => updateExportStep(index, 'outputColumnName', value)} required />
                {step.matchMode === 'order_item_sku' && <><Field label="规格编码 Excel 列" name={`step_spec_${index}`} value={step.specExcelColumn} onChange={(value) => updateExportStep(index, 'specExcelColumn', value)} required /><Field label="销售金额 Excel 列（对应 totAmtActual）" name={`step_price_${index}`} value={step.priceExcelColumn} onChange={(value) => updateExportStep(index, 'priceExcelColumn', value)} required /><Field label="销售数量 Excel 列" name={`step_qty_${index}`} value={step.qtyExcelColumn} onChange={(value) => updateExportStep(index, 'qtyExcelColumn', value)} required /></>}
              </div>
              {step.matchMode === 'order_item_sku' && <p className={styles.modeNote}>按数据库购物明细字段匹配并输出完整 no：优先用规格编码匹配 mProductName 前缀，并同时校验销售金额和数量；规格编码为 15 位或 16 位时直接跳过。</p>}
              <div className={styles.stepFilterEditor}><div className={styles.stepFilterHeading}><div><strong>本步骤筛选</strong><span>{step.matchMode === 'order_item_sku' ? '多条条件需要同时满足，订单商品 SKU 模式仅可引用原始 Excel 列。' : '多条条件需要同时满足，可引用原始列或前序步骤追加列。'}</span></div><button type="button" onClick={() => addExportStepFilter(index)}>添加条件</button></div>{step.filters.length === 0 && <p className={styles.stepFilterEmpty}>未设置条件，本步骤处理全部 Excel 行。</p>}{step.filters.map((filter, filterIndex) => { const valueNotRequired = filter.op === 'empty' || filter.op === 'not_empty'; return <div className={styles.stepFilterRow} key={`${exportFormKey}-${index}-${filterIndex}`}><Field label={`条件 ${filterIndex + 1} · Excel 列`} name={`step_filter_column_${index}_${filterIndex}`} value={filter.column} onChange={(value) => updateExportStepFilter(index, filterIndex, 'column', value)} required /><label>运算符<select name={`step_filter_op_${index}_${filterIndex}`} value={filter.op} onChange={(event) => updateExportStepFilter(index, filterIndex, 'op', event.currentTarget.value)}>{excelMatchFilterOperatorOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>{valueNotRequired ? <label>筛选值<input value="此运算无需填写" readOnly disabled /></label> : <Field label="筛选值" name={`step_filter_value_${index}_${filterIndex}`} value={filter.value} onChange={(value) => updateExportStepFilter(index, filterIndex, 'value', value)} required />}<button type="button" onClick={() => removeExportStepFilter(index, filterIndex)} aria-label={`删除步骤 ${index + 1} 的条件 ${filterIndex + 1}`}>删除条件</button></div> })}</div>
            </article>
          ))}
          <div className={styles.stepFilterEditor}>
            <div className={styles.stepFilterHeading}>
              <div><strong>空值填充</strong><span>所有匹配步骤完成后，目标列为空时从同一行来源列填充；可引用原始列或追加输出列，不覆盖已有值。</span></div>
              <button type="button" onClick={addExportEmptyCellFill}>添加填充规则</button>
            </div>
            {exportEmptyCellFills.length === 0 && <p className={styles.stepFilterEmpty}>未设置空值填充规则。</p>}
            {exportEmptyCellFills.map((fill, index) => <div className={styles.stepFilterRow} key={`${exportFormKey}-empty-cell-fill-${index}`}>
              <Field label="目标列（为空时）" name={`empty_cell_fill_target_${index}`} value={fill.targetColumn} onChange={(value) => updateExportEmptyCellFill(index, 'targetColumn', value)} required />
              <Field label="来源列（同一行）" name={`empty_cell_fill_source_${index}`} value={fill.sourceColumn} onChange={(value) => updateExportEmptyCellFill(index, 'sourceColumn', value)} required />
              <button type="button" onClick={() => removeExportEmptyCellFill(index)} aria-label={`删除空值填充规则 ${index + 1}`}>删除规则</button>
            </div>)}
          </div>
        </div>
        <details className={styles.schemeAdvanced}><summary>方案与高级设置</summary><div><label>已保存方案<select value={selectedExportSchemeID} onChange={(event) => applyExportScheme(event.currentTarget.value)}><option value="">选择方案</option>{exportSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}</select></label><button type="button" disabled={!selectedExportSchemeID || loading} onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) beginSchemeSave(form, 'export_match', 'current') }}>保存到当前方案</button><button type="button" disabled={loading} onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) beginSchemeSave(form, 'export_match', 'new') }}>另存为新方案</button><label>导出列内容格式<textarea name="exportColumnFormats" defaultValue={exportDefaults.exportColumnFormats} rows={4} placeholder={'每行一个：列名=格式\n例如：金额=number\n下单时间=date'} /><small>支持 text、number、integer、bool、date。</small></label><Field label="批量查询大小" name="batchSize" defaultValue={exportDefaults.batchSize} /></div></details>
        {previewResult && <ExcelMatchPreviewPanel preview={previewResult} />}
        {uploadProgress && <p className={styles.modeNote} role="status" aria-live="polite">{uploadProgress}</p>}
      </form>
    )
  }

  function renderImportWriteForm() {
    return <form className={styles.uploadForm} onSubmit={createImportJob} key={importFormKey}>
      <label>已保存方案<select value={selectedImportSchemeID} onChange={(event) => applyImportScheme(event.currentTarget.value)}><option value="">选择方案</option>{importSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}</select></label>
      <button type="button" disabled={!selectedImportSchemeID || loading} onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) beginSchemeSave(form, 'import_update', 'current') }}>保存到当前方案</button><button type="button" disabled={loading} onClick={(event) => { const form = event.currentTarget.form; if (form?.reportValidity()) beginSchemeSave(form, 'import_update', 'new') }}>另存为新方案</button>
      <label className={styles.fileInputLabel}>Excel 文件<input name="file" type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={(event) => { setSelectedImportFileName(event.currentTarget.files?.[0]?.name ?? ''); clearUploadRef('import') }} /><span>{selectedImportFileName || '请选择需要导入更新的 .xlsx 文件'}</span></label>
      <Field label="Sheet 页名称" name="sheetName" defaultValue={importDefaults.sheetName} /><label>匹配表名<select name="tableName" defaultValue={importDefaults.tableName}><option value="bojun_retail_orders">伯俊零售单 bojun_retail_orders</option></select></label><label>数据库匹配字段<select name="dbMatchField" defaultValue={importDefaults.dbMatchField}>{bojunMatchFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></label><Field label="Excel 匹配列名" name="matchExcelColumn" defaultValue={importDefaults.matchExcelColumn} /><label>写入字段<select name="dbWriteField" defaultValue={importDefaults.dbWriteField}><option value="matched_docno">匹配单号 matched_docno</option><option value="completed_at">订单完成时间 completed_at</option></select></label><Field label="Excel 写入值列名" name="writeExcelColumn" defaultValue={importDefaults.writeExcelColumn} /><Field label="批量更新大小" name="batchSize" defaultValue={importDefaults.batchSize} />
      <label className={styles.checkboxLabel}><input name="confirmWrite" type="checkbox" />确认写入数据库</label><p className={styles.modeNote}>不勾选时只预检匹配数量，不写库；匹配单号只填充空值；订单完成时间只填充为空的 completed_at。</p>{uploadProgress && <p className={styles.modeNote} role="status" aria-live="polite">{uploadProgress}</p>}<div className={styles.formActions}><button className={styles.primary} type="submit" disabled={loading}><Upload aria-hidden="true" />创建预检/导入任务</button></div>
    </form>
  }

  function renderClearWriteForm() {
    return <form className={styles.uploadForm} onSubmit={createClearMatchedJob}>
      <label className={styles.fileInputLabel}>Excel 文件<input name="file" type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={(event) => { setSelectedClearFileName(event.currentTarget.files?.[0]?.name ?? ''); clearUploadRef('clear') }} /><span>{selectedClearFileName || '请选择需要退回的 .xlsx 文件'}</span></label>
      <Field label="Sheet 页名称" name="sheetName" defaultValue="Sheet1" /><label>匹配表名<select name="tableName" defaultValue="bojun_retail_orders"><option value="bojun_retail_orders">伯俊零售单 bojun_retail_orders</option></select></label><label>数据库匹配字段<select name="dbMatchField" defaultValue="docno">{bojunMatchFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></label><Field label="Excel 匹配列名" name="matchExcelColumn" defaultValue="外部订单编号" /><Field label="批量处理大小" name="batchSize" defaultValue="1000" />
      <label className={styles.checkboxLabel}><input name="confirmWrite" type="checkbox" />确认清空 matched_docno</label><p className={styles.modeNote}>不勾选时只预检会命中的行；勾选后会把命中行的 matched_docno 清空，用于退回未匹配状态。</p>{uploadProgress && <p className={styles.modeNote} role="status" aria-live="polite">{uploadProgress}</p>}<div className={styles.formActions}><button type="submit" disabled={loading}><RefreshCcw aria-hidden="true" />创建预检/退回任务</button></div>
    </form>
  }

  return (
    <PageCanvas className={styles.page}>
      <PageHeader
        eyebrow="EXCEL WORKSPACE"
        title={section === 'jobs' ? 'Excel 任务' : section === 'schemes' ? 'Excel 多步骤匹配' : 'Excel 写入'}
        description={section === 'jobs' ? '查询任务状态、进度、日志和下载结果。' : section === 'schemes' ? '配置数据库模型、字段和顺序匹配步骤。' : '执行导入更新与退回未匹配操作。'}
      />
      {section === 'jobs' && <>
        <MetricStrip items={[{ key: 'history', label: '历史任务', value: jobHistoryPagination?.total ?? jobHistory.length }, { key: 'current', label: '当前任务', value: job ? `#${job.id}` : '-' }, { key: 'status', label: '任务状态', value: job ? excelJobStatusLabel(job.status) : '-' }, { key: 'processed', label: '已处理行', value: job ? `${job.processed_rows.toLocaleString('en-US')} / ${job.total_rows.toLocaleString('en-US')}` : '-' }, { key: 'tracking', label: '自动跟踪', value: trackingJobID ? `#${trackingJobID}` : '-' }]} />
        <form onSubmit={(event) => {
          event.preventDefault()
          setJobHistoryPage(1)
          setAppliedJobHistoryFilters({ keyword: jobQuery, status: jobStatus === 'all' ? '' : jobStatus, operation: jobOperation === 'all' ? '' : jobOperation })
          setJobHistoryReloadVersion((version) => version + 1)
        }}>
          <FilterToolbar>
            <label className={styles.search}><span className={styles.visuallyHidden}>任务 ID、文件名或错误</span><span><Search aria-hidden="true" /><input name="excel_job_query" type="search" value={jobQuery} placeholder="任务 ID / 文件名 / 错误" onChange={(event) => setJobQuery(event.currentTarget.value)} /></span></label>
            <SelectFilter label="状态" value={jobStatus} onChange={(value) => { setJobStatus(value); setJobHistoryPage(1); setAppliedJobHistoryFilters({ keyword: jobQuery, status: value === 'all' ? '' : value, operation: jobOperation === 'all' ? '' : jobOperation }) }} options={[{ value: 'pending', label: '等待处理' }, { value: 'running', label: '处理中' }, { value: 'success', label: '成功' }, { value: 'failed', label: '失败' }, { value: 'expired', label: '已过期' }]} />
            <SelectFilter label="操作" value={jobOperation} onChange={(value) => { setJobOperation(value); setJobHistoryPage(1); setAppliedJobHistoryFilters({ keyword: jobQuery, status: jobStatus === 'all' ? '' : jobStatus, operation: value === 'all' ? '' : value }) }} options={[{ value: 'match', label: '匹配任务' }, { value: 'write', label: '导入任务' }]} />
            <button type="submit" disabled={jobHistoryLoading}>{jobHistoryLoading ? '查询中…' : '查询'}</button>
          </FilterToolbar>
        </form>
        {jobHistoryError && <FeedbackState kind="error" title="任务列表加载提示" description={`${jobHistoryError}${jobHistoryPagination && !jobHistoryError.includes('兼容数据') ? ' 已保留最近一次成功数据。' : ''}`} action={<button type="button" onClick={() => setJobHistoryReloadVersion((version) => version + 1)} disabled={jobHistoryLoading}>重试</button>} />}
        <Section title="Excel 任务" description={jobHistoryLoading && !jobHistoryPagination ? '正在加载…' : `共 ${jobHistoryPagination?.total ?? 0} 条`} flush>
              <ExcelJobHistoryTable
                jobs={jobHistory}
                loading={jobHistoryLoading}
                downloadingJobID={downloadingJobID}
                selectedJobID={job?.id ?? null}
                onDownload={downloadJob}
                onView={(id, trigger) => void openJobDetail(id, trigger)}
              />
              <PaginationControls page={jobHistoryPagination?.page ?? jobHistoryPage} totalPages={jobHistoryPagination?.totalPages ?? 0} loading={jobHistoryLoading} onPrevious={() => setJobHistoryPage((page) => Math.max(1, page - 1))} onNext={() => setJobHistoryPage((page) => page + 1)} />
        </Section>
        {jobDetailOpen && job && <Drawer open size="medium" title={`任务 #${job.id}`} description="任务进度、有效期和实时日志" returnFocus={jobDetailTriggerRef.current} onClose={() => setJobDetailOpen(false)}><ExcelJobDetailContent job={job} logs={jobLogs} progress={selectedJobProgress} autoRefreshText={autoRefreshText} loading={loading} downloading={downloadingJobID === job.id} onRefresh={() => void refreshJobByID(job.id)} onDownload={() => void downloadJob(job.id)} /></Drawer>}
      </>}

      {section === 'schemes' && <>
        <MetricStrip items={[{ key: 'schemes', label: '导出方案', value: exportSchemes.length }, { key: 'steps', label: '当前步骤', value: exportSteps.length }, { key: 'maximum', label: '最大步骤', value: 20 }, { key: 'filters', label: '筛选规则', value: '可选' }]} />
        <section className={styles.masterDetail}>
          <Panel title="匹配导出配置" icon={<Upload />} meta="步骤顺序与预览都基于真实导出配置">
            <div className={styles.configSummary}>
              <strong>{selectedExportSchemeID ? '正在编辑已保存方案' : '新建匹配方案'}</strong>
              <span>当前包含 {exportSteps.length} 个步骤；可直接调整顺序、筛选和输出字段。</span>
            </div>
            {renderExportMatchForm()}
          </Panel>
          <aside className={styles.aside} aria-label="已保存导出方案">
            <Panel title="已保存导出方案" icon={<ListChecks />} meta={`${exportSchemes.length} 个方案`}><ExcelSchemeList schemes={exportSchemes} deletingSchemeID={deletingSchemeID} onDelete={setPendingSchemeDelete} onOpen={(id) => applyExportScheme(String(id))} /></Panel>
          </aside>
        </section>
      </>}

      {section === 'write' && <>
        <MetricStrip items={[{ key: 'schemes', label: '导入方案', value: importSchemes.length }, { key: 'default', label: '默认模式', value: '只预检' }, { key: 'write', label: '写入保护', value: '不覆盖' }, { key: 'clear', label: '清空保护', value: '需确认' }]} />
        <section className={styles.masterDetail}>
          <Panel title="数据库回写" icon={<Database />} meta="默认只预检；写入与退回均需二次确认">
            <div className={styles.modeSwitch} role="group" aria-label="数据库回写模式"><button type="button" className={writeMode === 'import' ? styles.active : undefined} onClick={() => setWriteMode('import')}><Database aria-hidden="true" />匹配导入</button><button type="button" className={writeMode === 'clear' ? `${styles.active} ${styles.danger}` : styles.danger} onClick={() => setWriteMode('clear')}><RefreshCcw aria-hidden="true" />退回未匹配</button></div>
            <p className={styles.modeNote}>{writeMode === 'import' ? '先预检，确认后只填充空字段。' : '先预检，确认后清空命中行的 matched_docno。'}</p>
            {writeMode === 'import' ? renderImportWriteForm() : renderClearWriteForm()}
            {latestWriteJob && <section className={styles.writeSummary} aria-labelledby="excel-write-summary-title"><div><strong id="excel-write-summary-title">最近预检/写入任务</strong><span>以下数据来自服务端安全任务摘要；不覆盖与错误计数未返回。</span></div><div className={styles.jobDetail}><Metric label="任务 ID" value={latestWriteJob.id} /><Metric label="状态" value={excelJobStatusLabel(latestWriteJob.status)} /><Metric label="总行数" value={latestWriteJob.total_rows} /><Metric label="已处理" value={latestWriteJob.processed_rows} /><Metric label="预计命中" value={latestWriteJob.matched_rows} /><Metric label="未匹配" value={latestWriteJob.unmatched_rows} /></div><div className={styles.detailActions}><span>不覆盖/错误：服务端未返回独立计数。</span><button type="button" onClick={onNavigateToJobs}>查看任务详情</button></div></section>}
          </Panel>
          <aside className={styles.aside} aria-label="已保存导入方案"><Panel title="已保存导入方案" icon={<ListChecks />} meta={`${importSchemes.length} 个方案`}><ExcelSchemeList schemes={importSchemes} deletingSchemeID={deletingSchemeID} onDelete={setPendingSchemeDelete} onOpen={(id) => { setWriteMode('import'); applyImportScheme(String(id)) }} /></Panel></aside>
        </section>
      </>}

      {pendingSchemeSave && excelDialog === null && <Dialog className={styles.dialog} open title="保存 Excel 匹配方案" closeDisabled={loading || schemeSaveInFlightRef.current} onClose={() => { if (!loading && !schemeSaveInFlightRef.current) { setPendingSchemeSave(null); setSchemeSaveError('') } }}>{renderPendingSchemeSave()}</Dialog>}

      {pendingWrite && excelDialog === null && <Dialog className={styles.dialog} open title={pendingWrite.slot === 'import' ? '确认写入数据库' : '确认退回未匹配'} closeDisabled={loading} onClose={() => { if (!loading) setPendingWrite(null) }}><div className={styles.viewStack}><p>{pendingWrite.message}</p><div className={styles.formActions}><button type="button" disabled={loading} onClick={() => setPendingWrite(null)}>返回修改</button><button className={pendingWrite.slot === 'clear' ? styles.danger : styles.primary} type="button" disabled={loading} onClick={() => void confirmPendingWrite()}>{loading ? '创建任务中…' : pendingWrite.slot === 'clear' ? '确认退回' : '确认写入'}</button></div></div></Dialog>}

      {excelDialog === 'export' && (
        <Dialog className={`${styles.dialog} ${styles.wideDialog}`} open
          title={pendingSchemeSave ? '保存 Excel 匹配方案' : '匹配导出参数'}

          closeDisabled={loading || schemeSaveInFlightRef.current}
          onClose={() => { if (!loading && !schemeSaveInFlightRef.current) closeExcelDialog() }}
          footer={pendingSchemeSave ? undefined : (
            <div className={styles.modalFooterContent}>
              {uploadProgress && <p className={`${styles.modeNote} ${styles.modalFooterStatus}`} role="status" aria-live="polite">{uploadProgress}</p>}
              <div className={styles.formActions}>
                <button
                  type="button"
                  form="excel-export-job-form"
                  onClick={(event) => {
                    const form = event.currentTarget.form
                    if (form?.reportValidity()) void previewExportJob(form)
                  }}
                  disabled={loading}
                >
                  <FileJson aria-hidden="true" />
                  预览匹配
                </button>
                <button className={styles.primary} type="submit" form="excel-export-job-form" disabled={loading}>
                  <Upload aria-hidden="true" />
                  创建导出任务
                </button>
              </div>
            </div>
          )}
        >
          {renderPendingSchemeSave()}
          <form id="excel-export-job-form" className={styles.uploadForm} onSubmit={createExportJob} key={exportFormKey} hidden={pendingSchemeSave !== null}>
            <label>
              已保存方案
              <select value={selectedExportSchemeID} onChange={(event) => applyExportScheme(event.currentTarget.value)}>
                <option value="">选择方案</option>
                {exportSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}
              </select>
            </label>
            <button
              type="button"
              disabled={!selectedExportSchemeID || loading}
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form?.reportValidity()) beginSchemeSave(form, 'export_match', 'current')
              }}
            >
              保存到当前方案
            </button>
            <button
              type="button"
              disabled={loading}
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form?.reportValidity()) beginSchemeSave(form, 'export_match', 'new')
              }}
            >
              另存为新方案
            </button>
            <label className={styles.fileInputLabel}>
              Excel 文件
              <input
                name="file"
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                onChange={(event) => {
                  setSelectedExportFileName(event.currentTarget.files?.[0]?.name ?? '')
                  clearUploadRef('export')
                  setPreviewResult(null)
                }}
              />
              <span>{selectedExportFileName || '请选择需要匹配导出的 .xlsx 文件'}</span>
            </label>
            <Field label="Sheet 页名称" name="sheetName" defaultValue={exportDefaults.sheetName} />
            <div className={styles.stepEditor}>
              <div className={styles.stepEditorTitle}>
                <div>
                  <strong>匹配步骤</strong>
                  <span>每一步都从完整 Excel 行集独立筛选；前一步跳过的行仍可进入后续步骤。</span>
                </div>
                <button type="button" onClick={addExportStep} disabled={exportSteps.length >= 20}>添加步骤</button>
              </div>
              {excelModelsLoading && <p className={styles.modeNote}>正在加载模型与字段目录…</p>}
              {excelModelsError && (
                <div className={styles.catalogError} role="alert">
                  <span>模型与字段目录加载失败：{excelModelsError}</span>
                  <button type="button" onClick={() => void loadExcelModels()}>重试加载</button>
                </div>
              )}
              {!excelModelsLoading && !excelModelsError && excelModels.length === 0 && (
                <p className={styles.modeNote}>当前数据库没有返回可选择的模型；历史配置仍可查看，但新步骤需要先确认数据库连接和模型表。</p>
              )}
              {exportSteps.map((step, index) => (
                <article className={styles.stepCard} key={`${exportFormKey}-${index}`}>
                  <div className={styles.stepHeading}>
                    <strong>步骤 {index + 1}</strong>
                    <div className={styles.tableActions}>
                      <button type="button" onClick={() => moveExportStep(index, -1)} disabled={index === 0}>上移</button>
                      <button type="button" onClick={() => moveExportStep(index, 1)} disabled={index === exportSteps.length - 1}>下移</button>
                      <button type="button" onClick={() => removeExportStep(index)} disabled={exportSteps.length === 1}>删除</button>
                    </div>
                  </div>
                  <div className={styles.stepFields}>
                    <Field label="步骤名称" name={`step_name_${index}`} value={step.name} onChange={(value) => updateExportStep(index, 'name', value)} required />
                    <label>
                      匹配模式
                      <select
                        name={`step_mode_${index}`}
                        value={step.matchMode}
                        onChange={(event) => updateExportStep(index, 'matchMode', event.currentTarget.value)}
                      >
                        <option value="field">普通字段匹配</option>
                        <option value="order_item_sku">订单商品 SKU 匹配</option>
                      </select>
                    </label>
                    <ExcelModelSelector
                      name={`step_table_${index}`}
                      models={excelModels}
                      value={step.tableName}
                      onChange={(value) => selectExportStepModel(index, value)}
                    />
                    <Field label={step.matchMode === 'order_item_sku' ? '订单号 Excel 列' : 'Excel 输入列'} name={`step_excel_${index}`} value={step.matchExcelColumn} onChange={(value) => updateExportStep(index, 'matchExcelColumn', value)} required />
                    <ExcelModelFieldSelector
                      label={step.matchMode === 'order_item_sku' ? '数据库订单号字段' : '匹配模型字段'}
                      name={`step_match_${index}`}
                      models={excelModels}
                      tableName={step.tableName}
                      value={step.dbMatchField}
                      onChange={(value) => updateExportStep(index, 'dbMatchField', value)}
                    />
                    <ExcelModelFieldSelector
                      label={step.matchMode === 'order_item_sku' ? '数据库购物明细字段' : '取值模型字段'}
                      name={`step_value_${index}`}
                      models={excelModels}
                      tableName={step.tableName}
                      value={step.dbValueField}
                      onChange={(value) => updateExportStep(index, 'dbValueField', value)}
                    />
                    <Field label={step.matchMode === 'order_item_sku' ? 'SKU 输出列' : '追加输出列'} name={`step_output_${index}`} value={step.outputColumnName} onChange={(value) => updateExportStep(index, 'outputColumnName', value)} required />
                    {step.matchMode === 'order_item_sku' && <>
                      <Field label="规格编码 Excel 列" name={`step_spec_${index}`} value={step.specExcelColumn} onChange={(value) => updateExportStep(index, 'specExcelColumn', value)} required />
                      <Field label="销售金额 Excel 列（对应 totAmtActual）" name={`step_price_${index}`} value={step.priceExcelColumn} onChange={(value) => updateExportStep(index, 'priceExcelColumn', value)} required />
                      <Field label="销售数量 Excel 列" name={`step_qty_${index}`} value={step.qtyExcelColumn} onChange={(value) => updateExportStep(index, 'qtyExcelColumn', value)} required />
                    </>}
                  </div>
                  {step.matchMode === 'order_item_sku' && (
                    <p className={styles.modeNote}>
                      按数据库购物明细字段（例如 items_json）匹配并输出完整 no：优先用 Excel 规格编码匹配 mProductName 前缀，并同时校验 totAmtActual（销售金额）和 qty，mProductName 长度不限；若规格编码在订单明细中没有候选，则按销售金额和数量兜底匹配。Excel 规格编码为 15 位或 16 位时直接跳过。每条购物明细按其在 JSON 中的出现次数使用一次；相同明细重复出现时，可按次数重复输出同一 no。
                    </p>
                  )}
                  <div className={styles.stepFilterEditor}>
                    <div className={styles.stepFilterHeading}>
                      <div>
                        <strong>本步骤筛选</strong>
                        <span>{step.matchMode === 'order_item_sku'
                          ? '只决定本步骤处理哪些行；多条条件需要同时满足。订单商品 SKU 模式仅可引用原始 Excel 列。'
                          : '只决定本步骤处理哪些行；多条条件需要同时满足。可引用原始列或前序步骤追加列。'}</span>
                      </div>
                      <button type="button" onClick={() => addExportStepFilter(index)}>添加条件</button>
                    </div>
                    {step.filters.length === 0 && <p className={styles.stepFilterEmpty}>未设置条件，本步骤处理全部 Excel 行。</p>}
                    {step.filters.map((filter, filterIndex) => {
                      const valueNotRequired = filter.op === 'empty' || filter.op === 'not_empty'
                      return (
                        <div className={styles.stepFilterRow} key={`${exportFormKey}-${index}-${filterIndex}`}>
                          <Field
                            label={`条件 ${filterIndex + 1} · Excel 列`}
                            name={`step_filter_column_${index}_${filterIndex}`}
                            value={filter.column}
                            onChange={(value) => updateExportStepFilter(index, filterIndex, 'column', value)}
                            required
                          />
                          <label>
                            运算符
                            <select
                              name={`step_filter_op_${index}_${filterIndex}`}
                              value={filter.op}
                              onChange={(event) => updateExportStepFilter(index, filterIndex, 'op', event.currentTarget.value)}
                            >
                              {excelMatchFilterOperatorOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                            </select>
                          </label>
                          {valueNotRequired
                            ? <label>筛选值<input value="此运算无需填写" readOnly disabled /></label>
                            : <Field
                                label="筛选值"
                                name={`step_filter_value_${index}_${filterIndex}`}
                                value={filter.value}
                                onChange={(value) => updateExportStepFilter(index, filterIndex, 'value', value)}
                                required
                              />}
                          <button type="button" onClick={() => removeExportStepFilter(index, filterIndex)} aria-label={`删除步骤 ${index + 1} 的条件 ${filterIndex + 1}`}>删除条件</button>
                        </div>
                      )
                    })}
                  </div>
                </article>
              ))}
            </div>
            <label>
              导出列内容格式
              <textarea
                name="exportColumnFormats"
                defaultValue={exportDefaults.exportColumnFormats}
                rows={4}
                placeholder={'每行一个：列名=格式\n例如：金额=number\n下单时间=date'}
              />
              <small>支持格式：text、number、integer、bool、date。列名可填原 Excel 表头或追加列名。</small>
            </label>
            <Field label="批量查询大小" name="batchSize" defaultValue={exportDefaults.batchSize} />
            {previewResult && <ExcelMatchPreviewPanel preview={previewResult} />}
          </form>
        </Dialog>
      )}

      {excelDialog === 'import' && (
        <Dialog className={`${styles.dialog} ${styles.mediumDialog}`} open title={pendingSchemeSave ? '保存 Excel 匹配方案' : pendingWrite?.slot === 'import' ? '确认写入数据库' : '匹配导入参数'} closeDisabled={loading || schemeSaveInFlightRef.current} onClose={() => { if (!loading && !schemeSaveInFlightRef.current) { setPendingWrite(null); closeExcelDialog() } }}>
          {pendingWrite?.slot === 'import' && <div className={styles.viewStack}><p>{pendingWrite.message}</p><div className={styles.formActions}><button type="button" disabled={loading} onClick={() => setPendingWrite(null)}>返回修改</button><button className={styles.primary} type="button" disabled={loading} onClick={() => void confirmPendingWrite()}>{loading ? '创建任务中…' : '确认写入'}</button></div></div>}
          {renderPendingSchemeSave()}
          <form className={styles.uploadForm} onSubmit={createImportJob} key={importFormKey} hidden={pendingWrite?.slot === 'import' || pendingSchemeSave !== null}>
            <label>
              已保存方案
              <select value={selectedImportSchemeID} onChange={(event) => applyImportScheme(event.currentTarget.value)}>
                <option value="">选择方案</option>
                {importSchemes.map((scheme) => <option value={scheme.id} key={scheme.id}>{scheme.name}</option>)}
              </select>
            </label>
            <button
              type="button"
              disabled={!selectedImportSchemeID || loading}
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form?.reportValidity()) beginSchemeSave(form, 'import_update', 'current')
              }}
            >
              保存到当前方案
            </button>
            <button
              type="button"
              disabled={loading}
              onClick={(event) => {
                const form = event.currentTarget.form
                if (form?.reportValidity()) beginSchemeSave(form, 'import_update', 'new')
              }}
            >
              另存为新方案
            </button>
            <label className={styles.fileInputLabel}>
              Excel 文件
              <input
                name="file"
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                onChange={(event) => {
                  setSelectedImportFileName(event.currentTarget.files?.[0]?.name ?? '')
                  clearUploadRef('import')
                }}
              />
              <span>{selectedImportFileName || '请选择需要导入更新的 .xlsx 文件'}</span>
            </label>
            <Field label="Sheet 页名称" name="sheetName" defaultValue={importDefaults.sheetName} />
            <label>
              匹配表名
              <select name="tableName" defaultValue={importDefaults.tableName}>
                <option value="bojun_retail_orders">伯俊零售单 bojun_retail_orders</option>
              </select>
            </label>
            <label>
              数据库匹配字段
              <select name="dbMatchField" defaultValue={importDefaults.dbMatchField}>
                {bojunMatchFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
              </select>
            </label>
            <Field label="Excel 匹配列名" name="matchExcelColumn" defaultValue={importDefaults.matchExcelColumn} />
            <label>
              写入字段
              <select name="dbWriteField" defaultValue={importDefaults.dbWriteField}>
                <option value="matched_docno">匹配单号 matched_docno</option>
                <option value="completed_at">订单完成时间 completed_at</option>
              </select>
            </label>
            <Field label="Excel 写入值列名" name="writeExcelColumn" defaultValue={importDefaults.writeExcelColumn} />
            <Field label="批量更新大小" name="batchSize" defaultValue={importDefaults.batchSize} />
            <label className={styles.checkboxLabel}>
              <input name="confirmWrite" type="checkbox" />
              确认写入数据库
            </label>
            <p className={styles.modeNote}>
              不勾选时只预检匹配数量，不写库；匹配单号只填充空值；订单完成时间要求 yyyy-mm-dd hh:mm:ss 格式且只填充为空的 completed_at。
            </p>
            {uploadProgress && <p className={styles.modeNote}>{uploadProgress}</p>}
            <div className={styles.formActions}>
              <button className={styles.primary} type="submit" disabled={loading}>
                <Upload aria-hidden="true" />
                创建预检/导入任务
              </button>
            </div>
          </form>
        </Dialog>
      )}

      {excelDialog === 'clear' && (
        <Dialog className={`${styles.dialog} ${styles.mediumDialog}`} open title={pendingWrite?.slot === 'clear' ? '确认退回未匹配' : '退回未匹配参数'} closeDisabled={loading} onClose={() => { if (!loading) { setPendingWrite(null); closeExcelDialog() } }}>
          {pendingWrite?.slot === 'clear' && <div className={styles.viewStack}><p>{pendingWrite.message}</p><div className={styles.formActions}><button type="button" disabled={loading} onClick={() => setPendingWrite(null)}>返回修改</button><button className={styles.danger} type="button" disabled={loading} onClick={() => void confirmPendingWrite()}>{loading ? '创建任务中…' : '确认退回'}</button></div></div>}
          <form className={styles.uploadForm} onSubmit={createClearMatchedJob} hidden={pendingWrite?.slot === 'clear'}>
            <label className={styles.fileInputLabel}>
              Excel 文件
              <input
                name="file"
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                onChange={(event) => {
                  setSelectedClearFileName(event.currentTarget.files?.[0]?.name ?? '')
                  clearUploadRef('clear')
                }}
              />
              <span>{selectedClearFileName || '请选择需要退回的 .xlsx 文件'}</span>
            </label>
            <Field label="Sheet 页名称" name="sheetName" defaultValue="Sheet1" />
            <label>
              匹配表名
              <select name="tableName" defaultValue="bojun_retail_orders">
                <option value="bojun_retail_orders">伯俊零售单 bojun_retail_orders</option>
              </select>
            </label>
            <label>
              数据库匹配字段
              <select name="dbMatchField" defaultValue="docno">
                {bojunMatchFieldOptions.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
              </select>
            </label>
            <Field label="Excel 匹配列名" name="matchExcelColumn" defaultValue="外部订单编号" />
            <Field label="批量处理大小" name="batchSize" defaultValue="1000" />
            <label className={styles.checkboxLabel}>
              <input name="confirmWrite" type="checkbox" />
              确认清空 matched_docno
            </label>
            <p className={styles.modeNote}>
              不勾选时只预检会命中的行；勾选后会把命中行的 matched_docno 清空，用于退回未匹配状态。
            </p>
            {uploadProgress && <p className={styles.modeNote}>{uploadProgress}</p>}
            <div className={styles.formActions}>
              <button type="submit" disabled={loading}>
                <RefreshCcw aria-hidden="true" />
                创建预检/退回任务
              </button>
            </div>
          </form>
        </Dialog>
      )}

      {excelDialog === 'query' && (
        <Dialog className={styles.dialog} open title="任务查询与下载" onClose={closeExcelDialog}>
          <div className={styles.jobActions}>
            <label>
              任务 ID
              <input value={jobID} onChange={(event) => setJobID(event.target.value)} />
            </label>
            <button type="button" onClick={refreshJob} disabled={loading}>
              <RefreshCcw aria-hidden="true" />
              查询状态
            </button>
            <button type="button" onClick={() => void downloadJob()} disabled={loading || !job || !canDownloadExcelJob(job)}>
              <Download aria-hidden="true" />
              {downloadingJobID === Number(jobID || job?.id) ? '下载中' : '下载结果'}
            </button>
          </div>
        </Dialog>
      )}

      {pendingSchemeDelete && (
        <Dialog className={styles.dialog} open title="删除 Excel 匹配方案" onClose={() => { if (deletingSchemeID === null) setPendingSchemeDelete(null) }}>
          <p>确认删除方案“{pendingSchemeDelete.name}”？删除后不能恢复，已创建的任务不会受影响。</p>
          <div className={styles.formActions}>
            <button type="button" onClick={() => setPendingSchemeDelete(null)} disabled={deletingSchemeID !== null}>取消</button>
            <button type="button" className={styles.danger} onClick={() => void deleteScheme(pendingSchemeDelete)} disabled={deletingSchemeID !== null}>
              {deletingSchemeID === pendingSchemeDelete.id ? '删除中…' : '确认删除'}
            </button>
          </div>
        </Dialog>
      )}
    </PageCanvas>
  )
}

function ExcelSchemeList({ schemes, deletingSchemeID, onDelete, onOpen }: { schemes: ExcelMatchScheme[]; deletingSchemeID: number | null; onDelete: (scheme: ExcelMatchScheme) => void; onOpen: (id: number) => void }) {
  if (schemes.length === 0) return <EmptyState text="暂无已保存方案。" />
  return (
    <DataTable className={styles.schemeTable} density="compact" minWidth={620} scrollLabel="Excel 匹配方案">
        <thead>
          <tr>
            <th>方案名称</th>
            <th>步骤数</th>
            <th>更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {schemes.map((scheme) => (
            <tr key={scheme.id}>
              <td>{scheme.name}</td>
              <td>{scheme.operation === 'export_match' ? (scheme.config.steps?.length || 1) : '-'}</td>
              <td>{formatUnixTime(scheme.updated_at)}</td>
              <td>
                <div className={styles.tableActions}>
                  <button type="button" onClick={() => onOpen(scheme.id)} disabled={deletingSchemeID !== null}>打开配置</button>
                  <button type="button" className={styles.danger} onClick={() => onDelete(scheme)} disabled={deletingSchemeID !== null}>
                    {deletingSchemeID === scheme.id ? '删除中…' : '删除'}
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
    </DataTable>
  )
}

function ExcelJobHistoryTable({
  jobs,
  loading,
  downloadingJobID,
  selectedJobID,
  onView,
  onDownload,
}: {
  jobs: ExcelMatchJob[]
  loading: boolean
  downloadingJobID: number | null
  selectedJobID: number | null
  onView: (id: number, trigger: HTMLButtonElement) => void
  onDownload: (id: number) => void
}) {
  if (jobs.length === 0) return <EmptyState text="暂无 Excel 任务历史。" />
  return (
    <DataTable className={styles.historyTable} density="compact" minWidth={900} scrollLabel="Excel 任务列表">
        <thead>
          <tr>
            <th>ID</th>
            <th>文件</th>
            <th>类型</th>
            <th>状态</th>
            <th>处理行</th>
            <th>匹配/未匹配</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {jobs.map((item) => (
            <tr className={item.id === selectedJobID ? styles.historyRowSelected : undefined} key={item.id}>
              <td>#{item.id}</td>
              <td>{item.source_file_name || '-'}</td>
              <td>{excelJobOperationLabel(excelJobOperation(item))}</td>
              <td><StatusTag tone={excelJobStatusTone(item.status)}>{excelJobStatusLabel(item.status)}</StatusTag></td>
              <td><div className={styles.jobRowProgress}><span>{item.processed_rows.toLocaleString('en-US')} / {item.total_rows.toLocaleString('en-US')}</span><span>{excelJobProgressPercent(item)}%</span><progress value={excelJobProgressPercent(item)} max="100" aria-label={`任务 #${item.id} 处理进度`} /></div></td>
              <td><span className={styles.jobMatchCount}>{item.matched_rows.toLocaleString('en-US')}</span> / <span className={styles.jobUnmatchedCount}>{item.unmatched_rows.toLocaleString('en-US')}</span></td>
              <td>{formatUnixTime(item.created_at)}</td>
              <td>
                <div className={styles.tableActions}>
                  <button type="button" onClick={(event) => onView(item.id, event.currentTarget)} disabled={loading}>
                    查看
                  </button>
                  <button
                    type="button"
                    onClick={() => onDownload(item.id)}
                    disabled={loading || downloadingJobID === item.id || !canDownloadExcelJob(item)}
                    title={item.download_message || undefined}
                  >
                    {downloadingJobID === item.id ? '下载中' : '下载'}
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
    </DataTable>
  )
}

function ExcelJobDetailContent({ job, logs, progress, autoRefreshText, loading, downloading, onRefresh, onDownload }: {
  job: ExcelMatchJob
  logs: ExcelMatchJobLog[]
  progress: number
  autoRefreshText: string
  loading: boolean
  downloading: boolean
  onRefresh: () => void
  onDownload: () => void
}) {
  return (
    <section className={styles.jobDetailPanel} aria-label={`Excel 任务 ${job.id} 执行详情`}>
      {autoRefreshText && <p className={styles.modeNote} role="status">{autoRefreshText}</p>}
      <dl className={styles.jobDetailMeta}><div><dt>源文件</dt><dd>{job.source_file_name || '-'}</dd></div><div><dt>类型</dt><dd>{excelJobOperationLabel(excelJobOperation(job))}</dd></div></dl>
      <div className={styles.jobDetailCounts}><Metric label="匹配行" value={job.matched_rows.toLocaleString('en-US')} /><Metric label="未匹配行" value={job.unmatched_rows.toLocaleString('en-US')} /></div>
      <div className={styles.jobProgress}>
        <strong>处理进度</strong><span>{job.processed_rows.toLocaleString('en-US')} / {job.total_rows.toLocaleString('en-US')}（{progress}%）</span>
        <progress value={progress} max="100" aria-label={`Excel 任务 #${job.id} 处理进度`} />
      </div>
      <dl className={styles.jobExpiry}><dt>过期时间</dt><dd>{formatDate(job.expires_at)}</dd></dl>
      <div className={styles.jobLogHeading}>实时日志</div><ExcelJobLogList logs={logs} />
      <div className={styles.jobActionHeading}>操作</div>
      <div className={styles.detailActions}>
        <button type="button" onClick={onRefresh} disabled={loading}><RefreshCcw aria-hidden="true" />刷新状态</button>
        <button type="button" onClick={onDownload} disabled={loading || downloading || !canDownloadExcelJob(job)}><Download aria-hidden="true" />{downloading ? '下载中' : '下载结果'}</button>
        {!canDownloadExcelJob(job) && <span>{job.download_message || '任务完成并生成结果后可下载。'}</span>}
      </div>
      {job.status === 'failed' && <div className={styles.errorBanner} role="alert">任务执行失败，请查看受控服务日志。</div>}
    </section>
  )
}

function ExcelMatchPreviewPanel({ preview }: { preview: ExcelMatchPreviewResult }) {
  return (
    <div className={styles.previewPanel}>
      <div className={styles.previewMetrics}>
        <Metric label="扫描行" value={excelPreviewStat(preview.stats, 'TotalRows')} />
        <Metric label="参与步骤行" value={excelPreviewStat(preview.stats, 'FilteredRows')} />
        <Metric label="已匹配" value={excelPreviewStat(preview.stats, 'MatchedRows')} />
        <Metric label="未匹配" value={excelPreviewStat(preview.stats, 'UnmatchedRows')} />
        <Metric label="扫描上限" value={preview.truncated ? `${preview.scanLimit}+` : preview.scanLimit} />
      </div>
      <div className={styles.tableWrap}>
        <table className={styles.previewTable}>
          <thead>
            <tr>
              <th>行号</th>
              <th>匹配键</th>
              <th>状态</th>
              <th>追加值</th>
              <th>步骤结果</th>
              <th>原因</th>
              <th>Excel 行内容</th>
            </tr>
          </thead>
          <tbody>
            {preview.samples.map((sample) => (
              <tr key={`${sample.rowNumber}-${sample.matchKey}-${sample.status}`}>
                <td>{sample.rowNumber}</td>
                <td>{sample.matchKey || '-'}</td>
                <td>{excelPreviewStatusLabel(sample.status)}</td>
                <td>{sample.matchedValue || '-'}</td>
                <td>
                  {sample.stepResults?.length ? (
                    <div className={styles.previewSteps}>
                      {sample.stepResults.map((step) => (
                        <span key={`${step.stepIndex}-${step.stepName}`}>
                          {step.stepIndex}. {step.stepName || '未命名'}：{step.matchedValue || excelPreviewStatusLabel(step.status)}
                        </span>
                      ))}
                    </div>
                  ) : '-'}
                </td>
                <td>{sample.reason || '-'}</td>
                <td>{compactText(JSON.stringify(sample.values || {})) || '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {preview.samples.length === 0 && <EmptyState text="预览没有返回样例。" />}
    </div>
  )
}

function ExcelJobLogList({ logs }: { logs: ExcelMatchJobLog[] }) {
  if (logs.length === 0) return <EmptyState text="暂无任务日志。" />
  return (
    <div className={styles.jobLogList}>
      {logs.map((log) => (
        <article className={styles.recordRow} key={log.id}>
          <div>
            <strong>{log.message || '任务日志已记录'}</strong>
            <span>{excelLogLevelLabel(log.level)} / {formatUnixTime(log.created_at)}</span>
          </div>
        </article>
      ))}
    </div>
  )
}

function Panel({ title, icon, meta, children }: { title: string; icon: ReactNode; meta: string; children: ReactNode }) { return <section className={styles.panel}><div className={styles.panelTitle}>{icon}<div><h3>{title}</h3><span>{meta}</span></div></div>{children}</section> }
function SelectFilter({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: Array<{ value: string; label: string }> }) { return <label>{label}<select name={`filter-${label}`} value={value} onChange={(event) => onChange(event.currentTarget.value)}><option value="all">全部</option>{options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></label> }
function excelJobStatusTone(status: string): 'neutral' | 'success' | 'warning' | 'danger' { if (status === 'success') return 'success'; if (status === 'failed' || status === 'expired') return 'danger'; if (status === 'pending' || status === 'running') return 'warning'; return 'neutral' }
function Metric({ label, value }: { label: string; value: ReactNode }) { return <div className={styles.metric}><span>{label}</span><strong>{value}</strong></div> }
function EmptyState({ text }: { text: string }) { return <div className={styles.emptyState}>{text}</div> }
function Field({ label, name, defaultValue = '', type = 'text', value, onChange, required = false }: { label: string; name: string; defaultValue?: string; type?: string; value?: string; onChange?: (value: string) => void; required?: boolean }) { return <label>{label}<input name={name} defaultValue={value === undefined ? defaultValue : undefined} value={value} type={type} required={required} onChange={onChange ? (event) => onChange(event.currentTarget.value) : undefined} /></label> }
function ExcelModelSelector({ name, models, value, onChange }: { name: string; models: ExcelMatchModel[]; value: string; onChange: (value: string) => void }) { const selectedModel = models.find((model) => model.tableName === value); const options = excelModelSelectOptions(models, value); return <label className={styles.catalogControl}>模型名称<select aria-label="模型名称" name={name} value={value} required onChange={(event) => onChange(event.currentTarget.value)}><option value="">选择模型名称</option>{options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select>{selectedModel ? <ExcelCatalogExplanation title={selectedModel.mapping} detail={selectedModel.description} /> : value ? <ExcelCatalogExplanation title={`当前配置 → 数据库表 ${value}`} detail="该表不在当前模型目录中，保留它是为了兼容历史方案；保存前请确认表仍然存在。" /> : <ExcelCatalogExplanation title="模型名称 → 数据库表" detail="选择模型后，这里会直接解释对应的数据表，无需另行查表。" />}</label> }
function ExcelModelFieldSelector({ label, name, models, tableName, value, onChange }: { label: string; name: string; models: ExcelMatchModel[]; tableName: string; value: string; onChange: (value: string) => void }) { const selectedModel = models.find((model) => model.tableName === tableName); const fields = selectedModel?.fields ?? []; const selectedField = fields.find((field) => field.columnName === value); const options = excelFieldSelectOptions(fields, value); return <label className={styles.catalogControl}>{label}<select aria-label={label} name={name} value={value} required disabled={!tableName} onChange={(event) => onChange(event.currentTarget.value)}><option value="">选择模型字段</option>{options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select>{selectedField ? <ExcelModelFieldExplanation field={selectedField} /> : value ? <ExcelCatalogExplanation title={`当前配置字段 → ${tableName}.${value}`} detail="该字段不在当前模型目录中，已作为历史配置保留；保存前请确认字段仍然存在。" /> : selectedModel ? <ExcelCatalogExplanation title={`${selectedModel.modelName}.字段 → ${selectedModel.tableName}.数据库列`} detail={`当前模型提供 ${fields.length} 个字段可选。`} /> : <ExcelCatalogExplanation title="模型字段 → 数据库列" detail="请先选择模型，再从该模型的字段列表中选择。" />}</label> }
function ExcelModelFieldExplanation({ field }: { field: ExcelMatchModelField }) { const typeDetail = field.dataType && !field.description.includes(field.dataType) ? `；数据库类型 ${field.dataType}` : ''; return <ExcelCatalogExplanation title={field.mapping} detail={`${field.description}${typeDetail}；${field.nullable ? '允许为空' : '不允许为空'}`} /> }
function ExcelCatalogExplanation({ title, detail }: { title: string; detail: string }) { return <span className={styles.catalogExplanation}><strong>{title}</strong><small>{detail}</small></span> }
