export type BrowserStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export function createSafeBrowserStorage(resolve: () => BrowserStorage): BrowserStorage {
  const fallback = new Map<string, string>()
  let storage: BrowserStorage | null = null
  try {
    storage = resolve()
  } catch {
    storage = null
  }
  return {
    getItem(key) {
      try {
        const value = storage?.getItem(key)
        if (value !== null && value !== undefined) fallback.set(key, value)
        return value ?? fallback.get(key) ?? null
      } catch {
        return fallback.get(key) ?? null
      }
    },
    setItem(key, value) {
      const normalized = String(value)
      fallback.set(key, normalized)
      try {
        storage?.setItem(key, normalized)
      } catch {
        // The in-memory copy keeps the current page usable when browser storage is blocked.
      }
    },
    removeItem(key) {
      fallback.delete(key)
      try {
        storage?.removeItem(key)
      } catch {
        // Removal from the in-memory copy is sufficient for the current page lifetime.
      }
    },
  }
}

export const browserLocalStorage = createSafeBrowserStorage(() => window.localStorage)
export const browserSessionStorage = createSafeBrowserStorage(() => window.sessionStorage)
