import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { FormPage, loadItem } from '@/features/organization/pages'
import { PurchaseStatus } from '@/features/purchasing/pages'
import { docPerms, PURCHASE_DOC_QUERY_KEYS, purchasingApi, purchasingDocsApi, purchasingPage, type PurchaseOrder } from '@/features/purchasing/types'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { branchLabel } from '@/features/sales/PrintDocument'
import { SalesDocForm } from '@/features/sales/SalesDocForm'
import { dateText, Facts, LinesTable } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.orders.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/**
 * ใบสั่งซื้อ — ร่าง: แก้ไข / อนุมัติ / ลบ
 * อนุมัติแล้ว: รับสินค้า / บันทึกใบแจ้งหนี้ (ไปหน้าสร้างพร้อม ?order_id=) / ปิด / ยกเลิก + ความคืบหน้ารายบรรทัด; มีเลขที่ = พิมพ์ A4
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing
  const a = t.actions
  const path = purchasingPage(companyId, 'orders')
  const listHref = listHrefFromBack(path, param(await searchParams, 'back'), PURCHASE_DOC_QUERY_KEYS)
  const res = await loadItem<PurchaseOrder>(purchasingDocsApi.orders(companyId), docId, t.orders.notFound)
  if ('panel' in res) return res.panel
  const o = res.item
  const has = (p: string) => can(ctx.permissions, p)
  const toReceive = o.lines.some((l) => toUnits(l.quantity) > toUnits(l.received_quantity))
  const toInvoice = o.lines.some((l) => toUnits(l.received_quantity) > toUnits(l.invoiced_quantity))

  const actions: DocAction[] = []
  if (o.status === 'draft') {
    if (has(docPerms.orderManage)) actions.push({ kind: 'delete', label: a.delete, confirm: a.confirm.delete! })
    if (has(docPerms.orderApprove)) actions.push({ kind: 'patch', status: 'approved', label: a.approve, confirm: a.confirm.approve, primary: true })
  }
  if (o.status === 'approved') {
    if (has(docPerms.orderManage)) {
      actions.push({ kind: 'patch', status: 'cancelled', label: a.cancelOrder, confirm: a.confirm.cancelOrder, danger: true })
      actions.push({ kind: 'patch', status: 'closed', label: a.close, confirm: a.confirm.close })
    }
    if (toInvoice && has(docPerms.invoicePost))
      actions.push({ kind: 'link', href: `${purchasingPage(companyId, 'invoices')}/new?order_id=${o.id}`, label: a.invoice, icon: 'invoice', primary: !toReceive })
    if (toReceive && has(docPerms.receiptPost))
      actions.push({ kind: 'link', href: `${purchasingPage(companyId, 'receipts')}/new?order_id=${o.id}`, label: a.receive, icon: 'deliver', primary: true })
  }
  if (o.order_no) actions.push({ kind: 'link', href: `${path}/${o.id}/print`, label: a.print, icon: 'print' })

  return (
    <FormPage
      crumb={{ app: texts.apps.purchasing.name, label: t.orders.heading, href: listHref }}
      title={o.order_no ?? t.orders.draftTitle}
      description={o.supplier.name}
      aside={<PurchaseStatus kind="order" status={o.status} />}
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${purchasingDocsApi.orders(companyId)}/${o.id}`} listHref={listHref} docNo={o.order_no} actions={actions} />
        {o.status === 'draft' && has(docPerms.orderManage) ? (
          <SalesDocForm
            kind="purchase-order"
            apiPath={purchasingDocsApi.orders(companyId)}
            partiesApi={purchasingApi.suppliers(companyId)}
            itemsApi={purchasingDocsApi.items(companyId)}
            docsPath={path}
            listHref={listHref}
            vat={null}
            doc={o}
          />
        ) : (
          <>
            <Facts
              items={[
                [t.columns.supplier, `${o.supplier.code} ${o.supplier.name}`],
                [t.facts.supplierBranch, branchLabel(o.supplier_branch_no)],
                [texts.sales.fields.date, dateText(o.date)],
                [texts.sales.fields.deliveryDate, dateText(o.delivery_date)],
                [texts.sales.fields.reference, o.reference],
                [texts.sales.fields.description, o.vat_registered ? o.description : [o.description, t.orders.vatNote].filter(Boolean).join(' · ')],
              ]}
            />
            <LinesTable
              rows={o.lines.map((l) => ({
                key: l.line_no,
                item: l.item,
                description: l.description,
                quantity: l.quantity,
                unitPrice: l.unit_price,
                amount: o.price_includes_vat ? l.amounts.gross : l.amounts.net,
                extra:
                  o.status === 'draft' ? undefined : (
                    <span className="mt-1 block text-[12px] text-ink-muted">
                      {format(t.orders.progress, {
                        received: formatAmount(l.received_quantity),
                        invoiced: formatAmount(l.invoiced_quantity),
                        ordered: formatAmount(l.quantity),
                      })}
                    </span>
                  ),
              }))}
              totals={o.totals}
              vatRate={o.vat_rate}
              inclusive={o.price_includes_vat}
            />
          </>
        )}
      </div>
    </FormPage>
  )
}
