import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** shadcn/ui: รวม class แบบมีเงื่อนไข + แก้ class ของ Tailwind ที่ชนกัน (เช่น p-4 กับ p-6 เหลือตัวหลัง) */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
