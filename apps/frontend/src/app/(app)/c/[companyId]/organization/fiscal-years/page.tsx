import type { Metadata } from 'next'
import Form from 'next/form'
import Link from 'next/link'
import { Button, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { ListHeader, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { orgResources, type FiscalYear } from '@/features/organization/resources'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.fiscalYears.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ปีบัญชี — เรียงปีล่าสุดก่อน, กรองด้วย ?year= */
export default async function FiscalYearsPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.fiscalYears
  const list = await loadList<FiscalYear>(r.api(companyId), sp, r.queryKeys)
  const f = texts.fiscalYears.fields
  const path = r.page(companyId)

  const columns: Column<FiscalYear>[] = [
    {
      key: 'year',
      header: f.year,
      sortKey: 'year',
      render: (y) => (
        <Link href={rowHref(path, y.id, list.query)} className={rowLinkClass}>
          {y.year}
        </Link>
      ),
    },
    { key: 'start', header: f.startDate, render: (y) => formatDate(y.start_date) },
    { key: 'end', header: f.endDate, render: (y) => formatDate(y.end_date) },
  ]

  return (
    <>
      <ListHeader title={texts.fiscalYears.heading} description={texts.fiscalYears.description} createHref={can(ctx.permissions, r.manage) && `${path}/new`} count={list.error ? undefined : list.meta.total} />
      <Form action="" className="mb-4 flex items-end gap-2">
        <TextField name="year" label={f.year} inputMode="numeric" maxLength={4} defaultValue={param(sp, 'year')} className="w-40" />
        <Button type="submit" variant="secondary" className="px-4">
          {texts.common.search}
        </Button>
      </Form>
      <ListView list={list} path={path} columns={columns} rowKey={(y) => y.id} sort={param(sp, 'sort') || r.defaultSort} createHref={can(ctx.permissions, r.manage) && `${path}/new`} />
    </>
  )
}
