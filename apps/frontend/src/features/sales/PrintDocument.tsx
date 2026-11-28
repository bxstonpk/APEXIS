import { documentPrintData, type PrintLine, type PrintParty, type PrintTotal } from '@/features/print-templates/data'
import { TemplatedPrint } from '@/features/print-templates/TemplatedPrint'
import type { TemplateDocType } from '@/features/print-templates/types'
import { formatDate } from '@/lib/format'
import type { Letterhead } from './types'

/**
 * เอกสารพิมพ์ A4 — ประกอบข้อมูลพิมพ์ (DocumentPrintData) แล้วพิมพ์ผ่านแม่แบบของชนิดเอกสาร
 * (แม่แบบค่าเริ่มต้นของบริษัท หรือแม่แบบมาตรฐาน "เอกสารทั่วไป" — หัวกระดาษบริษัท, ผู้ซื้อ, ตารางรายการ, ยอดรวม + ตัวอักษร, ช่องลงนาม)
 * ใช้กับใบเสนอราคา / ใบแจ้งหนี้ / ใบกำกับภาษี (ม.86/4) / ใบเสร็จรับเงิน / ใบสั่งซื้อ / ใบลดหนี้ ...
 */

export type { PrintLine, PrintParty, PrintTotal }
export { branchLabel, letterheadAddress } from '@/features/print-templates/data'

export function PrintDocument({
  docType,
  letterhead,
  title,
  docNo,
  info,
  party,
  lines,
  totals,
  amountInWords,
  note,
  signatures,
}: {
  docType: TemplateDocType
  letterhead: Letterhead
  title: string
  docNo: string
  /** ข้อมูลด้านขวา (วันที่, ครบกำหนด, อ้างอิง ...) — เลขที่แสดงให้อัตโนมัติ */
  info: [string, string][]
  party: PrintParty
  lines: PrintLine[]
  totals: PrintTotal[]
  /** ยอดที่อ่านเป็นตัวอักษร (ปกติ = ยอดรวมทั้งสิ้น) */
  amountInWords: string
  note?: string
  signatures: string[]
}) {
  const data = documentPrintData({ letterhead, title, docNo, info, party, lines, totals, amountInWords, note, signatures })
  return <TemplatedPrint docType={docType} data={data} />
}

export const printDate = (d: string | null | undefined) => (d ? formatDate(d) : '')
