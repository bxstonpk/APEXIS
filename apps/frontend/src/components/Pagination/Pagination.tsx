import { ChevronLeft, ChevronRight } from 'lucide-react'
import Link from 'next/link'
import { cx } from '@/lib/cx'

export type PaginationProps = {
  page: number
  pageSize: number
  total: number
  /** URL ของหน้าที่ n (คงเงื่อนไขค้นหา / เรียงเดิมไว้) */
  href: (page: number) => string
  /** ข้อความจาก config — summary ใช้ {from} {to} {total} */
  labels: { label: string; previous: string; next: string; summary: string }
}

/** แบ่งหน้าแบบลิงก์ (?page=) — แสดงช่วงรายการ + ปุ่มก่อนหน้า / ถัดไป */
export function Pagination({ page, pageSize, total, href, labels }: PaginationProps) {
  const lastPage = Math.max(1, Math.ceil(total / pageSize))
  const from = total === 0 ? 0 : (page - 1) * pageSize + 1
  const to = Math.min(total, page * pageSize)
  const summary = labels.summary
    .replace('{from}', from.toLocaleString('th-TH'))
    .replace('{to}', to.toLocaleString('th-TH'))
    .replace('{total}', total.toLocaleString('th-TH'))

  return (
    <nav aria-label={labels.label} className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-ink-muted">
      <p>{summary}</p>
      <div className="flex gap-2">
        <PageLink href={page > 1 ? href(page - 1) : undefined} label={labels.previous} icon="prev" />
        <PageLink href={page < lastPage ? href(page + 1) : undefined} label={labels.next} icon="next" />
      </div>
    </nav>
  )
}

function PageLink({ href, label, icon }: { href?: string; label: string; icon: 'prev' | 'next' }) {
  const cls = 'inline-flex min-h-9 items-center gap-1 rounded-lg border border-line-strong bg-surface px-3 font-semibold'
  const content = (
    <>
      {icon === 'prev' && <ChevronLeft aria-hidden className="size-4" />}
      {label}
      {icon === 'next' && <ChevronRight aria-hidden className="size-4" />}
    </>
  )
  if (!href) {
    return (
      <span aria-disabled="true" className={cx(cls, 'cursor-not-allowed opacity-50')}>
        {content}
      </span>
    )
  }
  return (
    <Link href={href} className={cx(cls, 'text-ink hover:border-primary')}>
      {content}
    </Link>
  )
}
