import Link from 'next/link'
import { Badge, SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { backendGet } from '@/lib/server/backend'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'
import type { Vat } from './SalesDocForm'
import { DOC_QUERY_KEYS, salesApi, type Customer, type InvoiceSummary, type Letterhead, type PostedSummary, type SalesDocSummary, type SalesSettings } from './types'

/** ส่วนที่ใช้ร่วมกันของหน้าแอปขาย (server) */

type StatusKind = 'quotation' | 'order' | 'posted'

const TONES: Record<string, 'success' | 'neutral' | 'info' | 'warning'> = {
  draft: 'neutral',
  sent: 'info',
  accepted: 'success',
  approved: 'success',
  posted: 'success',
  rejected: 'warning',
  cancelled: 'warning',
  closed: 'neutral',
}

export function SalesStatus({ kind, status }: { kind: StatusKind; status: string }) {
  return (
    <Badge tone={TONES[status] ?? 'neutral'} dot>
      {texts.sales.statuses[kind][status] ?? status}
    </Badge>
  )
}

/** ค่าตั้ง VAT สำหรับฟอร์มเอกสารใหม่ — ไม่มีสิทธิ์อ่านค่าตั้ง = null (ฟอร์มบอกว่า VAT คำนวณตอนบันทึก) */
export async function loadVat(companyId: string): Promise<Vat> {
  try {
    const { data } = await backendGet<SalesSettings>(salesApi.settings(companyId))
    return { registered: data.vat_registered, rate: data.vat_rate }
  } catch {
    return null
  }
}

export async function loadLetterhead(companyId: string): Promise<Letterhead | null> {
  try {
    return (await backendGet<Letterhead>(salesApi.letterhead(companyId))).data
  } catch {
    return null
  }
}

export async function loadCustomer(companyId: string, id: string): Promise<Customer | null> {
  try {
    return (await backendGet<Customer>(`${salesApi.customers(companyId)}/${encodeURIComponent(id)}`)).data
  } catch {
    return null
  }
}

function DocFilters({ sp, companyId, kind, customer }: { sp: SearchParams; companyId: string; kind: StatusKind; customer: Awaited<ReturnType<typeof loadRef>> }) {
  const t = texts.sales
  return (
    <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
      <SelectField
        name="status"
        label={texts.common.status}
        defaultValue={param(sp, 'status')}
        options={[{ value: '', label: texts.common.all }, ...Object.entries(t.statuses[kind]).map(([value, label]) => ({ value, label }))]}
      />
      <RefFilter label={t.fields.customer} name="customer_id" source={salesApi.customers(companyId)} value={customer} />
      <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
      <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
    </ListToolbar>
  )
}

const noCell = (path: string, id: string, no: string | null, query: URLSearchParams, draft: string) => (
  <Link href={rowHref(path, id, query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
    {no ?? <span className="font-sans text-ink-muted italic">{draft}</span>}
  </Link>
)

/** รายการใบเสนอราคา / ใบสั่งขาย */
export async function SalesDocList({
  companyId,
  sp,
  kind,
  path,
  apiPath,
  heading,
  description,
  createHref,
}: {
  companyId: string
  sp: SearchParams
  kind: 'quotation' | 'order'
  path: string
  apiPath: string
  heading: string
  description: string
  createHref: string | false
}) {
  const t = texts.sales
  const [list, customer] = await Promise.all([loadList<SalesDocSummary>(apiPath, sp, DOC_QUERY_KEYS), loadRef(salesApi.customers(companyId), param(sp, 'customer_id'))])
  const columns: Column<SalesDocSummary>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    { key: 'no', header: t.columns.no, sortKey: 'document_no', render: (d) => noCell(path, d.id, d.document_no, list.query, t.statuses[kind].draft!) },
    {
      key: 'customer',
      header: t.columns.customer,
      render: (d) => (
        <span className="block min-w-0">
          <span className="block truncate">{d.customer.name}</span>
          {d.reference && <span className="block text-[13px] text-ink-muted">{d.reference}</span>}
        </span>
      ),
    },
    { key: 'total', header: t.columns.total, align: 'right', render: (d) => <span className="font-medium tabular-nums">{formatAmount(d.totals.gross)}</span> },
    { key: 'status', header: t.columns.status, render: (d) => <SalesStatus kind={kind} status={d.status} /> },
  ]
  return (
    <>
      <ListHeader title={heading} description={description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <DocFilters sp={sp} companyId={companyId} kind={kind} customer={customer} />
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}

/** รายการใบส่งของ / ใบกำกับภาษี (เอกสารที่ลงบัญชีแล้ว) — สร้างจากหน้าใบสั่งขาย */
export async function PostedDocList({
  companyId,
  sp,
  invoices,
  path,
  apiPath,
  heading,
  description,
}: {
  companyId: string
  sp: SearchParams
  invoices: boolean
  path: string
  apiPath: string
  heading: string
  description: string
}) {
  const t = texts.sales
  const [list, customer] = await Promise.all([loadList<InvoiceSummary>(apiPath, sp, DOC_QUERY_KEYS), loadRef(salesApi.customers(companyId), param(sp, 'customer_id'))])
  const columns: Column<PostedSummary & Partial<InvoiceSummary>>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    { key: 'no', header: t.columns.no, sortKey: 'document_no', render: (d) => noCell(path, d.id, d.document_no, list.query, '') },
    {
      key: 'customer',
      header: t.columns.customer,
      render: (d) => (
        <span className="block min-w-0">
          <span className="block truncate">{d.customer.name}</span>
          {d.order && <span className="block font-mono text-[12px] text-ink-muted">{d.order.order_no}</span>}
        </span>
      ),
    },
    ...(invoices
      ? ([
          { key: 'due', header: t.columns.due, hideOnMobile: true, render: (d) => <span className="whitespace-nowrap">{formatDate(d.due_date ?? '')}</span> },
          { key: 'total', header: t.columns.total, align: 'right', hideOnMobile: true, render: (d) => <span className="tabular-nums">{formatAmount(d.totals?.gross)}</span> },
          {
            key: 'outstanding',
            header: t.columns.outstanding,
            align: 'right',
            render: (d) => <span className="font-medium tabular-nums">{d.status === 'cancelled' ? '–' : formatAmount(d.outstanding, { dashZero: true })}</span>,
          },
        ] as Column<PostedSummary & Partial<InvoiceSummary>>[])
      : []),
    { key: 'status', header: t.columns.status, render: (d) => <SalesStatus kind="posted" status={d.status} /> },
  ]
  return (
    <>
      <ListHeader title={heading} description={description} count={list.error ? undefined : list.meta.total} />
      <DocFilters sp={sp} companyId={companyId} kind="posted" customer={customer} />
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} />
    </>
  )
}
