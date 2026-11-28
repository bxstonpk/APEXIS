import Link from 'next/link'
import { redirect } from 'next/navigation'
import { Badge, PageHeader, SelectField, TextField, type Column } from '@/components'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { FormPage, loadItem, NoPermission, RefFilter } from '@/features/organization/pages'
import { MessagePanel } from '@/features/shell'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'
import { CreateMenu, type CreateMenuGroup } from './CreateMenu'
import { DirectPurchaseForm } from './DirectPurchaseForm'
import { PurchaseFulfilForm } from './FulfilForm'
import {
  docTypeMenu,
  documentsHref,
  flows,
  isOrderType,
  PURCHASE_DOC_TYPES,
  PURCHASE_DOCUMENTS_QUERY_KEYS,
  type PurchaseDocType,
  type PurchaseDocumentRow,
  type PurchaseFlow,
  type PurchaseKind,
  purchasingApi,
  purchasingDocsApi,
  purchasingPage,
  type PurchaseOrder,
  type PurchasingSettings,
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

const TYPE_TONES: Record<PurchaseKind, 'info' | 'neutral'> = { goods: 'info', service: 'neutral' }

/** ป้ายชนิดเอกสาร — สีแยกสายสินค้า / บริการ และมีชื่อชนิดเสมอ (ไม่สื่อด้วยสีอย่างเดียว) */
export function DocTypeBadge({ type }: { type: PurchaseDocType }) {
  const kind: PurchaseKind = flows.service.docTypes.orders === type || flows.service.docTypes.receipts === type || flows.service.docTypes.invoices === type ? 'service' : 'goods'
  return <Badge tone={TYPE_TONES[kind]}>{texts.purchasing.documents.types[type] ?? type}</Badge>
}

/** เมนูสร้างตามสิทธิ์: รับสินค้า / ตรวจรับงาน = รายการใบสั่งที่อนุมัติแล้ว (เลือกใบแล้วกดรับจากหน้าใบสั่ง) */
function createGroups(companyId: string, permissions: readonly string[]): CreateMenuGroup[] {
  const d = texts.purchasing.documents
  const docs = purchasingPage(companyId, 'documents')
  return (['goods', 'service'] as const)
    .map((kind) => {
      const flow = flows[kind]
      const items = [
        can(permissions, flow.perms.orderManage) && { label: d.createItems[flow.docTypes.orders]!, href: `${purchasingPage(companyId, flow.menu.orders)}/new` },
        can(permissions, flow.perms.receiptPost) && {
          label: d.createItems[flow.docTypes.receipts]!,
          href: `${docs}?type=${flow.docTypes.orders}&status=approved`,
        },
        can(permissions, flow.perms.invoicePost) && { label: d.createItems[flow.docTypes.invoices]!, href: `${purchasingPage(companyId, flow.menu.invoices)}/new` },
      ].filter((x): x is { label: string; href: string } => Boolean(x))
      return { label: d.groups[kind], items }
    })
    .filter((g) => g.items.length > 0)
}

/**
 * เอกสารซื้อ — รายการรวม 6 ชนิด (ใบสั่งซื้อ / ใบรับสินค้า / ซื้อสินค้า / ใบสั่งจ้าง / ตรวจรับงาน / ซื้อบริการ)
 * กรองชนิด / สถานะ / ผู้ขาย / วันที่ผ่าน URL; backend คืนเฉพาะชนิดที่มีสิทธิ์อ่าน; แถวลิงก์ไปหน้าเอกสารของชนิดนั้น
 */
export async function PurchaseDocumentList({ companyId, sp }: { companyId: string; sp: SearchParams }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing
  const d = t.documents
  const path = purchasingPage(companyId, 'documents')
  const [list, supplier] = await Promise.all([
    loadList<PurchaseDocumentRow>(purchasingDocsApi.documents(companyId), sp, PURCHASE_DOCUMENTS_QUERY_KEYS),
    loadRef(purchasingApi.suppliers(companyId), param(sp, 'supplier_id')),
  ])
  const groups = createGroups(companyId, ctx.permissions)
  const money = (v: string | null, opts?: { dashZero?: boolean }) => (v === null ? '' : <span className="tabular-nums">{formatAmount(v, opts)}</span>)
  const columns: Column<PurchaseDocumentRow>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (r) => <span className="whitespace-nowrap">{formatDate(r.date)}</span> },
    { key: 'no', header: t.columns.no, sortKey: 'document_no', render: (r) => noCell(purchasingPage(companyId, docTypeMenu[r.type]), r.id, r.document_no || null, list.query) },
    { key: 'type', header: d.type, render: (r) => <DocTypeBadge type={r.type} /> },
    { key: 'supplier', header: t.columns.supplier, render: (r) => supplierCell(r.supplier) },
    { key: 'reference', header: d.reference, hideOnMobile: true, render: (r) => <span className="font-mono text-[12.5px] text-ink-muted">{r.reference}</span> },
    { key: 'total', header: t.columns.total, align: 'right', hideOnMobile: true, render: (r) => money(r.total) },
    {
      key: 'outstanding',
      header: t.columns.outstanding,
      align: 'right',
      render: (r) => (r.outstanding === null ? '' : r.status === 'cancelled' ? '–' : <span className="font-medium">{money(r.outstanding, { dashZero: true })}</span>),
    },
    { key: 'vat', header: d.vat, hideOnMobile: true, render: (r) => (r.vat_status && r.status !== 'cancelled' ? <VatBadge status={r.vat_status} /> : null) },
    { key: 'status', header: t.columns.status, render: (r) => <PurchaseStatus kind={isOrderType(r.type) ? 'order' : 'posted'} status={r.status} /> },
  ]
  const typeOptions = [
    { value: '', label: texts.common.all },
    { value: 'order,receipt,invoice', label: d.typeGroups.goods },
    { value: 'work_order,service_acceptance,service_invoice', label: d.typeGroups.service },
    ...PURCHASE_DOC_TYPES.map((value) => ({ value, label: d.types[value]! })),
  ]
  const statusOptions = [
    { value: '', label: texts.common.all },
    ...Object.entries({ ...t.statuses.order, ...t.statuses.posted }).map(([value, label]) => ({ value, label })),
  ]
  return (
    <>
      <PageHeader
        title={d.heading}
        description={list.error ? undefined : format(texts.common.count, { n: list.meta.total.toLocaleString('th-TH') })}
        actions={groups.length > 0 && <CreateMenu label={d.create} groups={groups} />}
      />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="type" label={d.type} defaultValue={param(sp, 'type')} options={typeOptions} />
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusOptions} />
        <RefFilter label={t.columns.supplier} name="supplier_id" source={purchasingApi.suppliers(companyId)} value={supplier} />
        <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
        <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(r) => `${r.type}-${r.id}`} sort={param(sp, 'sort') || '-date'} />
    </>
  )
}

