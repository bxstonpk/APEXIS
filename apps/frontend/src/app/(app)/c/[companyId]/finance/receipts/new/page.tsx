import type { Metadata } from 'next'
import { texts } from '@/config'
import { MoneyDocForm } from '@/features/finance/MoneyDocForm'
import { financeApi, financePage, perms } from '@/features/finance/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.finance.receipts.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.receiptPost)) return <NoPermission />
  const path = financePage(companyId, 'receipts')
  return (
    <FormPage crumb={{ app: texts.apps.finance.name, label: texts.finance.receipts.heading, href: path }} title={texts.finance.receipts.newTitle}>
      <MoneyDocForm
        mode="receipt"
        docsPath={path}
        listHref={path}
        api={{
          submit: financeApi.receipts(companyId),
          open: `${financeApi.receipts(companyId)}/open-invoices`,
          parties: financeApi.customers(companyId),
          accounts: financeApi.accounts(companyId),
          cashAccounts: financeApi.cashAccounts(companyId),
          
        }}
      />
    </FormPage>
  )
}
