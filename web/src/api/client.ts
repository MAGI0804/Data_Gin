import { effectiveApiStatus } from '../apiResponse'
import { apiURL } from '../apiURL'
import { isSuccessfulPayload, readEnvelopeToken } from './auth'

export type HTTPMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export type ClientFailureKind = 'aborted' | 'offline' | 'timeout' | 'rate_limited' | 'unauthorized' | 'forbidden' | 'client' | 'server' | 'network'

export type ClientResponse = {
  ok: boolean
  status: number
  data: unknown
  error?: {
    kind: ClientFailureKind
    message: string
    retryAfterSeconds?: number
  }
}

export type ApiRequestOptions = {
  method: HTTPMethod
  body?: unknown
  headers?: Record<string, string>
  signal?: AbortSignal
  retry?: boolean
  timeoutMs?: number
}

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export type ApiClientOptions = {
  baseURL?: string
  getToken: () => string
  onTokenRefreshed: (token: string) => void
  onUnauthorized: () => void
  fetch?: FetchLike
  timeoutMs?: number
  maxGetRetries?: number
  wait?: (milliseconds: number) => Promise<void>
}

type RequestSignal = {
  signal: AbortSignal
  cleanup: () => void
  wasTimedOut: () => boolean
}

function createRequestSignal(external: AbortSignal | undefined, timeoutMs: number): RequestSignal {
  const controller = new AbortController()
  let timedOut = false
  const abortExternal = () => controller.abort(external?.reason)
  external?.addEventListener('abort', abortExternal, { once: true })
  const timer = globalThis.setTimeout(() => {
    timedOut = true
    controller.abort(new DOMException('请求超时', 'TimeoutError'))
  }, timeoutMs)
  return {
    signal: controller.signal,
    cleanup: () => {
      globalThis.clearTimeout(timer)
      external?.removeEventListener('abort', abortExternal)
    },
    wasTimedOut: () => timedOut,
  }
}

function publicFailure(status: number, retryAfterSeconds?: number): NonNullable<ClientResponse['error']> {
  if (status === 401) return { kind: 'unauthorized', message: '登录状态已失效，请重新登录。' }
  if (status === 403) return { kind: 'forbidden', message: '当前账号没有执行此操作的权限。' }
  if (status === 429) return { kind: 'rate_limited', message: retryAfterSeconds ? `请求过于频繁，请在 ${retryAfterSeconds} 秒后重试。` : '请求过于频繁，请稍后重试。', retryAfterSeconds }
  if (status >= 500) return { kind: 'server', message: '服务暂时不可用，请稍后重试。' }
  return { kind: 'client', message: '请求未能完成，请检查输入后重试。' }
}

function retryAfterSeconds(headers: Headers) {
  const value = headers.get('Retry-After')
  if (!value) return undefined
  const seconds = Number(value)
  return Number.isFinite(seconds) && seconds >= 0 ? Math.ceil(seconds) : undefined
}

function getRetryDelay(attempt: number) {
  return Math.min(2_000, 250 * 2 ** attempt)
}

function defaultWait(milliseconds: number) {
  return new Promise<void>((resolve) => globalThis.setTimeout(resolve, milliseconds))
}

function isFormData(body: unknown): body is FormData {
  return typeof FormData !== 'undefined' && body instanceof FormData
}

function withoutContentType(headers: Record<string, string> | undefined) {
  if (!headers) return undefined
  return Object.fromEntries(Object.entries(headers).filter(([name]) => name.toLowerCase() !== 'content-type'))
}

function authorizedHeaders(token: string, extra: Record<string, string> | undefined, hasJSONBody: boolean, omitContentType = false) {
  return {
    ...(hasJSONBody ? { 'Content-Type': 'application/json' } : {}),
    ...(omitContentType ? withoutContentType(extra) : extra),
    ...(token ? { token, Authorization: `Bearer ${token}` } : {}),
  }
}

