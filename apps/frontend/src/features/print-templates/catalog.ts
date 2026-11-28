import { pp30Data, vatReportData, withholdingData } from '@/features/finance/TaxForms'
import type { Payment } from '@/features/finance/types'
import { whtCertificateData } from '@/features/finance/WhtCertificate'
import type { Letterhead } from '@/features/sales/types'
import { familyOf, type PrintFamily } from './builtin'
import { documentPrintData } from './data'
import type { TemplateDocType } from './types'

/**
 * ตัวแปรของแม่แบบแต่ละกลุ่มข้อมูล (หน้าแก้แม่แบบแสดงให้คลิกแทรก) + ข้อมูลตัวอย่างสำหรับดูตัวอย่าง
 * path = ชื่อตัวแปรใน Handlebars; รายการ (array) ใช้ {{#each path}} ... {{/each}}
 */
export type Variable = { path: string; label: string; each?: boolean }

const COMPANY: Variable[] = [
  { path: 'company.legal_name_th', label: 'ชื่อนิติบุคคล (ไทย)' },
  { path: 'company.legal_name_en', label: 'ชื่อนิติบุคคล (อังกฤษ)' },
  { path: 'company.tax_id', label: 'เลขประจำตัวผู้เสียภาษีบริษัท' },
  { path: 'company.branch_label', label: 'สาขาบริษัท' },
  { path: 'company.address', label: 'ที่อยู่บริษัท' },
  { path: 'company.phone', label: 'โทรศัพท์' },
  { path: 'company.email', label: 'อีเมล' },
  { path: 'company.contact', label: 'ติดต่อ (โทร / อีเมล / เว็บ)' },
]

export const VARIABLES: Record<PrintFamily, Variable[]> = {
  document: [
    ...COMPANY,
    { path: 'title', label: 'ชื่อเอกสาร' },
    { path: 'copy_label', label: 'ต้นฉบับ / สำเนา' },
    { path: 'document_no', label: 'เลขที่เอกสาร' },
    { path: 'info', label: 'ข้อมูลเอกสาร (label, value)', each: true },
    { path: 'party.label', label: 'หัวข้อคู่ค้า' },
    { path: 'party.name', label: 'ชื่อคู่ค้า' },
    { path: 'party.tax_id', label: 'เลขผู้เสียภาษีคู่ค้า' },
    { path: 'party.branch_label', label: 'สาขาคู่ค้า' },
    { path: 'party.address', label: 'ที่อยู่คู่ค้า' },
    { path: 'columns', label: 'หัวตาราง (label, align, width)', each: true },
    { path: 'lines', label: 'รายการ (no, description, detail, quantity, unit_price, amount, cells)', each: true },
    { path: 'totals', label: 'ยอดรวม (label, value, raw, strong)', each: true },
    { path: 'amount', label: 'ยอดรวมทั้งสิ้น (ตัวเลข)' },
    { path: 'amount_in_words', label: 'ยอดรวมเป็นตัวอักษร' },
    { path: 'note', label: 'หมายเหตุ' },
    { path: 'signatures', label: 'ช่องลงนาม (label)', each: true },
    { path: 'labels.tax_id', label: 'ป้าย "เลขประจำตัวผู้เสียภาษี"' },
  ],
  wht_certificate: [
    ...COMPANY,
    { path: 'copies', label: 'ฉบับ (label)', each: true },
    { path: 'number', label: 'เลขที่หนังสือรับรอง' },
    { path: 'book', label: 'เล่มที่' },
    { path: 'date', label: 'วันที่จ่าย' },
    { path: 'payer.name', label: 'ผู้หักภาษี' },
    { path: 'payee.name', label: 'ผู้ถูกหักภาษี' },
    { path: 'payee.tax_id', label: 'เลขผู้เสียภาษีผู้ถูกหัก' },
    { path: 'payee.address', label: 'ที่อยู่ผู้ถูกหัก' },
    { path: 'sequence', label: 'ลำดับที่ในแบบ' },
    { path: 'forms', label: 'แบบ ภ.ง.ด. (label, checked)', each: true },
    { path: 'incomes', label: 'ประเภทเงินได้ (label, names, has, amount, tax)', each: true },
    { path: 'total_amount', label: 'รวมเงินที่จ่าย' },
    { path: 'total_tax', label: 'รวมภาษีที่หัก' },
    { path: 'modes', label: 'ผู้จ่ายเงิน (label, checked)', each: true },
    { path: 'issued_on', label: 'วันที่ออกหนังสือรับรอง' },
  ],
  withholding: [
    ...COMPANY,
    { path: 'title', label: 'ชื่อแบบ' },
    { path: 'period', label: 'เดือนภาษี' },
    { path: 'payer.name', label: 'ผู้มีหน้าที่หักภาษี' },
    { path: 'pages', label: 'แผ่น (page_label, rows, page_base, page_wht, last)', each: true },
    { path: 'summary.payees', label: 'จำนวนราย' },
    { path: 'summary.base', label: 'รวมเงินที่จ่าย' },
    { path: 'summary.wht', label: 'รวมภาษีที่หัก' },
  ],
  pp30: [
    ...COMPANY,
    { path: 'title', label: 'ชื่อแบบ' },
    { path: 'period', label: 'เดือนภาษี' },
    { path: 'rows', label: 'บรรทัด (no, label, value, strong)', each: true },
    { path: 'payable', label: 'ภาษีที่ต้องชำระ' },
    { path: 'overpaid', label: 'ภาษีที่ชำระเกิน' },
    { path: 'net_text', label: 'สรุปยอดชำระ' },
  ],
  vat_report: [
    ...COMPANY,
    { path: 'title', label: 'ชื่อรายงาน' },
    { path: 'period', label: 'เดือนภาษี' },
    { path: 'pages', label: 'แผ่น (page_label, rows, page_base, page_tax, last)', each: true },
    { path: 'base', label: 'รวมมูลค่า' },
    { path: 'tax', label: 'รวมภาษี' },
  ],
}

