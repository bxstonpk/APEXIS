'use client'

import { Plus, Trash2 } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import { lineAmounts, sumAmounts } from './calc'
import type { Order, Quotation, Ref } from './types'

/**
 * ฟอร์มใบเสนอราคา / ใบสั่งขาย (สร้าง / แก้ร่าง) — ตารางสินค้า + ยอดรวมแสดงสด (สูตรเดียวกับ backend)
 * VAT ใช้ค่าตั้งขายที่โหลดจาก server; ไม่มีสิทธิ์ดูค่าตั้ง = แสดงยอดก่อน VAT และบอกว่า VAT คำนวณตอนบันทึก
 */

type Line = { key: number; item: Ref | null; description: string; quantity: string; unitPrice: string }
let nextKey = 1
const newLine = (p: Partial<Line> = {}): Line => ({ key: nextKey++, item: null, description: '', quantity: '1', unitPrice: '', ...p })

/** YYYY-MM-DD + n วัน (UTC เพื่อไม่ให้เลื่อนตามเขตเวลา) — ใบเสนอราคาใหม่ยืนราคา 30 วันเป็นค่าเริ่มต้น */
export function addDays(iso: string, n: number): string {
  const d = new Date(`${iso}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + n)
  return d.toISOString().slice(0, 10)
}

export type Vat = { registered: boolean; rate: string } | null

export function SalesDocForm({
  kind,
  apiPath,
  customersApi,
  itemsApi,
  docsPath,
  listHref,
  vat,
  doc,
}: {
  kind: 'quotation' | 'order'
  apiPath: string
  customersApi: string
  itemsApi: string
  docsPath: string
  listHref: string
  vat: Vat
  doc?: Quotation | Order
}) {
  const t = texts.sales
  const tl = t.lines
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const quotation = kind === 'quotation' ? (doc as Quotation | undefined) : undefined
  const order = kind === 'order' ? (doc as Order | undefined) : undefined
  const [customer, setCustomer] = useState<Ref | null>(doc?.customer ?? null)
  const [head, setHead] = useState({
    date: doc?.date ?? today(),
    second: quotation?.valid_until ?? order?.delivery_date ?? (kind === 'quotation' && !doc ? addDays(today(), 30) : ''),
    reference: doc?.reference ?? '',
    description: doc?.description ?? '',
    inclusive: doc?.price_includes_vat ?? false,
  })
  const [lines, setLines] = useState<Line[]>(() =>
    doc ? doc.lines.map((l) => newLine({ item: l.item, description: l.description, quantity: l.quantity, unitPrice: l.unit_price })) : [newLine()],
  )
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const rate = doc ? doc.vat_rate : vat?.registered ? vat.rate : '0'
  const vatKnown = !!doc || vat !== null
  const amounts = lines.map((l) => lineAmounts(l.quantity, l.unitPrice, vatKnown ? rate : '0', head.inclusive))
  const total = sumAmounts(amounts)
  const canInclude = doc ? true : vat?.registered !== false

  const up = (key: number, patch: Partial<Line>) => setLines((ls) => ls.map((l) => (l.key === key ? { ...l, ...patch } : l)))

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setFormError(null)
    const errs: Record<string, string> = {}
    if (!customer) errs.customer = requiredMessage(t.fields.customer)
    if (!head.date) errs.date = requiredMessage(t.fields.date)
    if (kind === 'quotation' && !head.second) errs.second = requiredMessage(t.fields.validUntil)
    if (lines.length === 0) errs.lines = tl.noLines
    for (const l of lines) {
      if (!l.item) errs[`item-${l.key}`] = tl.itemRequired
      const q = normalizeAmount(l.quantity)
      if (!q || toUnits(q) <= 0n) errs[`qty-${l.key}`] = tl.qtyInvalid
      const p = normalizeAmount(l.unitPrice)
      if (p === null || p === '' || toUnits(p) < 0n) errs[`price-${l.key}`] = tl.priceInvalid
    }
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    const body: Record<string, unknown> = {
      customer_id: customer!.id,
      date: head.date,
      reference: head.reference.trim(),
      description: head.description.trim(),
      price_includes_vat: head.inclusive,
      lines: lines.map((l) => ({ item_id: l.item!.id, description: l.description.trim(), quantity: normalizeAmount(l.quantity), unit_price: normalizeAmount(l.unitPrice) })),
    }
    if (kind === 'quotation') body.valid_until = head.second
    else body.delivery_date = head.second || null
    setBusy(true)
    try {
      const saved = await apiRequest<{ id: string }>(doc ? `${apiPath}/${doc.id}` : apiPath, { method: doc ? 'PUT' : 'POST', body })
      showToast(t.toast.saved)
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(false)
    }
  }

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}
      <FormSection title={t.sections.header}>
        <ReferenceSelect
          label={t.fields.customer}
          required
          source={customersApi}
          params={{ status: 'active' }}
          value={customer}
          onChange={setCustomer}
          error={errors.customer}
          texts={texts.reference}
          className="sm:col-span-2"
        />
        <TextField label={t.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <TextField
          label={kind === 'quotation' ? t.fields.validUntil : t.fields.deliveryDate}
          required={kind === 'quotation'}
          type="date"
          value={head.second}
          error={errors.second}
          onChange={(e) => setHead((h) => ({ ...h, second: e.target.value }))}
        />
        <TextField label={t.fields.reference} value={head.reference} maxLength={100} onChange={(e) => setHead((h) => ({ ...h, reference: e.target.value }))} />
        <TextField label={t.fields.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
        <label className="flex cursor-pointer items-start gap-3 sm:col-span-2">
          <input
            type="checkbox"
            checked={head.inclusive}
            disabled={!canInclude}
            onChange={(e) => setHead((h) => ({ ...h, inclusive: e.target.checked }))}
            className="mt-0.5 size-4 accent-primary"
          />
          <span>
            <span className="block text-sm font-medium">{t.fields.priceIncludesVat}</span>
            <span className="block text-[13px] text-ink-muted">{t.fields.priceIncludesVatHint}</span>
          </span>
        </label>
      </FormSection>

      <section aria-labelledby="sales-lines" className="flex flex-col gap-3">
        <h2 id="sales-lines" className="text-[15px] font-semibold">
          {t.sections.lines}
        </h2>
        <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          <table className="w-full min-w-[52rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                <th scope="col" className="w-10 px-3 py-2.5 text-center font-medium">
                  #
                </th>
                <th scope="col" className="w-[30%] px-2 py-2.5 font-medium">
                  {tl.item}
                </th>
                <th scope="col" className="px-2 py-2.5 font-medium">
                  {tl.description}
                </th>
                <th scope="col" className="w-28 px-2 py-2.5 text-right font-medium">
                  {tl.quantity}
                </th>
                <th scope="col" className="w-36 px-2 py-2.5 text-right font-medium">
                  {tl.unitPrice}
                </th>
                <th scope="col" className="w-32 px-2 py-2.5 text-right font-medium">
                  {tl.amount}
                </th>
                <th scope="col" className="w-12 px-2 py-2.5">
                  <span className="sr-only">{texts.common.delete}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {lines.map((l, i) => {
                const n = i + 1
                const label = (field: string) => `${field} ${format(tl.line, { n })}`
                return (
                  <tr key={l.key} className="border-b border-line align-top last:border-b-0">
                    <td className="px-3 pt-4 text-center text-[13px] text-ink-subtle tabular-nums">{n}</td>
                    <td className="px-2 py-2">
                      <ReferenceSelect
                        label={label(tl.item)}
                        hideLabel
                        source={itemsApi}
                        params={{ status: 'active' }}
                        value={l.item}
                        onChange={(item) => up(l.key, { item })}
                        error={errors[`item-${l.key}`]}
                        texts={texts.reference}
                      />
                    </td>
                    <td className="px-2 py-2">
                      <TextField label={label(tl.description)} hideLabel value={l.description} maxLength={500} onChange={(e) => up(l.key, { description: e.target.value })} />
                    </td>
                    <td className="px-2 py-2">
                      <TextField
                        label={label(tl.quantity)}
                        hideLabel
                        inputMode="decimal"
                        value={l.quantity}
                        error={errors[`qty-${l.key}`]}
                        className="[&_input]:text-right [&_input]:tabular-nums"
                        onChange={(e) => up(l.key, { quantity: e.target.value })}
                      />
                    </td>
                    <td className="px-2 py-2">
                      <TextField
                        label={label(tl.unitPrice)}
                        hideLabel
                        inputMode="decimal"
                        value={l.unitPrice}
                        error={errors[`price-${l.key}`]}
                        className="[&_input]:text-right [&_input]:tabular-nums"
                        onChange={(e) => up(l.key, { unitPrice: e.target.value })}
                      />
                    </td>
                    <td className="px-2 pt-4 text-right tabular-nums">{formatAmount(head.inclusive ? amounts[i]!.gross : amounts[i]!.net)}</td>
                    <td className="px-2 py-2">
                      <button
                        type="button"
                        aria-label={format(tl.remove, { n })}
                        disabled={lines.length <= 1}
                        onClick={() => setLines((ls) => ls.filter((x) => x.key !== l.key))}
                        className="flex size-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-subtle hover:bg-danger-soft hover:text-danger disabled:cursor-not-allowed disabled:opacity-40"
                      >
                        <Trash2 aria-hidden className="size-4" />
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
        <div className="flex flex-wrap items-start justify-between gap-4">
          <Button size="sm" variant="secondary" onClick={() => setLines((ls) => [...ls, newLine()])}>
            <Plus aria-hidden />
            {tl.add}
          </Button>
          <dl aria-live="polite" className="w-full max-w-xs rounded-(--radius-panel) border border-line bg-surface p-4 text-sm shadow-card">
            <div className="flex justify-between gap-4">
              <dt className="text-ink-muted">{t.totals.net}</dt>
              <dd className="tabular-nums">{formatAmount(total.net)}</dd>
            </div>
            <div className="mt-2 flex justify-between gap-4">
              <dt className="text-ink-muted">{format(t.totals.vat, { rate: vatKnown ? rate : '–' })}</dt>
              <dd className="tabular-nums">{vatKnown ? formatAmount(total.vat) : '–'}</dd>
            </div>
            <div className="mt-2 flex justify-between gap-4 border-t border-line pt-2 font-semibold">
              <dt>{t.totals.gross}</dt>
              <dd className="tabular-nums">{formatAmount(total.gross)}</dd>
            </div>
            {!vatKnown && <p className="mt-2 text-[12px] text-ink-muted">{t.totals.vatLater}</p>}
          </dl>
        </div>
        {errors.lines && (
          <p role="alert" className="text-[13px] text-danger">
            {errors.lines}
          </p>
        )}
      </section>

      <FormBar>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {t.actions.save}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
