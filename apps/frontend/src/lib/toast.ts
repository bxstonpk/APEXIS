/**
 * ข้อความแจ้งผลสั้นๆ (toast) — เรียก showToast() จาก client component ใดก็ได้
 * - แสดงทันทีผ่าน event (Toaster ใน AppShell รับ)
 * - เก็บใน sessionStorage ด้วย เผื่อหน้าเปลี่ยนก่อน Toaster ได้แสดง (เช่น บันทึกแล้ว router.push ไปหน้ารายการ)
 */
export const TOAST_EVENT = 'apexis:toast'
export const TOAST_KEY = 'apexis.toast'

export function showToast(message: string): void {
  try {
    sessionStorage.setItem(TOAST_KEY, message)
  } catch {
    // private mode / storage ปิด — ยังแสดงผ่าน event ได้
  }
  window.dispatchEvent(new CustomEvent<string>(TOAST_EVENT, { detail: message }))
}

/** อ่านแล้วลบข้อความที่ค้างอยู่ (ใช้ใน Toaster ตอนเปิดหน้า) */
export function takePendingToast(): string | null {
  try {
    const m = sessionStorage.getItem(TOAST_KEY)
    if (m) sessionStorage.removeItem(TOAST_KEY)
    return m
  } catch {
    return null
  }
}
