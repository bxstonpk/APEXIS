import {
  BookOpenCheck,
  Building2,
  Landmark,
  type LucideIcon,
  Package,
  ShoppingBag,
  ShoppingCart,
  UsersRound,
} from 'lucide-react'
import { can } from '@/lib/permissions'
import { texts } from './texts'

/**
 * แอปในหน้ารวมแอป (ไอคอนกลางจอ) และเมนูด้านบนของแต่ละแอป
 *
 * - กดไอคอน = เปิดแอปในแท็บใหม่ที่ /c/{companyId}/{app.key}/{menu.path}
 * - แอปแสดงเมื่อผู้ใช้มีสิทธิ์อย่างน้อยหนึ่งเมนู; เมนูแสดงเมื่อมีสิทธิ์ `permission` ของเมนูนั้น
 * - ชื่อแอป / เมนูอยู่ที่ texts.apps[app.key] (locales/th.ts); สีไอคอนเป็น token ใน styles/app.css
 * - เมนูแรกที่มีสิทธิ์ = หน้าแรกของแอป
 */
export type AppMenuItem = {
  /** key ใน texts.apps[app].menu และ path ต่อจาก /c/{companyId}/{app}/ */
  path: string
  permission: string
}

export type AppDefinition = {
  key: AppKey
  icon: LucideIcon
  /** class สีพื้นของไอคอน (token --color-app-*) */
  tone: string
  menu: readonly AppMenuItem[]
}

export type AppKey = keyof typeof texts.apps

export const apps: readonly AppDefinition[] = [
  {
    key: 'sales',
    icon: ShoppingCart,
    tone: 'bg-app-sales',
    menu: [
      { path: 'quotations', permission: 'sales.quotation:read' },
      { path: 'orders', permission: 'sales.order:read' },
      { path: 'deliveries', permission: 'sales.delivery:read' },
      { path: 'invoices', permission: 'sales.invoice:read' },
      { path: 'customers', permission: 'sales.customer:read' },
      { path: 'reports', permission: 'sales.report:read' },
      { path: 'settings', permission: 'sales.settings:read' },
    ],
  },
  {
    key: 'purchasing',
    icon: ShoppingBag,
    tone: 'bg-app-purchasing',
    menu: [
      { path: 'orders', permission: 'purchasing.order:read' },
      { path: 'receipts', permission: 'purchasing.receipt:read' },
      { path: 'invoices', permission: 'purchasing.invoice:read' },
      { path: 'suppliers', permission: 'purchasing.supplier:read' },
      { path: 'reports', permission: 'purchasing.report:read' },
      { path: 'settings', permission: 'purchasing.settings:read' },
    ],
  },
  {
    key: 'inventory',
    icon: Package,
    tone: 'bg-app-inventory',
    menu: [
      { path: 'documents', permission: 'inventory.document:read' },
      { path: 'items', permission: 'inventory.item:read' },
      { path: 'categories', permission: 'inventory.category:read' },
      { path: 'uoms', permission: 'inventory.uom:read' },
      { path: 'warehouses', permission: 'inventory.warehouse:read' },
      { path: 'closing', permission: 'inventory.closing:read' },
      { path: 'reports', permission: 'inventory.report:read' },
      { path: 'settings', permission: 'inventory.settings:read' },
    ],
  },
  {
    key: 'accounting',
    icon: BookOpenCheck,
    tone: 'bg-app-accounting',
    menu: [
      { path: 'entries', permission: 'ledger.entry:read' },
      { path: 'accounts', permission: 'ledger.account:read' },
      { path: 'journals', permission: 'ledger.journal:read' },
      { path: 'reports', permission: 'ledger.report:read' },
    ],
  },
  {
    key: 'finance',
    icon: Landmark,
    tone: 'bg-app-finance',
    menu: [
      { path: 'receipts', permission: 'finance.receipt:read' },
      { path: 'payments', permission: 'finance.payment:read' },
      { path: 'transfers', permission: 'finance.transfer:read' },
      { path: 'cash-accounts', permission: 'finance.cash_account:read' },
      { path: 'reports', permission: 'finance.report:read' },
      { path: 'wht-types', permission: 'finance.settings:read' },
      { path: 'settings', permission: 'finance.settings:read' },
    ],
  },
  {
    key: 'organization',
    icon: Building2,
    tone: 'bg-app-organization',
    menu: [
      { path: 'branches', permission: 'organization.branch:read' },
      { path: 'business-units', permission: 'organization.business_unit:read' },
      { path: 'departments', permission: 'organization.department:read' },
      { path: 'cost-centers', permission: 'organization.cost_center:read' },
      { path: 'project-sites', permission: 'organization.project_site:read' },
      { path: 'fiscal-years', permission: 'organization.fiscal_period:read' },
      { path: 'numbering', permission: 'organization.numbering:read' },
      { path: 'company', permission: 'organization.company:read' },
    ],
  },
  {
    key: 'users',
    icon: UsersRound,
    tone: 'bg-app-users',
    menu: [
      { path: 'members', permission: 'identity.member:read' },
      { path: 'roles', permission: 'identity.role:read' },
    ],
  },
]

/** ชื่อแอปและชื่อเมนูจาก config ข้อความ */
export function appTexts(key: AppKey): { name: string; menu: Record<string, string> } {
  return texts.apps[key] as { name: string; menu: Record<string, string> }
}

/** แอปที่ผู้ใช้เห็น — เหลือเฉพาะเมนูที่มีสิทธิ์ และตัดแอปที่ไม่เหลือเมนู */
export function visibleApps(permissions: readonly string[]): AppDefinition[] {
  return apps
    .map((app) => ({ ...app, menu: app.menu.filter((m) => can(permissions, m.permission)) }))
    .filter((app) => app.menu.length > 0)
}

export function findApp(key: string): AppDefinition | undefined {
  return apps.find((a) => a.key === key)
}

/** path ของหน้าในบริษัท เช่น companyPath(id) = /c/{id}, companyPath(id, 'sales', 'orders') = /c/{id}/sales/orders */
export function companyPath(companyId: string, ...segments: string[]): string {
  return ['/c', companyId, ...segments].join('/')
}

/** ชื่อแท็บของเบราว์เซอร์ที่ใช้เปิดแอป — กดไอคอนเดิมซ้ำ = กลับไปแท็บเดิม ไม่เปิดแท็บซ้ำ */
export function appWindowName(companyId: string, app: AppKey): string {
  return `apexis-${companyId}-${app}`
}
