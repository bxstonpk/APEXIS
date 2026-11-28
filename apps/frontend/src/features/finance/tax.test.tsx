import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { findApp } from '@/config/apps'
import type { Letterhead } from '@/features/sales/types'
import { monthLabel, paginate, type Pp30, type TaxOverview, type VatReport, type WhtForm } from './tax'
import { TaxFilingPanel, validateFiling } from './TaxFilingPanel'
import { Pp30Print, VatReportPrint, WhtFormPrint } from './TaxForms'
import { TaxOverviewTable } from './TaxOverviewTable'

const refresh = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh }) }))

afterEach(() => {
  refresh.mockReset()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const t = texts.finance.taxFilings
const letterhead: Letterhead = {
  name: 'Demo',
  legal_name_th: 'บริษัท เดโม จำกัด',
  legal_name_en: '',
  tax_id: '0105555000001',
  address: { line: '1 ถนนสีลม', sub_district: 'สีลม', district: 'บางรัก', province: 'กรุงเทพมหานคร', postal_code: '10500', country: 'TH' },
  phone: '',
  email: '',
  website: '',
}
const cell = (amount: string, extra: Partial<TaxOverview['months'][number]['pnd3']> = {}) => ({ amount, payees: 0, computed: true, filing: null, ...extra })
const filing = { id: 'f1', form: 'pnd53' as const, month: '2026-10', filed_date: '2026-11-07', amount: '300', surcharge: '0', penalty: '0', carry_forward: '0',
  total: '300', reference: 'RD-1', note: '', payment_id: null, status: 'filed' as const, created_at: '', cancelled_at: null }

describe('tax helpers', () => {
  it('monthLabel / paginate', () => {
    expect(monthLabel('2026-11')).toBe('พฤศจิกายน 2569')
    expect(paginate([1, 2, 3, 4, 5], 2)).toEqual([[1, 2], [3, 4], [5]])
    expect(paginate([], 2)).toEqual([[]])
  })
  it('เมนูการเงินมี "ยื่นภาษี" ต่อจากรายงานภาษี', () => {
    const paths = findApp('finance')!.menu.map((m) => m.path)
    expect(paths.indexOf('tax-filings')).toBe(paths.indexOf('reports') + 1)
    expect(texts.apps.finance.menu['tax-filings']).toBe('ยื่นภาษี')
  })
})

describe('TaxOverviewTable', () => {
  it('ยอดต่อเดือน / สถานะการยื่น / ชำระเกิน / เดือนที่ยังไม่ถึง และลิงก์ไปแบบของเดือน', () => {
    const data: TaxOverview = {
      year: 2026,
      can_see_filings: true,
      months: [
        { month: '2026-10', pnd3: cell('0'), pnd53: cell('300', { payees: 1, filing }), pp30: cell('700') },
        { month: '2026-11', pnd3: cell('60', { payees: 1 }), pnd53: cell('320', { payees: 2 }), pp30: cell('-70') },
        { month: '2026-12', pnd3: cell('0', { computed: false }), pnd53: cell('0', { computed: false }), pp30: cell('0', { computed: false }) },
      ],
    }
    render(<TaxOverviewTable companyId="c1" data={data} />)
    const oct = screen.getByRole('link', { name: `${t.forms.pnd53} เดือนตุลาคม 2569` })
    expect(oct).toHaveAttribute('href', '/c/c1/finance/tax-filings/pnd53/2026-10')
    expect(within(oct).getByText('300.00')).toBeInTheDocument()
    expect(within(oct).getByText(/ยื่นแล้ว/)).toBeInTheDocument()
    const nov = screen.getByRole('link', { name: `${t.forms.pp30} เดือนพฤศจิกายน 2569` })
    expect(within(nov).getByText('ชำระเกิน 70.00')).toBeInTheDocument()
    expect(within(nov).getByText(t.status.notFiled)).toBeInTheDocument()
    expect(screen.getAllByText('–').length).toBeGreaterThanOrEqual(3)
  })

  it('ไม่มีสิทธิ์ดูทะเบียน = ไม่แสดงสถานะการยื่น', () => {
    render(<TaxOverviewTable companyId="c1" data={{ year: 2026, can_see_filings: false, months: [{ month: '2026-11', pnd3: cell('60'), pnd53: cell('0'), pp30: cell('0') }] }} />)
    expect(screen.queryByText(t.status.notFiled)).not.toBeInTheDocument()
  })
})

describe('การยื่นแบบ', () => {
  const blank = { filed_date: '2026-12-07', amount: '320', surcharge: '', penalty: '', carry_forward: '70', reference: ' RD-9 ' }
  it('validateFiling: วันที่บังคับ, ห้ามติดลบ, ภ.พ.30 ส่งชำระเกินยกไป, ภ.ง.ด. ไม่ส่ง', () => {
    expect(validateFiling('pnd53', '2026-11', { ...blank, filed_date: '' }).errors.filed_date).toBeTruthy()
    expect(validateFiling('pnd53', '2026-11', { ...blank, amount: '-1' }).errors.amount).toBe(t.filing.amountInvalid)
    expect(validateFiling('pnd53', '2026-11', { ...blank, amount: '' }).errors.amount).toBeTruthy()
    expect(validateFiling('pnd53', '2026-11', blank).body).toEqual({ form: 'pnd53', month: '2026-11', filed_date: '2026-12-07', reference: 'RD-9', amount: '320', surcharge: '0', penalty: '0' })
    expect(validateFiling('pp30', '2026-11', { ...blank, amount: '0' }).body).toMatchObject({ form: 'pp30', amount: '0', carry_forward: '70' })
  })

  it('บันทึกการยื่น: POST แล้ว refresh', async () => {
    const fetch = vi.fn(async () => new Response(JSON.stringify({ data: filing }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetch)
    render(<TaxFilingPanel apiPath="/tf" form="pnd53" month="2026-11" filing={null} suggestedAmount="320" canManage />)
    expect(screen.getByLabelText(new RegExp(t.filing.amount))).toHaveValue('320')
    expect(screen.queryByLabelText(t.filing.carryForward)).not.toBeInTheDocument()
    await userEvent.type(screen.getByLabelText(t.filing.reference), 'RD-1')
    await userEvent.click(screen.getByRole('button', { name: t.actions.record }))
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toContain('/tf')
    expect(JSON.parse(String(init.body))).toMatchObject({ form: 'pnd53', month: '2026-11', amount: '320', reference: 'RD-1' })
    expect(refresh).toHaveBeenCalled()
  })

  it('ยื่นแล้ว: แสดงวันที่ / เลขอ้างอิง และปุ่มยกเลิกเฉพาะผู้มีสิทธิ์', () => {
    const { rerender } = render(<TaxFilingPanel apiPath="/tf" form="pnd53" month="2026-10" filing={filing} suggestedAmount="0" canManage />)
    expect(screen.getByText('RD-1')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: t.actions.cancel })).toBeInTheDocument()
    rerender(<TaxFilingPanel apiPath="/tf" form="pnd53" month="2026-10" filing={filing} suggestedAmount="0" canManage={false} />)
    expect(screen.queryByRole('button', { name: t.actions.cancel })).not.toBeInTheDocument()
  })
})

describe('พิมพ์', () => {
  const line = (seq: number, base: string, wht: string) => ({ seq, payment_id: `p${seq}`, document_no: `PV-${seq}`, certificate_no: '', date: '2026-11-03',
    tax_id: '0105550000004', branch_no: '00000', name: `ราย ${seq}`, address: 'กรุงเทพฯ', income_type: '3_tres', wht_type_name: 'ค่าบริการ', rate: '3', base, wht, condition: 1 })

  it('ใบแนบ ภ.ง.ด.53: หัวกระดาษ / เดือน / จำนวนแผ่น / รวมทั้งสิ้น', () => {
    const lines = Array.from({ length: 13 }, (_, i) => line(i + 1, '1000', '30'))
    const data: WhtForm = { form: 'pnd53', month: '2026-11', summary: { payees: 13, lines: 13, base: '13000', wht: '390' }, lines, filing: null }
    const { container } = render(<WhtFormPrint letterhead={letterhead} data={data} />)
    expect(container.querySelectorAll('article.a4-landscape')).toHaveLength(2)
    expect(screen.getAllByText(`ใบแนบ ${t.forms.pnd53}`)).toHaveLength(2)
    expect(screen.getAllByText('เดือนภาษี พฤศจิกายน 2569')).toHaveLength(2)
    expect(screen.getByText('แผ่นที่ 2 / 2')).toBeInTheDocument()
    expect(screen.getAllByText('บริษัท เดโม จำกัด')).toHaveLength(2)
    expect(screen.getByText(/รวมทั้งสิ้น/)).toBeInTheDocument()
    expect(screen.getByText('390.00')).toBeInTheDocument()
  })

  it('ภ.พ.30 และรายงานภาษีซื้อ', () => {
    const pp30: Pp30 = { month: '2026-10', sales: { base: '10000', tax: '700', documents: 1 }, zero_rated: '0', exempt: '0', purchases: { base: '0', tax: '0', documents: 0 },
      carry_in: '0', net: '700', payable: '700', overpaid: '0', prev_filed: false, filing: null }
    render(<Pp30Print letterhead={letterhead} data={pp30} />)
    expect(screen.getByText(t.formNames.pp30!)).toBeInTheDocument()
    expect(screen.getByText(/ต้องชำระ 700.00/)).toBeInTheDocument()
    const rep: VatReport = { kind: 'input', month: '2026-11', rows: [{ seq: 1, date: '2026-11-10', document_no: 'TI-001', party_name: 'บริษัท เครื่องเขียน จำกัด', tax_id: '0105555000001', branch_no: '00000', base: '2000', tax: '140' }], base: '2000', tax: '140' }
    render(<VatReportPrint letterhead={letterhead} data={rep} />)
    expect(screen.getByText(t.vatReport.input)).toBeInTheDocument()
    expect(screen.getByText('TI-001')).toBeInTheDocument()
    expect(screen.getByText(t.vatReport.partyInput)).toBeInTheDocument()
  })
})
