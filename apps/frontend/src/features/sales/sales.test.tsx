import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { toUnits } from '@/features/accounting/money'
import { DocActions } from './Actions'
import { CustomerForm } from './Forms'
import { FulfilForm, remaining } from './FulfilForm'
import type { Order } from './types'

const push = vi.fn()
const refresh = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh }) }))

afterEach(() => {
  push.mockReset()
  refresh.mockReset()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

const t = texts.sales
const ref = (id: string) => ({ id, code: id.toUpperCase(), name: `สินค้า ${id}` })
const amounts = { net: '0', vat: '0', gross: '0' }

function order(lines: [string, string, string][]): Order {
  return {
    id: 'o1',
    document_no: 'SO2026-000001',
    date: '2026-11-01',
    customer: { id: 'cu1', code: 'C001', name: 'ลูกค้า ก' },
    customer_branch_no: '00000',
    reference: '',
    vat_rate: '7',
    price_includes_vat: false,
    totals: amounts,
    status: 'approved',
    quotation_id: null,
    delivery_date: null,
    description: '',
    payment_term_days: 30,
    created_by: 'u1',
    approved_at: '2026-11-01T00:00:00Z',
    closed_at: null,
    cancelled_at: null,
    lines: lines.map(([quantity, delivered_quantity, invoiced_quantity], i) => ({
      line_no: i + 1,
      item: ref(`i${i + 1}`),
      description: '',
      quantity,
      unit_price: '100',
      amounts,
      delivered_quantity,
      invoiced_quantity,
    })),
  }
}

describe('remaining', () => {
  const o = order([
    ['10', '6', '2'],
    ['5', '5', '5'],
  ])
  it('ส่งของ = สั่ง − ส่งแล้ว; ออกบิลจากของที่ส่ง = ส่งแล้ว − ออกบิลแล้ว; ส่งพร้อมออกบิล = สั่ง − ส่งแล้ว', () => {
    expect(remaining(o, 'delivery', false)).toEqual({ 1: toUnits('4'), 2: 0n })
    expect(remaining(o, 'invoice', false)).toEqual({ 1: toUnits('4'), 2: 0n })
    expect(remaining(o, 'invoice', true)).toEqual({ 1: toUnits('4'), 2: 0n })
    expect(remaining(order([['10', '0', '0']]), 'invoice', false)).toEqual({ 1: 0n })
  })
})

describe('FulfilForm', () => {
  it('ยังไม่เคยส่งของ = ตั้งต้นเป็นส่งพร้อมออกบิล และส่งจำนวนคงเหลือทุกบรรทัด', async () => {
    const fetch = vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === 'POST') return json(201, { data: { id: 'inv1', document_no: 'IV2026-000001' } })
      return json(200, { data: [] })
    })
    vi.stubGlobal('fetch', fetch)
    render(<FulfilForm mode="invoice" order={order([['3', '0', '0']])} apiPath="/companies/c1/sales-invoices" warehousesApi="/companies/c1/warehouses" docsPath="/c/c1/sales/invoices" orderHref="/o" />)
    expect(screen.getByRole('checkbox', { name: new RegExp(t.fields.withDelivery.slice(0, 10)) })).toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: t.actions.invoice }))
    const post = fetch.mock.calls.find(([, init]) => init?.method === 'POST')!
    const body = JSON.parse(String(post[1]!.body))
    expect(body).toMatchObject({ order_id: 'o1', with_delivery: true, lines: [{ order_line_no: 1, quantity: '3' }] })
    expect(push).toHaveBeenCalledWith('/c/c1/sales/invoices/inv1')
  })

  it('จำนวนเกินคงเหลือ = แจ้งที่ช่องและไม่ส่ง', async () => {
    const fetch = vi.fn(async () => json(200, { data: [] }))
    vi.stubGlobal('fetch', fetch)
    render(<FulfilForm mode="delivery" order={order([['10', '6', '0']])} apiPath="/d" warehousesApi="/w" docsPath="/docs" orderHref="/o" />)
    const qty = screen.getByLabelText(`${t.lines.thisTime} I1`)
    expect(qty).toHaveValue('4')
    await userEvent.clear(qty)
    await userEvent.type(qty, '5')
    await userEvent.click(screen.getByRole('button', { name: t.actions.deliver }))
    expect(screen.getByText(t.lines.overRemaining)).toBeInTheDocument()
    expect((fetch.mock.calls as unknown as [string, RequestInit?][]).some(([, init]) => init?.method === 'POST')).toBe(false)
  })
})

describe('DocActions', () => {
  it('อนุมัติ: ยืนยันก่อน, PATCH status และแสดงคำเตือนวงเงินที่ backend ส่งกลับ', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const fetch = vi.fn(async () => json(200, { data: { id: 'o1', status: 'approved', warnings: ['เกินวงเงินเครดิต'] } }))
    vi.stubGlobal('fetch', fetch)
    render(<DocActions apiPath="/companies/c1/sales-orders/o1" listHref="/l" actions={[{ kind: 'patch', status: 'approved', label: t.actions.approve, primary: true }]} />)
    await userEvent.click(screen.getByRole('button', { name: t.actions.approve }))
    expect(JSON.parse(String((fetch.mock.calls[0] as unknown as [string, RequestInit])[1].body))).toEqual({ status: 'approved' })
    expect(await screen.findByText('เกินวงเงินเครดิต')).toBeInTheDocument()
    expect(refresh).toHaveBeenCalled()
  })

  it('ผู้ใช้กดยกเลิกในกล่องยืนยัน = ไม่เรียก API', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const fetch = vi.fn()
    vi.stubGlobal('fetch', fetch)
    render(<DocActions apiPath="/x" listHref="/l" actions={[{ kind: 'delete', label: t.actions.delete, confirm: t.actions.confirm.delete! }]} />)
    await userEvent.click(screen.getByRole('button', { name: t.actions.delete }))
    expect(fetch).not.toHaveBeenCalled()
  })
})

