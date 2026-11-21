// @vitest-environment node
import { NextRequest } from 'next/server'
import { describe, expect, it } from 'vitest'
import { SESSION_COOKIE, encodeSession } from '@/lib/server/session'
import { proxy } from './proxy'

const valid = encodeSession({
  token: 'tok',
  expiresAt: '2099-01-01T00:00:00Z',
  user: { id: 'u1', email: 'owner@example.com', name: 'Owner', status: 'active' },
})

function req(path: string, cookie?: string) {
  return new NextRequest(`http://localhost:5173${path}`, {
    headers: cookie ? { cookie: `${SESSION_COOKIE}=${cookie}` } : {},
  })
}

describe('proxy', () => {
  it('ยังไม่เข้าสู่ระบบ -> ไป /login พร้อม redirect (เก็บ query เดิม)', () => {
    const res = proxy(req('/sales/orders?status=open&page=2'))
    expect(res.status).toBe(307)
    const loc = new URL(res.headers.get('location')!)
    expect(loc.pathname).toBe('/login')
    expect(loc.searchParams.get('redirect')).toBe('/sales/orders?status=open&page=2')
  })

  it('cookie เสีย -> redirect และลบ cookie', () => {
    const res = proxy(req('/', 'garbage'))
    expect(res.status).toBe(307)
    expect(res.headers.get('set-cookie')).toContain(`${SESSION_COOKIE}=;`)
  })

  it('เปิดหน้าในบริษัท -> จำบริษัทล่าสุดใน cookie', () => {
    const id = '01a1fcd7-77e8-7ef9-a38e-c7a83f5b437d'
    const res = proxy(req(`/c/${id}/organization/branches`, valid))
    expect(res.headers.get('set-cookie')).toContain(`apexis_company=${id}`)
    expect(res.headers.get('set-cookie')).toContain('HttpOnly')
    expect(proxy(req('/c/not-a-uuid', valid)).headers.get('set-cookie')).toBeNull()
  })

  it('บัญชีรหัสผ่านชั่วคราว -> ไปหน้าเปลี่ยนรหัสผ่านก่อน (ยกเว้นหน้าเปลี่ยนรหัสผ่านเอง)', () => {
    const temp = encodeSession({
      token: 'tok',
      expiresAt: '2099-01-01T00:00:00Z',
      user: { id: 'u2', email: 'new@example.com', name: 'New', status: 'active', must_change_password: true },
    })
    const res = proxy(req('/c/x?q=1', temp))
    const loc = new URL(res.headers.get('location')!)
    expect(loc.pathname).toBe('/change-password')
    expect(loc.searchParams.get('redirect')).toBe('/c/x?q=1')
    expect(proxy(req('/change-password', temp)).headers.get('x-middleware-next')).toBe('1')
  })

  it('เข้าสู่ระบบแล้ว / หน้า login -> ผ่าน', () => {
    expect(proxy(req('/', valid)).headers.get('x-middleware-next')).toBe('1')
    expect(proxy(req('/login')).headers.get('x-middleware-next')).toBe('1')
  })
})
