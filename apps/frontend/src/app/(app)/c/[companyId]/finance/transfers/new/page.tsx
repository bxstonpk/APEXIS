import type { Metadata } from 'next'
import { texts } from '@/config'
import { TransferForm } from '@/features/finance/Forms'
import { financeApi, financePage, perms } from '@/features/finance/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.finance.transfers.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.transferPost)) return <NoPermission />
  const path = financePage(companyId, 'transfers')
  return (
    <FormPage crumb={{ app: texts.apps.finance.name, label: texts.finance.transfers.heading, href: path }} title={texts.finance.transfers.newTitle}>
      <TransferForm apiPath={financeApi.transfers(companyId)} cashApi={financeApi.cashAccounts(companyId)} docsPath={path} listHref={path} />
    </FormPage>
  )
}
