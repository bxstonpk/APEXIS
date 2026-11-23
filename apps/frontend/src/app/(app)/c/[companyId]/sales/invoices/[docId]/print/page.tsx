import type { Metadata } from 'next'
import { Alert } from '@/components'
import { format, texts } from '@/config'
import { toUnits } from '@/features/accounting/money'
import { loadItem } from '@/features/organization/pages'
import { loadCustomer, loadLetterhead } from '@/features/sales/pages'
import { PrintDocument, printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { salesApi, salesPage, type Invoice } from '@/features/sales/types'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.sales.print.taxInvoice }

/**
 * พิมพ์ใบแจ้งหนี้ / ใบกำกับภาษี A4 — ชื่อเอกสารตามประเภท:
 *   ไม่มี VAT = ใบแจ้งหนี้; ส่งของพร้อมออกบิล = ใบส่งของ / ใบกำกับภาษี; ออกบิลจากของที่ส่ง = ใบแจ้งหนี้ / ใบกำกับภาษี
 */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const p = texts.sales.print
  const [res, letterhead] = await Promise.all([loadItem<Invoice>(salesApi.invoices(companyId), docId, texts.sales.invoices.notFound), loadLetterhead(companyId)])
  if ('panel' in res) return res.panel
  const inv = res.item
  if (!letterhead) return <Alert tone="error">{p.unavailable}</Alert>
  const customer = await loadCustomer(companyId, inv.customer.id)
  const vat = toUnits(inv.vat_rate) > 0n
  const title = !vat ? p.invoice : inv.with_delivery ? p.deliveryTaxInvoice : p.taxInvoice
  return (
    <>
      <PrintToolbar backHref={`${salesPage(companyId, 'invoices')}/${inv.id}`} />
      {inv.status === 'cancelled' && (
        <div className="mx-auto mb-4 max-w-[210mm] print:hidden">
          <Alert tone="error">{p.cancelled}</Alert>
        </div>
      )}
      <PrintDocument
        letterhead={letterhead}
        title={title}
        docNo={inv.document_no}
        info={[
          [p.date, printDate(inv.date)],
          [p.dueDate, printDate(inv.due_date)],
          [p.orderNo, inv.order?.order_no ?? ''],
          [p.reference, inv.reference],
        ]}
        party={{ label: p.customer, name: customer?.name ?? inv.customer.name, taxId: customer?.tax_id, branchNo: customer?.branch_no, address: customer?.address }}
        lines={inv.lines.map((l) => ({ description: l.item.name, quantity: l.quantity, unitPrice: l.unit_price, amount: inv.price_includes_vat ? l.amounts.gross : l.amounts.net }))}
        totals={[
          { label: p.subtotal, value: inv.totals.net },
          ...(vat ? [{ label: format(p.vat, { rate: inv.vat_rate }), value: inv.totals.vat }] : []),
          { label: p.grand, value: inv.totals.gross, strong: true },
        ]}
        amountInWords={inv.totals.gross}
        note={inv.description}
        signatures={[p.customerSign, p.approver]}
      />
    </>
  )
}
