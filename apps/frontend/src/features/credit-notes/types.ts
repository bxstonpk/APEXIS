import { texts } from '@/config'

/** ใบลดหนี้ (ขาย / จัดซื้อ) — ชนิดและรูปแบบใบกำกับต้นทางที่ฟอร์มใช้ */
export type CreditKind = 'return' | 'allowance'

type Amounts = { net: string; vat: string; gross: string }
type ItemRef = { id: string; code: string; name: string }

/** ใบกำกับภาษีต้นทางในรูปที่ฟอร์มใบลดหนี้ใช้ (แปลงจาก sales invoice / supplier invoice) */
export type CreditableInvoice = {
  id: string
  no: string
  party: string
  vat_rate: string
  price_includes_vat: boolean
  outstanding: string
  lines: {
    line_no: number
    item: ItemRef
    quantity: string
    unit_price: string
    amounts: Amounts
    returned_quantity: string
    credited_amount: string
  }[]
}

type Ref = { id: string; code: string; name: string }
type Party = { name: string; tax_id: string; branch_no: string; address: string }
export type CreditStatus = 'posted' | 'cancelled'

export type CreditNoteLine = {
  line_no: number
  invoice_line_no: number
  item: Ref
  /** ลดราคา = "0" */
  quantity: string
  /** ลดราคา = จำนวนเงินที่ลด */
  unit_price: string
  amounts: Amounts
  account_id: string
  cost: string
}

/** GET /sales-credit-notes */
export type SalesCreditNoteSummary = {
  id: string
  document_no: string
  date: string
  /** kind = ชนิดใบต้นทาง: invoice = ขายสินค้า, service_invoice = ใบแจ้งหนี้ค่าบริการ */
  invoice: { id: string; document_no: string; kind?: 'invoice' | 'service_invoice' }
  /** output = ลดภาษีขาย (ใบลดหนี้ / ใบกำกับภาษี), undue = ลดภาษีขายยังไม่ถึงกำหนด (ใบลดหนี้อย่างเดียว ไม่เข้ารายงานภาษีขาย) */
  vat_kind?: 'output' | 'undue'
  customer: Ref
  kind: CreditKind
  reason: string
  totals: Amounts
  status: CreditStatus
  entry_id: string | null
  entry_no: string
  cancelled_at: string | null
}

/** GET /sales-credit-notes/{id} — มูลค่าเดิม / ที่ถูกต้อง / ผลต่าง ตาม ม.86/10 */
export type SalesCreditNote = SalesCreditNoteSummary & {
  warehouse_id: string | null
  accounting_system: string
  stock_document_no: string
  invoice_date: string
  original_totals: Amounts
  credited_before: Amounts
  correct_totals: Amounts
  bill_to: Party
  vat_rate: string
  price_includes_vat: boolean
  description: string
  created_by: string
  lines: CreditNoteLine[]
}

/** GET /supplier-credit-notes */
export type SupplierCreditNoteSummary = {
  id: string
  document_no: string
  date: string
  supplier: Ref
  invoice: { id: string; document_no: string; supplier_invoice_no: string }
  kind: CreditKind
  /** goods = ซื้อสินค้า, service = ซื้อบริการ */
  invoice_kind?: 'goods' | 'service'
  /** output = ลดภาษีซื้อ, undue = ลดภาษีซื้อยังไม่ถึงกำหนด */
  vat_kind?: 'output' | 'undue'
  supplier_credit_note_no: string
  credit_note_date: string
  reason: string
  totals: Amounts
  status: CreditStatus
  entry_id: string | null
  entry_no: string
  cancelled_at: string | null
}

export type SupplierCreditNote = SupplierCreditNoteSummary & {
  bill_from: Party
  description: string
  vat_rate: string
  price_includes_vat: boolean
  warehouse_id: string | null
  accounting_system: string
  stock_document_no: string
  created_by: string
  lines: (CreditNoteLine & { stock: boolean; price_variance: string })[]
}

/** ชื่อใบลดหนี้ขายที่พิมพ์ — มี VAT และลดภาษีขาย = ใบลดหนี้ / ใบกำกับภาษี; ไม่มี VAT หรือภาษียังไม่ถึงกำหนด = ใบลดหนี้ */
export function creditNotePrintTitle(vat: boolean, vatKind: SalesCreditNote['vat_kind']): string {
  const p = texts.creditNotes.print
  return vat && vatKind !== 'undue' ? p.titleVat : p.title
}

export const CREDIT_QUERY_KEYS = ['q', 'status', 'customer_id', 'supplier_id', 'invoice_id', 'start_date', 'end_date', 'sort', 'page', 'page_size'] as const
