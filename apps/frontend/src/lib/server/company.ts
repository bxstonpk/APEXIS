import { cache } from 'react'
import type { BusinessType } from '@/config/businessTypes'
import { ApiError } from '@/lib/http'
import { backendGet } from './backend'

export type CompanySummary = {
  id: string
  code: string
  name: string
  tax_id?: string
  status: string
  /** ประเภทธุรกิจ (กำหนดตอนสร้างบริษัท) — ใช้เลือกแอปที่แสดง; ไม่มีค่า = แสดงทุกแอป */
  business_type?: BusinessType
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
 * บริษัท + สิทธิ์ของผู้ใช้ — เรียก backend 2 คำขอพร้อมกันครั้งเดียวต่อ request
 * (React cache: layout / page / component ที่เรียกซ้ำใน request เดียวกันได้ผลเดิม)
 *
 * - `/me/companies/{id}` (organization) ข้อมูลสรุปบริษัท — สมาชิกทุกคนอ่านได้ ไม่ต้องมีสิทธิ์อ่านบริษัท
 * - `/companies/{id}/access` (identity) สิทธิ์ของผู้ใช้
 * แยก 2 คำขอเพราะข้อมูลอยู่คนละ module (JOIN ข้าม module ไม่ได้) และเรียกพร้อมกันจึงไม่เพิ่มเวลารอ
 *
 * ไม่ใช่สมาชิก / ไม่มีบริษัท / id ผิดรูปแบบ → null
 */
export const getCompanyContext = cache(async (companyId: string): Promise<CompanyContext | null> => {
  if (!isCompanyId(companyId)) return null
  const [company, access] = await Promise.all([
    backendGet<CompanySummary>(`/me/companies/${companyId}`).catch(notMember),
    backendGet<{ permissions: string[] }>(`/companies/${companyId}/access`).catch(notMember),
  ])
  if (!company || !access) return null
  return { company: company.data, permissions: access.data.permissions ?? [] }
})

function notMember(err: unknown): null {
  if (err instanceof ApiError && [400, 403, 404].includes(err.status)) return null
  throw err
}
