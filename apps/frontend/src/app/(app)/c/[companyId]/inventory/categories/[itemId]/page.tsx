import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { CategoryForm } from '@/features/inventory/MasterForms'
import { MASTER_QUERY_KEYS, invApi, inventoryPage, invPerms, type Category } from '@/features/inventory/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.inventory.categories.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.categories
  const listHref = listHrefFromBack(inventoryPage(companyId, 'categories'), param(await searchParams, 'back'), MASTER_QUERY_KEYS)
  const res = await loadItem<Category>(invApi.categories(companyId), itemId, t.notFound)
  if ('panel' in res) return res.panel
  const x = res.item
  const [purchase, inventory, revenue] = await Promise.all([
    loadRef(invApi.accounts(companyId), x.purchase_account_id ?? ''),
    loadRef(invApi.accounts(companyId), x.inventory_account_id ?? ''),
    loadRef(invApi.accounts(companyId), x.revenue_account_id ?? ''),
  ])
  const accounts = { purchase, inventory, revenue }
  return (
    <FormPage crumb={{ app: texts.apps.inventory.name, label: t.heading, href: listHref }} title={format(t.editTitle, { code: x.code })} description={x.name}>
      <CategoryForm
        apiPath={invApi.categories(companyId)} accountsApi={invApi.accounts(companyId)} accounts={accounts}
        listHref={listHref}
        item={x}
        canManage={can(ctx.permissions, invPerms.categoryManage)}
      />
    </FormPage>
  )
}
