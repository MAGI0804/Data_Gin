import { useCallback, useEffect, useState } from 'react'
import { getReportCatalog, type ReportCenterClient } from './api'
import type { ReportCatalogQuery, ReportSummary } from './types'

export function useReportCatalog(client: ReportCenterClient, query: ReportCatalogQuery) {
  const { afterId, category, limit, search } = query
  const [items, setItems] = useState<ReportSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadVersion, setReloadVersion] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError('')
    void getReportCatalog(client, { afterId, category, limit, search }, controller.signal).then((result) => {
      if (controller.signal.aborted) return
      if (!result.ok) {
        setError(result.error)
        setLoading(false)
        return
      }
      setItems(result.page.items)
      setLoading(false)
    })
    return () => controller.abort()
  }, [afterId, category, client, limit, reloadVersion, search])

  const reload = useCallback(() => setReloadVersion((version) => version + 1), [])
  return { items, loading, error, reload }
}
