import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { statusFilterOptions } from '@/features/organization/resources'
import { CUSTOMER_QUERY_KEYS, perms, salesApi, salesPage, type Customer } from '@/features/sales/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.customers.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales.customers
  const path = salesPage(companyId, 'customers')
  const list = await loadList<Customer>(salesApi.customers(companyId), sp, CUSTOMER_QUERY_KEYS)
  const createHref = can(ctx.permissions, perms.customerManage) && `${path}/new`
  const columns: Column<Customer>[] = [
    { key: 'code', header: t.fields.code, sortKey: 'code', render: (c) => <Link href={rowHref(path, c.id, list.query)} className={rowLinkClass}>{c.code}</Link> },
    {
      key: 'name',
      header: t.fields.name,
      sortKey: 'name',
      render: (c) => (
        <span className="block min-w-0">
          <span className="block truncate">{c.name}</span>
          {c.tax_id && <span className="block font-mono text-[12px] text-ink-muted">{c.tax_id}</span>}
        </span>
      ),
    },
    { key: 'term', header: t.fields.term, align: 'right', hideOnMobile: true, render: (c) => <span className="tabular-nums">{c.payment_term_days}</span> },
    { key: 'credit', header: t.fields.creditLimit, align: 'right', hideOnMobile: true, render: (c) => <span className="tabular-nums">{c.credit_limit ? formatAmount(c.credit_limit) : '–'}</span> },
    { key: 'status', header: texts.common.status, render: (c) => <StatusBadge status={c.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading}createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(c) => c.id} sort={param(sp, 'sort') || 'code'} createHref={createHref} />
    </>
  )
}
