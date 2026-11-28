import { companyPath } from '@/config/apps'
import { toString, toUnits } from '@/features/accounting/money'

/** ตรงกับ backend manufacturing responses — จำนวน / มูลค่าเป็น string */
export type Ref = { id: string; code: string; name: string }
export type ItemRef = Ref & { uom?: string }

export type BomComponent = { line_no: number; item: ItemRef; quantity: string; scrap_percent: string }
export type BomConversion = { line_no: number; description: string; account: Ref; amount: string }

export type Bom = {
  id: string
  code: string
  name: string
  item: ItemRef
  output_quantity: string
  status: 'active' | 'inactive'
  is_default: boolean
  components?: BomComponent[]
  conversion_costs?: BomConversion[]
  created_at?: string
  updated_at?: string
}

export type OrderStatus = 'draft' | 'released' | 'completed' | 'cancelled'

export type OrderMaterial = {
  line_no: number
  item: ItemRef
  required_quantity: string
  issued_quantity: string
  returned_quantity: string
  net_quantity: string
  cost: string
}

export type OrderMovement = {
  id: string
  kind: 'issue' | 'return' | 'receipt'
  date: string
  document_id: string
  document_no: string
  value: string
  entry_no: string
}

export type ProductionOrder = {
  id: string
  document_no: string | null
  date: string
  item: ItemRef
  bom: Ref
  quantity: string
  warehouse: Ref
  planned_start: string | null
  planned_end: string | null
  status: OrderStatus
  description: string
  materials?: OrderMaterial[]
  conversion_cost?: string
  movements?: OrderMovement[]
  costs?: { materials: string; conversion: string; total: string; unit_cost: string }
  completed_quantity: string | null
  completed_at: string | null
  cancelled_at: string | null
  cancel_reason: string
  created_by?: string
  created_at?: string
}

export type ManufacturingSettings = {
  journal_id: string | null
  wip_account_id: string | null
  conversion_account_id: string | null
  default_warehouse_id: string | null
}

export const BOM_QUERY_KEYS = ['q', 'item_id', 'status', 'sort', 'page', 'page_size'] as const
export const ORDER_QUERY_KEYS = ['q', 'status', 'item_id', 'from', 'to', 'sort', 'page', 'page_size'] as const

export const mfgPerms = {
  bomRead: 'manufacturing.bom:read',
  bomManage: 'manufacturing.bom:manage',
  orderRead: 'manufacturing.order:read',
  orderManage: 'manufacturing.order:manage',
  reportRead: 'manufacturing.report:read',
  settingsRead: 'manufacturing.settings:read',
  settingsManage: 'manufacturing.settings:manage',
} as const

export const mfgApi = {
  boms: (c: string) => `/companies/${c}/boms`,
  orders: (c: string) => `/companies/${c}/production-orders`,
  settings: (c: string) => `/companies/${c}/manufacturing-settings`,
  reports: (c: string) => `/companies/${c}/reports`,
  items: (c: string) => `/companies/${c}/items`,
  warehouses: (c: string) => `/companies/${c}/warehouses`,
  accounts: (c: string) => `/companies/${c}/accounts`,
  journals: (c: string) => `/companies/${c}/journals`,
}

/** เมนูของแอปการผลิต: orders, boms, reports, settings */
export const mfgPage = (c: string, menu: string) => companyPath(c, 'manufacturing', menu)

export const ORDER_TONES: Record<string, 'success' | 'neutral' | 'warning' | 'info'> = {
  draft: 'neutral',
  released: 'info',
  completed: 'success',
  cancelled: 'warning',
}

/** max(a − b, 0) ของทศนิยม (BigInt สเกล 10^6) */
export function remaining(a: string, b: string): string {
  const d = toUnits(a) - toUnits(b)
  return toString(d > 0n ? d : 0n)
}

/** เปอร์เซ็นต์ (ปัดลง 0–100) ของ part / whole */
export function percent(part: string, whole: string): number {
  const w = toUnits(whole)
  if (w <= 0n) return 0
  const p = Number((toUnits(part) * 100n) / w)
  return Math.max(0, Math.min(100, p))
}
