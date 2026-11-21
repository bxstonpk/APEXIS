import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { ChangePasswordForm } from '@/features/auth/ChangePasswordForm'
import { buildMatrix, expandGranted, toggle, toggleGroup, toList } from './matrix'
import { generateTempPassword, MemberAddForm } from './MemberAddForm'
import { RoleForm } from './RoleForm'
import type { PermissionEntry, Role } from './types'

const push = vi.fn()
const replace = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace, refresh: vi.fn() }) }))

afterEach(() => {
  push.mockReset()
  replace.mockReset()
})

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}
const bodyOf = (fetch: { mock: { calls: unknown[][] } }, i = 0) => JSON.parse((fetch.mock.calls[i]![1] as RequestInit).body as string)

const entry = (code: string): PermissionEntry => {
  const [mr, action] = code.split(':') as [string, string]
  const [module, resource] = mr.split('.') as [string, string]
  return { code, module, resource, action }
}
const catalog = ['sales.order:read', 'sales.order:manage', 'sales.order:approve', 'sales.report:read', 'identity.role:read', 'identity.role:manage'].map(entry)

describe('matrix', () => {
  it('จัดกลุ่มตาม module / แถว resource / คอลัมน์ action (read ก่อน)', () => {
    const g = buildMatrix(catalog)
    expect(g.map((x) => x.module)).toEqual(['sales', 'identity'])
    expect(g[0]!.actions).toEqual(['read', 'manage', 'approve'])
    expect(g[0]!.rows.map((r) => r.resource)).toEqual(['order', 'report'])
    expect(g[0]!.rows[1]!.cells.manage).toBeUndefined()
  })

  it('wildcard ของบทบาทเดิมแสดงเป็นช่องที่ครอบคลุม', () => {
    expect([...expandGranted(['sales.order:*'], catalog)]).toEqual(['sales.order:read', 'sales.order:manage', 'sales.order:approve'])
    expect(expandGranted(['*'], catalog).size).toBe(catalog.length)
  })

  it('เลือก action อื่น = เลือก read ด้วย; ยกเลิก read = ยกเลิกทั้งแถว; เลือกทั้ง module', () => {
    const row = buildMatrix(catalog)[0]!.rows[0]!
    let s = toggle(new Set(), row, 'approve', true)
    expect([...s].sort()).toEqual(['sales.order:approve', 'sales.order:read'])
    s = toggle(s, row, 'read', false)
    expect(s.size).toBe(0)
    s = toggleGroup(new Set(), buildMatrix(catalog)[1]!, true)
    expect(toList(s, catalog)).toEqual(['identity.role:read', 'identity.role:manage'])
  })
})

describe('MemberAddForm', () => {
  const roles: Role[] = [{ id: 'r1', company_id: 'c1', code: 'sales', name: 'ฝ่ายขาย', description: '', permissions: ['sales.order:read'], is_system: false }]

  it('ไม่พบบัญชี → เปิดส่วนสร้างบัญชีใหม่ แล้วส่งอีกครั้งพร้อม new_user', async () => {
    const fetch = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(json(404, { error: { code: 'user_not_found', message: 'user not found' } }))
      .mockResolvedValueOnce(json(201, { data: {} }))
    const user = userEvent.setup()
    render(<MemberAddForm apiPath="/companies/c1/members" listHref="/list" roles={roles} />)
    const t = texts.members
    await user.type(screen.getByLabelText(t.fields.email), 'New@Example.com')
    await user.click(screen.getByRole('checkbox', { name: /ฝ่ายขาย/ }))
    await user.click(screen.getByRole('button', { name: texts.common.save }))

    expect(bodyOf(fetch)).toEqual({ email: 'new@example.com', role_ids: ['r1'] })
    expect(await screen.findByLabelText(t.newAccount.name)).toBeInTheDocument()

    await user.type(screen.getByLabelText(t.newAccount.name), 'พนักงานใหม่')
    await user.click(screen.getByRole('button', { name: t.newAccount.generate }))
    const pw = (screen.getByLabelText(t.newAccount.password) as HTMLInputElement).value
    expect(pw).toHaveLength(12)
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(bodyOf(fetch, 1)).toEqual({ email: 'new@example.com', role_ids: ['r1'], new_user: { name: 'พนักงานใหม่', password: pw } })
    expect(push).toHaveBeenCalledWith('/list')
  })

  it('ต้องเลือกบทบาทอย่างน้อย 1', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch')
    const user = userEvent.setup()
    render(<MemberAddForm apiPath="/x" listHref="/list" roles={roles} />)
    await user.type(screen.getByLabelText(texts.members.fields.email), 'a@b.co')
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(screen.getByText(texts.members.fields.rolesRequired)).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalled()
  })

  it('รหัสผ่านชั่วคราวไม่มีตัวอักษรที่สับสนง่าย', () => {
    for (let i = 0; i < 20; i++) expect(generateTempPassword()).toMatch(/^[A-HJ-NP-Za-km-z2-9]{12}$/)
  })
})

