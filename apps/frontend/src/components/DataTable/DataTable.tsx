import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { cx } from '@/lib/cx'

export type Column<T> = {
  key: string
  header: string
  render: (row: T) => ReactNode
  /** มีค่า = กดหัวคอลัมน์เพื่อเรียงได้ (ค่าที่ส่งใน ?sort= ของ API) */
  sortKey?: string
  align?: 'left' | 'right' | 'center'
  /** ซ่อนบนจอเล็ก */
  hideOnMobile?: boolean
}

export type DataTableProps<T> = {
  columns: readonly Column<T>[]
  rows: readonly T[]
  rowKey: (row: T) => string
  /** ค่า ?sort= ปัจจุบัน เช่น "code" หรือ "-name" */
  sort?: string
  /** URL ของหน้าเมื่อเรียงด้วยค่า sort ใหม่ (คงเงื่อนไขค้นหาเดิมไว้) */
  sortHref?: (sort: string) => string
  emptyText: string
  /** คำอธิบายทิศทางการเรียงสำหรับ screen reader (จาก config) */
  sortLabels: { ascending: string; descending: string }
  /** ไม่มีกรอบ (อยู่ในกรอบของผู้ใช้ component แล้ว) */
  bare?: boolean
}

const alignClass = { left: 'text-left', right: 'text-right', center: 'text-center' } as const

/**
 * ตารางข้อมูล — การเรียงเป็นลิงก์ (?sort=) จึงกด Back / คัดลอกลิงก์ได้ และ render ฝั่ง server ได้
 * กดหัวคอลัมน์: ยังไม่เรียง / เรียงลง → เรียงขึ้น, เรียงขึ้น → เรียงลง ("-key")
 */
export function DataTable<T>({ columns, rows, rowKey, sort = '', sortHref, emptyText, sortLabels, bare = false }: DataTableProps<T>) {
  const desc = sort.startsWith('-')
  const sortedBy = desc ? sort.slice(1) : sort

  return (
    <div className={cx('overflow-x-auto bg-surface', !bare && 'rounded-(--radius-panel) border border-line shadow-card')}>
      <table className="w-full border-collapse text-sm tabular-nums">
        <thead className="bg-subtle text-[13px] text-ink-muted">
          <tr>
            {columns.map((col) => {
              const active = col.sortKey !== undefined && col.sortKey === sortedBy
              const ariaSort = active ? (desc ? 'descending' : 'ascending') : undefined
              const Icon = active ? (desc ? ArrowDown : ArrowUp) : ArrowUpDown
              return (
                <th
                  key={col.key}
                  scope="col"
                  aria-sort={ariaSort}
                  className={cx(
                    'h-10 border-b border-line px-4 font-medium whitespace-nowrap',
                    alignClass[col.align ?? 'left'],
                    col.hideOnMobile && 'hidden md:table-cell',
                  )}
                >
                  {col.sortKey && sortHref ? (
                    <Link
                      href={sortHref(active && !desc ? `-${col.sortKey}` : col.sortKey)}
                      scroll={false}
                      className={cx('group inline-flex items-center gap-1 hover:text-ink', active && 'text-ink')}
                    >
                      {col.header}
                      <Icon aria-hidden className={cx('size-3.5', !active && 'opacity-0 transition-opacity group-hover:opacity-60')} />
                      {active && <span className="sr-only">{desc ? sortLabels.descending : sortLabels.ascending}</span>}
                    </Link>
                  ) : (
                    col.header
                  )}
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className="px-4 py-14 text-center text-ink-muted">
                {emptyText}
              </td>
            </tr>
          ) : (
            rows.map((row) => (
              <tr key={rowKey(row)} className="border-b border-line last:border-b-0 hover:bg-subtle">
                {columns.map((col) => (
                  <td
                    key={col.key}
                    className={cx('h-12 px-4 align-middle', alignClass[col.align ?? 'left'], col.hideOnMobile && 'hidden md:table-cell')}
                  >
                    {col.render(row)}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
