/** ตรงกับ backend GET /companies/{id}/dashboards/{app}?start_date=&end_date= — จำนวนเงินเป็น string */
type Ref = { id: string; code: string; name: string }
type Aging = { not_due: string; days_1_30: string; days_31_60: string; days_61_90: string; over_90: string }
type Period = { start_date: string; end_date: string }

export const dashboardApi = (c: string, app: 'sales' | 'purchasing' | 'inventory' | 'finance' | 'accounting') => `/companies/${c}/dashboards/${app}`

export type SalesDashboard = Period & {
  sales: { current: string; previous: string }
  invoice_count: number
  monthly: { month: string; net: string }[]
  /** ปีบัญชีของกราฟ (null = ไม่มีปีบัญชีครอบคลุมวันสิ้นช่วง — กราฟเป็น 12 เดือนล่าสุด) */
  fiscal_year: { year: number; start_date: string; end_date: string } | null
  /** ปีบัญชีก่อนหน้า เดือนต่อเดือนกับ monthly (null = ไม่มียอด) */
  previous_year: { month: string; net: string }[] | null
  quotations: { open_count: number; open_amount: string }
  orders: { draft_count: number; to_invoice_count: number }
  receivables: { outstanding: string; overdue: string; aging: Aging }
  top_customers: { customer: Ref; net: string }[]
}

export type PurchasingDashboard = Period & {
  purchases: { current: string; previous: string }
  invoice_count: number
  monthly: { month: string; net: string }[]
  orders: { draft_count: number; to_receive_count: number; to_invoice_count: number }
  payables: { outstanding: string; overdue: string; due_7_days: string; aging: Aging }
  top_suppliers: { supplier: Ref; net: string }[]
}

export type InventoryDashboard = Period & {
  stock_value: string
  stock_item_count: number
  warehouses: { warehouse: Ref; value: string }[]
  below_reorder_count: number
  below_reorder: { item: Ref; on_hand: string; reorder_point: string }[]
  movements: { receipt_count: number; issue_count: number }
  monthly: { month: string; in_value: string; out_value: string }[]
}

export type FinanceDashboard = Period & {
  cash_accounts: { account: Ref; type: 'cash' | 'bank'; balance: string }[]
  cash_total: string
  received: string
  paid: string
  net_flow: string
  transfer_count: number
  monthly: { month: string; received: string; paid: string }[]
  wht: { deducted: string; suffered: string }
  recent: { kind: 'receipt' | 'payment' | 'transfer'; id: string; document_no: string; date: string; party: string; amount: string }[]
}

type Income = { revenue: string; expense: string; net_income: string }

export type AccountingDashboard = Period & {
  income: Income
  previous: Income
  monthly: { month: string; revenue: string; expense: string }[]
  suspense_balance: string
  entries: { draft_count: number; posted_count: number }
  recent_entries: { id: string; entry_no: string; date: string; description: string; amount: string }[]
  period: { name: string; fiscal_year?: number; start_date: string; end_date: string; status: 'open' | 'closed' } | null
}
