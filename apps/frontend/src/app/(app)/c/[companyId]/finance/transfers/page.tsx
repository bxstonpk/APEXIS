import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { DOC_QUERY_KEYS, financeApi, financePage, perms, type Transfer } from '@/features/finance/types'
import { ListHeader, ListToolbar, ListView, RefText, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.transfers.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance
  const path = financePage(companyId, 'transfers')
  const list = await loadList<Transfer>(financeApi.transfers(companyId), sp, DOC_QUERY_KEYS)
  const createHref = can(ctx.permissions, perms.transferPost) && `${path}/new`
  const columns: Column<Transfer>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    { key: 'no', header: t.columns.no, sortKey: 'document_no', render: (d) => <Link href={rowHref(path, d.id, list.query)} className={`${rowLinkClass} font-mono text-[13px]`}>{d.document_no}</Link> },
    { key: 'from', header: t.columns.from, render: (d) => <RefText value={d.from} /> },
    { key: 'to', header: t.columns.to, render: (d) => <RefText value={d.to} /> },
    { key: 'amount', header: t.columns.amount, align: 'right', render: (d) => <span className="tabular-nums">{formatAmount(d.amount)}</span> },
    { key: 'status', header: t.columns.status, render: (d) => <Badge tone={d.status === 'posted' ? 'success' : 'neutral'} dot>{t.statuses[d.status] ?? d.status}</Badge> },
  ]
  return (
    <>
      <ListHeader title={t.transfers.heading} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(t.statuses).map(([value, label]) => ({ value, label }))]}
        />
        <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
        <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}
