import { NextResponse, type NextRequest } from 'next/server'
import { appConfig } from '@/config'
import { SESSION_COOKIE, decodeSession } from '@/lib/server/session'

/**
 * ด่านหน้าของทุกหน้า (Next.js proxy — เดิมชื่อ middleware)
 * - ยังไม่เข้าสู่ระบบ / session หมดอายุ -> ไปหน้า login พร้อม ?redirect=<หน้าเดิม รวม URL query>
 * - เข้าสู่ระบบแล้วแต่เปิดหน้า login -> หน้า login พาไปหน้าที่ระบุใน redirect เอง
 * - เปิดหน้าในบริษัท (/c/{companyId}/...) -> จำ companyId ไว้ใน cookie
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

  // รหัสผ่านชั่วคราว: ต้องตั้งรหัสผ่านใหม่ก่อนเข้าหน้าอื่น (backend ก็ปฏิเสธข้อมูลบริษัทจนกว่าจะเปลี่ยน)
  const changePassword = appConfig.routes.changePassword
  if (session?.user.must_change_password && pathname !== changePassword && !isLogin) {
    const url = request.nextUrl.clone()
    url.pathname = changePassword
    url.search = new URLSearchParams({ [appConfig.redirectParam]: pathname + search }).toString()
    return NextResponse.redirect(url)
  }

  const res = NextResponse.next()
  // จำบริษัทที่เปิดล่าสุด (/c/{companyId}/...) — หน้า / ใช้พาเข้าบริษัทนี้ทันทีในครั้งถัดไป
  const companyId = COMPANY_PATH.exec(pathname)?.[1]
  if (session && companyId && request.cookies.get(COMPANY_COOKIE)?.value !== companyId) {
    res.cookies.set(COMPANY_COOKIE, companyId, {
      httpOnly: true,
      sameSite: 'lax',
      secure: process.env.NODE_ENV === 'production',
      path: '/',
      maxAge: 60 * 60 * 24 * 365,
    })
  }
  return res
}

const COMPANY_COOKIE = appConfig.cookies.company
const COMPANY_PATH = /^\/c\/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:\/|$)/i

export const config = {
  matcher: ['/((?!api/|_next/|brand/|favicon|apple-touch-icon|robots.txt).*)'],
}
