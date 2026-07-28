export const mallWeatherDataNavigationItems = [
  { targetID: 'mall-weather-overview', label: '当前实况', requiresActor: false },
  { targetID: 'mall-weather-minutely', label: '约 1 km 分钟降水', requiresActor: false },
  { targetID: 'mall-weather-hourly', label: '未来逐小时预报', requiresActor: false },
  { targetID: 'mall-weather-daily', label: '15 天逐日预报', requiresActor: false },
  { targetID: 'mall-weather-alerts', label: '气象预警', requiresActor: false },
  { targetID: 'mall-weather-life-indices', label: '15 天生活指数', requiresActor: false },
  { targetID: 'mall-weather-export', label: '下载全部', requiresActor: true },
  { targetID: 'mall-weather-management', label: '管理操作', requiresActor: true },
] as const

const mallWeatherDataNavigationTargetIDs = new Set<string>(
  mallWeatherDataNavigationItems.map((item) => item.targetID),
)

export function navigateMallWeatherSection(
  documentRef: Pick<Document, 'getElementById'>,
  targetID: string,
  reduceMotion = false,
) {
  if (!documentRef || !mallWeatherDataNavigationTargetIDs.has(targetID)) return false
  const target = documentRef.getElementById(targetID)
  if (!target) return false
  if (target.tagName.toUpperCase() === 'DETAILS') {
    const detailsTarget = target as HTMLDetailsElement
    detailsTarget.open = true
  }
  target.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'start' })
  target.focus({ preventScroll: true })
  return true
}
