import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { DOC_QUERY_KEYS, financeApi, financePage, perms, type DocumentSummary } from '@/features/finance/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.receipts.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** รายการเอกสาร — กรองประเภท / สถานะ / บัญชีเงิน / ช่วงวันที่ใน URL (กรองและแบ่งหน้าใน DB) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance
  const path = financePage(companyId, 'receipts')
  const [list, cash] = await Promise.all([
    loadList<DocumentSummary>(financeApi.receipts(companyId), sp, DOC_QUERY_KEYS),
    loadRef(financeApi.cashAccounts(companyId), param(sp, 'cash_account_id')),
  ])
  const createHref = can(ctx.permissions, perms.receiptPost) && `${path}/new`
  const columns: Column<DocumentSummary>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    {
      key: 'no',
      header: t.columns.no,
      sortKey: 'document_no',
      render: (d) => (
        <Link href={rowHref(path, d.id, list.query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {d.document_no}
        </Link>
      ),
    },
    {
      key: 'party',
      header: t.columns.party,
      render: (d) => (
        <span className="block min-w-0">
          <span className="block truncate">{d.party.name || '–'}</span>
          <span className="block text-[13px] text-ink-muted">{t.kinds.receipt[d.kind] ?? d.kind}</span>
        </span>
      ),
    },
    { key: 'amount', header: t.columns.amount, align: 'right', hideOnMobile: true, render: (d) => <span className="tabular-nums">{formatAmount(d.amount)}</span> },
    { key: 'wht', header: t.columns.wht, align: 'right', hideOnMobile: true, render: (d) => <span className="tabular-nums">{formatAmount(d.wht, { dashZero: true })}</span> },
    { key: 'net', header: t.columns.net, align: 'right', render: (d) => <span className="font-medium tabular-nums">{formatAmount(d.net)}</span> },
    {
      key: 'status',
      header: t.columns.status,
      render: (d) => (
        <Badge tone={d.status === 'posted' ? 'success' : 'neutral'} dot>
          {t.statuses[d.status] ?? d.status}
        </Badge>
      ),
    },
  ]
  return (
    <>
      <ListHeader title={texts.finance.receipts.heading} description={texts.finance.receipts.description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="kind"
          label={t.fields.kind}
          defaultValue={param(sp, 'kind')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(t.kinds.receipt).map(([value, label]) => ({ value, label }))]}
        />
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(t.statuses).map(([value, label]) => ({ value, label }))]}
        />
        <RefFilter label={t.fields.cashAccount} name="cash_account_id" source={financeApi.cashAccounts(companyId)} value={cash} />
        <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
        <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}
