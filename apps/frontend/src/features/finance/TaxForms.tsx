import { format, texts } from '@/config'
import { formatAmount, sum, toString as unitsToString, toUnits } from '@/features/accounting/money'
import { branchLabel, companyPrint, letterheadAddress } from '@/features/print-templates/data'
import { TemplatedPrint } from '@/features/print-templates/TemplatedPrint'
import { bahtText } from '@/features/sales/calc'
import type { Letterhead } from '@/features/sales/types'
import { monthLabel, paginate, type Pp30, type VatReport, type WhtForm, type WhtFormLine } from './tax'

const t = () => texts.finance.taxFilings

/** บรรทัดของ ภ.พ.30 (ยอดจากภาษีขาย / ซื้อที่เกิดแล้วของเดือน) — แสดงบนจอ (ใบพิมพ์ใช้แม่แบบ finance.pp30) */
export function Pp30Lines({ data, print = false }: { data: Pp30; print?: boolean }) {
  return (
    <div className={print ? 'border border-neutral-500' : 'rounded-(--radius-panel) border border-line bg-surface shadow-card'}>
      <table className="w-full border-collapse text-sm">
        <tbody>
          {pp30Rows(data).map(({ no, label, value, strong }) => (
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

function pp30Rows(data: Pp30) {
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
  return rows.map(([no, label, value, strong]) => ({ no, label, value, strong: !!strong }))
}

function payer(letterhead: Letterhead, label: string) {
  return {
    label,
    name: letterhead.legal_name_th || letterhead.name,
    tax_id: letterhead.tax_id,
    branch_label: branchLabel('00000'),
    address: letterheadAddress(letterhead),
  }
}

function payerLabels() {
  const p = t().payer
  return { tax_id: p.taxId, branch: p.branch, address: p.address }
}

const pageLabel = (i: number, n: number) => format(t().page, { page: String(i + 1), pages: String(n) })

const LINES_PER_PAGE = 12

/** ข้อมูลพิมพ์ใบแนบ ภ.ง.ด.3 / 53 — แผ่นละ 12 บรรทัด (บรรทัดแรกของผู้รับเงินแต่ละรายแสดงข้อมูลผู้รับเงิน) */
export function withholdingData(letterhead: Letterhead, data: WhtForm) {
  const tt = t()
  const l = tt.lines
  const pages = paginate<WhtFormLine>(data.lines, LINES_PER_PAGE)
  return {
    company: companyPrint(letterhead),
    payer: payer(letterhead, tt.payer.title),
    form: data.form,
    month: data.month,
    title: format(tt.attachment, { form: tt.forms[data.form]! }),
    period: format(tt.period, { month: monthLabel(data.month) }),
    pages: pages.map((rows, pi) => ({
      page_label: pageLabel(pi, pages.length),
      last: pi === pages.length - 1,
      page_base: unitsToString(sum(rows.map((r) => r.base))),
      page_wht: unitsToString(sum(rows.map((r) => r.wht))),
      rows: rows.map((r, i) => ({
        first: i === 0 || rows[i - 1]!.seq !== r.seq,
        seq: r.seq,
        tax_id: r.tax_id,
        branch_label: branchLabel(r.branch_no),
        name: r.name,
        address: r.address,
        date: r.date,
        income: texts.finance.whtTypes.incomeTypesShort[r.income_type] ?? texts.finance.whtTypes.incomeTypes[r.income_type] ?? r.income_type,
        rate: r.rate,
        base: r.base,
        wht: r.wht,
        condition: r.condition,
      })),
    })),
    summary: { payees: data.summary.payees, payees_label: format(tt.payees, { n: String(data.summary.payees) }), base: data.summary.base, wht: data.summary.wht },
    labels: {
      ...payerLabels(),
      seq: l.seq,
      name_address: `${l.name} / ${l.address}`,
      date: l.date,
      income: l.income,
      rate: l.rate,
      base: l.base,
      wht: l.wht,
      condition: l.condition,
      page_total: tt.pageTotal,
      grand_total: tt.grandTotal,
      no_rows: tt.noRows,
    },
  }
}

export function WhtFormPrint({ letterhead, data }: { letterhead: Letterhead; data: WhtForm }) {
  return <TemplatedPrint docType={data.form === 'pnd3' ? 'finance.pnd3' : 'finance.pnd53'} data={withholdingData(letterhead, data)} />
}

/** ข้อมูลพิมพ์ ภ.พ.30 */
export function pp30Data(letterhead: Letterhead, data: Pp30) {
  const tt = t()
  return {
    company: companyPrint(letterhead),
    payer: payer(letterhead, tt.company),
    month: data.month,
    title: tt.formNames.pp30,
    period: format(tt.period, { month: monthLabel(data.month) }),
    rows: pp30Rows(data),
    payable: data.payable,
    overpaid: data.overpaid,
    net_text:
      toUnits(data.payable) > 0n ? `${tt.pp30.netPayable} ${formatAmount(data.payable)} (${bahtText(data.payable)})` : `${tt.pp30.netOverpaid} ${formatAmount(data.overpaid)}`,
    labels: payerLabels(),
  }
}

export function Pp30Print({ letterhead, data }: { letterhead: Letterhead; data: Pp30 }) {
  return <TemplatedPrint docType="finance.pp30" data={pp30Data(letterhead, data)} />
}

const VAT_ROWS_PER_PAGE = 20

/** ข้อมูลพิมพ์รายงานภาษีขาย / ภาษีซื้อ — แผ่นละ 20 บรรทัด */
export function vatReportData(letterhead: Letterhead, data: VatReport) {
  const tt = t()
  const v = tt.vatReport
  const pages = paginate(data.rows, VAT_ROWS_PER_PAGE)
  return {
    company: companyPrint(letterhead),
    payer: payer(letterhead, tt.company),
    kind: data.kind,
    month: data.month,
    title: data.kind === 'output' ? v.output : v.input,
    period: format(tt.period, { month: monthLabel(data.month) }),
    pages: pages.map((rows, pi) => ({
      page_label: pageLabel(pi, pages.length),
      last: pi === pages.length - 1,
      page_base: unitsToString(sum(rows.map((r) => r.base))),
      page_tax: unitsToString(sum(rows.map((r) => r.tax))),
      rows: rows.map((r) => ({ ...r, branch_label: branchLabel(r.branch_no) })),
    })),
    base: data.base,
    tax: data.tax,
    labels: {
      ...payerLabels(),
      seq: v.seq,
      date: v.date,
      no: v.no,
      party: data.kind === 'output' ? v.partyOutput : v.partyInput,
      base: v.base,
      tax: v.tax,
      page_total: tt.pageTotal,
      grand_total: tt.grandTotal,
      no_rows: tt.noRows,
    },
  }
}

export function VatReportPrint({ letterhead, data }: { letterhead: Letterhead; data: VatReport }) {
  return <TemplatedPrint docType={data.kind === 'output' ? 'finance.vat_report_output' : 'finance.vat_report_input'} data={vatReportData(letterhead, data)} />
}
