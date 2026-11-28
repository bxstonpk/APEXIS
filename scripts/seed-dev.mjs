#!/usr/bin/env node
// ข้อมูลตัวอย่างสำหรับทดสอบบน dev — บริษัทตัวอย่าง 4 ประเภทธุรกิจ พร้อมทะเบียนและเอกสารครบวงจร
//
//   node scripts/seed-dev.mjs            (backend ต้องรันอยู่ — ค่าเริ่มต้น http://localhost:18080)
//   API_URL=http://host:port node scripts/seed-dev.mjs
//
// - ล็อกอินด้วย INITIAL_ADMIN_USERNAME / INITIAL_ADMIN_PASSWORD จาก .env (ไม่พิมพ์ค่าออกหน้าจอ)
// - ทุกอย่างทำผ่าน API (กฎธุรกิจ / ลงบัญชี / สต็อกเหมือนใช้งานจริง)
// - รันซ้ำได้: บริษัทที่มีรหัสอยู่แล้วจะข้าม — ต้องการเริ่มใหม่ให้ล้างฐานข้อมูล dev แล้วรันใหม่
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const env = Object.fromEntries(
  readFileSync(join(root, '.env'), 'utf8')
    .split(/\r?\n/)
    .filter((l) => /^[A-Z_]+=/.test(l))
    .map((l) => [l.slice(0, l.indexOf('=')), l.slice(l.indexOf('=') + 1).replace(/^"|"$/g, '')]),
)
const API = `${process.env.API_URL || 'http://localhost:18080'}/api/v1`
let token = ''

