import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { texts } from '@/config'
import { ListView } from '@/features/lists/ListParts'
import type { ListResult } from '@/lib/server/list'
import { showToast } from '@/lib/toast'
import { Toaster } from './Toaster'

afterEach(() => {
  vi.useRealTimers()
  sessionStorage.clear()
})

describe('Toaster', () => {
  it('แสดงข้อความจาก showToast แบบไม่แย่งโฟกัส และหายเองใน 4 วินาที', () => {
    vi.useFakeTimers()
    render(<Toaster />)
    act(() => showToast('บันทึกเรียบร้อย'))
    expect(screen.getByRole('status')).toHaveTextContent('บันทึกเรียบร้อย')
    expect(document.activeElement).toBe(document.body)
    act(() => vi.advanceTimersByTime(4100))
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('ข้อความที่ค้างจากหน้าก่อน (บันทึกแล้วเปลี่ยนหน้า) แสดงตอนเปิดหน้า และปิดเองได้', async () => {
    sessionStorage.setItem('apexis.toast', 'ลบเรียบร้อย')
    render(<Toaster />)
    expect(screen.getByRole('status')).toHaveTextContent('ลบเรียบร้อย')
    expect(sessionStorage.getItem('apexis.toast')).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: texts.common.toast.close }))
    expect(screen.queryByRole('status')).toBeNull()
  })
})

describe('ListView หน้าว่าง', () => {
  const base: Omit<ListResult<{ id: string }>, 'query'> = { rows: [], meta: { page: 1, page_size: 20, total: 0 }, error: null }
  const view = (query: string, createHref?: string) =>
    render(<ListView list={{ ...base, query: new URLSearchParams(query) }} path="/list" columns={[]} rowKey={(r) => r.id} sort="code" createHref={createHref} />)

  it('ยังไม่มีข้อมูล + มีสิทธิ์สร้าง = ปุ่มสร้าง', () => {
    view('sort=-code', '/list/new')
    expect(screen.getByText(texts.common.emptyTitle)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: texts.common.create })).toHaveAttribute('href', '/list/new')
  })

  it('มีตัวกรองแต่ไม่พบ = ลิงก์ล้างตัวกรอง', () => {
    view('q=zzz&status=active')
    expect(screen.getByText(texts.common.noMatch)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: texts.common.clearFilters })).toHaveAttribute('href', '/list')
  })
})
