import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { AgingList, BarList, DashboardError, DashboardGrid, DashboardHeader, KpiCard, KpiGrid, Panel, PanelLink } from '@/features/dashboard/parts'
import { loadDashboard } from '@/features/dashboard/load'
import { TrendChart } from '@/features/dashboard/TrendChart'
import { dashboardApi, type SalesDashboard } from '@/features/dashboard/types'
import { NoPermission } from '@/features/organization/pages'
import { salesPage } from '@/features/sales/types'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.dashboard.sales.title }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ภาพรวมการขาย — ตัวเลขหลักคลิกไปหน้ารายการ / รายงานที่กรองช่วงเดียวกัน */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, 'sales.report:read')) return <NoPermission />
  const t = texts.dashboard.sales
  const { period, data: d, error } = await loadDashboard<SalesDashboard>(dashboardApi(companyId, 'sales'), await searchParams)
  const range = `start_date=${period.start}&end_date=${period.end}`
  const has = (p: string) => can(ctx.permissions, p)
  // กราฟตามปีบัญชี (เดือนเริ่มงวด -> สิ้นงวด) เทียบปีก่อนหน้าเมื่อปีก่อนมียอด
  const fy = d?.fiscal_year ?? null
  const prev = d?.previous_year ?? null
  return (
    <div className="flex flex-col gap-6">
      <DashboardHeader company={ctx.company.name} title={t.title} period={period} />
      {error && <DashboardError message={error} />}
      {d && (
        <>
          <KpiGrid>
            <KpiCard
              label={t.sales}
              value={formatAmount(d.sales.current)}
              delta={{ current: d.sales.current, previous: d.sales.previous }}
              sub={format(t.invoices, { n: String(d.invoice_count) })}
              href={has('sales.invoice:read') ? `${salesPage(companyId, 'invoices')}?${range}` : null}
            />
            <KpiCard
              label={t.receivables}
              value={formatAmount(d.receivables.outstanding)}
              sub={format(t.overdue, { amount: formatAmount(d.receivables.overdue) })}
              warn={toUnits(d.receivables.overdue) > 0n}
              href={`${salesPage(companyId, 'reports')}?book=ar-aging&end_date=${period.end}`}
            />
            <KpiCard
              label={t.quotations}
              value={formatAmount(d.quotations.open_amount)}
              sub={format(t.quotationsSub, { n: String(d.quotations.open_count) })}
              href={has('sales.quotation:read') ? `${salesPage(companyId, 'quotations')}?status=sent` : null}
            />
            <KpiCard
              label={t.toDeliver}
              value={String(d.orders.to_invoice_count)}
              sub={format(t.toDeliverSub, { draft: String(d.orders.draft_count) })}
              href={`${salesPage(companyId, 'reports')}?book=outstanding-sales-orders`}
            />
          </KpiGrid>
          <DashboardGrid>
            <TrendChart
              title={t.monthly}
              description={
                fy
                  ? format(prev ? t.fiscalHintCompare : t.fiscalHint, {
                      year: String(fy.year + 543),
                      start: formatDate(fy.start_date),
                      end: formatDate(fy.end_date),
                    })
                  : texts.dashboard.monthlyHint
              }
              data={d.monthly.map((m, i) => ({ month: m.month, net: m.net, prev: prev?.[i]?.net ?? '0' }))}
              series={[
                { key: 'net', label: fy ? format(t.fiscalYear, { year: String(fy.year + 543) }) : t.net },
                ...(fy && prev ? [{ key: 'prev', label: format(t.fiscalYear, { year: String(fy.year + 542) }) }] : []),
              ]}
            />
            <Panel title={t.aging}>
              <AgingList aging={d.receivables.aging} href={`${salesPage(companyId, 'reports')}?book=ar-aging&end_date=${period.end}`} />
            </Panel>
            <Panel
              title={t.top}
              description={t.topHint}
              className="lg:col-span-3"
              action={has('sales.customer:read') ? <PanelLink href={salesPage(companyId, 'customers')} label={texts.dashboard.viewAll} /> : undefined}
            >
              <BarList
                rows={d.top_customers.map((r) => ({
                  key: r.customer.id,
                  label: `${r.customer.code} · ${r.customer.name}`,
                  value: r.net,
                  href: has('sales.customer:read') ? `${salesPage(companyId, 'customers')}/${r.customer.id}` : undefined,
                }))}
              />
            </Panel>
          </DashboardGrid>
        </>
      )}
    </div>
  )
}
