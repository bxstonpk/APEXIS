import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { FormPage, loadItem } from '@/features/organization/pages'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { SalesStatus } from '@/features/sales/pages'
import { SalesDocForm } from '@/features/sales/SalesDocForm'
import { DOC_QUERY_KEYS, perms, salesApi, salesPage, type Order } from '@/features/sales/types'
import { dateText, EntryLink, Facts, LinesTable } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.orders.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/**
 * ใบสั่งขาย — ร่าง: แก้ไข / อนุมัติ (คำเตือนวงเงิน) / ลบ
 * อนุมัติแล้ว: ส่งของ / ออกใบกำกับภาษี (ไปหน้าสร้างพร้อม ?order_id=) / ปิด / ยกเลิก + ความคืบหน้ารายบรรทัด
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales
  const a = t.actions
  const path = salesPage(companyId, 'orders')
  const listHref = listHrefFromBack(path, param(await searchParams, 'back'), DOC_QUERY_KEYS)
  const res = await loadItem<Order>(salesApi.orders(companyId), docId, t.orders.notFound)
  if ('panel' in res) return res.panel
  const o = res.item
  const has = (p: string) => can(ctx.permissions, p)
  const toDeliver = o.lines.some((l) => toUnits(l.quantity) > toUnits(l.delivered_quantity))
  const toInvoice = o.lines.some((l) => toUnits(l.quantity) > toUnits(l.invoiced_quantity))

  const actions: DocAction[] = []
  if (o.status === 'draft') {
    if (has(perms.orderManage)) actions.push({ kind: 'delete', label: a.delete, confirm: a.confirm.delete! })
    if (has(perms.orderApprove)) actions.push({ kind: 'patch', status: 'approved', label: a.approve, confirm: a.confirm.approve, primary: true })
  }
  if (o.status === 'approved') {
    if (has(perms.orderManage)) {
      actions.push({ kind: 'patch', status: 'cancelled', label: a.cancelOrder, confirm: a.confirm.cancelOrder, danger: true })
      actions.push({ kind: 'patch', status: 'closed', label: a.close, confirm: a.confirm.close })
    }
    if (toInvoice && has(perms.invoicePost))
      actions.push({ kind: 'link', href: `${salesPage(companyId, 'invoices')}/new?order_id=${o.id}`, label: a.invoice, icon: 'invoice', primary: !toDeliver })
    if (toDeliver && has(perms.deliveryPost))
      actions.push({ kind: 'link', href: `${salesPage(companyId, 'deliveries')}/new?order_id=${o.id}`, label: a.deliver, icon: 'deliver', primary: true })
  }

  return (
    <FormPage
      crumb={{ app: texts.apps.sales.name, label: t.orders.heading, href: listHref }}
      title={o.document_no ?? t.orders.draftTitle}
      description={o.customer.name}
      aside={<SalesStatus kind="order" status={o.status} />}
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${salesApi.orders(companyId)}/${o.id}`} listHref={listHref} docNo={o.document_no} actions={actions} />
        {o.status === 'draft' && has(perms.orderManage) ? (
          <SalesDocForm
            kind="order"
            apiPath={salesApi.orders(companyId)}
            customersApi={salesApi.customers(companyId)}
            itemsApi={salesApi.items(companyId)}
            docsPath={path}
            listHref={listHref}
            vat={null}
            doc={o}
          />
        ) : (
          <>
            <Facts
              items={[
                [t.fields.customer, `${o.customer.code} ${o.customer.name}`],
                [t.fields.date, dateText(o.date)],
                [t.fields.deliveryDate, dateText(o.delivery_date)],
                [t.fields.reference, o.reference],
                [
                  t.quotations.heading,
                  o.quotation_id ? <EntryLink key="q" href={`${salesPage(companyId, 'quotations')}/${o.quotation_id}`} no={t.openQuotation} /> : '',
                ],
                [t.fields.description, o.description],
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
                      {format(t.progress, { delivered: formatAmount(l.delivered_quantity), invoiced: formatAmount(l.invoiced_quantity), ordered: formatAmount(l.quantity) })}
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
