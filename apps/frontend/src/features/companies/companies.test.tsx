import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { CompanyForm } from './CompanyForm'

const push = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, refresh: vi.fn(), replace: vi.fn() }) }))

afterEach(() => {
  push.mockReset()
  vi.unstubAllGlobals()
})

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const t = texts.companies.new
const bt = texts.businessTypes

async function fillBasics() {
  await userEvent.type(screen.getByLabelText(new RegExp(t.code)), 'ACME')
  await userEvent.type(screen.getByLabelText(new RegExp(t.name)), 'บริษัท แอคมี จำกัด')
}

describe('CompanyForm — ประเภทธุรกิจ', () => {
  it('แสดงครบ 4 ประเภทเป็น radio ในกลุ่มเดียว', () => {
    render(<CompanyForm />)
    const group = screen.getByRole('group', { name: new RegExp(t.businessType) })
    expect(group).toBeInTheDocument()
    expect(screen.getAllByRole('radio')).toHaveLength(4)
    for (const name of Object.values(bt.names)) expect(screen.getByRole('radio', { name: new RegExp(name) })).toBeInTheDocument()
  })

  it('ยังไม่เลือกประเภท: แจ้งใต้กลุ่มและไม่ส่ง', async () => {
    const fetch = vi.fn(async () => json(201, { data: { id: 'c1' } }))
    vi.stubGlobal('fetch', fetch)
    render(<CompanyForm />)
    await fillBasics()
    await userEvent.click(screen.getByRole('button', { name: t.submit }))
    expect(screen.getByRole('alert')).toHaveTextContent(t.businessTypeRequired)
    expect(fetch).not.toHaveBeenCalled()
  })

  it('เลือกธุรกิจผลิตสินค้า: ส่ง business_type=manufacturing แล้วไปหน้ารวมแอปของบริษัทใหม่', async () => {
    const fetch = vi.fn(async () => json(201, { data: { id: 'c1' } }))
    vi.stubGlobal('fetch', fetch)
    render(<CompanyForm />)
    await fillBasics()
    await userEvent.click(screen.getByRole('radio', { name: new RegExp(bt.names.manufacturing) }))
    await userEvent.click(screen.getByRole('button', { name: t.submit }))
    const [, init] = (fetch.mock.calls as unknown as [string, RequestInit][])[0]!
    expect(JSON.parse(String(init.body))).toMatchObject({ code: 'ACME', business_type: 'manufacturing' })
    expect(push).toHaveBeenCalledWith('/c/c1')
  })

  it('เลือกด้วยคีย์บอร์ด (ลูกศร) ได้', async () => {
    render(<CompanyForm />)
    const first = screen.getByRole('radio', { name: new RegExp(bt.names.service) })
    await userEvent.click(first)
    await userEvent.keyboard('{ArrowDown}')
    expect(screen.getByRole('radio', { name: new RegExp(bt.names.trading) })).toBeChecked()
  })
})
