import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { FundForm } from '@/features/petty-cash/FundForm'
import { loadCustodians } from '@/features/petty-cash/server'
import { pcApi, pcPage, pcPerms } from '@/features/petty-cash/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.pettyCash.funds.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, pcPerms.fundManage)) return <NoPermission />
  const t = texts.pettyCash.funds
  const path = pcPage(companyId, 'funds')
  return (
    <FormPage crumb={{ app: texts.apps['petty-cash'].name, label: t.heading, href: path }} title={t.newTitle}>
      <FundForm apiPath={pcApi.funds(companyId)} listHref={path} accountsApi={pcApi.accounts(companyId)} members={await loadCustodians(companyId)} canManage />
    </FormPage>
  )
}
