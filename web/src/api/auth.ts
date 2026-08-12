type JsonRecord = Record<string, unknown>

export type SessionUser = {
  id: number
  account: string
  nickname: string
  phone: string
  accountType: 'CONSOLE'
  status: 'ACTIVE'
  mallScopeMode: 'ALL' | 'SELECTED'
  roles: Array<{ code: string; name: string }>
  permissions: string[]
  mallIds: number[]
}

export type TokenInfo = {
  userID: number
  tokenType: string
  expireTime: number
  issuedTime: number
  ttl: number
}

export type SessionVerificationResponse = {
  ok: boolean
  data: unknown
  error?: { kind: string }
}

export type SessionVerification =
  | { kind: 'valid'; user: SessionUser; tokenInfo: TokenInfo }
  | { kind: 'unauthorized' | 'invalid' | 'transient' }

function readPositiveSafeInteger(value: unknown): number | null {
  if (typeof value === 'number') {
    return Number.isSafeInteger(value) && value > 0 ? value : null
  }
  if (typeof value !== 'string' || !/^[1-9]\d*$/.test(value)) return null
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) ? parsed : null
}

function dataRecord(payload: unknown): JsonRecord | null {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return null
  const data = (payload as JsonRecord).data
  return data && typeof data === 'object' && !Array.isArray(data) ? data as JsonRecord : null
}

export function isSuccessfulPayload(payload: unknown) {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return false
  const code = (payload as JsonRecord).code
  return code === 0 || code === 200
}

export function readEnvelopeToken(payload: unknown) {
  const token = dataRecord(payload)?.token
  return typeof token === 'string' && token.trim() ? token : ''
}

export function readSessionUser(payload: unknown): SessionUser | null {
  const data = dataRecord(payload)
  const roles = data?.roles
  const permissions = data?.permissions
  const mallIds = data?.mallIds
  if (!data || typeof data.id !== 'number' || !Number.isSafeInteger(data.id) || data.id <= 0 || typeof data.account !== 'string' || typeof data.nickname !== 'string' || typeof data.phone !== 'string' || data.accountType !== 'CONSOLE' || data.status !== 'ACTIVE' || (data.mallScopeMode !== 'ALL' && data.mallScopeMode !== 'SELECTED') || !Array.isArray(roles) || !roles.every((role) => role && typeof role === 'object' && typeof (role as JsonRecord).code === 'string' && typeof (role as JsonRecord).name === 'string') || !Array.isArray(permissions) || !permissions.every((permission) => typeof permission === 'string') || !Array.isArray(mallIds) || !mallIds.every((id) => typeof id === 'number' && Number.isSafeInteger(id) && id > 0)) return null
  return {
    id: data.id,
    account: data.account,
    nickname: data.nickname,
    phone: data.phone,
    accountType: data.accountType,
    status: data.status,
    mallScopeMode: data.mallScopeMode,
    roles: roles.map((role) => ({ code: (role as JsonRecord).code as string, name: (role as JsonRecord).name as string })),
    permissions: [...permissions] as string[],
    mallIds: [...mallIds] as number[],
  }
}

export function readTokenInfo(payload: unknown): TokenInfo | null {
  const data = dataRecord(payload)
  const userID = data ? readPositiveSafeInteger(data.user_id) : null
  if (!data || userID === null || typeof data.token_type !== 'string' || typeof data.expire_time !== 'number' || !Number.isSafeInteger(data.expire_time) || data.expire_time <= 0 || typeof data.issued_time !== 'number' || !Number.isSafeInteger(data.issued_time) || data.issued_time <= 0 || typeof data.ttl !== 'number' || !Number.isFinite(data.ttl)) return null
  return {
    userID,
    tokenType: data.token_type,
    expireTime: data.expire_time,
    issuedTime: data.issued_time,
    ttl: data.ttl,
  }
}

export function verifySessionResponses(
  profileResponse: SessionVerificationResponse,
  tokenInfoResponse: SessionVerificationResponse,
): SessionVerification {
  if (profileResponse.error?.kind === 'unauthorized' || tokenInfoResponse.error?.kind === 'unauthorized') {
    return { kind: 'unauthorized' }
  }
  if (!profileResponse.ok || !tokenInfoResponse.ok) return { kind: 'transient' }

  const user = readSessionUser(profileResponse.data)
  const tokenInfo = readTokenInfo(tokenInfoResponse.data)
  if (!user || !tokenInfo || tokenInfo.userID !== user.id) return { kind: 'invalid' }
  return { kind: 'valid', user, tokenInfo }
}
