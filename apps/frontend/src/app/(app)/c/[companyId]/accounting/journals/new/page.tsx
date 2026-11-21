import type { Metadata } from 'next'
import { texts } from '@/config'
import { JournalForm } from '@/features/accounting/MasterForms'
import { ledgerApi, ledgerPage, perms } from '@/features/accounting/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.journals.newTitle }

export default async function NewJournalPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.journalManage)) return <NoPermission />
  const listHref = ledgerPage.journals(companyId)
  return (
    <FormPage crumb={{ app: texts.apps.accounting.name, label: texts.journals.heading, href: listHref }} title={texts.journals.newTitle}>
      <JournalForm apiPath={ledgerApi.journals(companyId)} listHref={listHref} canManage />
    </FormPage>
  )
}
