import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { pcApi, pcPage, pcPerms, VOUCHER_QUERY_KEYS, type VoucherRow } from '@/features/petty-cash/types'
import { VoucherStatus } from '@/features/petty-cash/Views'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.pettyCash.vouchers.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ใบสำคัญจ่ายเงินสดย่อย — กรองกองทุน / สถานะ / ช่วงวันที่ใน URL (กรองและแบ่งหน้าใน DB) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.vouchers
  const c = t.columns
  const path = pcPage(companyId, 'vouchers')
  const [list, fund] = await Promise.all([
    loadList<VoucherRow>(pcApi.vouchers(companyId), sp, VOUCHER_QUERY_KEYS),
    loadRef(pcApi.funds(companyId), param(sp, 'fund_id')),
  ])
  const createHref = can(ctx.permissions, pcPerms.voucherManage) && `${path}/new`
  const columns: Column<VoucherRow>[] = [
    { key: 'date', header: c.date, sortKey: 'date', render: (v) => <span className="whitespace-nowrap">{formatDate(v.date)}</span> },
    {
      key: 'no',
      header: c.no,
      sortKey: 'document_no',
      render: (v) => (
        <Link href={rowHref(path, v.id, list.query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {v.document_no}
        </Link>
      ),
    },
    {
      key: 'payee',
      header: c.payee,
      render: (v) => (
        <span className="block min-w-0">
          <span className="block">{v.payee_name}</span>
          {v.description && <span className="block truncate text-[12px] text-ink-muted">{v.description}</span>}
        </span>
      ),
    },
    { key: 'fund', header: c.fund, hideOnMobile: true, render: (v) => <span className="whitespace-nowrap">{v.fund.code}</span> },
    { key: 'tax', header: c.taxInvoice, hideOnMobile: true, render: (v) => <span className="whitespace-nowrap text-ink-muted">{v.tax_invoice_no || '–'}</span> },
    { key: 'gross', header: c.gross, align: 'right', render: (v) => <span className="tabular-nums whitespace-nowrap">{formatAmount(v.totals.gross)}</span> },
    { key: 'status', header: texts.common.status, render: (v) => <VoucherStatus status={v.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(texts.pettyCash.voucherStatuses).map(([value, label]) => ({ value, label }))]}
        />
        <RefFilter label={c.fund} name="fund_id" source={pcApi.funds(companyId)} value={fund} />
        <TextField name="from" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'from')} />
        <TextField name="to" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'to')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(v) => v.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}
