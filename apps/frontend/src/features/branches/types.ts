/** สาขา — ตรงกับ backend organization responses.Branch */
export type Address = {
  line: string
  sub_district: string
  district: string
  province: string
  postal_code: string
  country: string
}

export type Branch = {
  id: string
  code: string
  name: string
  tax_branch_no: string
  is_head_office: boolean
  address: Address
  phone: string
  status: 'active' | 'inactive'
}

/** body ของ POST / PUT /branches (status ไม่ส่งตอนสร้าง = active) */
export type BranchInput = Omit<Branch, 'id' | 'is_head_office'>

export const emptyAddress: Address = { line: '', sub_district: '', district: '', province: '', postal_code: '', country: 'TH' }

/** key ของ URL query ที่หน้ารายการส่งต่อให้ API (GET /branches) */
export const BRANCH_QUERY_KEYS = ['q', 'status', 'sort', 'page', 'page_size'] as const
export const BRANCH_DEFAULT_SORT = 'code'

export function branchesPath(companyId: string): string {
  return `/companies/${companyId}/branches`
}
