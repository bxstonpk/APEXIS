import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { texts } from '@/config'
import { builtinTemplate } from '@/features/print-templates/builtin'
import { renderTemplate, sanitize } from '@/features/print-templates/engine'
import type { Letterhead } from '@/features/sales/types'
import type { Payment } from './types'
import { whtCertificateData } from './WhtCertificate'

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

const svc = { id: 'w1', code: 'SVC', name: 'ค่าบริการ', income_type: '3_tres' as const }
const rent = { id: 'w2', code: 'RENT', name: 'ค่าเช่า', income_type: '3_tres' as const }
const fee = { id: 'w3', code: 'FEE', name: 'ค่านายหน้า', income_type: '40_2' as const }

const payment: Payment = {
  id: 'p1',
  document_no: 'PV2026-000002',
  date: '2026-11-26',
  kind: 'other',
  party: { id: null, code: '', name: 'บริษัท ผู้รับเงิน จำกัด', tax_id: '0105559000001', branch_no: '00000', address: 'กรุงเทพฯ', tax_type: 'juristic' },
  amount: '17000',
  wht: '660',
  net: '16340',
  status: 'posted',
  entry_id: null,
  entry_no: '',
  cancelled_at: null,
  description: '',
  created_by: 'u',
  methods: [],
  wht_certificate_no: 'WT2026-000001',
  wht_form_seq: 17,
  lines: [
    { invoice: null, account_id: 'a', description: '', amount: '10000', wht_type: svc, wht_rate: '3', wht_base: '10000', wht: '300' },
    { invoice: null, account_id: 'a', description: '', amount: '4000', wht_type: rent, wht_rate: '5', wht_base: '4000', wht: '200' },
    { invoice: null, account_id: 'a', description: '', amount: '3000', wht_type: fee, wht_rate: '3', wht_base: '3000', wht: '90' },
    { invoice: null, account_id: 'a', description: 'ไม่หัก', amount: '0', wht_type: null, wht_rate: '0', wht_base: '0', wht: '0' },
  ],
}

describe('WhtCertificate', () => {
  it('พิมพ์ 2 ฉบับ, ภ.ง.ด.53 สำหรับนิติบุคคล, รวมยอดตามประเภทเงินได้ และยอดรวม', () => {
    // แม่แบบมาตรฐาน finance.wht_certificate + ข้อมูลพิมพ์ (หน้าจริงแสดงใน iframe)
    const container = document.createElement('div')
    container.innerHTML = sanitize(renderTemplate(builtinTemplate('finance.wht_certificate').html, whtCertificateData(letterhead, payment)))
    document.body.appendChild(container)
    const c = texts.finance.whtCertificate
    const sheets = container.querySelectorAll('.sheet')
    expect(sheets).toHaveLength(2)
    const first = within(sheets[0] as HTMLElement)
    expect(first.getByText(c.copy1)).toBeInTheDocument()
    expect(first.getAllByText('WT2026-000001').length).toBeGreaterThan(0)
    // ลำดับที่ในใบแนบ ภ.ง.ด. (คำนวณที่ backend)
    expect(first.getByText('17')).toBeInTheDocument()
    // ช่องที่เลือก = ภ.ง.ด.53 (มีเครื่องหมาย ✓)
    const checked = [...(sheets[0] as HTMLElement).querySelectorAll('.check')].filter((el) => el.querySelector('.tick')?.textContent === '✓').map((el) => el.textContent)
    expect(checked.some((t) => t?.includes(c.forms.pnd53!))).toBe(true)
    expect(checked.some((t) => t?.includes(c.forms.pnd3!))).toBe(false)
    // 3 เตรส รวมค่าบริการ + ค่าเช่า = 14,000 / 500; 40(2) = 3,000 / 90
    const row3 = first.getByText(texts.finance.whtTypes.incomeTypes['3_tres']!).closest('tr')!
    expect(within(row3).getByText('14,000.00')).toBeInTheDocument()
    expect(within(row3).getByText('500.00')).toBeInTheDocument()
    expect(within(row3).getByText('(ค่าบริการ, ค่าเช่า)')).toBeInTheDocument()
    const row2 = first.getByText(texts.finance.whtTypes.incomeTypes['40_2']!).closest('tr')!
    expect(within(row2).getByText('3,000.00')).toBeInTheDocument()
    const total = first.getByText(c.total).closest('tr')!
    expect(within(total).getByText('17,000.00')).toBeInTheDocument()
    expect(within(total).getByText('590.00')).toBeInTheDocument()
    expect(screen.getAllByText('(ห้าร้อยเก้าสิบบาทถ้วน)')).toHaveLength(2)
  })
})
