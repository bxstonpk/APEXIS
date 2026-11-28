import { companyPath } from '@/config/apps'

/** ตรงกับ backend finance responses — จำนวนเงินเป็น string */
export type Ref = { id: string; code: string; name: string }

export type Party = {
  id: string | null
  code: string
  name: string
  tax_id: string
  branch_no: string
  address: string
  tax_type: string
}

export type Method = { cash_account: Ref; gl_account_id: string; amount: string; reference: string }

export type DocStatus = 'posted' | 'cancelled'

export type DocumentSummary = {
  id: string
  document_no: string
  date: string
  kind: string
  party: Party
  amount: string
  wht: string
  net: string
  status: DocStatus
  entry_id: string | null
  entry_no: string
  cancelled_at: string | null
  description: string
}

export type ReceiptLine = { invoice: Ref | null; account_id: string | null; description: string; amount: string; wht: string }
export type PaymentLine = {
  invoice: Ref | null
  account_id: string | null
  description: string
  amount: string
  wht_type: (Ref & { income_type: IncomeType }) | null
  wht_rate: string
  wht_base: string
  wht: string
}

/** ภาษีขายที่ใบรับเงินทำให้เกิด (รับเงินค่าบริการที่ภาษียังไม่ถึงกำหนด) — ต่อใบขายบริการ */
export type ReceiptVatLine = { invoice: { id: string; document_no: string }; base: string; vat: string }

/** tax_invoice = ใบนี้เป็น "ใบเสร็จรับเงิน / ใบกำกับภาษี" ด้วย (มี vat_lines) */
export type Receipt = DocumentSummary & { created_by: string; lines: ReceiptLine[]; methods: Method[]; tax_invoice?: boolean; vat_lines?: ReceiptVatLine[] }
export type Payment = DocumentSummary & { created_by: string; lines: PaymentLine[]; methods: Method[]; wht_certificate_no: string | null }

export type Transfer = {
  id: string
  document_no: string
  date: string
  from: Ref
  to: Ref
  amount: string
  reference: string
  description: string
  status: DocStatus
  entry_id: string | null
  entry_no: string
  cancelled_at: string | null
}

export type CashAccount = {
  id: string
  code: string
  name: string
  type: 'cash' | 'bank'
  bank_name: string
  bank_branch: string
  account_no: string
  gl_account_id: string
  status: 'active' | 'inactive'
}

/** ประเภทเงินได้ตามแบบ 50 ทวิ (มาตรา 40 / 3 เตรส) — ตรงกับ backend finance */
export const INCOME_TYPES = ['40_1', '40_2', '40_3', '40_4a', '40_4b', '3_tres', 'other'] as const
export type IncomeType = (typeof INCOME_TYPES)[number]

export type WhtType = { id: string; code: string; name: string; rate: string; income_type: IncomeType; status: 'active' | 'inactive' }

export type FinanceSettings = {
  receipt_journal_id: string | null
  payment_journal_id: string | null
  transfer_journal_id: string | null
  wht_received_account_id: string | null
  wht_pnd3_account_id: string | null
  wht_pnd53_account_id: string | null
}

export type OpenInvoice = {
  id: string
  document_no: string
  partner_ref: string
  date: string
  due_date: string
  net: string
  gross: string
  outstanding: string
  /** ขาย: invoice = ขายสินค้า, service_invoice = ใบแจ้งหนี้ค่าบริการ, deposit = ใบรับเงินมัดจำ; จัดซื้อ: goods = ซื้อสินค้า, service = ซื้อบริการ */
  kind?: 'invoice' | 'service_invoice' | 'deposit' | 'goods' | 'service'
  /** ภาษีขายยังไม่ถึงกำหนด — รับเงินครั้งนี้จะออกใบเสร็จรับเงิน / ใบกำกับภาษี */
  vat_undue?: string
  /** ออกใบกำกับภาษีก่อนรับเงินแล้ว */
  tax_invoice?: boolean
}

export type WithholdingReport = {
  book: string
  start_date: string
  end_date: string
  rows: {
    date: string
    document_id: string
    document_no: string
    party: Party
    tax_form: string
    wht_type: Ref | null
    rate: string
    base: string
    wht: string
  }[]
  total_base: string
  total_wht: string
}

/** GET /reports?book=vat-summary — สรุป ภ.พ.30 (ภาษีขาย − ภาษีซื้อ ในช่วงวันที่) */
export type VatTotals = { base: string; tax: string; documents: number }
export type VatSummary = {
  book: 'vat-summary'
  start_date: string
  end_date: string
  output: VatTotals
  input: VatTotals
  net_tax: string
  payable: string
  refundable: string
}

export const DOC_QUERY_KEYS = ['q', 'status', 'kind', 'cash_account_id', 'start_date', 'end_date', 'sort', 'page', 'page_size'] as const
export const MASTER_QUERY_KEYS = ['q', 'type', 'status', 'sort', 'page', 'page_size'] as const

export const perms = {
  receiptPost: 'finance.receipt:post',
  paymentPost: 'finance.payment:post',
  transferPost: 'finance.transfer:post',
  accountManage: 'finance.cash_account:manage',
  settingsManage: 'finance.settings:manage',
} as const

export const financeApi = {
  receipts: (c: string) => `/companies/${c}/receipts`,
  payments: (c: string) => `/companies/${c}/payments`,
  transfers: (c: string) => `/companies/${c}/cash-transfers`,
  cashAccounts: (c: string) => `/companies/${c}/cash-accounts`,
  whtTypes: (c: string) => `/companies/${c}/withholding-tax-types`,
  paymentWhtTypes: (c: string) => `/companies/${c}/payments/wht-types`,
  settings: (c: string) => `/companies/${c}/finance-settings`,
  reports: (c: string) => `/companies/${c}/reports`,
  customers: (c: string) => `/companies/${c}/customers`,
  suppliers: (c: string) => `/companies/${c}/suppliers`,
  accounts: (c: string) => `/companies/${c}/accounts`,
  journals: (c: string) => `/companies/${c}/journals`,
}

export const financePage = (c: string, menu: string) => companyPath(c, 'finance', menu)
