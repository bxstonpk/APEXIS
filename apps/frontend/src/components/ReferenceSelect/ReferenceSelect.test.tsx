import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ReferenceSelect } from './ReferenceSelect'

const texts = { placeholder: 'พิมพ์ค้นหา', searching: 'กำลังค้นหา', noResults: 'ไม่พบ', clear: 'ล้างค่า{label}' }
const rows = [
  { id: 'b1', code: 'HQ', name: 'สำนักงานใหญ่', status: 'active' },
  { id: 'b2', code: 'CNX', name: 'เชียงใหม่', status: 'active' },
]

function json(body: unknown) {
  return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

async function flush() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(300)
  })
}

describe('ReferenceSelect', () => {
  it('พิมพ์แล้วค้นหาจาก API (q + params + page_size) และตัดรายการ excludeId', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ data: rows }))
    render(<ReferenceSelect label="สาขา" source="/companies/c1/branches" params={{ status: 'active' }} excludeId="b2" texts={texts} />)
    const input = screen.getByRole('combobox', { name: 'สาขา' })
    fireEvent.focus(input)
    fireEvent.change(input, { target: { value: 'hq' } })
    await flush()

    const url = String(fetch.mock.calls.at(-1)![0])
    expect(url).toContain('/api/v1/companies/c1/branches?')
    expect(url).toContain('status=active')
    expect(url).toContain('page_size=20')
    expect(url).toContain('q=hq')
    expect(screen.getAllByRole('option')).toHaveLength(1)
  })

  it('เลือกด้วยคีย์บอร์ด → ส่ง id ใน hidden input และแสดง "รหัส · ชื่อ"; ปุ่มล้างค่า', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ data: rows }))
    const onChange = vi.fn()
    const { container } = render(<ReferenceSelect label="สาขา" name="branch_id" source="/x" onChange={onChange} texts={texts} />)
    const input = screen.getByRole('combobox')
    fireEvent.focus(input)
    await flush()
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(onChange).toHaveBeenCalledWith({ id: 'b2', code: 'CNX', name: 'เชียงใหม่' })
    expect(input).toHaveValue('CNX · เชียงใหม่')
    expect(container.querySelector('input[type=hidden]')).toHaveValue('b2')

    fireEvent.click(screen.getByRole('button', { name: 'ล้างค่าสาขา' }))
    expect(onChange).toHaveBeenLastCalledWith(null)
    expect(container.querySelector('input[type=hidden]')).toHaveValue('')
  })

  it('ไม่พบรายการ / ค่าเริ่มต้นจาก server', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ data: [] }))
    render(<ReferenceSelect label="แผนก" source="/x" defaultValue={{ id: 'd1', code: 'ACC', name: 'บัญชี' }} texts={texts} />)
    const input = screen.getByRole('combobox')
    expect(input).toHaveValue('ACC · บัญชี')
    fireEvent.focus(input)
    await flush()
    expect(screen.getByText('ไม่พบ')).toBeInTheDocument()
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(screen.queryByRole('listbox')).toBeNull()
  })
})
