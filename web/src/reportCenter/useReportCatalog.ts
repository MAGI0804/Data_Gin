import { useCallback, useEffect, useRef, useState } from 'react'
import { getReportCatalog, type ReportCenterClient } from './api'
import type { ReportCatalogQuery, ReportSummary } from './types'

type ReportCatalogLoader = typeof getReportCatalog

export function useReportCatalog(client: ReportCenterClient, query: ReportCatalogQuery, loadCatalog: ReportCatalogLoader = getReportCatalog) {
  const { afterId, category, limit, search } = query
  const [items, setItems] = useState<ReportSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [loadingMore, setLoadingMore] = useState(false)
  const [hasMore, setHasMore] = useState(false)
  const [nextAfterId, setNextAfterId] = useState(0)
  const [reloadVersion, setReloadVersion] = useState(0)
  const requestGeneration = useRef(0)

  useEffect(() => {
    const generation = requestGeneration.current + 1
    requestGeneration.current = generation
    const controller = new AbortController()
    setLoading(true)
    setLoadingMore(false)
    setHasMore(false)
    setNextAfterId(0)
    setError('')
    void loadCatalog(client, { afterId, category, limit, search }, controller.signal).then((result) => {
      if (controller.signal.aborted || requestGeneration.current !== generation) return
      if (!result.ok) {
        setError(result.error)
        setLoading(false)
        return
      }
      setItems(result.page.items)
      setHasMore(result.page.hasMore)
      setNextAfterId(result.page.nextAfterId)
      setLoading(false)
    })
    return () => controller.abort()
  }, [afterId, category, client, limit, loadCatalog, reloadVersion, search])

  const reload = useCallback(() => setReloadVersion((version) => version + 1), [])
  const loadMore = useCallback(async () => {
    if (loading || loadingMore || !hasMore || nextAfterId < 1) return
    const generation = requestGeneration.current
    setLoadingMore(true)
    setError('')
    const result = await loadCatalog(client, { category, limit, search, afterId: nextAfterId })
    if (requestGeneration.current !== generation) return
    if (!result.ok) {
      setError(result.error)
      setLoadingMore(false)
      return
    }
    setItems((current) => [...current, ...result.page.items])
    setHasMore(result.page.hasMore)
    setNextAfterId(result.page.nextAfterId)
    setLoadingMore(false)
  }, [category, client, hasMore, limit, loadCatalog, loading, loadingMore, nextAfterId, search])
  return { items, loading, loadingMore, error, hasMore, reload, loadMore }
}
