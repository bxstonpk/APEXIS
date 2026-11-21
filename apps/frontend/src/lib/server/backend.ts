import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { appConfig } from '@/config'
import { ApiError, NETWORK_ERROR } from '@/lib/http'
import { SESSION_COOKIE, backendUrl, decodeSession } from './session'

/** meta ของหน้ารายการ (ตรงกับ backend httpx.Meta) */
export type ListMeta = { page: number; page_size: number; total: number }

/**
 * อ่านข้อมูลจาก backend ใน server component — เรียกตรงพร้อม token จาก cookie (ไม่ต้องผ่าน /api/v1 proxy)
 *
 * - ไม่มี session → ไปหน้า login; backend ตอบ 401 → ลบ cookie แล้วไปหน้า login
 * - error อื่น → throw ApiError (ใช้ errorMessage() แสดงข้อความจาก config)
 * - ใช้กับการอ่าน (GET) เท่านั้น — การเขียนทำจาก client ผ่าน /api/v1 (มีตรวจ origin)
 */
export async function backendGet<T>(path: string, query?: URLSearchParams): Promise<{ data: T; meta?: ListMeta }> {
  const session = decodeSession((await cookies()).get(SESSION_COOKIE)?.value)
  if (!session) redirect(appConfig.routes.login)

  const qs = query?.toString()
  let res: Response
  try {
    res = await fetch(`${backendUrl()}/api/v1${path}${qs ? `?${qs}` : ''}`, {
      headers: { Accept: 'application/json', Authorization: `Bearer ${session.token}` },
      cache: 'no-store',
    })
  } catch {
    throw new ApiError(0, NETWORK_ERROR, 'backend unavailable')
  }

  if (res.status === 401) redirect(appConfig.routes.sessionExpired)
  const payload = (await res.json().catch(() => null)) as
    | { data: T; meta?: ListMeta; error?: { code?: string; message?: string } }
    | null
  if (!res.ok || !payload) {
    throw new ApiError(res.status, payload?.error?.code ?? 'unknown', payload?.error?.message ?? res.statusText)
  }
  return { data: payload.data, meta: payload.meta }
}

/** ผู้ใช้ที่เข้าสู่ระบบอยู่ (จาก cookie) — ไม่มี = ไปหน้า login */
export async function currentUser() {
  const session = decodeSession((await cookies()).get(SESSION_COOKIE)?.value)
  if (!session) redirect(appConfig.routes.login)
  return session.user
}
