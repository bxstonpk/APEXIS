import { NextResponse, type NextRequest } from 'next/server'
import { appConfig } from '@/config'
import { SESSION_COOKIE, sessionCookieOptions } from '@/lib/server/session'

/**
 * GET /api/session/expired — backend ปฏิเสธ token (401) ระหว่าง render ฝั่ง server
 * server component ตั้ง cookie เองไม่ได้ จึง redirect มาที่นี่เพื่อลบ cookie แล้วไปหน้า login
 * (ไม่ลบ = หน้า login เห็น cookie ที่ยังไม่หมดอายุแล้วพากลับไปหน้าเดิม วนไม่จบ)
 */
export function GET(request: NextRequest) {
  const url = request.nextUrl.clone()
  url.pathname = appConfig.routes.login
  url.search = ''
  const res = NextResponse.redirect(url)
  res.cookies.set(SESSION_COOKIE, '', sessionCookieOptions())
  return res
}