export function createApiClient(options: ApiClientOptions) {
  const fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis)
  const timeoutMs = options.timeoutMs ?? 15_000
  const maxGetRetries = options.maxGetRetries ?? 2
  const wait = options.wait ?? defaultWait
  let refreshPromise: Promise<boolean> | null = null

  async function refreshToken() {
    if (refreshPromise) return refreshPromise
    const currentToken = options.getToken()
    if (!currentToken) return false
    refreshPromise = (async () => {
      const request = createRequestSignal(undefined, timeoutMs)
      try {
        const response = await fetchImpl(apiURL('/auth/token/refresh', options.baseURL), {
          method: 'POST',
          signal: request.signal,
          headers: authorizedHeaders(currentToken, undefined, true),
          body: JSON.stringify({ token_type: 'refreshable' }),
        })
        const payload: unknown = await response.json().catch(() => null)
        const status = effectiveApiStatus(response.status, payload)
        const token = response.ok && status < 400 && isSuccessfulPayload(payload) ? readEnvelopeToken(payload) : ''
        if (!token) return false
        if (options.getToken() !== currentToken) return false
        options.onTokenRefreshed(token)
        return true
      } catch {
        return false
      } finally {
        request.cleanup()
      }
    })().finally(() => { refreshPromise = null })
    return refreshPromise
  }

  async function request(path: string, requestOptions: ApiRequestOptions): Promise<ClientResponse> {
    const { method, body, headers, signal, retry = true } = requestOptions
    const retryable = method === 'GET' && retry
    let attempt = 0
    let replayedAfterRefresh = false

    while (true) {
      const requestSignal = createRequestSignal(signal, requestOptions.timeoutMs ?? timeoutMs)
      try {
        const canSendBody = method !== 'GET' && body !== undefined
        const formDataBody = canSendBody && isFormData(body)
        const response = await fetchImpl(apiURL(path, options.baseURL), {
          method,
          signal: requestSignal.signal,
          headers: authorizedHeaders(options.getToken(), headers, canSendBody && !formDataBody, formDataBody || method === 'GET'),
          body: !canSendBody ? undefined : formDataBody ? body : JSON.stringify(body),
        })
        const payload: unknown = await response.json().catch(() => null)
        const status = effectiveApiStatus(response.status, payload)
        if (response.ok && status < 400 && isSuccessfulPayload(payload)) return { ok: true, status, data: payload }

        if (status === 401) {
          if (retryable && !replayedAfterRefresh) {
            replayedAfterRefresh = true
            if (await refreshToken()) continue
          }
          options.onUnauthorized()
        }

        const retryAfter = retryAfterSeconds(response.headers)
        if (retryable && !replayedAfterRefresh && attempt < maxGetRetries && (status === 0 || status >= 500)) {
          await wait(getRetryDelay(attempt++))
          continue
        }
        const error = publicFailure(status, retryAfter)
        return { ok: false, status, data: { message: error.message }, error }
      } catch {
        if (signal?.aborted) return { ok: false, status: 0, data: { message: '请求已取消。' }, error: { kind: 'aborted', message: '请求已取消。' } }
        if (requestSignal.wasTimedOut()) return { ok: false, status: 0, data: { message: '请求超时，请稍后重试。' }, error: { kind: 'timeout', message: '请求超时，请稍后重试。' } }
        const offline = typeof navigator !== 'undefined' && navigator.onLine === false
        if (retryable && !replayedAfterRefresh && attempt < maxGetRetries) {
          await wait(getRetryDelay(attempt++))
          continue
        }
        const message = offline ? '当前处于离线状态，已保留最近一次数据。' : '网络连接异常，请检查网络后重试。'
        return { ok: false, status: 0, data: { message }, error: { kind: offline ? 'offline' : 'network', message } }
      } finally {
        requestSignal.cleanup()
      }
    }
  }

  return { request, refresh: refreshToken }
}
