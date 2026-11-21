'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cx } from '@/lib/cx'

export type AppNavItem = { href: string; label: string }

/** เมนูด้านบนของแอป (บนแถบสีเข้ม) — เมนูที่ตรงกับ URL ปัจจุบัน (รวมหน้าย่อย เช่น /branches/new) ถูกเน้น; จอเล็กเลื่อนซ้าย-ขวาได้ */
export function AppNav({ items, label }: { items: readonly AppNavItem[]; label: string }) {
  const pathname = usePathname()
  return (
    <nav aria-label={label} className="flex h-full min-w-0 items-stretch overflow-x-auto [scrollbar-width:none]">
      {items.map((item) => {
        const active = pathname === item.href || pathname.startsWith(`${item.href}/`)
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? 'page' : undefined}
            className={cx(
              'relative flex items-center px-3 text-[13.5px] whitespace-nowrap transition-colors outline-none focus-visible:bg-nav-hover',
              active ? 'font-medium text-white' : 'text-white/65 hover:bg-nav-hover hover:text-white',
            )}
          >
            {item.label}
            {active && <span aria-hidden className="absolute inset-x-3 bottom-0 h-0.5 rounded-full bg-accent" />}
          </Link>
        )
      })}
    </nav>
  )
}
