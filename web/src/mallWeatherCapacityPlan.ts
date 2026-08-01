export type MallWeatherCapacityPlanInput = {
  mallCount: string
  providerQps: string
  hourlySteps: string
  dailySteps: string
  lifeIndexDays: string
  alertsPerMall: string
  feishuBatchRows: string
}

export type MallWeatherCapacityDataset = {
  kind: string
  rows: number
  databaseBatches: number
  feishuBatches: number
}

export type MallWeatherCapacityPlan = {
  mallCount: number
  providerQps: number
  weatherV26ProviderRequestsPerDay: number
  lifeIndexV3ProviderRequestsPerDay: number
  providerRequests: number
  providerDrainSeconds: number
  minimumQpsForOneHourDrain: number
  totalDatabaseRows: number
  totalDatabaseBatches: number
  feishuBatchRows: number
  totalFeishuBatches: number
  datasets: MallWeatherCapacityDataset[]
}

export function mallWeatherCapacityPlanPath(input: MallWeatherCapacityPlanInput) {
  const mallCount = positiveInteger(input.mallCount, 1, 100_000)
  const providerQps = positiveNumber(input.providerQps, 10_000)
  const hourlySteps = positiveInteger(input.hourlySteps, 1, 360)
  const dailySteps = positiveInteger(input.dailySteps, 1, 15)
  const lifeIndexDays = positiveInteger(input.lifeIndexDays, 1, 15)
  const alertsPerMall = nonNegativeInteger(input.alertsPerMall, 256)
  const feishuBatchRows = positiveInteger(input.feishuBatchRows, 1, 500)
  const query = new URLSearchParams({
    mallCount: String(mallCount),
    providerQps: String(providerQps),
    hourlySteps: String(hourlySteps),
    dailySteps: String(dailySteps),
    lifeIndexDays: String(lifeIndexDays),
    alertsPerMall: String(alertsPerMall),
    feishuBatchRows: String(feishuBatchRows),
  })
  return `/v1/mall-weather/capacity-plan?${query.toString()}`
}

export function parseMallWeatherCapacityPlan(payload: unknown): MallWeatherCapacityPlan | null {
  const data = envelopeData(payload)
  if (!data || !positiveSafeInteger(data.mallCount) || !positiveFiniteNumber(data.providerQps) ||
    !nonNegativeSafeInteger(data.weatherV26ProviderRequestsPerDay) || !nonNegativeSafeInteger(data.lifeIndexV3ProviderRequestsPerDay) ||
    !nonNegativeSafeInteger(data.providerRequests) || !nonNegativeFiniteNumber(data.providerDrainSeconds) ||
    !nonNegativeFiniteNumber(data.minimumQpsForOneHourDrain) || !nonNegativeSafeInteger(data.totalDatabaseRows) ||
    !nonNegativeSafeInteger(data.totalDatabaseBatches) || !positiveSafeInteger(data.feishuBatchRows) ||
    !nonNegativeSafeInteger(data.totalFeishuBatches) || !Array.isArray(data.datasets)) return null

  const datasets: MallWeatherCapacityDataset[] = []
  for (const item of data.datasets) {
    if (!isRecord(item) || typeof item.kind !== 'string' || !item.kind.trim() || !nonNegativeSafeInteger(item.rows) ||
      !nonNegativeSafeInteger(item.databaseBatches) || !nonNegativeSafeInteger(item.feishuBatches)) return null
    datasets.push({ kind: item.kind, rows: item.rows, databaseBatches: item.databaseBatches, feishuBatches: item.feishuBatches })
  }
  return {
    mallCount: data.mallCount,
    providerQps: data.providerQps,
    weatherV26ProviderRequestsPerDay: data.weatherV26ProviderRequestsPerDay,
    lifeIndexV3ProviderRequestsPerDay: data.lifeIndexV3ProviderRequestsPerDay,
    providerRequests: data.providerRequests,
    providerDrainSeconds: data.providerDrainSeconds,
    minimumQpsForOneHourDrain: data.minimumQpsForOneHourDrain,
    totalDatabaseRows: data.totalDatabaseRows,
    totalDatabaseBatches: data.totalDatabaseBatches,
    feishuBatchRows: data.feishuBatchRows,
    totalFeishuBatches: data.totalFeishuBatches,
    datasets,
  }
}

function positiveInteger(value: string, minimum: number, maximum: number) {
  const parsed = Number(value)
  if (!Number.isSafeInteger(parsed) || parsed < minimum || parsed > maximum) throw new Error('invalid capacity plan input')
  return parsed
}

function nonNegativeInteger(value: string, maximum: number) {
  const parsed = Number(value)
  if (!Number.isSafeInteger(parsed) || parsed < 0 || parsed > maximum) throw new Error('invalid capacity plan input')
  return parsed
}

function positiveNumber(value: string, maximum: number) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0 || parsed > maximum) throw new Error('invalid capacity plan input')
  return parsed
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function envelopeData(payload: unknown): Record<string, unknown> | null {
  if (!isRecord(payload) || payload.code !== 0 || !isRecord(payload.data)) return null
  return payload.data
}

function positiveSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

function nonNegativeSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

function positiveFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
}

function nonNegativeFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
}
