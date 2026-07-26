const tokenStorageKey = 'warehouse-token'
const tokenExpiryStorageKey = 'warehouse-token-expires-at'

export const defaultTokenLifetimeMs = 24 * 60 * 60 * 1000

type TokenStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

function decodedTokenParts(token: string): string[] | null {
  try {
    const normalized = token.replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
    const parts = globalThis.atob(padded).split(':')
    return parts.length === 5 || parts.length === 3 ? parts : null
  } catch {
    return null
  }
}

export function tokenActorID(token: string): string | null {
  const actorID = decodedTokenParts(token)?.[0] ?? ''
  const numericActorID = Number(actorID)
  return /^[1-9]\d*$/.test(actorID) && Number.isSafeInteger(numericActorID) ? actorID : null
}

export function tokenExpiresAt(token: string): number | null {
  const parts = decodedTokenParts(token)
  const rawExpiry = parts?.length === 5 ? parts[2] : parts?.[1] ?? ''
  const expirySeconds = Number(rawExpiry)
  return Number.isSafeInteger(expirySeconds) && expirySeconds > 0 ? expirySeconds * 1000 : null
}

export function storedTokenExpiresAt(storage: TokenStorage): number | null {
  const value = Number(storage.getItem(tokenExpiryStorageKey))
  return Number.isSafeInteger(value) && value > 0 ? value : null
}

export function clearStoredToken(storage: TokenStorage) {
  storage.removeItem(tokenStorageKey)
  storage.removeItem(tokenExpiryStorageKey)
}

export function loadStoredToken(storage: TokenStorage, now = Date.now()): string {
  const token = storage.getItem(tokenStorageKey) ?? ''
  const expiresAt = storedTokenExpiresAt(storage)
  if (!token || expiresAt === null || expiresAt <= now) {
    clearStoredToken(storage)
    return ''
  }
  return token
}

export function saveStoredToken(token: string, storage: TokenStorage, now = Date.now()): number {
  const expiresAt = tokenExpiresAt(token) ?? now + defaultTokenLifetimeMs
  storage.setItem(tokenStorageKey, token)
  storage.setItem(tokenExpiryStorageKey, String(expiresAt))
  return expiresAt
}
