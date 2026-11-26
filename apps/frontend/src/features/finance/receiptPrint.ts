import { format, texts } from '@/config'
import { formatAmount, toString, toUnits } from '@/features/accounting/money'
import type { Receipt } from './types'

/**
 * ชื่อเอกสารพิมพ์ของใบรับเงิน — รับเงินค่าบริการที่ภาษียังไม่ถึงกำหนด (tax_invoice) = "ใบเสร็จรับเงิน / ใบกำกับภาษี" (ม.86/4),
 * อื่นๆ = "ใบเสร็จรับเงิน"
 */
export function receiptPrintTitle(r: Pick<Receipt, 'tax_invoice' | 'vat_lines'>): string {
  const p = texts.sales.print
  return r.tax_invoice && (r.vat_lines?.length ?? 0) > 0 ? p.receiptTax : p.receipt
}

/** ภาษีขายที่เกิดจากใบรับเงิน: ยอดรวมฐาน / ภาษี และบรรทัดอธิบายต่อใบแจ้งหนี้ (สำหรับหมายเหตุบนใบพิมพ์) */
export function receiptVatSummary(r: Pick<Receipt, 'vat_lines'>): { base: string; vat: string; notes: string[] } {
  const lines = r.vat_lines ?? []
  const base = lines.reduce((s, l) => s + toUnits(l.base), 0n)
  const vat = lines.reduce((s, l) => s + toUnits(l.vat), 0n)
  return {
    base: toString(base),
    vat: toString(vat),
    notes: lines.map((l) => format(texts.sales.print.vatBreakdown, { no: l.invoice.document_no, base: formatAmount(l.base), vat: formatAmount(l.vat) })),
  }
}
