import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { UomForm } from '@/features/inventory/MasterForms'
import { MASTER_QUERY_KEYS, invApi, inventoryPage, invPerms, type Uom } from '@/features/inventory/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.inventory.uoms.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.uoms
  const listHref = listHrefFromBack(inventoryPage(companyId, 'uoms'), param(await searchParams, 'back'), MASTER_QUERY_KEYS)
  const res = await loadItem<Uom>(invApi.uoms(companyId), itemId, t.notFound)
  if ('panel' in res) return res.panel
  const x = res.item
  return (
    <FormPage crumb={{ app: texts.apps.inventory.name, label: t.heading, href: listHref }} title={format(t.editTitle, { code: x.code })} description={x.name}>
      <UomForm
        apiPath={invApi.uoms(companyId)}
        listHref={listHref}
        item={x}
        canManage={can(ctx.permissions, invPerms.uomManage)}
      />
    </FormPage>
  )
}
