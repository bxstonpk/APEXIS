import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { BOM_QUERY_KEYS, mfgApi, mfgPage, mfgPerms, type Bom } from '@/features/manufacturing/types'
import { RefFilter } from '@/features/organization/pages'
import { statusFilterOptions } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.manufacturing.boms.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** สูตรการผลิต — ค้นหา / กรองสินค้าที่ผลิต / สถานะใน URL (กรองและแบ่งหน้าใน DB) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.manufacturing.boms
  const c = t.columns
  const path = mfgPage(companyId, 'boms')
  const [list, item] = await Promise.all([loadList<Bom>(mfgApi.boms(companyId), sp, BOM_QUERY_KEYS), loadRef(mfgApi.items(companyId), param(sp, 'item_id'))])
  const createHref = can(ctx.permissions, mfgPerms.bomManage) && `${path}/new`
  const columns: Column<Bom>[] = [
    {
      key: 'code',
      header: c.code,
      sortKey: 'code',
      render: (b) => (
        <Link href={rowHref(path, b.id, list.query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {b.code}
        </Link>
      ),
    },
    {
      key: 'name',
      header: c.name,
      sortKey: 'name',
      render: (b) => (
        <span className="flex flex-wrap items-center gap-2">
          {b.name}
          {b.is_default && <Badge tone="info">{t.defaultBadge}</Badge>}
        </span>
      ),
    },
    {
      key: 'item',
      header: c.item,
      render: (b) => (
        <span>
          <span className="text-[13px] text-ink-muted">{b.item.code}</span> {b.item.name}
        </span>
      ),
    },
    {
      key: 'output',
      header: c.output,
      align: 'right',
      hideOnMobile: true,
      render: (b) => (
        <span className="tabular-nums">
          {formatAmount(b.output_quantity)} {b.item.uom && <span className="text-[12px] text-ink-muted">{b.item.uom}</span>}
        </span>
      ),
    },
    { key: 'status', header: texts.common.status, render: (b) => <StatusBadge status={b.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading} description={t.description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <RefFilter label={c.item} name="item_id" source={mfgApi.items(companyId)} value={item} />
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(b) => b.id} sort={param(sp, 'sort') || 'code'} createHref={createHref} />
    </>
  )
}
