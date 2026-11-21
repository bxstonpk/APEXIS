import { cache } from 'react'
import { ApiError } from '@/lib/http'
import { backendGet } from './backend'

export type CompanySummary = {
  id: string
  code: string
  name: string
  status: string
}

export type CompanyContext = {
  company: CompanySummary
  /** สิทธิ์ของผู้ใช้ในบริษัทนี้ (จาก GET /companies/{id}/access) */
  permissions: string[]
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export function isCompanyId(value: string): boolean {
  return UUID.test(value)
}

/**
 * บริษัท + สิทธิ์ของผู้ใช้ — เรียก backend 2 คำขอพร้อมกัน (access, company) ครั้งเดียวต่อ request
 * (React cache: layout / page / component ที่เรียกซ้ำใน request เดียวกันได้ผลเดิม)
 *
 * - ไม่ใช่สมาชิก / ไม่มีบริษัท / id ผิดรูปแบบ → null
 * - สมาชิกที่ไม่มีสิทธิ์ organization.company:read ยังเข้าได้ แต่ไม่รู้ชื่อบริษัท (แสดงเป็นค่าว่าง)
 */
export const getCompanyContext = cache(async (companyId: string): Promise<CompanyContext | null> => {
  if (!isCompanyId(companyId)) return null
  const base = `/companies/${companyId}`
  const [access, company] = await Promise.all([
    backendGet<{ permissions: string[] }>(`${base}/access`).catch(notMember),
    backendGet<CompanySummary>(base).catch((err: unknown) => {
      if (err instanceof ApiError && err.status === 403) return null
      return notMember(err)
    }),
  ])
  if (!access) return null
  return {
    company: company?.data ?? { id: companyId, code: '', name: '', status: 'active' },
    permissions: access.data.permissions ?? [],
  }
})

function notMember(err: unknown): null {
  if (err instanceof ApiError && [400, 403, 404].includes(err.status)) return null
  throw err
}