/** ตัวช่วยที่ใช้ได้ในแม่แบบ */
export const HELPERS: { snippet: string; label: string }[] = [
  { snippet: '{{money amount}}', label: 'จำนวนเงิน 1,234.50' },
  { snippet: '{{qty quantity}}', label: 'จำนวน' },
  { snippet: '{{date date}}', label: 'วันที่ 26 พ.ย. 2569' },
  { snippet: '{{bahtText amount}}', label: 'จำนวนเงินเป็นตัวอักษร' },
  { snippet: '{{inc @index}}', label: 'ลำดับเริ่มที่ 1' },
  { snippet: '{{#if (eq a b)}}…{{/if}}', label: 'เปรียบเทียบ' },
  { snippet: '{{asset "logo"}}', label: 'URL ของรูปชื่อ logo' },
]

export function variableSnippet(v: Variable): string {
  return v.each ? `{{#each ${v.path}}}\n  \n{{/each}}` : `{{${v.path}}}`
}

// ---------- ข้อมูลตัวอย่าง ----------

const SAMPLE_LETTERHEAD: Letterhead = {
  name: 'บริษัท ตัวอย่าง จำกัด',
  legal_name_th: 'บริษัท ตัวอย่าง จำกัด',
  legal_name_en: 'Example Co., Ltd.',
  tax_id: '0105566000000',
  address: { line: '88 ถนนพระราม 9', sub_district: 'ห้วยขวาง', district: 'ห้วยขวาง', province: 'กรุงเทพมหานคร', postal_code: '10310', country: 'TH' },
  phone: '02-000-0000',
  email: 'contact@example.com',
  website: '',
} as Letterhead

