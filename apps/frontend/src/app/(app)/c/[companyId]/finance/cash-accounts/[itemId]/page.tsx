import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { CashAccountForm } from '@/features/finance/Forms'
import { financeApi, financePage, MASTER_QUERY_KEYS, perms, type CashAccount } from '@/features/finance/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.cashAccounts.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance.cashAccounts
  const listHref = listHrefFromBack(financePage(companyId, 'cash-accounts'), param(await searchParams, 'back'), MASTER_QUERY_KEYS)
  const res = await loadItem<CashAccount>(financeApi.cashAccounts(companyId), itemId, t.notFound)
  if ('panel' in res) return res.panel
  const a = res.item
  const gl = await loadRef(financeApi.accounts(companyId), a.gl_account_id)
  return (
    <FormPage crumb={{ app: texts.apps.finance.name, label: t.heading, href: listHref }} title={format(t.editTitle, { code: a.code })} description={a.name}>
      <CashAccountForm
        apiPath={financeApi.cashAccounts(companyId)}
        accountsApi={financeApi.accounts(companyId)}
        listHref={listHref}
        item={a}
        glAccount={gl}
        canManage={can(ctx.permissions, perms.accountManage)}
      />
    </FormPage>
  )
}
