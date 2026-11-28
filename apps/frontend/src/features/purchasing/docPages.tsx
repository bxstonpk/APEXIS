import { format, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { FormPage, loadItem, NoPermission } from '@/features/organization/pages'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { branchLabel } from '@/features/sales/PrintDocument'
import { SalesDocForm } from '@/features/sales/SalesDocForm'
import { dateText, EntryLink, Facts, LinesTable } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { param, type SearchParams } from '@/lib/url'
import { flowTexts, loadPurchaseVat, PurchaseStatus, VatBadge } from './pages'
import { TaxInvoicesPanel } from './TaxInvoicesPanel'
import {
  documentsHref,
  purchasingApi,
  purchasingDocsApi,
  purchasingPage,
  type GoodsReceipt,
  type PurchaseFlow,
  type PurchaseOrder,
  type SupplierInvoice,
} from './types'

/**
 * หน้าเอกสารจัดซื้อที่ใช้ร่วมกันของสองสาย (server) — สายซื้อสินค้า / สายจ้างบริการต่างกันที่ API / สิทธิ์ / ข้อความ (PurchaseFlow)
 * ไม่มีลิงก์ไปแอปการเงิน (จ่ายเงินทำที่แอปการเงินเท่านั้น)
 */

type DocProps = { companyId: string; docId: string; sp: Promise<SearchParams>; flow: PurchaseFlow }

/** สร้างใบสั่งซื้อ / ใบสั่งจ้าง */
export async function OrderNewPage({ companyId, flow }: { companyId: string; flow: PurchaseFlow }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, flow.perms.orderManage)) return <NoPermission />
  const ft = flowTexts(flow)
  const path = purchasingPage(companyId, flow.menu.orders)
  const listHref = documentsHref(companyId, flow.docTypes.orders)
  return (
    <FormPage crumb={{ app: texts.apps.purchasing.name, label: texts.purchasing.documents.heading, href: listHref }} title={ft.orders.newTitle}>
      <SalesDocForm
        kind={flow.kind === 'service' ? 'work-order' : 'purchase-order'}
        itemTypes={flow.itemTypes}
        apiPath={flow.api.orders(companyId)}
        partiesApi={purchasingApi.suppliers(companyId)}
        itemsApi={purchasingDocsApi.items(companyId)}
        docsPath={path}
        listHref={listHref}
        vat={await loadPurchaseVat(companyId)}
      />
    </FormPage>
  )
}

/**
 * ใบสั่งซื้อ / ใบสั่งจ้าง — ร่าง: แก้ไข / อนุมัติ / ลบ
 * อนุมัติแล้ว: รับสินค้า (ตรวจรับงาน) / บันทึกใบแจ้งหนี้ / ปิด / ยกเลิก + ความคืบหน้ารายบรรทัด; ใบสั่งซื้อที่มีเลขที่ = พิมพ์ A4
 */
