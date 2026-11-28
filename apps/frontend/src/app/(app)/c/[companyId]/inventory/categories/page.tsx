import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { MASTER_QUERY_KEYS, invApi, inventoryPage, invPerms, type Category } from '@/features/inventory/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { statusFilterOptions } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.inventory.categories.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.categories
  const f = texts.inventory.fields
  const path = inventoryPage(companyId, 'categories')
  const list = await loadList<Category>(invApi.categories(companyId), sp, MASTER_QUERY_KEYS)
  const createHref = can(ctx.permissions, invPerms.categoryManage) && `${path}/new`
  const columns: Column<Category>[] = [
    { key: 'code', header: f.code, sortKey: 'code', render: (x) => <Link href={rowHref(path, x.id, list.query)} className={rowLinkClass}>{x.code}</Link> },
    { key: 'name', header: f.name, sortKey: 'name', render: (x) => x.name },
    { key: 'status', header: texts.common.status, render: (x) => <StatusBadge status={x.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading}createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(x) => x.id} sort={param(sp, 'sort') || 'code'} createHref={createHref} />
    </>
  )
}
