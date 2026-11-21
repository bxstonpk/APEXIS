/**
 * ค่าตั้งของแอปที่ไม่ใช่ข้อความ: ที่อยู่ API, path ของหน้า, ชื่อ cookie
 * ข้อความที่แสดงบนหน้าจออยู่ที่ texts.ts / locales/
 */
export const appConfig = {
  api: {
    /** browser เรียก API ผ่าน Next (origin เดียวกัน) — Next เติม token จาก HttpOnly cookie แล้วส่งต่อไป backend */
    baseUrl: '/api/v1',
    /** route handler ของ Next สำหรับเข้า / ออกจากระบบ (ตั้ง / ลบ cookie) */
    session: '/api/session',
    /** path บน backend (Go) — ใช้ฝั่ง server ของ Next เท่านั้น */
    backend: {
      login: '/api/v1/auth/tokens',
    },
  },

  /** path ของหน้า — ใช้ทั้งใน router และลิงก์ / redirect */
  routes: {
    home: '/',
    login: '/login',
  },

  /** ชื่อ query parameter ที่ใช้พากลับไปหน้าเดิมหลังเข้าสู่ระบบ เช่น /login?redirect=/ledger/reports */
  redirectParam: 'redirect',

  /** cookie เก็บ session (HttpOnly — JavaScript ฝั่ง browser อ่านไม่ได้) */
  cookies: {
    session: 'apexis_session',
  },
} as const
