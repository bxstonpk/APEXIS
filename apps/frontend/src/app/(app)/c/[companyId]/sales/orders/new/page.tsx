import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { loadVat } from '@/features/sales/pages'
import { SalesDocForm } from '@/features/sales/SalesDocForm'
import { perms, salesApi, salesPage } from '@/features/sales/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.sales.orders.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.orderManage)) return <NoPermission />
  const t = texts.sales.orders
  const path = salesPage(companyId, 'orders')
  return (
    <FormPage crumb={{ app: texts.apps.sales.name, label: t.heading, href: path }} title={t.newTitle}>
      <SalesDocForm
        kind="order"
        apiPath={salesApi.orders(companyId)}
        partiesApi={salesApi.customers(companyId)}
        itemsApi={salesApi.items(companyId)}
        docsPath={path}
        listHref={path}
        vat={await loadVat(companyId)}
      />
    </FormPage>
  )
}
