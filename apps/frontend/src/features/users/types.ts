import { companyPath } from '@/config/apps'

/** ตรงกับ backend identity responses */
export type MemberUser = { id: string; email: string; name: string; status: string; must_change_password: boolean }
export type Member = { user: MemberUser; status: 'active' | 'suspended'; role_ids: string[]; joined_at: string }
export type Role = { id: string; company_id: string; code: string; name: string; description: string; permissions: string[]; is_system: boolean }
/** GET /api/v1/permissions */
export type PermissionEntry = { code: string; module: string; resource: string; action: string }

export const MEMBER_QUERY_KEYS = ['q', 'status', 'role_id', 'sort', 'page', 'page_size'] as const
export const ROLE_QUERY_KEYS = ['q', 'sort', 'page', 'page_size'] as const

export const MEMBER_READ = 'identity.member:read'
export const MEMBER_MANAGE = 'identity.member:manage'
export const ROLE_MANAGE = 'identity.role:manage'

export const usersApi = {
  members: (companyId: string) => `/companies/${companyId}/members`,
  roles: (companyId: string) => `/companies/${companyId}/roles`,
}

export const usersPage = {
  members: (companyId: string) => companyPath(companyId, 'users', 'members'),
  roles: (companyId: string) => companyPath(companyId, 'users', 'roles'),
}
