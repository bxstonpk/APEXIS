'use client'

import { X } from 'lucide-react'
import { useRouter } from 'next/navigation'

export type CloseAppButtonProps = {
  /** หน้ารวมแอปของบริษัท — ใช้เมื่อปิดแท็บไม่ได้ */
  launcherHref: string
  label: string
}

/**
 * ปุ่มปิดแอป: ปิดแท็บนี้ (กลับไปแท็บหน้ารวมแอปที่เปิดแอปนี้)
 * เบราว์เซอร์ยอมให้ปิดเฉพาะแท็บที่เปิดด้วย script — ถ้าเปิดเอง (พิมพ์ URL / bookmark) ปิดไม่ได้ จึงไปหน้ารวมแอปในแท็บนี้แทน
 */
export function CloseAppButton({ launcherHref, label }: CloseAppButtonProps) {
  const router = useRouter()

  function handleClick() {
    const opener = window.opener as Window | null
    window.close()
    // ปิดสำเร็จ = script หยุดทำงานไปพร้อมแท็บ; ยังอยู่ = ปิดไม่ได้
    window.setTimeout(() => {
      if (window.closed) return
      router.push(launcherHref)
    }, 100)
    try {
      opener?.focus()
    } catch {
      // opener คนละ origin / ถูกปิดไปแล้ว
    }
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      title={label}
      aria-label={label}
      className="inline-flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-lg text-ink-muted outline-none hover:bg-canvas hover:text-ink focus-visible:ring-3 focus-visible:ring-primary/35"
    >
      <X aria-hidden className="size-5" />
    </button>
  )
}
