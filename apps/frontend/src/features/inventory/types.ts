import { companyPath } from '@/config/apps'

/** ตรงกับ backend inventory responses — จำนวน / มูลค่าเป็น string */
export type Ref = { id: string; code: string; name: string }
export type Status = 'active' | 'inactive'

export type Uom = { id: string; code: string; name: string; status: Status }

export type Accounts = { purchase_account_id: string | null; inventory_account_id: string | null; revenue_account_id: string | null }

export type Category = Uom & Accounts

export type Warehouse = Uom & { branch_id: string | null }

export type ItemType = 'stock' | 'non_stock' | 'service'

export type Item = Accounts & {
  id: string
  code: string
  name: string
  name_en: string
  type: ItemType
  uom: Ref
  category: Ref | null
  reorder_point: string | null
  status: Status
}

export type DocType = 'receipt' | 'issue' | 'transfer' | 'adjustment'
export type DocStatus = 'draft' | 'posted' | 'cancelled'

export type StockDocSummary = {
  id: string
  type: DocType
  document_no: string | null
  date: string
  warehouse: Ref
  to_warehouse: Ref | null
  reference: string
  description: string
  status: DocStatus
  total_value: string
  posted_at: string | null
}

export type StockDocLine = { line_no: number; item: Ref; uom_code: string; quantity: string; unit_cost: string | null; value: string; description: string }

export type StockDoc = StockDocSummary & { created_by: string; cancelled_at: string | null; lines: StockDocLine[] }

export type InventorySettings = {
  default_purchase_account_id: string | null
  default_inventory_account_id: string | null
  cogs_account_id: string | null
  journal_id: string | null
  accounting_systems: { from_fiscal_year: number; system: 'perpetual' | 'periodic' }[]
}

export type Closing = {
  id: string
  fiscal_period_id: string
  fiscal_year: number
  period_end: string
  entry_id: string | null
  entry_no: string
  adjustment: string
  status: 'posted' | 'cancelled'
  created_at: string
  cancelled_at: string | null
}

export type StockOnHand = {
  book: string
  as_of: string
  rows: { item: Ref; uom_code: string; warehouse: Ref; quantity: string; value: string }[]
  total_value: string
}

export type StockCard = {
  book: string
  item: Ref
  uom_code: string
  warehouse: Ref | null
  start_date: string
  end_date: string
  opening_quantity: string
  opening_value: string
  lines: {
    date: string
    document_id: string
    document_no: string
    document_type: DocType
    warehouse: Ref
    quantity: string
    value: string
    balance_quantity: string
    balance_value: string
  }[]
}

export const MASTER_QUERY_KEYS = ['q', 'status', 'sort', 'page', 'page_size'] as const
export const ITEM_QUERY_KEYS = ['q', 'status', 'type', 'category_id', 'sort', 'page', 'page_size'] as const
export const DOC_QUERY_KEYS = ['q', 'type', 'status', 'warehouse_id', 'item_id', 'start_date', 'end_date', 'sort', 'page', 'page_size'] as const

export const invPerms = {
  uomRead: 'inventory.uom:read',
  uomManage: 'inventory.uom:manage',
  categoryRead: 'inventory.category:read',
  categoryManage: 'inventory.category:manage',
  itemRead: 'inventory.item:read',
  itemManage: 'inventory.item:manage',
  warehouseRead: 'inventory.warehouse:read',
  warehouseManage: 'inventory.warehouse:manage',
  documentRead: 'inventory.document:read',
  documentManage: 'inventory.document:manage',
  documentPost: 'inventory.document:post',
  reportRead: 'inventory.report:read',
  settingsRead: 'inventory.settings:read',
  settingsManage: 'inventory.settings:manage',
  closingRead: 'inventory.closing:read',
  closingPost: 'inventory.closing:post',
} as const

export const invApi = {
  uoms: (c: string) => `/companies/${c}/uoms`,
  categories: (c: string) => `/companies/${c}/item-categories`,
  warehouses: (c: string) => `/companies/${c}/warehouses`,
  items: (c: string) => `/companies/${c}/items`,
  documents: (c: string) => `/companies/${c}/stock-documents`,
  settings: (c: string) => `/companies/${c}/inventory-settings`,
  accountingSystem: (c: string, year: number) => `/companies/${c}/inventory-settings/accounting-systems/${year}`,
  closings: (c: string) => `/companies/${c}/inventory-closings`,
  reports: (c: string) => `/companies/${c}/reports`,
  branches: (c: string) => `/companies/${c}/branches`,
  accounts: (c: string) => `/companies/${c}/accounts`,
  journals: (c: string) => `/companies/${c}/journals`,
}

/** เมนูของแอปคลัง: documents, items, categories, uoms, warehouses, closing, reports, settings */
export const inventoryPage = (c: string, menu: string) => companyPath(c, 'inventory', menu)
