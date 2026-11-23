import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { formatThaiAddress, type VatRegistrant } from './thaiAddress'
import { VatLookup } from './VatLookup'

afterEach(() => vi.unstubAllGlobals())

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

describe('formatThaiAddress (กฎเดียวกับ backend)', () => {
  it('ต่างจังหวัดใช้ ตำบล / อำเภอ / จังหวัด และตัด "-"', () => {
    expect(formatThaiAddress({ house_no: '72/7', moo: '3', soi: '-', sub_district: 'ท่าทราย', district: 'เมืองสมุทรสาคร', province: 'สมุทรสาคร', postal_code: '74000' })).toBe(
      'เลขที่ 72/7 หมู่ที่ 3 ตำบลท่าทราย อำเภอเมืองสมุทรสาคร จังหวัดสมุทรสาคร 74000',
    )
  })
  it('กรุงเทพฯ ใช้ แขวง / เขต และไม่เติมคำนำหน้าซ้ำ', () => {
    expect(
      formatThaiAddress({ building: 'อาคารสาทรซิตี้', floor_no: '12', house_no: '175', soi: 'ซอยสาทร 1', street: 'สาทรใต้', sub_district: 'ทุ่งมหาเมฆ', district: 'เขตสาทร', province: 'กรุงเทพมหานคร', postal_code: '10120' }),
    ).toBe('อาคารสาทรซิตี้ ชั้นที่ 12 เลขที่ 175 ซอยสาทร 1 ถนนสาทรใต้ แขวงทุ่งมหาเมฆ เขตสาทร กรุงเทพมหานคร 10120')
  })
})

const row = (branch_no: string, branch_name: string): VatRegistrant => ({
  tax_id: '0745569002816',
  branch_no,
  branch_name,
  name: 'บริษัท ตัวอย่าง จำกัด',
  tax_type: 'juristic',
  phone: '',
  vat_registered_on: '',
  address: `ที่อยู่ ${branch_no}`,
  address_detail: { building: '', room_no: '', floor_no: '', village: '', house_no: '1', moo: '', soi: '', yaek: '', street: '', sub_district: '', district: '', province: '', postal_code: '' },
})

function Harness({ onApply, onAddBranch }: { onApply: (r: VatRegistrant) => void; onAddBranch?: (r: VatRegistrant) => void }) {
  const [id, setId] = useState('')
  const [added, setAdded] = useState<string[]>([])
  return (
    <VatLookup
      source="/companies/c1/vat-registrants"
      taxId={id}
      onTaxIdChange={setId}
      onApply={onApply}
      onAddBranch={(r) => {
        onAddBranch?.(r)
        setAdded((a) => [...a, r.branch_no])
      }}
      addedBranches={added}
    />
  )
}

describe('VatLookup', () => {
  it('หลายสาขา = สำนักงานใหญ่เป็นสาขาหลัก, สาขาอื่นกดเพิ่มทีละสาขา', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(200, { data: [row('00001', 'สาขา มหาชัย'), row('00000', 'สำนักงานใหญ่')] })))
    const onApply = vi.fn()
    const onAddBranch = vi.fn()
    render(<Harness onApply={onApply} onAddBranch={onAddBranch} />)
    await userEvent.type(screen.getByLabelText(texts.partners.lookup.taxId), '0745-5690-02816')
    expect(screen.getByLabelText(texts.partners.lookup.taxId)).toHaveValue('0745569002816') // ตัวอักษรที่ไม่ใช่ตัวเลขถูกตัด
    await userEvent.click(screen.getByRole('button', { name: texts.partners.lookup.button }))
    const add = await screen.findByRole('button', { name: texts.partners.lookup.addBranch })
    expect(onApply).toHaveBeenCalledWith(expect.objectContaining({ branch_no: '00000' }))
    await userEvent.click(add)
    expect(onAddBranch).toHaveBeenCalledWith(expect.objectContaining({ branch_no: '00001' }))
    expect(screen.getByRole('button', { name: texts.partners.lookup.added })).toBeDisabled()
  })

  it('ระบบสรรพากรล่ม (503) = แจ้งให้ลองใหม่หรือกรอกเอง', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(503, { error: { code: 'vat_registry_unavailable', message: 'x' } })))
    render(<Harness onApply={vi.fn()} />)
    await userEvent.type(screen.getByLabelText(texts.partners.lookup.taxId), '0745569002816')
    await userEvent.click(screen.getByRole('button', { name: texts.partners.lookup.button }))
    expect(await screen.findByText(texts.partners.lookup.unavailable)).toBeInTheDocument()
  })
})
