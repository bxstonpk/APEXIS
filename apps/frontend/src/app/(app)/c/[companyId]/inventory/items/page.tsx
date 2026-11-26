import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { RefFilter } from '@/features/organization/pages'
import { ITEM_QUERY_KEYS, invApi, inventoryPage, invPerms, type Item } from '@/features/inventory/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { statusFilterOptions } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.inventory.items.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.items
  const f = texts.inventory.fields
  const path = inventoryPage(companyId, 'items')
  const [list, category] = await Promise.all([
    loadList<Item>(invApi.items(companyId), sp, ITEM_QUERY_KEYS),
    loadRef(invApi.categories(companyId), param(sp, 'category_id')),
  ])
  const createHref = can(ctx.permissions, invPerms.itemManage) && `${path}/new`
  const columns: Column<Item>[] = [
    { key: 'code', header: f.code, sortKey: 'code', render: (x) => <Link href={rowHref(path, x.id, list.query)} className={rowLinkClass}>{x.code}</Link> },
    { key: 'name', header: f.name, sortKey: 'name', render: (x) => x.name },
    { key: 'type', header: f.type, hideOnMobile: true, render: (x) => texts.inventory.itemTypes[x.type] ?? x.type },
    { key: 'uom', header: f.uom, hideOnMobile: true, render: (x) => x.uom.name },
    { key: 'category', header: f.category, hideOnMobile: true, render: (x) => x.category?.name ?? '–' },
    { key: 'reorder', header: f.reorderPoint, align: 'right', hideOnMobile: true, render: (x) => <span className="tabular-nums">{x.reorder_point ? formatAmount(x.reorder_point) : '–'}</span> },
    { key: 'status', header: texts.common.status, render: (x) => <StatusBadge status={x.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading}createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="type"
          label={f.type}
          defaultValue={param(sp, 'type')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(texts.inventory.itemTypes).map(([value, label]) => ({ value, label }))]}
        />
        <RefFilter label={f.category} name="category_id" source={invApi.categories(companyId)} value={category} />
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(x) => x.id} sort={param(sp, 'sort') || 'code'} createHref={createHref} />
    </>
  )
}
