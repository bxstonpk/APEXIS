import Link from 'next/link'
import { Badge } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { formatDate } from '@/lib/format'
import { monthLabel, taxFormPage, TAX_FORMS, type TaxForm, type TaxOverview, type TaxOverviewCell } from './tax'

/** ตาราง 12 เดือน x ภ.ง.ด.3 / ภ.ง.ด.53 / ภ.พ.30 — ยอดภาษี + สถานะการยื่น (แสดงเมื่อมีสิทธิ์ดูทะเบียน) */
export function TaxOverviewTable({ companyId, data }: { companyId: string; data: TaxOverview }) {
  const t = texts.finance.taxFilings
  const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted'
  return (
    <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
      <table className="w-full min-w-[46rem] border-collapse text-sm">
        <thead>
          <tr className="border-b border-line bg-subtle">
            <th scope="col" className={th}>
              {t.columns.month}
            </th>
            {TAX_FORMS.map((f) => (
              <th key={f} scope="col" className={`${th} text-right`}>
                {t.forms[f]}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.months.map((m) => (
            <tr key={m.month} className="border-b border-line last:border-b-0">
              <th scope="row" className="px-3 py-3 text-left font-medium whitespace-nowrap">
                {monthLabel(m.month)}
              </th>
              {TAX_FORMS.map((f) => (
                <td key={f} className="px-3 py-2 align-top">
                  <Cell
                    cell={m[f]}
                    form={f}
                    href={taxFormPage(companyId, f, m.month)}
                    label={format(t.monthTitle, { form: t.forms[f]!, month: monthLabel(m.month) })}
                    showFiling={data.can_see_filings}
                  />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Cell({ cell, form, href, label, showFiling }: { cell: TaxOverviewCell; form: TaxForm; href: string; label: string; showFiling: boolean }) {
  const t = texts.finance.taxFilings
  const units = toUnits(cell.amount)
  // เดือนที่ยังไม่ถึง / ภ.ง.ด. ที่ไม่มีภาษีหัก (ไม่ต้องยื่น) และยังไม่มีบันทึก = ช่องว่าง
  if (!cell.filing && (!cell.computed || (form !== 'pp30' && units === 0n))) return <span className="block text-right text-ink-subtle">{t.status.future}</span>
  const amount = form === 'pp30' && units < 0n ? format(t.overpaid, { amount: formatAmount(-units) }) : formatAmount(cell.amount, { dashZero: true })
  return (
    <Link
      href={href}
      aria-label={label}
      className="-mx-2 -my-1 flex flex-col items-end gap-1 rounded-(--radius-control) px-2 py-1 hover:bg-subtle focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none"
    >
      <span className="font-medium tabular-nums">{amount}</span>
      {form !== 'pp30' && cell.payees > 0 && <span className="text-[12px] text-ink-muted">{format(t.payees, { n: String(cell.payees) })}</span>}
      {showFiling &&
        (cell.filing ? (
          <Badge tone="success" dot>
            {t.status.filed} {formatDate(cell.filing.filed_date)}
          </Badge>
        ) : (
          <Badge tone="neutral" dot>
            {t.status.notFiled}
          </Badge>
        ))}
    </Link>
  )
}
