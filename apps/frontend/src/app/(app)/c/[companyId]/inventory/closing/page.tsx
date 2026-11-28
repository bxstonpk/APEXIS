import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, type Column } from '@/components'
import { texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount } from '@/features/accounting/money'
import { CancelClosing, CreateClosing } from '@/features/inventory/ClosingActions'
import { invApi, inventoryPage, invPerms, type Closing } from '@/features/inventory/types'
import { ListHeader, ListView } from '@/features/lists/ListParts'
import { formatDate, today } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.inventory.closing.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ปิดงวดสต็อก (ระบบ periodic) — รายการการปิดงวด + ปิดงวดใหม่ / ยกเลิก */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.closing
  const c = t.columns
  const path = inventoryPage(companyId, 'closing')
  const list = await loadList<Closing>(invApi.closings(companyId), sp, ['status', 'sort', 'page', 'page_size'])
  const canPost = can(ctx.permissions, invPerms.closingPost)
  const canEntry = can(ctx.permissions, 'ledger.entry:read')
  const now = today()
  const lastMonthEnd = new Date(Date.UTC(Number(now.slice(0, 4)), Number(now.slice(5, 7)) - 1, 0)).toISOString().slice(0, 10)
  const columns: Column<Closing>[] = [
    { key: 'period', header: c.period, sortKey: 'period_end', render: (r) => <span className="whitespace-nowrap">{formatDate(r.period_end)}</span> },
    { key: 'year', header: c.year, hideOnMobile: true, render: (r) => r.fiscal_year },
    { key: 'adjustment', header: c.adjustment, align: 'right', render: (r) => <span className="tabular-nums">{formatAmount(r.adjustment)}</span> },
    {
      key: 'entry',
      header: c.entry,
      hideOnMobile: true,
      render: (r) =>
        r.entry_id && canEntry ? (
          <Link className="font-mono text-[13px] text-primary hover:underline" href={`${companyPath(companyId, 'accounting', 'entries')}/${r.entry_id}`}>
            {r.entry_no}
          </Link>
        ) : (
          <span className="font-mono text-[13px]">{r.entry_no || '–'}</span>
        ),
    },
    {
      key: 'status',
      header: c.status,
      render: (r) => (
        <Badge tone={r.status === 'posted' ? 'success' : 'warning'} dot>
          {t.statuses[r.status] ?? r.status}
        </Badge>
      ),
    },
    {
      key: 'actions',
      header: '',
      align: 'right',
      render: (r) => (r.status === 'posted' && canPost ? <CancelClosing apiPath={`${invApi.closings(companyId)}/${r.id}`} periodEnd={r.period_end} /> : null),
    },
  ]
  return (
    <div className="flex flex-col gap-4">
      <ListHeader title={t.heading}count={list.error ? undefined : list.meta.total} />
      {canPost && <CreateClosing apiPath={invApi.closings(companyId)} defaultDate={lastMonthEnd} />}
      <ListView list={list} path={path} columns={columns} rowKey={(r) => r.id} sort={param(sp, 'sort') || '-period_end'} />
    </div>
  )
}
