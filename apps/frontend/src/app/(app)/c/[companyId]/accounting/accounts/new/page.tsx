import type { Metadata } from 'next'
import { texts } from '@/config'
import { AccountForm } from '@/features/accounting/MasterForms'
import { ledgerApi, ledgerPage, perms } from '@/features/accounting/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.accounts.newTitle }

export default async function NewAccountPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.accountManage)) return <NoPermission />
  const listHref = ledgerPage.accounts(companyId)
  return (
    <FormPage crumb={{ app: texts.apps.accounting.name, label: texts.accounts.heading, href: listHref }} title={texts.accounts.newTitle}>
      <AccountForm apiPath={ledgerApi.accounts(companyId)} listHref={listHref} canManage />
    </FormPage>
  )
}
