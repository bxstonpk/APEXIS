'use client'

import { Plus, Trash2 } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { EditTable } from '@/features/manufacturing/BomForm'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import { TotalsCard } from './Views'
import { voucherTotals, type Ref, type Voucher } from './types'

type Line = { key: number; account: Ref | null; description: string; amount: string }

let nextKey = 1
const newLine = (p: Partial<Line> = {}): Line => ({ key: nextKey++, account: null, description: '', amount: '', ...p })

export type VoucherValues = {
  fund: Ref | null
  date: string
  payee: string
  taxId: string
  branchNo: string
  description: string
  vat: boolean
  vatRate: string
  includesVat: boolean
  taxInvoiceNo: string
  taxInvoiceDate: string
  lines: Line[]
}

/** ตรวจฟอร์มใบสำคัญจ่าย — มี VAT ต้องมีอัตรา 0–100 และเลขที่ / วันที่ใบกำกับภาษี (คืน errors ว่าง = ผ่าน) */
export function validateVoucher(v: VoucherValues): Record<string, string> {
  const e = texts.pettyCash.vouchers.errors
  const errs: Record<string, string> = {}
  if (!v.fund) errs.fund = e.fundRequired
  if (!v.date) errs.date = requiredMessage(texts.pettyCash.vouchers.fields.date)
  if (!v.payee.trim()) errs.payee = e.payeeRequired
  if (v.vat) {
    const r = normalizeAmount(v.vatRate)
    if (!r || toUnits(r) <= 0n || toUnits(r) > 100_000_000n) errs.vatRate = e.vatRateInvalid
    if (!v.taxInvoiceNo.trim()) errs.taxInvoiceNo = e.taxInvoiceRequired
    if (!v.taxInvoiceDate) errs.taxInvoiceDate = e.taxInvoiceRequired
  }
  if (v.lines.length === 0) errs.lines = e.noLines
  for (const l of v.lines) {
    if (!l.account) errs[`acc-${l.key}`] = e.accountRequired
    const n = normalizeAmount(l.amount)
    if (!n || toUnits(n) <= 0n) errs[`amt-${l.key}`] = e.amountInvalid
  }
  return errs
}

/** body ของ POST / PUT /petty-cash-vouchers */
export function voucherBody(v: VoucherValues) {
  return {
    fund_id: v.fund!.id,
    date: v.date,
    payee: { name: v.payee.trim(), tax_id: v.taxId.trim(), branch_no: v.branchNo.trim() },
    description: v.description.trim(),
    vat_rate: v.vat ? normalizeAmount(v.vatRate) : '0',
    price_includes_vat: v.vat && v.includesVat,
    tax_invoice: v.vat ? { no: v.taxInvoiceNo.trim(), date: v.taxInvoiceDate } : null,
    lines: v.lines.map((l) => ({ account_id: l.account!.id, description: l.description.trim(), amount: normalizeAmount(l.amount) })),
  }
}

/**
 * ใบสำคัญจ่ายเงินสดย่อย (ร่าง) — สร้าง / แก้ไข; บันทึกแล้วไปหน้าเอกสาร (อนุมัติจากหน้าเอกสาร)
 * VAT คิดจากยอดรวมทั้งใบ (สูตรเดียวกับ backend) — มี VAT ต้องกรอกเลขที่และวันที่ใบกำกับภาษี
 */
