import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ENTRY_QUERY_KEYS, ledgerApi, ledgerPage, perms, type EntrySummary } from '@/features/accounting/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.entries.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const statusTone = { draft: 'warning', posted: 'success', reversed: 'neutral' } as const

/** รายการบัญชี — กรองสถานะ / สมุด / บัญชี / ช่วงวันที่ ทั้งหมดใน URL; รายการ + ชื่อตัวกรองอ้างอิงโหลดพร้อมกัน */
export default async function EntriesPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.entries
  const path = ledgerPage.entries(companyId)
  const [list, journal, account] = await Promise.all([
    loadList<EntrySummary>(ledgerApi.entries(companyId), sp, ENTRY_QUERY_KEYS),
    loadRef(ledgerApi.journals(companyId), param(sp, 'journal_id')),
    loadRef(ledgerApi.accounts(companyId), param(sp, 'account_id')),
  ])
  const createHref = can(ctx.permissions, perms.entryManage) && `${path}/new`
  const columns: Column<EntrySummary>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'entry_date', render: (e) => <span className="whitespace-nowrap">{formatDate(e.date)}</span> },
    {
      key: 'no',
      header: t.columns.no,
      sortKey: 'entry_no',
      render: (e) => (
        <Link href={rowHref(path, e.id, list.query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {e.entry_no ?? t.noNumber}
        </Link>
      ),
    },
    { key: 'journal', header: t.columns.journal, hideOnMobile: true, render: (e) => <span className="text-ink-muted">{e.journal.code}</span> },
    {
      key: 'description',
      header: t.columns.description,
      render: (e) => (
        <span className="block min-w-0">
          <span className="line-clamp-1">{e.description || '–'}</span>
          {e.reference && <span className="block text-[13px] text-ink-muted">{e.reference}</span>}
        </span>
      ),
    },
    { key: 'amount', header: t.columns.amount, align: 'right', render: (e) => <span className="tabular-nums">{formatAmount(e.total_debit)}</span> },
    { key: 'status', header: t.columns.status, render: (e) => <Badge tone={statusTone[e.status]} dot>{t.statuses[e.status] ?? e.status}</Badge> },
  ]
  return (
    <>
      <ListHeader title={t.heading} description={t.description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...(['draft', 'posted', 'reversed'] as const).map((s) => ({ value: s, label: t.statuses[s]! }))]}
        />
        <RefFilter label={t.filters.journal} name="journal_id" source={ledgerApi.journals(companyId)} value={journal} />
        <RefFilter label={t.filters.account} name="account_id" source={ledgerApi.accounts(companyId)} value={account} />
        <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
        <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(e) => e.id} sort={param(sp, 'sort') || '-entry_date'} createHref={createHref} />
    </>
  )
}
