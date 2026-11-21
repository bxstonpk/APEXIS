import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { CompanySettingsForm, type Company } from './CompanySettingsForm'
import { CostCenterForm, DepartmentForm, ProjectSiteForm } from './MasterDataForms'
import { NumberingTable } from './NumberingTable'
import { PeriodTable } from './PeriodTable'
import type { FiscalPeriod } from './resources'

const push = vi.fn()
const refresh = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, refresh, replace: vi.fn() }) }))

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}
const bodyOf = (fetch: { mock: { calls: unknown[][] } }, i = 0) => JSON.parse((fetch.mock.calls[i]![1] as RequestInit).body as string)

afterEach(() => {
  push.mockReset()
  refresh.mockReset()
})

const C = 'c1'
const ref = (id: string, code: string) => ({ id, code, name: code })

describe('DepartmentForm', () => {
  it('แก้ไข: PUT พร้อม id ของข้อมูลอ้างอิง (ล้างค่า = null) แล้วกลับหน้ารายการ', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(200, { data: {} }))
    const user = userEvent.setup()
    render(
      <DepartmentForm
        companyId={C}
        apiPath={`/companies/${C}/departments`}
        canManage
        listHref="/list?q=a"
        item={{ id: 'd1', code: 'ACC', name: 'บัญชี', parent: null, business_unit: ref('u1', 'RETAIL'), branch: ref('b1', 'HQ'), status: 'active' }}
      />,
    )
    await user.click(screen.getByRole('button', { name: `ล้างค่า${texts.departments.fields.branch}` }))
    await user.click(screen.getByRole('button', { name: texts.common.save }))

    expect(fetch).toHaveBeenCalledWith(`/api/v1/companies/${C}/departments/d1`, expect.objectContaining({ method: 'PUT' }))
    expect(bodyOf(fetch)).toEqual({ code: 'ACC', name: 'บัญชี', parent_id: null, business_unit_id: 'u1', branch_id: null, status: 'active' })
    expect(push).toHaveBeenCalledWith('/list?q=a')
  })
})

describe('CostCenterForm', () => {
  it('วันสิ้นสุดก่อนวันเริ่ม = ไม่ส่ง; ถูกต้อง = POST valid_to null เมื่อเว้นว่าง', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(201, { data: {} }))
    const user = userEvent.setup()
    render(<CostCenterForm companyId={C} apiPath="/companies/c1/cost-centers" canManage listHref="/list" defaultValidFrom="2026-01-01" />)
    const f = texts.costCenters.fields
    await user.type(screen.getByLabelText(f.code), 'cc-1')
    await user.type(screen.getByLabelText(f.name), 'ต้นทุน')
    await user.type(screen.getByLabelText(f.validTo), '2025-12-31')
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(screen.getByText(texts.validation.dateRange)).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalled()

    await user.clear(screen.getByLabelText(f.validTo))
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(bodyOf(fetch)).toEqual({ code: 'CC-1', name: 'ต้นทุน', department_id: null, valid_from: '2026-01-01', valid_to: null, status: 'active' })
  })
})

describe('ProjectSiteForm', () => {
  it('ปิดโครงการต้องมีวันสิ้นสุด; ไม่มีสิทธิ์ = อ่านอย่างเดียว', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch')
    const user = userEvent.setup()
    const f = texts.projectSites.fields
    const { unmount } = render(<ProjectSiteForm companyId={C} apiPath="/x" canManage listHref="/list" defaultStartDate="2026-01-01" />)
    await user.type(screen.getByLabelText(f.code), 'P1')
    await user.type(screen.getByLabelText(f.name), 'ไซต์')
    await user.selectOptions(screen.getByLabelText(f.status), 'closed')
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(screen.getByText(texts.validation.closedEndDate)).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalled()
    unmount()

    render(<ProjectSiteForm companyId={C} apiPath="/x" canManage={false} listHref="/list" defaultStartDate="2026-01-01" />)
    expect(screen.getByLabelText(f.name)).toBeDisabled()
    expect(screen.queryByRole('button', { name: texts.common.save })).toBeNull()
  })
})

describe('PeriodTable', () => {
  const periods: FiscalPeriod[] = [1, 2, 3].map((n) => ({
    id: `p${n}`,
    fiscal_year_id: 'fy',
    fiscal_year: 2026,
    period_no: n,
    start_date: `2026-0${n}-01`,
    end_date: `2026-0${n}-28`,
    status: n === 1 ? 'closed' : 'open',
  }))
  const t = texts.fiscalYears.periods

  it('ปุ่มเฉพาะงวดที่ทำได้: ปิด = งวดแรกที่เปิด, เปิด = งวดล่าสุดที่ปิด; ยืนยันแล้ว PATCH', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(200, { data: {} }))
    const user = userEvent.setup()
    render(<PeriodTable apiPath="/companies/c1/fiscal-periods" periods={periods} canClose />)
    const rows = screen.getAllByRole('row').slice(1)
    expect(within(rows[0]!).getByRole('button', { name: t.reopen })).toBeInTheDocument()
    expect(within(rows[1]!).getByRole('button', { name: t.close })).toBeInTheDocument()
    expect(within(rows[2]!).queryByRole('button')).toBeNull()

    await user.click(within(rows[1]!).getByRole('button', { name: t.close }))
    expect(fetch).toHaveBeenCalledWith('/api/v1/companies/c1/fiscal-periods/p2', expect.objectContaining({ method: 'PATCH' }))
    expect(bodyOf(fetch)).toEqual({ status: 'closed' })
    expect(refresh).toHaveBeenCalled()
  })

  it('แถบความคืบหน้าบอกจำนวนงวดที่ปิด (ข้อความ + progressbar)', () => {
    render(<PeriodTable apiPath="/x" periods={periods} canClose={false} />)
    const bar = screen.getByRole('progressbar')
    expect(bar).toHaveAttribute('aria-valuenow', '1')
    expect(bar).toHaveAttribute('aria-valuemax', '3')
    expect(screen.getByText('ปิดงวดแล้ว 1 จาก 3 งวด')).toBeInTheDocument()
  })

  it('ไม่มีสิทธิ์ปิดงวด = ไม่มีปุ่ม', () => {
    render(<PeriodTable apiPath="/x" periods={periods} canClose={false} />)
    expect(screen.queryByRole('button')).toBeNull()
  })
})

