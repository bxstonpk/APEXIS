import { NextResponse, type NextRequest } from 'next/server'
import { appConfig } from '@/config'
import {
  SESSION_COOKIE,
  backendUrl,
  crossOriginError,
  decodeSession,
  encodeSession,
  sameOrigin,
  sessionCookieOptions,
} from '@/lib/server/session'

/**
 * PUT /api/session/password {"old_password","new_password"} — เปลี่ยนรหัสผ่านของตัวเอง
 * ส่งต่อไป backend PUT /api/v1/me/password แล้วล้างสถานะ "ต้องเปลี่ยนรหัสผ่าน" ใน session cookie
 * (token เดิมใช้ต่อได้ — backend อ่านสถานะจาก DB ทุก request)
 */
export async function PUT(request: NextRequest) {
  if (!sameOrigin(request)) return NextResponse.json(crossOriginError, { status: 403 })
  const session = decodeSession(request.cookies.get(SESSION_COOKIE)?.value)
  if (!session) return NextResponse.json({ error: { code: 'unauthenticated', message: 'unauthenticated' } }, { status: 401 })

  let body: unknown
  try {
    body = await request.json()
  } catch {
    return NextResponse.json({ error: { code: 'invalid_json_body', message: 'invalid JSON body' } }, { status: 400 })
  }

  let res: Response
  try {
    res = await fetch(backendUrl() + appConfig.api.backend.password, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json', Authorization: `Bearer ${session.token}` },
      body: JSON.stringify(body),
      cache: 'no-store',
    })
  } catch {
    return NextResponse.json({ error: { code: 'network_error', message: 'backend unavailable' } }, { status: 502 })
  }
  if (!res.ok) {
    const payload = (await res.json().catch(() => null)) as { error?: unknown } | null
    return NextResponse.json(payload?.error ? { error: payload.error } : { error: { code: 'unknown', message: res.statusText } }, { status: res.status })
  }

  const out = new NextResponse(null, { status: 204 })
  const next = { ...session, user: { ...session.user, must_change_password: false } }
  out.cookies.set(SESSION_COOKIE, encodeSession(next), sessionCookieOptions(session.expiresAt))
  return out
}
