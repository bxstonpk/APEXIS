import type { Metadata } from 'next'
import { Badge } from '@/components'
import { texts } from '@/config'
import { StatusBadge } from '@/features/lists/ListParts'
import { FormPage, loadItem } from '@/features/organization/pages'
import { MemberDetail } from '@/features/users/MemberDetail'
import { MEMBER_MANAGE, MEMBER_QUERY_KEYS, usersApi, usersPage, type Member, type Role } from '@/features/users/types'
import { can } from '@/lib/permissions'
import { currentUser } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack, loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.members.pageTitle }

type Props = { params: Promise<{ companyId: string; userId: string }>; searchParams: Promise<SearchParams> }

/** สมาชิกหนึ่งคน: บทบาท + สถานะ — สมาชิกและรายการบทบาทโหลดพร้อมกัน */
export default async function MemberPage({ params, searchParams }: Props) {
  const { companyId, userId } = await params
  const [ctx, me] = await Promise.all([getCompanyContext(companyId), currentUser()])
  if (!ctx) return null
  const t = texts.members
  const listHref = listHrefFromBack(usersPage.members(companyId), param(await searchParams, 'back'), MEMBER_QUERY_KEYS)
  const [res, roles] = await Promise.all([
    loadItem<Member>(usersApi.members(companyId), userId, texts.errors.membership_not_found!),
    loadList<Role>(usersApi.roles(companyId), { page_size: '100', sort: 'name' }, ['page_size', 'sort']),
  ])
  if ('panel' in res) return res.panel
  const member = res.item

  return (
    <FormPage
      crumb={{ app: texts.apps.users.name, label: t.heading, href: listHref }}
      title={member.user.name}
      description={member.user.email}
      aside={
        <span className="flex flex-wrap gap-1.5">
          <StatusBadge status={member.status} labels={t.statuses} />
          {member.user.must_change_password && (
            <Badge tone="warning" dot>
              {t.pendingPassword}
            </Badge>
          )}
        </span>
      }
    >
      <MemberDetail
        apiPath={usersApi.members(companyId)}
        member={member}
        roles={roles.rows}
        canManage={can(ctx.permissions, MEMBER_MANAGE)}
        isSelf={member.user.id === me.id}
      />
    </FormPage>
  )
}
