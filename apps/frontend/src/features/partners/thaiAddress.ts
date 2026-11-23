/**
 * ที่อยู่ตามแบบกรมสรรพากร (address_detail ของลูกค้า / ผู้ขาย / ผลค้นหาผู้จด VAT)
 * formatThaiAddress ใช้กฎเดียวกับ backend platform/thaiaddress.Format — ใช้แสดงตัวอย่างขณะกรอก (backend ประกอบจริงตอนบันทึก)
 */
export type ThaiAddress = {
  building: string
  room_no: string
  floor_no: string
  village: string
  house_no: string
  moo: string
  soi: string
  yaek: string
  street: string
  sub_district: string
  district: string
  province: string
  postal_code: string
}

export const ADDRESS_KEYS = [
  'house_no',
  'moo',
  'building',
  'room_no',
  'floor_no',
  'village',
  'soi',
  'yaek',
  'street',
  'sub_district',
  'district',
  'province',
  'postal_code',
] as const satisfies readonly (keyof ThaiAddress)[]

export const emptyThaiAddress: ThaiAddress = Object.fromEntries(ADDRESS_KEYS.map((k) => [k, ''])) as ThaiAddress

/** ตัดช่องว่างซ้ำ และ "-" (สรรพากรใช้แทนค่าว่าง) เป็นค่าว่าง */
export function cleanPart(s: string | null | undefined): string {
  const v = (s ?? '').split(/\s+/).filter(Boolean).join(' ')
  return v === '-' ? '' : v
}

export function cleanAddress(a: Partial<ThaiAddress> | null | undefined): ThaiAddress {
  return Object.fromEntries(ADDRESS_KEYS.map((k) => [k, cleanPart(a?.[k])])) as ThaiAddress
}

const BANGKOK = new Set(['กรุงเทพมหานคร', 'กรุงเทพฯ', 'กรุงเทพ', 'กทม.', 'กทม'])

export function isBangkok(province: string): boolean {
  return BANGKOK.has(cleanPart(province).replace(/^จังหวัด/, ''))
}

const prefixed = (prefix: string, v: string) => (!v ? '' : v.startsWith(prefix.trim()) ? v : prefix + v)

/** ที่อยู่บรรทัดเดียว เช่น "เลขที่ 72/7 หมู่ที่ 3 ตำบลท่าทราย อำเภอเมืองสมุทรสาคร จังหวัดสมุทรสาคร 74000" */
export function formatThaiAddress(input: Partial<ThaiAddress>): string {
  const a = cleanAddress(input)
  const bkk = isBangkok(a.province)
  const [sub, dist] = bkk ? ['แขวง', 'เขต'] : ['ตำบล', 'อำเภอ']
  return [
    prefixed('อาคาร', a.building),
    prefixed('ห้องเลขที่ ', a.room_no),
    prefixed('ชั้นที่ ', a.floor_no),
    prefixed('หมู่บ้าน', a.village),
    prefixed('เลขที่ ', a.house_no),
    prefixed('หมู่ที่ ', a.moo),
    prefixed('ซอย', a.soi),
    prefixed('แยก', a.yaek),
    prefixed('ถนน', a.street),
    prefixed(sub, a.sub_district.replace(/^(ตำบล|แขวง)/, '')),
    prefixed(dist, a.district.replace(/^(อำเภอ|เขต)/, '')),
    bkk ? 'กรุงเทพมหานคร' : prefixed('จังหวัด', a.province),
    a.postal_code,
  ]
    .filter(Boolean)
    .join(' ')
}

/** ผลค้นหาผู้จด VAT — GET /companies/{id}/vat-registrants?tax_id= */
export type VatRegistrant = {
  tax_id: string
  branch_no: string
  branch_name: string
  name: string
  tax_type: 'juristic' | 'individual'
  phone: string
  vat_registered_on: string
  address: string
  address_detail: ThaiAddress
}

export const vatRegistrantsApi = (companyId: string) => `/companies/${companyId}/vat-registrants`
