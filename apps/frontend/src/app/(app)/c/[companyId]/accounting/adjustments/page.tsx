import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ADJUSTMENT_QUERY_KEYS, ADJUSTMENT_TEMPLATES, ledgerApi, ledgerPage, perms, type EntrySummary } from '@/features/accounting/types'
import { ListHeader, ListToolbar, ListView, rowLinkClass } from '@/features/lists/ListParts'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.adjustments.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const statusTone = { draft: 'warning', posted: 'success', reversed: 'neutral' } as const

/** รายการปรับปรุงสิ้นงวด — กรองแบบ / สถานะ / ช่วงวันที่ใน URL; แถวเปิดหน้ารายการบัญชี */
export default async function AdjustmentsPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.adjustments
  const path = ledgerPage.adjustments(companyId)
  const entries = ledgerPage.entries(companyId)
  const list = await loadList<EntrySummary>(ledgerApi.adjustments(companyId), sp, ADJUSTMENT_QUERY_KEYS)
  const createHref = can(ctx.permissions, perms.entryPost) && `${path}/new`
  const columns: Column<EntrySummary>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'entry_date', render: (e) => <span className="whitespace-nowrap">{formatDate(e.date)}</span> },
    {
      key: 'no',
      header: t.columns.no,
      sortKey: 'entry_no',
      render: (e) => (
        <Link href={`${entries}/${e.id}`} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {e.entry_no ?? texts.entries.noNumber}
        </Link>
      ),
    },
    {
      key: 'template',
      header: t.columns.template,
      render: (e) => (
        <span className="flex flex-wrap items-center gap-1.5">
          <span>{t.templates[e.template ?? ''] ?? e.template}</span>
          {e.adjusting_period && <Badge tone="info">{texts.entries.meta.adjustingPeriod}</Badge>}
        </span>
      ),
    },
    { key: 'description', header: t.columns.description, hideOnMobile: true, render: (e) => <span className="line-clamp-1">{e.description || '–'}</span> },
    { key: 'amount', header: t.columns.amount, align: 'right', render: (e) => <span className="tabular-nums">{formatAmount(e.total_debit)}</span> },
    {
      key: 'reversal',
      header: t.columns.reversal,
      hideOnMobile: true,
      render: (e) =>
        e.auto_reversal ? (
          <Link href={`${entries}/${e.auto_reversal.id}`} className="font-mono text-[13px] whitespace-nowrap text-primary hover:underline">
            {e.auto_reversal.entry_no ?? `${t.reversal.pending} ${formatDate(e.auto_reversal.date)}`}
          </Link>
        ) : (
          t.reversal.none
        ),
    },
    { key: 'status', header: t.columns.status, render: (e) => <Badge tone={statusTone[e.status]} dot>{texts.entries.statuses[e.status] ?? e.status}</Badge> },
  ]
  return (
    <>
      <ListHeader title={t.heading} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="template"
          label={t.filters.template}
          defaultValue={param(sp, 'template')}
          options={[{ value: '', label: t.filters.all }, ...ADJUSTMENT_TEMPLATES.map((k) => ({ value: k, label: t.templates[k]! }))]}
        />
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...(['posted', 'reversed'] as const).map((s) => ({ value: s, label: texts.entries.statuses[s]! }))]}
        />
        <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
        <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(e) => e.id} sort={param(sp, 'sort') || '-entry_date'} createHref={createHref} />
    </>
  )
}
