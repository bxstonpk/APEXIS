import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { format, texts } from '@/config'
import { findApp } from '@/config/apps'
import { SalesDocForm } from '@/features/sales/SalesDocForm'
import { PurchaseFulfilForm } from './FulfilForm'
import { taxInvoicePayload, TaxInvoicesPanel } from './TaxInvoicesPanel'
import { flows, type PurchaseOrder } from './types'

/** จัดซื้อแยกสายซื้อสินค้า / จ้างบริการ: ตัวเลือกสินค้าตามชนิด, ตรวจรับงานไม่เกินที่สั่ง, บันทึกใบกำกับภาษีที่ได้รับ, ลำดับเมนู */

const push = vi.fn()
const refresh = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh }) }))

afterEach(() => {
  push.mockReset()
  refresh.mockReset()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
type Call = [string, RequestInit?]
const calls = (fetch: ReturnType<typeof vi.fn>) => fetch.mock.calls as unknown as Call[]
const posts = (fetch: ReturnType<typeof vi.fn>) => calls(fetch).filter(([, i]) => i?.method === 'POST')
const bodyOf = (c: Call) => JSON.parse(String(c[1]?.body)) as Record<string, unknown>
const t = texts.purchasing
const line = (field: string) => `${field} ${format(texts.sales.lines.line, { n: 1 })}`

describe('สายเอกสาร', () => {
  it('ตัวเลือกสินค้า: ใบสั่งจ้างค้นหาเฉพาะบริการ, ใบสั่งซื้อเฉพาะสินค้า', async () => {
    expect(flows.service.itemTypes).toBe('service')
    expect(flows.goods.itemTypes).toBe('stock,non_stock')
    const fetch = vi.fn(async () => json(200, { data: [] }))
    vi.stubGlobal('fetch', fetch)
    render(
      <SalesDocForm
        kind="work-order"
        itemTypes={flows.service.itemTypes}
        apiPath="/wo"
        partiesApi="/sup"
        itemsApi="/it"
        docsPath="/d"
        listHref="/l"
        vat={{ registered: true, rate: '7' }}
      />,
    )
    await userEvent.type(screen.getByRole('combobox', { name: line(texts.sales.lines.item) }), 'S')
    await vi.waitFor(() => expect(calls(fetch).some(([u]) => String(u).includes('/it?'))).toBe(true))
    const url = calls(fetch).find(([u]) => String(u).includes('/it?'))![0]
    expect(new URL(url, 'http://x').searchParams.get('type')).toBe('service')
  })

  it('เมนูจัดซื้อ: สายซื้อสินค้าก่อน แล้วสายจ้างบริการ', () => {
    expect(findApp('purchasing')!.menu.map((m) => m.path)).toEqual([
      'dashboard',
      'orders',
      'receipts',
      'invoices',
      'work-orders',
      'service-acceptances',
      'service-invoices',
      'credit-notes',
      'suppliers',
      'reports',
      'settings',
    ])
    expect(texts.apps.purchasing.menu['work-orders']).toBe('ใบสั่งจ้าง')
    expect(texts.apps.purchasing.menu['service-invoices']).toBe('ซื้อบริการ')
  })
})

const workOrder: PurchaseOrder = {
  kind: 'service',
  id: 'wo1',
  order_no: 'WO2026-000001',
  date: '2026-11-01',
  delivery_date: null,
  supplier: { id: 's1', code: 'V001', name: 'ผู้รับจ้าง ก' },
  supplier_branch_no: '00000',
  reference: '',
  vat_rate: '7',
  price_includes_vat: false,
  totals: { net: '4000', vat: '280', gross: '4280' },
  status: 'approved',
  description: '',
  vat_registered: true,
  payment_term_days: 30,
  created_by: 'u1',
  approved_at: '2026-11-01T00:00:00Z',
  closed_at: null,
  cancelled_at: null,
  lines: [
    {
      line_no: 1,
      item: { id: 'i1', code: 'SVC1', name: 'ค่าซ่อมบำรุง' },
      description: '',
      quantity: '4',
      unit_price: '1000',
      amounts: { net: '4000', vat: '280', gross: '4280' },
      received_quantity: '1',
      invoiced_quantity: '0',
    },
  ],
}

describe('ตรวจรับงาน', () => {
  it('ตรวจรับเกินที่ค้าง = แจ้งที่ช่อง ไม่ส่ง; ไม่มีช่องคลัง', async () => {
    const fetch = vi.fn(async () => json(201, { data: { id: 'sa1', document_no: 'SA2026-000001' } }))
    vi.stubGlobal('fetch', fetch)
    render(<PurchaseFulfilForm mode="receipt" service order={workOrder} apiPath="/sa" warehousesApi="/w" docsPath="/d" orderHref="/o" />)
    expect(screen.queryByRole('combobox', { name: t.fulfil.warehouse })).not.toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: t.fulfil.accepted })).toBeInTheDocument()
    const qty = screen.getByLabelText(`${t.fulfil.thisTime} SVC1`)
    expect(qty).toHaveValue('3')
    await userEvent.clear(qty)
    await userEvent.type(qty, '4')
    await userEvent.click(screen.getByRole('button', { name: t.actions.accept }))
    expect(screen.getByText(t.fulfil.overRemaining)).toBeInTheDocument()
    expect(posts(fetch)).toHaveLength(0)

    await userEvent.clear(qty)
    await userEvent.type(qty, '2')
    await userEvent.click(screen.getByRole('button', { name: t.actions.accept }))
    await vi.waitFor(() => expect(posts(fetch)).toHaveLength(1))
    const body = bodyOf(posts(fetch)[0]!)
    expect(body).toMatchObject({ order_id: 'wo1', lines: [{ order_line_no: 1, quantity: '2' }] })
    expect(body).not.toHaveProperty('warehouse_id')
  })
})