export async function OrderViewPage({ companyId, docId, sp, flow }: DocProps) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing
  const ft = flowTexts(flow)
  const a = t.actions
  const service = flow.kind === 'service'
  const path = purchasingPage(companyId, flow.menu.orders)
  const listHref = documentsHref(companyId, flow.docTypes.orders, param(await sp, 'back'))
  const res = await loadItem<PurchaseOrder>(flow.api.orders(companyId), docId, ft.orders.notFound)
  if ('panel' in res) return res.panel
  const o = res.item
  const has = (p: string) => can(ctx.permissions, p)
  const toReceive = o.lines.some((l) => toUnits(l.quantity) > toUnits(l.received_quantity))
  const toInvoice = o.lines.some((l) => toUnits(l.received_quantity) > toUnits(l.invoiced_quantity))

  const actions: DocAction[] = []
  if (o.status === 'draft') {
    if (has(flow.perms.orderManage)) actions.push({ kind: 'delete', label: a.delete, confirm: a.confirm.delete! })
    if (has(flow.perms.orderApprove)) actions.push({ kind: 'patch', status: 'approved', label: a.approve, confirm: a.confirm.approve, primary: true })
  }
  if (o.status === 'approved') {
    if (has(flow.perms.orderManage)) {
      actions.push({ kind: 'patch', status: 'cancelled', label: service ? a.cancelWorkOrder : a.cancelOrder, confirm: a.confirm.cancelOrder, danger: true })
      actions.push({ kind: 'patch', status: 'closed', label: service ? a.closeWorkOrder : a.close, confirm: a.confirm.close })
    }
    if (toInvoice && has(flow.perms.invoicePost))
      actions.push({
        kind: 'link',
        href: `${purchasingPage(companyId, flow.menu.invoices)}/new?order_id=${o.id}`,
        label: service ? a.serviceInvoice : a.invoice,
        icon: 'invoice',
        primary: !toReceive,
      })
    if (toReceive && has(flow.perms.receiptPost))
      actions.push({
        kind: 'link',
        href: `${purchasingPage(companyId, flow.menu.receipts)}/new?order_id=${o.id}`,
        label: service ? a.accept : a.receive,
        icon: 'deliver',
        primary: true,
      })
  }
  if (o.order_no) actions.push({ kind: 'link', href: `${path}/${o.id}/print`, label: a.print, icon: 'print' })

  return (
    <FormPage
      crumb={{ app: texts.apps.purchasing.name, label: texts.purchasing.documents.heading, href: listHref }}
      title={o.order_no ?? ft.orders.draftTitle}
      description={o.supplier.name}
      aside={<PurchaseStatus kind="order" status={o.status} />}
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${flow.api.orders(companyId)}/${o.id}`} listHref={listHref} docNo={o.order_no} actions={actions} />
        {o.status === 'draft' && has(flow.perms.orderManage) ? (
          <SalesDocForm
            kind={service ? 'work-order' : 'purchase-order'}
            itemTypes={flow.itemTypes}
            apiPath={flow.api.orders(companyId)}
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
                      {format(ft.orders.progress, {
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

/** ใบรับสินค้า (ลงบัญชีแล้ว) / ใบตรวจรับงาน (ไม่มีบัญชี / สต็อก) — ยกเลิกได้เมื่อยังไม่ได้บันทึกใบแจ้งหนี้ */
export async function ReceiptViewPage({ companyId, docId, sp, flow }: DocProps) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing
  const ft = flowTexts(flow)
  const service = flow.kind === 'service'
  const listHref = documentsHref(companyId, flow.docTypes.receipts, param(await sp, 'back'))
  const res = await loadItem<GoodsReceipt>(flow.api.receipts(companyId), docId, ft.receipts.notFound)
  if ('panel' in res) return res.panel
  const r = res.item
  const entryHref = r.entry_id && can(ctx.permissions, 'ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${r.entry_id}` : null
  const actions: DocAction[] =
    r.status === 'posted' && can(ctx.permissions, flow.perms.receiptPost)
      ? [{ kind: 'patch', status: 'cancelled', label: t.actions.cancelDoc, confirm: t.actions.confirm.cancelDoc, danger: true }]
      : []
  return (
    <FormPage
      crumb={{ app: texts.apps.purchasing.name, label: texts.purchasing.documents.heading, href: listHref }}
      title={r.document_no}
      description={r.supplier.name}
      aside={<PurchaseStatus kind="posted" status={r.status} />}
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${flow.api.receipts(companyId)}/${r.id}`} listHref={listHref} docNo={r.document_no} actions={actions} />
        <Facts
          items={[
            [t.columns.supplier, `${r.supplier.code} ${r.supplier.name}`],
            [texts.sales.fields.date, dateText(r.date)],
            [ft.orders.heading, <EntryLink key="o" href={`${purchasingPage(companyId, flow.menu.orders)}/${r.order.id}`} no={r.order.order_no} />],
            ...(service
              ? []
              : ([
                  [t.facts.stockDocument, r.stock_document_no],
                  [t.facts.entry, <EntryLink key="e" href={entryHref} no={r.entry_no} />],
                  [t.facts.value, formatAmount(r.total_value)],
                ] as [string, React.ReactNode][])),
            [texts.sales.fields.reference, r.reference],
            [texts.sales.fields.description, r.description],
          ]}
        />
        <LinesTable
          rows={r.lines.map((l) =>
            service
              ? { key: l.order_line_no, item: l.item, quantity: l.quantity }
              : { key: l.order_line_no, item: l.item, quantity: l.quantity, unitPrice: l.unit_cost, amount: l.value },
          )}
        />
      </div>
    </FormPage>
  )
}

/**
 * ซื้อสินค้า / ซื้อบริการ — ผู้ขายตามใบกำกับ ณ วันบันทึก (bill_from), ใบลดหนี้, ยกเลิก
 * ซื้อบริการ: สถานะภาษีซื้อ + ใบกำกับภาษีที่ได้รับ (บันทึก / ยกเลิกที่หน้านี้)
 */
export async function InvoiceViewPage({ companyId, docId, sp, flow }: DocProps) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing
  const ft = flowTexts(flow)
  const a = t.actions
  const service = flow.kind === 'service'
  const listHref = documentsHref(companyId, flow.docTypes.invoices, param(await sp, 'back'))
  const res = await loadItem<SupplierInvoice>(flow.api.invoices(companyId), docId, ft.invoices.notFound)
  if ('panel' in res) return res.panel
  const inv = res.item
  const posted = inv.status === 'posted'
  const entryHref = inv.entry_id && can(ctx.permissions, 'ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${inv.entry_id}` : null
  const actions: DocAction[] = []
  if (posted && can(ctx.permissions, flow.perms.invoicePost)) actions.push({ kind: 'patch', status: 'cancelled', label: a.cancelDoc, confirm: a.confirm.cancelDoc, danger: true })
  if (posted && toUnits(inv.outstanding) > 0n && can(ctx.permissions, purchasingCreditPerm))
    actions.push({ kind: 'link', href: `${purchasingPage(companyId, 'credit-notes')}/new?invoice_id=${inv.id}${service ? '&kind=service' : ''}`, label: texts.creditNotes.purchasing.create, icon: 'invoice' })
  const b = inv.bill_from
  const vat = toUnits(inv.totals.vat) > 0n
  return (
    <FormPage
      crumb={{ app: texts.apps.purchasing.name, label: texts.purchasing.documents.heading, href: listHref }}
      title={inv.document_no}
      description={`${inv.supplier.name} · ${inv.supplier_invoice_no}`}
      aside={
        <span className="flex flex-wrap items-center gap-2">
          {service && posted && vat && <VatBadge status={inv.vat_status} />}
          <PurchaseStatus kind="posted" status={inv.status} />
        </span>
      }
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${flow.api.invoices(companyId)}/${inv.id}`} listHref={listHref} docNo={inv.document_no} actions={actions} />
        <Facts
          items={[
            [t.facts.billFrom, [b.name || inv.supplier.name, b.tax_id, branchLabel(b.branch_no)].filter(Boolean).join(' · ')],
            [service ? t.fulfil.serviceInvoiceNo : t.columns.invoiceNo, inv.supplier_invoice_no],
            [service ? t.fulfil.serviceInvoiceDate : t.facts.invoiceDate, dateText(inv.invoice_date)],
            [texts.sales.fields.date, dateText(inv.date)],
            [t.columns.due, dateText(inv.due_date)],
            [ft.orders.heading, <EntryLink key="o" href={`${purchasingPage(companyId, flow.menu.orders)}/${inv.order.id}`} no={inv.order.order_no} />],
            [t.facts.entry, <EntryLink key="e" href={entryHref} no={inv.entry_no} />],
            [t.facts.paid, formatAmount(inv.paid_amount)],
            [
              texts.creditNotes.columns.total,
              toUnits(inv.credited_amount) > 0n ? <EntryLink key="cn" href={`${purchasingPage(companyId, 'credit-notes')}?invoice_id=${inv.id}`} no={formatAmount(inv.credited_amount)} /> : '',
            ],
            [t.columns.outstanding, posted ? formatAmount(inv.outstanding) : ''],
            ...(service && vat
              ? ([
                  [t.vat.claimed, formatAmount(inv.vat_claimed ?? '0')],
                  [t.vat.undue, formatAmount(inv.vat_undue ?? '0', { dashZero: true })],
                ] as [string, React.ReactNode][])
              : []),
          ]}
        />
        <LinesTable
          rows={inv.lines.map((l) => ({
            key: l.line_no,
            item: l.item,
            quantity: l.quantity,
            unitPrice: l.unit_price,
            amount: inv.price_includes_vat ? l.amounts.gross : l.amounts.net,
            extra:
              (!service && toUnits(l.price_variance) !== 0n) || toUnits(l.credited_amount) > 0n ? (
                <span className="mt-1 block text-[12px] text-ink-muted">
                  {!service && toUnits(l.price_variance) !== 0n && `${t.facts.variance} ${formatAmount(l.price_variance)} `}
                  {toUnits(l.credited_amount) > 0n && `${texts.creditNotes.form.credited} ${formatAmount(l.credited_amount)}`}
                  {toUnits(l.returned_quantity) > 0n && ` · ${texts.creditNotes.form.returned} ${formatAmount(l.returned_quantity)}`}
                </span>
              ) : undefined,
          }))}
          totals={inv.totals}
          vatRate={inv.vat_rate}
          inclusive={inv.price_includes_vat}
        />
        {service && vat && (
          <TaxInvoicesPanel
            apiPath={`${flow.api.invoices(companyId)}/${inv.id}/tax-invoices`}
            records={inv.tax_invoices ?? []}
            withInvoice={inv.tax_invoice_no ? { no: inv.tax_invoice_no, date: inv.tax_invoice_date ?? inv.invoice_date, vat: inv.totals.vat } : null}
            undue={inv.vat_undue ?? '0'}
            suggested={inv.vat_suggested ?? '0'}
            canPost={posted && can(ctx.permissions, flow.perms.invoicePost)}
            entryBase={can(ctx.permissions, 'ledger.entry:read') ? companyPath(companyId, 'accounting', 'entries') : null}
          />
        )}
      </div>
    </FormPage>
  )
}

/** ใบลดหนี้ผู้ขายใช้สิทธิ์ของใบแจ้งหนี้สายสินค้า (purchasing.invoice:post) ทั้งสองสาย */
const purchasingCreditPerm = 'purchasing.invoice:post'
