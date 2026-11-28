import { format, texts } from '@/config'
import { toString, toUnits } from '@/features/accounting/money'
import { branchLabel, companyPrint, letterheadAddress } from '@/features/print-templates/data'
import { TemplatedPrint } from '@/features/print-templates/TemplatedPrint'
import { formatDate } from '@/lib/format'
import type { Letterhead } from '@/features/sales/types'
import { INCOME_TYPES, type IncomeType, type Payment } from './types'

/**
 * หนังสือรับรองการหักภาษี ณ ที่จ่าย ตามมาตรา 50 ทวิ แห่งประมวลรัษฎากร — พิมพ์ผ่านแม่แบบ (finance.wht_certificate)
 * - ผู้หักภาษี = หัวกระดาษบริษัท, ผู้ถูกหักภาษี = snapshot ผู้รับเงินบนใบสำคัญจ่าย
 * - แบบ ภ.ง.ด.3 / 53 ตามประเภทผู้เสียภาษีของผู้รับเงิน (บุคคลธรรมดา / นิติบุคคล)
 * - ตารางเงินได้: รวมยอดตามประเภทเงินได้ของประเภทภาษีหัก ณ ที่จ่าย (มาตรา 40 / 3 เตรส); ช่อง 5 / 6 แสดงชื่อประเภท
 * - copies = 2 ฉบับ (ฉบับที่ 1 แนบแบบแสดงรายการภาษี / ฉบับที่ 2 เก็บเป็นหลักฐาน)
 */
export function WhtCertificate({ letterhead, payment }: { letterhead: Letterhead; payment: Payment }) {
  return <TemplatedPrint docType="finance.wht_certificate" data={whtCertificateData(letterhead, payment)} />
}

export type WhtCertificateData = ReturnType<typeof whtCertificateData>

/** ข้อมูลพิมพ์ 50 ทวิ — ตัวแปรของแม่แบบ finance.wht_certificate */
export function whtCertificateData(letterhead: Letterhead, payment: Payment) {
  const c = texts.finance.whtCertificate
  const rows = groupByIncome(payment)
  const total = rows.reduce((s, r) => ({ base: s.base + r.base, wht: s.wht + r.wht }), { base: 0n, wht: 0n })
  const form = formOf(payment)
  return {
    company: companyPrint(letterhead),
    title: c.title,
    subtitle: c.subtitle,
    copies: [{ label: c.copy1 }, { label: c.copy2 }],
    book: payment.date.slice(0, 4),
    number: payment.wht_certificate_no ?? '',
    date: payment.date,
    payer: {
      label: c.payer,
      name: letterhead.legal_name_th || letterhead.name,
      tax_id: letterhead.tax_id,
      branch_label: texts.sales.print.headOffice,
      address: letterheadAddress(letterhead),
    },
    payee: {
      label: c.payee,
      name: payment.party.name,
      tax_id: payment.party.tax_id,
      branch_label: branchLabel(payment.party.branch_no),
      address: payment.party.address,
    },
    sequence: payment.wht_form_seq ? String(payment.wht_form_seq) : '',
    form: form ?? '',
    forms: FORMS.map((f) => ({ key: f, label: c.forms[f]!, checked: form === f })),
    incomes: INCOME_TYPES.map((k) => {
      const r = rows.find((x) => x.type === k)
      return {
        key: k,
        label: texts.finance.whtTypes.incomeTypes[k] ?? k,
        names: r && r.names.length > 0 && (k === '3_tres' || k === 'other') ? r.names.join(', ') : '',
        has: !!r,
        amount: r ? toString(r.base, 2) : '',
        tax: r ? toString(r.wht, 2) : '',
      }
    }),
    total_amount: toString(total.base, 2),
    total_tax: toString(total.wht, 2),
    modes: c.modes.map((m, i) => ({ label: m, checked: i === 0 })),
    issued_on: format(c.issuedOn, { date: formatDate(payment.date) }),
    labels: {
      book: c.book,
      number: c.number,
      tax_id: c.taxId,
      name: c.name,
      address: c.address,
      sequence: c.sequence,
      in_form: c.inForm,
      col_income: c.columns.income,
      col_date: c.columns.date,
      col_amount: c.columns.amount,
      col_tax: c.columns.tax,
      total: c.total,
      total_in_words: c.totalInWords,
      funds: c.funds,
      payer_mode: c.payerMode,
      warning: c.warning,
      certify: c.certify,
      signer: c.signer,
      seal: c.seal,
      checked: c.checked,
    },
  }
}

const FORMS = ['pnd1a', 'pnd1a_special', 'pnd2', 'pnd3', 'pnd2a', 'pnd3a', 'pnd53'] as const

/** ภ.ง.ด.3 = บุคคลธรรมดา, ภ.ง.ด.53 = นิติบุคคล (ตาม tax_type ของผู้รับเงิน) */
function formOf(p: Payment): (typeof FORMS)[number] | null {
  if (p.party.tax_type === 'individual') return 'pnd3'
  if (p.party.tax_type === 'juristic') return 'pnd53'
  return null
}

type IncomeRow = { type: IncomeType; names: string[]; base: bigint; wht: bigint }

/** รวมเงินที่จ่าย (ฐานภาษี) / ภาษีที่หัก ตามประเภทเงินได้ — หน่วยตาม toUnits */
function groupByIncome(p: Payment): IncomeRow[] {
  const out: IncomeRow[] = []
  for (const l of p.lines) {
    if (!l.wht_type || toUnits(l.wht) === 0n) continue
    const type = l.wht_type.income_type ?? 'other'
    let r = out.find((x) => x.type === type)
    if (!r) out.push((r = { type, names: [], base: 0n, wht: 0n }))
    if (!r.names.includes(l.wht_type.name)) r.names.push(l.wht_type.name)
    r.base += toUnits(l.wht_base)
    r.wht += toUnits(l.wht)
  }
  return out
}
