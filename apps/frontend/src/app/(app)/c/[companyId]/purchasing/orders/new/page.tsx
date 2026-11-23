import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { loadPurchaseVat } from '@/features/purchasing/pages'
import { docPerms, purchasingApi, purchasingDocsApi, purchasingPage } from '@/features/purchasing/types'
import { SalesDocForm } from '@/features/sales/SalesDocForm'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.purchasing.orders.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, docPerms.orderManage)) return <NoPermission />
  const t = texts.purchasing.orders
  const path = purchasingPage(companyId, 'orders')
  return (
    <FormPage crumb={{ app: texts.apps.purchasing.name, label: t.heading, href: path }} title={t.newTitle}>
      <SalesDocForm
        kind="purchase-order"
        apiPath={purchasingDocsApi.orders(companyId)}
        partiesApi={purchasingApi.suppliers(companyId)}
        itemsApi={purchasingDocsApi.items(companyId)}
        docsPath={path}
        listHref={path}
        vat={await loadPurchaseVat(companyId)}
      />
    </FormPage>
  )
}
