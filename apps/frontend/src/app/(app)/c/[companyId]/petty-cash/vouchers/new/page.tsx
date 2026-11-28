import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { pcApi, pcPage, pcPerms } from '@/features/petty-cash/types'
import { VoucherForm } from '@/features/petty-cash/VoucherForm'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.pettyCash.vouchers.newTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, pcPerms.voucherManage)) return <NoPermission />
  const t = texts.pettyCash.vouchers
  const path = pcPage(companyId, 'vouchers')
  const fund = await loadRef(pcApi.funds(companyId), param(await searchParams, 'fund_id'))
  return (
    <FormPage crumb={{ app: texts.apps['petty-cash'].name, label: t.heading, href: path }} title={t.newTitle}>
      <VoucherForm
        apiPath={pcApi.vouchers(companyId)}
        docsPath={path}
        listHref={path}
        fundsApi={pcApi.funds(companyId)}
        accountsApi={pcApi.accounts(companyId)}
        vatRate="7"
        initialFund={fund}
      />
    </FormPage>
  )
}
