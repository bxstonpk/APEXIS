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
}

const alignClass = { left: 'text-left', right: 'text-right', center: 'text-center' } as const

/**
 * ตารางข้อมูล — การเรียงเป็นลิงก์ (?sort=) จึงกด Back / คัดลอกลิงก์ได้ และ render ฝั่ง server ได้
 * กดหัวคอลัมน์: ยังไม่เรียง / เรียงลง → เรียงขึ้น, เรียงขึ้น → เรียงลง ("-key")
 */
export function DataTable<T>({ columns, rows, rowKey, sort = '', sortHref, emptyText, sortLabels }: DataTableProps<T>) {
  const desc = sort.startsWith('-')
  const sortedBy = desc ? sort.slice(1) : sort

  return (
    <div className="overflow-x-auto rounded-xl border border-line bg-surface">
      <table className="w-full border-collapse text-sm">
        <thead className="bg-canvas text-ink-muted">
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
                    'border-b border-line px-4 py-3 font-semibold whitespace-nowrap',
                    alignClass[col.align ?? 'left'],
                    col.hideOnMobile && 'hidden md:table-cell',
                  )}
                >
                  {col.sortKey && sortHref ? (
                    <Link
                      href={sortHref(active && !desc ? `-${col.sortKey}` : col.sortKey)}
                      scroll={false}
                      className={cx('inline-flex items-center gap-1 hover:text-ink', active && 'text-ink')}
                    >
                      {col.header}
                      <Icon aria-hidden className={cx('size-3.5', !active && 'opacity-40')} />
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
              <td colSpan={columns.length} className="px-4 py-10 text-center text-ink-muted">
                {emptyText}
              </td>
            </tr>
          ) : (
            rows.map((row) => (
              <tr key={rowKey(row)} className="border-b border-line last:border-b-0 hover:bg-canvas/60">
                {columns.map((col) => (
                  <td
                    key={col.key}
                    className={cx('px-4 py-3 align-middle', alignClass[col.align ?? 'left'], col.hideOnMobile && 'hidden md:table-cell')}
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
