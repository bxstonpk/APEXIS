import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { appConfig, brand, format, texts } from '@/config'
import { safeRedirect } from '@/features/auth'
import { App } from './App'

const user = { id: 'u1', email: 'owner@example.com', name: 'Owner', status: 'active' }

describe('App', () => {
  it('ยังไม่เข้าสู่ระบบ -> ไปหน้า login พร้อม redirect แล้วกลับหน้าเดิม (รวม query) หลังเข้าสู่ระบบ', async () => {
    window.history.pushState({}, '', '/?tab=reports')
    render(<App />)
    expect(await screen.findByRole('heading', { name: texts.auth.login.heading })).toBeInTheDocument()
    expect(window.location.pathname).toBe(appConfig.routes.login)
    expect(new URLSearchParams(window.location.search).get(appConfig.redirectParam)).toBe('/?tab=reports')
    expect(document.title).toBe(`${texts.auth.login.pageTitle} | ${texts.app.name}`)
    expect(screen.getByRole('img', { name: texts.app.logoAlt })).toHaveAttribute('src', brand.logo.src)

    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ data: { access_token: 't', token_type: 'Bearer', expires_at: '2099-01-01T00:00:00Z', user } }), { status: 201 }),
    )
    const u = userEvent.setup()
    await u.type(screen.getByLabelText(texts.auth.login.emailLabel), user.email)
    await u.type(screen.getByLabelText(texts.auth.login.passwordLabel), 'secret-pass-1')
    await u.click(screen.getByRole('button', { name: texts.auth.login.submit }))

    expect(await screen.findByRole('heading', { name: format(texts.home.welcome, { name: 'Owner' }) })).toBeInTheDocument()
    expect(window.location.pathname + window.location.search).toBe('/?tab=reports')
    expect(localStorage.getItem(appConfig.storage.session)).toContain('"accessToken":"t"')

    await u.click(screen.getByRole('button', { name: texts.auth.logout }))
    expect(await screen.findByRole('heading', { name: texts.auth.login.heading })).toBeInTheDocument()
    expect(localStorage.getItem(appConfig.storage.session)).toBeNull()
  })

  it('session หมดอายุใน storage ไม่ถูกใช้', async () => {
    localStorage.setItem(appConfig.storage.session, JSON.stringify({ accessToken: 'old', expiresAt: '2000-01-01T00:00:00Z', user }))
    window.history.pushState({}, '', '/')
    render(<App />)
    expect(await screen.findByRole('heading', { name: texts.auth.login.heading })).toBeInTheDocument()
  })
})

describe('safeRedirect', () => {
  it('รับเฉพาะ path ภายในแอป', () => {
    expect(safeRedirect('/ledger/reports?book=balance-sheet')).toBe('/ledger/reports?book=balance-sheet')
    for (const bad of [null, '', 'https://evil.com', '//evil.com', '/\\evil.com', 'javascript:alert(1)']) {
      expect(safeRedirect(bad)).toBe(appConfig.routes.home)
    }
  })
})

describe('format', () => {
  it('แทนค่าตัวแปรในข้อความจาก config', () => {
    expect(format('ยินดีต้อนรับ, {name}', { name: 'สมชาย' })).toBe('ยินดีต้อนรับ, สมชาย')
    expect(format('{a} {missing}', { a: 1 })).toBe('1 {missing}')
  })
})
