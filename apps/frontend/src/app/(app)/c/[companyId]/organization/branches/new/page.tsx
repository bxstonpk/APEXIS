import type { Metadata } from 'next'
import { texts } from '@/config'
import { BranchForm } from '@/features/branches/BranchForm'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { orgResources } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.branches.newTitle }

export default async function NewBranchPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.branches
  if (!can(ctx.permissions, r.manage)) return <NoPermission />

  return (
    <FormPage crumb={{ label: texts.branches.heading, href: r.page(companyId) }} title={texts.branches.newTitle}>
      <BranchForm companyId={companyId} canManage listHref={r.page(companyId)} />
    </FormPage>
  )
}
