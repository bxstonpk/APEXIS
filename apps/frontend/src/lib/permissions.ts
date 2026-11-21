/**
 * ตรวจสิทธิ์ฝั่ง frontend — ใช้ซ่อน / แสดงแอป เมนู และปุ่มเท่านั้น
 * backend ตรวจสิทธิ์เองทุกคำขอเสมอ (ซ่อนเมนูไม่ใช่การป้องกัน)
 *
 * กติกาเดียวกับ backend (identity Permission.Covers):
 *   "*"                 = ทุกสิทธิ์
 *   "sales.order:*"     = ทุก action ของ sales.order
 *   "sales.*:read"      = read ของทุก resource ที่ขึ้นต้นด้วย "sales."
 */
export function covers(granted: string, required: string): boolean {
  if (granted === '*') return true
  if (required === '*') return false
  const [gRes = '', gAct = ''] = granted.split(':', 2)
  const [rRes = '', rAct = ''] = required.split(':', 2)
  if (gAct !== '*' && gAct !== rAct) return false
  if (gRes === rRes) return true
  if (gRes.endsWith('*')) return rRes.startsWith(gRes.slice(0, -1))
  return false
}

/** ผู้ใช้ที่มีชุดสิทธิ์ permissions ทำ required ได้หรือไม่ */
export function can(permissions: readonly string[], required: string): boolean {
  return permissions.some((p) => covers(p, required))
}
