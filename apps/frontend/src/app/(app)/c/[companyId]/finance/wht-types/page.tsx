import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { financeApi, financePage, MASTER_QUERY_KEYS, perms, type WhtType } from '@/features/finance/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { statusFilterOptions } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.whtTypes.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance.whtTypes
  const path = financePage(companyId, 'wht-types')
  const list = await loadList<WhtType>(financeApi.whtTypes(companyId), sp, MASTER_QUERY_KEYS)
  const createHref = can(ctx.permissions, perms.settingsManage) && `${path}/new`
  const columns: Column<WhtType>[] = [
    { key: 'code', header: t.fields.code, sortKey: 'code', render: (w) => <Link href={rowHref(path, w.id, list.query)} className={rowLinkClass}>{w.code}</Link> },
    { key: 'name', header: t.fields.name, sortKey: 'name', render: (w) => w.name },
    { key: 'income', header: t.fields.incomeType, render: (w) => t.incomeTypesShort[w.income_type] ?? w.income_type },
    { key: 'rate', header: t.fields.rate, align: 'right', render: (w) => <span className="tabular-nums">{w.rate}</span> },
    { key: 'status', header: texts.common.status, render: (w) => <StatusBadge status={w.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading}createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(w) => w.id} sort={param(sp, 'sort') || 'code'} createHref={createHref} />
    </>
  )
}
