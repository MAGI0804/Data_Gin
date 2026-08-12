export type AccessAccountRole = { code: string; name: string }

export type AccessAccount = {
  id: number
  account: string
  phone: string
  nickname: string
  status: string
  mallScopeMode: string
  roles: AccessAccountRole[]
  mallIds: number[]
}

export function parseAccessAccounts(payload: unknown): AccessAccount[] {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return []
  const data = (payload as Record<string, unknown>).data
  if (!data || typeof data !== 'object' || Array.isArray(data)) return []
  const accounts = (data as Record<string, unknown>).accounts
  if (!Array.isArray(accounts)) return []
  return accounts.flatMap((value) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return []
    const account = value as Record<string, unknown>
    if (!Number.isSafeInteger(account.id) || Number(account.id) <= 0 || typeof account.account !== 'string') return []
    const roles = Array.isArray(account.roles) ? account.roles.flatMap((role) => {
      if (!role || typeof role !== 'object' || Array.isArray(role)) return []
      const item = role as Record<string, unknown>
      return typeof item.code === 'string' && typeof item.name === 'string' ? [{ code: item.code, name: item.name }] : []
    }) : []
    const mallIds = Array.isArray(account.mallIds)
      ? account.mallIds.filter((id): id is number => Number.isSafeInteger(id) && Number(id) > 0)
      : []
    return [{
      id: Number(account.id),
      account: account.account,
      phone: typeof account.phone === 'string' ? account.phone : '',
      nickname: typeof account.nickname === 'string' ? account.nickname : account.account,
      status: typeof account.status === 'string' ? account.status : '',
      mallScopeMode: typeof account.mallScopeMode === 'string' ? account.mallScopeMode : '',
      roles,
      mallIds,
    }]
  })
}
