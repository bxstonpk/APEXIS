import type { Metadata } from 'next'
import { Alert } from '@/components'
import { format, texts } from '@/config'
import { FormPage, loadItem } from '@/features/organization/pages'
import { RoleForm } from '@/features/users/RoleForm'
import { ROLE_MANAGE, ROLE_QUERY_KEYS, usersApi, usersPage, type PermissionEntry, type Role } from '@/features/users/types'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.roles.pageTitle }

type Props = { params: Promise<{ companyId: string; roleId: string }>; searchParams: Promise<SearchParams> }

/** แก้ไขบทบาท — บทบาทและรายการสิทธิ์ทั้งระบบโหลดพร้อมกัน */
export default async function RolePage({ params, searchParams }: Props) {
  const { companyId, roleId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const listHref = listHrefFromBack(usersPage.roles(companyId), param(await searchParams, 'back'), ROLE_QUERY_KEYS)
  const [res, catalog] = await Promise.all([
    loadItem<Role>(usersApi.roles(companyId), roleId, texts.errors.role_not_found!),
    backendGet<PermissionEntry[]>('/permissions').then(
      (r) => r.data,
      (err: unknown) => err,
    ),
  ])
  if ('panel' in res) return res.panel
  if (!Array.isArray(catalog)) return <Alert tone="error">{errorMessage(catalog)}</Alert>
  const role = res.item

  return (
    <FormPage crumb={{ app: texts.apps.users.name, label: texts.roles.heading, href: listHref }} title={format(texts.roles.editTitle, { name: role.name })} description={role.is_system ? texts.roles.systemDescription : role.description || role.code}>
      <RoleForm apiPath={usersApi.roles(companyId)} listHref={listHref} catalog={catalog} role={role} canManage={can(ctx.permissions, ROLE_MANAGE)} />
    </FormPage>
  )
}
