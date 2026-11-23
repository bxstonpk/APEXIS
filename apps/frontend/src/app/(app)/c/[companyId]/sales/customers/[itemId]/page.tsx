import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { FormPage, loadItem } from '@/features/organization/pages'
import { CustomerForm } from '@/features/sales/Forms'
import { CUSTOMER_QUERY_KEYS, perms, salesApi, salesPage, type Customer } from '@/features/sales/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.customers.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales.customers
  const listHref = listHrefFromBack(salesPage(companyId, 'customers'), param(await searchParams, 'back'), CUSTOMER_QUERY_KEYS)
  const res = await loadItem<Customer>(salesApi.customers(companyId), itemId, t.notFound)
  if ('panel' in res) return res.panel
  const c = res.item
  return (
    <FormPage crumb={{ app: texts.apps.sales.name, label: t.heading, href: listHref }} title={format(t.editTitle, { code: c.code })} description={c.name}>
      <CustomerForm apiPath={salesApi.customers(companyId)} listHref={listHref} item={c} canManage={can(ctx.permissions, perms.customerManage)} />
    </FormPage>
  )
}
