'use client'

import { Download, Printer } from 'lucide-react'
import { Button } from '@/components'
import { texts } from '@/config'
import { downloadCsv, type CsvCell } from './csv'

/** ปุ่มพิมพ์ (หน้าพิมพ์ซ่อนเมนู / ตัวกรองด้วย print:hidden) และส่งออก CSV — ไม่แสดงในหน้าพิมพ์ */
export function ReportActions({ rows, filename }: { rows: CsvCell[][]; filename: string }) {
  return (
    <div className="flex gap-2 print:hidden">
      <Button variant="secondary" onClick={() => window.print()}>
        <Printer aria-hidden />
        {texts.reports.print}
      </Button>
      <Button variant="secondary" onClick={() => downloadCsv(filename, rows)}>
        <Download aria-hidden />
        {texts.reports.exportCsv}
      </Button>
    </div>
  )
}