describe('ใบกำกับภาษีที่ได้รับ (ซื้อบริการ)', () => {
  it('ข้อมูลที่ส่ง: ภาษีเท่าที่ระบบเสนอ = ไม่ส่ง vat (ให้ backend คิดตามสัดส่วน), แก้แล้ว = ส่ง', () => {
    expect(taxInvoicePayload({ no: ' TX-001 ', date: '2026-11-20', vat: '140.00' }, '140')).toEqual({ tax_invoice_no: 'TX-001', tax_invoice_date: '2026-11-20' })
    expect(taxInvoicePayload({ no: 'TX-001', date: '2026-11-20', vat: '100' }, '140')).toEqual({ tax_invoice_no: 'TX-001', tax_invoice_date: '2026-11-20', vat: '100' })
  })

  it('ภาษีเกินที่รอ = แจ้งที่ช่อง ไม่ส่ง; ถูกต้อง = POST แล้ว refresh', async () => {
    const fetch = vi.fn(async () => json(201, { data: { id: 'tx1' } }))
    vi.stubGlobal('fetch', fetch)
    const v = t.vat
    render(<TaxInvoicesPanel apiPath="/si/1/tax-invoices" records={[]} withInvoice={null} undue="280" suggested="140" canPost entryBase={null} />)
    await userEvent.click(screen.getByRole('button', { name: v.record }))
    const vat = screen.getByLabelText(v.fields.vat, { exact: false })
    expect(vat).toHaveValue('140')
    await userEvent.type(screen.getByLabelText(v.fields.no, { exact: false }), 'TX-001')
    await userEvent.clear(vat)
    await userEvent.type(vat, '300')
    const submit = screen.getAllByRole('button', { name: v.record }).at(-1)!
    await userEvent.click(submit)
    expect(screen.getByText(format(v.overUndue, { max: '280.00' }))).toBeInTheDocument()
    expect(posts(fetch)).toHaveLength(0)

    await userEvent.clear(vat)
    await userEvent.type(vat, '140')
    await userEvent.click(submit)
    await vi.waitFor(() => expect(posts(fetch)).toHaveLength(1))
    expect(String(posts(fetch)[0]![0])).toContain('/si/1/tax-invoices')
    expect(bodyOf(posts(fetch)[0]!)).toEqual({ tax_invoice_no: 'TX-001', tax_invoice_date: expect.any(String) })
    await vi.waitFor(() => expect(refresh).toHaveBeenCalled())
  })
})
