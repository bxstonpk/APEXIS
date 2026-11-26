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
  /** ภาษีขายยังไม่ถึงกำหนด (ขายบริการ — ภาษีเกิดเมื่อรับเงิน) */
  undue_output_vat_account_id: string | null
  /** เงินมัดจำรับ */
  deposit_account_id: string | null
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

/** ชนิดสินค้า: service = บริการ (ภาษีเกิดเมื่อรับเงิน), stock / non_stock = สินค้า (ภาษีเกิดเมื่อส่งมอบ) */
export type ItemType = 'stock' | 'non_stock' | 'service'
export type ItemRef = Ref & { type?: ItemType }

export type SalesLine = { line_no: number; item: ItemRef; description: string; quantity: string; unit_price: string; amounts: Totals }
/** ใบสั่งขายติดตามเฉพาะจำนวนที่ออกเอกสารขายแล้ว (ขายสินค้า / ขายบริการ) */
export type OrderLine = SalesLine & { invoiced_quantity: string }

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

/**
 * เอกสารขายที่ลงบัญชีแล้ว — kind:
 *   invoice = ขายสินค้า (ใบส่งของ / ใบแจ้งหนี้ / ใบกำกับภาษี — ภาษีเกิดวันที่เอกสาร)
 *   service_invoice = ใบแจ้งหนี้ค่าบริการ (ภาษีขายยังไม่ถึงกำหนดจนรับเงินหรือออกใบกำกับภาษีก่อน)
 * เมนูตามประเภทธุรกิจ: ซื้อขาย / ผลิต = ขายสินค้า, บริการ / ก่อสร้าง = ใบแจ้งหนี้ — รับเงินทั้งหมดยืนยันที่แอปการเงิน
 */
export type SaleKind = 'invoice' | 'service_invoice'
/** output = ภาษีขายเกิดแล้วทั้งใบ, undue = ยังไม่ถึงกำหนด, partial = เกิดบางส่วน (รับเงินบางส่วน) */
export type VatStatus = 'output' | 'undue' | 'partial'
export type BillTo = { name: string; tax_id: string; branch_no: string; address: string }
export type DocRef = { id: string; document_no: string }

/** received_amount = รับเงินแล้ว (ใบเสร็จ), credited_amount = ลดหนี้แล้ว (รวม VAT), outstanding = ยอดรวม − รับ − ลดหนี้ − มัดจำที่หัก */
export type InvoiceSummary = PostedSummary & {
  kind?: SaleKind
  due_date: string
  totals: Totals
  received_amount: string
  credited_amount: string
  outstanding: string
  vat_status?: VatStatus
}

export type DepositApplied = { deposit: DocRef; amount: string; vat: string }

/** ประวัติภาษีขายที่เกิดของใบขายบริการ: ใบเสร็จ (ตามสัดส่วนที่รับ) หรือใบกำกับภาษีที่ออกก่อนรับเงิน */
export type VatRecognition = { kind: 'receipt' | 'tax_invoice'; document: DocRef; date: string; base: string; vat: string }

