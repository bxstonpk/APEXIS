import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { format, texts } from '@/config'
import { findApp, menuAvailableFor, visibleApps } from '@/config/apps'
import { creditNotePrintTitle } from '@/features/credit-notes/types'
import { MoneyDocForm } from '@/features/finance/MoneyDocForm'
import { receiptPrintTitle } from '@/features/finance/receiptPrint'
import { salePrintTitle } from './printTitles'
import { SaleForm } from './SaleForm'
import type { Order } from './types'

/** เอกสารขายตามจุดเกิดภาษี: ใบแจ้งหนี้ค่าบริการ, หักมัดจำ, รับเงินตามใบวางบิล (การเงิน), ชื่อเอกสารพิมพ์, เมนูตามประเภทธุรกิจ */

const push = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh: vi.fn() }) }))

afterEach(() => {
  push.mockReset()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const posts = (fetch: ReturnType<typeof vi.fn>) => (fetch.mock.calls as unknown as [string, RequestInit?][]).filter(([, i]) => i?.method === 'POST')
const t = texts.sales
const line = (field: string) => `${field} ${format(t.lines.line, { n: 1 })}`
const apis = { submit: '/sv', customers: '/cu', items: '/it', warehouses: '/w', openDeposits: '/dep/open' }

const order: Order = {
  id: 'o1',
  document_no: 'SO2026-000001',
  date: '2026-11-01',
  customer: { id: 'cu1', code: 'C001', name: 'ลูกค้า ก' },
  customer_branch_no: '00000',
  reference: '',
  vat_rate: '7',
  price_includes_vat: false,
  totals: { net: '1000', vat: '70', gross: '1070' },
  status: 'approved',
  quotation_id: null,
  delivery_date: null,
  description: '',
  payment_term_days: 30,
  created_by: 'u1',
  approved_at: '2026-11-01T00:00:00Z',
  closed_at: null,
  cancelled_at: null,
  lines: [{ line_no: 1, item: { id: 's1', code: 'S1', name: 'ค่าบริการ', type: 'service' }, description: '', quantity: '1', unit_price: '1000', amounts: { net: '1000', vat: '70', gross: '1070' }, invoiced_quantity: '0' }],
}

describe('ใบแจ้งหนี้ค่าบริการ', () => {
  it('เลือกสินค้าที่ไม่ใช่บริการ = แจ้งที่ช่องและไม่ส่ง', async () => {
    const fetch = vi.fn(async (raw: string) => {
      const url = raw.replace('/api/v1', '')
      if (url.startsWith('/it/g1')) return json(200, { data: { id: 'g1', type: 'stock' } })
      if (url.startsWith('/it')) return json(200, { data: [{ id: 'g1', code: 'G1', name: 'สินค้า' }] })
      return json(200, { data: [] })
    })
    vi.stubGlobal('fetch', fetch)
    render(<SaleForm kind="service_invoice" apis={apis} vat={{ registered: true, rate: '7' }} docsPath="/d" listHref="/l" />)
    await userEvent.type(screen.getByRole('combobox', { name: line(t.lines.item) }), 'G')
    await userEvent.click(await screen.findByRole('option'))
    await vi.waitFor(() => expect(fetch.mock.calls.some(([u]) => String(u).includes('/it/g1'))).toBe(true))
    await userEvent.type(screen.getByLabelText(line(t.lines.unitPrice)), '500')
    await userEvent.click(screen.getByRole('button', { name: t.actions.postSale }))
    expect(screen.getByText(t.lines.serviceOnly)).toBeInTheDocument()
    expect(posts(fetch)).toHaveLength(0)
    // ตัวเลือกสินค้าค้นหาเฉพาะชนิดบริการ
    expect(fetch.mock.calls.some(([u]) => String(u).includes('/it?') && String(u).includes('type=service'))).toBe(true)
  })

  it('บอกว่าภาษีส่วนนี้ยังไม่ถึงกำหนด', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(200, { data: [] })))
    render(<SaleForm kind="service_invoice" order={order} apis={apis} vat={null} docsPath="/d" listHref="/l" />)
    expect(screen.getByText(t.totals.undue)).toBeInTheDocument()
  })
})

