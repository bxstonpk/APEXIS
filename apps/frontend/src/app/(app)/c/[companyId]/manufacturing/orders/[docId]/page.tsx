import type { Metadata } from 'next'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { OrderActions } from '@/features/manufacturing/OrderActions'
import { OrderForm } from '@/features/manufacturing/OrderForm'
import { mfgApi, mfgPage, mfgPerms, ORDER_QUERY_KEYS, type ProductionOrder } from '@/features/manufacturing/types'
import { CostCard, MaterialsTable, Movements, OrderStatus } from '@/features/manufacturing/Views'
import { FormPage, loadItem } from '@/features/organization/pages'
import { dateText, Facts } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.manufacturing.orders.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/**
 * ใบสั่งผลิต — ร่าง: แก้ไข (หัวเอกสาร + จำนวนวัตถุดิบ) / ปล่อยผลิต / ยกเลิก
 * กำลังผลิต: เบิก / คืนวัตถุดิบ / ผลิตเสร็จ / ยกเลิก; ผลิตเสร็จ / ยกเลิก: ดูอย่างเดียว
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.manufacturing.orders
  const f = t.fields
  const path = mfgPage(companyId, 'orders')
  const listHref = listHrefFromBack(path, param(await searchParams, 'back'), ORDER_QUERY_KEYS)
  const res = await loadItem<ProductionOrder>(mfgApi.orders(companyId), docId, t.notFound)
  if ('panel' in res) return res.panel
  const o = res.item
  const manage = can(ctx.permissions, mfgPerms.orderManage)
  const editing = o.status === 'draft' && manage
  const uom = o.item.uom ? ` ${o.item.uom}` : ''
  return (
    <FormPage
      crumb={{ app: texts.apps.manufacturing.name, label: t.heading, href: listHref }}
      title={o.document_no ?? t.draftTitle}
      description={`${o.item.code} ${o.item.name}`}
      aside={<OrderStatus status={o.status} />}
    >
      <div className="flex flex-col gap-6">
        <OrderActions apiPath={`${mfgApi.orders(companyId)}/${o.id}`} order={o} canManage={manage} printHref={`${path}/${o.id}/print`} />
        {editing ? (
          <OrderForm
            apiPath={mfgApi.orders(companyId)}
            bomsApi={mfgApi.boms(companyId)}
            itemsApi={mfgApi.items(companyId)}
            warehousesApi={mfgApi.warehouses(companyId)}
            docsPath={path}
            listHref={listHref}
            order={o}
          />
        ) : (
          <>
            <Facts
              items={[
                [f.date, dateText(o.date)],
                [f.bom, `${o.bom.code} ${o.bom.name}`],
                [f.warehouse, `${o.warehouse.code} ${o.warehouse.name}`],
                [f.quantity, `${formatAmount(o.quantity)}${uom}`],
                [f.plan, [dateText(o.planned_start), dateText(o.planned_end)].filter(Boolean).join(' – ')],
                ...(o.status === 'completed'
                  ? ([
                      [f.completed, `${formatAmount(o.completed_quantity)}${uom}`],
                      [f.completedAt, dateText(o.completed_at)],
                    ] as [string, string][])
                  : []),
                ...(o.status === 'cancelled' ? ([[f.cancelReason, o.cancel_reason]] as [string, string][]) : []),
                [f.description, o.description],
              ]}
            />
            <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
              <MaterialsTable materials={o.materials ?? []} showProgress={o.status === 'released'} />
              <div className="flex flex-col gap-6">
                <CostCard order={o} />
                <Movements companyId={companyId} movements={o.movements ?? []} />
              </div>
            </div>
          </>
        )}
      </div>
    </FormPage>
  )
}
