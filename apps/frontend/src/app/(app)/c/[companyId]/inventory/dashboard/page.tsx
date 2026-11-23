import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { BarList, DashboardError, DashboardGrid, DashboardHeader, KpiCard, KpiGrid, Panel, RowList } from '@/features/dashboard/parts'
import { loadDashboard } from '@/features/dashboard/load'
import { TrendChart } from '@/features/dashboard/TrendChart'
import { dashboardApi, type InventoryDashboard } from '@/features/dashboard/types'
import { NoPermission } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.dashboard.inventory.title }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ภาพรวมคลังสินค้า — มูลค่า ณ วันสิ้นช่วง, ความเคลื่อนไหวในช่วง, สินค้าที่ควรสั่งเพิ่ม */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, 'inventory.report:read')) return <NoPermission />
  const t = texts.dashboard.inventory
  const { period, data: d, error } = await loadDashboard<InventoryDashboard>(dashboardApi(companyId, 'inventory'), await searchParams)
  return (
    <div className="flex flex-col gap-6">
      <DashboardHeader company={ctx.company.name} title={t.title} description={t.description} period={period} />
      {error && <DashboardError message={error} />}
      {d && (
        <>
          <KpiGrid>
            <KpiCard label={t.value} value={formatAmount(d.stock_value)} sub={format(t.asOf, { date: formatDate(d.end_date) })} />
            <KpiCard label={t.items} value={String(d.stock_item_count)} sub={t.itemsSub} />
            <KpiCard label={t.belowReorder} value={String(d.below_reorder_count)} sub={t.belowReorderSub} warn={d.below_reorder_count > 0} />
            <KpiCard label={t.movements} value={String(d.movements.receipt_count)} sub={format(t.movementsSub, { n: String(d.movements.issue_count) })} />
          </KpiGrid>
          <DashboardGrid>
            <TrendChart
              title={t.monthly}
              description={t.monthlyHint}
              data={d.monthly}
              series={[
                { key: 'in_value', label: t.in },
                { key: 'out_value', label: t.out },
              ]}
            />
            <Panel title={t.warehouses}>
              <BarList rows={d.warehouses.map((w) => ({ key: w.warehouse.id, label: `${w.warehouse.code} · ${w.warehouse.name}`, value: w.value }))} />
            </Panel>
            <Panel title={t.reorder} description={t.reorderHint} className="lg:col-span-3">
              <RowList
                empty={t.reorderEmpty}
                rows={d.below_reorder.map((r) => ({
                  key: r.item.id,
                  title: `${r.item.code} · ${r.item.name}`,
                  meta: format(t.onHand, { qty: formatAmount(r.on_hand), rp: formatAmount(r.reorder_point) }),
                }))}
              />
            </Panel>
          </DashboardGrid>
        </>
      )}
    </div>
  )
}
