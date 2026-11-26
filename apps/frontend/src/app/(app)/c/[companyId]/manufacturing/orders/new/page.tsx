import type { Metadata } from 'next'
import { texts } from '@/config'
import { OrderForm } from '@/features/manufacturing/OrderForm'
import { mfgApi, mfgPage, mfgPerms, type ManufacturingSettings } from '@/features/manufacturing/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'

export const metadata: Metadata = { title: texts.manufacturing.orders.newTitle }

/** สร้างใบสั่งผลิต — คลังตั้งต้นจากค่าตั้งการผลิต (ไม่มีสิทธิ์อ่านค่าตั้ง / ไม่ได้ตั้ง = ว่าง) */
export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, mfgPerms.orderManage)) return <NoPermission />
  const t = texts.manufacturing.orders
  const path = mfgPage(companyId, 'orders')
  const settings = await backendGet<ManufacturingSettings>(mfgApi.settings(companyId)).then(
    (r) => r.data,
    () => null,
  )
  const warehouse = settings?.default_warehouse_id ? await loadRef(mfgApi.warehouses(companyId), settings.default_warehouse_id) : null
  return (
    <FormPage crumb={{ app: texts.apps.manufacturing.name, label: t.heading, href: path }} title={t.newTitle}>
      <OrderForm
        apiPath={mfgApi.orders(companyId)}
        bomsApi={mfgApi.boms(companyId)}
        itemsApi={mfgApi.items(companyId)}
        warehousesApi={mfgApi.warehouses(companyId)}
        docsPath={path}
        listHref={path}
        defaultWarehouse={warehouse}
      />
    </FormPage>
  )
}
