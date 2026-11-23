import { CircleAlert, CircleCheck } from 'lucide-react'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { format, texts } from '@/config'
import { cx } from '@/lib/cx'
import { formatDate } from '@/lib/format'
import { formatAmount, toUnits } from './money'
import type { BalanceSheet, GeneralLedger, IncomeStatement, StatementLine, TrialBalance } from './types'

/**
 * ตารางรายงาน (server) — ตัวเลขชิดขวา tabular-nums, ติดลบในวงเล็บ
 * หน้าพิมพ์: กรอบ / เงาหายไป, หัวตารางซ้ำทุกหน้า (thead), แถวไม่ถูกตัดกลาง
 */

const box = 'overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card print:overflow-visible print:rounded-none print:border-0 print:shadow-none'
const table = 'w-full border-collapse text-sm print:text-[11px]'
const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted print:py-1'
const num = 'px-3 py-2 text-right tabular-nums whitespace-nowrap print:py-0.5'
const cell = 'px-3 py-2 print:py-0.5'
const row = 'border-b border-line break-inside-avoid'

const amount = (v: string, dashZero = false) => formatAmount(v, { dashZero })

export function TrialBalanceTable({ r, ledgerHref }: { r: TrialBalance; ledgerHref: (accountId: string) => string }) {
  const t = texts.reports
  const c = t.columns
  const ok = toUnits(r.total_debit) === toUnits(r.total_credit)
  return (
    <>
      <div className={box}>
        <table className={cx(table, 'min-w-[48rem]')}>
          <thead>
            <tr className="border-b border-line bg-subtle print:bg-transparent">
              <th scope="col" className={th}>
                {c.code}
              </th>
              <th scope="col" className={th}>
                {c.account}
              </th>
              {[c.opening, c.debit, c.credit, c.closing].map((h) => (
                <th key={h} scope="col" className={cx(th, 'text-right')}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {r.rows.length === 0 && (
              <tr>
                <td colSpan={6} className="px-3 py-10 text-center text-ink-muted">
                  {t.noRows}
                </td>
              </tr>
            )}
            {r.rows.map((x) => (
              <tr key={x.account.id} className={row}>
                <td className={cx(cell, 'font-mono text-[13px] text-ink-muted')}>{x.account.code}</td>
                <td className={cell}>
                  <Link href={ledgerHref(x.account.id)} className="text-primary hover:underline print:text-ink print:no-underline">
                    {x.account.name}
                  </Link>
                </td>
                <td className={num}>{amount(x.opening_balance, true)}</td>
                <td className={num}>{amount(x.debit, true)}</td>
                <td className={num}>{amount(x.credit, true)}</td>
                <td className={num}>{amount(x.closing_balance, true)}</td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr className="border-t-2 border-ink/70 font-semibold">
              <td colSpan={3} className={cx(cell, 'text-right')}>
                {t.total}
              </td>
              <td className={cx(num, 'border-b-4 border-double border-ink/70')}>{amount(r.total_debit)}</td>
              <td className={cx(num, 'border-b-4 border-double border-ink/70')}>{amount(r.total_credit)}</td>
              <td />
            </tr>
          </tfoot>
        </table>
      </div>
      <p className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-[13px] text-ink-muted">
        <span className={cx('inline-flex items-center gap-1.5 font-medium', ok ? 'text-success' : 'text-danger')}>
          {ok ? <CircleCheck aria-hidden className="size-4" /> : <CircleAlert aria-hidden className="size-4" />}
          {ok ? t.tbBalanced : t.tbUnbalanced}
        </span>
        <span>{t.balanceHint}</span>
        {toUnits(r.prior_years_earnings) !== 0n && (
          <span>
            {t.priorYears}: <span className="tabular-nums">{amount(r.prior_years_earnings)}</span>
          </span>
        )}
      </p>
    </>
  )
}

/** ส่วนหนึ่งของงบ: หัวส่วน, บรรทัดตามระดับ (บัญชีคุมตัวหนา), บรรทัดรวม */
function Section({ title, lines, extra, totalLabel, total }: { title: string; lines: StatementLine[]; extra?: ReactNode; totalLabel: string; total: string }) {
  return (
    <tbody>
      <tr className="break-after-avoid">
        <th colSpan={2} scope="colgroup" className="bg-subtle px-3 pt-4 pb-2 text-left text-[13px] font-semibold tracking-wide text-ink print:bg-transparent">
          {title}
        </th>
      </tr>
      {lines.length === 0 && !extra && (
        <tr className={row}>
          <td colSpan={2} className={cx(cell, 'text-ink-subtle')}>
            –
          </td>
        </tr>
      )}
      {lines.map((l) => (
        <tr key={l.account_id} className={row}>
          <td className={cx(cell, l.is_header && 'font-semibold')} style={{ paddingLeft: `${12 + l.level * 20}px` }}>
            <span className="mr-2 font-mono text-[12px] text-ink-subtle">{l.code}</span>
            {l.name}
          </td>
          <td className={cx(num, l.is_header && 'font-semibold')}>{amount(l.amount, true)}</td>
        </tr>
      ))}
      {extra}
      <tr className="font-semibold">
        <td className={cx(cell, 'text-right')}>{totalLabel}</td>
        <td className={cx(num, 'border-t border-ink/50')}>{amount(total)}</td>
      </tr>
    </tbody>
  )
}

function GrandTotal({ label, value }: { label: string; value: string }) {
  return (
    <tbody>
      <tr className="text-[15px] font-semibold">
        <td className={cx(cell, 'pt-4 text-right')}>{label}</td>
        <td className={cx(num, 'border-b-4 border-double border-ink/70 pt-4')}>{amount(value)}</td>
      </tr>
    </tbody>
  )
}

function StatementFrame({ children }: { children: ReactNode }) {
  return (
    <div className={box}>
      <table className={cx(table, 'min-w-[32rem]')}>
        <colgroup>
          <col />
          <col className="w-44" />
        </colgroup>
        {children}
      </table>
    </div>
  )
}

export function BalanceSheetTable({ r }: { r: BalanceSheet }) {
  const t = texts.reports
  const suspense = toUnits(r.total_suspense)
  const extraRow = (label: string, v: string) => (
    <tr className={row}>
      <td className={cell}>{label}</td>
      <td className={num}>{amount(v, true)}</td>
    </tr>
  )
  return (
    <div className="flex flex-col gap-4">
      {suspense !== 0n && (
        <p role="status" className="flex items-start gap-2 rounded-(--radius-control) border border-warning/40 bg-warning-soft px-3.5 py-2.5 text-sm text-warning print:border-ink/40">
          <CircleAlert aria-hidden className="mt-0.5 size-4 shrink-0" />
          {format(t.suspenseWarning, { amount: amount(r.total_suspense) })}
        </p>
      )}
      <StatementFrame>
        <Section title={t.sections.assets} lines={r.assets} totalLabel={t.totals.assets} total={r.total_assets} />
        <Section title={t.sections.liabilities} lines={r.liabilities} totalLabel={t.totals.liabilities} total={r.total_liabilities} />
        <Section
          title={t.sections.equity}
          lines={r.equity}
          extra={
            <>
              {extraRow(t.retainedPrior, r.retained_earnings_prior_years)}
              {extraRow(t.currentYear, r.current_year_earnings)}
            </>
          }
          totalLabel={t.totals.equity}
          total={r.total_equity}
        />
        <GrandTotal label={t.totals.liabilitiesAndEquity} value={r.total_liabilities_and_equity} />
        {(r.suspense.length > 0 || suspense !== 0n) && <Section title={t.sections.suspense} lines={r.suspense} totalLabel={t.totals.suspense} total={r.total_suspense} />}
      </StatementFrame>
      {(r.suspense.length > 0 || suspense !== 0n) && <p className="text-[13px] text-ink-muted">{t.suspenseHint}</p>}
    </div>
  )
}

/**
 * งบกำไรขาดทุน — บัญชีพักแสดงเป็นส่วนแยกท้ายงบ (ไม่รวมในกำไรสุทธิ) ให้เห็นยอดที่ยังรอโอนไปบัญชีที่ถูกต้อง
 * งบระหว่างปีจึงเห็นรายการพักชัดเจน; ก่อนปิดงบโอนออกจนเป็นศูนย์แล้วส่วนนี้จะหายไป
 */
export function IncomeStatementTable({ r }: { r: IncomeStatement }) {
  const t = texts.reports
  const suspense = (r.suspense?.length ?? 0) > 0 || toUnits(r.total_suspense) !== 0n
  return (
    <div className="flex flex-col gap-4">
      {suspense && (
        <p role="status" className="flex items-start gap-2 rounded-(--radius-control) border border-warning/40 bg-warning-soft px-3.5 py-2.5 text-sm text-warning print:border-ink/40">
          <CircleAlert aria-hidden className="mt-0.5 size-4 shrink-0" />
          {format(t.suspenseInPeriod, { amount: amount(r.total_suspense) })}
        </p>
      )}
      <StatementFrame>
        <Section title={t.sections.revenue} lines={r.revenue} totalLabel={t.totals.revenue} total={r.total_revenue} />
        <Section title={t.sections.expenses} lines={r.expenses} totalLabel={t.totals.expenses} total={r.total_expenses} />
        <GrandTotal label={t.totals.netIncome} value={r.net_income} />
        {suspense && <Section title={t.sections.suspenseExcluded} lines={r.suspense} totalLabel={t.totals.suspensePeriod} total={r.total_suspense} />}
      </StatementFrame>
      {suspense && <p className="text-[13px] text-ink-muted">{t.suspenseIncomeHint}</p>}
    </div>
  )
}

export function GeneralLedgerTable({ r, entryHref }: { r: GeneralLedger; entryHref: (id: string) => string }) {
  const t = texts.reports
  const c = t.columns
  return (
    <div className={box}>
      <table className={cx(table, 'min-w-[48rem]')}>
        <thead>
          <tr className="border-b border-line bg-subtle print:bg-transparent">
            <th scope="col" className={th}>
              {c.date}
            </th>
            <th scope="col" className={th}>
              {c.no}
            </th>
            <th scope="col" className={th}>
              {c.description}
            </th>
            {[c.debit, c.credit, c.balance].map((h) => (
              <th key={h} scope="col" className={cx(th, 'text-right')}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          <tr className={cx(row, 'bg-subtle/60 font-medium print:bg-transparent')}>
            <td className={cell}>{formatDate(r.start_date)}</td>
            <td className={cell} />
            <td className={cell}>{t.opening}</td>
            <td className={num} />
            <td className={num} />
            <td className={num}>{amount(r.opening_balance)}</td>
          </tr>
          {r.lines.length === 0 && (
            <tr>
              <td colSpan={6} className="px-3 py-10 text-center text-ink-muted">
                {t.noRows}
              </td>
            </tr>
          )}
          {r.lines.map((l, i) => (
            <tr key={`${l.entry_id}-${i}`} className={row}>
              <td className={cx(cell, 'whitespace-nowrap')}>{formatDate(l.date)}</td>
              <td className={cx(cell, 'font-mono text-[13px] whitespace-nowrap')}>
                <Link href={entryHref(l.entry_id)} className="text-primary hover:underline print:text-ink print:no-underline">
                  {l.entry_no}
                </Link>
              </td>
              <td className={cell}>
                {l.description}
                {l.reference && <span className="block text-[12px] text-ink-muted">{l.reference}</span>}
              </td>
              <td className={num}>{amount(l.debit, true)}</td>
              <td className={num}>{amount(l.credit, true)}</td>
              <td className={num}>{amount(l.balance)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
