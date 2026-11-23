import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { financeApi, financePage, MASTER_QUERY_KEYS, perms, type CashAccount } from '@/features/finance/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { statusFilterOptions } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.cashAccounts.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance.cashAccounts
  const path = financePage(companyId, 'cash-accounts')
  const list = await loadList<CashAccount>(financeApi.cashAccounts(companyId), sp, MASTER_QUERY_KEYS)
  const createHref = can(ctx.permissions, perms.accountManage) && `${path}/new`
  const columns: Column<CashAccount>[] = [
    { key: 'code', header: t.fields.code, sortKey: 'code', render: (a) => <Link href={rowHref(path, a.id, list.query)} className={rowLinkClass}>{a.code}</Link> },
    { key: 'name', header: t.fields.name, sortKey: 'name', render: (a) => a.name },
    { key: 'type', header: t.fields.type, render: (a) => t.types[a.type] ?? a.type },
    { key: 'bank', header: t.fields.accountNo, hideOnMobile: true, render: (a) => (a.type === 'bank' ? <span className="text-ink-muted">{a.bank_name} {a.account_no}</span> : '–') },
    { key: 'status', header: texts.common.status, render: (a) => <StatusBadge status={a.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading} description={t.description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="type"
          label={t.fields.type}
          defaultValue={param(sp, 'type')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(t.types).map(([value, label]) => ({ value, label }))]}
        />
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(a) => a.id} sort={param(sp, 'sort') || 'code'} createHref={createHref} />
    </>
  )
}
