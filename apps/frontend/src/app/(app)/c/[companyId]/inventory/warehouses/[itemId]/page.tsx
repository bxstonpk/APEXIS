import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { WarehouseForm } from '@/features/inventory/MasterForms'
import { MASTER_QUERY_KEYS, invApi, inventoryPage, invPerms, type Warehouse } from '@/features/inventory/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.inventory.warehouses.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.warehouses
  const listHref = listHrefFromBack(inventoryPage(companyId, 'warehouses'), param(await searchParams, 'back'), MASTER_QUERY_KEYS)
  const res = await loadItem<Warehouse>(invApi.warehouses(companyId), itemId, t.notFound)
  if ('panel' in res) return res.panel
  const x = res.item
  const branch = await loadRef(invApi.branches(companyId), x.branch_id ?? '')
  return (
    <FormPage crumb={{ app: texts.apps.inventory.name, label: t.heading, href: listHref }} title={format(t.editTitle, { code: x.code })} description={x.name}>
      <WarehouseForm
        apiPath={invApi.warehouses(companyId)} branchesApi={invApi.branches(companyId)} branch={branch}
        listHref={listHref}
        item={x}
        canManage={can(ctx.permissions, invPerms.warehouseManage)}
      />
    </FormPage>
  )
}
