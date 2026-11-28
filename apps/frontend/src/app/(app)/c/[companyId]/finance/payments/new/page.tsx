import type { Metadata } from 'next'
import { texts } from '@/config'
import { MoneyDocForm } from '@/features/finance/MoneyDocForm'
import { financeApi, financePage, perms } from '@/features/finance/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.finance.payments.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.paymentPost)) return <NoPermission />
  const path = financePage(companyId, 'payments')
  return (
    <FormPage crumb={{ app: texts.apps.finance.name, label: texts.finance.payments.heading, href: path }} title={texts.finance.payments.newTitle}>
      <MoneyDocForm
        mode="payment"
        docsPath={path}
        listHref={path}
        api={{
          submit: financeApi.payments(companyId),
          open: `${financeApi.payments(companyId)}/open-invoices`,
          parties: financeApi.suppliers(companyId),
          accounts: financeApi.accounts(companyId),
          cashAccounts: financeApi.cashAccounts(companyId),
          whtTypes: financeApi.paymentWhtTypes(companyId),
          pettyCash: `${financeApi.payments(companyId)}/petty-cash-requests`,
        }}
      />
    </FormPage>
  )
}
