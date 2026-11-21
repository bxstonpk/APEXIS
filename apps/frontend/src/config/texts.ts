import { th, type Texts } from './locales/th'

/**
 * ข้อความที่ทั้งแอปใช้ — เพิ่มภาษาได้โดยสร้างไฟล์ใน locales/ ที่มีรูปร่างเดียวกัน (type Texts) แล้วสลับที่นี่
 */
export const texts: Texts = th

export type { Texts }

/** แทนค่า {ชื่อ} ในข้อความ เช่น format('ยินดีต้อนรับ, {name}', { name: 'สมชาย' }) */
export function format(template: string, values: Record<string, string | number>): string {
  return template.replace(/\{(\w+)\}/g, (match, key: string) =>
    key in values ? String(values[key]) : match,
  )
}
