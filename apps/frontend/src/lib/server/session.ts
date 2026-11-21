import { appConfig } from '@/config'

/**
 * session ของผู้ใช้ — เก็บใน cookie แบบ HttpOnly (JavaScript ฝั่ง browser อ่าน / ขโมยไม่ได้)
 * ใช้เฉพาะฝั่ง server ของ Next: proxy.ts, route handler, server component
 */
export type User = {
  id: string
  email: string
  name: string
  status: string
  /** บัญชีที่ผู้ดูแลสร้างให้ (รหัสผ่านชั่วคราว) — ต้องเปลี่ยนรหัสผ่านก่อนใช้งาน */
  must_change_password?: boolean
}

export type Session = {
  token: string
  expiresAt: string // ISO 8601 (ตาม token ของ backend)
  user: User
}

export const SESSION_COOKIE = appConfig.cookies.session

/** ที่อยู่ backend (Go) — อ่านตอนเรียกใช้ เพื่อให้เปลี่ยนผ่าน env ได้โดยไม่ต้อง build ใหม่ */
export function backendUrl(): string {
  return (process.env.APEXIS_API_URL ?? 'http://127.0.0.1:8080').replace(/\/+$/, '')
}

export function isExpired(session: Pick<Session, 'expiresAt'>, now = Date.now()): boolean {
  const exp = Date.parse(session.expiresAt)
  return Number.isNaN(exp) || exp <= now
}

/** แปลง session เป็นค่า cookie (base64url ของ JSON) */
export function encodeSession(session: Session): string {
  return Buffer.from(JSON.stringify(session), 'utf8').toString('base64url')
}

/** อ่านค่า cookie — รูปแบบผิด / หมดอายุ = null */
export function decodeSession(value: string | undefined, now = Date.now()): Session | null {
  if (!value) return null
  try {
    const s = JSON.parse(Buffer.from(value, 'base64url').toString('utf8')) as Session
    if (typeof s?.token !== 'string' || !s.token || typeof s.user?.id !== 'string' || isExpired(s, now)) return null
    return s
  } catch {
    return null
  }
}

/** ตัวเลือกของ cookie: HttpOnly, SameSite=Lax, Secure เมื่อ production, หมดอายุพร้อม token */
export function sessionCookieOptions(expiresAt?: string) {
  return {
    httpOnly: true,
    sameSite: 'lax' as const,
    secure: process.env.NODE_ENV === 'production',
    path: '/',
    ...(expiresAt ? { expires: new Date(expiresAt) } : { maxAge: 0 }),
  }
}

/** path ภายในแอปเท่านั้น (กัน open redirect เช่น //evil.com, https://..., /\\evil.com) */
export function safeRedirect(target: string | null | undefined): string {
  if (!target || !target.startsWith('/') || target.startsWith('//') || target.startsWith('/\\')) {
    return appConfig.routes.home
  }
  return target
}

/**
 * คำขอมาจาก origin เดียวกับแอป — ใช้กับคำขอที่เปลี่ยนข้อมูล / ตั้ง cookie (กัน CSRF ร่วมกับ SameSite=Lax)
 * ไม่มี Origin header (บาง browser กับ same-origin) ใช้ Sec-Fetch-Site แทน
 */
export function sameOrigin(request: Request): boolean {
  const origin = request.headers.get('origin')
  if (!origin) return request.headers.get('sec-fetch-site') !== 'cross-site'
  return origin === new URL(request.url).origin
}

export const crossOriginError = { error: { code: 'permission_denied', message: 'cross-origin request' } }
