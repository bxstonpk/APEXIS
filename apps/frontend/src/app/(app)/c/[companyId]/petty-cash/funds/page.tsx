import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { statusFilterOptions } from '@/features/organization/resources'
import { FUND_QUERY_KEYS, pcApi, pcPage, pcPerms, type Fund } from '@/features/petty-cash/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.pettyCash.funds.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** กองทุนเงินสดย่อย — วงเงิน / เงินคงเหลือ / รอเบิกชดเชย (ค้นหาและแบ่งหน้าใน DB) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.funds
  const f = t.fields
  const path = pcPage(companyId, 'funds')
  const list = await loadList<Fund>(pcApi.funds(companyId), sp, FUND_QUERY_KEYS)
  const createHref = can(ctx.permissions, pcPerms.fundManage) && `${path}/new`
  const money = (v: string) => <span className="tabular-nums whitespace-nowrap">{formatAmount(v)}</span>
  const columns: Column<Fund>[] = [
    {
      key: 'code',
      header: f.code,
      sortKey: 'code',
      render: (x) => (
        <Link href={rowHref(path, x.id, list.query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {x.code}
        </Link>
      ),
    },
    {
      key: 'name',
      header: f.name,
      sortKey: 'name',
      render: (x) => (
        <span className="flex flex-wrap items-center gap-2">
          {x.name}
          {!x.funded && <Badge tone="warning">{f.notFunded}</Badge>}
        </span>
      ),
    },
    { key: 'custodian', header: f.custodian, hideOnMobile: true, render: (x) => x.custodian.name },
    { key: 'imprest', header: f.imprest, align: 'right', render: (x) => money(x.imprest_amount) },
    { key: 'cash', header: f.cashOnHand, align: 'right', render: (x) => money(x.cash_on_hand) },
    { key: 'pending', header: f.pending, align: 'right', hideOnMobile: true, render: (x) => money(x.pending_replenishment) },
    { key: 'status', header: texts.common.status, render: (x) => <StatusBadge status={x.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(x) => x.id} sort={param(sp, 'sort') || 'code'} createHref={createHref} />
    </>
  )
}
