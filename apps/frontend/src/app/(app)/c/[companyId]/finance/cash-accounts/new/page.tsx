import type { Metadata } from 'next'
import { texts } from '@/config'
import { CashAccountForm } from '@/features/finance/Forms'
import { financeApi, financePage, perms } from '@/features/finance/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.finance.cashAccounts.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.accountManage)) return <NoPermission />
  const path = financePage(companyId, 'cash-accounts')
  const t = texts.finance.cashAccounts
  return (
    <FormPage crumb={{ app: texts.apps.finance.name, label: t.heading, href: path }} title={t.newTitle}>
      <CashAccountForm apiPath={financeApi.cashAccounts(companyId)} accountsApi={financeApi.accounts(companyId)} listHref={path} canManage />
    </FormPage>
  )
}
