import { companyPath } from '@/config/apps'
import { appConfig } from '@/config/app'

/** แบบยื่นภาษีรายเดือน */
export const TAX_FORMS = ['pnd3', 'pnd53', 'pp30'] as const
export type TaxForm = (typeof TAX_FORMS)[number]
export const isTaxForm = (v: string): v is TaxForm => (TAX_FORMS as readonly string[]).includes(v)
export const isWithholdingForm = (f: TaxForm) => f === 'pnd3' || f === 'pnd53'

export type TaxFiling = {
  id: string
  form: TaxForm
  month: string // YYYY-MM
  filed_date: string
  amount: string
  surcharge: string
  penalty: string
  carry_forward: string
  total: string
  reference: string
  note: string
  payment_id: string | null
  status: 'filed' | 'cancelled'
  created_at: string
  cancelled_at: string | null
}

export type TaxOverviewCell = { amount: string; payees: number; computed: boolean; filing: TaxFiling | null }
export type TaxOverview = {
  year: number
  can_see_filings: boolean
  months: { month: string; pnd3: TaxOverviewCell; pnd53: TaxOverviewCell; pp30: TaxOverviewCell }[]
}

export type WhtFormLine = {
  seq: number
  payment_id: string
  document_no: string
  certificate_no: string
  date: string
  tax_id: string
  branch_no: string
  name: string
  address: string
  income_type: string
  wht_type_name: string
  rate: string
  base: string
  wht: string
  condition: number
}

export type WhtForm = {
  form: 'pnd3' | 'pnd53'
  month: string
  summary: { payees: number; lines: number; base: string; wht: string }
  lines: WhtFormLine[]
  filing: TaxFiling | null
}

export type VatAmounts = { base: string; tax: string; documents: number }
export type Pp30 = {
  month: string
  sales: VatAmounts
  zero_rated: string
  exempt: string
  purchases: VatAmounts
  carry_in: string
  net: string
  payable: string
  overpaid: string
  prev_filed: boolean
  filing: TaxFiling | null
}

export type VatReportRow = { seq: number; date: string; document_no: string; party_name: string; tax_id: string; branch_no: string; base: string; tax: string }
export type VatReport = { kind: 'output' | 'input'; month: string; rows: VatReportRow[]; base: string; tax: string }

export const taxApi = {
  overview: (c: string) => `/companies/${c}/tax-forms/overview`,
  withholding: (c: string) => `/companies/${c}/tax-forms/withholding`,
  pp30: (c: string) => `/companies/${c}/tax-forms/pp30`,
  vatReport: (c: string) => `/companies/${c}/tax-forms/vat-report`,
  filings: (c: string) => `/companies/${c}/tax-filings`,
  /** ลิงก์ดาวน์โหลดผ่าน proxy ของ Next (browser) */
  exportUrl: (c: string, form: string, month: string) =>
    `${appConfig.api.baseUrl}/companies/${c}/tax-forms/withholding/export?${new URLSearchParams({ form, month })}`,
}

export const taxPerms = { read: 'finance.tax_filing:read', manage: 'finance.tax_filing:manage' } as const

export const taxPage = (c: string) => companyPath(c, 'finance', 'tax-filings')
export const taxFormPage = (c: string, form: string, month: string) => `${taxPage(c)}/${form}/${month}`

const monthFormat = new Intl.DateTimeFormat('th-TH', { month: 'long', year: 'numeric', timeZone: 'UTC' })

/** "2026-11" -> "พฤศจิกายน 2569" */
export function monthLabel(month: string): string {
  const d = new Date(`${month}-01T00:00:00Z`)
  return Number.isNaN(d.getTime()) ? month : monthFormat.format(d)
}

/** เดือนถูกต้อง YYYY-MM */
export const isMonth = (v: string) => /^\d{4}-(0[1-9]|1[0-2])$/.test(v)

/** แบ่งรายการเป็นหน้า (ใบแนบ / รายงานพิมพ์) */
export function paginate<T>(rows: T[], size: number): T[][] {
  if (rows.length === 0) return [[]]
  const out: T[][] = []
  for (let i = 0; i < rows.length; i += size) out.push(rows.slice(i, i + size))
  return out
}
