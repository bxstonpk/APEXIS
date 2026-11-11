/**
 * ค่าตั้งของแอปที่ไม่ใช่ข้อความ: ที่อยู่ API, path ของหน้า, key ของ storage
 * ข้อความที่แสดงบนหน้าจออยู่ที่ texts.ts / locales/
 */
export const appConfig = {
  api: {
    /** ค่าเริ่มต้นใช้ path เดียวกับ frontend (dev server proxy /api ไป backend) — ตั้ง VITE_API_BASE_URL เพื่อเปลี่ยน */
    baseUrl: import.meta.env.VITE_API_BASE_URL ?? '/api/v1',
    endpoints: {
      login: '/auth/tokens',
      me: '/me',
    },
  },

  /** path ของหน้า — ใช้ทั้งใน router และลิงก์ / redirect */
  routes: {
    home: '/',
    login: '/login',
  },

  /** ชื่อ query parameter ที่ใช้พากลับไปหน้าเดิมหลังเข้าสู่ระบบ เช่น /login?redirect=/ledger/reports */
  redirectParam: 'redirect',

  storage: {
    session: 'apexis.session',
  },
} as const
