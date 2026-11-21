// @vitest-environment node
import { NextRequest } from 'next/server'
import { describe, expect, it, vi } from 'vitest'
import { SESSION_COOKIE, decodeSession, encodeSession } from '@/lib/server/session'
import { PUT } from './route'

const ORIGIN = 'http://localhost:5173'
const cookie = encodeSession({
  token: 'tok',
  expiresAt: '2099-01-01T00:00:00Z',
  user: { id: 'u', email: 'a@b.co', name: 'A', status: 'active', must_change_password: true },
})
const req = (body: unknown) =>
  new NextRequest(`${ORIGIN}/api/session/password`, {
    method: 'PUT',
    headers: { origin: ORIGIN, 'content-type': 'application/json', cookie: `${SESSION_COOKIE}=${cookie}` },
    body: JSON.stringify(body),
  })

describe('PUT /api/session/password', () => {
  it('สำเร็จ -> ส่งต่อ backend พร้อม Bearer และล้างสถานะต้องเปลี่ยนรหัสผ่านใน cookie', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }))
    const res = await PUT(req({ old_password: 'temp-pass-1', new_password: 'new-pass-12' }))
    expect(res.status).toBe(204)
    const [url, init] = fetch.mock.calls[0]!
    expect(String(url)).toMatch(/\/api\/v1\/me\/password$/)
    expect((init!.headers as Record<string, string>).Authorization).toBe('Bearer tok')
    expect(decodeSession(res.cookies.get(SESSION_COOKIE)!.value)?.user.must_change_password).toBe(false)
  })

  it('รหัสเดิมผิด -> ส่งต่อ error และไม่แก้ cookie', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: { code: 'invalid_email_or_password', message: 'x' } }), { status: 401, headers: { 'content-type': 'application/json' } }),
    )
    const res = await PUT(req({ old_password: 'wrong', new_password: 'new-pass-12' }))
    expect(res.status).toBe(401)
    expect(res.cookies.get(SESSION_COOKIE)).toBeUndefined()
  })
})