describe('หักเงินมัดจำ', () => {
  const deposits = [{ id: 'd1', document_no: 'DP2026-000001', date: '2026-11-02', vat_rate: '7', base: '300', vat: '21', remaining_amount: '300' }]

  it('เกินคงเหลือ = แจ้งที่ช่องและไม่ส่ง; ยอดถูกต้อง = ส่ง deposits และหัก VAT ของมัดจำจากยอดที่ต้องชำระ', async () => {
    const fetch = vi.fn(async (raw: string, init?: RequestInit) => {
      const url = raw.replace('/api/v1', '')
      if (init?.method === 'POST') return json(201, { data: { id: 'sv1', document_no: 'SV2026-000001' } })
      if (url.startsWith('/dep/open')) return json(200, { data: deposits })
      return json(200, { data: [] })
    })
    vi.stubGlobal('fetch', fetch)
    render(<SaleForm kind="service_invoice" order={order} apis={apis} vat={null} docsPath="/d" listHref="/l" />)
    const input = await screen.findByLabelText(`${t.depositsBlock.apply} DP2026-000001`)
    expect(fetch.mock.calls.some(([u]) => String(u).endsWith('/dep/open?customer_id=cu1'))).toBe(true)
    await userEvent.type(input, '400')
    await userEvent.click(screen.getByRole('button', { name: t.actions.postSale }))
    expect(screen.getByText(format(t.depositsBlock.over, { max: '300.00' }))).toBeInTheDocument()
    expect(posts(fetch)).toHaveLength(0)

    await userEvent.clear(input)
    await userEvent.type(input, '200')
    // 1,070 − (200 + VAT 14) = 856
    expect(screen.getByText('856.00')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: t.actions.postSale }))
    const body = JSON.parse(String(posts(fetch)[0]![1]!.body))
    expect(body).toMatchObject({ order_id: 'o1', deposits: [{ deposit_id: 'd1', amount: '200' }] })
    expect(body).not.toHaveProperty('warehouse_id')
  })
})

describe('รับเงิน (การเงิน) ตามใบวางบิล', () => {
  it('เปิดจากใบวางบิล = เลือกใบแจ้งหนี้ในใบวางบิลพร้อมยอดค้าง และแสดงชนิด / ภาษียังไม่ถึงกำหนด', async () => {
    const fetch = vi.fn(async (raw: string) => {
      const url = raw.replace('/api/v1', '')
      if (url.startsWith('/bn')) return json(200, { data: [{ id: 'bn1', document_no: 'BN2026-000001', due_date: '2026-11-30', outstanding: '1070' }] })
      if (url.startsWith('/open')) return json(200, { data: [{ id: 'sv1', document_no: 'SV2026-000001', partner_ref: '', date: '2026-11-01', due_date: '2026-11-30', net: '1000', gross: '1070', outstanding: '1070', kind: 'service_invoice', vat_undue: '70', tax_invoice: false }] })
      return json(200, { data: [] })
    })
    vi.stubGlobal('fetch', fetch)
    render(
      <MoneyDocForm
        mode="receipt"
        docsPath="/r"
        listHref="/r"
        initialParty={{ id: 'cu1', code: 'C001', name: 'ลูกค้า ก' }}
        initialBillingNote="bn1"
        api={{ submit: '/receipts', open: '/open', parties: '/cu', accounts: '/acc', cashAccounts: '/cash', billingNotes: '/bn' }}
      />,
    )
    const f = texts.finance
    const box = await screen.findByRole('checkbox', { name: format(f.invoices.select, { no: 'SV2026-000001' }) })
    expect(box).toBeChecked()
    expect(screen.getByLabelText(`${f.invoices.pay} SV2026-000001`)).toHaveValue('1070')
    expect(fetch.mock.calls.some(([u]) => String(u).includes('billing_note_id=bn1'))).toBe(true)
    const row = box.closest('tr')!
    expect(within(row).getByText(f.receipts.kindBadge.service_invoice!)).toBeInTheDocument()
    expect(within(row).getByText(format(f.receipts.vatUndue, { vat: '70.00' }), { exact: false })).toBeInTheDocument()
  })
})

