import { appConfig } from '@/config'
import { apiRequest } from '@/lib/http'

export type User = {
  id: string
  email: string
  name: string
  status: string
}

/** ผลการเข้าสู่ระบบที่ browser ได้รับ — token อยู่ใน HttpOnly cookie ไม่ออกมาที่นี่ */
export type SignedIn = {
  user: User
  expiresAt: string // ISO 8601
}

/** POST /api/session — เข้าสู่ระบบ (Next ส่งต่อไป backend แล้วตั้ง cookie) */
export async function login(email: string, password: string, signal?: AbortSignal): Promise<SignedIn> {
  const r = await apiRequest<{ user: User; expires_at: string }>(appConfig.api.session, {
    method: 'POST',
    body: { email, password },
    signal,
    baseUrl: '',
  })
  return { user: r.user, expiresAt: r.expires_at }
}

/** DELETE /api/session — ออกจากระบบ (ลบ cookie) */
export async function logout(): Promise<void> {
  await apiRequest<void>(appConfig.api.session, { method: 'DELETE', baseUrl: '' })
}
