import type { Metadata } from 'next'
import { PageHeader } from '@/components'
import { texts } from '@/config'
import { companyPath } from '@/config/apps'
import { BranchForm } from '@/features/branches/BranchForm'
import { MessagePanel } from '@/features/shell'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.branches.newTitle }

export default async function NewBranchPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, 'organization.branch:manage')) return <MessagePanel title={texts.common.noPermission} />

  return (
    <div className="mx-auto max-w-3xl">
      <PageHeader title={texts.branches.newTitle} />
      <BranchForm companyId={companyId} canManage listHref={companyPath(companyId, 'organization', 'branches')} />
    </div>
  )
}
