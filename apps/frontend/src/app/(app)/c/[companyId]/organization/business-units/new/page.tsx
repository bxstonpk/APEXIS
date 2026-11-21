import type { Metadata } from 'next'
import { texts } from '@/config'
import { BusinessUnitForm } from '@/features/organization/MasterDataForms'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { orgResources } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.businessUnits.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.businessUnits
  if (!can(ctx.permissions, r.manage)) return <NoPermission />
  return (
    <FormPage crumb={{ label: texts.businessUnits.heading, href: r.page(companyId) }} title={texts.businessUnits.newTitle}>
      <BusinessUnitForm companyId={companyId} apiPath={r.api(companyId)} canManage listHref={r.page(companyId)} />
    </FormPage>
  )
}
