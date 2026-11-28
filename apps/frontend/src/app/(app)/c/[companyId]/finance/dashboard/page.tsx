import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { BarList, DashboardError, DashboardGrid, DashboardHeader, KpiCard, KpiGrid, Panel, RowList } from '@/features/dashboard/parts'
import { loadDashboard } from '@/features/dashboard/load'
import { TrendChart } from '@/features/dashboard/TrendChart'
import { dashboardApi, type FinanceDashboard } from '@/features/dashboard/types'
import { financePage } from '@/features/finance/types'
import { NoPermission } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.dashboard.finance.title }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const MENU = { receipt: 'receipts', payment: 'payments', transfer: 'transfers' } as const
const READ = { receipt: 'finance.receipt:read', payment: 'finance.payment:read', transfer: 'finance.transfer:read' } as const

/** ภาพรวมการเงิน — ยอดเงิน ณ วันสิ้นช่วง (ตามบัญชีแยกประเภท), รับ-จ่ายในช่วง, รายการล่าสุด */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, 'finance.report:read')) return <NoPermission />
  const t = texts.dashboard.finance
  const { period, data: d, error } = await loadDashboard<FinanceDashboard>(dashboardApi(companyId, 'finance'), await searchParams)
  const range = `start_date=${period.start}&end_date=${period.end}`
  const has = (p: string) => can(ctx.permissions, p)
  return (
    <div className="flex flex-col gap-6">
      <DashboardHeader company={ctx.company.name} title={t.title} period={period} />
      {error && <DashboardError message={error} />}
      {d && (
        <>
          <KpiGrid>
            <KpiCard
              label={t.cash}
              value={formatAmount(d.cash_total)}
              sub={format(t.cashSub, { n: String(d.cash_accounts.length) })}
              href={has('finance.cash_account:read') ? financePage(companyId, 'cash-accounts') : null}
            />
            <KpiCard label={t.received} value={formatAmount(d.received)} href={has(READ.receipt) ? `${financePage(companyId, 'receipts')}?${range}` : null} />
            <KpiCard label={t.paid} value={formatAmount(d.paid)} href={has(READ.payment) ? `${financePage(companyId, 'payments')}?${range}` : null} />
            <KpiCard
              label={t.net}
              value={formatAmount(d.net_flow)}
              sub={format(t.transfers, { n: String(d.transfer_count) })}
              warn={toUnits(d.net_flow) < 0n}
            />
          </KpiGrid>
          <DashboardGrid>
            <TrendChart
              title={t.monthly}
              description={t.monthlyHint}
              data={d.monthly}
              series={[
                { key: 'received', label: t.received },
                { key: 'paid', label: t.paid },
              ]}
            />
            <Panel title={t.accounts} description={t.accountsHint}>
              <BarList
                rows={d.cash_accounts.map((a) => ({
                  key: a.account.id,
                  label: `${a.account.code} · ${a.account.name} (${t.types[a.type] ?? a.type})`,
                  value: a.balance,
                  href: has('finance.cash_account:read') ? `${financePage(companyId, 'cash-accounts')}/${a.account.id}` : undefined,
                }))}
              />
            </Panel>
            <Panel title={t.recent} className="lg:col-span-2">
              <RowList
                rows={d.recent.map((r) => ({
                  key: `${r.kind}-${r.id}`,
                  href: has(READ[r.kind]) ? `${financePage(companyId, MENU[r.kind])}/${r.id}` : null,
                  title: `${r.document_no} · ${t.kinds[r.kind] ?? r.kind}`,
                  meta: [formatDate(r.date), r.party].filter(Boolean).join(' · '),
                  value: r.amount,
                }))}
              />
            </Panel>
            <Panel title={t.wht}>
              <dl className="flex flex-col gap-3 text-sm">
                <div className="flex items-baseline justify-between gap-3">
                  <dt className="text-ink-muted">{t.whtDeducted}</dt>
                  <dd className="shrink-0 font-medium tabular-nums">{formatAmount(d.wht.deducted)}</dd>
                </div>
                <div className="flex items-baseline justify-between gap-3">
                  <dt className="text-ink-muted">{t.whtSuffered}</dt>
                  <dd className="shrink-0 font-medium tabular-nums">{formatAmount(d.wht.suffered)}</dd>
                </div>
              </dl>
            </Panel>
          </DashboardGrid>
        </>
      )}
    </div>
  )
}
