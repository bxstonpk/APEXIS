import { CircleAlert, CircleCheck } from 'lucide-react'
import type { Metadata } from 'next'
import Link from 'next/link'
import { Alert, Badge } from '@/components'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ledgerApi, ledgerPage, perms, type LedgerSettings, type YearEnd } from '@/features/accounting/types'
import { LedgerSettingsForm, YearEndActions } from '@/features/accounting/YearEnd'
import { formatDate } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.yearEnd.pageTitle }

const deadlineTone = { on_time: 'success', overdue: 'warning', closed: 'neutral' } as const

/**
 * ปิดบัญชีสิ้นปี — ทุกปีบัญชี (ล่าสุดก่อน): สถานะ, กำหนดปิดบัญชี (สิ้นปี + 120 วัน), เงื่อนไขก่อนปิด, ปิด / เปิดปี
 * ปีที่ยังไม่ปิดโหลดรายละเอียด (บัญชีพัก / ค่าตั้ง) พร้อมกัน
 */
export default async function YearEndPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.yearEnd
  const api = ledgerApi.yearEnds(companyId)
  const listRes = await backendGet<YearEnd[]>(api).then(
    (r) => ({ data: r.data }),
    (err: unknown) => ({ error: errorMessage(err) }),
  )
  if ('error' in listRes) return <Alert tone="error">{listRes.error}</Alert>
  const [details, settings] = await Promise.all([
    Promise.all(listRes.data.map((y) => (y.status === 'open' ? backendGet<YearEnd>(`${api}/${y.fiscal_year}`).then((r) => r.data, () => y) : Promise.resolve(y)))),
    backendGet<LedgerSettings>(ledgerApi.ledgerSettings(companyId)).then((r) => r.data, () => null),
  ])
  const canClose = can(ctx.permissions, perms.yearEndClose)
  const entries = ledgerPage.entries(companyId)
  const check = (ok: boolean, label: string) => (
    <li className="flex items-center gap-2 text-sm">
      {ok ? <CircleCheck aria-hidden className="size-4 text-success" /> : <CircleAlert aria-hidden className="size-4 text-danger" />}
      <span>{label}</span>
      <span className="sr-only">{ok ? t.checks.ok : t.checks.fail}</span>
    </li>
  )
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-[22px] leading-tight font-semibold tracking-tight">{t.heading}</h1>
      {details.length === 0 && <p className="text-sm text-ink-muted">{t.empty}</p>}
      {details.map((y) => (
        <section key={y.fiscal_year} aria-labelledby={`fy-${y.fiscal_year}`} className="flex flex-col gap-4 rounded-(--radius-panel) border border-line bg-surface p-5 shadow-card">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="flex flex-col gap-1.5">
              <h2 id={`fy-${y.fiscal_year}`} className="flex flex-wrap items-center gap-2 text-[17px] font-semibold">
                {format(t.yearTitle, { year: String(y.fiscal_year + 543) })}
                <Badge tone={y.status === 'closed' ? 'success' : 'warning'} dot>
                  {t.statuses[y.status]}
                </Badge>
              </h2>
              <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
                <dt className="text-ink-muted">{t.columns.period}</dt>
                <dd>
                  {formatDate(y.start_date)} – {formatDate(y.end_date)}
                </dd>
                <dt className="text-ink-muted">{t.columns.deadline}</dt>
                <dd className="flex flex-wrap items-center gap-2">
                  {formatDate(y.deadline)}
                  <Badge tone={deadlineTone[y.deadline_status]}>{t.deadlineStatuses[y.deadline_status]}</Badge>
                </dd>
                {y.closing && y.status === 'closed' && (
                  <>
                    <dt className="text-ink-muted">{t.columns.netIncome}</dt>
                    <dd className="tabular-nums">{formatAmount(y.closing.net_income)}</dd>
                    <dt className="text-ink-muted">{t.columns.entry}</dt>
                    <dd>
                      {y.closing.entry_id ? (
                        <Link href={`${entries}/${y.closing.entry_id}`} className="font-mono text-[13px] text-primary hover:underline">
                          {y.closing.entry_no}
                        </Link>
                      ) : (
                        '–'
                      )}
                    </dd>
                  </>
                )}
              </dl>
            </div>
            <YearEndActions api={api} y={y} canClose={canClose} />
          </div>
          {y.status === 'open' && (
            <div>
              <h3 className="mb-2 text-[13px] font-medium text-ink-muted">{t.checks.title}</h3>
              <ul className="flex flex-col gap-1.5">
                {check(y.open_periods === 0, format(t.checks.periods, { closed: String(y.periods - y.open_periods), total: String(y.periods) }))}
                {check(y.suspense.length === 0, t.checks.suspense)}
                {y.suspense.map((s) => (
                  <li key={s.account.id} className="ml-6 text-[13px] text-ink-muted tabular-nums">
                    {s.account.code} {s.account.name} · {formatAmount(s.balance)}
                  </li>
                ))}
                {check(y.settings_ready, t.checks.settings)}
              </ul>
            </div>
          )}
        </section>
      ))}
      <section aria-labelledby="ledger-settings" className="flex flex-col gap-4 rounded-(--radius-panel) border border-line bg-surface p-5 shadow-card">
        <h2 id="ledger-settings" className="text-[15px] font-semibold">
          {t.settings.title}
        </h2>
        <LedgerSettingsForm
          api={{ settings: ledgerApi.ledgerSettings(companyId), accounts: ledgerApi.accounts(companyId), journals: ledgerApi.journals(companyId) }}
          initial={settings}
          readOnly={!can(ctx.permissions, perms.accountManage)}
        />
      </section>
    </div>
  )
}