describe('NumberingTable', () => {
  const item = { doc_type: 'purchasing.payment', name: 'ใบจ่ายชำระ', default_prefix: 'PV', prefix: 'PV', template: '{prefix}{fy}-{seq:6}', example: 'PV2026-000001', custom: false }

  it('แก้แถว: ตัวอย่างเปลี่ยนตามที่พิมพ์, รูปแบบผิด = บันทึกไม่ได้, ถูก = PUT', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(200, { data: item }))
    const user = userEvent.setup()
    render(<NumberingTable apiPath="/companies/c1/document-numberings" items={[item]} canManage />)
    await user.click(screen.getByRole('button', { name: texts.numbering.edit }))
    const template = screen.getByLabelText(texts.numbering.columns.template)

    await user.clear(template)
    await user.type(template, '{{prefix}{{fy}')
    expect(screen.getByText(texts.numbering.invalid.seqOnce)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: texts.common.save })).toBeDisabled()

    await user.type(template, '-{{seq:4}')
    const yy = String(new Date().getFullYear())
    expect(screen.getByText(`PV${yy}-0001`)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(fetch).toHaveBeenCalledWith('/api/v1/companies/c1/document-numberings/purchasing.payment', expect.objectContaining({ method: 'PUT' }))
    expect(bodyOf(fetch)).toEqual({ prefix: 'PV', template: '{prefix}{fy}-{seq:4}' })
  })

  it('ไม่มีสิทธิ์ = ไม่มีปุ่มแก้ไข', () => {
    render(<NumberingTable apiPath="/x" items={[item]} canManage={false} />)
    expect(screen.queryByRole('button', { name: texts.numbering.edit })).toBeNull()
  })
})

describe('CompanySettingsForm', () => {
  const company: Company = {
    id: 'c1',
    code: 'DEMO',
    name: 'เดโม',
    tax_id: '',
    legal_name_th: '',
    legal_name_en: '',
    address: { line: '', sub_district: '', district: '', province: '', postal_code: '', country: 'TH' },
    phone: '',
    email: '',
    website: '',
    base_currency: 'THB',
    fiscal_year_start_month: 1,
    amount_decimals: 2,
    quantity_decimals: 3,
    status: 'active',
  }

  it('ทศนิยมเลือกได้ตั้งแต่ค่าปัจจุบัน (ลดไม่ได้); PUT ไม่ส่ง id / code; บันทึกแล้วอยู่หน้าเดิม', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(200, { data: company }))
    const user = userEvent.setup()
    render(<CompanySettingsForm company={company} canManage />)
    const f = texts.company.fields
    const qty = screen.getByLabelText(f.quantityDecimals)
    expect(within(qty).getAllByRole('option').map((o) => o.textContent)).toEqual(['3', '4', '5', '6'])
    // ยังไม่แก้ = บันทึกไม่ได้
    expect(screen.getByRole('button', { name: texts.common.save })).toBeDisabled()

    await user.selectOptions(screen.getByLabelText(f.fiscalYearStartMonth), '10')
    expect(screen.getByText(texts.company.dirty)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    const body = bodyOf(fetch)
    expect(body).not.toHaveProperty('id')
    expect(body).not.toHaveProperty('code')
    expect(body.fiscal_year_start_month).toBe(10)
    expect(await screen.findByText(texts.company.savedAt)).toBeInTheDocument()
    expect(push).not.toHaveBeenCalled()
  })

  it('ปิดใช้งานบริษัท / เพิ่มทศนิยม ต้องยืนยันก่อน — ไม่ยืนยัน = ไม่บันทึก', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const fetch = vi.spyOn(globalThis, 'fetch')
    const user = userEvent.setup()
    render(<CompanySettingsForm company={company} canManage />)
    await user.selectOptions(screen.getByLabelText(texts.common.status), 'inactive')
    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(confirm).toHaveBeenCalledWith(texts.company.confirm.inactive)
    expect(fetch).not.toHaveBeenCalled()
  })

  it('ยกเลิกการแก้ไข = กลับเป็นค่าที่บันทึกไว้', async () => {
    const user = userEvent.setup()
    render(<CompanySettingsForm company={company} canManage />)
    const name = screen.getByLabelText(texts.company.fields.name)
    await user.clear(name)
    await user.type(name, 'ชื่อใหม่')
    await user.click(screen.getByRole('button', { name: texts.company.discard }))
    expect(name).toHaveValue('เดโม')
    expect(screen.queryByText(texts.company.dirty)).toBeNull()
  })
})
