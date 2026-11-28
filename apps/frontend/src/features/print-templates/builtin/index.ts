import type { Paper, TemplateDocType } from '../types'
import { DOCUMENT_TEMPLATE } from './document'
import { PP30_TEMPLATE, VAT_REPORT_TEMPLATE, WITHHOLDING_TEMPLATE } from './taxforms'
import { WHT_CERTIFICATE_TEMPLATE } from './wht'

/** กลุ่มรูปแบบข้อมูลพิมพ์ — แม่แบบของชนิดเอกสารในกลุ่มเดียวกันใช้ตัวแปรชุดเดียวกัน (ดู catalog.ts) */
export type PrintFamily = 'document' | 'wht_certificate' | 'withholding' | 'pp30' | 'vat_report'

export function familyOf(docType: TemplateDocType): PrintFamily {
  switch (docType) {
    case 'finance.wht_certificate':
      return 'wht_certificate'
    case 'finance.pnd3':
    case 'finance.pnd53':
      return 'withholding'
    case 'finance.pp30':
      return 'pp30'
    case 'finance.vat_report_output':
    case 'finance.vat_report_input':
      return 'vat_report'
    default:
      return 'document'
  }
}

const BY_FAMILY: Record<PrintFamily, { html: string; paper: Paper }> = {
  document: { html: DOCUMENT_TEMPLATE, paper: 'a4_portrait' },
  wht_certificate: { html: WHT_CERTIFICATE_TEMPLATE, paper: 'a4_portrait' },
  withholding: { html: WITHHOLDING_TEMPLATE, paper: 'a4_landscape' },
  pp30: { html: PP30_TEMPLATE, paper: 'a4_portrait' },
  vat_report: { html: VAT_REPORT_TEMPLATE, paper: 'a4_landscape' },
}

/** แม่แบบมาตรฐานของระบบ (อ่านอย่างเดียว — บริษัทคัดลอกไปแก้เป็นแม่แบบของตัวเองได้) */
export function builtinTemplate(docType: TemplateDocType): { html: string; paper: Paper } {
  return BY_FAMILY[familyOf(docType)]
}
