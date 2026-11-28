import type { Metadata } from 'next'
import { texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { loadItem } from '@/features/organization/pages'
import { pcApi, pcPage, type Voucher } from '@/features/petty-cash/types'
import { tableDocument } from '@/features/print-templates/data'
import { TemplatedPrint } from '@/features/print-templates/TemplatedPrint'
import { printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.pettyCash.vouchers.print.title }

/** พิมพ์ A4 ผ่านแม่แบบ petty_cash.voucher: ใบสำคัญจ่ายเงินสดย่อย — ผู้รับเงิน / รายการ / ยอด VAT / จำนวนเงินตัวอักษร / ลงนาม 3 ฝ่าย */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.vouchers
  const p = t.print
  const f = t.fields
  const res = await loadItem<Voucher>(pcApi.vouchers(companyId), docId, t.notFound)
  if ('panel' in res) return res.panel
  const v = res.item
  const hasVat = toUnits(v.vat_rate) > 0n
  const totalRows: [string, string][] = hasVat
    ? [
        [t.totals.net, v.totals.net],
        [`${t.totals.vat} ${formatAmount(v.vat_rate)}%`, v.totals.vat],
        [t.totals.gross, v.totals.gross],
      ]
    : [[t.totals.gross, v.totals.gross]]
  const data = tableDocument({
    letterhead: null,
    companyName: ctx.company.name,
    copyLabel: '',
    title: p.title,
    docNo: v.document_no,
    info: [
      [f.date, printDate(v.date)],
      [f.fund, v.fund.code],
      ...(hasVat && v.tax_invoice
        ? ([
            [f.taxInvoiceNo, v.tax_invoice.no],
            [f.taxInvoiceDate, printDate(v.tax_invoice.date)],
          ] as [string, string][])
        : []),
    ],
    party: { label: f.payee, name: v.payee.name, taxId: v.payee.tax_id, branchNo: v.payee.branch_no, extra: v.description ? [v.description] : [] },
    columns: [
      { label: p.seq, align: 'center', width: '3rem' },
      { label: f.lineDescription, align: 'left', width: '' },
      { label: f.account, align: 'left', width: '12rem' },
      { label: f.amount, align: 'right', width: '8rem' },
    ],
    lines: v.lines.map((l, i) => ({
      no: i + 1,
      description: l.description,
      detail: '',
      quantity: '',
      unit_price: '',
      amount: l.amount,
      cells: [
        { value: String(i + 1), align: 'center' as const },
        { value: l.description },
        { value: `${l.account.code} ${l.account.name}` },
        { value: formatAmount(l.amount), align: 'right' as const },
      ],
    })),
    totals: totalRows.map(([label, raw], i) => ({ label, value: formatAmount(raw), raw, strong: i === totalRows.length - 1 })),
    amountInWords: v.totals.gross,
    signatures: [p.signPayee, p.signCustodian, p.signApprover],
  })
  return (
    <>
      <PrintToolbar backHref={`${pcPage(companyId, 'vouchers')}/${v.id}`} />
      <TemplatedPrint docType="petty_cash.voucher" data={data} />
    </>
  )
}
