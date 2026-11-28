import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { pcApi, pcPage, pcPerms, REQUEST_QUERY_KEYS, type ReplenishmentRow } from '@/features/petty-cash/types'
import { RequestStatus } from '@/features/petty-cash/Views'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.pettyCash.requests.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** คำขอตั้งวงเงิน / เพิ่มวงเงิน / เบิกชดเชย — การเงินเป็นผู้จ่าย (กรองและแบ่งหน้าใน DB) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.requests
  const c = t.columns
  const path = pcPage(companyId, 'replenishments')
  const [list, fund] = await Promise.all([
    loadList<ReplenishmentRow>(pcApi.requests(companyId), sp, REQUEST_QUERY_KEYS),
    loadRef(pcApi.funds(companyId), param(sp, 'fund_id')),
  ])
  const createHref = can(ctx.permissions, pcPerms.requestPost) && `${path}/new`
  const columns: Column<ReplenishmentRow>[] = [
    { key: 'date', header: c.date, sortKey: 'date', render: (r) => <span className="whitespace-nowrap">{formatDate(r.date)}</span> },
    {
      key: 'no',
      header: c.no,
      sortKey: 'document_no',
      render: (r) => (
        <Link href={rowHref(path, r.id, list.query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {r.document_no}
        </Link>
      ),
    },
    { key: 'kind', header: c.kind, render: (r) => texts.pettyCash.requestKinds[r.kind] ?? r.kind },
    {
      key: 'fund',
      header: c.fund,
      hideOnMobile: true,
      render: (r) => (
        <span className="block min-w-0">
          <span className="text-[13px] text-ink-muted">{r.fund.code}</span> {r.fund.name}
        </span>
      ),
    },
    { key: 'vouchers', header: c.vouchers, align: 'right', hideOnMobile: true, render: (r) => <span className="tabular-nums">{r.kind === 'replenish' ? r.vouchers : '–'}</span> },
    { key: 'amount', header: c.amount, align: 'right', render: (r) => <span className="tabular-nums whitespace-nowrap">{formatAmount(r.amount)}</span> },
    { key: 'payment', header: c.payment, hideOnMobile: true, render: (r) => <span className="font-mono text-[13px] whitespace-nowrap text-ink-muted">{r.payment?.document_no ?? '–'}</span> },
    { key: 'status', header: texts.common.status, render: (r) => <RequestStatus status={r.status} /> },
  ]
  const opts = (m: Record<string, string>) => [{ value: '', label: texts.common.all }, ...Object.entries(m).map(([value, label]) => ({ value, label }))]
  return (
    <>
      <ListHeader title={t.heading} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={opts(texts.pettyCash.requestStatuses)} />
        <SelectField name="kind" label={c.kind} defaultValue={param(sp, 'kind')} options={opts(texts.pettyCash.requestKinds)} />
        <RefFilter label={c.fund} name="fund_id" source={pcApi.funds(companyId)} value={fund} />
        <TextField name="from" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'from')} />
        <TextField name="to" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'to')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(r) => r.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}