export function VoucherForm({
  apiPath,
  docsPath,
  listHref,
  fundsApi,
  accountsApi,
  vatRate,
  voucher,
  initialFund = null,
}: {
  apiPath: string
  docsPath: string
  listHref: string
  fundsApi: string
  accountsApi: string
  /** อัตรา VAT ตั้งต้น (เช่น 7) */
  vatRate: string
  voucher?: Voucher
  initialFund?: Ref | null
}) {
  const t = texts.pettyCash.vouchers
  const f = t.fields
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const hasVat = !!voucher && toUnits(voucher.vat_rate) > 0n
  const [v, setV] = useState<VoucherValues>({
    fund: voucher?.fund ?? initialFund,
    date: voucher?.date ?? today(),
    payee: voucher?.payee.name ?? '',
    taxId: voucher?.payee.tax_id ?? '',
    branchNo: voucher?.payee.branch_no ?? '',
    description: voucher?.description ?? '',
    vat: hasVat,
    vatRate: hasVat ? voucher!.vat_rate : vatRate,
    includesVat: voucher?.price_includes_vat ?? false,
    taxInvoiceNo: voucher?.tax_invoice?.no ?? '',
    taxInvoiceDate: voucher?.tax_invoice?.date ?? '',
    lines: voucher?.lines.length ? voucher.lines.map((l) => newLine({ account: l.account, description: l.description, amount: l.amount })) : [newLine()],
  })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const set = <K extends keyof VoucherValues>(k: K, value: VoucherValues[K]) => setV((s) => ({ ...s, [k]: value }))
  const upLine = (key: number, patch: Partial<Line>) => setV((s) => ({ ...s, lines: s.lines.map((l) => (l.key === key ? { ...l, ...patch } : l)) }))
  const totals = voucherTotals(
    v.lines.map((l) => l.amount),
    v.vat ? v.vatRate : '0',
    v.includesVat,
  )

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setFormError(null)
    const errs = validateVoucher(v)
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    setBusy(true)
    try {
      const saved = voucher
        ? await apiRequest<{ id: string }>(`${apiPath}/${voucher.id}`, { method: 'PUT', body: voucherBody(v) })
        : await apiRequest<{ id: string }>(apiPath, { method: 'POST', body: voucherBody(v) })
      showToast(voucher ? texts.common.toast.saved : texts.common.toast.created)
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(false)
    }
  }

  const num = '[&_input]:text-right [&_input]:tabular-nums'
  const line = (label: string, n: number) => `${label} ${format(f.line, { n })}`

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}
      <FormSection title={t.sections.general}>
        <ReferenceSelect
          label={f.fund}
          required
          source={fundsApi}
          params={{ status: 'active' }}
          value={v.fund}
          onChange={(x) => set('fund', x)}
          error={errors.fund}
          texts={texts.reference}
        />
        <TextField label={f.date} required type="date" value={v.date} error={errors.date} onChange={(e) => set('date', e.target.value)} />
        <TextField label={f.payee} required value={v.payee} maxLength={200} error={errors.payee} onChange={(e) => set('payee', e.target.value)} />
        <TextField label={f.description} value={v.description} maxLength={500} onChange={(e) => set('description', e.target.value)} />
      </FormSection>

      <FormSection title={t.sections.tax}>
        <label className="flex cursor-pointer items-center gap-3 sm:col-span-2">
          <input type="checkbox" checked={v.vat} onChange={(e) => set('vat', e.target.checked)} className="size-4 accent-primary" />
          <span className="text-sm font-medium">{f.vat}</span>
        </label>
        {v.vat && (
          <>
            <TextField label={f.taxInvoiceNo} required value={v.taxInvoiceNo} maxLength={50} error={errors.taxInvoiceNo} onChange={(e) => set('taxInvoiceNo', e.target.value)} />
            <TextField label={f.taxInvoiceDate} required type="date" value={v.taxInvoiceDate} error={errors.taxInvoiceDate} onChange={(e) => set('taxInvoiceDate', e.target.value)} />
            <TextField label={f.taxId} value={v.taxId} maxLength={20} inputMode="numeric" onChange={(e) => set('taxId', e.target.value)} />
            <TextField label={f.branchNo} value={v.branchNo} maxLength={5} inputMode="numeric" onChange={(e) => set('branchNo', e.target.value)} />
            <TextField label={f.vatRate} required inputMode="decimal" value={v.vatRate} error={errors.vatRate} className={num} onChange={(e) => set('vatRate', e.target.value)} />
            <label className="flex cursor-pointer items-center gap-3 self-end pb-2.5">
              <input type="checkbox" checked={v.includesVat} onChange={(e) => set('includesVat', e.target.checked)} className="size-4 accent-primary" />
              <span className="text-sm font-medium">{f.priceIncludesVat}</span>
            </label>
          </>
        )}
      </FormSection>

      <section aria-labelledby="pcv-lines" className="flex min-w-0 flex-col gap-3">
        <h2 id="pcv-lines" className="text-[15px] font-semibold">
          {t.sections.lines}
        </h2>
        <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_18rem]">
          <div className="flex min-w-0 flex-col gap-3">
            <EditTable head={[['#', 'w-10 text-center'], [f.account, 'w-[38%]'], [f.lineDescription, ''], [f.amount, 'w-40 text-right'], ['', 'w-12']]}>
              {v.lines.map((l, i) => (
                <tr key={l.key} className="border-b border-line align-top last:border-b-0">
                  <td className="px-3 pt-4 text-center text-[13px] text-ink-subtle tabular-nums">{i + 1}</td>
                  <td className="px-2 py-2">
                    <ReferenceSelect
                      label={line(f.account, i + 1)}
                      hideLabel
                      source={accountsApi}
                      params={{ postable: 'true', status: 'active' }}
                      value={l.account}
                      onChange={(a) => upLine(l.key, { account: a })}
                      error={errors[`acc-${l.key}`]}
                      texts={texts.reference}
                    />
                  </td>
                  <td className="px-2 py-2">
                    <TextField label={line(f.lineDescription, i + 1)} hideLabel value={l.description} maxLength={300} onChange={(e) => upLine(l.key, { description: e.target.value })} />
                  </td>
                  <td className="px-2 py-2">
                    <TextField
                      label={line(f.amount, i + 1)}
                      hideLabel
                      inputMode="decimal"
                      value={l.amount}
                      error={errors[`amt-${l.key}`]}
                      className={num}
                      onChange={(e) => upLine(l.key, { amount: e.target.value })}
                    />
                  </td>
                  <td className="px-2 py-2">
                    <button
                      type="button"
                      aria-label={format(t.removeLine, { n: i + 1 })}
                      disabled={v.lines.length <= 1}
                      onClick={() => set('lines', v.lines.filter((x) => x.key !== l.key))}
                      className="flex size-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-subtle hover:bg-danger-soft hover:text-danger disabled:cursor-not-allowed disabled:opacity-40"
                    >
                      <Trash2 aria-hidden className="size-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </EditTable>
            <div>
              <Button size="sm" variant="secondary" onClick={() => set('lines', [...v.lines, newLine()])}>
                <Plus aria-hidden />
                {t.addLine}
              </Button>
            </div>
            {errors.lines && (
              <p role="alert" className="text-[13px] text-danger">
                {errors.lines}
              </p>
            )}
          </div>
          <div aria-live="polite">
            <TotalsCard totals={totals} />
            <p className="sr-only">{formatAmount(totals.gross)}</p>
          </div>
        </div>
      </section>

      <FormBar>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {t.actions.saveDraft}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
