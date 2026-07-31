export type OrderPushSkipPolicyInput = {
  cycle: number
  skip: number
}

export type SingleFlightLock = {
  current: boolean
}

export function validateOrderPushSkipPolicy(inputs: OrderPushSkipPolicyInput[]) {
  const invalid = inputs.some((item) => !Number.isInteger(item.cycle) || !Number.isInteger(item.skip)
    || item.cycle < 0 || item.skip < 0 || (item.cycle === 0 && item.skip !== 0)
    || (item.cycle > 0 && item.skip >= item.cycle))
  return invalid ? '循环和少推数量必须是非负整数；循环为 0 时少推单数必须为 0，启用少推时必须小于循环总单数。' : ''
}

export function runSingleFlight<T>(lock: SingleFlightLock, operation: () => Promise<T>) {
  if (lock.current) return null
  lock.current = true
  return operation().finally(() => { lock.current = false })
}
