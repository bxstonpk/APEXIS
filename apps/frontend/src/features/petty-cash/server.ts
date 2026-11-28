import type { Member } from '@/features/users/types'
import { backendGet, currentUser } from '@/lib/server/backend'
import { pcApi, type UserRef } from './types'

/**
 * ผู้ถือเงินที่เลือกได้ = สมาชิก active ของบริษัท (ต้องมีสิทธิ์ดูสมาชิก)
 * ไม่มีสิทธิ์ดูสมาชิก = เลือกได้เฉพาะตัวเอง (backend ตรวจว่าเป็นสมาชิกอีกครั้ง)
 */
export async function loadCustodians(companyId: string): Promise<UserRef[]> {
  const me = await currentUser()
  try {
    const { data } = await backendGet<Member[]>(pcApi.members(companyId), new URLSearchParams({ status: 'active', page_size: '100' }))
    const out = data.map((m) => ({ id: m.user.id, name: m.user.name }))
    return out.some((m) => m.id === me.id) ? out : [{ id: me.id, name: me.name }, ...out]
  } catch {
    return [{ id: me.id, name: me.name }]
  }
}
