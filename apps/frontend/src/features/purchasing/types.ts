import { companyPath } from '@/config/apps'
import type { ThaiAddress } from '@/features/partners/thaiAddress'

/** ตรงกับ backend purchasing responses */
export type Supplier = {
  id: string
  code: string
  name: string
  tax_id: string
  branch_no: string
  address: string
  address_detail: ThaiAddress
  payment_term_days: number
  vat_registered: boolean
  tax_type: 'juristic' | 'individual'
  status: 'active' | 'inactive'
  /** สาขาเพิ่มเติม — มีเฉพาะ GET รายตัว */
  branches?: { branch_no: string; name: string; address: string; address_detail: ThaiAddress }[]
}

export const SUPPLIER_QUERY_KEYS = ['q', 'status', 'sort', 'page', 'page_size'] as const

export const perms = {
  supplierManage: 'purchasing.supplier:manage',
} as const

export const purchasingApi = {
  suppliers: (c: string) => `/companies/${c}/suppliers`,
}

export const purchasingPage = (c: string, menu: string) => companyPath(c, 'purchasing', menu)

// ---------- เอกสารซื้อ (ตรงกับ backend purchasing responses — จำนวนเงิน / จำนวนเป็น string) ----------

export type Ref = { id: string; code: string; name: string }
export type Totals = { net: string; vat: string; gross: string }

export type PurchasingSettings = {
  vat_rate: string
  journal_id: string | null
  grir_account_id: string | null
  ap_account_id: string | null
  input_vat_account_id: string | null
  price_variance_account_id: string | null
  purchase_returns_account_id: string | null
  undue_input_vat_account_id: string | null
  service_expense_account_id: string | null
}

/** สายเอกสารจัดซื้อ: goods = ซื้อสินค้า, service = จ้างบริการ (backend แยกด้วย kind) */
export type PurchaseKind = 'goods' | 'service'

export type PurchaseOrderSummary = {
  kind?: PurchaseKind
  id: string
  order_no: string | null
  date: string
  delivery_date: string | null
  supplier: Ref
  supplier_branch_no: string
  reference: string
  vat_rate: string
  price_includes_vat: boolean
  totals: Totals
  status: 'draft' | 'approved' | 'closed' | 'cancelled'
}

export type PurchaseOrderLine = {
  line_no: number
  item: Ref
  description: string
  quantity: string
  unit_price: string
  amounts: Totals
  received_quantity: string
  invoiced_quantity: string
}

export type PurchaseOrder = PurchaseOrderSummary & {
  description: string
  vat_registered: boolean
  payment_term_days: number
  created_by: string
  approved_at: string | null
  closed_at: string | null
  cancelled_at: string | null
  lines: PurchaseOrderLine[]
}

export type PostedDoc = {
  kind?: PurchaseKind
  id: string
  document_no: string
  date: string
  order: { id: string; order_no: string }
  supplier: Ref
  status: 'posted' | 'cancelled'
  entry_id: string | null
  entry_no: string
  cancelled_at: string | null
}

export type GoodsReceipt = PostedDoc & {
  warehouse_id: string | null
  stock_document_no: string
  accounting_system: string
  total_value: string
  reference: string
  description: string
  lines: { order_line_no: number; item: Ref; quantity: string; unit_cost: string; value: string; account_id: string }[]
}

export type SupplierInvoiceSummary = PostedDoc & {
  supplier_invoice_no: string
  invoice_date: string
  due_date: string
  totals: Totals
  /** จ่ายแล้ว (ใบสำคัญจ่าย) */
  paid_amount: string
  /** ลดหนี้แล้ว (รวม VAT) — outstanding = ยอดรวม − จ่าย − ลดหนี้ */
  credited_amount: string
  outstanding: string
  /** ภาษีซื้อ: claimed = ใช้สิทธิแล้ว, undue = รอใบกำกับ, partial = ได้รับบางส่วน, none = ไม่มี VAT */
  vat_status?: 'claimed' | 'undue' | 'partial' | 'none'
  vat_claimed?: string
  vat_undue?: string
  tax_invoice_no?: string
  tax_invoice_date?: string | null
}

/** ใบกำกับภาษีที่ได้รับจากผู้ขายภายหลัง (ซื้อบริการ) */
export type ReceivedTaxInvoice = {
  id: string
  tax_invoice_no: string
  tax_invoice_date: string
  date: string
  base: string
  vat: string
  entry_id: string | null
  entry_no: string
  status: 'posted' | 'cancelled'
  cancelled_at: string | null
}