/**
 * รายการเดิมของแต่ละชนิด (/purchasing/orders ฯลฯ) → "เอกสารซื้อ" กรองชนิดนั้น (คงตัวกรองเดิม; ?type= ที่ส่งมาก่อน)
 * ใช้กับลิงก์เดิม / bookmark — หน้าใหม่ / หน้าเอกสาร / พิมพ์ ของแต่ละชนิดยังอยู่ที่ path เดิม
 */
export function redirectToDocuments(companyId: string, type: PurchaseDocType, sp: SearchParams): never {
  const back = new URLSearchParams()
  for (const k of PURCHASE_DOCUMENTS_QUERY_KEYS) {
    const v = param(sp, k)
    if (v) back.set(k, v)
  }
  redirect(documentsHref(companyId, type, back.toString()))
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
      <FormPage crumb={{ app: texts.apps.purchasing.name, label: t.documents.heading, href: documentsHref(companyId, flow.docTypes.invoices) }} title={ft.invoices.directTitle}>
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
    <FormPage
      crumb={{ app: texts.apps.purchasing.name, label: t.documents.heading, href: documentsHref(companyId, mode === 'receipt' ? flow.docTypes.receipts : flow.docTypes.invoices) }}
      title={page.newTitle} description={`${o.order_no ?? ''} · ${o.supplier.name}`}>
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
