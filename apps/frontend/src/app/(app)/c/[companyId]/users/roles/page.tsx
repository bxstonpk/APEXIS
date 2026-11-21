import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, type Column } from '@/components'
import { format, texts } from '@/config'
import { ListHeader, ListToolbar, ListView, rowHref } from '@/features/lists/ListParts'
import { ROLE_MANAGE, ROLE_QUERY_KEYS, usersApi, usersPage, type Role } from '@/features/users/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.roles.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function RolesPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const list = await loadList<Role>(usersApi.roles(companyId), sp, ROLE_QUERY_KEYS)
  const t = texts.roles
  const path = usersPage.roles(companyId)
  const createHref = can(ctx.permissions, ROLE_MANAGE) && `${path}/new`

  const columns: Column<Role>[] = [
    {
      key: 'name',
      header: t.columns.name,
      sortKey: 'name',
      render: (r) => (
        <span className="block min-w-0">
          <Link href={rowHref(path, r.id, list.query)} className="font-medium text-primary hover:underline">
            {r.name}
          </Link>
          {(r.is_system || r.description) && <span className="block truncate text-[13px] text-ink-muted">{r.is_system ? t.systemDescription : r.description}</span>}
        </span>
      ),
    },
    { key: 'code', header: t.columns.code, sortKey: 'code', render: (r) => <span className="text-ink-muted">{r.code}</span> },
    {
      key: 'permissions',
      header: t.columns.permissions,
      render: (r) => (r.permissions.includes('*') ? t.allPermissions : format(t.permissionCount, { n: r.permissions.length })),
    },
    { key: 'type', header: t.columns.type, sortKey: 'is_system', render: (r) => (r.is_system ? <Badge tone="info">{t.system}</Badge> : null) },
  ]

  return (
    <>
      <ListHeader title={t.heading} description={t.description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }} />
      <ListView list={list} path={path} columns={columns} rowKey={(r) => r.id} sort={param(sp, 'sort') || '-is_system'} createHref={createHref} />
    </>
  )
}
