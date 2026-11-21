import { format, texts } from '@/config'

/**
 * กติกาตรวจข้อมูลก่อนส่ง (ช่วยผู้ใช้เท่านั้น — backend ตรวจซ้ำเสมอ)
 * รูปแบบเดียวกับ backend organization/domain/value_objects
 */
export const patterns = {
  code: /^[A-Z0-9][A-Z0-9_-]{0,19}$/,
  taxBranchNo: /^[0-9]{5}$/,
  thPostalCode: /^[0-9]{5}$/,
  country: /^[A-Z]{2}$/,
  phone: /^[0-9+()\- ]{3,30}$/,
} as const

export type FieldErrors<K extends string> = Partial<Record<K, string>>

/** ข้อความ "กรุณากรอก<ชื่อช่อง>" */
export function requiredMessage(label: string): string {
  return format(texts.validation.required, { field: label })
}

/** รหัส (code) ใช้ตัวพิมพ์ใหญ่เสมอ — แปลงระหว่างพิมพ์ */
export function normalizeCode(value: string): string {
  return value.toUpperCase().replace(/\s+/g, '')
}
