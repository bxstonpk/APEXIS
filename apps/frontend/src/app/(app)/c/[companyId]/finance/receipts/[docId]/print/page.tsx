import type { Metadata } from 'next'
import { Alert } from '@/components'
import { format, texts } from '@/config'
import { toUnits } from '@/features/accounting/money'
import { receiptPrintTitle, receiptVatSummary } from '@/features/finance/receiptPrint'
import { financeApi, financePage, type Receipt } from '@/features/finance/types'
import { loadItem } from '@/features/organization/pages'
import { loadLetterhead } from '@/features/sales/pages'
import { PrintDocument, printDate, type PrintTotal } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.sales.print.receipt }

/**
 * พิมพ์ใบเสร็จรับเงิน A4 — ผู้จ่ายเงินจาก snapshot บนเอกสาร (ไม่ต้องโหลดลูกค้าเพิ่ม), รายการ = ใบแจ้งหนี้ที่ชำระ / รายการอื่น
 * ยอดรวม → หักภาษี ณ ที่จ่าย → รับเงินสุทธิ; จำนวนเงินตัวอักษร = ยอดรวมที่ชำระ (รวมภาษีถูกหัก)
 * รับเงินค่าบริการที่ภาษียังไม่ถึงกำหนด (tax_invoice) = "ใบเสร็จรับเงิน / ใบกำกับภาษี" พร้อมมูลค่า / ภาษีที่เกิดต่อใบแจ้งหนี้
 */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const p = texts.sales.print
  const [res, letterhead] = await Promise.all([loadItem<Receipt>(financeApi.receipts(companyId), docId, texts.finance.receipts.notFound), loadLetterhead(companyId)])
  if ('panel' in res) return res.panel
  const r = res.item
  if (!letterhead || r.status !== 'posted') return <Alert tone="error">{p.unavailable}</Alert>
  const hasWht = toUnits(r.wht) > 0n
  const title = receiptPrintTitle(r)
  const taxInvoice = title === p.receiptTax
  const vat = receiptVatSummary(r)
  const methods = r.methods.map((m) => [m.cash_account.name, m.reference].filter(Boolean).join(' ')).join(', ')
  const totals: PrintTotal[] = [
    ...(taxInvoice
      ? [
          { label: p.taxBase, value: vat.base },
          { label: p.vatAmount, value: vat.vat },
        ]
      : []),
    ...(hasWht
      ? [
          { label: p.subtotal, value: r.amount },
          { label: p.wht, value: r.wht },
        ]
      : []),
    { label: hasWht ? p.paid : p.grand, value: hasWht ? r.net : r.amount, strong: true },
  ]
  const note = [...(taxInvoice ? vat.notes : []), methods ? `${p.paidBy}: ${methods}` : ''].filter(Boolean).join(' · ')
  return (
    <>
      <PrintToolbar backHref={`${financePage(companyId, 'receipts')}/${r.id}`} />
      <PrintDocument
        docType="finance.receipt"
        letterhead={letterhead}
        title={title}
        docNo={r.document_no}
        info={[[p.date, printDate(r.date)]]}
        party={{ label: p.receivedFrom, name: r.party.name, taxId: r.party.tax_id, branchNo: r.party.branch_no, address: r.party.address }}
        lines={r.lines.map((l) => ({
          description: l.invoice ? format(p.forInvoice, { no: l.invoice.code }) : l.description || r.description,
          detail: l.invoice ? l.description : undefined,
          amount: l.amount,
        }))}
        totals={totals}
        amountInWords={r.amount}
        note={note || undefined}
        signatures={[p.receiver, p.approver]}
      />
    </>
  )
}
