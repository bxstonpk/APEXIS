import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { FormPage, loadItem } from '@/features/organization/pages'
import { vatRegistrantsApi } from '@/features/partners/thaiAddress'
import { SupplierForm } from '@/features/purchasing/SupplierForm'
import { perms, purchasingApi, purchasingPage, SUPPLIER_QUERY_KEYS, type Supplier } from '@/features/purchasing/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.suppliers.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing.suppliers
  const listHref = listHrefFromBack(purchasingPage(companyId, 'suppliers'), param(await searchParams, 'back'), SUPPLIER_QUERY_KEYS)
  const res = await loadItem<Supplier>(purchasingApi.suppliers(companyId), itemId, t.notFound)
  if ('panel' in res) return res.panel
  const c = res.item
  return (
    <FormPage crumb={{ app: texts.apps.purchasing.name, label: t.heading, href: listHref }} title={format(t.editTitle, { code: c.code })} description={c.name}>
      <SupplierForm apiPath={purchasingApi.suppliers(companyId)} lookupApi={vatRegistrantsApi(companyId)} suppliersPath={purchasingPage(companyId, 'suppliers')} listHref={listHref} item={c} canManage={can(ctx.permissions, perms.supplierManage)} />
    </FormPage>
  )
}
