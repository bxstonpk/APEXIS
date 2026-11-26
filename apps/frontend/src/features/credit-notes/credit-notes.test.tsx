import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { CreditNoteForm } from './CreditNoteForm'
import type { CreditableInvoice } from './types'

const push = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh: vi.fn() }) }))

afterEach(() => {
  push.mockReset()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const t = texts.creditNotes
const f = t.form

const invoice: CreditableInvoice = {
  id: 'inv1',
  no: 'IV2026-000001',
  party: 'ลูกค้า ก',
  vat_rate: '7',
  price_includes_vat: false,
  outstanding: '1070',
  lines: [
    {
      line_no: 1,
      item: { id: 'i1', code: 'A', name: 'สินค้า A' },
      quantity: '10',
      unit_price: '100',
      amounts: { net: '1000', vat: '70', gross: '1070' },
      returned_quantity: '4',
      credited_amount: '400',
    },
  ],
}

const posts = (fetch: ReturnType<typeof vi.fn>) => (fetch.mock.calls as unknown as [string, RequestInit][]).filter(([, i]) => i?.method === 'POST')

describe('CreditNoteForm', () => {
  it('รับคืน: จำนวนเกินที่คืนได้ (10 − 4) แจ้งที่ช่องและไม่ส่ง', async () => {
    const fetch = vi.fn(async () => json(200, { data: [] }))
    vi.stubGlobal('fetch', fetch)
    render(<CreditNoteForm mode="sales" invoice={invoice} apiPath="/cn" warehousesApi="/w" docsPath="/docs" invoiceHref="/inv" />)
    await userEvent.type(screen.getByLabelText(`${f.returnQty} A`), '7')
    await userEvent.type(screen.getByLabelText(new RegExp(f.reason)), 'สินค้าชำรุด')
    await userEvent.click(screen.getByRole('button', { name: t.actions.post }))
    expect(screen.getByText(f.overRemaining)).toBeInTheDocument()
    expect(posts(fetch)).toHaveLength(0)
  })

  it('ลดราคา: ส่ง kind=allowance + amount ต่อบรรทัด แล้วไปหน้าใบลดหนี้', async () => {
    const fetch = vi.fn(async () => json(201, { data: { id: 'cn1', document_no: 'CN2026-000001' } }))
    vi.stubGlobal('fetch', fetch)
    render(<CreditNoteForm mode="sales" invoice={invoice} apiPath="/cn" warehousesApi="/w" docsPath="/docs" invoiceHref="/inv" />)
    await userEvent.click(screen.getByRole('radio', { name: new RegExp(t.kinds.sales.allowance) }))
    await userEvent.type(screen.getByLabelText(`${f.allowanceExcl} A`), '100')
    await userEvent.type(screen.getByLabelText(new RegExp(f.reason)), 'คำนวณราคาผิด')
    expect(screen.getByText('107.00')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: t.actions.post }))
    const [call] = posts(fetch)
    expect(call![0]).toContain('/cn')
    expect(JSON.parse(String(call![1].body))).toMatchObject({ invoice_id: 'inv1', kind: 'allowance', reason: 'คำนวณราคาผิด', warehouse_id: null, lines: [{ invoice_line_no: 1, amount: '100' }] })
    expect(push).toHaveBeenCalledWith('/docs/cn1')
  })

  it('ลดราคาเกินมูลค่าที่ลดได้ (1,000 − 400) ไม่ส่ง', async () => {
    const fetch = vi.fn(async () => json(200, { data: [] }))
    vi.stubGlobal('fetch', fetch)
    render(<CreditNoteForm mode="sales" invoice={invoice} apiPath="/cn" warehousesApi="/w" docsPath="/docs" invoiceHref="/inv" />)
    await userEvent.click(screen.getByRole('radio', { name: new RegExp(t.kinds.sales.allowance) }))
    await userEvent.type(screen.getByLabelText(`${f.allowanceExcl} A`), '700')
    await userEvent.type(screen.getByLabelText(new RegExp(f.reason)), 'ลดราคา')
    await userEvent.click(screen.getByRole('button', { name: t.actions.post }))
    expect(screen.getByText(f.overValue)).toBeInTheDocument()
    expect(posts(fetch)).toHaveLength(0)
  })

  it('จัดซื้อ: ต้องมีเลขที่ใบลดหนี้ของผู้ขาย และส่ง supplier_credit_note_no / credit_note_date', async () => {
    const fetch = vi.fn(async () => json(201, { data: { id: 'pcn1', document_no: 'PCN2026-000001' } }))
    vi.stubGlobal('fetch', fetch)
    render(<CreditNoteForm mode="purchasing" invoice={invoice} apiPath="/scn" warehousesApi="/w" docsPath="/docs" invoiceHref="/inv" />)
    await userEvent.type(screen.getByLabelText(`${f.returnQty} A`), '2')
    await userEvent.type(screen.getByLabelText(new RegExp(f.reason)), 'ส่งคืน')
    await userEvent.click(screen.getByRole('button', { name: t.actions.post }))
    expect(posts(fetch)).toHaveLength(0)
    await userEvent.type(screen.getByLabelText(new RegExp(f.supplierNo)), 'SCN-1')
    await userEvent.click(screen.getByRole('button', { name: t.actions.post }))
    const body = JSON.parse(String(posts(fetch)[0]![1].body))
    expect(body).toMatchObject({ kind: 'return', supplier_credit_note_no: 'SCN-1', lines: [{ invoice_line_no: 1, quantity: '2', unit_price: '100' }] })
    expect(body.credit_note_date).toBe(body.date)
  })
})
