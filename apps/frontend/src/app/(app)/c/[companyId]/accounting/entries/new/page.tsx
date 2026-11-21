import type { Metadata } from 'next'
import { texts } from '@/config'
import { EntryForm } from '@/features/accounting/EntryForm'
import { entryFormApi } from '@/features/accounting/pages'
import { ledgerPage, perms } from '@/features/accounting/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.entries.newTitle }

export default async function NewEntryPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.entryManage)) return <NoPermission />
  const path = ledgerPage.entries(companyId)
  return (
    <FormPage crumb={{ app: texts.apps.accounting.name, label: texts.entries.heading, href: path }} title={texts.entries.newTitle}>
      <EntryForm companyApi={entryFormApi(companyId)} listHref={path} entriesPath={path} canPost={can(ctx.permissions, perms.entryPost)} />
    </FormPage>
  )
}
