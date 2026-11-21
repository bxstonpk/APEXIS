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
  type Session,
} from '@/lib/server/session'

type TokenResponse = {
  access_token: string
  token_type: string
  expires_at: string
  user: Session['user']
}

function errorBody(code: string, message: string) {
  return { error: { code, message } }
}

/**
 * POST /api/session {"email","password"} — เข้าสู่ระบบ: ส่งต่อไป backend แล้วเก็บ token ใน HttpOnly cookie
 * ตอบเฉพาะข้อมูลผู้ใช้ (token ไม่ออกไปที่ browser); error ของ backend ส่งต่อตามเดิม (status + {"error":{code,message}})
 */
export async function POST(request: NextRequest) {
  if (!sameOrigin(request)) return NextResponse.json(crossOriginError, { status: 403 })
  let body: unknown
  try {
    body = await request.json()
  } catch {
    return NextResponse.json(errorBody('invalid_json_body', 'invalid JSON body'), { status: 400 })
  }

  let res: Response
  try {
    res = await fetch(backendUrl() + appConfig.api.backend.login, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(body),
      cache: 'no-store',
    })
  } catch {
    return NextResponse.json(errorBody('network_error', 'backend unavailable'), { status: 502 })
  }

  const payload = (await res.json().catch(() => null)) as { data?: TokenResponse; error?: unknown } | null
  if (!res.ok || !payload?.data?.access_token) {
    return NextResponse.json(payload?.error ? { error: payload.error } : errorBody('unknown', res.statusText), {
      status: res.ok ? 502 : res.status,
    })
  }

  const t = payload.data
  const session: Session = { token: t.access_token, expiresAt: t.expires_at, user: t.user }
  const out = NextResponse.json({ data: { user: t.user, expires_at: t.expires_at } }, { status: 201 })
  out.cookies.set(SESSION_COOKIE, encodeSession(session), sessionCookieOptions(t.expires_at))
  return out
}

/** GET /api/session — ผู้ใช้ปัจจุบัน (ไม่มี / หมดอายุ = 401) */
export async function GET(request: NextRequest) {
  const session = decodeSession(request.cookies.get(SESSION_COOKIE)?.value)
  if (!session) {
    return NextResponse.json(errorBody('unauthenticated', 'unauthenticated'), { status: 401 })
  }
  return NextResponse.json({ data: { user: session.user, expires_at: session.expiresAt } })
}

/** DELETE /api/session — ออกจากระบบ (ลบ cookie) */
export async function DELETE(request: NextRequest) {
  if (!sameOrigin(request)) return NextResponse.json(crossOriginError, { status: 403 })
  const out = new NextResponse(null, { status: 204 })
  out.cookies.set(SESSION_COOKIE, '', sessionCookieOptions())
  return out
}
