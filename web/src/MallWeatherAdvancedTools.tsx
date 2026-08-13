import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { MallWeatherExportProfilePanel } from './MallWeatherExportProfilePanel'
import { DataTable, FeedbackState, MetricStrip } from './ui'
import { mallWeatherCapacityPlanPath, parseMallWeatherCapacityPlan, type MallWeatherCapacityPlan, type MallWeatherCapacityPlanInput } from './mallWeatherCapacityPlan'
import { parseMallWeatherMetricsSummary, type MallWeatherMetricsSummary } from './monitoring'
import styles from './MallWeatherAdvancedTools.module.css'

type ApiResult = { ok: boolean; status: number; data: unknown }
type ApiClient = (path: string, options?: {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  body?: unknown
  headers?: Record<string, string>
  showResult?: boolean
  silentLoading?: boolean
  signal?: AbortSignal
}) => Promise<ApiResult>

const defaultCapacityInput: MallWeatherCapacityPlanInput = {
  mallCount: '', providerQps: '', hourlySteps: '360', dailySteps: '15', lifeIndexDays: '15', alertsPerMall: '0', feishuBatchRows: '200',
}

export function MallWeatherAdvancedTools({ client }: { client: ApiClient }) {
  return (
    <details className={styles.tools}>
      <summary>天气服务高级配置与运营工具</summary>
      <div className={styles.stack}>
        <CapacityPlanPanel client={client} />
        <OperationalMetricsPanel client={client} />
        <div className={styles.section}>
          <MallWeatherExportProfilePanel client={client} />
        </div>
      </div>
    </details>
  )
}

function CapacityPlanPanel({ client }: { client: ApiClient }) {
  const [form, setForm] = useState(defaultCapacityInput)
  const [plan, setPlan] = useState<MallWeatherCapacityPlan | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const controllerRef = useRef<AbortController | null>(null)

  useEffect(() => () => controllerRef.current?.abort(), [])

  function change(field: keyof MallWeatherCapacityPlanInput, value: string) {
    setForm((current) => ({ ...current, [field]: value }))
    setError('')
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    let path: string
    try {
      path = mallWeatherCapacityPlanPath(form)
    } catch {
      setError('请输入有效的目标商场数、供应商 QPS 和各数据集范围。')
      return
    }
    controllerRef.current?.abort()
    const controller = new AbortController()
    controllerRef.current = controller
    setSubmitting(true)
    setError('')
    try {
      const response = await client(path, { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal })
      if (controller.signal.aborted) return
      if (!response.ok) {
        setError(requestError(response.status, '容量规划计算失败', '当前账号缺少 weather.config.manage 权限'))
        return
      }
      const parsed = parseMallWeatherCapacityPlan(response.data)
      if (!parsed) {
        setError('容量规划响应格式不正确，请联系管理员。')
        return
      }
      setPlan(parsed)
    } catch {
      if (!controller.signal.aborted) setError('容量规划请求未完成，请检查网络后重试。')
    } finally {
      if (controllerRef.current === controller) {
        controllerRef.current = null
        if (!controller.signal.aborted) setSubmitting(false)
      }
    }
  }

  return <section className={styles.section} aria-busy={submitting}>
    <SectionTitle title="天气容量规划" description="按规划目标测算供应商调用、数据库写入和飞书批次；不会修改任何配置。" />
    <form className={styles.capacityForm} onSubmit={submit}>
      <NumberField label="目标商场数 *" name="capacityMallCount" min="1" max="100000" value={form.mallCount} disabled={submitting} onChange={(value) => change('mallCount', value)} />
      <NumberField label="供应商 QPS *" name="capacityProviderQps" min="0" max="10000" step="any" value={form.providerQps} disabled={submitting} onChange={(value) => change('providerQps', value)} />
      <NumberField label="逐小时预报步数" name="capacityHourlySteps" min="1" max="360" value={form.hourlySteps} disabled={submitting} onChange={(value) => change('hourlySteps', value)} />
      <NumberField label="逐日预报天数" name="capacityDailySteps" min="1" max="15" value={form.dailySteps} disabled={submitting} onChange={(value) => change('dailySteps', value)} />
      <NumberField label="生活指数天数" name="capacityLifeIndexDays" min="1" max="15" value={form.lifeIndexDays} disabled={submitting} onChange={(value) => change('lifeIndexDays', value)} />
      <NumberField label="每商场预警数" name="capacityAlertsPerMall" min="0" max="256" value={form.alertsPerMall} disabled={submitting} onChange={(value) => change('alertsPerMall', value)} />
      <NumberField label="飞书每批行数" name="capacityFeishuBatchRows" min="1" max="500" value={form.feishuBatchRows} disabled={submitting} onChange={(value) => change('feishuBatchRows', value)} />
      <div className={styles.actions}><button className={styles.primary} type="submit" disabled={submitting}>{submitting ? '计算中' : '计算容量'}</button></div>
    </form>
    {plan ? <CapacityResult plan={plan} /> : null}
    {error ? <p className={styles.error} role="alert">{error}</p> : null}
  </section>
}

