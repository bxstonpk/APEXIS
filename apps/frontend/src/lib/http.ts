import { appConfig, texts } from '@/config'

/**
 * error จาก API — code ตรงกับ error.code ของ backend (เช่น "invalid_email_or_password")
 * status 0 = เชื่อมต่อเซิร์ฟเวอร์ไม่ได้
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export const NETWORK_ERROR = 'network_error'

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  token?: string
  signal?: AbortSignal
}

/**
 * เรียก API แล้วคืน `data` จาก envelope {"data": ...}
 * error ทุกชนิดถูกแปลงเป็น ApiError (ใช้ errorMessage() เพื่อได้ข้อความจาก config)
 */
export async function apiRequest<T>(path: string, { method = 'GET', body, token, signal }: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (token) headers.Authorization = `Bearer ${token}`

  let res: Response
  try {
    res = await fetch(appConfig.api.baseUrl + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, NETWORK_ERROR, 'network error')
  }

  if (res.status === 204) return undefined as T
  const payload: unknown = await res.json().catch(() => null)
  if (!res.ok) {
    const error = (payload as { error?: { code?: string; message?: string } } | null)?.error
    throw new ApiError(res.status, error?.code ?? 'unknown', error?.message ?? res.statusText)
  }
  return (payload as { data: T }).data
}

/** ข้อความสำหรับแสดงผู้ใช้ของ error ใดๆ — มาจาก texts.errors ตาม error.code */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return texts.errors[err.code] ?? texts.errors.unknown
  return texts.errors.unknown
}
