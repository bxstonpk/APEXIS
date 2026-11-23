import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { CategoryForm, ItemForm, UomForm } from './MasterForms'
import type { Item } from './types'

const push = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh: vi.fn() }) }))

afterEach(() => {
  push.mockReset()
  vi.unstubAllGlobals()
})

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const f = texts.inventory.fields
const bodyOf = (fetch: ReturnType<typeof vi.fn>, method = 'POST') => {
  const call = fetch.mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === method)!
  return JSON.parse(String((call[1] as RequestInit).body))
}

describe('UomForm', () => {
  it('ไม่ส่งเมื่อขาดรหัส / ชื่อ', async () => {
    const fetch = vi.fn()
    vi.stubGlobal('fetch', fetch)
    render(<UomForm apiPath="/companies/c1/uoms" listHref="/l" canManage />)
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    expect(fetch).not.toHaveBeenCalled()
    expect(screen.getAllByRole('textbox').some((el) => el.getAttribute('aria-invalid') === 'true')).toBe(true)
  })
})

describe('CategoryForm', () => {
  it('บัญชีที่ไม่ได้เลือกส่ง null', async () => {
    const fetch = vi.fn(async () => json(201, { data: { id: 'cat1' } }))
    vi.stubGlobal('fetch', fetch)
    render(<CategoryForm apiPath="/companies/c1/item-categories" accountsApi="/companies/c1/accounts" listHref="/l" canManage />)
    await userEvent.type(screen.getByLabelText(f.code, { exact: true }), 'fg')
    await userEvent.type(screen.getByLabelText(f.name, { exact: true }), 'สินค้าสำเร็จรูป')
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    expect(bodyOf(fetch)).toEqual({
      code: 'FG',
      name: 'สินค้าสำเร็จรูป',
      status: 'active',
      purchase_account_id: null,
      inventory_account_id: null,
      revenue_account_id: null,
    })
  })
})

describe('ItemForm', () => {
  const item: Item = {
    id: 'i1',
    code: 'SVC',
    name: 'บริการติดตั้ง',
    name_en: '',
    type: 'service',
    uom: { id: 'u1', code: 'JOB', name: 'งาน' },
    category: null,
    reorder_point: null,
    status: 'active',
    purchase_account_id: null,
    inventory_account_id: null,
    revenue_account_id: null,
  }

  it('บริการไม่มีช่องจุดสั่งซื้อ และส่ง reorder_point / id ที่ว่างเป็น null', async () => {
    const fetch = vi.fn(async () => json(200, { data: { id: 'i1' } }))
    vi.stubGlobal('fetch', fetch)
    render(
      <ItemForm apiPath="/companies/c1/items" uomsApi="/u" categoriesApi="/c" accountsApi="/a" listHref="/l" canManage item={item} />,
    )
    expect(screen.queryByLabelText(f.reorderPoint)).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    expect(bodyOf(fetch, 'PUT')).toEqual({
      code: 'SVC',
      name: 'บริการติดตั้ง',
      name_en: '',
      type: 'service',
      uom_id: 'u1',
      category_id: null,
      reorder_point: null,
      status: 'active',
      purchase_account_id: null,
      inventory_account_id: null,
      revenue_account_id: null,
    })
  })

  it('สินค้านับสต็อกส่งจุดสั่งซื้อที่กรอก; ไม่มีหน่วยนับ = ไม่ส่ง', async () => {
    const fetch = vi.fn(async () => json(200, { data: { id: 'i1' } }))
    vi.stubGlobal('fetch', fetch)
    const { unmount } = render(<ItemForm apiPath="/companies/c1/items" uomsApi="/u" categoriesApi="/c" accountsApi="/a" listHref="/l" canManage />)
    await userEvent.type(screen.getByLabelText(f.code, { exact: true }), 'm1')
    await userEvent.type(screen.getByLabelText(f.name, { exact: true }), 'เหล็ก')
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    expect((fetch.mock.calls as unknown as [string, RequestInit?][]).some(([, init]) => init?.method === 'POST')).toBe(false)
    unmount()

    render(
      <ItemForm
        apiPath="/companies/c1/items"
        uomsApi="/u"
        categoriesApi="/c"
        accountsApi="/a"
        listHref="/l"
        canManage
        item={{ ...item, code: 'M1', type: 'stock', reorder_point: null }}
      />,
    )
    await userEvent.type(screen.getByLabelText(f.reorderPoint), '25')
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    expect(bodyOf(fetch, 'PUT')).toMatchObject({ code: 'M1', type: 'stock', uom_id: 'u1', reorder_point: '25' })
  })
})
