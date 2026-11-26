import { companyPath } from '@/config/apps'
import type { ThaiAddress } from '@/features/partners/thaiAddress'

/** ตรงกับ backend sales responses — จำนวนเงิน / จำนวนเป็น string */
export type Ref = { id: string; code: string; name: string }
export type Totals = { net: string; vat: string; gross: string }

export type Customer = {
  id: string
  code: string
  name: string
  tax_id: string
  branch_no: string
  address: string
  address_detail: ThaiAddress
  payment_term_days: number
  tax_type: 'juristic' | 'individual'
  credit_limit: string | null
  status: 'active' | 'inactive'
  /** สาขาเพิ่มเติม — มีเฉพาะ GET รายตัว */
  branches?: PartyBranch[]
}

export type PartyBranch = { branch_no: string; name: string; address: string; address_detail: ThaiAddress }

export type SalesSettings = {
  vat_registered: boolean
  vat_rate: string
  journal_id: string | null
  ar_account_id: string | null
  output_vat_account_id: string | null
  revenue_account_id: string | null
  cogs_account_id: string | null
  sales_returns_account_id: string | null
}

export type SalesDocSummary = {
  id: string
  document_no: string | null
  date: string
  customer: Ref
  customer_branch_no: string
  reference: string
  vat_rate: string
  price_includes_vat: boolean
  totals: Totals
  status: string
}

export type SalesLine = { line_no: number; item: Ref; description: string; quantity: string; unit_price: string; amounts: Totals }
export type OrderLine = SalesLine & { delivered_quantity: string; invoiced_quantity: string }

export type Quotation = SalesDocSummary & {
  valid_until: string
  description: string
  payment_term_days: number
  order_id: string | null
  created_by: string
  lines: SalesLine[]
}

export type Order = SalesDocSummary & {
  quotation_id: string | null
  delivery_date: string | null
  description: string
  payment_term_days: number
  created_by: string
  approved_at: string | null
  closed_at: string | null
  cancelled_at: string | null
  lines: OrderLine[]
  warnings?: string[]
}

export type PostedSummary = {
  id: string
  document_no: string
  date: string
  order?: { id: string; order_no: string }
  customer: Ref
  status: 'posted' | 'cancelled'
  entry_id: string | null
  entry_no: string
  cancelled_at: string | null
}

export type Delivery = PostedSummary & {
  warehouse_id: string | null
  accounting_system: string
  stock_document_no: string
  total_cost: string
  reference: string
  description: string
  lines: { order_line_no: number; item: Ref; quantity: string; cost: string }[]
}

/** received_amount = รับเงินแล้ว (ใบเสร็จ), credited_amount = ลดหนี้แล้ว (รวม VAT), outstanding = ยอดรวม − รับ − ลดหนี้ */
export type InvoiceSummary = PostedSummary & { with_delivery: boolean; due_date: string; totals: Totals; received_amount: string; credited_amount: string; outstanding: string }

export type Invoice = InvoiceSummary & {
  /** ผู้ซื้อตามใบกำกับภาษี ณ วันออก */
  bill_to: { name: string; tax_id: string; branch_no: string; address: string }
  warehouse_id: string | null
  stock_document_no: string
  vat_rate: string
  price_includes_vat: boolean
  reference: string
  description: string
  lines: {
    line_no: number
    order_line_no: number
    item: Ref
    quantity: string
    unit_price: string
    amounts: Totals
    revenue_account_id: string
    cost: string
    /** คืนแล้ว (ใบลดหนี้ชนิดรับคืน) */
    returned_quantity: string
    /** ลดหนี้แล้ว (ก่อน VAT) */
    credited_amount: string
  }[]
}

/** GET /me/companies/{id}/letterhead */
export type Letterhead = {
  name: string
  legal_name_th: string
  legal_name_en: string
  tax_id: string
  address: { line: string; sub_district: string; district: string; province: string; postal_code: string; country: string }
  phone: string
  email: string
  website: string
}

export const DOC_QUERY_KEYS = ['q', 'status', 'customer_id', 'start_date', 'end_date', 'sort', 'page', 'page_size'] as const
export const CUSTOMER_QUERY_KEYS = ['q', 'status', 'sort', 'page', 'page_size'] as const

export const perms = {
  customerManage: 'sales.customer:manage',
  quotationManage: 'sales.quotation:manage',
  orderManage: 'sales.order:manage',
  orderApprove: 'sales.order:approve',
  deliveryPost: 'sales.delivery:post',
  invoicePost: 'sales.invoice:post',
  settingsRead: 'sales.settings:read',
  settingsManage: 'sales.settings:manage',
} as const

export const salesApi = {
  customers: (c: string) => `/companies/${c}/customers`,
  quotations: (c: string) => `/companies/${c}/quotations`,
  orders: (c: string) => `/companies/${c}/sales-orders`,
  deliveries: (c: string) => `/companies/${c}/deliveries`,
  invoices: (c: string) => `/companies/${c}/sales-invoices`,
  creditNotes: (c: string) => `/companies/${c}/sales-credit-notes`,
  settings: (c: string) => `/companies/${c}/sales-settings`,
  reports: (c: string) => `/companies/${c}/reports`,
  items: (c: string) => `/companies/${c}/items`,
  warehouses: (c: string) => `/companies/${c}/warehouses`,
  accounts: (c: string) => `/companies/${c}/accounts`,
  journals: (c: string) => `/companies/${c}/journals`,
  letterhead: (c: string) => `/me/companies/${c}/letterhead`,
}

export const salesPage = (c: string, menu: string) => companyPath(c, 'sales', menu)

// ---------- รายงาน (GET /reports?book=) ----------

export type OutstandingRow = {
  order: { id: string; order_no: string }
  date: string
  delivery_date: string | null
  customer: Ref
  line_no: number
  item: Ref
  quantity: string
  delivered_quantity: string
  invoiced_quantity: string
  to_deliver: string
  to_invoice: string
}

export type AgingAmounts = { not_due: string; days_1_30: string; days_31_60: string; days_61_90: string; over_90: string; total: string }
export type Aging = { book: string; as_of: string; rows: (AgingAmounts & { customer: Ref })[]; totals: AgingAmounts }

/** kind = credit_note: ยอดติดลบ, reference_no = เลขที่ใบกำกับเดิม */
export type TaxRow = {
  date: string
  document_id: string
  document_no: string
  kind: 'invoice' | 'credit_note'
  reference_no: string
  customer: Ref
  tax_id: string
  branch_no: string
  tax_type: string
  base: string
  tax: string
}
export type TaxReport = { book: string; start_date: string; end_date: string; rows: TaxRow[]; total_base: string; total_tax: string }
