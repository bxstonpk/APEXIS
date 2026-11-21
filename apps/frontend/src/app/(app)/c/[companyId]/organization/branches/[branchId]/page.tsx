import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { BranchForm } from '@/features/branches/BranchForm'
import type { Branch } from '@/features/branches/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { orgResources } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

type Props = { params: Promise<{ companyId: string; branchId: string }>; searchParams: Promise<SearchParams> }

export const metadata: Metadata = { title: texts.branches.pageTitle }

/**
 * ดู / แก้ไขสาขา — ?back=<query ของหน้ารายการ> ใช้พากลับไปรายการพร้อมเงื่อนไขค้นหาเดิมหลังบันทึก
 * ไม่มีสิทธิ์ manage = แสดงแบบอ่านอย่างเดียว
 */
export default async function BranchPage({ params, searchParams }: Props) {
  const { companyId, branchId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null

  const r = orgResources.branches
  const listHref = listHrefFromBack(r.page(companyId), param(await searchParams, 'back'), r.queryKeys)
  const res = await loadItem<Branch>(r.api(companyId), branchId, texts.errors.branch_not_found!)
  if ('panel' in res) return res.panel
  const branch = res.item

  return (
    <FormPage crumb={{ label: texts.branches.heading, href: listHref }} title={format(texts.branches.editTitle, { code: branch.code })} description={branch.name}>
      <BranchForm companyId={companyId} branch={branch} canManage={can(ctx.permissions, r.manage)} listHref={listHref} />
    </FormPage>
  )
}
