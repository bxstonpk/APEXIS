import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { apps } from '@/config/apps'
import { texts } from '@/config'
import { buildAdjustment, monthEnd, templateAccounts, type AdjustmentState } from './AdjustmentForm'
import { DBDTable } from './DBDTable'
import { reportCsvRows } from './reportCsv'
import type { DBDStatement } from './types'

const ref = (id: string) => ({ id, code: id, name: id })
const base: AdjustmentState = {
  template: 'accrued_revenue',
  period: 'adj:2025',
  month: '',
  journal: ref('GEN'),
  debit: ref('1103020400'),
  credit: ref('4101000000'),
  amount: '10,000',
  description: ' งานบริการค้างเรียกเก็บ ',
  reference: '',
  autoReverse: true,
}
const years = [{ fiscal_year: 2025, end_date: '2025-12-31' }]

describe('รายการปรับปรุง', () => {
  it('วันสิ้นเดือน', () => {
    expect(monthEnd('2026-02')).toBe('2026-02-28')
    expect(monthEnd('2024-02')).toBe('2024-02-29')
    expect(monthEnd('')).toBe('')
  })

  it('งวดปรับปรุง: ลงวันสิ้นปี + adjusting_period + บรรทัดเดบิต / เครดิต', () => {
    const { errors, body } = buildAdjustment(base, years)
    expect(errors).toEqual({})
    expect(body).toMatchObject({
      date: '2025-12-31',
      adjusting_period: true,
      template: 'accrued_revenue',
      auto_reverse: true,
      description: 'งานบริการค้างเรียกเก็บ',
      lines: [
        { account_id: '1103020400', debit: '10000', credit: '0' },
        { account_id: '4101000000', debit: '0', credit: '10000' },
      ],
    })
  })

  it('สิ้นเดือน: วันสุดท้ายของเดือนที่เลือก ไม่ใช่งวดปรับปรุง', () => {
    const { body } = buildAdjustment({ ...base, period: 'month', month: '2026-03' }, years)
    expect(body).toMatchObject({ date: '2026-03-31', adjusting_period: false })
    expect(buildAdjustment({ ...base, period: 'month', month: '' }, years).errors.month).toBe(texts.adjustments.errors.monthRequired)
  })

  it('ตรวจบัญชีซ้ำ / จำนวนเงิน / บัญชีว่าง', () => {
    const e = texts.adjustments.errors
    expect(buildAdjustment({ ...base, credit: ref('1103020400') }, years).errors.credit).toBe(e.sameAccount)
    expect(buildAdjustment({ ...base, amount: '0' }, years).errors.amount).toBe(e.amountInvalid)
    expect(buildAdjustment({ ...base, debit: null }, years).errors.debit).toBe(e.accountRequired)
  })

  it('บัญชีตั้งต้นของแบบ: ด้านงบแสดงฐานะเป็นเดบิต / เครดิตตามแบบ', () => {
    const t = { template: 'x', balance_account: ref('B'), result_account: ref('R'), auto_reverse: true }
    expect(templateAccounts({ ...t, balance_debit: true })).toEqual([ref('B'), ref('R')])
    expect(templateAccounts({ ...t, balance_debit: false })).toEqual([ref('R'), ref('B')])
  })
})

const dbd: DBDStatement = {
  book: 'dbd-income-statement',
  columns: [
    { start_date: '2026-01-01', end_date: '2026-11-30', fiscal_year: 2026 },
    { start_date: '2025-01-01', end_date: '2025-11-30', fiscal_year: 2025 },
  ],
  rows: [
    { key: 'revenue', kind: 'header', level: 0, amounts: [] },
    { key: 'revenue_services', kind: 'line', level: 1, amounts: ['60000', '0'] },
    { key: 'net_profit', kind: 'total', level: 0, amounts: ['-1500.5', '0'] },
  ],
}

describe('งบตามแบบกรมพัฒนาธุรกิจการค้า', () => {
  it('หัวคอลัมน์เป็นปี พ.ศ., ชื่อบรรทัดจาก config, ขาดทุนในวงเล็บ, หัวข้อไม่มียอด', () => {
    render(<DBDTable r={dbd} />)
    expect(screen.getByText('ปี 2569')).toBeInTheDocument()
    expect(screen.getByText('ปี 2568')).toBeInTheDocument()
    const row = screen.getByText(texts.reports.dbd.lines.revenue_services!).closest('tr')!
    expect(within(row).getByText('60,000.00')).toBeInTheDocument()
    const total = screen.getByText(texts.reports.dbd.lines.net_profit!).closest('tr')!
    expect(within(total).getByText('(1,500.50)')).toBeInTheDocument()
    expect(screen.getByText(texts.reports.dbd.lines.revenue!).closest('tr')!.querySelectorAll('td')[0]!.textContent).toBe('')
  })

  it('CSV: ปี พ.ศ. + ตัวเลขล้วน', () => {
    const rows = reportCsvRows(dbd, 'ACME')
    expect(rows).toContainEqual([texts.reports.dbd.unit, '2569', '2568'])
    expect(rows).toContainEqual([`  ${texts.reports.dbd.lines.revenue_services}`, '60000.00', '0.00'])
  })

  it('เมนูแอปบัญชีมีรายการปรับปรุงและปิดบัญชีสิ้นปี', () => {
    const acc = apps.find((a) => a.key === 'accounting')!
    expect(acc.menu.map((m) => m.path)).toEqual(['dashboard', 'entries', 'adjustments', 'accounts', 'journals', 'reports', 'year-end'])
  })
})