describe('CustomerForm', () => {
  const registrant = {
    tax_id: '0745569002816',
    branch_no: '00000',
    branch_name: 'บริษัท ตัวอย่าง จำกัด',
    name: 'บริษัท ตัวอย่าง จำกัด',
    tax_type: 'juristic',
    phone: '',
    vat_registered_on: '21/09/2569',
    address: 'เลขที่ 72/7 หมู่ที่ 3 ตำบลท่าทราย อำเภอเมืองสมุทรสาคร จังหวัดสมุทรสาคร 74000',
    address_detail: { house_no: '72/7', moo: '3', sub_district: 'ท่าทราย', district: 'เมืองสมุทรสาคร', province: 'สมุทรสาคร', postal_code: '74000' },
  }

  it('กรอกเลขครบ 13 หลัก = ดึงข้อมูลจากกรมสรรพากรเอง แล้วส่งที่อยู่แยกช่อง', async () => {
    const fetch = vi.fn(async (url: string, init?: RequestInit) => {
      if (url.includes('vat-registrants')) return json(200, { data: [registrant] })
      if (init?.method === 'POST') return json(201, { data: { id: 'cu1' } })
      return json(200, { data: [] })
    })
    vi.stubGlobal('fetch', fetch)
    render(<CustomerForm apiPath="/companies/c1/customers" lookupApi="/companies/c1/vat-registrants" customersPath="/c/c1/sales/customers" listHref="/c/c1/sales/customers" canManage />)
    await userEvent.type(screen.getByLabelText(texts.partners.lookup.taxId), '0745569002816')
    expect(await screen.findByText(/เติมข้อมูลจากกรมสรรพากรแล้ว/)).toBeInTheDocument()
    expect(screen.getByLabelText(t.customers.fields.name, { exact: true })).toHaveValue('บริษัท ตัวอย่าง จำกัด')
    expect(screen.getByText(registrant.address)).toBeInTheDocument() // ตัวอย่างที่อยู่ที่จะพิมพ์

    await userEvent.type(screen.getByLabelText(t.customers.fields.code, { exact: true }), 'c001')
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    const post = fetch.mock.calls.find(([, init]) => init?.method === 'POST')!
    const body = JSON.parse(String(post[1]!.body))
    expect(body).toMatchObject({
      code: 'C001',
      tax_id: '0745569002816',
      branch_no: '00000',
      credit_limit: null,
      address_detail: { house_no: '72/7', moo: '3', soi: '', province: 'สมุทรสาคร', postal_code: '74000' },
    })
    expect(body).not.toHaveProperty('address')
  })

  it('ไม่พบในฐานข้อมูลผู้จด VAT = แจ้งให้กรอกเอง; เลขไม่ครบ 13 หลัก = ไม่ส่ง', async () => {
    const fetch = vi.fn(async (url: string) =>
      url.includes('vat-registrants')
        ? json(404, { error: { code: 'vat_registrant_not_found', message: 'x' } })
        : url.includes('?q=')
          ? json(200, { data: [] })
          : json(201, { data: { id: 'cu1' } }),
    )
    vi.stubGlobal('fetch', fetch)
    render(<CustomerForm apiPath="/companies/c1/customers" lookupApi="/companies/c1/vat-registrants" customersPath="/c/c1/sales/customers" listHref="/c/c1/sales/customers" canManage />)
    const taxId = screen.getByLabelText(texts.partners.lookup.taxId)
    await userEvent.type(taxId, '12345')
    expect(screen.getByRole('button', { name: texts.partners.lookup.button })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    expect(screen.getByText(texts.validation.taxId)).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalled()

    await userEvent.type(taxId, '67890123')
    expect(await screen.findByText(texts.partners.lookup.notFound)).toBeInTheDocument()
  })
})

describe('CustomerForm: เลขผู้เสียภาษีซ้ำ', () => {
  it('เลขนี้มีรหัสอยู่แล้ว = แจ้งพร้อมลิงก์ไปรหัสเดิม และไม่ดึงข้อมูลกรมสรรพากร', async () => {
    const fetch = vi.fn(async (url: string) =>
      url.includes('?q=') ? json(200, { data: [{ id: 'old', code: 'C001', name: 'บริษัท เดิม จำกัด', tax_id: '0745569002816' }] }) : json(200, { data: [] }),
    )
    vi.stubGlobal('fetch', fetch)
    render(<CustomerForm apiPath="/companies/c1/customers" lookupApi="/companies/c1/vat-registrants" customersPath="/c/c1/sales/customers" listHref="/l" canManage />)
    await userEvent.type(screen.getByLabelText(texts.partners.lookup.taxId), '0745569002816')
    expect(await screen.findByRole('link', { name: 'เปิดรหัส C001' })).toHaveAttribute('href', '/c/c1/sales/customers/old')
    expect(fetch.mock.calls.some(([u]) => String(u).includes('vat-registrants'))).toBe(false)
  })
})

describe('warningText', () => {
  it('แปลคำเตือนวงเงินจาก backend เป็นภาษาไทย และคงข้อความที่ไม่รู้จัก', async () => {
    const { warningText } = await import('./Actions')
    expect(warningText('credit limit exceeded: customer C001 exposure 4066, limit 3000 (over by 1066)')).toBe(
      'ลูกค้า C001 มียอดค้างรวมใบนี้ 4,066.00 เกินวงเงินเครดิต 3,000.00 อยู่ 1,066.00',
    )
    expect(warningText('other')).toBe('other')
  })
})
