import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { JournalForm } from '@/features/accounting/MasterForms'
import { JOURNAL_QUERY_KEYS, ledgerApi, ledgerPage, perms, type Journal } from '@/features/accounting/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.journals.pageTitle }

type Props = { params: Promise<{ companyId: string; journalId: string }>; searchParams: Promise<SearchParams> }

export default async function JournalPage({ params, searchParams }: Props) {
  const { companyId, journalId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const listHref = listHrefFromBack(ledgerPage.journals(companyId), param(await searchParams, 'back'), JOURNAL_QUERY_KEYS)
  const res = await loadItem<Journal>(ledgerApi.journals(companyId), journalId, texts.journals.notFound)
  if ('panel' in res) return res.panel
  const j = res.item
  return (
    <FormPage crumb={{ app: texts.apps.accounting.name, label: texts.journals.heading, href: listHref }} title={format(texts.journals.editTitle, { code: j.code })} description={j.name}>
      <JournalForm apiPath={ledgerApi.journals(companyId)} listHref={listHref} item={j} canManage={can(ctx.permissions, perms.journalManage)} />
    </FormPage>
  )
}
