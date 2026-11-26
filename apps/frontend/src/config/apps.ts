import {
  BookOpenCheck,
  Building2,
  Factory,
  Landmark,
  type LucideIcon,
  Package,
  ShoppingBag,
  ShoppingCart,
  UsersRound,
} from 'lucide-react'
import { can } from '@/lib/permissions'
import type { BusinessType } from './businessTypes'
import { texts } from './texts'

/**
 * แอปในหน้ารวมแอป (ไอคอนกลางจอ) และเมนูด้านบนของแต่ละแอป
 *
 * - กดไอคอน = เปิดแอปในแท็บใหม่ที่ /c/{companyId}/{app.key}/{menu.path}
 * - แอปแสดงเมื่อผู้ใช้มีสิทธิ์อย่างน้อยหนึ่งเมนู; เมนูแสดงเมื่อมีสิทธิ์ `permission` ของเมนูนั้น
 * - ชื่อแอป / เมนูอยู่ที่ texts.apps[app.key] (locales/th.ts); สีไอคอนเป็น token ใน styles/app.css
 * - เมนูแรกที่มีสิทธิ์ = หน้าแรกของแอป
 * - `businessTypes`: แอปแสดงเฉพาะบริษัทประเภทธุรกิจที่ระบุ (ไม่ระบุ = ทุกประเภท) เช่น คลังสินค้าไม่แสดงในธุรกิจบริการ
 *   แอปเฉพาะประเภท (ผลิต / โครงการก่อสร้าง) เพิ่มที่นี่โดยระบุ businessTypes ของตัวเอง
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
  /** ประเภทธุรกิจที่ใช้แอปนี้ — ไม่ระบุ = ทุกประเภท */
  businessTypes?: readonly BusinessType[]
}

export type AppKey = keyof typeof texts.apps

export const apps: readonly AppDefinition[] = [
  {
    key: 'sales',
    icon: ShoppingCart,
    tone: 'bg-app-sales',
    menu: [
      { path: 'dashboard', permission: 'sales.report:read' },
      { path: 'quotations', permission: 'sales.quotation:read' },
      { path: 'orders', permission: 'sales.order:read' },
      { path: 'deliveries', permission: 'sales.delivery:read' },
      { path: 'invoices', permission: 'sales.invoice:read' },
      { path: 'credit-notes', permission: 'sales.invoice:read' },
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
      { path: 'dashboard', permission: 'purchasing.report:read' },
      { path: 'orders', permission: 'purchasing.order:read' },
      { path: 'receipts', permission: 'purchasing.receipt:read' },
      { path: 'invoices', permission: 'purchasing.invoice:read' },
      { path: 'credit-notes', permission: 'purchasing.invoice:read' },
      { path: 'suppliers', permission: 'purchasing.supplier:read' },
      { path: 'reports', permission: 'purchasing.report:read' },
      { path: 'settings', permission: 'purchasing.settings:read' },
    ],
  },
  {
    key: 'inventory',
    icon: Package,
    tone: 'bg-app-inventory',
    businessTypes: ['trading', 'manufacturing', 'construction'],
    menu: [
      { path: 'dashboard', permission: 'inventory.report:read' },
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
    key: 'manufacturing',
    icon: Factory,
    tone: 'bg-app-manufacturing',
    businessTypes: ['manufacturing'],
    menu: [
      { path: 'orders', permission: 'manufacturing.order:read' },
      { path: 'boms', permission: 'manufacturing.bom:read' },
      { path: 'reports', permission: 'manufacturing.report:read' },
      { path: 'settings', permission: 'manufacturing.settings:read' },
    ],
  },
  {
    key: 'accounting',
    icon: BookOpenCheck,
    tone: 'bg-app-accounting',
    menu: [
      { path: 'dashboard', permission: 'ledger.report:read' },
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
      { path: 'dashboard', permission: 'finance.report:read' },
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

/**
 * แอปใช้กับประเภทธุรกิจนี้ได้หรือไม่ — ไม่รู้ประเภท (ข้อมูลบริษัทเก่า / backend ยังไม่ส่ง) = ใช้ได้ทุกแอป
 */
export function appAvailableFor(app: AppDefinition, businessType: BusinessType | null | undefined): boolean {
  return !app.businessTypes || !businessType || app.businessTypes.includes(businessType)
}

/** แอปที่ผู้ใช้เห็น — ตัดแอปที่ไม่ใช้กับประเภทธุรกิจ, เหลือเฉพาะเมนูที่มีสิทธิ์ และตัดแอปที่ไม่เหลือเมนู */
export function visibleApps(permissions: readonly string[], businessType?: BusinessType | null): AppDefinition[] {
  return apps
    .filter((app) => appAvailableFor(app, businessType))
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
