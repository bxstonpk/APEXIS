import type { ReactNode } from 'react'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { formatDate } from '@/lib/format'
import { bahtText } from './calc'
import type { Letterhead } from './types'

/**
 * เอกสารพิมพ์ A4 (server) — หัวกระดาษบริษัท, ชื่อเอกสาร, ผู้ซื้อ, ตารางรายการ, ยอดรวม + จำนวนเงินตัวอักษร, ช่องลงนาม
 * ใช้กับใบเสนอราคา / ใบแจ้งหนี้ / ใบกำกับภาษี (ม.86/4: ชื่อ ที่อยู่ เลขผู้เสียภาษี และสาขาของผู้ขาย-ผู้ซื้อ) / ใบเสร็จรับเงิน
 * แผ่น A4 จริง (.a4-sheet ใน app.css): จอ = 210×297mm (มือถือย่อทั้งแผ่น), พิมพ์ = @page a4doc ขอบ 12mm ทุกหน้า
 * ช่องลงนามชิดล่างของหน้า; เอกสารยาวเกิน 1 หน้าไหลต่อหน้าถัดไปพร้อมขอบกระดาษ
 */

export type PrintParty = { label: string; name: string; taxId?: string; branchNo?: string; address?: string }
export type PrintLine = { description: string; detail?: string; quantity?: string; unitPrice?: string; amount: string }
export type PrintTotal = { label: string; value: string; strong?: boolean }

export function branchLabel(branchNo: string | undefined): string {
  if (!branchNo) return ''
  return branchNo === '00000' ? texts.sales.print.headOffice : format(texts.sales.print.branch, { no: branchNo })
}

export function letterheadAddress(l: Letterhead): string {
  const a = l.address
  return [a.line, a.sub_district, a.district, a.province, a.postal_code].filter(Boolean).join(' ')
}

