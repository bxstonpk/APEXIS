import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { mfgApi, mfgPage, mfgPerms, ORDER_QUERY_KEYS, type ProductionOrder } from '@/features/manufacturing/types'
import { OrderStatus } from '@/features/manufacturing/Views'
import { RefFilter } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.manufacturing.orders.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ใบสั่งผลิต — กรองสถานะ / สินค้า / ช่วงวันที่ใน URL (กรองและแบ่งหน้าใน DB) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.manufacturing.orders
  const f = t.fields
  const path = mfgPage(companyId, 'orders')
  const [list, item] = await Promise.all([
    loadList<ProductionOrder>(mfgApi.orders(companyId), sp, ORDER_QUERY_KEYS),
    loadRef(mfgApi.items(companyId), param(sp, 'item_id')),
  ])
  const createHref = can(ctx.permissions, mfgPerms.orderManage) && `${path}/new`
  const columns: Column<ProductionOrder>[] = [
    { key: 'date', header: f.date, sortKey: 'date', render: (o) => <span className="whitespace-nowrap">{formatDate(o.date)}</span> },
    {
      key: 'no',
      header: f.no,
      sortKey: 'document_no',
      render: (o) => (
        <Link href={rowHref(path, o.id, list.query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {o.document_no ?? <span className="font-sans text-ink-muted italic">{texts.manufacturing.statuses.draft}</span>}
        </Link>
      ),
    },
    {
      key: 'item',
      header: f.item,
      render: (o) => (
        <span className="block min-w-0">
          <span className="block">
            <span className="text-[13px] text-ink-muted">{o.item.code}</span> {o.item.name}
          </span>
          <span className="block truncate text-[12px] text-ink-muted">{o.bom.code} · {o.warehouse.name}</span>
        </span>
      ),
    },
    {
      key: 'quantity',
      header: f.quantity,
      align: 'right',
      render: (o) => (
        <span className="tabular-nums whitespace-nowrap">
          {o.status === 'completed' && o.completed_quantity ? `${formatAmount(o.completed_quantity)} / ` : ''}
          {formatAmount(o.quantity)}
        </span>
      ),
    },
    { key: 'plan', header: f.plannedEnd, hideOnMobile: true, render: (o) => <span className="whitespace-nowrap text-ink-muted">{o.planned_end ? formatDate(o.planned_end) : '–'}</span> },
    { key: 'status', header: texts.common.status, render: (o) => <OrderStatus status={o.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading} description={t.description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(texts.manufacturing.statuses).map(([value, label]) => ({ value, label }))]}
        />
        <RefFilter label={f.item} name="item_id" source={mfgApi.items(companyId)} value={item} />
        <TextField name="from" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'from')} />
        <TextField name="to" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'to')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(o) => o.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}
