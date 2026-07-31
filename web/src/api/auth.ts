type JsonRecord = Record<string, unknown>

export type SessionUser = {
  id: number
  account: string
  nickname: string
  email: string
  consoleManaged: boolean
}

export type TokenInfo = {
  userID: number
  tokenType: string
  expireTime: number
  issuedTime: number
  ttl: number
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
  if (!data || typeof data.id !== 'number' || !Number.isSafeInteger(data.id) || data.id <= 0 || typeof data.account !== 'string' || typeof data.nickname !== 'string' || typeof data.email !== 'string' || typeof data.consoleManaged !== 'boolean') return null
  return {
    id: data.id,
    account: data.account,
    nickname: data.nickname,
    email: data.email,
    consoleManaged: data.consoleManaged,
  }
}

export function readTokenInfo(payload: unknown): TokenInfo | null {
  const data = dataRecord(payload)
  if (!data || typeof data.user_id !== 'number' || !Number.isSafeInteger(data.user_id) || data.user_id <= 0 || typeof data.token_type !== 'string' || typeof data.expire_time !== 'number' || !Number.isSafeInteger(data.expire_time) || data.expire_time <= 0 || typeof data.issued_time !== 'number' || !Number.isSafeInteger(data.issued_time) || data.issued_time <= 0 || typeof data.ttl !== 'number' || !Number.isFinite(data.ttl)) return null
  return {
    userID: data.user_id,
    tokenType: data.token_type,
    expireTime: data.expire_time,
    issuedTime: data.issued_time,
    ttl: data.ttl,
  }
}
