export function apiURL(path: string, baseURL = '') {
  const normalizedPath = path.startsWith('/api') ? path : `/api${path.startsWith('/') ? path : `/${path}`}`
  const base = baseURL.trim().replace(/\/+$/, '')
  return `${base}${normalizedPath}`
}
