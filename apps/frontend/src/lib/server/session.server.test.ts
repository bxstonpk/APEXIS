// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { decodeSession, encodeSession, safeRedirect, sameOrigin, type Session } from './session'

const session: Session = {
  token: 'tok',
  expiresAt: '2099-01-01T00:00:00Z',
  user: { id: 'u1', email: 'owner@example.com', name: 'Owner', status: 'active' },
}

describe('session cookie', () => {
  it('encode แล้ว decode ได้ค่าเดิม', () => {
    expect(decodeSession(encodeSession(session))).toEqual(session)
  })

  it('หมดอายุ / ค่าเสีย / ไม่มี token = null', () => {
    expect(decodeSession(encodeSession({ ...session, expiresAt: '2000-01-01T00:00:00Z' }))).toBeNull()
    expect(decodeSession(encodeSession({ ...session, token: '' }))).toBeNull()
    expect(decodeSession('not-base64-json')).toBeNull()
    expect(decodeSession(undefined)).toBeNull()
  })
})

describe('safeRedirect', () => {
  it('รับเฉพาะ path ภายในแอป', () => {
    expect(safeRedirect('/sales?page=2')).toBe('/sales?page=2')
    for (const bad of [null, '', 'https://evil.com', '//evil.com', '/\\evil.com', 'sales']) {
      expect(safeRedirect(bad)).toBe('/')
    }
  })
})

describe('sameOrigin', () => {
  const url = 'http://localhost:5173/api/session'
  it('Origin ตรงกัน = ผ่าน, ต่างกัน = ไม่ผ่าน', () => {
    expect(sameOrigin(new Request(url, { headers: { origin: 'http://localhost:5173' } }))).toBe(true)
    expect(sameOrigin(new Request(url, { headers: { origin: 'https://evil.com' } }))).toBe(false)
  })
  it('ไม่มี Origin ใช้ Sec-Fetch-Site', () => {
    expect(sameOrigin(new Request(url, { headers: { 'sec-fetch-site': 'same-origin' } }))).toBe(true)
    expect(sameOrigin(new Request(url, { headers: { 'sec-fetch-site': 'cross-site' } }))).toBe(false)
  })
})
