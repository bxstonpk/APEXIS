import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { format, texts } from '@/config'
import { findApp, visibleApps } from '@/config/apps'
import { SalesDocForm } from '@/features/sales/SalesDocForm'
import { CreateMenu } from './CreateMenu'
import { PurchaseFulfilForm } from './FulfilForm'
import { taxInvoicePayload, TaxInvoicesPanel } from './TaxInvoicesPanel'
import { docTypeMenu, documentsHref, flows, PURCHASE_DOC_TYPES, type PurchaseOrder } from './types'

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

  it('เมนูจัดซื้อ: เอกสารซื้อเมนูเดียวรวม 6 ชนิด — แท็บเน้นในหน้าเอกสารทุกชนิด', () => {
    const menu = findApp('purchasing')!.menu
    expect(menu.map((m) => m.path)).toEqual(['dashboard', 'documents', 'credit-notes', 'suppliers', 'reports', 'settings'])
    expect(texts.apps.purchasing.menu.documents).toBe('เอกสารซื้อ')
    const docs = menu.find((m) => m.path === 'documents')!
    expect(docs.also).toEqual(['orders', 'receipts', 'invoices', 'work-orders', 'service-acceptances', 'service-invoices'])
    // มีสิทธิ์อ่านชนิดใดชนิดหนึ่ง = เห็นเมนู
    expect(visibleApps(['purchasing.service_invoice:read']).find((a) => a.key === 'purchasing')?.menu.map((m) => m.path)).toEqual(['documents'])
    expect(visibleApps(['purchasing.supplier:read']).find((a) => a.key === 'purchasing')?.menu.map((m) => m.path)).toEqual(['suppliers'])
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

describe('เอกสารซื้อ (รายการรวม)', () => {
  it('ลิงก์กลับรายการคงตัวกรองเดิม (รวม type) — ไม่มี type เดิม = ชนิดของเอกสารนี้', () => {
    expect(documentsHref('c1', 'work_order')).toBe('/c/c1/purchasing/documents?type=work_order')
    expect(documentsHref('c1', 'order', 'type=order%2Cinvoice&q=abc&page=2&foo=x')).toBe('/c/c1/purchasing/documents?type=order%2Cinvoice&q=abc&page=2')
    expect(Object.keys(docTypeMenu).sort()).toEqual([...PURCHASE_DOC_TYPES].sort())
  })

  it('ชื่อชนิดเอกสารครบ 6 ชนิดสำหรับป้ายในตาราง', () => {
    expect(PURCHASE_DOC_TYPES.map((t) => texts.purchasing.documents.types[t])).toEqual(['ใบสั่งซื้อ', 'ใบรับสินค้า', 'ซื้อสินค้า', 'ใบสั่งจ้าง', 'ตรวจรับงาน', 'ซื้อบริการ'])
  })

  it('ปุ่มสร้าง: เมนูกลุ่มสินค้า / บริการ, ลูกศรเลื่อน, Esc ปิดแล้วโฟกัสกลับปุ่ม', async () => {
    render(
      <CreateMenu
        label="สร้าง"
        groups={[
          { label: 'สินค้า', items: [{ label: 'ใบสั่งซื้อ', href: '/po/new' }, { label: 'ซื้อสินค้า', href: '/pi/new' }] },
          { label: 'บริการ', items: [{ label: 'ใบสั่งจ้าง', href: '/wo/new' }] },
        ]}
      />,
    )
    const button = screen.getByRole('button', { name: 'สร้าง' })
    expect(button).toHaveAttribute('aria-expanded', 'false')
    button.focus()
    await userEvent.keyboard('{ArrowDown}')
    expect(button).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getAllByRole('group').map((g) => g.getAttribute('aria-labelledby') && g.textContent)).toEqual(['สินค้าใบสั่งซื้อซื้อสินค้า', 'บริการใบสั่งจ้าง'])
    await vi.waitFor(() => expect(screen.getByRole('menuitem', { name: 'ใบสั่งซื้อ' })).toHaveFocus())
    await userEvent.keyboard('{ArrowDown}{ArrowDown}')
    expect(screen.getByRole('menuitem', { name: 'ใบสั่งจ้าง' })).toHaveFocus()
    expect(screen.getByRole('menuitem', { name: 'ใบสั่งจ้าง' })).toHaveAttribute('href', '/wo/new')
    await userEvent.keyboard('{Escape}')
    expect(button).toHaveAttribute('aria-expanded', 'false')
    expect(button).toHaveFocus()
  })
})
