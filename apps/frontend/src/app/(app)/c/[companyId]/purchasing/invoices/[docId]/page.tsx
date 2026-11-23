import type { Metadata } from 'next'
import { texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { FormPage, loadItem } from '@/features/organization/pages'
import { PurchaseStatus } from '@/features/purchasing/pages'
import { docPerms, PURCHASE_DOC_QUERY_KEYS, purchasingDocsApi, purchasingPage, type SupplierInvoice } from '@/features/purchasing/types'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { branchLabel } from '@/features/sales/PrintDocument'
import { dateText, EntryLink, Facts, LinesTable } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.invoices.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** ใบแจ้งหนี้ / ใบกำกับภาษีซื้อ — ผู้ขายตามใบกำกับ ณ วันออก (bill_from), จ่ายเงินที่แอปการเงิน, ยกเลิก */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing
  const a = t.actions
  const listHref = listHrefFromBack(purchasingPage(companyId, 'invoices'), param(await searchParams, 'back'), PURCHASE_DOC_QUERY_KEYS)
  const res = await loadItem<SupplierInvoice>(purchasingDocsApi.invoices(companyId), docId, t.invoices.notFound)
  if ('panel' in res) return res.panel
  const inv = res.item
  const posted = inv.status === 'posted'
  const entryHref = inv.entry_id && can(ctx.permissions, 'ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${inv.entry_id}` : null
  const actions: DocAction[] = []
  if (posted && can(ctx.permissions, docPerms.invoicePost)) actions.push({ kind: 'patch', status: 'cancelled', label: a.cancelDoc, confirm: a.confirm.cancelDoc, danger: true })
  if (posted && toUnits(inv.outstanding) > 0n && can(ctx.permissions, 'finance.payment:post'))
    actions.push({ kind: 'link', href: `${companyPath(companyId, 'finance', 'payments')}/new`, label: a.pay, icon: 'receive', primary: true })
  const b = inv.bill_from
  return (
    <FormPage
      crumb={{ app: texts.apps.purchasing.name, label: t.invoices.heading, href: listHref }}
      title={inv.document_no}
      description={`${inv.supplier.name} · ${inv.supplier_invoice_no}`}
      aside={<PurchaseStatus kind="posted" status={inv.status} />}
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${purchasingDocsApi.invoices(companyId)}/${inv.id}`} listHref={listHref} docNo={inv.document_no} actions={actions} />
        <Facts
          items={[
            [t.facts.billFrom, [b.name || inv.supplier.name, b.tax_id, branchLabel(b.branch_no)].filter(Boolean).join(' · ')],
            [t.columns.invoiceNo, inv.supplier_invoice_no],
            [t.facts.invoiceDate, dateText(inv.invoice_date)],
            [texts.sales.fields.date, dateText(inv.date)],
            [t.columns.due, dateText(inv.due_date)],
            [t.columns.order, <EntryLink key="o" href={`${purchasingPage(companyId, 'orders')}/${inv.order.id}`} no={inv.order.order_no} />],
            [t.facts.entry, <EntryLink key="e" href={entryHref} no={inv.entry_no} />],
            [t.facts.paid, formatAmount(inv.paid_amount)],
            [t.columns.outstanding, posted ? formatAmount(inv.outstanding) : ''],
          ]}
        />
        <LinesTable
          rows={inv.lines.map((l) => ({
            key: l.order_line_no,
            item: l.item,
            quantity: l.quantity,
            unitPrice: l.unit_price,
            amount: inv.price_includes_vat ? l.amounts.gross : l.amounts.net,
            extra:
              toUnits(l.price_variance) !== 0n ? (
                <span className="mt-1 block text-[12px] text-ink-muted">
                  {t.facts.variance} {formatAmount(l.price_variance)}
                </span>
              ) : undefined,
          }))}
          totals={inv.totals}
          vatRate={inv.vat_rate}
          inclusive={inv.price_includes_vat}
        />
      </div>
    </FormPage>
  )
}
