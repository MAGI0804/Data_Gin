export type GuardedRequest = {
  signal: AbortSignal
  isCurrent: () => boolean
}

export function createLatestRequestGuard() {
  let generation = 0
  let controller: AbortController | null = null
  return {
    begin(): GuardedRequest {
      controller?.abort()
      controller = new AbortController()
      const requestGeneration = ++generation
      return {
        signal: controller.signal,
        isCurrent: () => requestGeneration === generation && !controller?.signal.aborted,
      }
    },
    cancel() {
      generation++
      controller?.abort()
      controller = null
    },
  }
}
