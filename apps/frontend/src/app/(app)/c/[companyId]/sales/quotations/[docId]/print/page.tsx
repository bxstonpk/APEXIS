import type { Metadata } from 'next'
import { Alert } from '@/components'
import { format, texts } from '@/config'
import { loadItem } from '@/features/organization/pages'
import { loadCustomer, loadLetterhead } from '@/features/sales/pages'
import { PrintDocument, printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { salesApi, salesPage, type Quotation } from '@/features/sales/types'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.sales.print.quotation }

/** พิมพ์ใบเสนอราคา A4 — เอกสาร + หัวกระดาษโหลดขนานกัน แล้วโหลดที่อยู่ลูกค้า */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const p = texts.sales.print
  const [res, letterhead] = await Promise.all([loadItem<Quotation>(salesApi.quotations(companyId), docId, texts.sales.quotations.notFound), loadLetterhead(companyId)])
  if ('panel' in res) return res.panel
  const q = res.item
  if (!q.document_no || !letterhead) return <Alert tone="error">{p.unavailable}</Alert>
  const customer = await loadCustomer(companyId, q.customer.id)
  const branch = customer?.branches?.find((b) => b.branch_no === q.customer_branch_no)
  return (
    <>
      <PrintToolbar backHref={`${salesPage(companyId, 'quotations')}/${q.id}`} />
      <PrintDocument
        docType="sales.quotation"
        letterhead={letterhead}
        title={p.quotation}
        docNo={q.document_no}
        info={[
          [p.date, printDate(q.date)],
          [p.validUntil, printDate(q.valid_until)],
          [p.reference, q.reference],
        ]}
        party={{ label: p.customer, name: customer?.name ?? q.customer.name, taxId: customer?.tax_id, branchNo: q.customer_branch_no || customer?.branch_no, address: branch ? branch.address : customer?.address }}
        lines={q.lines.map((l) => ({ description: l.item.name, detail: l.description, quantity: l.quantity, unitPrice: l.unit_price, amount: q.price_includes_vat ? l.amounts.gross : l.amounts.net }))}
        totals={[
          { label: p.subtotal, value: q.totals.net },
          { label: format(p.vat, { rate: q.vat_rate }), value: q.totals.vat },
          { label: p.grand, value: q.totals.gross, strong: true },
        ]}
        amountInWords={q.totals.gross}
        note={q.description}
        signatures={[p.customer, p.approver]}
      />
    </>
  )
}