function CapacityResult({ plan }: { plan: MallWeatherCapacityPlan }) {
  const metrics = [
    ['每日供应商请求', String(plan.providerRequests)], ['预计耗时', `${plan.providerDrainSeconds.toFixed(1)} 秒`],
    ['一小时最低 QPS', plan.minimumQpsForOneHourDrain.toFixed(2)], ['数据库总行数', String(plan.totalDatabaseRows)],
    ['数据库批次', String(plan.totalDatabaseBatches)], ['飞书批次', String(plan.totalFeishuBatches)],
    ['飞书每批行数', String(plan.feishuBatchRows)], ['规划商场数', String(plan.mallCount)],
  ]
  return <>
    <MetricStrip label="天气容量规划指标" items={metrics.map(([label, value]) => ({ key: label, label, value }))} />
    <DataTable density="compact" minWidth={620} scrollLabel="各天气数据集容量明细">
      <caption>各天气数据集容量明细</caption>
      <thead><tr><th scope="col">数据集</th><th scope="col">行数</th><th scope="col">数据库批次</th><th scope="col">飞书批次</th></tr></thead>
      <tbody>{plan.datasets.map((dataset) => <tr key={dataset.kind}><td>{dataset.kind}</td><td>{dataset.rows}</td><td>{dataset.databaseBatches}</td><td>{dataset.feishuBatches}</td></tr>)}</tbody>
    </DataTable>
  </>
}

function OperationalMetricsPanel({ client }: { client: ApiClient }) {
  const [metrics, setMetrics] = useState<MallWeatherMetricsSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const controllerRef = useRef<AbortController | null>(null)
  const requestSequence = useRef(0)

  const loadMetrics = useCallback(async () => {
    controllerRef.current?.abort()
    const controller = new AbortController()
    controllerRef.current = controller
    const sequence = ++requestSequence.current
    setLoading(true)
    setError('')
    try {
      const response = await client('/v1/mall-weather/metrics', { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal })
      if (controller.signal.aborted || sequence !== requestSequence.current) return
      if (!response.ok) {
        setError(metricsRequestError(response.status))
        return
      }
      const parsed = parseMallWeatherMetricsSummary(response.data)
      if (!parsed) {
        setError('天气运维指标响应格式不正确，请稍后重试。')
        return
      }
      setMetrics(parsed)
    } catch {
      if (!controller.signal.aborted && sequence === requestSequence.current) setError('天气运维指标加载异常，请检查网络后重试。')
    } finally {
      if (!controller.signal.aborted && sequence === requestSequence.current) setLoading(false)
    }
  }, [client])

  useEffect(() => {
    void loadMetrics()
    return () => controllerRef.current?.abort()
  }, [loadMetrics])

  const values = metrics ? [
    ['运维告警', String(metrics.totalAlerts)], ['严重告警', String(metrics.criticalAlerts)], ['警告告警', String(metrics.warningAlerts)],
    ['触发中告警', String(metrics.firingAlerts)], ['采集次数', metricInteger(metrics.fetchTotal)], ['供应商限流', metricInteger(metrics.providerRateLimited)],
    ['供应商鉴权失败', metricInteger(metrics.providerAuthFailures)], ['采集失败', metricInteger(metrics.failedFetches)],
    ['最大数据时效', metricDuration(metrics.maxDataAgeSeconds)], ['最大队列等待', metricDuration(metrics.maxQueueLagSeconds)],
  ] : []

  return <section className={styles.section} aria-busy={loading} aria-label="天气运维指标">
    <SectionTitle title="天气运维指标" description="仅展示聚合运行指标与告警数量，不展示第三方响应、标签或敏感配置。" action={<button type="button" onClick={() => void loadMetrics()} disabled={loading}>{loading ? '加载中' : '刷新指标'}</button>} />
    {metrics ? <MetricStrip label="天气运维指标" items={values.map(([label, value]) => ({ key: label, label, value }))} /> : null}
    {loading && !metrics ? <FeedbackState kind="loading" title="正在加载天气运维指标" /> : null}
    {!loading && !metrics && !error ? <FeedbackState kind="empty" title="暂无天气运维指标" description="尚未采集到可展示的聚合运行数据。" /> : null}
    {error ? <FeedbackState kind="error" title={metrics ? '天气运维指标刷新失败' : '天气运维指标不可用'} description={metrics ? `${error} 当前展示最近一次成功数据。` : error} action={<button type="button" onClick={() => void loadMetrics()}>重试</button>} /> : null}
  </section>
}

function NumberField({ label, name, min, max, step, value, disabled, onChange }: { label: string; name: string; min: string; max: string; step?: string; value: string; disabled: boolean; onChange: (value: string) => void }) {
  return <label><span>{label}</span><input name={name} inputMode={step === 'any' ? 'decimal' : 'numeric'} type="number" min={min} max={max} step={step} value={value} onChange={(event) => onChange(event.currentTarget.value)} required disabled={disabled} /></label>
}

function SectionTitle({ title, description, action }: { title: string; description: string; action?: ReactNode }) {
  return <div className={styles.sectionTitle}><div><strong>{title}</strong><span>{description}</span></div>{action}</div>
}

function metricInteger(value: number | null) {
  return value === null ? '—' : new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 0 }).format(value)
}

function metricDuration(value: number | null) {
  return value === null ? '—' : `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 1 }).format(value)} 秒`
}

function metricsRequestError(status: number) {
  if (status === 0) return '无法连接服务，请检查网络后重试。'
  if (status === 403) return '当前账号缺少天气运维指标查看权限。'
  if (status === 429) return '请求过于频繁，请稍后重试。'
  return '天气运维指标暂时不可用，请稍后重试。'
}

function requestError(status: number, fallback: string, forbidden: string) {
  if (status === 0) return '无法连接服务，请检查网络后重试'
  if (status === 403) return forbidden
  if (status === 404) return '商场或天气数据不存在'
  if (status === 422) return '商场坐标尚未确认，暂时无法查询天气'
  return `${fallback}（HTTP ${status}）`
}