export type SupplierInvoice = SupplierInvoiceSummary & {
  tax_invoices?: ReceivedTaxInvoice[]
  /** ภาษีตั้งต้นของใบกำกับถัดไป (0 = ไม่มีที่รอ) */
  vat_suggested?: string
  bill_from: { name: string; tax_id: string; branch_no: string; address: string }
  description: string
  vat_rate: string
  price_includes_vat: boolean
  lines: {
    line_no: number
    order_line_no: number
    item: Ref
    quantity: string
    unit_price: string
    amounts: Totals
    grir_cleared: string
    price_variance: string
    returned_quantity: string
    credited_amount: string
  }[]
}

export type PurchasingOutstandingRow = {
  order: { id: string; order_no: string }
  date: string
  delivery_date: string | null
  supplier: Ref
  line_no: number
  item: Ref
  quantity: string
  received_quantity: string
  invoiced_quantity: string
  to_receive: string
  to_invoice: string
}

export type ApAging = {
  book: string
  as_of: string
  rows: { supplier: Ref; not_due: string; days_1_30: string; days_31_60: string; days_61_90: string; over_90: string; total: string }[]
  totals: { not_due: string; days_1_30: string; days_31_60: string; days_61_90: string; over_90: string; total: string }
}

export const PURCHASE_DOC_QUERY_KEYS = ['q', 'status', 'supplier_id', 'start_date', 'end_date', 'sort', 'page', 'page_size'] as const

/** GET /reports?book=input-vat — ใบลดหนี้ผู้ขายเป็นยอดติดลบ */
export type InputVatRow = {
  date: string
  document_id: string
  document_no: string
  kind: 'invoice' | 'service_invoice_with_tx' | 'tax_invoice' | 'credit_note'
  tax_invoice_no: string
  tax_invoice_date: string
  reference_no: string
  supplier: Ref
  tax_id: string
  branch_no: string
  base: string
  tax: string
}
export type InputVatReport = { book: string; start_date: string; end_date: string; rows: InputVatRow[]; total_base: string; total_tax: string }

/** GET /reports?book=undue-input-vat */
export type UndueInputVatReport = {
  book: string
  end_date: string
  rows: {
    invoice: { id: string; document_no: string; supplier_invoice_no: string }
    date: string
    supplier: Ref
    gross: string
    vat_total: string
    vat_claimed: string
    vat_credited: string
    vat_undue: string
  }[]
  totals: { gross: string; vat_total: string; vat_claimed: string; vat_credited: string; vat_undue: string }
}

/**
 * สายเอกสาร — หน้า / API / สิทธิ์ / ข้อความของสายซื้อสินค้า (ใบสั่งซื้อ -> ใบรับสินค้า -> ซื้อสินค้า)
 * และสายจ้างบริการ (ใบสั่งจ้าง -> ตรวจรับงาน -> ซื้อบริการ) — หน้าเดียวกันรับ flow
 */
export type PurchaseFlow = {
  kind: PurchaseKind
  /** เมนู (path ต่อจาก /purchasing/) */
  menu: { orders: string; receipts: string; invoices: string }
  api: { orders: (c: string) => string; receipts: (c: string) => string; invoices: (c: string) => string }
  perms: { orderRead: string; orderManage: string; orderApprove: string; receiptRead: string; receiptPost: string; invoiceRead: string; invoicePost: string }
  /** ชนิดสินค้าที่เลือกได้ (query type=) */
  itemTypes: string
  /** ชนิดเอกสารของแต่ละขั้นในรายการรวม "เอกสารซื้อ" (?type=) */
  docTypes: { orders: PurchaseDocType; receipts: PurchaseDocType; invoices: PurchaseDocType }
}

// ---------- รายการรวม "เอกสารซื้อ" (GET /purchase-documents) ----------

export type PurchaseDocType = 'order' | 'receipt' | 'invoice' | 'work_order' | 'service_acceptance' | 'service_invoice'

export const PURCHASE_DOC_TYPES: readonly PurchaseDocType[] = ['order', 'receipt', 'invoice', 'work_order', 'service_acceptance', 'service_invoice']

/** total = null สำหรับใบรับ / ตรวจรับงาน; outstanding / vat_status เฉพาะใบแจ้งหนี้ */
export type PurchaseDocumentRow = {
  type: PurchaseDocType
  id: string
  document_no: string
  date: string
  supplier: Ref
  status: string
  reference: string
  total: string | null
  outstanding: string | null
  vat_status: string | null
}

export const PURCHASE_DOCUMENTS_QUERY_KEYS = ['type', ...PURCHASE_DOC_QUERY_KEYS] as const

