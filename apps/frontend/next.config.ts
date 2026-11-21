import type { NextConfig } from 'next'

/**
 * - output standalone: build เป็น Node server ขนาดเล็ก (ต้องมี server เพราะ login / API ผ่าน route handler ของ Next)
 * - browser ไม่คุยกับ backend (Go) ตรง: เรียก /api/v1/* ของ Next ซึ่งเติม token จาก HttpOnly cookie แล้วส่งต่อ
 *   (ดู src/app/api/v1/[...path]/route.ts และ src/app/api/session/route.ts)
 */
const nextConfig: NextConfig = {
  output: 'standalone',
  reactStrictMode: true,
  poweredByHeader: false,
}

export default nextConfig
