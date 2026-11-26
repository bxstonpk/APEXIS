/**
 * ภาพของแบรนด์ — เปลี่ยนโลโก้ทั้งแอปได้ที่นี่ที่เดียว (ข้อความ alt อยู่ที่ texts.app.logoAlt)
 *
 * โลโก้: แนว "Folded Apex" (ริบบิ้นพับเป็นตัว A ยอดส้ม + เงา) — ต้นฉบับ docs/brand/logo/apexis-logo.html (+ apexis-mark.svg)
 * ไฟล์บนเว็บ: public/brand/apexis-logo.webp (แนวนอน 720×207, แสดงจริงประมาณครึ่งหนึ่ง)
 * favicon / apple-touch-icon อยู่ใน public/ (export จาก #favicon / #touch ในไฟล์ต้นฉบับเดียวกัน)
 * ภาพแผงแบรนด์หน้า login: ต้นฉบับ docs/brand/banners/login-hero/glass-1080x1080.html (จัตุรัส) → WebP 1280px (ข้อความ alt อยู่ที่ texts.app.loginHeroAlt)
 */
export const brand = {
  logo: {
    src: '/brand/apexis-logo.webp',
    width: 720,
    height: 207,
  },
  /** ภาพแบนเนอร์บนแผงซ้ายของหน้า login / เปลี่ยนรหัสผ่าน (จอกว้างเท่านั้น) — แผงใช้พื้น loginHero.background */
  loginHero: {
    src: '/brand/login-hero.webp',
    width: 1280,
    height: 1280,
    background: '#0a1330',
  },
  icons: {
    favicon: '/favicon-32.png',
    appleTouch: '/apple-touch-icon.png',
  },
} as const
