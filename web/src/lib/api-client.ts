export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

type RequestOptions = Omit<RequestInit, 'body'> & { body?: unknown }

export async function apiRequest<T>(url: string, options: RequestOptions = {}): Promise<T> {
  const { body, ...requestOptions } = options
  const method = (requestOptions.method || (body === undefined ? 'GET' : 'POST')).toUpperCase()
  const headers = new Headers(requestOptions.headers)

  if (method !== 'GET' && method !== 'HEAD') headers.set('X-ENT-Request', '1')
  if (body !== undefined) headers.set('Content-Type', 'application/json')

  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 20_000)

  try {
    const response = await fetch(url, {
      ...requestOptions,
      method,
      headers,
      credentials: 'same-origin',
      cache: 'no-store',
      signal: requestOptions.signal ?? controller.signal,
      ...(body === undefined ? {} : { body: JSON.stringify(body) })
    })
    const raw = await response.text()
    if (!response.ok) {
      throw new ApiError(raw.trim() || '请求失败（HTTP ' + response.status + '）', response.status)
    }
    if (response.status === 204 || !raw) return undefined as T
    if (!(response.headers.get('content-type') || '').includes('application/json')) return raw as T
    return JSON.parse(raw) as T
  } catch (error) {
    if (error instanceof ApiError) throw error
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw new Error('请求超时，请检查控制端或主机连接')
    }
    throw error
  } finally {
    clearTimeout(timeout)
  }
}
