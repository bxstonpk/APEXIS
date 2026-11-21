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

/** ตรวจ code / name ที่ทุกข้อมูลหลักมี */
export function validateCodeName(v: { code: string; name: string }, labels: { code: string; name: string }): FieldErrors<'code' | 'name'> {
  const e: FieldErrors<'code' | 'name'> = {}
  if (!v.code) e.code = requiredMessage(labels.code)
  else if (!patterns.code.test(v.code)) e.code = texts.validation.code
  if (!v.name.trim()) e.name = requiredMessage(labels.name)
  return e
}

/** ตรวจประเทศ / รหัสไปรษณีย์ของที่อยู่ (ไทยต้อง 5 หลัก) */
export function validateAddress(a: { postal_code: string; country: string }): FieldErrors<'postal_code' | 'country'> {
  const e: FieldErrors<'postal_code' | 'country'> = {}
  const country = a.country.trim().toUpperCase() || 'TH'
  if (!patterns.country.test(country)) e.country = texts.validation.country
  else if (country === 'TH' && a.postal_code.trim() && !patterns.thPostalCode.test(a.postal_code.trim())) e.postal_code = texts.validation.postalCode
  return e
}

/** วันสิ้นสุดต้องไม่ก่อนวันเริ่ม (YYYY-MM-DD เทียบเป็นข้อความได้) */
export function isDateRangeValid(from: string, to: string | null | undefined): boolean {
  return !from || !to || to >= from
}

/**
 * ส่งฟอร์มไม่ผ่าน: ย้ายโฟกัสไปช่องแรกที่ผิด (หลัง React วาดข้อความ error แล้ว)
 * ผู้ใช้คีย์บอร์ด / screen reader ไม่ต้องหาเองว่าช่องไหนผิด
 */
export function focusFirstInvalid(form: HTMLFormElement | null): void {
  requestAnimationFrame(() => form?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus())
}

/** รหัส (code) ใช้ตัวพิมพ์ใหญ่เสมอ — แปลงระหว่างพิมพ์ */
export function normalizeCode(value: string): string {
  return value.toUpperCase().replace(/\s+/g, '')
}