export type Invoice = InvoiceSummary & {
  /** ผู้ซื้อตามใบกำกับภาษี ณ วันออก */
  bill_to: BillTo
  warehouse_id: string | null
  stock_document_no: string
  vat_rate: string
  price_includes_vat: boolean
  reference: string
  description: string
  vat_recognized?: string
  vat_undue?: string
  deposits_applied?: DepositApplied[]
  /** ขายบริการ: ใบกำกับภาษีที่ออกก่อนรับเงิน */
  tax_invoice?: { no: string; date: string } | null
  vat_recognitions?: VatRecognition[]
  lines: {
    line_no: number
    order_line_no: number | null
    item: ItemRef
    description?: string
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

/**
 * GET /sales-deposits — ใบรับเงินมัดจำ / ใบกำกับภาษี (ฐาน = ก่อน VAT)
 * ลงบัญชีเป็นลูกหนี้แล้วรอยืนยันรับเงินที่การเงิน (outstanding = ยอดรวมที่ยังไม่รับ) — หักในเอกสารขายได้เมื่อรับเงินครบแล้วเท่านั้น
 */
export type Deposit = {
  id: string
  document_no: string
  date: string
  customer: Ref
  customer_branch_no?: string
  order?: { id: string; order_no: string } | null
  bill_to: BillTo
  description: string
  vat_rate: string
  price_includes_vat: boolean
  base: string
  vat: string
  gross: string
  applied_amount: string
  remaining_amount: string
  received_amount: string
  outstanding: string
  status: 'posted' | 'cancelled'
  entry_id: string | null
  entry_no: string
  cancelled_at: string | null
}

/** GET /sales-deposits/open?customer_id= — มัดจำที่รับเงินครบแล้วและยังหักได้ */
export type OpenDeposit = Pick<Deposit, 'id' | 'document_no' | 'date' | 'vat_rate' | 'base' | 'vat' | 'remaining_amount'>

export type BillingNoteInvoice = { id: string; document_no: string; kind: 'invoice' | 'service_invoice'; date: string; due_date: string; gross: string; outstanding: string }

/** GET /billing-notes — ใบวางบิล (ไม่มีผลทางบัญชี) */
export type BillingNote = {
  id: string
  document_no: string
  date: string
  due_date: string
  customer: Ref
  description: string
  status: 'open' | 'paid' | 'cancelled'
  invoices: BillingNoteInvoice[]
  total: string
  outstanding: string
  cancelled_at?: string | null
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

export const BILLING_QUERY_KEYS = ['q', 'status', 'customer_id', 'start_date', 'end_date', 'sort', 'page', 'page_size'] as const

export const perms = {
  customerManage: 'sales.customer:manage',
  quotationManage: 'sales.quotation:manage',
  orderManage: 'sales.order:manage',
  orderApprove: 'sales.order:approve',
  invoicePost: 'sales.invoice:post',
  billingNoteManage: 'sales.billing_note:manage',
  /** ยืนยันรับเงินที่แอปการเงิน (ลิงก์จากเอกสารขาย) */
  settingsRead: 'sales.settings:read',
  settingsManage: 'sales.settings:manage',
} as const

export const salesApi = {
  customers: (c: string) => `/companies/${c}/customers`,
  quotations: (c: string) => `/companies/${c}/quotations`,
  orders: (c: string) => `/companies/${c}/sales-orders`,
  invoices: (c: string) => `/companies/${c}/sales-invoices`,
  serviceInvoices: (c: string) => `/companies/${c}/service-invoices`,
  deposits: (c: string) => `/companies/${c}/sales-deposits`,
  billingNotes: (c: string) => `/companies/${c}/billing-notes`,
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

/** เมนูของเอกสารขายแต่ละชนิด (ใช้สร้างลิงก์ข้ามเอกสาร) */
export const SALE_MENU: Record<SaleKind, string> = { invoice: 'invoices', service_invoice: 'service-invoices' }
export const saleApi = (c: string, kind: SaleKind) => (kind === 'invoice' ? salesApi.invoices(c) : salesApi.serviceInvoices(c))

// ---------- รายงาน (GET /reports?book=) ----------

export type OutstandingRow = {
  order: { id: string; order_no: string }
  date: string
  delivery_date: string | null
  customer: Ref
  line_no: number
  item: Ref
  quantity: string
  invoiced_quantity: string
  to_invoice: string
}

export type AgingAmounts = { not_due: string; days_1_30: string; days_31_60: string; days_61_90: string; over_90: string; total: string }
export type Aging = { book: string; as_of: string; rows: (AgingAmounts & { customer: Ref })[]; totals: AgingAmounts }

/**
 * แถวรายงานภาษีขาย — เฉพาะเอกสารที่ภาษีเกิดแล้ว (tax point):
 *   invoice / deposit = วันที่เอกสาร, tax_invoice = ใบกำกับภาษีที่ออกก่อนรับเงินของใบแจ้งหนี้ค่าบริการ,
 *   receipt = ภาษีที่เกิดเมื่อรับเงินค่าบริการ, credit_note = ติดลบ
 * reference_no = ใบแจ้งหนี้ค่าบริการต้นทาง (tax_invoice / receipt) หรือใบกำกับเดิม (credit_note)
 */
export type TaxRowKind = 'invoice' | 'deposit' | 'tax_invoice' | 'receipt' | 'credit_note'
export type TaxRow = {
  date: string
  document_id: string
  document_no: string
  kind: TaxRowKind
  /** id ของเอกสารอ้างอิง (ถ้ามี) — ใช้ลิงก์ */
  reference_id?: string
  reference_no: string
  customer: Ref
  tax_id: string
  branch_no: string
  tax_type: string
  base: string
  tax: string
}
export type TaxReport = { book: string; start_date: string; end_date: string; rows: TaxRow[]; total_base: string; total_tax: string }

/** GET /reports?book=undue-output-vat — ใบขายบริการที่ยังมีภาษีขายไม่ถึงกำหนด ณ วันที่ */
export type UndueVatRow = {
  invoice: DocRef
  date: string
  customer: Ref
  gross: string
  vat_total: string
  vat_recognized: string
  vat_undue: string
}
export type UndueVatReport = {
  book: string
  end_date: string
  rows: UndueVatRow[]
  totals: { gross: string; vat_total: string; vat_recognized: string; vat_undue: string }
}
