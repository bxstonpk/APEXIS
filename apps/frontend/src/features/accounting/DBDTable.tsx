import { format, texts } from '@/config'
import { cx } from '@/lib/cx'
import { formatAmount } from './money'
import type { DBDStatement } from './types'

/** ปี พ.ศ. ของคอลัมน์ (ปีบัญชี) */
export const thaiYear = (fiscalYear: number) => String(fiscalYear + 543)

/**
 * งบการเงินตามแบบกรมพัฒนาธุรกิจการค้า (server) — 2 คอลัมน์ ปีปัจจุบัน / ปีก่อน, หน่วยบาท
 * หัวข้อ = ตัวหนาไม่มียอด, ยอดรวม = เส้นคั่นบน, ยอดรวมสุดท้าย = เส้นคู่ล่าง; ติดลบในวงเล็บ
 */
export function DBDTable({ r }: { r: DBDStatement }) {
  const d = texts.reports.dbd
  return (
    <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card print:overflow-visible print:rounded-none print:border-0 print:shadow-none">
      <table className="w-full min-w-[36rem] border-collapse text-sm print:text-[11px]">
        <thead>
          <tr className="border-b border-line bg-subtle print:bg-transparent">
            <th scope="col" className="px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted print:py-1">
              {d.unit}
            </th>
            {r.columns.map((c) => (
              <th key={c.end_date} scope="col" className="w-40 px-3 py-2.5 text-right text-[13px] font-medium text-ink-muted print:py-1">
                {format(d.year, { year: thaiYear(c.fiscal_year) })}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {r.rows.map((row) => (
            <tr
              key={row.key}
              className={cx(
                'break-inside-avoid',
                row.kind === 'subtotal' && 'border-t border-line font-medium',
                row.kind === 'total' && 'border-t border-ink/60 border-b-4 border-b-ink/60 border-double font-semibold',
              )}
            >
              <th
                scope="row"
                className={cx('px-3 py-1.5 text-left font-normal print:py-0.5', row.kind === 'header' && 'pt-3 font-semibold', row.kind !== 'line' && 'font-medium')}
                style={{ paddingLeft: `${0.75 + row.level * 1.25}rem` }}
              >
                {d.lines[row.key] ?? row.key}
              </th>
              {r.columns.map((c, i) => (
                <td key={c.end_date} className="px-3 py-1.5 text-right tabular-nums whitespace-nowrap print:py-0.5">
                  {row.kind === 'header' ? '' : formatAmount(row.amounts[i] ?? '0', { dashZero: true })}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
