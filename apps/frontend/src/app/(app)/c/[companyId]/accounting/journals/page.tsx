import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { JOURNAL_QUERY_KEYS, ledgerApi, ledgerPage, perms, type Journal } from '@/features/accounting/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { statusFilterOptions } from '@/features/organization/resources'
import { DEFAULT_TEMPLATE } from '@/lib/numbering'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.journals.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function JournalsPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.journals
  const path = ledgerPage.journals(companyId)
  const list = await loadList<Journal>(ledgerApi.journals(companyId), sp, JOURNAL_QUERY_KEYS)
  const createHref = can(ctx.permissions, perms.journalManage) && `${path}/new`
  const columns: Column<Journal>[] = [
    { key: 'code', header: t.columns.code, sortKey: 'code', render: (j) => <Link href={rowHref(path, j.id, list.query)} className={rowLinkClass}>{j.code}</Link> },
    { key: 'name', header: t.columns.name, sortKey: 'name', render: (j) => j.name },
    { key: 'template', header: t.columns.template, hideOnMobile: true, render: (j) => <span className="font-mono text-[13px] text-ink-muted">{j.number_template || DEFAULT_TEMPLATE}</span> },
    { key: 'status', header: t.columns.status, render: (j) => <StatusBadge status={j.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.heading}createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(j) => j.id} sort={param(sp, 'sort') || 'code'} createHref={createHref} />
    </>
  )
}