describe('ชื่อเอกสารพิมพ์', () => {
  const p = t.print
  it('ใบรับเงิน: มีภาษีที่เกิด = ใบเสร็จรับเงิน / ใบกำกับภาษี, ไม่มี = ใบเสร็จรับเงิน', () => {
    const vatLines = [{ invoice: { id: 'sv1', document_no: 'SV1' }, base: '1000', vat: '70' }]
    expect(receiptPrintTitle({ tax_invoice: true, vat_lines: vatLines })).toBe(p.receiptTax)
    expect(receiptPrintTitle({ tax_invoice: false, vat_lines: [] })).toBe(p.receipt)
    expect(receiptPrintTitle({})).toBe(p.receipt)
  })
  it('ขายสินค้า / ใบแจ้งหนี้ค่าบริการ / ใบกำกับภาษีก่อนรับเงิน', () => {
    expect(salePrintTitle('invoice', true)).toBe(p.deliveryTaxInvoice)
    expect(salePrintTitle('invoice', false)).toBe(p.deliveryInvoice)
    expect(salePrintTitle('service_invoice', true)).toBe(p.invoice)
    expect(salePrintTitle('service_invoice', true, true)).toBe(p.taxInvoiceOnly)
  })
  it('ใบลดหนี้: ภาษียังไม่ถึงกำหนด = ใบลดหนี้ (ไม่ใช่ใบกำกับภาษี)', () => {
    const c = texts.creditNotes.print
    expect(creditNotePrintTitle(true, 'undue')).toBe(c.title)
    expect(creditNotePrintTitle(true, 'output')).toBe(c.titleVat)
    expect(creditNotePrintTitle(true, undefined)).toBe(c.titleVat)
    expect(creditNotePrintTitle(false, 'output')).toBe(c.title)
  })
})

describe('เมนูแอปขาย', () => {
  const salesMenu = (bt?: Parameters<typeof visibleApps>[1]) => visibleApps(['*'], bt).find((a) => a.key === 'sales')!.menu.map((m) => m.path)

  it('ไม่มีใบส่งของ / รับชำระเงิน / ขายเงินสด — รับเงินยืนยันที่แอปการเงิน', () => {
    const all = findApp('sales')!.menu.map((m) => m.path)
    for (const p of ['deliveries', 'receipts', 'cash-sales']) expect(all).not.toContain(p)
  })

  it.each([
    ['trading', true, false],
    ['manufacturing', true, false],
    ['service', false, true],
    ['construction', false, true],
  ] as const)('%s: ขายสินค้า=%s, ใบแจ้งหนี้=%s', (bt, goods, service) => {
    const menu = salesMenu(bt)
    expect(menu.includes('invoices')).toBe(goods)
    expect(menu.includes('service-invoices')).toBe(service)
    expect(menu).toEqual(expect.arrayContaining(['quotations', 'orders', 'deposits', 'billing-notes', 'credit-notes']))
    expect(menuAvailableFor('sales', 'invoices', bt)).toBe(goods)
    expect(menuAvailableFor('sales', 'service-invoices', bt)).toBe(service)
  })

  it('ไม่รู้ประเภทธุรกิจ = แสดงทั้งสองเมนู', () => {
    expect(salesMenu()).toEqual(expect.arrayContaining(['invoices', 'service-invoices']))
    expect(texts.apps.sales.menu['service-invoices']).toBe('ใบแจ้งหนี้')
  })
})
