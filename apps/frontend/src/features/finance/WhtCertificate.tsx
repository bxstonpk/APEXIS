import { format, texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { branchLabel, letterheadAddress, printDate } from '@/features/sales/PrintDocument'
import { bahtText } from '@/features/sales/calc'
import type { Letterhead } from '@/features/sales/types'
import { INCOME_TYPES, type IncomeType, type Payment } from './types'

/**
 * หนังสือรับรองการหักภาษี ณ ที่จ่าย ตามมาตรา 50 ทวิ แห่งประมวลรัษฎากร (A4, server)
 * - ผู้หักภาษี = หัวกระดาษบริษัท, ผู้ถูกหักภาษี = snapshot ผู้รับเงินบนใบสำคัญจ่าย
 * - แบบ ภ.ง.ด.3 / 53 ตามประเภทผู้เสียภาษีของผู้รับเงิน (บุคคลธรรมดา / นิติบุคคล)
 * - ตารางเงินได้: รวมยอดตามประเภทเงินได้ของประเภทภาษีหัก ณ ที่จ่าย (มาตรา 40 / 3 เตรส); ช่อง 5 / 6 แสดงชื่อประเภท
 * - พิมพ์ 2 ฉบับ (ฉบับที่ 1 แนบแบบแสดงรายการภาษี / ฉบับที่ 2 เก็บเป็นหลักฐาน) แผ่นละ 1 หน้า A4
 */
export function WhtCertificate({ letterhead, payment }: { letterhead: Letterhead; payment: Payment }) {
  const c = texts.finance.whtCertificate
  const rows = groupByIncome(payment)
  const total = rows.reduce((s, r) => ({ base: s.base + r.base, wht: s.wht + r.wht }), { base: 0n, wht: 0n })
  return (
    <>
      {[c.copy1, c.copy2].map((copy) => (
        <article key={copy} className="a4-sheet mx-auto mb-6 flex flex-col bg-white text-[12px] leading-snug text-neutral-900 shadow-card print:mb-0 print:shadow-none">
          <header className="flex items-start justify-between gap-4">
            <p className="max-w-[60%] text-[11px] text-neutral-600">{copy}</p>
            <dl className="grid grid-cols-[auto_auto] gap-x-2 text-[12px]">
              <dt className="text-neutral-600">{c.book}</dt>
              <dd className="tabular-nums">{payment.date.slice(0, 4)}</dd>
              <dt className="text-neutral-600">{c.number}</dt>
              <dd className="font-semibold tabular-nums">{payment.wht_certificate_no}</dd>
            </dl>
          </header>
          <div className="mt-2 text-center">
            <h1 className="text-[18px] font-bold">{c.title}</h1>
            <p className="text-[12px]">{c.subtitle}</p>
          </div>

          <Party
            label={c.payer}
            name={letterhead.legal_name_th || letterhead.name}
            taxId={letterhead.tax_id}
            branch={texts.sales.print.headOffice}
            address={letterheadAddress(letterhead)}
          />
          <Party
            label={c.payee}
            name={payment.party.name}
            taxId={payment.party.tax_id}
            branch={branchLabel(payment.party.branch_no)}
            address={payment.party.address}
          />

          <section className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-1 rounded border border-neutral-500 px-3 py-2">
            <span>
              {c.sequence}{' '}
              <span className="inline-block w-16 border-b border-dotted border-neutral-500 text-center font-semibold tabular-nums">
                {payment.wht_form_seq ? payment.wht_form_seq : ''}
              </span>{' '}
              {c.inForm}
            </span>
            {FORMS.map((f) => (
              <Check key={f} on={formOf(payment) === f} label={c.forms[f]!} />
            ))}
          </section>

          <table className="mt-3 w-full border-collapse border border-neutral-500">
            <thead>
              <tr className="bg-neutral-100 text-[11.5px]">
                <th scope="col" className="border border-neutral-500 px-2 py-1.5 text-left font-semibold">
                  {c.columns.income}
                </th>
                <th scope="col" className="w-28 border border-neutral-500 px-2 py-1.5 font-semibold">
                  {c.columns.date}
                </th>
                <th scope="col" className="w-32 border border-neutral-500 px-2 py-1.5 font-semibold">
                  {c.columns.amount}
                </th>
                <th scope="col" className="w-32 border border-neutral-500 px-2 py-1.5 font-semibold">
                  {c.columns.tax}
                </th>
              </tr>
            </thead>
            <tbody>
              {INCOME_TYPES.map((k) => {
                const r = rows.find((x) => x.type === k)
                return (
                  <tr key={k} className="align-top">
                    <td className="border-x border-neutral-500 px-2 py-1.5">
                      {texts.finance.whtTypes.incomeTypes[k]}
                      {r && r.names.length > 0 && (k === '3_tres' || k === 'other') && <span className="block text-[11px] text-neutral-600">({r.names.join(', ')})</span>}
                    </td>
                    <td className="border-x border-neutral-500 px-2 py-1.5 text-center tabular-nums">{r ? printDate(payment.date) : ''}</td>
                    <td className="border-x border-neutral-500 px-2 py-1.5 text-right tabular-nums">{r ? formatAmount(r.base) : ''}</td>
                    <td className="border-x border-neutral-500 px-2 py-1.5 text-right tabular-nums">{r ? formatAmount(r.wht) : ''}</td>
                  </tr>
                )
              })}
              <tr className="font-semibold">
                <td colSpan={2} className="border border-neutral-500 px-2 py-1.5 text-right">
                  {c.total}
                </td>
                <td className="border border-neutral-500 px-2 py-1.5 text-right tabular-nums">{formatAmount(total.base)}</td>
                <td className="border border-b-4 border-double border-neutral-500 px-2 py-1.5 text-right tabular-nums">{formatAmount(total.wht)}</td>
              </tr>
              <tr>
                <td colSpan={4} className="border border-neutral-500 px-2 py-1.5">
                  {c.totalInWords} <span className="ml-2 rounded bg-neutral-100 px-2 py-0.5 font-semibold">({bahtText(total.wht)})</span>
                </td>
              </tr>
            </tbody>
          </table>

          <p className="mt-2 text-[11px] text-neutral-700">{c.funds}</p>

          <section className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-1 rounded border border-neutral-500 px-3 py-2">
            <span className="font-semibold">{c.payerMode}</span>
            {c.modes.map((m, i) => (
              <Check key={m} on={i === 0} label={m} />
            ))}
          </section>

          <section className="mt-auto grid break-inside-avoid grid-cols-[1fr_auto] items-end gap-6 pt-8">
            <p className="text-[11px] text-neutral-700">{c.warning}</p>
            <div className="w-72 text-center">
              <p>{c.certify}</p>
              <p className="mt-10 border-b border-dotted border-neutral-600" />
              <p className="mt-1">{c.signer}</p>
              <p className="mt-1">{format(c.issuedOn, { date: printDate(payment.date) })}</p>
              <div className="mx-auto mt-2 flex size-16 items-center justify-center rounded-full border border-dashed border-neutral-400 text-[10px] text-neutral-500">{c.seal}</div>
            </div>
          </section>
        </article>
      ))}
    </>
  )
}

const FORMS = ['pnd1a', 'pnd1a_special', 'pnd2', 'pnd3', 'pnd2a', 'pnd3a', 'pnd53'] as const

/** ภ.ง.ด.3 = บุคคลธรรมดา, ภ.ง.ด.53 = นิติบุคคล (ตาม tax_type ของผู้รับเงิน) */
function formOf(p: Payment): (typeof FORMS)[number] | null {
  if (p.party.tax_type === 'individual') return 'pnd3'
  if (p.party.tax_type === 'juristic') return 'pnd53'
  return null
}

type IncomeRow = { type: IncomeType; names: string[]; base: bigint; wht: bigint }

/** รวมเงินที่จ่าย (ฐานภาษี) / ภาษีที่หัก ตามประเภทเงินได้ — หน่วยตาม toUnits (ทศนิยม 6 ตำแหน่ง) */
function groupByIncome(p: Payment): IncomeRow[] {
  const out: IncomeRow[] = []
  for (const l of p.lines) {
    if (!l.wht_type || toUnits(l.wht) === 0n) continue
    const type = l.wht_type.income_type ?? 'other'
    let r = out.find((x) => x.type === type)
    if (!r) out.push((r = { type, names: [], base: 0n, wht: 0n }))
    if (!r.names.includes(l.wht_type.name)) r.names.push(l.wht_type.name)
    r.base += toUnits(l.wht_base)
    r.wht += toUnits(l.wht)
  }
  return out
}

function Party({ label, name, taxId, branch, address }: { label: string; name: string; taxId: string; branch: string; address: string }) {
  const c = texts.finance.whtCertificate
  return (
    <section className="mt-3 rounded border border-neutral-500 px-3 py-2">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4">
        <p className="font-semibold">{label}</p>
        <p>
          {c.taxId} <span className="font-semibold tabular-nums tracking-wider">{taxId || '-'}</span>
          {branch && <span className="ml-2 text-neutral-600">{branch}</span>}
        </p>
      </div>
      <p className="mt-1">
        <span className="text-neutral-600">{c.name}</span> {name}
      </p>
      <p className="mt-0.5">
        <span className="text-neutral-600">{c.address}</span> {address || '-'}
      </p>
    </section>
  )
}

function Check({ on, label }: { on: boolean; label: string }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span aria-hidden className="flex size-3.5 items-center justify-center border border-neutral-700 text-[10px] leading-none font-bold">
        {on ? '✓' : ''}
      </span>
      <span className={on ? 'font-semibold' : ''}>{label}</span>
      <span className="sr-only">{on ? texts.finance.whtCertificate.checked : ''}</span>
    </span>
  )
}
