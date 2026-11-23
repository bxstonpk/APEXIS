import type { Metadata } from 'next'
import { branchLabel } from '@/features/sales/PrintDocument'
import { texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { FormPage, loadItem } from '@/features/organization/pages'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { SalesStatus } from '@/features/sales/pages'
import { DOC_QUERY_KEYS, perms, salesApi, salesPage, type Invoice } from '@/features/sales/types'
import { dateText, EntryLink, Facts, LinesTable } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.invoices.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** ใบแจ้งหนี้ / ใบกำกับภาษี — พิมพ์, รับเงิน (ไปแอปการเงิน), ยกเลิก */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales
  const a = t.actions
  const path = salesPage(companyId, 'invoices')
  const listHref = listHrefFromBack(path, param(await searchParams, 'back'), DOC_QUERY_KEYS)
  const res = await loadItem<Invoice>(salesApi.invoices(companyId), docId, t.invoices.notFound)
  if ('panel' in res) return res.panel
  const inv = res.item
  const posted = inv.status === 'posted'
  const entryHref = inv.entry_id && can(ctx.permissions, 'ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${inv.entry_id}` : null
  const actions: DocAction[] = []
  if (posted && can(ctx.permissions, perms.invoicePost)) actions.push({ kind: 'patch', status: 'cancelled', label: a.cancelDoc, confirm: a.confirm.cancelDoc, danger: true })
  actions.push({ kind: 'link', href: `${path}/${inv.id}/print`, label: a.print, icon: 'print' })
  if (posted && toUnits(inv.outstanding) > 0n && can(ctx.permissions, 'finance.receipt:post'))
    actions.push({ kind: 'link', href: `${companyPath(companyId, 'finance', 'receipts')}/new`, label: a.receive, icon: 'receive', primary: true })

  return (
    <FormPage
      crumb={{ app: texts.apps.sales.name, label: t.invoices.heading, href: listHref }}
      title={inv.document_no}
      description={inv.customer.name}
      aside={<SalesStatus kind="posted" status={inv.status} />}
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${salesApi.invoices(companyId)}/${inv.id}`} listHref={listHref} docNo={inv.document_no} actions={actions} />
        <Facts
          items={[
            [t.fields.customer, `${inv.customer.code} ${inv.bill_to.name || inv.customer.name}`],
            [t.fields.customerBranch, branchLabel(inv.bill_to.branch_no)],
            [t.fields.date, dateText(inv.date)],
            [t.columns.due, dateText(inv.due_date)],
            [t.columns.order, inv.order ? <EntryLink key="o" href={`${salesPage(companyId, 'orders')}/${inv.order.id}`} no={inv.order.order_no} /> : ''],
            [t.fields.entry, <EntryLink key="e" href={entryHref} no={inv.entry_no} />],
            [t.fields.received, formatAmount(inv.received_amount)],
            [t.columns.outstanding, posted ? formatAmount(inv.outstanding) : ''],
            [t.fields.stockDocument, inv.with_delivery ? inv.stock_document_no : ''],
            [t.fields.reference, inv.reference],
          ]}
        />
        <LinesTable
          rows={inv.lines.map((l) => ({ key: l.order_line_no, item: l.item, quantity: l.quantity, unitPrice: l.unit_price, amount: inv.price_includes_vat ? l.amounts.gross : l.amounts.net }))}
          totals={inv.totals}
          vatRate={inv.vat_rate}
          inclusive={inv.price_includes_vat}
        />
      </div>
    </FormPage>
  )
}
