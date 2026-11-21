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
    search: 'ค้นหา',
    searchPlaceholder: 'ค้นหารหัส / ชื่อ',
    create: 'สร้าง',
    save: 'บันทึก',
    saving: 'กำลังบันทึก...',
    cancel: 'ยกเลิก',
    delete: 'ลบ',
    deleting: 'กำลังลบ...',
    back: 'กลับ',
    all: 'ทั้งหมด',
    empty: 'ไม่พบข้อมูล',
    status: 'สถานะ',
    statuses: { active: 'ใช้งาน', inactive: 'ปิดใช้งาน' } as Record<string, string>,
    sortAscending: 'เรียงจากน้อยไปมาก',
    sortDescending: 'เรียงจากมากไปน้อย',
    pagination: {
      label: 'แบ่งหน้า',
      previous: 'ก่อนหน้า',
      next: 'ถัดไป',
      summary: '{from}–{to} จาก {total} รายการ',
    },
    readOnly: 'คุณมีสิทธิ์ดูอย่างเดียว',
    noPermission: 'คุณไม่มีสิทธิ์เข้าถึงหน้านี้',
    notFound: 'ไม่พบหน้าที่ต้องการ',
    comingSoon: 'หน้านี้กำลังพัฒนา',
    comingSoonDescription: 'เมนูนี้ยังไม่เปิดให้ใช้งาน จะเพิ่มในขั้นถัดไป',
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

  companies: {
    pageTitle: 'เลือกบริษัท',
    heading: 'เลือกบริษัท',
    description: 'เลือกบริษัทที่ต้องการทำงาน',
    searchPlaceholder: 'ค้นหารหัส / ชื่อบริษัท',
    empty: 'คุณยังไม่เป็นสมาชิกของบริษัทใด — สร้างบริษัทแรกได้เลย',
    create: 'สร้างบริษัท',
    switch: 'เปลี่ยนบริษัท',
    unavailable: 'เปิดบริษัทนี้ไม่ได้',
    unavailableDescription: 'คุณอาจไม่ได้เป็นสมาชิกของบริษัทนี้แล้ว หรือบริษัทไม่มีอยู่',
    inactive: 'ปิดใช้งาน (อ่านอย่างเดียว)',
    new: {
      pageTitle: 'สร้างบริษัท',
      heading: 'สร้างบริษัท',
      description: 'คุณจะเป็นเจ้าของ (owner) ของบริษัทนี้ — แก้รายละเอียดอื่นได้ภายหลัง',
      code: 'รหัสบริษัท',
      codeHint: 'ตัวอักษรภาษาอังกฤษพิมพ์ใหญ่ ตัวเลข - หรือ _ ไม่เกิน 20 ตัว เช่น ACME',
      name: 'ชื่อบริษัท',
      taxId: 'เลขประจำตัวผู้เสียภาษี',
      submit: 'สร้างบริษัท',
    },
  },

  launcher: {
    pageTitle: 'แอปพลิเคชัน',
    empty: 'คุณยังไม่มีสิทธิ์ใช้งานแอปใดในบริษัทนี้ กรุณาติดต่อผู้ดูแลบริษัท',
    openHint: '{name} (เปิดในแท็บใหม่)',
  },

  shell: {
    close: 'ปิดและกลับหน้ารวมแอป',
    menu: 'เมนูของแอป',
  },

  /** ชื่อแอป (ไอคอนในหน้ารวมแอป) และเมนูด้านบนของแต่ละแอป — key ตรงกับ src/config/apps.ts */
  apps: {
    organization: {
      name: 'องค์กร',
      menu: {
        company: 'ข้อมูลบริษัท',
        branches: 'สาขา',
        'business-units': 'หน่วยธุรกิจ',
        departments: 'แผนก',
        'cost-centers': 'ศูนย์ต้นทุน',
        'project-sites': 'โครงการ / ไซต์งาน',
        'fiscal-years': 'ปีบัญชี / งวด',
        numbering: 'เลขที่เอกสาร',
      },
    },
    accounting: {
      name: 'บัญชี',
      menu: { entries: 'รายการบัญชี', accounts: 'ผังบัญชี', journals: 'สมุดรายวัน', reports: 'รายงาน' },
    },
    inventory: {
      name: 'คลังสินค้า',
      menu: {
        documents: 'เอกสารสต็อก',
        items: 'สินค้า',
        categories: 'หมวดสินค้า',
        uoms: 'หน่วยนับ',
        warehouses: 'คลัง',
        closing: 'ปิดงวดสินค้า',
        reports: 'รายงาน',
        settings: 'ตั้งค่า',
      },
    },
    purchasing: {
      name: 'จัดซื้อ',
      menu: {
        orders: 'ใบสั่งซื้อ',
        receipts: 'ใบรับสินค้า',
        invoices: 'ใบแจ้งหนี้',
        payments: 'ใบจ่ายชำระ',
        suppliers: 'ผู้ขาย',
        reports: 'รายงาน',
        settings: 'ตั้งค่า',
      },
    },
    sales: {
      name: 'ขาย',
      menu: {
        quotations: 'ใบเสนอราคา',
        orders: 'ใบสั่งขาย',
        deliveries: 'ใบส่งของ',
        invoices: 'ใบกำกับภาษี',
        receipts: 'ใบเสร็จรับเงิน',
        customers: 'ลูกค้า',
        reports: 'รายงาน',
        settings: 'ตั้งค่า',
      },
    },
    cashbank: {
      name: 'การเงิน',
      menu: { accounts: 'บัญชีเงินสด / ธนาคาร' },
    },
    users: {
      name: 'ผู้ใช้และสิทธิ์',
      menu: { members: 'สมาชิก', roles: 'บทบาท' },
    },
  },

  branches: {
    pageTitle: 'สาขา',
    heading: 'สาขา',
    newTitle: 'สร้างสาขา',
    editTitle: 'สาขา {code}',
    headOffice: 'สำนักงานใหญ่',
    headOfficeHint: 'สำนักงานใหญ่ (เลขที่สาขา 00000) ปิดใช้งานหรือลบไม่ได้',
    deleteConfirm: 'ต้องการลบสาขา {code} ใช่หรือไม่?',
    columns: {
      code: 'รหัส',
      name: 'ชื่อสาขา',
      taxBranchNo: 'เลขที่สาขา',
      phone: 'โทรศัพท์',
      province: 'จังหวัด',
      status: 'สถานะ',
    },
    form: {
      general: 'ข้อมูลทั่วไป',
      address: 'ที่อยู่',
      code: 'รหัสสาขา',
      codeHint: 'ตัวอักษรภาษาอังกฤษพิมพ์ใหญ่ ตัวเลข - หรือ _ ไม่เกิน 20 ตัว',
      name: 'ชื่อสาขา',
      taxBranchNo: 'เลขที่สาขา (กรมสรรพากร)',
      taxBranchNoHint: 'ตัวเลข 5 หลัก — สำนักงานใหญ่ใช้ 00000',
      phone: 'โทรศัพท์',
      status: 'สถานะ',
      addressLine: 'เลขที่ / อาคาร / ถนน',
      subDistrict: 'ตำบล / แขวง',
      district: 'อำเภอ / เขต',
      province: 'จังหวัด',
      postalCode: 'รหัสไปรษณีย์',
      country: 'ประเทศ (รหัส 2 ตัว)',
    },
  },

  validation: {
    required: 'กรุณากรอก{field}',
    code: 'ใช้ได้เฉพาะ A–Z, 0–9, - และ _ ไม่เกิน 20 ตัว',
    taxBranchNo: 'ต้องเป็นตัวเลข 5 หลัก',
    postalCode: 'รหัสไปรษณีย์ไทยต้องเป็นตัวเลข 5 หลัก',
    country: 'ต้องเป็นตัวอักษรภาษาอังกฤษ 2 ตัว เช่น TH',
    phone: 'ใช้ได้เฉพาะตัวเลข + ( ) - และช่องว่าง',
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
    invalid_query_parameter: 'เงื่อนไขการค้นหาไม่ถูกต้อง',
    invalid_code: 'รหัสใช้ได้เฉพาะ A–Z, 0–9, - และ _ ไม่เกิน 20 ตัว',
    invalid_name: 'ชื่อไม่ถูกต้องหรือยาวเกินไป',
    invalid_tax_branch_number: 'เลขที่สาขาต้องเป็นตัวเลข 5 หลัก',
    invalid_tax_id: 'เลขประจำตัวผู้เสียภาษีไม่ถูกต้อง',
    invalid_postal_code: 'รหัสไปรษณีย์ไม่ถูกต้อง',
    invalid_country_code: 'รหัสประเทศไม่ถูกต้อง',
    invalid_phone: 'เบอร์โทรศัพท์ไม่ถูกต้อง',
    invalid_address: 'ที่อยู่ยาวเกินไป',
    code_already_exists: 'รหัสนี้ถูกใช้แล้ว',
    branch_code_or_tax_branch_number_already_exists: 'รหัสสาขาหรือเลขที่สาขาซ้ำกับที่มีอยู่แล้ว',
    head_office_cannot_be_deactivated_or_deleted: 'สำนักงานใหญ่ปิดใช้งานหรือลบไม่ได้',
    record_is_referenced_by_other_records: 'ลบไม่ได้ เนื่องจากมีข้อมูลอื่นอ้างอิงอยู่',
    'company_is_inactive_(read-only)': 'บริษัทนี้ถูกปิดใช้งาน แก้ไขข้อมูลไม่ได้',
    company_not_found: 'ไม่พบบริษัท',
    branch_not_found: 'ไม่พบสาขา',
    invalid_json_body: 'ข้อมูลที่ส่งไม่ถูกต้อง',
    network_error: 'ไม่สามารถเชื่อมต่อเซิร์ฟเวอร์ได้ กรุณาตรวจสอบการเชื่อมต่อแล้วลองใหม่',
    internal_error: 'ระบบขัดข้อง กรุณาลองใหม่ภายหลัง',
    unknown: 'เกิดข้อผิดพลาด กรุณาลองใหม่อีกครั้ง',
  } as Record<string, string> & { unknown: string; network_error: string },
}

export type Texts = typeof th
