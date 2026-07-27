import { type ReactNode } from 'react'
import { mallWeatherChartSegments, mallWeatherMetric } from './mallWeather'

type MallWeatherChartProps = {
  title: string
  detail: string
  unit: string
  icon: ReactNode
  series: Array<{ time: string; value?: number }>
  showDetails?: boolean
}

export function MallWeatherChart({
  title,
  detail,
  unit,
  icon,
  series,
  showDetails = true,
}: MallWeatherChartProps) {
  const values = series.map((item) => item.value)
  const segments = mallWeatherChartSegments(values, 640, 140)
  const available = series.filter(
    (item): item is { time: string; value: number } =>
      typeof item.value === 'number' && Number.isFinite(item.value),
  )
  const minimum = available.length ? Math.min(...available.map((item) => item.value)) : undefined
  const maximum = available.length ? Math.max(...available.map((item) => item.value)) : undefined
  const startTime = available[0]?.time || '未知'
  const endTime = available[available.length - 1]?.time || '未知'
  const description = `${startTime} 至 ${endTime}，最低 ${mallWeatherMetric(minimum, unit)}，最高 ${mallWeatherMetric(maximum, unit)}`

  return (
    <article className="workbench-panel mall-weather-chart-panel">
      <div className="mall-weather-section-title">
        <div><strong>{title}</strong><span>{detail} · {unit}</span></div>
        {icon}
      </div>
      {segments.length === 0 ? (
        <div className="empty-state" role="status">
          <strong>暂无趋势数据</strong>
          <span>当前时间窗口没有可绘制的数据。</span>
        </div>
      ) : (
        <>
          <svg viewBox="0 0 640 160" role="img" aria-label={`${title}趋势图`} preserveAspectRatio="none">
            <title>{title}</title>
            <desc>{description}</desc>
            <path d="M0 140 H640" />
            <path d="M0 70 H640" />
            {segments.map((points, index) => points.includes(' ')
              ? <polyline points={points} key={`${points}-${index}`} />
              : <ChartPoint point={points} key={`${points}-${index}`} />)}
          </svg>
          <p className="mall-weather-chart-summary">{description}</p>
          {showDetails && (
            <details className="mall-weather-chart-data">
              <summary>查看趋势明细（{series.length} 条）</summary>
              <div className="data-table-wrap">
                <table className="data-table">
                  <thead><tr><th scope="col">时间</th><th scope="col">数值</th></tr></thead>
                  <tbody>
                    {series.map((item, index) => (
                      <tr key={`${item.time}-${index}`}>
                        <td>{item.time || '未知'}</td>
                        <td>{mallWeatherMetric(item.value, unit)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </details>
          )}
        </>
      )}
    </article>
  )
}

function ChartPoint({ point }: { point: string }) {
  const [cx = '0', cy = '0'] = point.split(',')
  return <circle cx={cx} cy={cy} r="4" />
}
