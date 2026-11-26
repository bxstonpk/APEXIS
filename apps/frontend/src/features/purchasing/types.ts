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
}

export type PurchaseOrderSummary = {
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
}

export type SupplierInvoice = SupplierInvoiceSummary & {
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
  kind: 'invoice' | 'credit_note'
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
