import type { Metadata } from 'next'
import { texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount } from '@/features/accounting/money'
import { FormPage, loadItem } from '@/features/organization/pages'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { SalesStatus } from '@/features/sales/pages'
import { DOC_QUERY_KEYS, perms, salesApi, salesPage, type Delivery } from '@/features/sales/types'
import { dateText, EntryLink, Facts, LinesTable } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.deliveries.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** ใบส่งของ (ลงบัญชีแล้ว) — ยกเลิกได้ = กลับรายการบัญชีและคืนสต็อก */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales
  const listHref = listHrefFromBack(salesPage(companyId, 'deliveries'), param(await searchParams, 'back'), DOC_QUERY_KEYS)
  const res = await loadItem<Delivery>(salesApi.deliveries(companyId), docId, t.deliveries.notFound)
  if ('panel' in res) return res.panel
  const d = res.item
  const entryHref = d.entry_id && can(ctx.permissions, 'ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${d.entry_id}` : null
  const actions: DocAction[] =
    d.status === 'posted' && can(ctx.permissions, perms.deliveryPost)
      ? [{ kind: 'patch', status: 'cancelled', label: t.actions.cancelDoc, confirm: t.actions.confirm.cancelDoc, danger: true }]
      : []
  return (
    <FormPage
      crumb={{ app: texts.apps.sales.name, label: t.deliveries.heading, href: listHref }}
      title={d.document_no}
      description={d.customer.name}
      aside={<SalesStatus kind="posted" status={d.status} />}
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${salesApi.deliveries(companyId)}/${d.id}`} listHref={listHref} docNo={d.document_no} actions={actions} />
        <Facts
          items={[
            [t.fields.customer, `${d.customer.code} ${d.customer.name}`],
            [t.fields.date, dateText(d.date)],
            [t.columns.order, d.order ? <EntryLink key="o" href={`${salesPage(companyId, 'orders')}/${d.order.id}`} no={d.order.order_no} /> : ''],
            [t.fields.stockDocument, d.stock_document_no],
            [t.fields.entry, <EntryLink key="e" href={entryHref} no={d.entry_no} />],
            [t.fields.cost, formatAmount(d.total_cost)],
            [t.fields.reference, d.reference],
            [t.fields.description, d.description],
          ]}
        />
        <LinesTable rows={d.lines.map((l) => ({ key: l.order_line_no, item: l.item, quantity: l.quantity }))} />
      </div>
    </FormPage>
  )
}
