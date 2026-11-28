'use client'

import { ArrowLeft, Printer } from 'lucide-react'
import { Button, ButtonLink } from '@/components'
import { texts } from '@/config'

/** พิมพ์เอกสารที่แสดงผ่านแม่แบบ (iframe data-print-frame) — ไม่มี iframe = พิมพ์ทั้งหน้า */
function printDocument() {
  const frame = document.querySelector<HTMLIFrameElement>('iframe[data-print-frame]')
  if (frame?.contentWindow) frame.contentWindow.print()
  else window.print()
}

/** แถบเหนือเอกสารพิมพ์ (ไม่พิมพ์) — กลับไปหน้าเอกสาร / สั่งพิมพ์ */
export function PrintToolbar({ backHref }: { backHref: string }) {
  const p = texts.sales.print
  return (
    <div className="mx-auto mb-4 flex w-full max-w-[210mm] flex-wrap items-center gap-2 print:hidden">
      <ButtonLink href={backHref} variant="secondary">
        <ArrowLeft aria-hidden />
        {p.back}
      </ButtonLink>
      <Button onClick={printDocument} className="ml-auto">
        <Printer aria-hidden />
        {p.printNow}
      </Button>
    </div>
  )
}
