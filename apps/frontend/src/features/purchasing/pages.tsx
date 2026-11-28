import Link from 'next/link'
import { Badge, SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { FormPage, loadItem, NoPermission, RefFilter } from '@/features/organization/pages'
import { MessagePanel } from '@/features/shell'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'
import { DirectPurchaseForm } from './DirectPurchaseForm'
import { PurchaseFulfilForm } from './FulfilForm'
import {
  flows,
  PURCHASE_DOC_QUERY_KEYS,
  type PurchaseFlow,
  purchasingApi,
  purchasingDocsApi,
  purchasingPage,
  type GoodsReceipt,
  type PurchaseOrder,
  type PurchaseOrderSummary,
  type PurchasingSettings,
  type SupplierInvoiceSummary,
} from './types'

/** ส่วนที่ใช้ร่วมกันของหน้าแอปจัดซื้อ (server) */

/** ข้อความหัวหน้าของแต่ละสาย */
export function flowTexts(flow: PurchaseFlow) {
  const t = texts.purchasing
  return flow.kind === 'service'
    ? { orders: { ...t.orders, ...t.workOrders }, receipts: t.acceptances, invoices: t.serviceInvoices }
    : { orders: t.orders, receipts: t.receipts, invoices: t.invoices }
}

/** ป้ายสถานะภาษีซื้อของใบซื้อ */
export function VatBadge({ status }: { status: string | undefined }) {
  if (!status) return null
  const tone = status === 'claimed' ? 'success' : status === 'none' ? 'neutral' : 'warning'
  return <Badge tone={tone}>{texts.purchasing.vat.statuses[status] ?? status}</Badge>
}

const TONES: Record<string, 'success' | 'neutral' | 'warning'> = { draft: 'neutral', approved: 'success', posted: 'success', closed: 'neutral', cancelled: 'warning' }

export function PurchaseStatus({ kind, status }: { kind: 'order' | 'posted'; status: string }) {
  return (
    <Badge tone={TONES[status] ?? 'neutral'} dot>
      {texts.purchasing.statuses[kind][status] ?? status}
    </Badge>
  )
}

/** อัตรา VAT จากค่าตั้งจัดซื้อ — ใบสั่งซื้อคิด VAT เมื่อผู้ขายจด VAT; ไม่มีสิทธิ์อ่านค่าตั้ง = null (VAT คำนวณตอนบันทึก) */
export async function loadPurchaseVat(companyId: string): Promise<{ registered: boolean; rate: string } | null> {
  try {
    const { data } = await backendGet<PurchasingSettings>(purchasingDocsApi.settings(companyId))
    return { registered: true, rate: data.vat_rate }
  } catch {
    return null
  }
}

function Filters({ sp, companyId, kind, supplier }: { sp: SearchParams; companyId: string; kind: 'order' | 'posted'; supplier: Awaited<ReturnType<typeof loadRef>> }) {
  const t = texts.purchasing
  return (
    <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
      <SelectField
        name="status"
        label={texts.common.status}
        defaultValue={param(sp, 'status')}
        options={[{ value: '', label: texts.common.all }, ...Object.entries(t.statuses[kind]).map(([value, label]) => ({ value, label }))]}
      />
      <RefFilter label={t.columns.supplier} name="supplier_id" source={purchasingApi.suppliers(companyId)} value={supplier} />
      <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
      <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
    </ListToolbar>
  )
}

const noCell = (path: string, id: string, no: string | null, query: URLSearchParams) => (
  <Link href={rowHref(path, id, query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
    {no ?? <span className="font-sans text-ink-muted italic">{texts.purchasing.statuses.order.draft}</span>}
  </Link>
)

const supplierCell = (s: { name: string }, sub?: string | null) => (
  <span className="block min-w-0">
    <span className="block truncate">{s.name}</span>
    {sub && <span className="block font-mono text-[12px] text-ink-muted">{sub}</span>}
  </span>
)

export async function OrderList({
  companyId,
  sp,
  createHref,
  flow = flows.goods,
}: {
  companyId: string
  sp: SearchParams
  createHref: string | false
  flow?: PurchaseFlow
}) {
  const t = texts.purchasing
  const path = purchasingPage(companyId, flow.menu.orders)
  const [list, supplier] = await Promise.all([
    loadList<PurchaseOrderSummary>(flow.api.orders(companyId), sp, PURCHASE_DOC_QUERY_KEYS),
    loadRef(purchasingApi.suppliers(companyId), param(sp, 'supplier_id')),
  ])
  const columns: Column<PurchaseOrderSummary>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    { key: 'no', header: t.columns.no, sortKey: 'order_no', render: (d) => noCell(path, d.id, d.order_no, list.query) },
    { key: 'supplier', header: t.columns.supplier, render: (d) => supplierCell(d.supplier, d.reference) },
    { key: 'total', header: t.columns.total, align: 'right', render: (d) => <span className="font-medium tabular-nums">{formatAmount(d.totals.gross)}</span> },
    { key: 'status', header: t.columns.status, render: (d) => <PurchaseStatus kind="order" status={d.status} /> },
  ]
  return (
    <>
      <ListHeader title={flowTexts(flow).orders.heading} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <Filters sp={sp} companyId={companyId} kind="order" supplier={supplier} />
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}

export async function ReceiptList({ companyId, sp, flow = flows.goods }: { companyId: string; sp: SearchParams; flow?: PurchaseFlow }) {
  const t = texts.purchasing
  const path = purchasingPage(companyId, flow.menu.receipts)
  const service = flow.kind === 'service'
  const [list, supplier] = await Promise.all([
    loadList<GoodsReceipt>(flow.api.receipts(companyId), sp, PURCHASE_DOC_QUERY_KEYS),
    loadRef(purchasingApi.suppliers(companyId), param(sp, 'supplier_id')),
  ])
  const columns: Column<GoodsReceipt>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    { key: 'no', header: t.columns.no, sortKey: 'document_no', render: (d) => noCell(path, d.id, d.document_no, list.query) },
    { key: 'supplier', header: t.columns.supplier, render: (d) => supplierCell(d.supplier, d.order.order_no) },
    ...(service
      ? []
      : [{ key: 'value', header: t.columns.value, align: 'right', render: (d) => <span className="tabular-nums">{formatAmount(d.total_value)}</span> } as Column<GoodsReceipt>]),
    { key: 'status', header: t.columns.status, render: (d) => <PurchaseStatus kind="posted" status={d.status} /> },
  ]
  return (
    <>
      <ListHeader title={flowTexts(flow).receipts.heading} count={list.error ? undefined : list.meta.total} />
      <Filters sp={sp} companyId={companyId} kind="posted" supplier={supplier} />
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} />
    </>
  )
}

export async function InvoiceList({
  companyId,
  sp,
  flow = flows.goods,
  createHref = false,
}: {
  companyId: string
  sp: SearchParams
  flow?: PurchaseFlow
  createHref?: string | false
}) {
  const t = texts.purchasing
  const path = purchasingPage(companyId, flow.menu.invoices)
  const [list, supplier] = await Promise.all([
    loadList<SupplierInvoiceSummary>(flow.api.invoices(companyId), sp, PURCHASE_DOC_QUERY_KEYS),
    loadRef(purchasingApi.suppliers(companyId), param(sp, 'supplier_id')),
  ])
  const columns: Column<SupplierInvoiceSummary>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    { key: 'no', header: t.columns.no, sortKey: 'document_no', render: (d) => noCell(path, d.id, d.document_no, list.query) },
    { key: 'supplier', header: t.columns.supplier, render: (d) => supplierCell(d.supplier, d.supplier_invoice_no) },
    { key: 'due', header: t.columns.due, hideOnMobile: true, render: (d) => <span className="whitespace-nowrap">{formatDate(d.due_date)}</span> },
    { key: 'total', header: t.columns.total, align: 'right', hideOnMobile: true, render: (d) => <span className="tabular-nums">{formatAmount(d.totals.gross)}</span> },
    {
      key: 'outstanding',
      header: t.columns.outstanding,
      align: 'right',
      render: (d) => <span className="font-medium tabular-nums">{d.status === 'cancelled' ? '–' : formatAmount(d.outstanding, { dashZero: true })}</span>,
    },
    ...(flow.kind === 'service'
      ? [{ key: 'vat', header: t.vat.fields.vat, hideOnMobile: true, render: (d) => (d.status === 'cancelled' ? '–' : <VatBadge status={d.vat_status} />) } as Column<SupplierInvoiceSummary>]
      : []),
    { key: 'status', header: t.columns.status, render: (d) => <PurchaseStatus kind="posted" status={d.status} /> },
  ]
  return (
    <>
      <ListHeader title={flowTexts(flow).invoices.heading} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <Filters sp={sp} companyId={companyId} kind="posted" supplier={supplier} />
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}

/**
 * หน้ารับสินค้า / ตรวจรับงาน / บันทึกใบแจ้งหนี้จาก ?order_id= — ใบสั่งต้องอนุมัติแล้ว
 * ไม่มี order_id: ใบแจ้งหนี้ = ซื้อตรงไม่ผ่านใบสั่ง (ฟอร์มกรอกรายการเอง)
 */
export async function PurchaseFulfilPage({
  companyId,
  orderId,
  mode,
  flow = flows.goods,
}: {
  companyId: string
  orderId: string
  mode: 'receipt' | 'invoice'
  flow?: PurchaseFlow
}) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, mode === 'receipt' ? flow.perms.receiptPost : flow.perms.invoicePost)) return <NoPermission />
  const t = texts.purchasing
  const ft = flowTexts(flow)
  const page = mode === 'receipt' ? ft.receipts : ft.invoices
  const docsPath = purchasingPage(companyId, mode === 'receipt' ? flow.menu.receipts : flow.menu.invoices)
  if (!orderId) {
    if (mode === 'receipt') return <MessagePanel title={ft.orders.notFound} />
    return (
      <FormPage crumb={{ app: texts.apps.purchasing.name, label: page.heading, href: docsPath }} title={ft.invoices.directTitle}>
        <DirectPurchaseForm
          flow={flow.kind}
          apiPath={flow.api.invoices(companyId)}
          suppliersApi={purchasingApi.suppliers(companyId)}
          itemsApi={purchasingDocsApi.items(companyId)}
          itemTypes={flow.itemTypes}
          warehousesApi={purchasingDocsApi.warehouses(companyId)}
          docsPath={docsPath}
          vat={await loadPurchaseVat(companyId)}
        />
      </FormPage>
    )
  }
  const res = await loadItem<PurchaseOrder>(flow.api.orders(companyId), orderId, ft.orders.notFound)
  if ('panel' in res) return res.panel
  const o = res.item
  return (
    <FormPage crumb={{ app: texts.apps.purchasing.name, label: page.heading, href: docsPath }} title={page.newTitle} description={`${o.order_no ?? ''} · ${o.supplier.name}`}>
      {o.status === 'approved' ? (
        <PurchaseFulfilForm
          mode={mode}
          service={flow.kind === 'service'}
          order={o}
          apiPath={mode === 'receipt' ? flow.api.receipts(companyId) : flow.api.invoices(companyId)}
          warehousesApi={purchasingDocsApi.warehouses(companyId)}
          docsPath={docsPath}
          orderHref={`${purchasingPage(companyId, flow.menu.orders)}/${o.id}`}
        />
      ) : (
        <MessagePanel title={t.fulfil.orderNotApproved} />
      )}
    </FormPage>
  )
}
