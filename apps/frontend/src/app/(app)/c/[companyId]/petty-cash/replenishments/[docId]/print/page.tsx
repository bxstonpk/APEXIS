import type { Metadata } from 'next'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { loadItem } from '@/features/organization/pages'
import { pcApi, pcPage, type Replenishment } from '@/features/petty-cash/types'
import { tableDocument } from '@/features/print-templates/data'
import { TemplatedPrint } from '@/features/print-templates/TemplatedPrint'
import { printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.pettyCash.requests.print.title }

/** พิมพ์ A4 ผ่านแม่แบบ petty_cash.replenishment: ใบขอเบิกชดเชยเงินสดย่อย — ใบสำคัญจ่ายที่แนบ + ยอดรวม + ลงนาม 3 ฝ่าย */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.requests
  const p = t.print
  const f = t.fields
  const c = texts.pettyCash.vouchers.columns
  const res = await loadItem<Replenishment>(pcApi.requests(companyId), docId, t.notFound)
  if ('panel' in res) return res.panel
  const r = res.item
  const replenish = r.kind === 'replenish'
  const data = tableDocument({
    letterhead: null,
    companyName: ctx.company.name,
    copyLabel: '',
    title: p.title,
    docNo: r.document_no,
    info: [
      [f.date, printDate(r.date)],
      [f.kind, texts.pettyCash.requestKinds[r.kind] ?? r.kind],
    ],
    party: { label: f.fund, name: `${r.fund.code} ${r.fund.name}`, extra: r.description ? [r.description] : [] },
    columns: replenish
      ? [
          { label: p.seq, align: 'center', width: '3rem' },
          { label: c.date, align: 'left', width: '6rem' },
          { label: c.no, align: 'left', width: '8rem' },
          { label: c.payee, align: 'left', width: '' },
          { label: c.gross, align: 'right', width: '8rem' },
        ]
      : [],
    lines: replenish
      ? r.vouchers.map((v, i) => ({
          no: i + 1,
          description: v.payee_name,
          detail: v.description,
          quantity: '',
          unit_price: '',
          amount: v.totals.gross,
          cells: [
            { value: String(i + 1), align: 'center' as const },
            { value: printDate(v.date) },
            { value: v.document_no },
            { value: v.payee_name, detail: v.description },
            { value: formatAmount(v.totals.gross), align: 'right' as const },
          ],
        }))
      : [],
    totals: [{ label: p.total, value: formatAmount(r.amount), raw: r.amount, strong: true }],
    amountInWords: r.amount,
    signatures: [p.signRequester, p.signApprover, p.signFinance],
  })
  return (
    <>
      <PrintToolbar backHref={`${pcPage(companyId, 'replenishments')}/${r.id}`} />
      <TemplatedPrint docType="petty_cash.replenishment" data={data} />
    </>
  )
}
