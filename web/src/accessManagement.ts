export type AccessAccountRole = { code: string; name: string }

export type AccessRole = {
  id: number
  code: string
  name: string
  description: string
  status: string
  isSystem: boolean
  isSuper: boolean
  permissions: string[]
}

export type AccessPermission = {
  code: string
  name: string
  module: string
  description: string
  riskLevel: string
}

export type RoleCatalogStatus = 'idle' | 'loading' | 'ready' | 'error'

export function accessManagementCapabilities(permissions: readonly string[]) {
  return {
    canAccountRead: permissions.includes('system.account.read'),
    canAccountManage: permissions.includes('system.account.manage'),
    canRoleRead: permissions.includes('system.role.read'),
    canRoleManage: permissions.includes('system.role.manage'),
    canAuditRead: permissions.includes('system.audit.read'),
  }
}

export function canReplaceAccountRoles(status: RoleCatalogStatus) {
  return status === 'ready'
}

export function updateRoleSelection(current: readonly number[], roleID: number, checked: boolean): number[] {
  return checked ? [...new Set([...current, roleID])] : current.filter((id) => id !== roleID)
}

export type AccessMallScopeRequest = {
  mallScopeMode: 'ALL' | 'SELECTED'
  mallIds: number[]
}

export function accessMallScopeRequest(mode: string, rawMallIDs: string): AccessMallScopeRequest | null {
  if (mode === 'ALL') return { mallScopeMode: 'ALL', mallIds: [] }
  if (mode !== 'SELECTED') return null
  const mallIds = [...new Set(rawMallIDs.split(',').map((item) => Number(item.trim())).filter((id) => Number.isSafeInteger(id) && id > 0))]
  return mallIds.length > 0 ? { mallScopeMode: 'SELECTED', mallIds } : null
}

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

export function parseCreatedAccessRole(payload: unknown): AccessRole | null {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return null
  const data = (payload as Record<string, unknown>).data
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  const role = (data as Record<string, unknown>).role
  if (!role || typeof role !== 'object' || Array.isArray(role)) return null
  const item = role as Record<string, unknown>
  if (!Number.isSafeInteger(item.id) || Number(item.id) <= 0 || typeof item.code !== 'string' || typeof item.name !== 'string' ||
    typeof item.description !== 'string' || typeof item.status !== 'string' || typeof item.isSystem !== 'boolean' || typeof item.isSuper !== 'boolean' ||
    !Array.isArray(item.permissions) || !item.permissions.every((permission) => typeof permission === 'string')) return null
  return {
    id: Number(item.id),
    code: item.code,
    name: item.name,
    description: item.description,
    status: item.status,
    isSystem: item.isSystem,
    isSuper: item.isSuper,
    permissions: item.permissions as string[],
  }
}

export function accountRoleIDs(account: AccessAccount | undefined, roles: readonly AccessRole[]): number[] {
  if (!account) return []
  const assignedCodes = new Set(account.roles.map((role) => role.code))
  return roles.filter((role) => assignedCodes.has(role.code)).map((role) => role.id)
}
