import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { format, texts } from '@/config'
import { toUnits } from '@/features/accounting/money'
import { MoneyDocForm, previewWht } from './MoneyDocForm'

const push = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh: vi.fn() }) }))

afterEach(() => {
  push.mockReset()
  vi.unstubAllGlobals()
})

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

describe('previewWht', () => {
  it('สูตรเดียวกับ backend: ฐานตามสัดส่วนก่อน VAT, จ่ายครบ = ยอดก่อน VAT', () => {
    const inv = { net: toUnits('1000'), gross: toUnits('1070') }
    expect(previewWht(toUnits('1070'), toUnits('3'), inv)).toBe(toUnits('30'))
    expect(previewWht(toUnits('535'), toUnits('3'), inv)).toBe(toUnits('15'))
    expect(previewWht(toUnits('2500'), toUnits('3'))).toBe(toUnits('75'))
  })
})

const api = {
  submit: '/companies/c1/payments',
  open: '/companies/c1/payments/open-invoices',
  parties: '/companies/c1/suppliers',
  accounts: '/companies/c1/accounts',
  cashAccounts: '/companies/c1/cash-accounts',
  whtTypes: '/companies/c1/payments/wht-types',
}
const t = texts.finance

describe('MoneyDocForm (จ่ายเงินอื่น)', () => {
  it('คำนวณภาษีหัก ณ ที่จ่าย ยอดเงินจริง และบอกผลต่างของช่องทางทั้งไอคอนและข้อความ', async () => {
    const fetch = vi.fn(async (url: string) => {
      if (url.includes('wht-types')) return json(200, { data: [{ id: 'w3', code: 'SVC', name: 'ค่าบริการ', rate: '3', status: 'active' }] })
      return json(200, { data: [] })
    })
    vi.stubGlobal('fetch', fetch)
    render(<MoneyDocForm mode="payment" api={api} docsPath="/c/c1/finance/payments" listHref="/c/c1/finance/payments" />)
    await userEvent.click(screen.getByRole('radio', { name: new RegExp(t.kinds.payment.other!) }))
    await userEvent.type(screen.getByLabelText(`${t.fields.amount} ${format(t.lines.line, { n: 1 })}`), '2500')
    await screen.findByRole('option', { name: 'ค่าบริการ 3%' })
    await userEvent.selectOptions(screen.getByLabelText(`${t.fields.whtType} ${format(t.lines.line, { n: 1 })}`), 'w3')
    const summary = screen.getByRole('complementary', { name: t.totals.net })
    expect(within(summary).getByText('(75.00)')).toBeInTheDocument()
    expect(within(summary).getByText('2,425.00')).toBeInTheDocument()
    expect(within(summary).getByText(format(t.totals.unbalanced, { amount: '2,425.00' }))).toBeInTheDocument()

    await userEvent.type(screen.getByLabelText(`${t.fields.amount} ${format(t.lines.method, { n: 1 })}`), '2425')
    expect(within(summary).getByText(t.totals.balanced)).toBeInTheDocument()
  })

  it('ไม่ส่งเมื่อข้อมูลไม่ครบ — ชื่อผู้รับ / บัญชี / ช่องทาง', async () => {
    const fetch = vi.fn(async () => json(200, { data: [] }))
    vi.stubGlobal('fetch', fetch)
    render(<MoneyDocForm mode="payment" api={api} docsPath="/p" listHref="/p" />)
    await userEvent.click(screen.getByRole('radio', { name: new RegExp(t.kinds.payment.other!) }))
    await userEvent.click(screen.getByRole('button', { name: t.actions.save }))
    expect(fetch).toHaveBeenCalledTimes(1) // โหลดประเภทภาษีเท่านั้น
    expect(screen.getByText(format(t.errors.nameRequired, { field: t.fields.payeeName }))).toBeInTheDocument()
    expect(screen.getAllByText(t.errors.accountRequired).length).toBeGreaterThan(0)
  })
})
