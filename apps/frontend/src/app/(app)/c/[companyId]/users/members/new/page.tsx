import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { MemberAddForm } from '@/features/users/MemberAddForm'
import { MEMBER_MANAGE, usersApi, usersPage, type Role } from '@/features/users/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'

export const metadata: Metadata = { title: texts.members.newTitle }

export default async function NewMemberPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, MEMBER_MANAGE)) return <NoPermission />
  const roles = await loadList<Role>(usersApi.roles(companyId), { page_size: '100', sort: 'name' }, ['page_size', 'sort'])
  const listHref = usersPage.members(companyId)
  return (
    <FormPage crumb={{ app: texts.apps.users.name, label: texts.members.heading, href: listHref }} title={texts.members.newTitle}>
      <MemberAddForm apiPath={usersApi.members(companyId)} listHref={listHref} roles={roles.rows} />
    </FormPage>
  )
}
