/** แม่แบบเอกสารพิมพ์ — ชนิดข้อมูลร่วม (หน้าจัดการ / หน้าพิมพ์) */

export type Paper = 'a4_portrait' | 'a4_landscape' | 'a5_portrait'
export const PAPERS: Paper[] = ['a4_portrait', 'a4_landscape', 'a5_portrait']

/** ชนิดเอกสารที่พิมพ์ผ่านแม่แบบได้ — key ตรงกับ backend (vo.TemplateDocTypes) */
export const TEMPLATE_DOC_TYPES = [
  'sales.quotation',
  'sales.invoice',
  'sales.service_invoice',
  'sales.tax_invoice',
  'sales.deposit',
  'sales.billing_note',
  'sales.credit_note',
  'purchasing.order',
  'purchasing.work_order',
  'finance.receipt',
  'finance.wht_certificate',
  'finance.pnd3',
  'finance.pnd53',
  'finance.pp30',
  'finance.vat_report_output',
  'finance.vat_report_input',
  'manufacturing.production_order',
  'petty_cash.voucher',
  'petty_cash.replenishment',
] as const
export type TemplateDocType = (typeof TEMPLATE_DOC_TYPES)[number]

export const isTemplateDocType = (v: string): v is TemplateDocType => (TEMPLATE_DOC_TYPES as readonly string[]).includes(v)

/** แม่แบบของบริษัท (จาก API) — รายการไม่มี html */
export type DocumentTemplate = {
  id: string
  doc_type: TemplateDocType
  name: string
  html?: string
  paper: Paper
  is_default: boolean
  status: 'active' | 'inactive'
  created_at: string
  updated_at: string
}

export type DocumentAsset = { id: string; name: string; content_type: string; size: number; created_at: string }

// ---------- ข้อมูลพิมพ์ ----------

export type CompanyPrint = {
  name: string
  legal_name_th: string
  legal_name_en: string
  tax_id: string
  branch_label: string
  address: string
  phone: string
  email: string
  website: string
  /** โทร / อีเมล / เว็บ รวมเป็นบรรทัดเดียว */
  contact: string
}

export type PrintCell = { value: string; align?: 'left' | 'right' | 'center'; detail?: string; blank?: boolean }

/** เอกสารทั่วไป (ใบเสนอราคา / ขาย / ใบกำกับ / ใบเสร็จ / ใบสั่งซื้อ / ใบสั่งผลิต / เงินสดย่อย ...) */
export type DocumentPrintData = {
  company: CompanyPrint
  title: string
  copy_label: string
  document_no: string
  info: { label: string; value: string }[]
  party: { label: string; name: string; tax_id: string; branch_label: string; address: string; extra: string[] } | null
  columns: { label: string; align: 'left' | 'right' | 'center'; width: string }[]
  lines: { no: number; description: string; detail: string; quantity: string; unit_price: string; amount: string; cells: PrintCell[] }[]
  totals: { label: string; value: string; raw: string; strong: boolean }[]
  /** ยอดเป็นตัวอักษร เช่น หนึ่งพันบาทถ้วน (ว่าง = ไม่แสดง) */
  amount_in_words: string
  /** ยอดรวมทั้งสิ้น (ตัวเลข) */
  amount: string
  note: string
  signatures: { label: string }[]
  labels: Record<string, string>
}
