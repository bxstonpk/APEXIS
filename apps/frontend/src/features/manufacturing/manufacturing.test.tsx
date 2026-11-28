import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { visibleApps } from '@/config/apps'
import { BomForm } from './BomForm'
import { checkQuantities, issueDefaults, OrderActions } from './OrderActions'
import type { Bom, OrderMaterial, ProductionOrder } from './types'

const push = vi.fn()
const refresh = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh }) }))

afterEach(() => {
  push.mockReset()
  refresh.mockReset()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const calls = (fetch: ReturnType<typeof vi.fn>, method: string) => (fetch.mock.calls as unknown as [string, RequestInit][]).filter(([, i]) => i?.method === method)
const m = texts.manufacturing

const bom: Bom = {
  id: 'b1',
  code: 'BOM-A',
  name: 'สูตร A',
  item: { id: 'fg', code: 'FG', name: 'สินค้า A', uom: 'ชิ้น' },
  output_quantity: '10',
  status: 'active',
  is_default: true,
  components: [
    { line_no: 1, item: { id: 'rm1', code: 'RM1', name: 'แป้ง' }, quantity: '5', scrap_percent: '2' },
    { line_no: 2, item: { id: 'rm2', code: 'RM2', name: 'น้ำตาล' }, quantity: '1', scrap_percent: '0' },
  ],
  conversion_costs: [{ line_no: 1, description: 'ค่าแรง', account: { id: 'a1', code: '5062000000', name: 'ค่าแรงทางตรง' }, amount: '200' }],
}

describe('BomForm', () => {
  it('ส่ง PUT ทั้งชุด (วัตถุดิบ + ค่าแปลงสภาพ) และกลับหน้ารายการ', async () => {
    const fetch = vi.fn(async () => json(200, { data: { id: 'b1' } }))
    vi.stubGlobal('fetch', fetch)
    render(<BomForm apiPath="/boms" listHref="/list" itemsApi="/items" accountsApi="/acc" canManage bom={bom} />)
    const qty = screen.getByLabelText(`${m.boms.fields.quantity} บรรทัด 1`)
    await userEvent.clear(qty)
    await userEvent.type(qty, '6')
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    const [call] = calls(fetch, 'PUT')
    expect(call![0]).toContain('/boms/b1')
    expect(JSON.parse(String(call![1].body))).toEqual({
      code: 'BOM-A',
      name: 'สูตร A',
      item_id: 'fg',
      output_quantity: '10',
      status: 'active',
      is_default: true,
      components: [
        { item_id: 'rm1', quantity: '6', scrap_percent: '2' },
        { item_id: 'rm2', quantity: '1', scrap_percent: '0' },
      ],
      conversion_costs: [{ description: 'ค่าแรง', account_id: 'a1', amount: '200' }],
    })
    expect(push).toHaveBeenCalledWith('/list')
  })

  it('จำนวนต่อรอบ 0 / % สูญเสียเกิน 100 / จำนวนผลิตว่าง แจ้งที่ช่องและไม่ส่ง', async () => {
    const fetch = vi.fn(async () => json(200, { data: {} }))
    vi.stubGlobal('fetch', fetch)
    render(<BomForm apiPath="/boms" listHref="/list" itemsApi="/items" accountsApi="/acc" canManage bom={bom} />)
    await userEvent.clear(screen.getByLabelText(`${m.boms.fields.quantity} บรรทัด 2`))
    await userEvent.type(screen.getByLabelText(`${m.boms.fields.quantity} บรรทัด 2`), '0')
    await userEvent.clear(screen.getByLabelText(`${m.boms.fields.scrap} บรรทัด 1`))
    await userEvent.type(screen.getByLabelText(`${m.boms.fields.scrap} บรรทัด 1`), '120')
    await userEvent.clear(screen.getByLabelText(new RegExp(m.boms.fields.outputQty)))
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    expect(screen.getAllByText(m.boms.errors.qtyInvalid)).toHaveLength(2)
    expect(screen.getByText(m.boms.errors.scrapInvalid)).toBeInTheDocument()
    expect(calls(fetch, 'PUT')).toHaveLength(0)
  })

  it('สร้างใหม่ไม่เลือกสินค้า / วัตถุดิบ แจ้งเตือนและไม่ส่ง', async () => {
    const fetch = vi.fn(async () => json(200, { data: [] }))
    vi.stubGlobal('fetch', fetch)
    render(<BomForm apiPath="/boms" listHref="/list" itemsApi="/items" accountsApi="/acc" canManage />)
    await userEvent.type(screen.getByLabelText(new RegExp(m.boms.fields.code)), 'b-1')
    await userEvent.type(screen.getByLabelText(new RegExp(m.boms.fields.name)), 'สูตร')
    await userEvent.click(screen.getByRole('button', { name: texts.common.save }))
    expect(screen.getAllByText(m.boms.errors.itemRequired).length).toBeGreaterThanOrEqual(2)
    expect(calls(fetch, 'POST')).toHaveLength(0)
  })
})

const materials: OrderMaterial[] = [
  { line_no: 1, item: { id: 'rm1', code: 'RM1', name: 'แป้ง', uom: 'กก.' }, required_quantity: '10', issued_quantity: '4', returned_quantity: '1', net_quantity: '3', cost: '90' },
  { line_no: 2, item: { id: 'rm2', code: 'RM2', name: 'น้ำตาล', uom: 'กก.' }, required_quantity: '2', issued_quantity: '3', returned_quantity: '0', net_quantity: '3', cost: '30' },
]

const order: ProductionOrder = {
  id: 'o1',
  document_no: 'MO2026-000001',
  date: '2026-11-26',
  item: { id: 'fg', code: 'FG', name: 'สินค้า A', uom: 'ชิ้น' },
  bom: { id: 'b1', code: 'BOM-A', name: 'สูตร A' },
  quantity: '20',
  warehouse: { id: 'w1', code: 'WH', name: 'คลังหลัก' },
  planned_start: null,
  planned_end: null,
  status: 'released',
  description: '',
  materials,
  completed_quantity: null,
  completed_at: null,
  cancelled_at: null,
  cancel_reason: '',
}

describe('การเบิก / คืนวัตถุดิบ', () => {
  it('ค่าตั้งต้นของการเบิก = ต้องใช้ − ใช้จริง (ไม่ติดลบ)', () => {
    expect(issueDefaults(materials)).toEqual({ rm1: '7', rm2: '0' })
  })

  it('ตรวจเพดาน: เบิกเกินที่ยังต้องเบิก / คืนเกินที่ใช้จริง / ไม่มีจำนวนเลย', () => {
    expect(checkQuantities('issue', materials, { rm1: '8' }).errors.rm1).toContain('7.00')
    expect(checkQuantities('return', materials, { rm2: '3.5' }).errors.rm2).toContain('3.00')
    expect(checkQuantities('issue', materials, { rm1: '0', rm2: '' }).errors._).toBe(m.orders.panels.nothing)
    expect(checkQuantities('issue', materials, { rm1: '7', rm2: '0' })).toEqual({ errors: {}, lines: [{ item_id: 'rm1', quantity: '7' }] })
  })

  it('แผงเบิก: ตั้งต้นจำนวนที่ยังต้องเบิก, เกินเพดานไม่ส่ง, แก้แล้วส่ง POST /issue', async () => {
    const fetch = vi.fn(async () => json(200, { data: {} }))
    vi.stubGlobal('fetch', fetch)
    render(<OrderActions apiPath="/po/o1" order={order} canManage printHref="/print" />)
    await userEvent.click(screen.getByRole('button', { name: m.orders.actions.issue }))
    const panel = screen.getByRole('form', { name: m.orders.panels.issueTitle })
    const rm1 = within(panel).getByLabelText('จำนวน แป้ง')
    expect(rm1).toHaveValue('7')
    await userEvent.clear(rm1)
    await userEvent.type(rm1, '9')
    await userEvent.click(within(panel).getByRole('button', { name: m.orders.actions.issue }))
    expect(within(panel).getByText(/เกินจำนวนที่ยังต้องเบิก/)).toBeInTheDocument()
    expect(calls(fetch, 'POST')).toHaveLength(0)
    await userEvent.clear(rm1)
    await userEvent.type(rm1, '5')
    await userEvent.click(within(panel).getByRole('button', { name: m.orders.actions.issue }))
    const [call] = calls(fetch, 'POST')
    expect(call![0]).toContain('/po/o1/issue')
    expect(JSON.parse(String(call![1].body))).toMatchObject({ lines: [{ item_id: 'rm1', quantity: '5' }] })
    expect(refresh).toHaveBeenCalled()
  })

  it('ผลิตเสร็จ: จำนวนตั้งต้น = จำนวนสั่งผลิต และส่ง completed_quantity', async () => {
    const fetch = vi.fn(async () => json(200, { data: {} }))
    vi.stubGlobal('fetch', fetch)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<OrderActions apiPath="/po/o1" order={order} canManage printHref="/print" />)
    await userEvent.click(screen.getByRole('button', { name: m.orders.actions.complete }))
    const panel = screen.getByRole('form', { name: m.orders.panels.completeTitle })
    const qty = within(panel).getByLabelText(new RegExp(m.orders.panels.completedQty))
    expect(qty).toHaveValue('20')
    await userEvent.clear(qty)
    await userEvent.type(qty, '18')
    await userEvent.click(within(panel).getByRole('button', { name: m.orders.actions.complete }))
    const [call] = calls(fetch, 'POST')
    expect(call![0]).toContain('/po/o1/complete')
    expect(JSON.parse(String(call![1].body))).toMatchObject({ completed_quantity: '18' })
  })

  it('ยกเลิกต้องมีเหตุผล', async () => {
    const fetch = vi.fn(async () => json(200, { data: {} }))
    vi.stubGlobal('fetch', fetch)
    render(<OrderActions apiPath="/po/o1" order={order} canManage printHref="/print" />)
    await userEvent.click(screen.getByRole('button', { name: m.orders.actions.cancel }))
    const panel = screen.getByRole('form', { name: m.orders.panels.cancelTitle })
    await userEvent.click(within(panel).getByRole('button', { name: m.orders.actions.cancel }))
    expect(within(panel).getByText(m.orders.panels.reasonRequired)).toBeInTheDocument()
    await userEvent.type(within(panel).getByLabelText(new RegExp(m.orders.panels.reason)), 'ลูกค้ายกเลิก')
    await userEvent.click(within(panel).getByRole('button', { name: m.orders.actions.cancel }))
    expect(JSON.parse(String(calls(fetch, 'POST')[0]![1].body))).toEqual({ reason: 'ลูกค้ายกเลิก' })
  })

  it('ไม่มีสิทธิ์จัดการ: เหลือแค่ปุ่มพิมพ์', () => {
    render(<OrderActions apiPath="/po/o1" order={order} canManage={false} printHref="/print" />)
    expect(screen.queryByRole('button', { name: m.orders.actions.issue })).toBeNull()
    expect(screen.getByRole('link', { name: m.orders.actions.print })).toBeInTheDocument()
  })
})

describe('แอปการผลิต', () => {
  it('แสดงเฉพาะธุรกิจผลิตสินค้า', () => {
    expect(visibleApps(['*'], 'manufacturing').map((a) => a.key)).toContain('manufacturing')
    for (const type of ['service', 'trading', 'construction'] as const) {
      expect(visibleApps(['*'], type).map((a) => a.key)).not.toContain('manufacturing')
    }
  })
})
