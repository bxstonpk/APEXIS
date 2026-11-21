'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cx } from '@/lib/cx'

export type AppNavItem = { href: string; label: string }

/** เมนูด้านบนของแอป — เมนูที่ตรงกับ URL ปัจจุบัน (รวมหน้าย่อย เช่น /branches/new) ถูกเน้น; จอเล็กเลื่อนซ้าย-ขวาได้ */
export function AppNav({ items, label }: { items: readonly AppNavItem[]; label: string }) {
  const pathname = usePathname()
  return (
    <nav aria-label={label} className="-mb-px flex min-w-0 gap-1 overflow-x-auto">
      {items.map((item) => {
        const active = pathname === item.href || pathname.startsWith(`${item.href}/`)
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? 'page' : undefined}
            className={cx(
              'border-b-2 px-3 py-4 text-sm font-medium whitespace-nowrap transition-colors',
              active ? 'border-primary text-primary' : 'border-transparent text-ink-muted hover:text-ink',
            )}
          >
            {item.label}
          </Link>
        )
      })}
    </nav>
  )
}
