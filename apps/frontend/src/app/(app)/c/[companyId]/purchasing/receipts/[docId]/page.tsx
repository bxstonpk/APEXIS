import type { Metadata } from 'next'
import { texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount } from '@/features/accounting/money'
import { FormPage, loadItem } from '@/features/organization/pages'
import { PurchaseStatus } from '@/features/purchasing/pages'
import { docPerms, PURCHASE_DOC_QUERY_KEYS, purchasingDocsApi, purchasingPage, type GoodsReceipt } from '@/features/purchasing/types'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { dateText, EntryLink, Facts, LinesTable } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.receipts.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** ใบรับสินค้า (ลงบัญชีแล้ว) — ยกเลิกได้เมื่อยังไม่ได้บันทึกใบแจ้งหนี้ = กลับรายการบัญชีและเอาสต็อกออก */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing
  const listHref = listHrefFromBack(purchasingPage(companyId, 'receipts'), param(await searchParams, 'back'), PURCHASE_DOC_QUERY_KEYS)
  const res = await loadItem<GoodsReceipt>(purchasingDocsApi.receipts(companyId), docId, t.receipts.notFound)
  if ('panel' in res) return res.panel
  const r = res.item
  const entryHref = r.entry_id && can(ctx.permissions, 'ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${r.entry_id}` : null
  const actions: DocAction[] =
    r.status === 'posted' && can(ctx.permissions, docPerms.receiptPost)
      ? [{ kind: 'patch', status: 'cancelled', label: t.actions.cancelDoc, confirm: t.actions.confirm.cancelDoc, danger: true }]
      : []
  return (
    <FormPage
      crumb={{ app: texts.apps.purchasing.name, label: t.receipts.heading, href: listHref }}
      title={r.document_no}
      description={r.supplier.name}
      aside={<PurchaseStatus kind="posted" status={r.status} />}
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${purchasingDocsApi.receipts(companyId)}/${r.id}`} listHref={listHref} docNo={r.document_no} actions={actions} />
        <Facts
          items={[
            [t.columns.supplier, `${r.supplier.code} ${r.supplier.name}`],
            [texts.sales.fields.date, dateText(r.date)],
            [t.columns.order, <EntryLink key="o" href={`${purchasingPage(companyId, 'orders')}/${r.order.id}`} no={r.order.order_no} />],
            [t.facts.stockDocument, r.stock_document_no],
            [t.facts.entry, <EntryLink key="e" href={entryHref} no={r.entry_no} />],
            [t.facts.value, formatAmount(r.total_value)],
            [texts.sales.fields.reference, r.reference],
            [texts.sales.fields.description, r.description],
          ]}
        />
        <LinesTable rows={r.lines.map((l) => ({ key: l.order_line_no, item: l.item, quantity: l.quantity, unitPrice: l.unit_cost, amount: l.value }))} />
      </div>
    </FormPage>
  )
}
