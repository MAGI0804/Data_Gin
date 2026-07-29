import { type KeyboardEvent, type PointerEvent, type ReactNode, useId, useState } from 'react'
import {
  mallWeatherChartPoints,
  mallWeatherChartSegments,
  mallWeatherMetric,
  mallWeatherNearestChartPoint,
} from './mallWeather'

const chartWidth = 640
const chartHeight = 140
const chartViewBoxHeight = 160

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
  const tooltipID = useId()
  const [activeIndex, setActiveIndex] = useState<number | null>(null)
  const values = series.map((item) => item.value)
  const points = mallWeatherChartPoints(values, chartWidth, chartHeight)
  const segments = mallWeatherChartSegments(values, chartWidth, chartHeight)
  const available = series.filter(
    (item): item is { time: string; value: number } =>
      typeof item.value === 'number' && Number.isFinite(item.value),
  )
  const minimum = available.length ? Math.min(...available.map((item) => item.value)) : undefined
  const maximum = available.length ? Math.max(...available.map((item) => item.value)) : undefined
  const startTime = available[0]?.time || '未知'
  const endTime = available[available.length - 1]?.time || '未知'
  const description = `${startTime} 至 ${endTime}，最低 ${mallWeatherMetric(minimum, unit)}，最高 ${mallWeatherMetric(maximum, unit)}`
  const activePosition = points.findIndex((point) => point.index === activeIndex)
  const activePoint = activePosition >= 0 ? points[activePosition] : undefined
  const selectedPosition = activePosition >= 0 ? activePosition : 0
  const selectedPoint = points[selectedPosition]
  const selectedItem = selectedPoint ? series[selectedPoint.index] : undefined
  const activeItem = activePoint ? series[activePoint.index] : undefined

  const selectNearestPoint = (event: PointerEvent<SVGSVGElement>) => {
    const bounds = event.currentTarget.getBoundingClientRect()
    if (bounds.width <= 0) return
    const pointerX = Math.max(0, Math.min(chartWidth, (event.clientX - bounds.left) / bounds.width * chartWidth))
    const nearest = mallWeatherNearestChartPoint(points, pointerX)
    if (nearest) setActiveIndex(nearest.index)
  }

  const selectPointByKeyboard = (event: KeyboardEvent<SVGSVGElement>) => {
    let nextPosition = selectedPosition
    if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') nextPosition = Math.max(0, selectedPosition - 1)
    else if (event.key === 'ArrowRight' || event.key === 'ArrowUp') nextPosition = Math.min(points.length - 1, selectedPosition + 1)
    else if (event.key === 'Home') nextPosition = 0
    else if (event.key === 'End') nextPosition = points.length - 1
    else if (event.key === 'Escape') {
      setActiveIndex(null)
      return
    } else return
    event.preventDefault()
    setActiveIndex(points[nextPosition]?.index ?? null)
  }

  const tooltipHorizontal = activePoint && activePoint.x <= chartWidth * 0.15
    ? 'start'
    : activePoint && activePoint.x >= chartWidth * 0.85 ? 'end' : 'center'
  const tooltipVertical = activePoint && activePoint.y < chartViewBoxHeight * 0.3 ? 'below' : 'above'

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
          <div className="mall-weather-chart-visual">
            <svg
              viewBox={`0 0 ${chartWidth} ${chartViewBoxHeight}`}
              role="slider"
              tabIndex={0}
              aria-label={`${title}时间点`}
              aria-orientation="horizontal"
              aria-valuemin={0}
              aria-valuemax={Math.max(points.length - 1, 0)}
              aria-valuenow={selectedPosition}
              aria-valuetext={selectedItem ? `${selectedItem.time || '时间未知'}，${mallWeatherMetric(selectedItem.value, unit)}` : '暂无数据'}
              aria-describedby={activePoint ? tooltipID : undefined}
              preserveAspectRatio="none"
              onFocus={() => setActiveIndex((current) => points.some((point) => point.index === current) ? current : points[0]?.index ?? null)}
              onBlur={() => setActiveIndex(null)}
              onKeyDown={selectPointByKeyboard}
              onPointerMove={selectNearestPoint}
              onPointerLeave={(event) => {
                if (!event.currentTarget.matches(':focus')) setActiveIndex(null)
              }}
            >
              <title>{title}</title>
              <desc>{description}。鼠标悬停或使用方向键可查看对应时间点。</desc>
              <path d={`M0 ${chartHeight} H${chartWidth}`} />
              <path d={`M0 ${chartHeight / 2} H${chartWidth}`} />
              {segments.map((segment, index) => segment.includes(' ')
                ? <polyline points={segment} key={`${segment}-${index}`} />
                : <ChartPoint point={segment} key={`${segment}-${index}`} />)}
              {activePoint && (
                <g className="mall-weather-chart-active" aria-hidden="true">
                  <line x1={activePoint.x} x2={activePoint.x} y1="0" y2={chartHeight} />
                  <circle cx={activePoint.x} cy={activePoint.y} r="6" />
                </g>
              )}
            </svg>
            {activePoint && activeItem && (
              <div
                className={`mall-weather-chart-tooltip ${tooltipHorizontal} ${tooltipVertical}`}
                id={tooltipID}
                role="tooltip"
                style={{
                  left: `${activePoint.x / chartWidth * 100}%`,
                  top: `${activePoint.y / chartViewBoxHeight * 100}%`,
                }}
              >
                <strong>{activeItem.time || '时间未知'}</strong>
                <span>{mallWeatherMetric(activeItem.value, unit)}</span>
              </div>
            )}
          </div>
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
