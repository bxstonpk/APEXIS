import { companyPath } from '@/config/apps'
import { normalizeAmount, toString, toUnits } from '@/features/accounting/money'

/** ตรงกับ backend pettycash responses — จำนวนเงินเป็น string */
export type Ref = { id: string; code: string; name: string }
export type UserRef = { id: string; name: string }
export type DocRef = { id: string; document_no: string }
export type Totals = { net: string; vat: string; gross: string }

export type Fund = {
  id: string
  code: string
  name: string
  custodian: UserRef
  gl_account: Ref
  imprest_amount: string
  status: 'active' | 'inactive'
  funded: boolean
  cash_on_hand: string
  pending_replenishment: string
  requested: string
}

export type VoucherStatus = 'draft' | 'approved' | 'replenished' | 'cancelled'

export type VoucherLine = { line_no: number; account: Ref; description: string; amount: string; net: string }

export type Voucher = {
  id: string
  document_no: string
  fund: Ref
  date: string
  payee: { name: string; tax_id: string; branch_no: string }
  description: string
  lines: VoucherLine[]
  vat_rate: string
  price_includes_vat: boolean
  tax_invoice: { no: string; date: string } | null
  totals: Totals
  status: VoucherStatus
  replenishment: DocRef | null
  created_by: UserRef
  approved_by: UserRef | null
  approved_at: string | null
}

export type VoucherRow = {
  id: string
  document_no: string
  date: string
  fund: Ref
  payee_name: string
  description: string
  tax_invoice_no: string
  totals: Totals
  status: VoucherStatus
  replenishment: DocRef | null
}

export type RequestKind = 'establish' | 'increase' | 'replenish'
export type RequestStatus = 'requested' | 'paid' | 'cancelled'

export type Payment = { document_id: string; document_no: string; date: string }

export type Replenishment = {
  id: string
  document_no: string
  fund: Ref
  kind: RequestKind
  date: string
  description: string
  amount: string
  status: RequestStatus
  vouchers: VoucherRow[]
  payment: Payment | null
  created_by: UserRef
}

export type ReplenishmentRow = Omit<Replenishment, 'vouchers' | 'created_by'> & { vouchers: number }

export const FUND_QUERY_KEYS = ['q', 'status', 'sort', 'page', 'page_size'] as const
export const VOUCHER_QUERY_KEYS = ['q', 'fund_id', 'status', 'from', 'to', 'sort', 'page', 'page_size'] as const
export const REQUEST_QUERY_KEYS = ['q', 'fund_id', 'kind', 'status', 'from', 'to', 'sort', 'page', 'page_size'] as const

export const pcPerms = {
  fundRead: 'petty_cash.fund:read',
  fundManage: 'petty_cash.fund:manage',
  voucherRead: 'petty_cash.voucher:read',
  voucherManage: 'petty_cash.voucher:manage',
  voucherApprove: 'petty_cash.voucher:approve',
  requestRead: 'petty_cash.replenishment:read',
  requestPost: 'petty_cash.replenishment:post',
  reportRead: 'petty_cash.report:read',
} as const

export const pcApi = {
  funds: (c: string) => `/companies/${c}/petty-cash-funds`,
  vouchers: (c: string) => `/companies/${c}/petty-cash-vouchers`,
  requests: (c: string) => `/companies/${c}/petty-cash-replenishments`,
  reports: (c: string) => `/companies/${c}/reports`,
  accounts: (c: string) => `/companies/${c}/accounts`,
  members: (c: string) => `/companies/${c}/members`,
}

/** เมนูของแอปเงินสดย่อย: vouchers, replenishments, funds, reports */
export const pcPage = (c: string, menu: string) => companyPath(c, 'petty-cash', menu)

export const VOUCHER_TONES: Record<string, 'success' | 'neutral' | 'warning' | 'info'> = {
  draft: 'neutral',
  approved: 'info',
  replenished: 'success',
  cancelled: 'warning',
}

export const REQUEST_TONES: Record<string, 'success' | 'neutral' | 'warning' | 'info'> = {
  requested: 'info',
  paid: 'success',
  cancelled: 'warning',
}

const HUNDRED = 100_000_000n // 100 ในสเกล 10^6

/** ปัดหน่วยย่อย (สเกล 10^6) เป็นทศนิยม 2 ตำแหน่ง แบบปัดครึ่งขึ้น */
function round2(units: bigint): bigint {
  const q = 10_000n
  return units >= 0n ? ((units + q / 2n) / q) * q : -((-units + q / 2n) / q) * q
}

/**
 * ยอดของใบสำคัญจ่าย — สูตรเดียวกับ backend: VAT คิดจากผลรวมทั้งใบ ปัด 2 ตำแหน่ง
 * (ราคารวม VAT: VAT = รวม × อัตรา / (100 + อัตรา); ราคาก่อน VAT: VAT = รวม × อัตรา / 100)
 */
export function voucherTotals(amounts: string[], rate: string, includesVat: boolean): Totals {
  const total = amounts.reduce((a, v) => a + toUnits(normalizeAmount(v) ?? '0'), 0n)
  const r = toUnits(normalizeAmount(rate) ?? '0')
  if (r <= 0n) return { net: toString(total), vat: '0', gross: toString(total) }
  if (includesVat) {
    const vat = round2((total * r) / (HUNDRED + r))
    return { net: toString(total - vat), vat: toString(vat), gross: toString(total) }
  }
  const vat = round2((total * r) / HUNDRED)
  return { net: toString(total), vat: toString(vat), gross: toString(total + vat) }
}

/** ผลรวมยอดรวมของใบสำคัญจ่ายที่เลือก (คำขอเบิกชดเชย) */
export function selectedTotal(rows: VoucherRow[], ids: ReadonlySet<string>): string {
  return toString(rows.filter((r) => ids.has(r.id)).reduce((a, r) => a + toUnits(r.totals.gross), 0n))
}
