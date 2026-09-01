export type OverviewSignalAccess = {
  runs: boolean
  deliveryLogs: boolean
  statistics: boolean
  weather: boolean
}

export type OverviewRequestPlan = {
  runs: string | null
  deliveryLogs: string | null
  statistics: string | null
  weather: string | null
  health: string
}

export type OverviewDataState<Run, DeliveryLog, Statistics, Weather, Health> = {
  runs: Run[]
  deliveryLogs: DeliveryLog[]
  overviewTotals: { runs: number | null; deliveryLogs: number | null }
  monitoring: { statistics: Statistics | null; weather: Weather | null; health: Health | null }
}

export function overviewSignalAccess(permissions: readonly string[]): OverviewSignalAccess {
  return {
    runs: permissions.includes('pipeline.read') || permissions.includes('pipeline.manage'),
    deliveryLogs: permissions.includes('delivery.read') || permissions.includes('delivery.manage'),
    statistics: permissions.includes('data.read'),
    weather: permissions.includes('weather.read'),
  }
}

export function overviewRequestPlan(permissions: readonly string[], startTime: string): OverviewRequestPlan {
  const access = overviewSignalAccess(permissions)
  const encodedStartTime = encodeURIComponent(startTime)
  return {
    runs: access.runs ? `/v1/runs?page=1&page_size=100&start_time=${encodedStartTime}` : null,
    deliveryLogs: access.deliveryLogs ? `/v1/delivery-logs?page=1&page_size=100&start_time=${encodedStartTime}` : null,
    statistics: access.statistics ? '/v1/data/statistics' : null,
    weather: access.weather ? '/v1/mall-weather/metrics' : null,
    health: '/health',
  }
}

export function canCommitOverviewResponse(signal: Pick<AbortSignal, 'aborted'>): boolean {
  return !signal.aborted
}

export function restrictOverviewData<Run, DeliveryLog, Statistics, Weather, Health>(
  current: OverviewDataState<Run, DeliveryLog, Statistics, Weather, Health>,
  permissions: readonly string[],
  isOverviewActive = true,
): OverviewDataState<Run, DeliveryLog, Statistics, Weather, Health> {
  if (!isOverviewActive) {
    const isEmpty = current.runs.length === 0
      && current.deliveryLogs.length === 0
      && current.overviewTotals.runs === null
      && current.overviewTotals.deliveryLogs === null
      && current.monitoring.statistics === null
      && current.monitoring.weather === null
      && current.monitoring.health === null
    if (isEmpty) return current
    return {
      runs: [],
      deliveryLogs: [],
      overviewTotals: { runs: null, deliveryLogs: null },
      monitoring: { statistics: null, weather: null, health: null },
    }
  }

  const access = overviewSignalAccess(permissions)
  const hasHiddenRuns = !access.runs && (current.runs.length > 0 || current.overviewTotals.runs !== null)
  const hasHiddenDeliveryLogs = !access.deliveryLogs && (current.deliveryLogs.length > 0 || current.overviewTotals.deliveryLogs !== null)
  const hasHiddenStatistics = !access.statistics && current.monitoring.statistics !== null
  const hasHiddenWeather = !access.weather && current.monitoring.weather !== null
  if (!hasHiddenRuns && !hasHiddenDeliveryLogs && !hasHiddenStatistics && !hasHiddenWeather) return current

  return {
    runs: access.runs ? current.runs : [],
    deliveryLogs: access.deliveryLogs ? current.deliveryLogs : [],
    overviewTotals: {
      runs: access.runs ? current.overviewTotals.runs : null,
      deliveryLogs: access.deliveryLogs ? current.overviewTotals.deliveryLogs : null,
    },
    monitoring: {
      statistics: access.statistics ? current.monitoring.statistics : null,
      weather: access.weather ? current.monitoring.weather : null,
      health: current.monitoring.health,
    },
  }
}
