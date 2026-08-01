export type SingleFlightLock = {
  current: boolean
}

export function runSingleFlight<T>(lock: SingleFlightLock, operation: () => Promise<T>) {
  if (lock.current) return null
  lock.current = true
  return operation().finally(() => { lock.current = false })
}
