/**
 * ข้อความทั้งหมดที่แสดงบนหน้าจอ (ภาษาไทย)
 *
 * แก้ข้อความที่นี่ที่เดียว แล้วทุก component ที่ใช้ข้อความนั้นจะเปลี่ยนตาม
 * ห้ามเขียนข้อความที่ผู้ใช้เห็นลงใน component โดยตรง — ให้เพิ่ม key ที่นี่แล้วอ้างผ่าน `texts`
 *
 * ข้อความที่มีตัวแปรใช้รูปแบบ {ชื่อ} แล้วแทนค่าด้วย format() เช่น format(texts.home.welcome, { name })
 */
export const th = {
  app: {
    name: 'APEXIS',
    tagline: 'ระบบบริหารจัดการองค์กร',
    logoAlt: 'APEXIS — Enterprise Resource Planning',
  },

  common: {
    loading: 'กำลังโหลด...',
    required: 'จำเป็น',
  },

  auth: {
    login: {
      pageTitle: 'เข้าสู่ระบบ',
      heading: 'เข้าสู่ระบบ',
      description: 'กรอกอีเมลและรหัสผ่านเพื่อเข้าใช้งาน',
      emailLabel: 'อีเมล',
      emailPlaceholder: 'name@company.com',
      passwordLabel: 'รหัสผ่าน',
      passwordPlaceholder: 'รหัสผ่านของคุณ',
      showPassword: 'แสดงรหัสผ่าน',
      hidePassword: 'ซ่อนรหัสผ่าน',
      submit: 'เข้าสู่ระบบ',
      submitting: 'กำลังเข้าสู่ระบบ...',
    },
    validation: {
      emailRequired: 'กรุณากรอกอีเมล',
      emailInvalid: 'รูปแบบอีเมลไม่ถูกต้อง',
      passwordRequired: 'กรุณากรอกรหัสผ่าน',
    },
    logout: 'ออกจากระบบ',
    sessionExpired: 'หมดเวลาการใช้งาน กรุณาเข้าสู่ระบบอีกครั้ง',
  },

  home: {
    pageTitle: 'หน้าหลัก',
    welcome: 'ยินดีต้อนรับ, {name}',
    description: 'คุณเข้าสู่ระบบในชื่อ {email}',
  },

  footer: {
    copyright: '© {year} APEXIS',
  },

  /**
   * ข้อความของ error จาก API — key คือ error.code ที่ backend ส่งมา
   * (backend ส่ง {"error": {"code": "...", "message": "..."}}; message เป็นภาษาอังกฤษสำหรับ developer)
   * code ที่ไม่มีในนี้จะแสดง `unknown`
   */
  errors: {
    invalid_email_or_password: 'อีเมลหรือรหัสผ่านไม่ถูกต้อง',
    user_is_inactive: 'บัญชีนี้ถูกปิดใช้งาน กรุณาติดต่อผู้ดูแลระบบ',
    unauthenticated: 'กรุณาเข้าสู่ระบบอีกครั้ง',
    permission_denied: 'คุณไม่มีสิทธิ์ทำรายการนี้',
    validation_failed: 'ข้อมูลไม่ครบหรือไม่ถูกต้อง',
    invalid_json_body: 'ข้อมูลที่ส่งไม่ถูกต้อง',
    network_error: 'ไม่สามารถเชื่อมต่อเซิร์ฟเวอร์ได้ กรุณาตรวจสอบการเชื่อมต่อแล้วลองใหม่',
    internal_error: 'ระบบขัดข้อง กรุณาลองใหม่ภายหลัง',
    unknown: 'เกิดข้อผิดพลาด กรุณาลองใหม่อีกครั้ง',
  } as Record<string, string> & { unknown: string; network_error: string },
}

export type Texts = typeof th
