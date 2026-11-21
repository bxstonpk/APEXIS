'use client'

import { CircleCheck, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { texts } from '@/config'
import { TOAST_EVENT, TOAST_KEY, takePendingToast } from '@/lib/toast'

const DISMISS_MS = 4000

/**
 * แสดง toast มุมขวาล่าง (ไม่บังปุ่มด้านบน เช่น "สร้าง") — หายเองใน 4 วินาที, ปิดเองได้, ไม่แย่งโฟกัส (aria-live="polite")
 * รับข้อความจาก showToast() (lib/toast.ts)
 */
export function Toaster() {
  const [message, setMessage] = useState<string | null>(null)

  useEffect(() => {
    const pending = takePendingToast()
    if (pending) setMessage(pending)
    function onToast(e: Event) {
      try {
        sessionStorage.removeItem(TOAST_KEY)
      } catch {
        // ignore
      }
      setMessage((e as CustomEvent<string>).detail)
    }
    window.addEventListener(TOAST_EVENT, onToast)
    return () => window.removeEventListener(TOAST_EVENT, onToast)
  }, [])

  useEffect(() => {
    if (!message) return
    const id = window.setTimeout(() => setMessage(null), DISMISS_MS)
    return () => window.clearTimeout(id)
  }, [message])

  return (
    <div aria-live="polite" className="pointer-events-none fixed inset-x-4 bottom-6 z-40 flex justify-center sm:inset-x-auto sm:right-6 sm:justify-end">
      {message && (
        <div
          role="status"
          className="pointer-events-auto flex items-center gap-2.5 rounded-(--radius-panel) border border-line bg-surface py-2.5 pr-2 pl-3.5 text-sm shadow-pop motion-safe:animate-[toast-in_160ms_ease-out]"
        >
          <CircleCheck aria-hidden className="size-4 shrink-0 text-success" />
          <span className="font-medium">{message}</span>
          <button
            type="button"
            onClick={() => setMessage(null)}
            aria-label={texts.common.toast.close}
            className="ml-1 inline-flex size-7 cursor-pointer items-center justify-center rounded-md text-ink-subtle hover:bg-neutral-soft hover:text-ink"
          >
            <X aria-hidden className="size-4" />
          </button>
        </div>
      )}
    </div>
  )
}