export function PrintDocument({
  letterhead,
  title,
  docNo,
  info,
  party,
  lines,
  totals,
  amountInWords,
  note,
  signatures,
}: {
  letterhead: Letterhead
  title: string
  docNo: string
  /** ข้อมูลด้านขวา (วันที่, ครบกำหนด, อ้างอิง ...) — เลขที่แสดงให้อัตโนมัติ */
  info: [string, string][]
  party: PrintParty
  lines: PrintLine[]
  totals: PrintTotal[]
  /** ยอดที่อ่านเป็นตัวอักษร (ปกติ = ยอดรวมทั้งสิ้น) */
  amountInWords: string
  note?: ReactNode
  signatures: string[]
}) {
  const p = texts.sales.print
  const priced = lines.some((l) => l.unitPrice !== undefined)
  const th = 'border-b border-neutral-400 px-2 py-1.5 font-semibold'
  const legal = letterhead.legal_name_th || letterhead.name
  const contact = [letterhead.phone && `โทร ${letterhead.phone}`, letterhead.email, letterhead.website].filter(Boolean).join(' · ')
  return (
    <article className="a4-sheet mx-auto flex flex-col bg-white text-[12.5px] leading-snug text-neutral-900 shadow-card print:shadow-none">
      {/* หัวกระดาษ */}
      <header className="flex items-start justify-between gap-4 border-b-2 border-neutral-800 pb-4">
        <div className="min-w-0">
          <p className="text-[17px] font-bold">{legal}</p>
          {letterhead.legal_name_en && <p className="text-neutral-600">{letterhead.legal_name_en}</p>}
          <p className="mt-1">{letterheadAddress(letterhead)}</p>
          {letterhead.tax_id && (
            <p>
              {p.taxId} {letterhead.tax_id} ({p.headOffice})
            </p>
          )}
          {contact && <p className="text-neutral-600">{contact}</p>}
        </div>
        <div className="shrink-0 text-right">
          <h1 className="text-[20px] font-bold tracking-tight">{title}</h1>
          <p className="mt-0.5 inline-block rounded border border-neutral-500 px-2 text-[12px]">{p.original}</p>
        </div>
      </header>

      {/* ผู้ซื้อ + ข้อมูลเอกสาร */}
      <section className="mt-4 grid grid-cols-[1fr_auto] gap-4">
        <div className="rounded border border-neutral-400 p-3">
          <p className="text-[12px] text-neutral-600">{party.label}</p>
          <p className="font-semibold">{party.name}</p>
          {party.address && <p className="whitespace-pre-line">{party.address}</p>}
          {party.taxId && (
            <p>
              {p.taxId} {party.taxId}
              {party.branchNo && ` (${branchLabel(party.branchNo)})`}
            </p>
          )}
        </div>
        <dl className="grid min-w-[14rem] grid-cols-[auto_1fr] content-start gap-x-4 gap-y-1 rounded border border-neutral-400 p-3">
          {([[p.no, docNo], ...info] as [string, string][])
            .filter(([, v]) => v)
            .map(([k, v]) => (
              <div key={k} className="contents">
                <dt className="text-neutral-600">{k}</dt>
                <dd className="text-right font-medium tabular-nums">{v}</dd>
              </div>
            ))}
        </dl>
      </section>

      {/* รายการ */}
      <table className="mt-4 w-full border-collapse">
        <thead>
          <tr className="bg-neutral-100 text-left">
            <th scope="col" className={`${th} w-10 text-center`}>
              {p.seq}
            </th>
            <th scope="col" className={th}>
              {p.itemDesc}
            </th>
            {priced && (
              <>
                <th scope="col" className={`${th} w-20 text-right`}>
                  {p.qty}
                </th>
                <th scope="col" className={`${th} w-28 text-right`}>
                  {p.unitPrice}
                </th>
              </>
            )}
            <th scope="col" className={`${th} w-32 text-right`}>
              {p.amount}
            </th>
          </tr>
        </thead>
        <tbody>
          {lines.map((l, i) => (
            <tr key={i} className="break-inside-avoid border-b border-neutral-200 align-top">
              <td className="px-2 py-1.5 text-center tabular-nums">{i + 1}</td>
              <td className="px-2 py-1.5">
                {l.description}
                {l.detail && <span className="block text-[12px] text-neutral-600">{l.detail}</span>}
              </td>
              {priced && (
                <>
                  <td className="px-2 py-1.5 text-right tabular-nums">{l.quantity ? formatAmount(l.quantity) : ''}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums">{l.unitPrice ? formatAmount(l.unitPrice) : ''}</td>
                </>
              )}
              <td className="px-2 py-1.5 text-right tabular-nums">{formatAmount(l.amount)}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {/* ยอดรวม */}
      <section className="mt-3 grid break-inside-avoid grid-cols-[1fr_18rem] gap-3">
        <div className="flex flex-col gap-2">
          <p className="rounded bg-neutral-100 px-3 py-2 text-center font-semibold">({bahtText(amountInWords)})</p>
          {note && <div className="text-[12px] text-neutral-700">{note}</div>}
        </div>
        <dl className="grid grid-cols-[1fr_auto]">
          {totals.map((t) => (
            <div key={t.label} className={`contents ${t.strong ? 'font-bold' : ''}`}>
              <dt className={`py-1 ${t.strong ? 'border-t border-b-4 border-double border-neutral-700' : ''}`}>{t.label}</dt>
              <dd className={`py-1 pl-4 text-right tabular-nums ${t.strong ? 'border-t border-b-4 border-double border-neutral-700' : ''}`}>{formatAmount(t.value)}</dd>
            </div>
          ))}
        </dl>
      </section>

      {/* ลงนาม */}
      <footer className="mt-auto grid break-inside-avoid grid-cols-2 gap-8 pt-14">
        {signatures.map((s) => (
          <div key={s} className="text-center">
            <div className="mx-auto h-10 w-56 border-b border-dotted border-neutral-600" />
            <p className="mt-1">{s}</p>
            <p className="text-[12px] text-neutral-600">{p.signDate}</p>
          </div>
        ))}
      </footer>
    </article>
  )
}

export const printDate = (d: string | null | undefined) => (d ? formatDate(d) : '')
