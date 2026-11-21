import { NextResponse, type NextRequest } from 'next/server'
import { appConfig } from '@/config'
import { SESSION_COOKIE, decodeSession } from '@/lib/server/session'

/**
 * ด่านหน้าของทุกหน้า (Next.js proxy — เดิมชื่อ middleware)
 * - ยังไม่เข้าสู่ระบบ / session หมดอายุ -> ไปหน้า login พร้อม ?redirect=<หน้าเดิม รวม URL query>
 * - เข้าสู่ระบบแล้วแต่เปิดหน้า login -> ไปหน้าที่ระบุใน redirect (หน้า login ตรวจซ้ำอีกชั้น)
 * ไม่ครอบ /api/* (route handler ตอบ 401 เอง) และไฟล์ static
 */
export function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl
  const session = decodeSession(request.cookies.get(SESSION_COOKIE)?.value)
  const isLogin = pathname === appConfig.routes.login

  if (!session && !isLogin) {
    const url = request.nextUrl.clone()
    url.pathname = appConfig.routes.login
    url.search = new URLSearchParams({ [appConfig.redirectParam]: pathname + search }).toString()
    const res = NextResponse.redirect(url)
    if (request.cookies.has(SESSION_COOKIE)) res.cookies.delete(SESSION_COOKIE) // หมดอายุ / เสีย
    return res
  }
  return NextResponse.next()
}

export const config = {
  matcher: ['/((?!api/|_next/|brand/|favicon|apple-touch-icon|robots.txt).*)'],
}
