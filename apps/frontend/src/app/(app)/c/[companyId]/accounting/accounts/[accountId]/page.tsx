import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { AccountForm } from '@/features/accounting/MasterForms'
import { ACCOUNT_QUERY_KEYS, ledgerApi, ledgerPage, perms, type Account } from '@/features/accounting/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.accounts.pageTitle }

type Props = { params: Promise<{ companyId: string; accountId: string }>; searchParams: Promise<SearchParams> }

export default async function AccountPage({ params, searchParams }: Props) {
  const { companyId, accountId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const listHref = listHrefFromBack(ledgerPage.accounts(companyId), param(await searchParams, 'back'), ACCOUNT_QUERY_KEYS)
  const res = await loadItem<Account>(ledgerApi.accounts(companyId), accountId, texts.accounts.notFound)
  if ('panel' in res) return res.panel
  const a = res.item
  return (
    <FormPage crumb={{ app: texts.apps.accounting.name, label: texts.accounts.heading, href: listHref }} title={format(texts.accounts.editTitle, { code: a.code })} description={a.name}>
      <AccountForm apiPath={ledgerApi.accounts(companyId)} listHref={listHref} item={a} canManage={can(ctx.permissions, perms.accountManage)} />
    </FormPage>
  )
}
