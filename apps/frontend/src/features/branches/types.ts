import type { Address } from '@/features/forms/FormParts'

/** สาขา — ตรงกับ backend organization responses.Branch */
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

export function branchesPath(companyId: string): string {
  return `/companies/${companyId}/branches`
}
