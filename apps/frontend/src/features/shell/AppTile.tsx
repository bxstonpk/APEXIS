'use client'

import type { MouseEvent, ReactNode } from 'react'
import { cx } from '@/lib/cx'

export type AppTileProps = {
  href: string
  /** ชื่อแท็บของเบราว์เซอร์ — กดแอปเดิมซ้ำจะกลับไปที่แท็บเดิม */
  windowName: string
  label: string
  /** ข้อความสำหรับ screen reader / tooltip เช่น "ขาย (เปิดในแท็บใหม่)" */
  title: string
  /** ไอคอนที่ render แล้ว (ส่ง component ข้าม server -> client ไม่ได้) */
  icon: ReactNode
  tone: string
}

/**
 * ไอคอนแอปในหน้ารวมแอป — กดแล้วเปิดแอปในแท็บใหม่ (window.open ด้วยชื่อแท็บของแอป)
 * เปิดด้วย script เพื่อให้ปุ่มปิดในแท็บนั้นปิดแท็บได้ (window.close); คลิกกลาง / Ctrl+คลิก ใช้ลิงก์ปกติ
 */
export function AppTile({ href, windowName, label, title, icon, tone }: AppTileProps) {
  function handleClick(e: MouseEvent<HTMLAnchorElement>) {
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
    const win = window.open(href, windowName)
    if (win) {
      e.preventDefault()
      win.focus()
    }
  }

  return (
    <a
      href={href}
      target={windowName}
      title={title}
      aria-label={title}
      onClick={handleClick}
      className="group flex flex-col items-center gap-2 rounded-2xl p-2 outline-none focus-visible:ring-3 focus-visible:ring-primary/35"
    >
      <span
        className={cx(
          'flex size-16 items-center justify-center rounded-2xl text-white shadow-card transition-transform sm:size-20',
          'group-hover:-translate-y-0.5 group-hover:shadow-lg',
          tone,
        )}
      >
        {icon}
      </span>
      <span className="text-center text-sm font-medium text-ink">{label}</span>
    </a>
  )
}
