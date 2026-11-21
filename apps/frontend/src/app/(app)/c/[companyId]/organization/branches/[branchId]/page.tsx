import type { Metadata } from 'next'
import { Alert, PageHeader } from '@/components'
import { format, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { BranchForm } from '@/features/branches/BranchForm'
import { BRANCH_QUERY_KEYS, branchesPath, type Branch } from '@/features/branches/types'
import { MessagePanel } from '@/features/shell'
import { ApiError, errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext, isCompanyId } from '@/lib/server/company'
import { param, pickQuery, type SearchParams } from '@/lib/url'

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

  // back ผ่าน pickQuery อีกชั้น: เหลือเฉพาะ key ของหน้ารายการ (กันใส่ค่าแปลกๆ ลง URL)
  const back = pickQuery(Object.fromEntries(new URLSearchParams(param(await searchParams, 'back'))), BRANCH_QUERY_KEYS).toString()
  const listPath = companyPath(companyId, 'organization', 'branches')
  const listHref = back ? `${listPath}?${back}` : listPath

  if (!isCompanyId(branchId)) return <MessagePanel title={texts.errors.branch_not_found!} />
  let branch: Branch
  try {
    branch = (await backendGet<Branch>(`${branchesPath(companyId)}/${branchId}`)).data
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return <MessagePanel title={texts.errors.branch_not_found!} />
    return <Alert tone="error">{errorMessage(err)}</Alert>
  }

  return (
    <div className="mx-auto max-w-3xl">
      <PageHeader title={format(texts.branches.editTitle, { code: branch.code })} description={branch.name} />
      <BranchForm companyId={companyId} branch={branch} canManage={can(ctx.permissions, 'organization.branch:manage')} listHref={listHref} />
    </div>
  )
}
