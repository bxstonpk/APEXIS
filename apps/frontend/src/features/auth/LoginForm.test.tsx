import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { LoginForm } from './LoginForm'

const t = texts.auth.login

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

// /api/session ตอบเฉพาะข้อมูลผู้ใช้ — token อยู่ใน HttpOnly cookie ไม่ถึง browser
const signedIn = {
  expires_at: '2099-01-01T00:00:00Z',
  user: { id: 'u1', email: 'owner@example.com', name: 'Owner', status: 'active' },
}

async function fill(email: string, password: string) {
  const user = userEvent.setup()
  if (email) await user.type(screen.getByLabelText(t.emailLabel), email)
  if (password) await user.type(screen.getByLabelText(t.passwordLabel), password)
  await user.click(screen.getByRole('button', { name: t.submit }))
  return user
}

describe('LoginForm', () => {
  it('ข้อความทั้งหมดมาจาก config', () => {
    render(<LoginForm onSuccess={vi.fn()} />)
    expect(screen.getByLabelText(t.emailLabel)).toHaveAttribute('placeholder', t.emailPlaceholder)
    expect(screen.getByLabelText(t.passwordLabel)).toHaveAttribute('type', 'password')
    expect(screen.getByRole('button', { name: t.showPassword })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: t.submit })).toBeInTheDocument()
  })

  it('ส่งข้อความแทน config ได้เมื่อ embed ในที่อื่น', () => {
    render(<LoginForm onSuccess={vi.fn()} texts={{ ...t, submit: 'ยืนยันตัวตน' }} />)
    expect(screen.getByRole('button', { name: 'ยืนยันตัวตน' })).toBeInTheDocument()
  })

  it('ตรวจช่องว่าง / รูปแบบอีเมลก่อนส่ง และไม่เรียก API', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch')
    render(<LoginForm onSuccess={vi.fn()} />)
    await fill('', '')
    expect(screen.getByText(texts.auth.validation.emailRequired)).toBeInTheDocument()
    expect(screen.getByText(texts.auth.validation.passwordRequired)).toBeInTheDocument()
    expect(screen.getByLabelText(t.emailLabel)).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText(t.emailLabel)).toHaveFocus()

    await fill('not-an-email', 'x')
    expect(screen.getByText(texts.auth.validation.emailInvalid)).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalled()
  })

  it('เข้าสู่ระบบสำเร็จ -> เรียก /api/session และ onSuccess ได้ผู้ใช้', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse(201, { data: signedIn }))
    const onSuccess = vi.fn()
    render(<LoginForm onSuccess={onSuccess} />)
    await fill('  owner@example.com ', 'secret-pass-1')

    expect(fetch).toHaveBeenCalledWith('/api/session', expect.objectContaining({ method: 'POST', credentials: 'same-origin' }))
    const body = JSON.parse(fetch.mock.calls[0]![1]!.body as string)
    expect(body).toEqual({ email: 'owner@example.com', password: 'secret-pass-1' })
    expect(onSuccess).toHaveBeenCalledWith({ expiresAt: signedIn.expires_at, user: signedIn.user })
  })

  it('รหัสผ่านผิด -> แสดงข้อความจาก config ตาม error.code และล้างรหัสผ่าน', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      jsonResponse(401, { error: { code: 'invalid_email_or_password', message: 'invalid email or password' } }),
    )
    const onSuccess = vi.fn()
    render(<LoginForm onSuccess={onSuccess} />)
    await fill('owner@example.com', 'wrong')

    expect(await screen.findByRole('alert')).toHaveTextContent(texts.errors.invalid_email_or_password!)
    expect(screen.getByLabelText(t.passwordLabel)).toHaveValue('')
    expect(onSuccess).not.toHaveBeenCalled()
  })

  it('เชื่อมต่อเซิร์ฟเวอร์ไม่ได้ -> ข้อความ network_error', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'))
    render(<LoginForm onSuccess={vi.fn()} />)
    await fill('owner@example.com', 'secret-pass-1')
    expect(await screen.findByRole('alert')).toHaveTextContent(texts.errors.network_error)
  })

  it('error.code ที่ไม่รู้จัก -> ข้อความ unknown', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse(418, { error: { code: 'teapot', message: '?' } }))
    render(<LoginForm onSuccess={vi.fn()} />)
    await fill('owner@example.com', 'secret-pass-1')
    expect(await screen.findByRole('alert')).toHaveTextContent(texts.errors.unknown)
  })

  it('ปุ่มแสดง / ซ่อนรหัสผ่าน', async () => {
    const user = userEvent.setup()
    render(<LoginForm onSuccess={vi.fn()} />)
    await user.click(screen.getByRole('button', { name: t.showPassword }))
    expect(screen.getByLabelText(t.passwordLabel)).toHaveAttribute('type', 'text')
    expect(screen.getByRole('button', { name: t.hidePassword })).toBeInTheDocument()
  })
})
