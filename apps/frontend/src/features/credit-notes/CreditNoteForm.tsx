'use client'

import { useRouter } from 'next/navigation'
import { useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toString, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { lineAmounts, sumAmounts } from '@/features/sales/calc'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import type { CreditKind, CreditableInvoice } from './types'

/**
 * ใบลดหนี้จากใบกำกับภาษีที่ผ่านรายการแล้ว — ใช้ร่วมกันทั้งขาย (ใบลดหนี้ที่ออกให้ลูกค้า) และจัดซื้อ (ใบลดหนี้ที่ได้รับจากผู้ขาย)
 *
 *   รับคืน / ส่งคืนสินค้า (return): จำนวน ≤ ออกบิล − คืนแล้ว, ราคาตั้งต้นตามใบกำกับ (ลดได้) — สินค้านับสต็อกเข้า / ออกคลังที่เลือก
 *   ลดราคา / ส่วนลด (allowance): ระบุจำนวนเงินที่ลดต่อบรรทัด ไม่เคลื่อนไหวสต็อก
 *   ยอดลดหนี้ต่อบรรทัดไม่เกินมูลค่าบรรทัดที่ยังไม่ลด; ยอดรวมไม่เกินยอดค้างของใบกำกับ (backend ตรวจซ้ำ)
 * ราคา / จำนวนเงินใช้เงื่อนไข VAT เดียวกับใบกำกับเดิม (รวม / ไม่รวม VAT)
 */
export function CreditNoteForm({
  mode,
  invoice,
  apiPath,
  warehousesApi,
  docsPath,
  invoiceHref,
}: {
  mode: 'sales' | 'purchasing'
  invoice: CreditableInvoice
  apiPath: string
  warehousesApi: string
  docsPath: string
  invoiceHref: string
}) {
  const t = texts.creditNotes
  const f = t.form
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const [kind, setKind] = useState<CreditKind>('return')
  const [head, setHead] = useState({ date: today(), docDate: today(), supplierNo: '', reason: '', description: '' })
  const [warehouse, setWarehouse] = useState<{ id: string; code: string; name: string } | null>(null)
  const [qty, setQty] = useState<Record<number, string>>({})
  const [price, setPrice] = useState<Record<number, string>>(() => Object.fromEntries(invoice.lines.map((l) => [l.line_no, l.unit_price])))
  const [amount, setAmount] = useState<Record<number, string>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const purchasing = mode === 'purchasing'

  const left = (l: CreditableInvoice['lines'][number]) => {
    const q = toUnits(l.quantity) - toUnits(l.returned_quantity)
    return q > 0n ? q : 0n
  }
  const leftValue = (l: CreditableInvoice['lines'][number]) => {
    const v = toUnits(l.amounts.net) - toUnits(l.credited_amount)
    return v > 0n ? v : 0n
  }

  /** ยอดของบรรทัด (ตามเงื่อนไข VAT ของใบกำกับ) — ใช้แสดงตัวอย่าง, backend คำนวณจริง */
  const lineTotal = (l: CreditableInvoice['lines'][number]) => {
    if (kind === 'return') {
      const q = normalizeAmount(qty[l.line_no] ?? '')
      const p = normalizeAmount(price[l.line_no] ?? '')
      if (!q || !p || toUnits(q) <= 0n) return null
      return lineAmounts(q, p, invoice.vat_rate, invoice.price_includes_vat)
    }
    const a = normalizeAmount(amount[l.line_no] ?? '')
    if (!a || toUnits(a) <= 0n) return null
    return lineAmounts('1', a, invoice.vat_rate, invoice.price_includes_vat)
  }
  const preview = sumAmounts(invoice.lines.map(lineTotal).filter((x) => x !== null))

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: Record<string, string> = {}
    if (!head.date) errs.date = requiredMessage(purchasing ? f.postingDate : f.date)
    if (purchasing && !head.supplierNo.trim()) errs.supplierNo = requiredMessage(f.supplierNo)
    if (!head.reason.trim()) errs.reason = requiredMessage(f.reason)
    const lines: Record<string, unknown>[] = []
    for (const l of invoice.lines) {
      if (kind === 'return') {
        const raw = qty[l.line_no] ?? ''
        if (!raw.trim()) continue
        const q = normalizeAmount(raw)
        if (q === null || toUnits(q) < 0n) errs[`q-${l.line_no}`] = texts.sales.lines.qtyInvalid
        else if (toUnits(q) > left(l)) errs[`q-${l.line_no}`] = f.overRemaining
        else if (toUnits(q) > 0n) {
          const p = normalizeAmount(price[l.line_no] ?? '')
          if (p === null || p === '' || toUnits(p) <= 0n) errs[`p-${l.line_no}`] = texts.sales.lines.priceInvalid
          else if (toUnits(p) > toUnits(l.unit_price)) errs[`p-${l.line_no}`] = f.overPrice
          else lines.push({ invoice_line_no: l.line_no, quantity: q, unit_price: p })
        }
      } else {
        const raw = amount[l.line_no] ?? ''
        if (!raw.trim()) continue
        const a = normalizeAmount(raw)
        if (a === null || toUnits(a) < 0n) errs[`a-${l.line_no}`] = texts.sales.lines.priceInvalid
        else if (toUnits(a) > 0n) {
          const net = lineAmounts('1', a, invoice.vat_rate, invoice.price_includes_vat).net
          if (net > leftValue(l)) errs[`a-${l.line_no}`] = f.overValue
          else lines.push({ invoice_line_no: l.line_no, amount: a })
        }
      }
    }
    if (!Object.keys(errs).length && lines.length === 0) errs.lines = f.nothingToDo
    if (!errs.lines && preview.gross > toUnits(invoice.outstanding)) errs.lines = format(f.overOutstanding, { amount: formatAmount(invoice.outstanding) })
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    const body: Record<string, unknown> = {
      invoice_id: invoice.id,
      date: head.date,
      kind,
      reason: head.reason.trim(),
      description: head.description.trim(),
      warehouse_id: kind === 'return' ? (warehouse?.id ?? null) : null,
      lines,
    }
    if (purchasing) {
      body.supplier_credit_note_no = head.supplierNo.trim()
      body.credit_note_date = head.docDate || head.date
    }
    setBusy(true)
    setError(null)
    try {
      const saved = await apiRequest<{ id: string; document_no: string }>(apiPath, { method: 'POST', body })
      showToast(format(texts.sales.toast.posted, { no: saved.document_no }))
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
      setBusy(false)
    }
  }

  const num = 'px-3 py-2.5 text-right tabular-nums'
  const priceLabel = invoice.price_includes_vat ? f.priceIncl : f.priceExcl
  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {error && <Alert tone="error">{error}</Alert>}
      <FormSection title={texts.sales.sections.header} description={`${f.forInvoice} ${invoice.no} · ${invoice.party}`}>
        <fieldset className="flex flex-col gap-2 sm:col-span-2">
          <legend className="mb-1 text-[13px] font-medium">{f.kind}</legend>
          <div className="grid gap-2 sm:grid-cols-2">
            {(['return', 'allowance'] as const).map((k) => (
              <label
                key={k}
                className={`flex cursor-pointer items-start gap-3 rounded-(--radius-control) border p-3 text-sm transition-colors ${kind === k ? 'border-primary bg-primary-soft' : 'border-line hover:border-ink-subtle'}`}
              >
                <input type="radio" name="kind" value={k} checked={kind === k} onChange={() => setKind(k)} className="mt-0.5 size-4 accent-primary" />
                <span>
                  <span className="block font-medium">{t.kinds[mode][k]}</span>
                  <span className="block text-[13px] text-ink-muted">{t.kindHints[mode][k]}</span>
                </span>
              </label>
            ))}
          </div>
        </fieldset>
        {purchasing && (
          <>
            <TextField
              label={f.supplierNo}
              required
              value={head.supplierNo}
              maxLength={50}
              error={errors.supplierNo}
              onChange={(e) => setHead((h) => ({ ...h, supplierNo: e.target.value }))}
            />
            <TextField label={f.docDate} type="date" value={head.docDate} onChange={(e) => setHead((h) => ({ ...h, docDate: e.target.value }))} />
          </>
        )}
        <TextField
          label={purchasing ? f.postingDate : f.date}
          required
          type="date"
          value={head.date}
          error={errors.date}
          onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))}
        />
        {kind === 'return' && (
          <ReferenceSelect
            label={purchasing ? f.warehouseOut : f.warehouseIn}
            hint={f.warehouseHint}
            source={warehousesApi}
            params={{ status: 'active' }}
            value={warehouse}
            onChange={setWarehouse}
            texts={texts.reference}
          />
        )}
        <TextField
          label={f.reason}
          required
          hint={f.reasonHint}
          value={head.reason}
          maxLength={500}
          error={errors.reason}
          className="sm:col-span-2"
          list="credit-note-reasons"
          onChange={(e) => setHead((h) => ({ ...h, reason: e.target.value }))}
        />
        <datalist id="credit-note-reasons">
          {t.reasons[kind].map((r) => (
            <option key={r} value={r} />
          ))}
        </datalist>
        <TextField
          label={texts.sales.fields.description}
          value={head.description}
          maxLength={500}
          className="sm:col-span-2"
          onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))}
        />
      </FormSection>

      <section aria-labelledby="cn-lines" className="flex min-w-0 flex-col gap-3">
        <h2 id="cn-lines" className="text-[15px] font-semibold">
          {f.lines}
        </h2>
        <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          <table className="w-full min-w-[52rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                <th scope="col" className="px-3 py-2.5 font-medium">
                  {texts.sales.lines.item}
                </th>
                {kind === 'return'
                  ? [f.invoiced, f.returned, f.remaining].map((h) => (
                      <th key={h} scope="col" className="px-3 py-2.5 text-right font-medium">
                        {h}
                      </th>
                    ))
                  : [f.lineValue, f.credited, f.remainingValue].map((h) => (
                      <th key={h} scope="col" className="px-3 py-2.5 text-right font-medium">
                        {h}
                      </th>
                    ))}
                {kind === 'return' ? (
                  <>
                    <th scope="col" className="w-32 px-3 py-2.5 text-right font-medium">
                      {f.returnQty}
                    </th>
                    <th scope="col" className="w-36 px-3 py-2.5 text-right font-medium">
                      {priceLabel}
                    </th>
                  </>
                ) : (
                  <th scope="col" className="w-40 px-3 py-2.5 text-right font-medium">
                    {invoice.price_includes_vat ? f.allowanceIncl : f.allowanceExcl}
                  </th>
                )}
              </tr>
            </thead>
            <tbody>
              {invoice.lines.map((l) => {
                const code = l.item.code
                return (
                  <tr key={l.line_no} className="border-b border-line align-top last:border-b-0">
                    <td className="px-3 py-2.5">
                      <span className="text-[13px] text-ink-muted">{code}</span> {l.item.name}
                      <span className="block text-[12px] text-ink-muted">
                        {format(f.invoicePrice, { price: formatAmount(l.unit_price) })}
                      </span>
                    </td>
                    {kind === 'return' ? (
                      <>
                        <td className={num}>{formatAmount(l.quantity)}</td>
                        <td className={num}>{formatAmount(l.returned_quantity, { dashZero: true })}</td>
                        <td className={`${num} font-medium`}>{formatAmount(left(l))}</td>
                        <td className="px-3 py-2">
                          <TextField
                            label={`${f.returnQty} ${code}`}
                            hideLabel
                            inputMode="decimal"
                            placeholder="0"
                            value={qty[l.line_no] ?? ''}
                            disabled={left(l) === 0n}
                            error={errors[`q-${l.line_no}`]}
                            className="[&_input]:text-right [&_input]:tabular-nums"
                            onChange={(e) => setQty((q) => ({ ...q, [l.line_no]: e.target.value }))}
                          />
                        </td>
                        <td className="px-3 py-2">
                          <TextField
                            label={`${priceLabel} ${code}`}
                            hideLabel
                            inputMode="decimal"
                            value={price[l.line_no] ?? ''}
                            disabled={left(l) === 0n}
                            error={errors[`p-${l.line_no}`]}
                            className="[&_input]:text-right [&_input]:tabular-nums"
                            onChange={(e) => setPrice((p) => ({ ...p, [l.line_no]: e.target.value }))}
                          />
                        </td>
                      </>
                    ) : (
                      <>
                        <td className={num}>{formatAmount(l.amounts.net)}</td>
                        <td className={num}>{formatAmount(l.credited_amount, { dashZero: true })}</td>
                        <td className={`${num} font-medium`}>{formatAmount(leftValue(l))}</td>
                        <td className="px-3 py-2">
                          <TextField
                            label={`${f.allowanceExcl} ${code}`}
                            hideLabel
                            inputMode="decimal"
                            placeholder="0.00"
                            value={amount[l.line_no] ?? ''}
                            disabled={leftValue(l) === 0n}
                            error={errors[`a-${l.line_no}`]}
                            className="[&_input]:text-right [&_input]:tabular-nums"
                            onChange={(e) => setAmount((a) => ({ ...a, [l.line_no]: e.target.value }))}
                          />
                        </td>
                      </>
                    )}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
        {errors.lines && (
          <p role="alert" className="text-[13px] text-danger">
            {errors.lines}
          </p>
        )}
        <dl className="ml-auto grid w-full max-w-sm grid-cols-[1fr_auto] gap-x-6 gap-y-1 rounded-(--radius-panel) border border-line bg-surface p-4 text-sm shadow-card">
          <dt className="text-ink-muted">{f.net}</dt>
          <dd className="text-right tabular-nums">{formatAmount(preview.net)}</dd>
          <dt className="text-ink-muted">{format(f.vat, { rate: toString(toUnits(invoice.vat_rate)) })}</dt>
          <dd className="text-right tabular-nums">{formatAmount(preview.vat)}</dd>
          <dt className="border-t border-line pt-1 font-semibold">{f.gross}</dt>
          <dd className="border-t border-line pt-1 text-right font-semibold tabular-nums">{formatAmount(preview.gross)}</dd>
          <dt className="text-[12px] text-ink-muted">{f.outstanding}</dt>
          <dd className="text-right text-[12px] text-ink-muted tabular-nums">{formatAmount(invoice.outstanding)}</dd>
        </dl>
      </section>

      <FormBar>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={invoiceHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {t.actions.post}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
