import { useCallback, useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import type { ClientResponse } from '../api/client'
import type { LegacyTask } from '../backfillPages/youzanDistributionSupport'
import type { TransformRule } from '../configurationPages/ruleContracts'
import type { DestinationDefinition, SourceDefinition } from '../configurationPages/types'
import { parseDataStatisticsSummary, parseHealthSummary, parseMallWeatherMetricsSummary } from '../monitoring'
import { parseDeliveryLog, parsePipelineRun } from '../monitoringPages/contracts'
import type { DeliveryLog, PipelineRun } from '../monitoringPages/types'
import { parseMonitoringPage } from '../monitoringRecords'
import { pipelineListPath } from '../pipelineRun'
import type { NavKey } from './navigation'
import type { ConsoleSessionState } from './useConsoleSession'
import type { MonitoringSnapshot, PipelineDefinition, WorkspaceApiClient } from './WorkspaceRouter'

export function useWorkspaceData(
  activeNav: NavKey,
  client: WorkspaceApiClient,
  token: string,
  sessionState: ConsoleSessionState,
  setResult: Dispatch<SetStateAction<ClientResponse | null>>,
) {
  const [refreshing, setRefreshing] = useState(false)
  const [refreshVersion, setRefreshVersion] = useState(0)
  const [workspaceError, setWorkspaceError] = useState('')
  const [runs, setRuns] = useState<PipelineRun[]>([])
  const [stepRunFocusID, setStepRunFocusID] = useState<number | null>(null)
  const requestRef = useRef<AbortController | null>(null)
  const [pipelines, setPipelines] = useState<PipelineDefinition[]>([])
  const [sources, setSources] = useState<SourceDefinition[]>([])
  const [transformRules, setTransformRules] = useState<TransformRule[]>([])
  const [destinations, setDestinations] = useState<DestinationDefinition[]>([])
  const [deliveryLogs, setDeliveryLogs] = useState<DeliveryLog[]>([])
  const [overviewTotals, setOverviewTotals] = useState({ runs: null as number | null, deliveryLogs: null as number | null })
  const [monitoring, setMonitoring] = useState<MonitoringSnapshot>({ statistics: null, weather: null, health: null })
  const [monitoringStale, setMonitoringStale] = useState(false)
  const [legacyTasks, setLegacyTasks] = useState<LegacyTask[]>([])

  const refresh = useCallback(async (showResult = false) => {
    if (!token) return
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    setRefreshing(true)
    setWorkspaceError('')
    try {
      const get = (path: string) => client(path, { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
      if (activeNav === 'overview') {
        const startTime = monitoringDayStartTime()
        const [runResult, logResult] = await Promise.all([
          get(`/v1/runs?page=1&page_size=100&start_time=${encodeURIComponent(startTime)}`),
          get(`/v1/delivery-logs?page=1&page_size=100&start_time=${encodeURIComponent(startTime)}`),
        ])
        if (!controller.signal.aborted) {
          const runPage = runResult.ok ? parseMonitoringPage<unknown>(runResult.data, 'runs') : null
          const parsedRuns = runPage?.list.map(parsePipelineRun) ?? []
          if (runPage && parsedRuns.every((run): run is PipelineRun => run !== null)) {
            setRuns(parsedRuns)
            setOverviewTotals((current) => ({ ...current, runs: runPage.pagination.total }))
          } else if (runResult.ok) {
            setRuns(readList<PipelineRun>(runResult, 'runs'))
            setOverviewTotals((current) => ({ ...current, runs: null }))
          }
          const logPage = logResult.ok ? parseMonitoringPage<unknown>(logResult.data, 'logs') : null
          const parsedLogs = logPage?.list.map(parseDeliveryLog) ?? []
          if (logPage && parsedLogs.every((log): log is DeliveryLog => log !== null)) {
            setDeliveryLogs(parsedLogs)
            setOverviewTotals((current) => ({ ...current, deliveryLogs: logPage.pagination.total }))
          } else if (logResult.ok) {
            setDeliveryLogs(readList<DeliveryLog>(logResult, 'logs'))
            setOverviewTotals((current) => ({ ...current, deliveryLogs: null }))
          }
        }
      } else if (activeNav === 'step_runs') {
        const response = await get('/v1/runs?limit=50')
        if (!controller.signal.aborted && response.ok) setRuns(readList<PipelineRun>(response, 'runs'))
      } else if (activeNav === 'runs') {
        const response = await get(pipelineListPath())
        if (!controller.signal.aborted) {
          if (response.ok) setPipelines(readList<PipelineDefinition>(response, 'pipelines'))
          else setWorkspaceError('可执行流水线加载失败，已保留上一次成功数据。')
        }
      } else if (activeNav === 'rules') {
        const response = await get('/v1/sources')
        if (!controller.signal.aborted) {
          if (response.ok) setSources(readList<SourceDefinition>(response, 'sources'))
          else setWorkspaceError('规则来源加载失败，已保留上一次成功数据。')
        }
      } else if (activeNav === 'tasks') {
        const [sourceResult, destinationResult] = await Promise.all([get('/v1/sources'), get('/v1/destinations')])
        if (!controller.signal.aborted) {
          if (sourceResult.ok) setSources(readList<SourceDefinition>(sourceResult, 'sources'))
          if (destinationResult.ok) setDestinations(readList<DestinationDefinition>(destinationResult, 'destinations'))
          if (!sourceResult.ok || !destinationResult.ok) setWorkspaceError('推送任务关联配置加载不完整，已保留上一次成功数据。')
        }
      } else if (activeNav === 'youzan_distribution') {
        const response = await get('/v1/legacy-tasks')
        if (!controller.signal.aborted && response.ok) setLegacyTasks(readList<LegacyTask>(response, 'tasks'))
      }
      if (!controller.signal.aborted && showResult) {
        setResult({ ok: true, status: 200, data: { refreshed_at: new Date().toISOString() } })
      }
    } finally {
      if (requestRef.current === controller) {
        requestRef.current = null
        setRefreshing(false)
        if (!controller.signal.aborted) setRefreshVersion((version) => version + 1)
      }
    }
  }, [activeNav, client, setResult, token])

  useEffect(() => {
    if (sessionState === 'authenticated') void refresh(false)
    return () => requestRef.current?.abort()
  }, [refresh, sessionState])

  useEffect(() => {
    if (sessionState !== 'authenticated' || activeNav !== 'overview') return
    const controller = new AbortController()
    void Promise.all([
      client('/v1/data/statistics', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }),
      client('/v1/mall-weather/metrics', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true }),
      client('/health', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true, acceptBareJSONSuccess: true }),
    ]).then(([statisticsResponse, weatherResponse, healthResponse]) => {
      if (controller.signal.aborted) return
      const nextStatistics = statisticsResponse.ok ? parseDataStatisticsSummary(statisticsResponse.data) : null
      const nextWeather = weatherResponse.ok ? parseMallWeatherMetricsSummary(weatherResponse.data) : null
      const nextHealth = healthResponse.ok ? parseHealthSummary(healthResponse.data) : null
      setMonitoring((current) => ({
        statistics: nextStatistics ?? current.statistics,
        weather: nextWeather ?? current.weather,
        health: nextHealth ?? current.health,
      }))
      setMonitoringStale(!nextStatistics || !nextWeather || !nextHealth)
    })
    return () => controller.abort()
  }, [activeNav, client, sessionState])

  return {
    deliveryLogs, destinations, legacyTasks, monitoring, monitoringStale, overviewTotals, pipelines,
    refresh, refreshing, refreshVersion, runs, setStepRunFocusID, setTransformRules, sources,
    stepRunFocusID, transformRules, workspaceError,
  }
}

function readList<T>(result: { data: unknown }, key: string): T[] {
  const value = readDataField(result.data, key)
  return Array.isArray(value) ? (value as T[]) : []
}

function readDataField(data: unknown, key: string) {
  if (!data || typeof data !== 'object') return undefined
  return (data as { data?: Record<string, unknown> }).data?.[key]
}

function monitoringDayStartTime() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}T00:00`
}
