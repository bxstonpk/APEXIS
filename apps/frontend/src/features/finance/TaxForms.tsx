import { format, texts } from '@/config'
import { formatAmount, sum, toString as unitsToString, toUnits } from '@/features/accounting/money'
import { bahtText } from '@/features/sales/calc'
import { branchLabel, letterheadAddress } from '@/features/sales/PrintDocument'
import type { Letterhead } from '@/features/sales/types'
import { formatDate } from '@/lib/format'
import { monthLabel, paginate, type Pp30, type VatReport, type WhtForm, type WhtFormLine } from './tax'

const t = () => texts.finance.taxFilings

/** บรรทัดของ ภ.พ.30 (ยอดจากภาษีขาย / ซื้อที่เกิดแล้วของเดือน) — ใช้ทั้งบนจอและพิมพ์ */
export function Pp30Lines({ data, print = false }: { data: Pp30; print?: boolean }) {
  const p = t().pp30
  const rows: [string, string, string, boolean?][] = [
    ['1', p.sales, data.sales.base],
    ['2', p.zeroRated, data.zero_rated],
    ['3', p.exempt, data.exempt],
    ['4', p.taxableSales, data.sales.base],
    ['5', p.outputTax, data.sales.tax],
    ['6', p.purchases, data.purchases.base],
    ['7', p.inputTax, data.purchases.tax],
    ['8', p.payable, data.payable, true],
    ['9', p.overpaid, data.overpaid, true],
    ['10', p.carryIn, data.carry_in],
  ]
  return (
    <div className={print ? 'border border-neutral-500' : 'rounded-(--radius-panel) border border-line bg-surface shadow-card'}>
      <table className="w-full border-collapse text-sm">
        <tbody>
          {rows.map(([no, label, value, strong]) => (
            <tr key={no} className={print ? 'border-b border-neutral-400 last:border-b-0' : 'border-b border-line last:border-b-0'}>
              <td className="w-10 px-3 py-2.5 text-right text-ink-muted tabular-nums">{no}.</td>
              <th scope="row" className={`px-3 py-2.5 text-left ${strong ? 'font-semibold' : 'font-normal'}`}>
                {label}
              </th>
              <td className={`px-4 py-2.5 text-right tabular-nums whitespace-nowrap ${strong ? 'font-semibold' : ''}`}>{formatAmount(value)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Payer({ letterhead, label }: { letterhead: Letterhead; label: string }) {
  const p = t().payer
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-[12px]">
      <dt className="text-neutral-600">{label}</dt>
      <dd className="font-semibold">{letterhead.legal_name_th || letterhead.name}</dd>
      <dt className="text-neutral-600">{p.taxId}</dt>
      <dd className="tabular-nums">
        {letterhead.tax_id || '-'} · {p.branch} {branchLabel('00000')}
      </dd>
      <dt className="text-neutral-600">{p.address}</dt>
      <dd>{letterheadAddress(letterhead) || '-'}</dd>
    </dl>
  )
}

const LINES_PER_PAGE = 12

/** ใบแนบ ภ.ง.ด.3 / 53 — A4 แนวนอน แผ่นละ 12 บรรทัด รวมหน้า + รวมทั้งสิ้นที่แผ่นสุดท้าย */
export function WhtFormPrint({ letterhead, data }: { letterhead: Letterhead; data: WhtForm }) {
  const tt = t()
  const l = tt.lines
  const pages = paginate<WhtFormLine>(data.lines, LINES_PER_PAGE)
  const cell = 'border border-neutral-500 px-1.5 py-1'
  const num = `${cell} text-right tabular-nums whitespace-nowrap`
  return (
    <>
      {pages.map((rows, pi) => (
        <article key={pi} className="a4-landscape mx-auto mb-6 flex flex-col bg-white text-[11px] leading-snug text-neutral-900 shadow-card print:mb-0 print:shadow-none">
          <header className="flex items-start justify-between gap-6">
            <Payer letterhead={letterhead} label={tt.payer.title} />
            <div className="text-right">
              <h1 className="text-[16px] font-bold">{format(tt.attachment, { form: tt.forms[data.form]! })}</h1>
              <p className="text-[12px]">{format(tt.period, { month: monthLabel(data.month) })}</p>
              <p className="mt-1 text-[11px] text-neutral-600 tabular-nums">{format(tt.page, { page: String(pi + 1), pages: String(pages.length) })}</p>
            </div>
          </header>
          <table className="mt-3 w-full border-collapse">
            <thead>
              <tr className="bg-neutral-100 text-[10.5px]">
                <th scope="col" className={`${cell} w-10`}>{l.seq}</th>
                <th scope="col" className={`${cell} w-32`}>{l.taxId}</th>
                <th scope="col" className={cell}>{l.name} / {l.address}</th>
                <th scope="col" className={`${cell} w-20`}>{l.date}</th>
                <th scope="col" className={`${cell} w-36`}>{l.income}</th>
                <th scope="col" className={`${cell} w-12`}>{l.rate}</th>
                <th scope="col" className={`${cell} w-24`}>{l.base}</th>
                <th scope="col" className={`${cell} w-20`}>{l.wht}</th>
                <th scope="col" className={`${cell} w-12`}>{l.condition}</th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 && (
                <tr>
                  <td colSpan={9} className={`${cell} py-6 text-center text-neutral-500`}>{tt.noRows}</td>
                </tr>
              )}
              {rows.map((r, i) => {
                const first = i === 0 || rows[i - 1]!.seq !== r.seq
                return (
                  <tr key={`${r.payment_id}-${i}`} className="break-inside-avoid align-top">
                    <td className={`${cell} text-center tabular-nums`}>{first ? r.seq : ''}</td>
                    <td className={`${cell} tabular-nums`}>
                      {first && (
                        <>
                          {r.tax_id}
                          <span className="block text-[10px] text-neutral-600">{branchLabel(r.branch_no)}</span>
                        </>
                      )}
                    </td>
                    <td className={cell}>
                      {first && (
                        <>
                          <span className="font-medium">{r.name}</span>
                          <span className="block text-[10px] text-neutral-600">{r.address}</span>
                        </>
                      )}
                    </td>
                    <td className={`${cell} text-center tabular-nums whitespace-nowrap`}>{first ? formatDate(r.date) : ''}</td>
                    <td className={cell}>{texts.finance.whtTypes.incomeTypesShort[r.income_type] ?? texts.finance.whtTypes.incomeTypes[r.income_type] ?? r.income_type}</td>
                    <td className={num}>{r.rate}</td>
                    <td className={num}>{formatAmount(r.base)}</td>
                    <td className={num}>{formatAmount(r.wht)}</td>
                    <td className={`${cell} text-center`}>{r.condition}</td>
                  </tr>
                )
              })}
            </tbody>
            <tfoot>
              <tr className="font-semibold">
                <td colSpan={6} className={`${cell} text-right`}>{tt.pageTotal}</td>
                <td className={num}>{formatAmount(unitsToString(sum(rows.map((r) => r.base))))}</td>
                <td className={num}>{formatAmount(unitsToString(sum(rows.map((r) => r.wht))))}</td>
                <td className={cell} />
              </tr>
              {pi === pages.length - 1 && (
                <tr className="font-bold">
                  <td colSpan={6} className={`${cell} text-right`}>
                    {tt.grandTotal} ({format(tt.payees, { n: String(data.summary.payees) })})
                  </td>
                  <td className={num}>{formatAmount(data.summary.base)}</td>
                  <td className={num}>{formatAmount(data.summary.wht)}</td>
                  <td className={cell} />
                </tr>
              )}
            </tfoot>
          </table>
          {pi === pages.length - 1 && <p className="mt-2 text-right text-[11px]">({bahtText(data.summary.wht)})</p>}
        </article>
      ))}
    </>
  )
}

/** ภ.พ.30 — A4 แนวตั้ง */
export function Pp30Print({ letterhead, data }: { letterhead: Letterhead; data: Pp30 }) {
  const tt = t()
  return (
    <article className="a4-sheet mx-auto mb-6 flex flex-col gap-4 bg-white text-[12px] leading-snug text-neutral-900 shadow-card print:mb-0 print:shadow-none">
      <header className="text-center">
        <h1 className="text-[17px] font-bold">{tt.formNames.pp30}</h1>
        <p>{format(tt.period, { month: monthLabel(data.month) })}</p>
      </header>
      <Payer letterhead={letterhead} label={tt.company} />
      <Pp30Lines data={data} print />
      <p className="text-right font-semibold">
        {toUnits(data.payable) > 0n ?`${tt.pp30.netPayable} ${formatAmount(data.payable)} (${bahtText(data.payable)})` : `${tt.pp30.netOverpaid} ${formatAmount(data.overpaid)}`}
      </p>
    </article>
  )
}

const VAT_ROWS_PER_PAGE = 20

/** รายงานภาษีขาย / ภาษีซื้อตามแบบกรมสรรพากร — A4 แนวนอน แผ่นละ 20 บรรทัด รวมหน้า + รวมทั้งสิ้น */
export function VatReportPrint({ letterhead, data }: { letterhead: Letterhead; data: VatReport }) {
  const tt = t()
  const v = tt.vatReport
  const pages = paginate(data.rows, VAT_ROWS_PER_PAGE)
  const cell = 'border border-neutral-500 px-1.5 py-1'
  const num = `${cell} text-right tabular-nums whitespace-nowrap`
  const title = data.kind === 'output' ? v.output : v.input
  return (
    <>
      {pages.map((rows, pi) => (
        <article key={pi} className="a4-landscape mx-auto mb-6 flex flex-col bg-white text-[11px] leading-snug text-neutral-900 shadow-card print:mb-0 print:shadow-none">
          <header className="flex items-start justify-between gap-6">
            <Payer letterhead={letterhead} label={tt.company} />
            <div className="text-right">
              <h1 className="text-[16px] font-bold">{title}</h1>
              <p className="text-[12px]">{format(tt.period, { month: monthLabel(data.month) })}</p>
              <p className="mt-1 text-[11px] text-neutral-600 tabular-nums">{format(tt.page, { page: String(pi + 1), pages: String(pages.length) })}</p>
            </div>
          </header>
          <table className="mt-3 w-full border-collapse">
            <thead>
              <tr className="bg-neutral-100 text-[10.5px]">
                <th scope="col" className={`${cell} w-10`}>{v.seq}</th>
                <th scope="col" className={`${cell} w-20`}>{v.date}</th>
                <th scope="col" className={`${cell} w-32`}>{v.no}</th>
                <th scope="col" className={cell}>{data.kind === 'output' ? v.partyOutput : v.partyInput}</th>
                <th scope="col" className={`${cell} w-32`}>{v.taxId}</th>
                <th scope="col" className={`${cell} w-24`}>{v.branch}</th>
                <th scope="col" className={`${cell} w-28`}>{v.base}</th>
                <th scope="col" className={`${cell} w-24`}>{v.tax}</th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 && (
                <tr>
                  <td colSpan={8} className={`${cell} py-6 text-center text-neutral-500`}>{tt.noRows}</td>
                </tr>
              )}
              {rows.map((r) => (
                <tr key={r.seq} className="break-inside-avoid">
                  <td className={`${cell} text-center tabular-nums`}>{r.seq}</td>
                  <td className={`${cell} text-center tabular-nums whitespace-nowrap`}>{formatDate(r.date)}</td>
                  <td className={`${cell} tabular-nums`}>{r.document_no}</td>
                  <td className={cell}>{r.party_name}</td>
                  <td className={`${cell} tabular-nums`}>{r.tax_id}</td>
                  <td className={cell}>{branchLabel(r.branch_no)}</td>
                  <td className={num}>{formatAmount(r.base)}</td>
                  <td className={num}>{formatAmount(r.tax)}</td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr className="font-semibold">
                <td colSpan={6} className={`${cell} text-right`}>{tt.pageTotal}</td>
                <td className={num}>{formatAmount(unitsToString(sum(rows.map((r) => r.base))))}</td>
                <td className={num}>{formatAmount(unitsToString(sum(rows.map((r) => r.tax))))}</td>
              </tr>
              {pi === pages.length - 1 && (
                <tr className="font-bold">
                  <td colSpan={6} className={`${cell} text-right`}>{tt.grandTotal}</td>
                  <td className={num}>{formatAmount(data.base)}</td>
                  <td className={num}>{formatAmount(data.tax)}</td>
                </tr>
              )}
            </tfoot>
          </table>
        </article>
      ))}
    </>
  )
}
