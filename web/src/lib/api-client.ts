export type ApiErrorDetails = {
  code?: string
  stage?: string
}

export class ApiError extends Error {
  readonly status: number
  readonly code?: string
  readonly stage?: string

  constructor(message: string, status: number, details: ApiErrorDetails = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = details.code
    this.stage = details.stage
  }
}

// 配对诊断的阶段来源于控制端受限的错误码集合，不展示 Agent 原始响应正文。
const stageLabels: Record<string, string> = {
  input: '输入校验',
  identity: '身份核验',
  exchange: '配对请求',
  response: '响应校验',
  binding: '绑定检查',
  save: '保存配对凭据',
  readback: '读取配对结果',
  unknown: '未知阶段'
}

export function formatApiError(error: unknown): string {
  if (!(error instanceof Error)) return '请求失败，请稍后重试'
  if (!(error instanceof ApiError)) return error.message

  const details: string[] = []
  if (error.code) details.push('错误码：' + error.code)
  if (error.stage) details.push('阶段：' + (stageLabels[error.stage] || error.stage) + '（' + error.stage + '）')
  return error.message + (details.length ? '（' + details.join('；') + '）' : '')
}

function decodeErrorBody(raw: string, status: number): ApiError {
  const fallback = '请求失败（HTTP ' + status + '）'
  if (!raw.trim()) return new ApiError(fallback, status)

  try {
    const decoded: unknown = JSON.parse(raw)
    if (decoded && typeof decoded === 'object' && !Array.isArray(decoded)) {
      const result = decoded as Record<string, unknown>
      const message = typeof result.message === 'string' && result.message.trim()
        ? result.message.trim()
        : fallback
      // 只采纳受控格式的机器错误码，避免把任意对象或超长响应透传给界面。
      const isMachineCode = (value: unknown): value is string =>
        typeof value === 'string' && /^[a-z][a-z0-9_-]{0,63}$/.test(value)
      return new ApiError(message, status, {
        code: isMachineCode(result.code) ? result.code : undefined,
        stage: isMachineCode(result.stage) ? result.stage : undefined
      })
    }
  } catch {
    // 兼容后端现有的 text/plain HTTP 错误。
  }
  return new ApiError(raw.trim(), status)
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
      throw decodeErrorBody(raw, response.status)
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
