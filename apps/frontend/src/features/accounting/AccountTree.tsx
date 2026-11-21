'use client'

import { ChevronRight, ChevronsDownUp, ChevronsUpDown } from 'lucide-react'
import Link from 'next/link'
import { useMemo, useState, type CSSProperties } from 'react'
import { Badge, Button } from '@/components'
import { format, texts } from '@/config'
import { StatusBadge } from '@/features/lists/ListParts'
import { cx } from '@/lib/cx'
import { branchIds, buildTree, initialExpanded, visibleRows } from './tree'
import type { Account } from './types'

/**
 * ผังบัญชีแบบต้นไม้ (แบบเดียวกับโปรเจกต์ต้นแบบ):
 * ขยาย / ยุบทีละกลุ่ม หรือทั้งหมด, ย่อหน้าตามระดับ, บัญชีคุมตัวหนา + ป้าย "กลุ่มบัญชี"
 * กดรหัส / ชื่อ = ไปหน้าแก้ไขบัญชี
 *
 * ปุ่มขยาย / ยุบเป็น <button> ที่มี aria-expanded (ใช้คีย์บอร์ดได้ทุกแถว)
 */
export function AccountTree({ accounts, basePath }: { accounts: Account[]; /** หน้าแก้ไขบัญชี = {basePath}/{id} */ basePath: string }) {
  const t = texts.accounts
  const roots = useMemo(() => buildTree(accounts), [accounts])
  const allBranches = useMemo(() => branchIds(roots), [roots])
  const [expanded, setExpanded] = useState<Set<string>>(() => initialExpanded(roots))
  const rows = visibleRows(roots, expanded)

  const toggle = (id: string) =>
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  return (
    <div className="overflow-hidden rounded-(--radius-panel) border border-line bg-surface shadow-card">
      <div className="flex flex-wrap items-center gap-2 border-b border-line bg-subtle px-3 py-2">
        <Button size="sm" variant="ghost" onClick={() => setExpanded(new Set(allBranches))}>
          <ChevronsUpDown aria-hidden />
          {t.expandAll}
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setExpanded(new Set())}>
          <ChevronsDownUp aria-hidden />
          {t.collapseAll}
        </Button>
      </div>
      <div>
        <table className="w-full table-fixed border-collapse text-sm md:table-auto" aria-label={t.treeLabel}>
          <thead>
            <tr className="border-b border-line text-left text-[13px] text-ink-muted">
              <th scope="col" className="px-3 py-2.5 font-medium md:w-[45%]">
                {t.columns.code} / {t.columns.name}
              </th>
              <th scope="col" className="hidden px-3 py-2.5 font-medium md:table-cell">
                {texts.accounts.fields.nameEn}
              </th>
              <th scope="col" className="hidden px-3 py-2.5 font-medium md:table-cell">
                {t.columns.type}
              </th>
              <th scope="col" className="hidden w-28 px-3 py-2.5 font-medium sm:table-cell">
                {t.columns.status}
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map(({ account: a, depth, children }) => {
              const open = expanded.has(a.id)
              const header = !a.postable
              return (
                <tr key={a.id} className={cx('border-b border-line last:border-b-0 hover:bg-subtle', depth === 0 && 'bg-primary-soft/40')}>
                  <td className="py-1.5 pr-3 pl-[calc(4px+var(--depth)*12px)] md:pl-[calc(12px+var(--depth)*22px)]" style={{ '--depth': depth } as CSSProperties}>
                    <div className="flex min-w-0 items-center gap-1.5">
                      {children.length > 0 ? (
                        <button
                          type="button"
                          onClick={() => toggle(a.id)}
                          aria-expanded={open}
                          aria-label={format(open ? t.collapse : t.expand, { name: a.name })}
                          className="-my-1 flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-ink-muted hover:bg-neutral-soft hover:text-ink focus-visible:outline-2 focus-visible:outline-primary"
                        >
                          <ChevronRight aria-hidden className={cx('size-4 transition-transform motion-reduce:transition-none', open && 'rotate-90')} />
                        </button>
                      ) : (
                        <span aria-hidden className="-my-1 size-7 shrink-0" />
                      )}
                      <Link href={`${basePath}/${a.id}`} className={cx('flex min-w-0 items-baseline gap-2 hover:underline', header ? 'font-semibold text-ink' : 'text-primary')}>
                        <span className="font-mono text-[13px] tabular-nums text-ink-muted">{a.code}</span>
                        <span className="md:truncate">{a.name}</span>
                      </Link>
                      {header && depth > 0 && <span className="hidden sm:inline-flex"><Badge>{t.group}</Badge></span>}
                      {a.status === 'inactive' && <span className="sm:hidden"><StatusBadge status={a.status} /></span>}
                    </div>
                  </td>
                  <td className="hidden px-3 py-1.5 text-[13px] text-ink-muted md:table-cell">{a.name_en}</td>
                  <td className="hidden px-3 py-1.5 text-[13px] whitespace-nowrap md:table-cell">{t.types[a.type] ?? a.type}</td>
                  <td className="hidden px-3 py-1.5 sm:table-cell">{a.status === 'inactive' && <StatusBadge status={a.status} />}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