async function req(method, path, body) {
  const r = await fetch(API + path, {
    method,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await r.text()
  const json = text ? JSON.parse(text) : {}
  if (!r.ok) throw new Error(`${method} ${path} → ${r.status} ${JSON.stringify(json.error ?? json).slice(0, 300)}`)
  return json.data ?? json
}
const list = (x) => (Array.isArray(x) ? x : (x.data ?? x.rows ?? []))

const login = await req('POST', '/auth/tokens', { email: env.INITIAL_ADMIN_USERNAME, password: env.INITIAL_ADMIN_PASSWORD })
token = login.access_token ?? login.token
if (!token) throw new Error('login: ไม่พบ access_token')

const addr = (house_no, street, sub_district, district, province, postal_code) => ({ house_no, street, sub_district, district, province, postal_code })
const BKK = addr('99/1', 'สีลม', 'สีลม', 'บางรัก', 'กรุงเทพมหานคร', '10500')
const NBI = addr('55', 'แจ้งวัฒนะ', 'ปากเกร็ด', 'ปากเกร็ด', 'นนทบุรี', '11120')
const SPK = addr('120', 'เทพารักษ์', 'บางพลีใหญ่', 'บางพลี', 'สมุทรปราการ', '10540')

const day = (d) => `2026-11-${String(d).padStart(2, '0')}`

// ---------- ส่วนร่วมของทุกบริษัท ----------
async function base(code, name, businessType, taxId, legal) {
  const mine = list(await req('GET', `/me/companies?q=${code}&page_size=50`))
  if (mine.some((c) => c.code === code)) {
    console.log(`- ${code} มีอยู่แล้ว ข้าม`)
    return null
  }
  const co = await req('POST', '/companies', { code, name, business_type: businessType, tax_id: taxId })
  const C = `/companies/${co.id}`
  await req('PUT', C, {
    name, tax_id: taxId, legal_name_th: legal, legal_name_en: '', business_type: businessType,
    address: { line: '88 อาคารเอเพ็กซ์ ชั้น 12 ถนนพระราม 9', sub_district: 'ห้วยขวาง', district: 'ห้วยขวาง', province: 'กรุงเทพมหานคร', postal_code: '10310', country: 'TH' },
    phone: '02-000-1234', email: 'contact@example.com', website: '', base_currency: 'THB', fiscal_year_start_month: 1,
    amount_decimals: 2, quantity_decimals: 2, status: 'active',
  })
  await req('POST', `${C}/fiscal-years`, { year: 2026, start_date: '2026-01-01' })
  const glCode = async (c) => list(await req('GET', `${C}/accounts?q=${c}&page_size=5`)).find((a) => a.code === c)
  const cash = await req('POST', `${C}/cash-accounts`, { code: 'CASH', name: 'เงินสดในมือ', type: 'cash', gl_account_id: (await glCode('1101010100')).id, status: 'active' })
  const bank = await req('POST', `${C}/cash-accounts`, {
    code: 'KBANK', name: 'กสิกรไทย ออมทรัพย์', type: 'bank', bank_name: 'กสิกรไทย', account_no: '123-4-56789-0',
    gl_account_id: (await glCode('1101030100')).id, status: 'active',
  })
  const wht = await req('POST', `${C}/withholding-tax-types`, { code: 'SVC3', name: 'ค่าบริการ 3%', rate: '3', income_type: '3_tres', status: 'active' })
  const uom = {}
  for (const [k, n] of [['EA', 'ชิ้น'], ['SET', 'ชุด'], ['HR', 'ชั่วโมง'], ['JOB', 'งาน'], ['BAG', 'ถุง']]) uom[k] = await req('POST', `${C}/uoms`, { code: k, name: n, status: 'active' })
  const wh = await req('POST', `${C}/warehouses`, { code: 'WH-MAIN', name: 'คลังหลัก', status: 'active' })
  const customers = [
    await req('POST', `${C}/customers`, { code: 'C001', name: 'บริษัท สยามรีเทล จำกัด', tax_id: '0105561000011', branch_no: '00000', tax_type: 'juristic', address_detail: BKK, payment_term_days: 30, credit_limit: '500000', status: 'active' }),
    await req('POST', `${C}/customers`, { code: 'C002', name: 'บริษัท นนทบุรีเทรดดิ้ง จำกัด', tax_id: '0125562000022', branch_no: '00000', tax_type: 'juristic', address_detail: NBI, payment_term_days: 15, status: 'active' }),
  ]
  const suppliers = [
    await req('POST', `${C}/suppliers`, { code: 'S001', name: 'บริษัท ซัพพลายเออร์ไทย จำกัด', tax_id: '0115563000033', branch_no: '00000', tax_type: 'juristic', vat_registered: true, address_detail: SPK, payment_term_days: 30, status: 'active' }),
    await req('POST', `${C}/suppliers`, { code: 'S002', name: 'นายสมชาย บริการดี', tax_id: '3100500000004', branch_no: '00000', tax_type: 'individual', vat_registered: false, address_detail: NBI, payment_term_days: 0, status: 'active' }),
  ]
  const item = (code, name, type, u) => req('POST', `${C}/items`, { code, name, type, uom_id: uom[u].id, status: 'active' })
  return { co, C, cash, bank, wht, uom, wh, customers, suppliers, item }
}

// ซื้อเข้า: ใบสั่งซื้อ → อนุมัติ → รับสินค้า → ใบแจ้งหนี้ผู้ขาย → จ่ายเงิน (การเงิน)
async function purchase(b, supplier, date, lines, { pay = true, invNo } = {}) {
  const po = await req('POST', `${b.C}/purchase-orders`, { supplier_id: supplier.id, date, delivery_date: date, price_includes_vat: false, lines })
  await req('PATCH', `${b.C}/purchase-orders/${po.id}`, { status: 'approved' })
  const ol = lines.map((l, i) => ({ order_line_no: i + 1, quantity: l.quantity }))
  await req('POST', `${b.C}/goods-receipts`, { order_id: po.id, date, warehouse_id: b.wh.id, lines: ol })
  const inv = await req('POST', `${b.C}/supplier-invoices`, { order_id: po.id, supplier_invoice_no: invNo, date, lines: ol })
  if (pay) {
    await req('POST', `${b.C}/payments`, {
      kind: 'supplier', supplier_id: supplier.id, date, lines: [{ invoice_id: inv.id, amount: inv.totals.gross }],
      methods: [{ cash_account_id: b.bank.id, amount: inv.totals.gross, reference: 'โอนผ่านธนาคาร' }],
    })
  }
  return inv
}

// จ้างบริการ (ทุกประเภทธุรกิจ): ใบสั่งจ้าง → ตรวจรับงาน → ซื้อบริการ (ยังไม่ได้ใบกำกับ = ภาษีซื้อยังไม่ถึงกำหนด)
// → จ่ายเงิน (การเงิน หัก ณ ที่จ่าย 3%) → บันทึกใบกำกับภาษีที่ได้รับ; + ซื้อบริการตรงที่ยังรอใบกำกับ 1 ใบ และที่ได้ใบกำกับพร้อมใบแจ้งหนี้ 1 ใบ
async function servicePurchases(b) {
  const clean = await b.item('PS-CLEAN', 'ค่าบริการทำความสะอาดสำนักงาน', 'service', 'JOB')
  const air = await b.item('PS-AIR', 'ค่าบำรุงรักษาเครื่องปรับอากาศ', 'service', 'JOB')
  const s = b.suppliers[0]
  const wo = await req('POST', `${b.C}/work-orders`, { supplier_id: s.id, date: day(5), delivery_date: day(10), price_includes_vat: false, description: 'จ้างทำความสะอาดสำนักงานเดือนพฤศจิกายน', lines: [{ item_id: clean.id, quantity: '1', unit_price: '20000' }] })
  await req('PATCH', `${b.C}/work-orders/${wo.id}`, { status: 'approved' })
  await req('POST', `${b.C}/service-acceptances`, { order_id: wo.id, date: day(10), lines: [{ order_line_no: 1, quantity: '1' }] })
  const si = await req('POST', `${b.C}/supplier-service-invoices`, { order_id: wo.id, supplier_invoice_no: 'SV-2611-001', invoice_date: day(10), date: day(10), lines: [{ order_line_no: 1, quantity: '1' }] })
  const wht = (Number(si.totals.net) * 0.03).toFixed(2)
  await req('POST', `${b.C}/payments`, {
    kind: 'supplier', supplier_id: s.id, date: day(18), lines: [{ invoice_id: si.id, amount: si.totals.gross, wht_type_id: b.wht.id }],
    methods: [{ cash_account_id: b.bank.id, amount: (Number(si.totals.gross) - Number(wht)).toFixed(2), reference: 'โอนผ่านธนาคาร' }],
  })
  await req('POST', `${b.C}/supplier-service-invoices/${si.id}/tax-invoices`, { tax_invoice_no: 'TX-2611-001', tax_invoice_date: day(18), date: day(18) })
  // ซื้อบริการตรง ยังไม่ได้ใบกำกับ (ภาษีซื้อยังไม่ถึงกำหนด)
  await req('POST', `${b.C}/supplier-service-invoices`, { supplier_id: s.id, supplier_invoice_no: 'SV-2611-002', invoice_date: day(24), date: day(24), price_includes_vat: false, lines: [{ item_id: air.id, quantity: '4', unit_price: '2000' }] })
  // ซื้อบริการตรง ได้ใบกำกับพร้อมใบแจ้งหนี้ (ภาษีซื้อเกิดทันที)
  await req('POST', `${b.C}/supplier-service-invoices`, { supplier_id: s.id, supplier_invoice_no: 'SV-2611-003', invoice_date: day(26), date: day(26), tax_invoice_no: 'TX-2611-003', tax_invoice_date: day(26), price_includes_vat: false, lines: [{ item_id: air.id, quantity: '2', unit_price: '2000' }] })
}

// รับเงิน (การเงิน) — amount รวม wht แล้ว
const receive = (b, customer, date, lines, cashAccount = b.bank) => {
  const net = lines.reduce((s, l) => s + Number(l.amount) - Number(l.wht ?? 0), 0).toFixed(2)
  return req('POST', `${b.C}/receipts`, { kind: 'customer', customer_id: customer.id, date, lines, methods: [{ cash_account_id: cashAccount.id, amount: net, reference: '' }] })
}

// ---------- ธุรกิจซื้อขายสินค้า ----------
async function trading() {
  const b = await base('DEMO-TRD', 'เดโม ซื้อขายสินค้า', 'trading', '0105566000101', 'บริษัท เดโม เทรดดิ้ง จำกัด')
  if (!b) return
  const [c1, c2] = b.customers
  const nb = await b.item('NB-14', 'โน้ตบุ๊ก 14 นิ้ว', 'stock', 'EA')
  const ms = await b.item('MS-01', 'เมาส์ไร้สาย', 'stock', 'EA')
  const mn = await b.item('MN-27', 'จอมอนิเตอร์ 27 นิ้ว', 'stock', 'EA')
  const inst = await b.item('SV-INST', 'ค่าติดตั้งและตั้งค่าเครื่อง', 'service', 'JOB')
  await purchase(b, b.suppliers[0], day(2), [
    { item_id: nb.id, quantity: '20', unit_price: '18000' },
    { item_id: ms.id, quantity: '100', unit_price: '250' },
    { item_id: mn.id, quantity: '15', unit_price: '5500' },
  ], { invNo: 'SUP-2611-001' })

  const qt = await req('POST', `${b.C}/quotations`, { customer_id: c1.id, date: day(5), valid_until: day(30), price_includes_vat: false, lines: [{ item_id: nb.id, quantity: '5', unit_price: '24900' }, { item_id: ms.id, quantity: '5', unit_price: '490' }, { item_id: inst.id, quantity: '5', unit_price: '500' }] })
  await req('PATCH', `${b.C}/quotations/${qt.id}`, { status: 'sent' })
  await req('PATCH', `${b.C}/quotations/${qt.id}`, { status: 'accepted' })
  const so = await req('POST', `${b.C}/sales-orders`, { quotation_id: qt.id, date: day(6) })
  await req('PATCH', `${b.C}/sales-orders/${so.id}`, { status: 'approved' })
  const dp = await req('POST', `${b.C}/sales-deposits`, { customer_id: c1.id, order_id: so.id, date: day(6), description: 'มัดจำ 20% ตามใบสั่งขาย', amount: '25000', price_includes_vat: false })
  await receive(b, c1, day(6), [{ invoice_id: dp.id, amount: dp.totals.gross }])
  const iv1 = await req('POST', `${b.C}/sales-invoices`, { order_id: so.id, date: day(10), warehouse_id: b.wh.id, lines: [{ order_line_no: 1, quantity: '5' }, { order_line_no: 2, quantity: '5' }, { order_line_no: 3, quantity: '5' }], deposits: [{ deposit_id: dp.id, amount: '25000' }] })
  await receive(b, c1, day(20), [{ invoice_id: iv1.id, amount: iv1.outstanding }])
  const iv2 = await req('POST', `${b.C}/sales-invoices`, { customer_id: c2.id, date: day(15), warehouse_id: b.wh.id, price_includes_vat: false, lines: [{ item_id: mn.id, quantity: '4', unit_price: '7900' }, { item_id: ms.id, quantity: '10', unit_price: '490' }] })
  const iv3 = await req('POST', `${b.C}/sales-invoices`, { customer_id: c2.id, date: day(22), warehouse_id: b.wh.id, price_includes_vat: false, lines: [{ item_id: nb.id, quantity: '2', unit_price: '24900' }] })
  await req('POST', `${b.C}/billing-notes`, { customer_id: c2.id, date: day(25), due_date: '2026-12-10', invoice_ids: [iv2.id, iv3.id], description: 'วางบิลประจำเดือนพฤศจิกายน' })
  await req('POST', `${b.C}/sales-credit-notes`, { invoice_id: iv2.id, date: day(24), kind: 'return', reason: 'เมาส์ชำรุด 1 ชิ้น', warehouse_id: b.wh.id, lines: [{ invoice_line_no: 2, quantity: '1' }] })
  await req('POST', `${b.C}/quotations`, { customer_id: c2.id, date: day(26), valid_until: '2026-12-26', price_includes_vat: false, lines: [{ item_id: mn.id, quantity: '10', unit_price: '7500' }] })
  await servicePurchases(b)
  console.log('✓ DEMO-TRD ธุรกิจซื้อขายสินค้า')
}

// ---------- ธุรกิจบริการทั่วไป ----------
async function service() {
  const b = await base('DEMO-SVC', 'เดโม บริการ', 'service', '0105566000202', 'บริษัท เดโม เซอร์วิส จำกัด')
  if (!b) return
  const [c1, c2] = b.customers
  const cons = await b.item('CS-HR', 'ที่ปรึกษาระบบบัญชี (รายชั่วโมง)', 'service', 'HR')
  const ma = await b.item('MA-MONTH', 'บริการดูแลระบบรายเดือน', 'service', 'JOB')
  const impl = await b.item('IMPL', 'ค่าติดตั้งและฝึกอบรมระบบ', 'service', 'JOB')
  // ใบแจ้งหนี้ 1: รับครบ (ลูกค้าหัก ณ ที่จ่าย 3%) → ภาษีขายเกิดทั้งใบ
  const sv1 = await req('POST', `${b.C}/service-invoices`, { customer_id: c1.id, date: day(3), price_includes_vat: false, lines: [{ item_id: impl.id, quantity: '1', unit_price: '50000' }] })
  await receive(b, c1, day(12), [{ invoice_id: sv1.id, amount: sv1.totals.gross, wht: '1500' }])
  // ใบแจ้งหนี้ 2: รับบางส่วน → ภาษีเกิดตามสัดส่วน ที่เหลือยังไม่ถึงกำหนด
  const sv2 = await req('POST', `${b.C}/service-invoices`, { customer_id: c1.id, date: day(10), price_includes_vat: false, lines: [{ item_id: cons.id, quantity: '40', unit_price: '1500' }] })
  await receive(b, c1, day(25), [{ invoice_id: sv2.id, amount: '32100', wht: '900' }])
  // ใบแจ้งหนี้ 3: ลูกค้าขอใบกำกับภาษีก่อนจ่าย
  const sv3 = await req('POST', `${b.C}/service-invoices`, { customer_id: c2.id, date: day(15), price_includes_vat: false, lines: [{ item_id: ma.id, quantity: '1', unit_price: '12000' }] })
  await req('POST', `${b.C}/service-invoices/${sv3.id}/tax-invoice`, { date: day(16) })
  // ใบแจ้งหนี้ 4 + ใบวางบิล: ยังไม่รับเงิน
  const sv4 = await req('POST', `${b.C}/service-invoices`, { customer_id: c2.id, date: day(25), price_includes_vat: false, lines: [{ item_id: ma.id, quantity: '1', unit_price: '12000' }, { item_id: cons.id, quantity: '6', unit_price: '1500' }] })
  await req('POST', `${b.C}/billing-notes`, { customer_id: c2.id, date: day(26), due_date: '2026-12-15', invoice_ids: [sv3.id, sv4.id], description: 'วางบิลค่าบริการเดือนพฤศจิกายน' })
  // มัดจำโครงการใหม่ (ยังไม่รับเงิน — รอการเงินยืนยัน)
  await req('POST', `${b.C}/sales-deposits`, { customer_id: c1.id, date: day(27), description: 'มัดจำโครงการ Phase 2', amount: '30000', price_includes_vat: false })
  // ค่าใช้จ่าย: จ้างผู้รับจ้างบุคคล หัก ณ ที่จ่าย 3%
  const exp = list(await req('GET', `${b.C}/accounts?q=5103000000&page_size=5`)).find((a) => a.code === '5103000000')
  await req('POST', `${b.C}/payments`, {
    kind: 'other', date: day(20), payee: { name: b.suppliers[1].name, tax_id: '3100500000004', tax_type: 'individual' },
    lines: [{ account_id: exp.id, description: 'ค่าจ้างผู้รับเหมาช่วงงานติดตั้ง', amount: '10000', wht_type_id: b.wht.id }],
    methods: [{ cash_account_id: b.bank.id, amount: '9700', reference: '' }],
  })
  await servicePurchases(b)
  console.log('✓ DEMO-SVC ธุรกิจบริการทั่วไป')
}

// ---------- ธุรกิจผลิตสินค้า ----------
async function manufacturing() {
  const b = await base('DEMO-MFG', 'เดโม ผลิตเฟอร์นิเจอร์', 'manufacturing', '0105566000303', 'บริษัท เดโม เฟอร์นิเจอร์ จำกัด')
  if (!b) return
  const [c1, c2] = b.customers
  const wood = await b.item('RM-WOOD', 'ไม้ยางพาราแปรรูป', 'stock', 'EA')
  const screw = await b.item('RM-SCREW', 'สกรูเกลียวปล่อย', 'stock', 'EA')
  const paint = await b.item('RM-PAINT', 'สีเคลือบเงา', 'stock', 'EA')
  const chair = await b.item('FG-CHAIR', 'เก้าอี้ไม้ยางพารา', 'stock', 'EA')
  const table = await b.item('FG-TABLE', 'โต๊ะทำงานไม้ยางพารา', 'stock', 'EA')
  await purchase(b, b.suppliers[0], day(2), [
    { item_id: wood.id, quantity: '500', unit_price: '120' },
    { item_id: screw.id, quantity: '5000', unit_price: '1.5' },
    { item_id: paint.id, quantity: '60', unit_price: '350' },
  ], { invNo: 'RM-2611-01' })
  const acct = list(await req('GET', `${b.C}/accounts?q=5065000000&page_size=5`)).find((a) => a.code === '5065000000')
  const bomChair = await req('POST', `${b.C}/boms`, {
    code: 'BOM-CHAIR', name: 'สูตรเก้าอี้ไม้', item_id: chair.id, output_quantity: '1', status: 'active', is_default: true,
    components: [{ item_id: wood.id, quantity: '4', scrap_percent: '5' }, { item_id: screw.id, quantity: '16', scrap_percent: '0' }, { item_id: paint.id, quantity: '0.2', scrap_percent: '0' }],
    conversion_costs: [{ description: 'ค่าแรงประกอบ', account_id: acct.id, amount: '150' }, { description: 'โสหุ้ยการผลิต', account_id: acct.id, amount: '40' }],
  })
  const bomTable = await req('POST', `${b.C}/boms`, {
    code: 'BOM-TABLE', name: 'สูตรโต๊ะทำงาน', item_id: table.id, output_quantity: '1', status: 'active', is_default: true,
    components: [{ item_id: wood.id, quantity: '10', scrap_percent: '5' }, { item_id: screw.id, quantity: '30', scrap_percent: '0' }, { item_id: paint.id, quantity: '0.5', scrap_percent: '0' }],
    conversion_costs: [{ description: 'ค่าแรงประกอบ', account_id: acct.id, amount: '350' }],
  })
  // ใบสั่งผลิต 1: ผลิตเสร็จ
  const mo1 = await req('POST', `${b.C}/production-orders`, { date: day(5), item_id: chair.id, bom_id: bomChair.id, quantity: '40', warehouse_id: b.wh.id, planned_start: day(5), planned_end: day(9), description: 'ล็อตเก้าอี้ส่งลูกค้า C001' })
  await req('POST', `${b.C}/production-orders/${mo1.id}/release`, {})
  const full1 = await req('GET', `${b.C}/production-orders/${mo1.id}`)
  await req('POST', `${b.C}/production-orders/${mo1.id}/issue`, { date: day(6), lines: full1.materials.map((m) => ({ item_id: m.item.id, quantity: m.required_quantity })) })
  await req('POST', `${b.C}/production-orders/${mo1.id}/complete`, { date: day(9), completed_quantity: '40' })
  // ใบสั่งผลิต 2: กำลังผลิต (เบิกวัตถุดิบบางส่วน)
  const mo2 = await req('POST', `${b.C}/production-orders`, { date: day(20), item_id: table.id, bom_id: bomTable.id, quantity: '10', warehouse_id: b.wh.id, planned_start: day(21), planned_end: day(30), description: 'โต๊ะทำงานสต็อก' })
  await req('POST', `${b.C}/production-orders/${mo2.id}/release`, {})
  const full2 = await req('GET', `${b.C}/production-orders/${mo2.id}`)
  await req('POST', `${b.C}/production-orders/${mo2.id}/issue`, { date: day(21), lines: [{ item_id: full2.materials[0].item.id, quantity: '60' }] })
  // ใบสั่งผลิต 3: ร่าง
  await req('POST', `${b.C}/production-orders`, { date: day(26), item_id: chair.id, bom_id: bomChair.id, quantity: '20', warehouse_id: b.wh.id, description: 'แผนผลิตเดือนธันวาคม' })
  // ขายสินค้าสำเร็จรูป
  const iv = await req('POST', `${b.C}/sales-invoices`, { customer_id: c1.id, date: day(12), warehouse_id: b.wh.id, price_includes_vat: false, lines: [{ item_id: chair.id, quantity: '30', unit_price: '1890' }] })
  await receive(b, c1, day(26), [{ invoice_id: iv.id, amount: iv.totals.gross }])
  await req('POST', `${b.C}/sales-invoices`, { customer_id: c2.id, date: day(24), warehouse_id: b.wh.id, price_includes_vat: false, lines: [{ item_id: chair.id, quantity: '8', unit_price: '1950' }] })
  await servicePurchases(b)
  console.log('✓ DEMO-MFG ธุรกิจผลิตสินค้า')
}

// ---------- ธุรกิจรับเหมาก่อสร้าง ----------
async function construction() {
  const b = await base('DEMO-CON', 'เดโม รับเหมาก่อสร้าง', 'construction', '0105566000404', 'บริษัท เดโม คอนสตรัคชั่น จำกัด')
  if (!b) return
  const [c1] = b.customers
  const cement = await b.item('MT-CEMENT', 'ปูนซีเมนต์ปอร์ตแลนด์ 50 กก.', 'stock', 'BAG')
  const steel = await b.item('MT-STEEL', 'เหล็กเส้น DB16', 'stock', 'EA')
  const prog = await b.item('CON-PROG', 'ค่างานก่อสร้างตามงวดงาน', 'service', 'JOB')
  const extra = await b.item('CON-VO', 'ค่างานเพิ่ม (Variation Order)', 'service', 'JOB')
  await purchase(b, b.suppliers[0], day(3), [{ item_id: cement.id, quantity: '400', unit_price: '145' }, { item_id: steel.id, quantity: '300', unit_price: '420' }], { invNo: 'MAT-2611-01' })
  // เบิกวัสดุเข้าหน้างาน
  await req('POST', `${b.C}/stock-documents`, { type: 'issue', date: day(8), warehouse_id: b.wh.id, status: 'posted', reference: 'โครงการอาคารสำนักงาน 3 ชั้น', lines: [{ item_id: cement.id, quantity: '250' }, { item_id: steel.id, quantity: '180' }] })
  // มัดจำ / เงินรับล่วงหน้าตามสัญญา 10%
  const dp = await req('POST', `${b.C}/sales-deposits`, { customer_id: c1.id, date: day(2), description: 'เงินรับล่วงหน้า 10% สัญญาก่อสร้างอาคาร 3 ชั้น', amount: '300000', price_includes_vat: false })
  await receive(b, c1, day(4), [{ invoice_id: dp.id, amount: dp.totals.gross, wht: '9000' }])
  // งวดงานที่ 1: หักเงินรับล่วงหน้าตามสัดส่วน รับเงินแล้ว (หัก ณ ที่จ่าย 3%)
  const sv1 = await req('POST', `${b.C}/service-invoices`, { customer_id: c1.id, date: day(15), price_includes_vat: false, description: 'เบิกงวดงานที่ 1 — งานฐานรากและเสาชั้น 1', lines: [{ item_id: prog.id, quantity: '1', unit_price: '900000' }], deposits: [{ deposit_id: dp.id, amount: '90000' }] })
  await receive(b, c1, day(28), [{ invoice_id: sv1.id, amount: sv1.outstanding, wht: '24300' }])
  // งวดงานที่ 2 + งานเพิ่ม: ยังไม่รับเงิน (ภาษียังไม่ถึงกำหนด)
  const sv2 = await req('POST', `${b.C}/service-invoices`, { customer_id: c1.id, date: day(28), price_includes_vat: false, description: 'เบิกงวดงานที่ 2 — งานโครงสร้างชั้น 2', lines: [{ item_id: prog.id, quantity: '1', unit_price: '750000' }, { item_id: extra.id, quantity: '1', unit_price: '45000' }], deposits: [{ deposit_id: dp.id, amount: '75000' }] })
  await req('POST', `${b.C}/billing-notes`, { customer_id: c1.id, date: day(29), due_date: '2026-12-29', invoice_ids: [sv2.id], description: 'วางบิลงวดงานที่ 2' })
  await servicePurchases(b)
  console.log('✓ DEMO-CON ธุรกิจรับเหมาก่อสร้าง')
}

// ---------- เงินสดย่อย (ทุกบริษัทตัวอย่าง — เพิ่มให้บริษัทที่มีอยู่แล้วได้ ข้ามถ้ามีกองทุน PC-HO แล้ว) ----------
// ตั้งกองทุน → ขอตั้งวงเงิน → การเงินจ่าย → ใบสำคัญจ่าย 3 ใบ (มีใบกำกับภาษี 1 ใบ) → ขอเบิกชดเชย → การเงินจ่าย
// → ใบสำคัญจ่ายที่อนุมัติแล้วแต่ยังไม่ได้ขอเบิกอีก 1 ใบ
async function pettyCash(code) {
  const co = list(await req('GET', `/me/companies?q=${code}&page_size=50`)).find((c) => c.code === code)
  if (!co) return
  const C = `/companies/${co.id}`
  if (list(await req('GET', `${C}/petty-cash-funds?q=PC-HO&page_size=5`)).some((f) => f.code === 'PC-HO')) {
    console.log(`- ${code} เงินสดย่อยมีอยู่แล้ว ข้าม`)
    return
  }
  const me = await req('GET', '/me')
  const gl = async (c) => list(await req('GET', `${C}/accounts?q=${c}&page_size=5`)).find((a) => a.code === c)
  const bank = list(await req('GET', `${C}/cash-accounts?q=KBANK&page_size=5`)).find((a) => a.code === 'KBANK')
  const [postage, travel, supplies] = [await gl('5309080000'), await gl('5308020000'), await gl('5309050000')]
  const fund = await req('POST', `${C}/petty-cash-funds`, { code: 'PC-HO', name: 'เงินสดย่อยสำนักงานใหญ่', custodian_user_id: me.id, imprest_amount: '20000', status: 'active' })
  const payRequest = (r, date) =>
    req('POST', `${C}/payments`, { kind: 'petty_cash', petty_cash_request_id: r.id, date, methods: [{ cash_account_id: bank.id, amount: r.amount, reference: 'โอนเข้าบัญชีผู้ถือเงินสดย่อย' }] })
  const est = await req('POST', `${C}/petty-cash-replenishments`, { fund_id: fund.id, kind: 'establish', date: day(1), description: 'ตั้งวงเงินสดย่อยสำนักงานใหญ่' })
  await payRequest(est, day(2))
  const voucher = async (date, payee, description, lines, vat) => {
    const v = await req('POST', `${C}/petty-cash-vouchers`, {
      fund_id: fund.id, date, payee, description, lines,
      vat_rate: vat ? '7' : '0', price_includes_vat: true, tax_invoice: vat ?? null,
    })
    await req('POST', `${C}/petty-cash-vouchers/${v.id}/approve`, {})
    return v
  }
  await voucher(day(4), { name: 'ไปรษณีย์ไทย สาขาห้วยขวาง' }, 'ส่งเอกสารวางบิลลูกค้า', [{ account_id: postage.id, description: 'ค่าส่ง EMS 6 ฉบับ', amount: '384' }])
  await voucher(day(7), { name: 'นายวิชัย ขับดี' }, 'ค่าแท็กซี่ไปพบลูกค้า', [{ account_id: travel.id, description: 'แท็กซี่ ไป-กลับ สีลม', amount: '420' }])
  await voucher(day(10), { name: 'บริษัท ออฟฟิศเมท จำกัด', tax_id: '0105560000555', branch_no: '00000' }, 'วัสดุสำนักงานประจำเดือน', [
    { account_id: supplies.id, description: 'กระดาษ A4 5 รีม', amount: '650' },
    { account_id: supplies.id, description: 'หมึกพิมพ์', amount: '1490' },
  ], { no: 'IV6611-0042', date: day(10) })
  const rep = await req('POST', `${C}/petty-cash-replenishments`, { fund_id: fund.id, kind: 'replenish', date: day(14), description: 'เบิกชดเชยรอบครึ่งเดือนแรก', voucher_ids: list(await req('GET', `${C}/petty-cash-vouchers?fund_id=${fund.id}&unrequested=true&page_size=50`)).map((v) => v.id) })
  await payRequest(rep, day(15))
  await voucher(day(18), { name: 'ร้านเครื่องเขียนสยาม' }, 'วัสดุสิ้นเปลืองเพิ่มเติม', [{ account_id: supplies.id, description: 'แฟ้มและปากกา', amount: '275' }])
  console.log(`✓ ${code} เงินสดย่อย`)
}

for (const fn of [trading, service, manufacturing, construction]) {
  try {
    await fn()
  } catch (e) {
    console.error(`✗ ${fn.name}: ${e.message}`)
    process.exitCode = 1
  }
}

for (const code of ['DEMO-TRD', 'DEMO-SVC', 'DEMO-MFG', 'DEMO-CON']) {
  try {
    await pettyCash(code)
  } catch (e) {
    console.error(`✗ pettyCash ${code}: ${e.message}`)
    process.exitCode = 1
  }
}
