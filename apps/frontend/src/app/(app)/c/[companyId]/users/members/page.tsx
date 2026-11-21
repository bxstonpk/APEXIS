import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, SelectField, type Column } from '@/components'
import { format, texts } from '@/config'
import { ListHeader, ListToolbar, ListView, rowHref, StatusBadge } from '@/features/lists/ListParts'
import { initials } from '@/features/shell/TopBar'
import { MEMBER_MANAGE, MEMBER_QUERY_KEYS, usersApi, usersPage, type Member, type Role } from '@/features/users/types'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { currentUser } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.members.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/**
 * สมาชิก — รายการ + บทบาททั้งหมดของบริษัท โหลดพร้อมกัน 2 คำขอ:
 * บทบาทใช้แสดงชื่อบทบาทในตาราง และเป็นตัวเลือกกรอง (จำนวนบทบาทต่อบริษัทมีน้อย โหลดครั้งเดียวคุ้มกว่า JOIN ทุกแถว)
 */
export default async function MembersPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const [ctx, me] = await Promise.all([getCompanyContext(companyId), currentUser()])
  if (!ctx) return null

  const allRoles = new URLSearchParams({ page_size: '100', sort: 'name' })
  const [list, roles] = await Promise.all([
    loadList<Member>(usersApi.members(companyId), sp, MEMBER_QUERY_KEYS),
    loadList<Role>(usersApi.roles(companyId), Object.fromEntries(allRoles), ['page_size', 'sort']),
  ])
  const roleName = new Map(roles.rows.map((r) => [r.id, r.name]))
  const t = texts.members
  const path = usersPage.members(companyId)
  const createHref = can(ctx.permissions, MEMBER_MANAGE) && `${path}/new`

  const columns: Column<Member>[] = [
    {
      key: 'name',
      header: t.columns.name,
      sortKey: 'name',
      render: (m) => (
        <Link href={rowHref(path, m.user.id, list.query)} className="group flex items-center gap-3">
          <span aria-hidden className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary-soft text-[11px] font-semibold text-primary">
            {initials(m.user.name)}
          </span>
          <span className="min-w-0">
            <span className="flex flex-wrap items-center gap-1.5 font-medium text-primary group-hover:underline">
              {m.user.name}
              {m.user.id === me.id && <Badge>{t.you}</Badge>}
            </span>
            <span className="block truncate text-[13px] text-ink-muted">{m.user.email}</span>
          </span>
        </Link>
      ),
    },
    {
      key: 'roles',
      header: t.columns.roles,
      render: (m) => (
        <span className="flex flex-wrap gap-1">
          {m.role_ids.map((id) => (
            <Badge key={id} tone="info">
              {roleName.get(id) ?? '–'}
            </Badge>
          ))}
        </span>
      ),
    },
    {
      key: 'status',
      header: t.columns.status,
      sortKey: 'status',
      render: (m) => (
        <span className="flex flex-wrap gap-1">
          <StatusBadge status={m.status} labels={t.statuses} />
          {m.user.must_change_password && (
            <Badge tone="warning" dot>
              {t.pendingPassword}
            </Badge>
          )}
        </span>
      ),
    },
    { key: 'joined', header: t.columns.joinedAt, sortKey: 'joined_at', hideOnMobile: true, render: (m) => formatDate(m.joined_at.slice(0, 10)) },
  ]

  return (
    <>
      <ListHeader title={t.heading} description={t.description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(t.statuses).map(([value, label]) => ({ value, label }))]}
        />
        <SelectField
          name="role_id"
          label={t.filters.role}
          defaultValue={param(sp, 'role_id')}
          options={[{ value: '', label: texts.common.all }, ...roles.rows.map((r) => ({ value: r.id, label: r.name }))]}
        />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(m) => m.user.id} sort={param(sp, 'sort') || 'name'} createHref={createHref} />
      <p className="sr-only">{format(texts.common.count, { n: list.meta.total })}</p>
    </>
  )
}
