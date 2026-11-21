import type { Metadata } from 'next'
import { texts } from '@/config'
import { ProjectSiteForm } from '@/features/organization/MasterDataForms'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { orgResources } from '@/features/organization/resources'
import { today } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.projectSites.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.projectSites
  if (!can(ctx.permissions, r.manage)) return <NoPermission />
  return (
    <FormPage crumb={{ label: texts.projectSites.heading, href: r.page(companyId) }} title={texts.projectSites.newTitle}>
      <ProjectSiteForm companyId={companyId} apiPath={r.api(companyId)} canManage listHref={r.page(companyId)} defaultStartDate={today()} />
    </FormPage>
  )
}
