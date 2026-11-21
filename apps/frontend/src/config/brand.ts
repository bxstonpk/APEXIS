/**
 * ภาพของแบรนด์ — เปลี่ยนโลโก้ทั้งแอปได้ที่นี่ที่เดียว (ข้อความ alt อยู่ที่ texts.app.logoAlt)
 *
 * ต้นฉบับ: docs/brand/Apexis_Enterprise_Growth_Logo.png (1536x1024)
 * ไฟล์ที่ใช้บนเว็บอยู่ใน public/brand/ (ตัดขอบโปร่งใส ย่อเป็น WebP กว้าง 640px — แสดงจริงประมาณครึ่งหนึ่ง)
 * favicon / apple-touch-icon อยู่ใน public/ (ทำจากสัญลักษณ์ "A" ของโลโก้)
 */
export const brand = {
  logo: {
    src: '/brand/apexis-logo.webp',
    width: 640,
    height: 358,
  },
  icons: {
    favicon: '/favicon-32.png',
    appleTouch: '/apple-touch-icon.png',
  },
} as const
