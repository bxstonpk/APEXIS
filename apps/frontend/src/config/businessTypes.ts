import { Briefcase, Factory, HardHat, type LucideIcon, Store } from 'lucide-react'
import { texts } from './texts'

/**
 * ประเภทธุรกิจของบริษัท — เลือกครั้งเดียวตอนสร้างบริษัท (แก้ภายหลังไม่ได้)
 *
 * backend ใช้กำหนดผังบัญชีมาตรฐาน + ค่าตั้งต้นที่นำเข้าตอนสร้างบริษัท
 * frontend ใช้กำหนดแอปที่แสดง (AppDefinition.businessTypes ใน config/apps.ts)
 */
export const BUSINESS_TYPES = ['service', 'trading', 'manufacturing', 'construction'] as const

export type BusinessType = (typeof BUSINESS_TYPES)[number]

export const businessTypeIcons: Record<BusinessType, LucideIcon> = {
  service: Briefcase,
  trading: Store,
  manufacturing: Factory,
  construction: HardHat,
}

export function isBusinessType(value: unknown): value is BusinessType {
  return typeof value === 'string' && (BUSINESS_TYPES as readonly string[]).includes(value)
}

/** ชื่อประเภทธุรกิจ (ค่าที่ไม่รู้จัก = คืนค่าเดิม) */
export function businessTypeName(value: string | undefined | null): string {
  return isBusinessType(value) ? texts.businessTypes.names[value] : (value ?? '')
}
