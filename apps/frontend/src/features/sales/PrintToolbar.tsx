'use client'

import { ArrowLeft, Printer } from 'lucide-react'
import { Button, ButtonLink } from '@/components'
import { texts } from '@/config'

/** แถบเหนือเอกสารพิมพ์ (ไม่พิมพ์) — กลับไปหน้าเอกสาร / สั่งพิมพ์ */
export function PrintToolbar({ backHref }: { backHref: string }) {
  const p = texts.sales.print
  return (
    <div className="mx-auto mb-4 flex w-full max-w-[210mm] flex-wrap items-center gap-2 print:hidden">
      <ButtonLink href={backHref} variant="secondary">
        <ArrowLeft aria-hidden />
        {p.back}
      </ButtonLink>
      <p className="order-last w-full text-[13px] text-ink-muted sm:order-none sm:w-auto sm:flex-1">{p.hint}</p>
      <Button onClick={() => window.print()} className="ml-auto sm:ml-0">
        <Printer aria-hidden />
        {p.printNow}
      </Button>
    </div>
  )
}