describe('RoleForm', () => {
  it('สร้างบทบาท: POST รหัส ชื่อ และสิทธิ์ที่เลือก (เลือก manage = ได้ read ด้วย)', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(201, { data: {} }))
    const user = userEvent.setup()
    render(<RoleForm apiPath="/companies/c1/roles" listHref="/roles" catalog={catalog} canManage />)
    const f = texts.roles.fields
    await user.type(screen.getByLabelText(f.code), 'Sales Staff')
    await user.type(screen.getByLabelText(f.name), 'พนักงานขาย')
    await user.click(screen.getByRole('checkbox', { name: 'จัดการ ใบสั่งขาย' }))
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(bodyOf(fetch)).toEqual({ code: 'sales-staff', name: 'พนักงานขาย', description: '', permissions: ['sales.order:read', 'sales.order:manage'] })
  })

  it('ไม่เลือกสิทธิ์ = บันทึกไม่ได้; บทบาทของระบบ = อ่านอย่างเดียว ทุกช่องถูกเลือก', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch')
    const user = userEvent.setup()
    const { unmount } = render(<RoleForm apiPath="/x" listHref="/roles" catalog={catalog} canManage />)
    await user.type(screen.getByLabelText(texts.roles.fields.code), 'aa')
    await user.type(screen.getByLabelText(texts.roles.fields.name), 'x')
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(screen.getByText(texts.roles.noPermission)).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalled()
    unmount()

    const owner: Role = { id: 'o', company_id: 'c', code: 'owner', name: 'Owner', description: '', permissions: ['*'], is_system: true }
    render(<RoleForm apiPath="/x" listHref="/roles" catalog={catalog} role={owner} canManage />)
    const sales = screen.getByRole('region', { name: 'ขาย' })
    for (const cb of within(sales).getAllByRole('checkbox')) {
      expect(cb).toBeChecked()
      expect(cb).toBeDisabled()
    }
    expect(screen.queryByRole('button', { name: texts.common.save })).toBeNull()
  })
})

describe('ChangePasswordForm', () => {
  const t = texts.auth.changePassword
  it('รหัสใหม่ไม่ตรงกัน = ไม่ส่ง; ถูกต้อง = PUT /api/session/password แล้วไปหน้าเดิม', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }))
    const user = userEvent.setup()
    const assign = vi.fn()
    vi.stubGlobal('location', { ...window.location, assign })
    render(<ChangePasswordForm redirectTo="/c/1" email="a@b.co" />)
    await user.type(screen.getByLabelText(t.oldPassword), 'temp-pass-1')
    await user.type(screen.getByLabelText(t.newPassword), 'my-new-pass')
    await user.type(screen.getByLabelText(t.confirmPassword), 'my-new-pasX')
    await user.click(screen.getByRole('button', { name: t.submit }))
    expect(screen.getByText(t.mismatch)).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalled()

    await user.clear(screen.getByLabelText(t.confirmPassword))
    await user.type(screen.getByLabelText(t.confirmPassword), 'my-new-pass')
    await user.click(screen.getByRole('button', { name: t.submit }))
    expect(fetch).toHaveBeenCalledWith('/api/session/password', expect.objectContaining({ method: 'PUT' }))
    expect(bodyOf(fetch)).toEqual({ old_password: 'temp-pass-1', new_password: 'my-new-pass' })
    expect(assign).toHaveBeenCalledWith('/c/1')
    vi.unstubAllGlobals()
  })

  it('ช่องรหัสผ่านให้ password manager ทำงานได้ (autocomplete)', () => {
    render(<ChangePasswordForm redirectTo="/" email="a@b.co" />)
    expect(screen.getByLabelText(t.oldPassword)).toHaveAttribute('autocomplete', 'current-password')
    expect(screen.getByLabelText(t.newPassword)).toHaveAttribute('autocomplete', 'new-password')
  })
})
