import { NextResponse, type NextRequest } from 'next/server'
import { SESSION_COOKIE, backendUrl, crossOriginError, decodeSession, sameOrigin, sessionCookieOptions } from '@/lib/server/session'

/**
 * /api/v1/* — ส่งต่อทุก request ไป backend (Go) โดยเติม Authorization: Bearer <token> จาก HttpOnly cookie
 *
 * - browser ไม่เคยเห็น token; cookie ของ Next ไม่ถูกส่งต่อไป backend
 * - คำขอที่เปลี่ยนข้อมูล (POST / PUT / PATCH / DELETE) ต้องมาจาก origin เดียวกัน (กัน CSRF ร่วมกับ SameSite=Lax)
 * - backend ตอบ 401 = token ใช้ไม่ได้แล้ว -> ลบ cookie
 */
const SAFE = new Set(['GET', 'HEAD'])
const FORWARD_REQUEST_HEADERS = ['accept', 'content-type', 'accept-language', 'if-none-match']
// รูปของแม่แบบเอกสาร (document-assets/.../content): ส่ง CSP / nosniff / ETag ของ backend ต่อให้ browser ด้วย
const FORWARD_RESPONSE_HEADERS = ['content-type', 'cache-control', 'content-security-policy', 'x-content-type-options', 'etag']

async function forward(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  if (!SAFE.has(request.method) && !sameOrigin(request)) {
    return NextResponse.json(crossOriginError, { status: 403 })
  }
  const { path } = await params
  const target = `${backendUrl()}/api/v1/${path.map(encodeURIComponent).join('/')}${request.nextUrl.search}`

  const headers = new Headers()
  for (const name of FORWARD_REQUEST_HEADERS) {
    const v = request.headers.get(name)
    if (v) headers.set(name, v)
  }
  const session = decodeSession(request.cookies.get(SESSION_COOKIE)?.value)
  if (session) headers.set('authorization', `Bearer ${session.token}`)

  let res: Response
  try {
    res = await fetch(target, {
      method: request.method,
      headers,
      body: SAFE.has(request.method) ? undefined : await request.arrayBuffer(),
      cache: 'no-store',
      redirect: 'manual',
    })
  } catch {
    return NextResponse.json({ error: { code: 'network_error', message: 'backend unavailable' } }, { status: 502 })
  }

  const outHeaders = new Headers()
  for (const name of FORWARD_RESPONSE_HEADERS) {
    const v = res.headers.get(name)
    if (v) outHeaders.set(name, v)
  }
  const out = new NextResponse(res.status === 204 ? null : res.body, { status: res.status, headers: outHeaders })
  if (res.status === 401 && request.cookies.has(SESSION_COOKIE)) {
    out.cookies.set(SESSION_COOKIE, '', sessionCookieOptions())
  }
  return out
}

export { forward as GET, forward as POST, forward as PUT, forward as PATCH, forward as DELETE }
