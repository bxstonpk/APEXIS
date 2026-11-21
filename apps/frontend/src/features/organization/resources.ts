import { texts } from '@/config'
import { companyPath } from '@/config/apps'
import type { Address } from '@/features/forms/FormParts'

/** ข้อมูลอ้างอิงที่ API ส่งมาในรายการ / รายละเอียด (id + code + name) */
export type Ref = { id: string; code: string; name: string }

export type BusinessUnit = { id: string; code: string; name: string; parent: Ref | null; status: string }

export type Department = {
  id: string
  code: string
  name: string
  parent: Ref | null
  business_unit: Ref | null
  branch: Ref | null
  status: string
}

export type CostCenter = {
  id: string
  code: string
  name: string
  department: Ref | null
  valid_from: string
  valid_to: string | null
  status: string
}

export type ProjectSite = {
  id: string
  code: string
  name: string
  branch: Ref | null
  cost_center: Ref | null
  address: Address
  start_date: string
  end_date: string | null
  status: string
}

export type FiscalPeriod = {
  id: string
  fiscal_year_id: string
  fiscal_year: number
  period_no: number
  start_date: string
  end_date: string
  status: 'open' | 'closed'
}

export type FiscalYear = { id: string; year: number; start_date: string; end_date: string; periods?: FiscalPeriod[] }

export type DocumentNumbering = {
  doc_type: string
  name: string
  default_prefix: string
  prefix: string
  template: string
  example: string
  custom: boolean
}

/**
 * ข้อมูลของแต่ละหน้าในแอปองค์กร
 * - segment: path ต่อจาก /c/{companyId}/organization/ (ตรงกับเมนูใน config/apps.ts)
 * - api: path ของ API ต่อจาก /api/v1
 * - queryKeys: key ใน URL ที่ส่งต่อให้ API (ตรงกับ backend requests/queries.go)
 */
function resource(segment: string, apiName: string, permission: string, queryKeys: readonly string[], defaultSort: string) {
  return {
    segment,
    api: (companyId: string) => `/companies/${companyId}/${apiName}`,
    page: (companyId: string) => companyPath(companyId, 'organization', segment),
    read: `organization.${permission}:read`,
    manage: `organization.${permission}:manage`,
    queryKeys,
    defaultSort,
  }
}

const BASE_KEYS = ['q', 'status', 'sort', 'page', 'page_size'] as const

export const orgResources = {
  branches: resource('branches', 'branches', 'branch', BASE_KEYS, 'code'),
  businessUnits: resource('business-units', 'business-units', 'business_unit', [...BASE_KEYS, 'parent_id'], 'code'),
  departments: resource('departments', 'departments', 'department', [...BASE_KEYS, 'parent_id', 'business_unit_id', 'branch_id'], 'code'),
  costCenters: resource('cost-centers', 'cost-centers', 'cost_center', [...BASE_KEYS, 'department_id', 'active_on'], 'code'),
  projectSites: resource(
    'project-sites',
    'project-sites',
    'project_site',
    [...BASE_KEYS, 'branch_id', 'cost_center_id', 'start_date', 'end_date'],
    'code',
  ),
  fiscalYears: resource('fiscal-years', 'fiscal-years', 'fiscal_period', ['year', 'sort', 'page', 'page_size'], '-year'),
} as const

export const FISCAL_PERIOD_CLOSE = 'organization.fiscal_period:close'
export const NUMBERING_READ = 'organization.numbering:read'
export const NUMBERING_MANAGE = 'organization.numbering:manage'
export const COMPANY_MANAGE = 'organization.company:manage'

/** ข้อความของช่องเลือกข้อมูลอ้างอิง */
export const refTexts = texts.reference

/** ตัวเลือกกรองสถานะ active / inactive (ค่าว่าง = ทั้งหมด) */
export function statusFilterOptions(labels: Record<string, string> = texts.common.statuses) {
  return [{ value: '', label: texts.common.all }, ...Object.entries(labels).map(([value, label]) => ({ value, label }))]
}
