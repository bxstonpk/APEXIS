import type { Metadata } from 'next'
import { Alert } from '@/components'
import { format, texts } from '@/config'
import { toUnits } from '@/features/accounting/money'
import { loadItem } from '@/features/organization/pages'
import { purchasingApi, purchasingDocsApi, purchasingPage, type PurchaseOrder, type Supplier } from '@/features/purchasing/types'
import { loadLetterhead } from '@/features/sales/pages'
import { PrintDocument, printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.purchasing.print.purchaseOrder }

/** พิมพ์ใบสั่งซื้อ A4 — ผู้ขาย + สาขาตามใบสั่งซื้อ (เอกสาร + หัวกระดาษโหลดขนานกัน แล้วโหลดผู้ขาย) */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const p = texts.sales.print
  const pp = texts.purchasing.print
  const [res, letterhead] = await Promise.all([loadItem<PurchaseOrder>(purchasingDocsApi.orders(companyId), docId, texts.purchasing.orders.notFound), loadLetterhead(companyId)])
  if ('panel' in res) return res.panel
  const o = res.item
  if (!o.order_no || !letterhead) return <Alert tone="error">{p.unavailable}</Alert>
  const supplier = await backendGet<Supplier>(`${purchasingApi.suppliers(companyId)}/${o.supplier.id}`).then(
    (r) => r.data,
    () => null,
  )
  const branch = supplier?.branches?.find((b) => b.branch_no === o.supplier_branch_no)
  const vat = toUnits(o.vat_rate) > 0n
  const service = o.kind === 'service'
  return (
    <>
      <PrintToolbar backHref={`${purchasingPage(companyId, service ? 'work-orders' : 'orders')}/${o.id}`} />
      <PrintDocument
        docType={service ? 'purchasing.work_order' : 'purchasing.order'}
        letterhead={letterhead}
        title={service ? pp.workOrder : pp.purchaseOrder}
        docNo={o.order_no}
        info={[
          [p.date, printDate(o.date)],
          [pp.deliveryDate, printDate(o.delivery_date)],
          [p.reference, o.reference],
        ]}
        party={{
          label: pp.supplier,
          name: supplier?.name ?? o.supplier.name,
          taxId: supplier?.tax_id,
          branchNo: o.supplier_branch_no || supplier?.branch_no,
          address: branch ? branch.address : supplier?.address,
        }}
        lines={o.lines.map((l) => ({
          description: l.item.name,
          detail: l.description,
          quantity: l.quantity,
          unitPrice: l.unit_price,
          amount: o.price_includes_vat ? l.amounts.gross : l.amounts.net,
        }))}
        totals={[
          { label: p.subtotal, value: o.totals.net },
          ...(vat ? [{ label: format(p.vat, { rate: o.vat_rate }), value: o.totals.vat }] : []),
          { label: p.grand, value: o.totals.gross, strong: true },
        ]}
        amountInWords={o.totals.gross}
        note={o.description}
        signatures={[pp.orderer, p.approver]}
      />
    </>
  )
}