const SAMPLE_PAYMENT = {
  id: 'p',
  document_no: 'PV2026-000001',
  date: '2026-11-26',
  kind: 'supplier',
  party: { id: null, code: '', name: 'บริษัท ผู้รับเงิน จำกัด', tax_id: '0105559000001', branch_no: '00000', address: 'กรุงเทพมหานคร', tax_type: 'juristic' },
  amount: '10000',
  wht: '300',
  net: '9700',
  status: 'posted',
  entry_id: null,
  entry_no: '',
  cancelled_at: null,
  description: '',
  created_by: '',
  methods: [],
  wht_certificate_no: 'WT2026-000001',
  wht_form_seq: 1,
  lines: [{ invoice: null, account_id: 'a', description: '', amount: '10000', wht_type: { id: 'w', code: 'SVC', name: 'ค่าบริการ', income_type: '3_tres' }, wht_rate: '3', wht_base: '10000', wht: '300' }],
} as unknown as Payment

export function sampleData(docType: TemplateDocType): Record<string, unknown> {
  switch (familyOf(docType)) {
    case 'wht_certificate':
      return whtCertificateData(SAMPLE_LETTERHEAD, SAMPLE_PAYMENT)
    case 'withholding':
      return withholdingData(SAMPLE_LETTERHEAD, {
        form: docType === 'finance.pnd3' ? 'pnd3' : 'pnd53',
        month: '2026-11',
        summary: { payees: 1, lines: 1, base: '10000', wht: '300' },
        lines: [{ seq: 1, payment_id: 'p', document_no: 'PV2026-000001', certificate_no: 'WT2026-000001', date: '2026-11-26', tax_id: '0105559000001', branch_no: '00000', name: 'บริษัท ผู้รับเงิน จำกัด', address: 'กรุงเทพมหานคร', income_type: '3_tres', wht_type_name: 'ค่าบริการ', rate: '3', base: '10000', wht: '300', condition: 1 }],
        filing: null,
      } as never)
    case 'pp30':
      return pp30Data(SAMPLE_LETTERHEAD, {
        month: '2026-11', sales: { base: '100000', tax: '7000', documents: 3 }, zero_rated: '0', exempt: '0', purchases: { base: '40000', tax: '2800', documents: 2 },
        carry_in: '0', net: '4200', payable: '4200', overpaid: '0', prev_filed: false, filing: null,
      } as never)
    case 'vat_report':
      return vatReportData(SAMPLE_LETTERHEAD, {
        kind: docType === 'finance.vat_report_input' ? 'input' : 'output', month: '2026-11',
        rows: [{ seq: 1, date: '2026-11-10', document_no: 'IV2026-000001', party_name: 'บริษัท ลูกค้า จำกัด', tax_id: '0105561000011', branch_no: '00000', base: '10000', tax: '700' }],
        base: '10000', tax: '700',
      } as never)
    default:
      return documentPrintData({
        letterhead: SAMPLE_LETTERHEAD,
        title: 'ใบส่งของ / ใบแจ้งหนี้ / ใบกำกับภาษี',
        docNo: 'IV2026-000001',
        info: [['วันที่', '26 พ.ย. 2569'], ['ครบกำหนด', '26 ธ.ค. 2569']],
        party: { label: 'ลูกค้า', name: 'บริษัท ลูกค้า จำกัด', taxId: '0105561000011', branchNo: '00000', address: '99/1 ถนนสีลม แขวงสีลม เขตบางรัก กรุงเทพมหานคร 10500' },
        lines: [
          { description: 'สินค้า A', quantity: '2', unitPrice: '500', amount: '1000' },
          { description: 'ค่าติดตั้ง', detail: 'ติดตั้งและตั้งค่า', quantity: '1', unitPrice: '300', amount: '300' },
        ],
        totals: [
          { label: 'รวมเงิน', value: '1300' },
          { label: 'ภาษีมูลค่าเพิ่ม 7%', value: '91' },
          { label: 'จำนวนเงินรวมทั้งสิ้น', value: '1391', strong: true },
        ],
        amountInWords: '1391',
        signatures: ['ผู้รับสินค้า / ลูกค้า', 'ผู้มีอำนาจลงนาม'],
      })
  }
}
