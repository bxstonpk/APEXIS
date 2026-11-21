import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Pagination } from '../Pagination/Pagination'
import { DataTable, type Column } from './DataTable'

type Row = { id: string; code: string; name: string }
const columns: Column<Row>[] = [
  { key: 'code', header: 'รหัส', sortKey: 'code', render: (r) => r.code },
  { key: 'name', header: 'ชื่อ', sortKey: 'name', render: (r) => r.name },
  { key: 'x', header: 'อื่น', render: () => '-' },
]
const labels = { ascending: 'น้อยไปมาก', descending: 'มากไปน้อย' }
const sortHref = (s: string) => `/list?sort=${s}`

describe('DataTable', () => {
  it('หัวคอลัมน์เป็นลิงก์เรียง: กดคอลัมน์ที่เรียงขึ้นอยู่ = เรียงลง, คอลัมน์อื่น = เรียงขึ้น', () => {
    render(<DataTable columns={columns} rows={[]} rowKey={(r) => r.id} sort="code" sortHref={sortHref} emptyText="ว่าง" sortLabels={labels} />)
    expect(screen.getByRole('link', { name: /รหัส/ })).toHaveAttribute('href', '/list?sort=-code')
    expect(screen.getByRole('link', { name: /ชื่อ/ })).toHaveAttribute('href', '/list?sort=name')
    expect(screen.getByRole('columnheader', { name: /รหัส/ })).toHaveAttribute('aria-sort', 'ascending')
    expect(within(screen.getByRole('columnheader', { name: 'อื่น' })).queryByRole('link')).toBeNull()
  })

  it('เรียงลงอยู่ → กดแล้วเรียงขึ้น; ไม่มีข้อมูล = แสดงข้อความว่าง', () => {
    render(<DataTable columns={columns} rows={[]} rowKey={(r) => r.id} sort="-code" sortHref={sortHref} emptyText="ว่าง" sortLabels={labels} />)
    expect(screen.getByRole('link', { name: /รหัส/ })).toHaveAttribute('href', '/list?sort=code')
    expect(screen.getByRole('columnheader', { name: /รหัส/ })).toHaveAttribute('aria-sort', 'descending')
    expect(screen.getByText('ว่าง')).toBeInTheDocument()
  })

  it('แสดงแถวข้อมูล', () => {
    render(
      <DataTable columns={columns} rows={[{ id: '1', code: 'HQ', name: 'สำนักงานใหญ่' }]} rowKey={(r) => r.id} emptyText="ว่าง" sortLabels={labels} />,
    )
    expect(screen.getByRole('cell', { name: 'HQ' })).toBeInTheDocument()
  })
})

describe('Pagination', () => {
  const pl = { label: 'แบ่งหน้า', previous: 'ก่อนหน้า', next: 'ถัดไป', summary: '{from}–{to} จาก {total}' }
  it('ช่วงรายการ + ลิงก์ก่อนหน้า / ถัดไป', () => {
    render(<Pagination page={2} pageSize={20} total={45} href={(p) => `/l?page=${p}`} labels={pl} />)
    expect(screen.getByText('21–40 จาก 45')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /ก่อนหน้า/ })).toHaveAttribute('href', '/l?page=1')
    expect(screen.getByRole('link', { name: /ถัดไป/ })).toHaveAttribute('href', '/l?page=3')
  })

  it('หน้าสุดท้าย: ถัดไปกดไม่ได้', () => {
    render(<Pagination page={3} pageSize={20} total={45} href={(p) => `/l?page=${p}`} labels={pl} />)
    expect(screen.getByText('41–45 จาก 45')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /ถัดไป/ })).toBeNull()
  })
})
