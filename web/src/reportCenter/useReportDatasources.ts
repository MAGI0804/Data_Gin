import { useCallback, useEffect, useState } from 'react'
import { getReportDatasources, type ReportCenterClient } from './api'
import type { ReportDatasource } from './types'

export function useReportDatasources(client: ReportCenterClient, enabled = true) {
  const [revision, setRevision] = useState(0)
  const [state, setState] = useState<{ items: ReportDatasource[]; loading: boolean; error: string }>({ items: [], loading: enabled, error: '' })
  const reload = useCallback(() => setRevision((value) => value + 1), [])

  useEffect(() => {
    if (!enabled) return
    const controller = new AbortController()
    setState((current) => ({ ...current, loading: true, error: '' }))
    void getReportDatasources(client, controller.signal).then((response) => {
      if (controller.signal.aborted) return
      setState(response.ok
        ? { items: response.data, loading: false, error: '' }
        : { items: [], loading: false, error: response.error })
    })
    return () => controller.abort()
  }, [client, enabled, revision])

  return { ...state, reload }
}
