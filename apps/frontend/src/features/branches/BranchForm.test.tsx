import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { BranchForm } from './BranchForm'
import type { Branch } from './types'

const push = vi.fn()
const refresh = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, refresh, replace: vi.fn() }) }))

const t = texts.branches.form
const CID = '11111111-1111-4111-8111-111111111111'
const LIST = `/c/${CID}/organization/branches?sort=-code`

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

const headOffice: Branch = {
  id: 'b1',
  code: 'HQ',
  name: 'สำนักงานใหญ่',
  tax_branch_no: '00000',
  is_head_office: true,
  phone: '',
  status: 'active',
  address: { line: '', sub_district: '', district: '', province: 'กรุงเทพมหานคร', postal_code: '10110', country: 'TH' },
}

afterEach(() => {
  push.mockReset()
  refresh.mockReset()
})

describe('BranchForm', () => {
  it('ตรวจข้อมูลก่อนส่ง และไม่เรียก API', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch')
    const user = userEvent.setup()
    render(<BranchForm companyId={CID} canManage listHref={LIST} />)
    await user.type(screen.getByLabelText(t.taxBranchNo), '12')
    await user.type(screen.getByLabelText(t.postalCode), '123')
    await user.click(screen.getByRole('button', { name: texts.common.save }))

    expect(screen.getByText(texts.validation.required.replace('{field}', t.code))).toBeInTheDocument()
    expect(screen.getByText(texts.validation.taxBranchNo)).toBeInTheDocument()
    expect(screen.getByText(texts.validation.postalCode)).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalled()
    // โฟกัสย้ายไปช่องแรกที่ผิด และช่องบังคับบอก screen reader ด้วย aria-required
    await waitFor(() => expect(screen.getByLabelText(t.code)).toHaveFocus())
    expect(screen.getByLabelText(t.code)).toHaveAttribute('aria-required', 'true')
    expect(screen.getByLabelText(t.phone)).not.toHaveAttribute('aria-required')
  })

  it('สร้างสาขา: รหัสเป็นตัวพิมพ์ใหญ่, POST แล้วกลับหน้ารายการเดิม', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(201, { data: { id: 'new' } }))
    const user = userEvent.setup()
    render(<BranchForm companyId={CID} canManage listHref={LIST} />)
    await user.type(screen.getByLabelText(t.code), 'cnx 01')
    await user.type(screen.getByLabelText(t.name), ' สาขาเชียงใหม่ ')
    await user.type(screen.getByLabelText(t.taxBranchNo), '0000a1')
    await user.click(screen.getByRole('button', { name: texts.common.save }))

    expect(fetch).toHaveBeenCalledWith(`/api/v1/companies/${CID}/branches`, expect.objectContaining({ method: 'POST' }))
    const body = JSON.parse(fetch.mock.calls[0]![1]!.body as string)
    expect(body).toMatchObject({ code: 'CNX01', name: 'สาขาเชียงใหม่', tax_branch_no: '00001', status: 'active' })
    expect(body.address.country).toBe('TH')
    expect(push).toHaveBeenCalledWith(LIST)
  })

  it('สำนักงานใหญ่: แก้เลขที่สาขา / สถานะไม่ได้ และไม่มีปุ่มลบ; บันทึกด้วย PUT', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json(200, { data: headOffice }))
    const user = userEvent.setup()
    render(<BranchForm companyId={CID} branch={headOffice} canManage listHref={LIST} />)
    expect(screen.getByLabelText(t.taxBranchNo)).toBeDisabled()
    expect(screen.getByLabelText(t.status)).toBeDisabled()
    expect(screen.queryByRole('button', { name: texts.common.delete })).toBeNull()

    await user.click(screen.getByRole('button', { name: texts.common.save }))
    expect(fetch).toHaveBeenCalledWith(`/api/v1/companies/${CID}/branches/b1`, expect.objectContaining({ method: 'PUT' }))
  })

  it('ลบสาขา (ยืนยันก่อน) — backend ปฏิเสธ = แสดงข้อความจาก config', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      json(409, { error: { code: 'record_is_referenced_by_other_records', message: 'record is referenced by other records' } }),
    )
    const user = userEvent.setup()
    render(<BranchForm companyId={CID} branch={{ ...headOffice, id: 'b2', code: 'CNX', tax_branch_no: '00001', is_head_office: false }} canManage listHref={LIST} />)
    await user.click(screen.getByRole('button', { name: texts.common.delete }))
    expect(await screen.findByRole('alert')).toHaveTextContent(texts.errors.record_is_referenced_by_other_records!)
    expect(push).not.toHaveBeenCalled()
  })

  it('ไม่มีสิทธิ์ manage = อ่านอย่างเดียว ไม่มีปุ่มบันทึก / ลบ', () => {
    render(<BranchForm companyId={CID} branch={{ ...headOffice, is_head_office: false }} canManage={false} listHref={LIST} />)
    expect(screen.getByText(texts.common.readOnly)).toBeInTheDocument()
    expect(screen.getByLabelText(t.name)).toBeDisabled()
    expect(screen.queryByRole('button', { name: texts.common.save })).toBeNull()
    expect(screen.queryByRole('button', { name: texts.common.delete })).toBeNull()
    expect(screen.getByRole('link', { name: texts.common.back })).toHaveAttribute('href', LIST)
  })
})
