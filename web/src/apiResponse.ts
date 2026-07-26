type JsonRecord = Record<string, unknown>

const applicationStatusCodes: Record<number, number> = {
  100400: 400,
  100401: 401,
  100403: 403,
  100404: 404,
  100405: 405,
  100408: 408,
  100409: 409,
  100415: 415,
  100422: 422,
  100429: 429,
  100500: 500,
  100502: 502,
  100504: 504,
}

export function effectiveApiStatus(httpStatus: number, payload: unknown) {
  if (httpStatus !== 200) return httpStatus
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return httpStatus
  const code = (payload as JsonRecord).code
  if (typeof code !== 'number') return httpStatus
  return applicationStatusCodes[code] ?? httpStatus
}
