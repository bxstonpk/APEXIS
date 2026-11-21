import type { Metadata } from 'next'
import { Alert } from '@/components'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { RoleForm } from '@/features/users/RoleForm'
import { ROLE_MANAGE, usersApi, usersPage, type PermissionEntry } from '@/features/users/types'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.roles.newTitle }

export default async function NewRolePage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, ROLE_MANAGE)) return <NoPermission />
  let catalog: PermissionEntry[]
  try {
    catalog = (await backendGet<PermissionEntry[]>('/permissions')).data
  } catch (err) {
    return <Alert tone="error">{errorMessage(err)}</Alert>
  }
  const listHref = usersPage.roles(companyId)
  return (
    <FormPage crumb={{ app: texts.apps.users.name, label: texts.roles.heading, href: listHref }} title={texts.roles.newTitle}>
      <RoleForm apiPath={usersApi.roles(companyId)} listHref={listHref} catalog={catalog} canManage />
    </FormPage>
  )
}
