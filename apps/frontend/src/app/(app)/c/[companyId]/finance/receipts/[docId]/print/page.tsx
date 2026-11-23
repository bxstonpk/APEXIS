import type { Metadata } from 'next'
import { Alert } from '@/components'
import { format, texts } from '@/config'
import { toUnits } from '@/features/accounting/money'
import { financeApi, financePage, type Receipt } from '@/features/finance/types'
import { loadItem } from '@/features/organization/pages'
import { loadLetterhead } from '@/features/sales/pages'
import { PrintDocument, printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.sales.print.receipt }

/**
 * พิมพ์ใบเสร็จรับเงิน A4 — ผู้จ่ายเงินจาก snapshot บนเอกสาร (ไม่ต้องโหลดลูกค้าเพิ่ม), รายการ = ใบแจ้งหนี้ที่ชำระ / รายการอื่น
 * ยอดรวม → หักภาษี ณ ที่จ่าย → รับเงินสุทธิ; จำนวนเงินตัวอักษร = ยอดรวมที่ชำระ (รวมภาษีถูกหัก)
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
  const methods = r.methods.map((m) => [m.cash_account.name, m.reference].filter(Boolean).join(' ')).join(', ')
  return (
    <>
      <PrintToolbar backHref={`${financePage(companyId, 'receipts')}/${r.id}`} />
      <PrintDocument
        letterhead={letterhead}
        title={p.receipt}
        docNo={r.document_no}
        info={[[p.date, printDate(r.date)]]}
        party={{ label: p.receivedFrom, name: r.party.name, taxId: r.party.tax_id, branchNo: r.party.branch_no, address: r.party.address }}
        lines={r.lines.map((l) => ({
          description: l.invoice ? format(p.forInvoice, { no: l.invoice.code }) : l.description || r.description,
          detail: l.invoice ? l.description : undefined,
          amount: l.amount,
        }))}
        totals={[
          ...(hasWht
            ? [
                { label: p.subtotal, value: r.amount },
                { label: p.wht, value: r.wht },
              ]
            : []),
          { label: hasWht ? p.paid : p.grand, value: hasWht ? r.net : r.amount, strong: true },
        ]}
        amountInWords={r.amount}
        note={methods ? `${p.paidBy}: ${methods}` : undefined}
        signatures={[p.receiver, p.approver]}
      />
    </>
  )
}
