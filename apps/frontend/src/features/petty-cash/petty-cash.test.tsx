import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { visibleApps } from '@/config/apps'
import { PcActions } from './Actions'
import { selectedTotal, voucherTotals, type VoucherRow } from './types'
import { validateVoucher, voucherBody, type VoucherValues } from './VoucherForm'

const refresh = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh }) }))

afterEach(() => {
  refresh.mockReset()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const t = texts.pettyCash.vouchers
const fund = { id: 'f1', code: 'PC-HO', name: 'เงินสดย่อยสำนักงานใหญ่' }
const account = { id: 'a1', code: '5309050000', name: 'ค่าเครื่องเขียน' }

const values = (p: Partial<VoucherValues> = {}): VoucherValues => ({
  fund,
  date: '2026-11-20',
  payee: 'ร้านเครื่องเขียน',
  taxId: '',
  branchNo: '',
  description: '',
  vat: false,
  vatRate: '7',
  includesVat: true,
  taxInvoiceNo: '',
  taxInvoiceDate: '',
  lines: [{ key: 1, account, description: 'กระดาษ', amount: '300' }],
  ...p,
})

describe('voucherTotals (สูตรเดียวกับ backend)', () => {
  it('ไม่มี VAT = ยอดรวมเท่ากับยอดก่อน VAT', () => {
    expect(voucherTotals(['100', '50.5'], '0', false)).toEqual({ net: '150.5', vat: '0', gross: '150.5' })
  })
  it('ราคารวม VAT: 300 → VAT 19.63, ก่อน VAT 280.37', () => {
    expect(voucherTotals(['100', '200'], '7', true)).toEqual({ net: '280.37', vat: '19.63', gross: '300' })
  })
  it('ราคาก่อน VAT: 2,000 → VAT 140, รวม 2,140', () => {
    expect(voucherTotals(['2000'], '7', false)).toEqual({ net: '2000', vat: '140', gross: '2140' })
  })
})

describe('validateVoucher', () => {
  it('ผ่านเมื่อไม่มี VAT และไม่ต้องมีใบกำกับภาษี', () => {
    expect(validateVoucher(values())).toEqual({})
  })
  it('มี VAT ต้องกรอกเลขที่และวันที่ใบกำกับภาษี', () => {
    const errs = validateVoucher(values({ vat: true }))
    expect(errs.taxInvoiceNo).toBe(t.errors.taxInvoiceRequired)
    expect(errs.taxInvoiceDate).toBe(t.errors.taxInvoiceRequired)
    expect(validateVoucher(values({ vat: true, taxInvoiceNo: 'IV-001', taxInvoiceDate: '2026-11-20' }))).toEqual({})
  })
  it('อัตรา VAT ต้องมากกว่า 0 และไม่เกิน 100', () => {
    expect(validateVoucher(values({ vat: true, vatRate: '0', taxInvoiceNo: 'X', taxInvoiceDate: '2026-11-20' })).vatRate).toBe(t.errors.vatRateInvalid)
    expect(validateVoucher(values({ vat: true, vatRate: '101', taxInvoiceNo: 'X', taxInvoiceDate: '2026-11-20' })).vatRate).toBe(t.errors.vatRateInvalid)
  })
  it('ต้องมีกองทุน ผู้รับเงิน บัญชี และจำนวนเงินมากกว่า 0', () => {
    const errs = validateVoucher(values({ fund: null, payee: ' ', lines: [{ key: 7, account: null, description: '', amount: '0' }] }))
    expect(errs).toMatchObject({ fund: t.errors.fundRequired, payee: t.errors.payeeRequired, 'acc-7': t.errors.accountRequired, 'amt-7': t.errors.amountInvalid })
    expect(validateVoucher(values({ lines: [] })).lines).toBe(t.errors.noLines)
  })
  it('ไม่มี VAT = ส่งอัตรา 0 และไม่ส่งใบกำกับภาษี', () => {
    const body = voucherBody(values({ taxInvoiceNo: 'ไม่ส่ง' }))
    expect(body).toMatchObject({ vat_rate: '0', price_includes_vat: false, tax_invoice: null })
    expect(voucherBody(values({ vat: true, taxInvoiceNo: ' IV-1 ', taxInvoiceDate: '2026-11-20' }))).toMatchObject({
      vat_rate: '7',
      price_includes_vat: true,
      tax_invoice: { no: 'IV-1', date: '2026-11-20' },
    })
  })
})

describe('selectedTotal (คำขอเบิกชดเชย)', () => {
  const row = (id: string, gross: string): VoucherRow => ({
    id,
    document_no: id,
    date: '2026-11-20',
    fund,
    payee_name: '',
    description: '',
    tax_invoice_no: '',
    totals: { net: gross, vat: '0', gross },
    status: 'approved',
    replenishment: null,
  })
  it('รวมเฉพาะใบที่เลือก', () => {
    const rows = [row('a', '1000'), row('b', '2140'), row('c', '0.5')]
    expect(selectedTotal(rows, new Set(['b', 'c']))).toBe('2140.5')
    expect(selectedTotal(rows, new Set())).toBe('0')
  })
})

describe('แอปเงินสดย่อย', () => {
  it('เป็นไอคอนของตัวเอง เห็นได้ทุกประเภทธุรกิจ', () => {
    for (const type of ['service', 'trading', 'manufacturing', 'construction'] as const) {
      expect(visibleApps(['*'], type).map((a) => a.key)).toContain('petty-cash')
    }
  })
  it('เห็นเฉพาะเมนูที่มีสิทธิ์อ่าน', () => {
    const app = visibleApps(['petty_cash.voucher:read', 'petty_cash.voucher:manage']).find((a) => a.key === 'petty-cash')
    expect(app?.menu.map((m) => m.path)).toEqual(['vouchers'])
    expect(visibleApps(['finance.payment:read']).map((a) => a.key)).not.toContain('petty-cash')
  })
})

describe('PcActions', () => {
  it('ยืนยันแล้ว POST ไปที่ action และ refresh', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: {} }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetch)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<PcActions apiPath="/api/x/v1" docNo="PCV-0001" actions={[{ path: 'approve', label: t.actions.approve, confirm: t.confirm.approve, primary: true }]} />)
    await userEvent.click(screen.getByRole('button', { name: t.actions.approve }))
    expect(window.confirm).toHaveBeenCalledWith('อนุมัติ PCV-0001?')
    const [url, init] = fetch.mock.calls[0] as [string, RequestInit]
    expect(url).toContain('/api/x/v1/approve')
    expect(init.method).toBe('POST')
    expect(refresh).toHaveBeenCalled()
  })
  it('ไม่ยืนยัน = ไม่เรียก API', async () => {
    const fetch = vi.fn()
    vi.stubGlobal('fetch', fetch)
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<PcActions apiPath="/x" docNo="PCV-1" actions={[{ path: 'cancel', label: t.actions.cancel, confirm: t.confirm.cancel, danger: true }]} />)
    await userEvent.click(screen.getByRole('button', { name: t.actions.cancel }))
    expect(fetch).not.toHaveBeenCalled()
  })
})