/** หน้าเอกสาร (path ต่อจาก /purchasing/) ของแต่ละชนิด */
export const docTypeMenu: Record<PurchaseDocType, string> = {
  order: 'orders',
  receipt: 'receipts',
  invoice: 'invoices',
  work_order: 'work-orders',
  service_acceptance: 'service-acceptances',
  service_invoice: 'service-invoices',
}

/** สถานะแบบใบสั่ง (ร่าง / อนุมัติ / ปิด) หรือแบบเอกสารที่บันทึกแล้ว */
export const isOrderType = (t: PurchaseDocType) => t === 'order' || t === 'work_order'

/**
 * ลิงก์กลับรายการ "เอกสารซื้อ" จากหน้าเอกสาร — คงตัวกรองเดิมจาก ?back= (รวม type);
 * ไม่มี type เดิม = กรองชนิดของเอกสารนี้
 */
export function documentsHref(companyId: string, type: PurchaseDocType, back = ''): string {
  const from = new URLSearchParams(back)
  const q = new URLSearchParams({ type: from.get('type') || type })
  for (const k of PURCHASE_DOC_QUERY_KEYS) {
    const v = from.get(k)
    if (v) q.set(k, v)
  }
  return `${purchasingPage(companyId, 'documents')}?${q}`
}

export const flows: Record<PurchaseKind, PurchaseFlow> = {
  goods: {
    kind: 'goods',
    menu: { orders: 'orders', receipts: 'receipts', invoices: 'invoices' },
    api: {
      orders: (c) => `/companies/${c}/purchase-orders`,
      receipts: (c) => `/companies/${c}/goods-receipts`,
      invoices: (c) => `/companies/${c}/supplier-invoices`,
    },
    perms: {
      orderRead: 'purchasing.order:read',
      orderManage: 'purchasing.order:manage',
      orderApprove: 'purchasing.order:approve',
      receiptRead: 'purchasing.receipt:read',
      receiptPost: 'purchasing.receipt:post',
      invoiceRead: 'purchasing.invoice:read',
      invoicePost: 'purchasing.invoice:post',
    },
    itemTypes: 'stock,non_stock',
    docTypes: { orders: 'order', receipts: 'receipt', invoices: 'invoice' },
  },
  service: {
    kind: 'service',
    menu: { orders: 'work-orders', receipts: 'service-acceptances', invoices: 'service-invoices' },
    api: {
      orders: (c) => `/companies/${c}/work-orders`,
      receipts: (c) => `/companies/${c}/service-acceptances`,
      invoices: (c) => `/companies/${c}/supplier-service-invoices`,
    },
    perms: {
      orderRead: 'purchasing.work_order:read',
      orderManage: 'purchasing.work_order:manage',
      orderApprove: 'purchasing.work_order:approve',
      receiptRead: 'purchasing.service_acceptance:read',
      receiptPost: 'purchasing.service_acceptance:post',
      invoiceRead: 'purchasing.service_invoice:read',
      invoicePost: 'purchasing.service_invoice:post',
    },
    itemTypes: 'service',
    docTypes: { orders: 'work_order', receipts: 'service_acceptance', invoices: 'service_invoice' },
  },
}

export const docPerms = {
  orderRead: 'purchasing.order:read',
  orderManage: 'purchasing.order:manage',
  orderApprove: 'purchasing.order:approve',
  receiptRead: 'purchasing.receipt:read',
  receiptPost: 'purchasing.receipt:post',
  invoiceRead: 'purchasing.invoice:read',
  invoicePost: 'purchasing.invoice:post',
  settingsRead: 'purchasing.settings:read',
  settingsManage: 'purchasing.settings:manage',
  reportRead: 'purchasing.report:read',
} as const

export const purchasingDocsApi = {
  documents: (c: string) => `/companies/${c}/purchase-documents`,
  orders: (c: string) => `/companies/${c}/purchase-orders`,
  receipts: (c: string) => `/companies/${c}/goods-receipts`,
  invoices: (c: string) => `/companies/${c}/supplier-invoices`,
  creditNotes: (c: string) => `/companies/${c}/supplier-credit-notes`,
  settings: (c: string) => `/companies/${c}/purchasing-settings`,
  reports: (c: string) => `/companies/${c}/reports`,
  items: (c: string) => `/companies/${c}/items`,
  warehouses: (c: string) => `/companies/${c}/warehouses`,
  accounts: (c: string) => `/companies/${c}/accounts`,
  journals: (c: string) => `/companies/${c}/journals`,
  letterhead: (c: string) => `/me/companies/${c}/letterhead`,
}
