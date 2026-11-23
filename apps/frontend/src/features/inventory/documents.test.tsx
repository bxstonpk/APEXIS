import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { needsCost, StockDocForm } from './DocForm'
import type { StockDoc } from './types'

const push = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh: vi.fn() }) }))

afterEach(() => {
  push.mockReset()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const t = texts.inventory.documents

describe('needsCost', () => {
  it('รับเข้าต้องมีต้นทุนเสมอ; ปรับปรุงยอดเฉพาะจำนวนบวก; เบิก / โอนใช้ FIFO', () => {
    expect(needsCost('receipt', '5')).toBe(true)
    expect(needsCost('adjustment', '3')).toBe(true)
    expect(needsCost('adjustment', '-3')).toBe(false)
    expect(needsCost('issue', '5')).toBe(false)
    expect(needsCost('transfer', '5')).toBe(false)
  })
})

const draft: StockDoc = {
  id: 'd1',
  type: 'transfer',
  document_no: null,
  date: '2026-11-20',
  warehouse: { id: 'w1', code: 'W1', name: 'คลัง 1' },
  to_warehouse: { id: 'w2', code: 'W2', name: 'คลัง 2' },
  reference: '',
  description: '',
  status: 'draft',
  total_value: '0',
  posted_at: null,
  created_by: 'u',
  cancelled_at: null,
  lines: [{ line_no: 1, item: { id: 'i1', code: 'M1', name: 'เหล็ก' }, uom_code: 'KG', quantity: '4', unit_cost: null, value: '0', description: '' }],
}

describe('StockDocForm', () => {
  it('รับเข้า: ไม่ระบุต้นทุน = แจ้งที่ช่องและไม่ส่ง', async () => {
    const fetch = vi.fn(async () => json(200, { data: [] }))
    vi.stubGlobal('fetch', fetch)
    render(<StockDocForm apiPath="/companies/c1/stock-documents" itemsApi="/i" warehousesApi="/w" docsPath="/docs" listHref="/l" canPost />)
    await userEvent.type(screen.getByLabelText(`${t.lines.quantity} บรรทัด 1`), '5')
    await userEvent.click(screen.getByRole('button', { name: t.actions.saveDraft }))
    expect(screen.getByText(t.lines.costRequired)).toBeInTheDocument()
    expect((fetch.mock.calls as unknown as [string, RequestInit?][]).some(([, init]) => init?.method === 'POST')).toBe(false)
  })

  it('แก้ร่างโอนคลัง แล้วบันทึกและผ่านเอกสาร = PUT (ไม่มีต้นทุน, มีคลังปลายทาง) แล้ว PATCH posted', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const fetch = vi.fn(async () => json(200, { data: { id: 'd1' } }))
    vi.stubGlobal('fetch', fetch)
    render(<StockDocForm apiPath="/companies/c1/stock-documents" itemsApi="/i" warehousesApi="/w" docsPath="/docs" listHref="/l" canPost doc={draft} />)
    expect(screen.getByRole('radio', { name: t.types.transfer })).toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: t.actions.savePost }))
    const calls = (fetch.mock.calls as unknown as [string, RequestInit][]).filter(([, i]) => i?.method === 'PUT' || i?.method === 'PATCH')
    expect(calls.map(([, i]) => i.method)).toEqual(['PUT', 'PATCH'])
    const body = JSON.parse(String(calls[0]![1].body))
    expect(body).toMatchObject({ type: 'transfer', warehouse_id: 'w1', to_warehouse_id: 'w2', lines: [{ item_id: 'i1', quantity: '4', unit_cost: null }] })
    expect(JSON.parse(String(calls[1]![1].body))).toEqual({ status: 'posted' })
    expect(push).toHaveBeenCalledWith('/docs/d1')
  })
})
