import { texts } from '@/config'
import type { SaleKind } from './types'

/**
 * ชื่อเอกสารขายที่พิมพ์ตามชนิด (ม.86/4):
 *   ขายสินค้า = ใบส่งของ / ใบแจ้งหนี้ / ใบกำกับภาษี (ไม่จด VAT = ใบส่งของ / ใบแจ้งหนี้)
 *   ใบแจ้งหนี้ค่าบริการ = ใบแจ้งหนี้ (ไม่ใช่ใบกำกับภาษี), asTaxInvoice = ใบกำกับภาษีที่ออกก่อนรับเงิน
 */
export function salePrintTitle(kind: SaleKind, vat: boolean, asTaxInvoice = false): string {
  const p = texts.sales.print
  if (kind === 'invoice') return vat ? p.deliveryTaxInvoice : p.deliveryInvoice
  return asTaxInvoice ? p.taxInvoiceOnly : p.invoice
}
