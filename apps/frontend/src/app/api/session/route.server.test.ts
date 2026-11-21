// @vitest-environment node
import { NextRequest } from 'next/server'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SESSION_COOKIE, decodeSession, encodeSession } from '@/lib/server/session'
import { DELETE, GET, POST } from './route'

const ORIGIN = 'http://localhost:5173'
const user = { id: 'u1', email: 'owner@example.com', name: 'Owner', status: 'active' }

function req(method: string, init: { body?: unknown; origin?: string; cookie?: string } = {}) {
  const headers: Record<string, string> = { 'content-type': 'application/json', origin: init.origin ?? ORIGIN }
  if (init.cookie) headers.cookie = `${SESSION_COOKIE}=${init.cookie}`
  return new NextRequest(`${ORIGIN}/api/session`, {
    method,
    headers,
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  })
}

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

afterEach(() => vi.unstubAllEnvs())

describe('POST /api/session', () => {
  it('สำเร็จ -> ตั้ง HttpOnly cookie และไม่ส่ง token กลับ', async () => {
    vi.stubEnv('APEXIS_API_URL', 'http://backend:8080/')
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      json(201, { data: { access_token: 'tok', token_type: 'Bearer', expires_at: '2099-01-01T00:00:00Z', user } }),
    )
    const res = await POST(req('POST', { body: { email: user.email, password: 'secret-pass-1' } }))

    expect(fetch).toHaveBeenCalledWith('http://backend:8080/api/v1/auth/tokens', expect.objectContaining({ method: 'POST' }))
    expect(res.status).toBe(201)
    const body = await res.json()
    expect(body).toEqual({ data: { user, expires_at: '2099-01-01T00:00:00Z' } })
    expect(JSON.stringify(body)).not.toContain('tok"')

    const cookie = res.cookies.get(SESSION_COOKIE)!
    expect(cookie.httpOnly).toBe(true)
    expect(cookie.sameSite).toBe('lax')
    expect(decodeSession(cookie.value)?.token).toBe('tok')
  })

  it('backend ตอบ error -> ส่งต่อ status + error เดิม และไม่ตั้ง cookie', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      json(401, { error: { code: 'invalid_email_or_password', message: 'invalid email or password' } }),
    )
    const res = await POST(req('POST', { body: { email: user.email, password: 'x' } }))
    expect(res.status).toBe(401)
    expect((await res.json()).error.code).toBe('invalid_email_or_password')
    expect(res.cookies.get(SESSION_COOKIE)).toBeUndefined()
  })

  it('backend ล่ม -> 502 network_error', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('fetch failed'))
    const res = await POST(req('POST', { body: {} }))
    expect(res.status).toBe(502)
    expect((await res.json()).error.code).toBe('network_error')
  })

  it('ต่าง origin -> 403 โดยไม่เรียก backend', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch')
    const res = await POST(req('POST', { body: {}, origin: 'https://evil.com' }))
    expect(res.status).toBe(403)
    expect(fetch).not.toHaveBeenCalled()
  })
})

describe('GET / DELETE /api/session', () => {
  const cookie = encodeSession({ token: 'tok', expiresAt: '2099-01-01T00:00:00Z', user })

  it('GET: มี session -> ผู้ใช้, ไม่มี -> 401', async () => {
    const ok = await GET(req('GET', { cookie }))
    expect((await ok.json()).data.user).toEqual(user)
    expect((await GET(req('GET'))).status).toBe(401)
  })

  it('DELETE: ลบ cookie', async () => {
    const res = await DELETE(req('DELETE', { cookie }))
    expect(res.status).toBe(204)
    expect(res.cookies.get(SESSION_COOKIE)?.value).toBe('')
  })
})
