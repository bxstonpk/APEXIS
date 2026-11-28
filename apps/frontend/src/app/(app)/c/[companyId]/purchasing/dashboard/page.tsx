import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { AgingList, BarList, DashboardError, DashboardGrid, DashboardHeader, KpiCard, KpiGrid, Panel, PanelLink } from '@/features/dashboard/parts'
import { loadDashboard } from '@/features/dashboard/load'
import { TrendChart } from '@/features/dashboard/TrendChart'
import { dashboardApi, type PurchasingDashboard } from '@/features/dashboard/types'
import { NoPermission } from '@/features/organization/pages'
import { purchasingPage } from '@/features/purchasing/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.dashboard.purchasing.title }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ภาพรวมการจัดซื้อ — ตัวเลขหลักคลิกไปหน้ารายการ / รายงานที่กรองช่วงเดียวกัน */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, 'purchasing.report:read')) return <NoPermission />
  const t = texts.dashboard.purchasing
  const { period, data: d, error } = await loadDashboard<PurchasingDashboard>(dashboardApi(companyId, 'purchasing'), await searchParams)
  const suppliers = can(ctx.permissions, 'purchasing.supplier:read')
  const range = `start_date=${period.start}&end_date=${period.end}`
  const aging = `${purchasingPage(companyId, 'reports')}?book=ap-aging&end_date=${period.end}`
  return (
    <div className="flex flex-col gap-6">
      <DashboardHeader company={ctx.company.name} title={t.title} period={period} />
      {error && <DashboardError message={error} />}
      {d && (
        <>
          <KpiGrid>
            <KpiCard
              label={t.purchases}
              value={formatAmount(d.purchases.current)}
              delta={{ current: d.purchases.current, previous: d.purchases.previous, upIsGood: false }}
              sub={format(t.invoices, { n: String(d.invoice_count) })}
              href={
                can(ctx.permissions, 'purchasing.invoice:read') || can(ctx.permissions, 'purchasing.service_invoice:read')
                  ? `${purchasingPage(companyId, 'documents')}?type=invoice%2Cservice_invoice&${range}`
                  : null
              }
            />
            <KpiCard
              label={t.payables}
              value={formatAmount(d.payables.outstanding)}
              sub={format(t.overdue, { amount: formatAmount(d.payables.overdue) })}
              warn={toUnits(d.payables.overdue) > 0n}
              href={aging}
            />
            <KpiCard label={t.due7} value={formatAmount(d.payables.due_7_days)} sub={t.due7Sub} />
            <KpiCard
              label={t.toReceive}
              value={String(d.orders.to_receive_count)}
              sub={format(t.toReceiveSub, { invoice: String(d.orders.to_invoice_count), draft: String(d.orders.draft_count) })}
              href={`${purchasingPage(companyId, 'reports')}?book=outstanding-purchase-orders`}
            />
          </KpiGrid>
          <DashboardGrid>
            <TrendChart title={t.monthly} description={texts.dashboard.monthlyHint} data={d.monthly} series={[{ key: 'net', label: t.net }]} />
            <Panel title={t.aging}>
              <AgingList aging={d.payables.aging} href={aging} />
            </Panel>
            <Panel
              title={t.top}
              description={t.topHint}
              className="lg:col-span-3"
              action={suppliers ? <PanelLink href={purchasingPage(companyId, 'suppliers')} label={texts.dashboard.viewAll} /> : undefined}
            >
              <BarList
                rows={d.top_suppliers.map((r) => ({
                  key: r.supplier.id,
                  label: `${r.supplier.code} · ${r.supplier.name}`,
                  value: r.net,
                  href: suppliers ? `${purchasingPage(companyId, 'suppliers')}/${r.supplier.id}` : undefined,
                }))}
              />
            </Panel>
          </DashboardGrid>
        </>
      )}
    </div>
  )
}
