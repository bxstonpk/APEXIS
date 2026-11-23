import { texts } from '@/config'
import { FormPage, loadItem, NoPermission } from '@/features/organization/pages'
import { MessagePanel } from '@/features/shell'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { FulfilForm } from './FulfilForm'
import { perms, salesApi, salesPage, type Order } from './types'

/** หน้าสร้างใบส่งของ / ใบกำกับภาษีจาก ?order_id= (server) — ใบสั่งขายต้องอนุมัติแล้ว */
export async function FulfilPage({ companyId, orderId, mode }: { companyId: string; orderId: string; mode: 'delivery' | 'invoice' }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, mode === 'delivery' ? perms.deliveryPost : perms.invoicePost)) return <NoPermission />
  const t = texts.sales
  const page = mode === 'delivery' ? t.deliveries : t.invoices
  const docsPath = salesPage(companyId, mode === 'delivery' ? 'deliveries' : 'invoices')
  if (!orderId) return <MessagePanel title={t.orders.notFound} description={t.pickOrder} />
  const res = await loadItem<Order>(salesApi.orders(companyId), orderId, t.orders.notFound)
  if ('panel' in res) return res.panel
  const o = res.item
  const orderHref = `${salesPage(companyId, 'orders')}/${o.id}`
  return (
    <FormPage crumb={{ app: texts.apps.sales.name, label: page.heading, href: docsPath }} title={page.newTitle} description={`${o.document_no ?? ''} · ${o.customer.name}`}>
      {o.status === 'approved' ? (
        <FulfilForm
          mode={mode}
          order={o}
          apiPath={mode === 'delivery' ? salesApi.deliveries(companyId) : salesApi.invoices(companyId)}
          warehousesApi={salesApi.warehouses(companyId)}
          docsPath={docsPath}
          orderHref={orderHref}
        />
      ) : (
        <MessagePanel title={t.orderNotApproved} />
      )}
    </FormPage>
  )
}
