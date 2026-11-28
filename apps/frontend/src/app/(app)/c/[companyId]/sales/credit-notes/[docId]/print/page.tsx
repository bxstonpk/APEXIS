import type { Metadata } from 'next'
import { Alert } from '@/components'
import { format, texts } from '@/config'
import { toUnits } from '@/features/accounting/money'
import { creditNotePrintTitle, type SalesCreditNote } from '@/features/credit-notes/types'
import { loadItem } from '@/features/organization/pages'
import { loadLetterhead } from '@/features/sales/pages'
import { PrintDocument, printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { salesApi, salesPage } from '@/features/sales/types'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.creditNotes.print.title }

/**
 * พิมพ์ใบลดหนี้ A4 (ประมวลรัษฎากร ม.86/10) — คำว่า "ใบลดหนี้", ชื่อ / เลขผู้เสียภาษีของผู้ขาย-ผู้ซื้อ (bill_to เดียวกับใบกำกับเดิม),
 * อ้างอิงเลขที่และวันที่ใบกำกับเดิม, มูลค่าเดิม / มูลค่าที่ถูกต้อง / ผลต่าง, ภาษีของผลต่าง และเหตุผลที่ลดหนี้
 */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const p = texts.sales.print
  const c = texts.creditNotes
  const [res, letterhead] = await Promise.all([loadItem<SalesCreditNote>(salesApi.creditNotes(companyId), docId, c.sales.notFound), loadLetterhead(companyId)])
  if ('panel' in res) return res.panel
  const d = res.item
  if (!letterhead) return <Alert tone="error">{p.unavailable}</Alert>
  const vat = toUnits(d.vat_rate) > 0n
  return (
    // ภาษียังไม่ถึงกำหนด (ใบแจ้งหนี้ค่าบริการที่ยังไม่รับเงิน) = ไม่ใช่ใบกำกับภาษี → "ใบลดหนี้"
    <>
      <PrintToolbar backHref={`${salesPage(companyId, 'credit-notes')}/${d.id}`} />
      {d.status === 'cancelled' && (
        <div className="mx-auto mb-4 max-w-[210mm] print:hidden">
          <Alert tone="error">{p.cancelled}</Alert>
        </div>
      )}
      <PrintDocument
        docType="sales.credit_note"
        letterhead={letterhead}
        title={creditNotePrintTitle(vat, d.vat_kind)}
        docNo={d.document_no}
        info={[
          [p.date, printDate(d.date)],
          [c.fields.invoice, d.invoice.document_no],
          [c.fields.invoiceDate, printDate(d.invoice_date)],
        ]}
        party={{ label: p.customer, name: d.bill_to.name || d.customer.name, taxId: d.bill_to.tax_id, branchNo: d.bill_to.branch_no, address: d.bill_to.address }}
        lines={d.lines.map((l) => ({
          description: l.item.name,
          detail: toUnits(l.quantity) === 0n ? c.kindLabel.allowance : undefined,
          quantity: toUnits(l.quantity) === 0n ? undefined : l.quantity,
          unitPrice: toUnits(l.quantity) === 0n ? undefined : l.unit_price,
          amount: d.price_includes_vat ? l.amounts.gross : l.amounts.net,
        }))}
        totals={[
          { label: c.fields.original, value: d.original_totals.net },
          { label: c.fields.corrected, value: d.correct_totals.net },
          { label: c.fields.difference, value: d.totals.net },
          ...(vat ? [{ label: format(p.vat, { rate: d.vat_rate }), value: d.totals.vat }] : []),
          { label: c.form.gross, value: d.totals.gross, strong: true },
        ]}
        amountInWords={d.totals.gross}
        note={`${c.print.reason}: ${d.reason}${d.description ? ` — ${d.description}` : ''}`}
        signatures={[p.customerSign, p.approver]}
      />
    </>
  )
}
