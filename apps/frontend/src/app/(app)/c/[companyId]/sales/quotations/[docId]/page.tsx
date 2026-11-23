import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, loadItem } from '@/features/organization/pages'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { SalesStatus } from '@/features/sales/pages'
import { SalesDocForm } from '@/features/sales/SalesDocForm'
import { DOC_QUERY_KEYS, perms, salesApi, salesPage, type Quotation } from '@/features/sales/types'
import { dateText, Facts, LinesTable } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.quotations.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** ใบเสนอราคา — ร่าง: แก้ไขได้ + ส่ง / ลบ; ส่งแล้ว: ตอบรับ / ไม่ตอบรับ; ตอบรับ: สร้างใบสั่งขาย; มีเลขที่: พิมพ์ */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales
  const a = t.actions
  const path = salesPage(companyId, 'quotations')
  const ordersPath = salesPage(companyId, 'orders')
  const listHref = listHrefFromBack(path, param(await searchParams, 'back'), DOC_QUERY_KEYS)
  const res = await loadItem<Quotation>(salesApi.quotations(companyId), docId, t.quotations.notFound)
  if ('panel' in res) return res.panel
  const q = res.item
  const manage = can(ctx.permissions, perms.quotationManage)

  const actions: DocAction[] = []
  if (manage && q.status === 'draft') {
    actions.push({ kind: 'delete', label: a.delete, confirm: a.confirm.delete! })
    actions.push({ kind: 'patch', status: 'sent', label: a.send, confirm: a.confirm.send, primary: true })
  }
  if (manage && q.status === 'sent') {
    actions.push({ kind: 'patch', status: 'rejected', label: a.reject })
    actions.push({ kind: 'patch', status: 'accepted', label: a.accept, primary: true })
  }
  if (q.document_no) actions.push({ kind: 'link', href: `${path}/${q.id}/print`, label: a.print, icon: 'print' })
  if (q.order_id) actions.push({ kind: 'link', href: `${ordersPath}/${q.order_id}`, label: t.columns.order })
  else if (q.status === 'accepted' && can(ctx.permissions, perms.orderManage)) actions.push({ kind: 'toOrder', label: a.toOrder, primary: true })

  return (
    <FormPage
      crumb={{ app: texts.apps.sales.name, label: t.quotations.heading, href: listHref }}
      title={q.document_no ?? t.quotations.draftTitle}
      description={q.customer.name}
      aside={<SalesStatus kind="quotation" status={q.status} />}
    >
      <div className="flex flex-col gap-6">
        <DocActions
          apiPath={`${salesApi.quotations(companyId)}/${q.id}`}
          listHref={listHref}
          docNo={q.document_no}
          actions={actions}
          ordersApi={salesApi.orders(companyId)}
          ordersPath={ordersPath}
          quotationId={q.id}
        />
        {q.status === 'draft' && manage ? (
          <SalesDocForm
            kind="quotation"
            apiPath={salesApi.quotations(companyId)}
            customersApi={salesApi.customers(companyId)}
            itemsApi={salesApi.items(companyId)}
            docsPath={path}
            listHref={listHref}
            vat={null}
            doc={q}
          />
        ) : (
          <>
            <Facts
              items={[
                [t.fields.customer, `${q.customer.code} ${q.customer.name}`],
                [t.fields.date, dateText(q.date)],
                [t.fields.validUntil, dateText(q.valid_until)],
                [t.fields.reference, q.reference],
                [t.fields.description, q.description],
              ]}
            />
            <LinesTable
              rows={q.lines.map((l) => ({ key: l.line_no, item: l.item, description: l.description, quantity: l.quantity, unitPrice: l.unit_price, amount: q.price_includes_vat ? l.amounts.gross : l.amounts.net }))}
              totals={q.totals}
              vatRate={q.vat_rate}
              inclusive={q.price_includes_vat}
            />
          </>
        )}
      </div>
    </FormPage>
  )
}
