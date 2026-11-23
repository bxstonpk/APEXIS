import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, SelectField, TextField, type Column } from '@/components'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { DOC_QUERY_KEYS, invApi, inventoryPage, invPerms, type StockDocSummary } from '@/features/inventory/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.inventory.documents.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const TONES: Record<string, 'success' | 'neutral' | 'warning'> = { draft: 'neutral', posted: 'success', cancelled: 'warning' }

/** เอกสารสต็อก — กรองประเภท / สถานะ / คลัง / ช่วงวันที่ใน URL (กรองและแบ่งหน้าใน DB) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.documents
  const path = inventoryPage(companyId, 'documents')
  const [list, warehouse] = await Promise.all([
    loadList<StockDocSummary>(invApi.documents(companyId), sp, DOC_QUERY_KEYS),
    loadRef(invApi.warehouses(companyId), param(sp, 'warehouse_id')),
  ])
  const createHref = can(ctx.permissions, invPerms.documentManage) && `${path}/new`
  const columns: Column<StockDocSummary>[] = [
    { key: 'date', header: t.fields.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    {
      key: 'no',
      header: texts.sales.columns.no,
      sortKey: 'document_no',
      render: (d) => (
        <Link href={rowHref(path, d.id, list.query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {d.document_no ?? <span className="font-sans text-ink-muted italic">{t.statuses.draft}</span>}
        </Link>
      ),
    },
    {
      key: 'type',
      header: t.fields.type,
      render: (d) => (
        <span className="block min-w-0">
          <span className="block">{t.types[d.type] ?? d.type}</span>
          <span className="block truncate text-[12px] text-ink-muted">
            {d.to_warehouse ? format(t.fromTo, { from: d.warehouse.name, to: d.to_warehouse.name }) : d.warehouse.name}
          </span>
        </span>
      ),
    },
    { key: 'reference', header: t.fields.reference, hideOnMobile: true, render: (d) => <span className="text-ink-muted">{d.reference || '–'}</span> },
    { key: 'value', header: t.lines.value, align: 'right', render: (d) => <span className="tabular-nums">{formatAmount(d.total_value)}</span> },
    {
      key: 'status',
      header: texts.common.status,
      render: (d) => (
        <Badge tone={TONES[d.status] ?? 'neutral'} dot>
          {t.statuses[d.status] ?? d.status}
        </Badge>
      ),
    },
  ]
  return (
    <>
      <ListHeader title={t.heading} description={t.description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="type"
          label={t.fields.type}
          defaultValue={param(sp, 'type')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(t.types).map(([value, label]) => ({ value, label }))]}
        />
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(t.statuses).map(([value, label]) => ({ value, label }))]}
        />
        <RefFilter label={t.fields.warehouse} name="warehouse_id" source={invApi.warehouses(companyId)} value={warehouse} />
        <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
        <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}
