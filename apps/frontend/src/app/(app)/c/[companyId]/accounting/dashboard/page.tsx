import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { DashboardError, DashboardGrid, DashboardHeader, KpiCard, KpiGrid, Panel, RowList } from '@/features/dashboard/parts'
import { loadDashboard } from '@/features/dashboard/load'
import { TrendChart } from '@/features/dashboard/TrendChart'
import { dashboardApi, type AccountingDashboard } from '@/features/dashboard/types'
import { NoPermission } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.dashboard.accounting.title }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ภาพรวมบัญชี — ผลการดำเนินงานในช่วง (เทียบช่วงก่อน), บัญชีพักที่ต้องจัดการก่อนปิดงบ, รายการบัญชีล่าสุด */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, 'ledger.report:read')) return <NoPermission />
  const t = texts.dashboard.accounting
  const { period, data: d, error } = await loadDashboard<AccountingDashboard>(dashboardApi(companyId, 'accounting'), await searchParams)
  const reports = companyPath(companyId, 'accounting', 'reports')
  const income = `${reports}?book=income-statement&start_date=${period.start}&end_date=${period.end}`
  const entries = companyPath(companyId, 'accounting', 'entries')
  const canEntries = can(ctx.permissions, 'ledger.entry:read')
  const suspense = d ? toUnits(d.suspense_balance) !== 0n : false
  return (
    <div className="flex flex-col gap-6">
      <DashboardHeader company={ctx.company.name} title={t.title} description={t.description} period={period} />
      {error && <DashboardError message={error} />}
      {d && (
        <>
          <KpiGrid>
            <KpiCard label={t.revenue} value={formatAmount(d.income.revenue)} delta={{ current: d.income.revenue, previous: d.previous.revenue }} href={income} />
            <KpiCard
              label={t.expense}
              value={formatAmount(d.income.expense)}
              delta={{ current: d.income.expense, previous: d.previous.expense, upIsGood: false }}
              href={income}
            />
            <KpiCard label={t.netIncome} value={formatAmount(d.income.net_income)} delta={{ current: d.income.net_income, previous: d.previous.net_income }} href={income} />
            <KpiCard
              label={t.suspense}
              value={formatAmount(d.suspense_balance)}
              sub={suspense ? t.suspenseHint : t.suspenseClear}
              warn={suspense}
              href={`${reports}?book=balance-sheet&end_date=${period.end}`}
            />
          </KpiGrid>
          <DashboardGrid>
            <TrendChart
              title={t.monthly}
              description={t.monthlyHint}
              data={d.monthly}
              series={[
                { key: 'revenue', label: t.revenue },
                { key: 'expense', label: t.expense },
              ]}
            />
            <Panel title={t.period}>
              {d.period ? (
                <dl className="flex flex-col gap-3 text-sm">
                  <div className="flex justify-between gap-3">
                    <dt className="text-ink-muted">{d.period.name}</dt>
                    <dd className="font-medium">{d.period.status === 'open' ? t.open : t.closed}</dd>
                  </div>
                  <div className="text-[13px] text-ink-muted">{format(texts.reports.period, { start: formatDate(d.period.start_date), end: formatDate(d.period.end_date) })}</div>
                  <div className="flex justify-between gap-3 border-t border-line pt-3">
                    <dt className="text-ink-muted">{t.entries}</dt>
                    <dd className="font-medium tabular-nums">
                      {d.entries.posted_count}
                      {d.entries.draft_count > 0 && <span className="ml-2 text-[13px] font-normal text-warning">{format(t.drafts, { n: String(d.entries.draft_count) })}</span>}
                    </dd>
                  </div>
                </dl>
              ) : (
                <p className="py-6 text-center text-sm text-ink-muted">{t.noPeriod}</p>
              )}
            </Panel>
            <Panel title={t.recent} className="lg:col-span-3">
              <RowList
                rows={d.recent_entries.map((e) => ({
                  key: e.id,
                  href: canEntries ? `${entries}/${e.id}` : null,
                  title: `${e.entry_no} · ${e.description || '–'}`,
                  meta: formatDate(e.date),
                  value: e.amount,
                }))}
              />
            </Panel>
          </DashboardGrid>
        </>
      )}
    </div>
  )
}
