import type { Metadata } from 'next'
import { texts } from '@/config'
import { FiscalYearForm } from '@/features/organization/FiscalYearForm'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { orgResources } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.fiscalYears.newTitle }

export default async function NewFiscalYearPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.fiscalYears
  if (!can(ctx.permissions, r.manage)) return <NoPermission />
  const path = r.page(companyId)
  return (
    <FormPage crumb={{ label: texts.fiscalYears.heading, href: r.page(companyId) }} title={texts.fiscalYears.newTitle} description={texts.fiscalYears.description}>
      <FiscalYearForm apiPath={r.api(companyId)} listHref={path} defaultYear={new Date().getFullYear()} />
    </FormPage>
  )
}
