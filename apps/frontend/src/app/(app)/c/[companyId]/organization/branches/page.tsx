import { Plus, Search } from 'lucide-react'
import type { Metadata } from 'next'
import Form from 'next/form'
import Link from 'next/link'
import { Alert, Badge, Button, ButtonLink, DataTable, PageHeader, Pagination, SelectField, TextField, type Column } from '@/components'
import { appConfig, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { BRANCH_DEFAULT_SORT, BRANCH_QUERY_KEYS, branchesPath, type Branch } from '@/features/branches/types'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet, type ListMeta } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { param, pickQuery, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.branches.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/**
 * รายการสาขา — เงื่อนไขทั้งหมดอยู่ใน URL (?q=&status=&sort=&page=) แล้วส่งต่อให้ API ตรงๆ
 * คิวรี backend 1 ครั้ง (GET /branches — ค้นหา / เรียง / แบ่งหน้าใน DB); สิทธิ์มาจาก context ที่ AppShell โหลดไว้แล้ว (cache)
 */
export default async function BranchesPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null // AppShell แสดงข้อความแล้ว

  const query = pickQuery(sp, BRANCH_QUERY_KEYS)
  const apiQuery = new URLSearchParams(query)
  if (!apiQuery.has('page_size')) apiQuery.set('page_size', String(appConfig.pageSize))

  let rows: Branch[] = []
  let meta: ListMeta = { page: 1, page_size: appConfig.pageSize, total: 0 }
  let error: string | null = null
  try {
    const res = await backendGet<Branch[]>(branchesPath(companyId), apiQuery)
    rows = res.data
    meta = res.meta ?? meta
  } catch (err) {
    error = errorMessage(err)
  }

  const t = texts.branches
  const path = companyPath(companyId, 'organization', 'branches')
  const back = query.toString()
  const rowHref = (b: Branch) => withQuery(`${path}/${b.id}`, back ? { back } : {})
  const sort = param(sp, 'sort') || BRANCH_DEFAULT_SORT

  const columns: Column<Branch>[] = [
    {
      key: 'code',
      header: t.columns.code,
      sortKey: 'code',
      render: (b) => (
        <Link href={rowHref(b)} className="font-semibold text-primary hover:underline">
          {b.code}
        </Link>
      ),
    },
    {
      key: 'name',
      header: t.columns.name,
      sortKey: 'name',
      render: (b) => (
        <span className="flex flex-wrap items-center gap-2">
          {b.name}
          {b.is_head_office && <Badge tone="info">{t.headOffice}</Badge>}
        </span>
      ),
    },
    { key: 'tax', header: t.columns.taxBranchNo, sortKey: 'tax_branch_no', render: (b) => b.tax_branch_no },
    { key: 'phone', header: t.columns.phone, hideOnMobile: true, render: (b) => b.phone },
    { key: 'province', header: t.columns.province, hideOnMobile: true, render: (b) => b.address.province },
    {
      key: 'status',
      header: t.columns.status,
      render: (b) => <Badge tone={b.status === 'active' ? 'success' : 'neutral'}>{texts.common.statuses[b.status] ?? b.status}</Badge>,
    },
  ]

  return (
    <>
      <PageHeader
        title={t.heading}
        actions={
          can(ctx.permissions, 'organization.branch:manage') && (
            <ButtonLink href={`${path}/new`}>
              <Plus aria-hidden className="size-4" />
              {texts.common.create}
            </ButtonLink>
          )
        }
      />

      <Form action="" className="mb-4 flex flex-wrap items-end gap-2">
        <TextField
          name="q"
          type="search"
          label={texts.common.search}
          placeholder={texts.common.searchPlaceholder}
          defaultValue={param(sp, 'q')}
          className="min-w-48 flex-1 [&>label]:sr-only"
        />
        <SelectField
          name="status"
          label={texts.common.status}
          hideLabel
          defaultValue={param(sp, 'status')}
          options={[
            { value: '', label: `${texts.common.status}: ${texts.common.all}` },
            { value: 'active', label: texts.common.statuses.active! },
            { value: 'inactive', label: texts.common.statuses.inactive! },
          ]}
        />
        {param(sp, 'sort') && <input type="hidden" name="sort" value={param(sp, 'sort')} />}
        {param(sp, 'page_size') && <input type="hidden" name="page_size" value={param(sp, 'page_size')} />}
        <Button type="submit" variant="secondary" className="px-4">
          <Search aria-hidden className="size-4" />
          {texts.common.search}
        </Button>
      </Form>

      {error ? (
        <Alert tone="error">{error}</Alert>
      ) : (
        <>
          <DataTable
            columns={columns}
            rows={rows}
            rowKey={(b) => b.id}
            sort={sort}
            sortHref={(s) => withQuery(path, query, { sort: s, page: null })}
            emptyText={texts.common.empty}
            sortLabels={{ ascending: texts.common.sortAscending, descending: texts.common.sortDescending }}
          />
          <Pagination
            page={meta.page}
            pageSize={meta.page_size}
            total={meta.total}
            href={(page) => withQuery(path, query, { page })}
            labels={texts.common.pagination}
          />
        </>
      )}
    </>
  )
}
